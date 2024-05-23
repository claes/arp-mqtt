package lib

import (
	"log/slog"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/irai/packet"
	"github.com/irai/packet/fastlog"
)

type NicSessionMQTTBridge struct {
	MQTTClient mqtt.Client
	NicSession *packet.Session
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
	go func() {
		for {
			notification := <-bridge.NicSession.C
			slog.Debug("Address notification", "notification", notification, "mac", notification.Addr.MAC.String())
			bridge.PublishMQTT("net/mac/"+notification.Addr.MAC.String()+"/online", strconv.FormatBool(notification.Online), false)
		}
	}()

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
