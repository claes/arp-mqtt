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
	MQTTClient  mqtt.Client
	NicSession  *packet.Session
	TopicPrefix string
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

func NewNicSessionMQTTBridge(session *packet.Session, mqttClient mqtt.Client, topicPrefix string) *NicSessionMQTTBridge {
	slog.Info("Creating NicSession-MQTT bridge")
	bridge := &NicSessionMQTTBridge{
		MQTTClient:  mqttClient,
		NicSession:  session,
		TopicPrefix: topicPrefix,
	}
	slog.Info("NicSession-MQTT bridge initialized")
	return bridge
}

func (bridge *NicSessionMQTTBridge) PublishMQTT(subtopic string, message string, retained bool) {
	token := bridge.MQTTClient.Publish(bridge.TopicPrefix+"/"+subtopic, 0, retained, message)
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
			if addr.Is4() {
				slog.Debug("Pinging", "addr", addr, "host", host)
				pinger, err := probing.NewPinger(addr.String())
				if err != nil {
					slog.Error("Could not create pinger", "addr", addr.String())
					continue
				}
				pinger.Count = 1
				pinger.Timeout = time.Duration(2 * time.Second)
				pinger.OnRecv = func(pkt *probing.Packet) {
					slog.Debug("Ping returned", "ip", addr.String())
					bridge.PublishMQTT("net/mac/"+host.Addr.MAC.String()+"/ping",
						strconv.FormatInt(pkt.Rtt.Milliseconds(), 10), false)
					bridge.PublishMQTT("net/ip/"+addr.String()+"/ping",
						strconv.FormatInt(pkt.Rtt.Milliseconds(), 10), false)
				}
				// pinger.OnRecvError = func(err error) {
				// 	slog.Debug("Ping error", "ip", addr.String())
				// 	bridge.PublishMQTT("net/mac/"+host.Addr.MAC.String()+"/ping", "error", false)
				// 	bridge.PublishMQTT("net/ip/"+addr.String()+"/ping", "error", false)
				// }
				err = pinger.Run() // Blocks until finished.
				if err != nil {
					slog.Error("Error pinging", "addr", addr, "host", host)
				}
			}
		}
	}
}
