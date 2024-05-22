package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	lib "github.com/claes/arp-mqtt/lib"
)

var debug *bool

func MainLoop(bridge lib.NicSessionMQTTBridge) {
	for {
	}
}

func printHelp() {
	fmt.Println("Usage: arp-mqtt [OPTIONS]")
	fmt.Println("Options:")
	flag.PrintDefaults()
}

func main() {
	nic := flag.String("nic", "wlo1", "Network interface")
	mqttBroker := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	help := flag.Bool("help", false, "Print help")
	debug = flag.Bool("debug", false, "Debug logging")
	flag.Parse()

	if *debug {
		var programLevel = new(slog.LevelVar)
		programLevel.Set(slog.LevelDebug)
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel})
		slog.SetDefault(slog.New(handler))
	}

	if *help {
		printHelp()
		os.Exit(0)
	}

	s := lib.CreateNicSession(*nic)
	defer s.Close()
	bridge := lib.NewNicSessionMQTTBridge(s, lib.CreateMQTTClient(*mqttBroker))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	slog.Info("Started")
	go MainLoop(*bridge)
	<-c
	// bridge.Controller.Close()

	slog.Info("Shut down")
	//bridge.CECConnection.Destroy()
	slog.Info("Exit")

	os.Exit(0)
}
