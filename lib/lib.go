package lib

import (
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/irai/packet"
	"github.com/irai/packet/fastlog"
	probing "github.com/prometheus-community/pro-bing"
)

type NicSessionMQTTBridge struct {
	MQTTClient mqtt.Client
	NicSession *packet.Session
}

func foo() {
	pinger, err := probing.NewPinger("www.google.com")
	if err != nil {
		panic(err)
	}
	pinger.Count = 3
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		panic(err)
	}
}

func CreateNicSession(nic string) *packet.Session {
	packet.Logger.SetLevel(fastlog.LevelError)

	s, err := packet.Config{
		ProbeDeadline:   packet.DefaultProbeDeadline,
		OfflineDeadline: time.Minute * 10,
		PurgeDeadline:   packet.DefaultPurgeDeadline}.
		NewSession(nic)

	if err != nil {
		slog.Error("Could not connect to nic", "nic", nic, "error", err)
		panic(err)
	}

	slog.Info("Nic session opened", "nic", nic)
	return s
}

func CreateMQTTClient(mqttBroker string) mqtt.Client {
	slog.Info("Creating MQTT client", "broker", mqttBroker)
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		slog.Error("Could not connect to broker", "mqttBroker", mqttBroker, "error", token.Error())
		panic(token.Error())
	}
	slog.Info("Connected to MQTT broker", "mqttBroker", mqttBroker)
	return client
}

func NewNicSessionMQTTBridge(session *packet.Session, mqttClient mqtt.Client) *NicSessionMQTTBridge {
	slog.Info("Creating NicSession-MQTT bridge")
	bridge := &NicSessionMQTTBridge{
		MQTTClient: mqttClient,
		NicSession: session,
	}
	slog.Info("NicSession-MQTT bridge initialized")
	return bridge
}

func (bridge *NicSessionMQTTBridge) PublishMQTT(topic string, message string, retained bool) {
	token := bridge.MQTTClient.Publish(topic, 0, retained, message)
	token.Wait()
}

func (bridge *NicSessionMQTTBridge) Close() {
	bridge.NicSession.Close()
}

func (bridge *NicSessionMQTTBridge) MainLoop() {
	go bridge.NotificationLoop()
	go bridge.PingLoop()
	bridge.ReadLoop()
}

func (bridge *NicSessionMQTTBridge) ReadLoop() {
	buffer := make([]byte, packet.EthMaxSize)
	for {
		n, _, err := bridge.NicSession.ReadFrom(buffer)
		if err != nil {
			//slog.Debug("Error reading packet", "error", err)
			return
		}

		frame, err := bridge.NicSession.Parse(buffer[:n])
		if err != nil {
			//slog.Debug("Parse error", "error", err)
			continue
		}
		bridge.NicSession.Notify(frame)
	}
}

func (bridge *NicSessionMQTTBridge) NotificationLoop() {
	for {
		notification := <-bridge.NicSession.C
		slog.Debug("Address notification", "notification", notification, "mac", notification.Addr.MAC.String())
		bridge.PublishMQTT("net/mac/"+notification.Addr.MAC.String()+"/online", strconv.FormatBool(notification.Online), false)
	}
}

func (bridge *NicSessionMQTTBridge) PingLoop() {
	for {
		time.Sleep(2 * time.Minute)
		for addr, host := range bridge.NicSession.HostTable.Table {
			slog.Debug("Pinging", "addr", addr, "host", host)
			pinger, err := probing.NewPinger(addr.String())
			if err != nil {
				panic(err)
			}
			pinger.Count = 1
			pinger.OnRecv = func(pkt *probing.Packet) {
				slog.Debug("Ping returned", "ip", pkt.IPAddr.String())
				bridge.PublishMQTT("net/ip/"+pkt.IPAddr.String()+"/ping",
					strconv.FormatInt(pkt.Rtt.Milliseconds(), 10), false)
			}
			err = pinger.Run() // Blocks until finished.
			if err != nil {
				slog.Error("Error pinging", "addr", addr, "host", host)
			}
		}
	}
}
