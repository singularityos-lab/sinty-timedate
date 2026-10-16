package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/singularityos-lab/sinty-timedate/internal/timedated"
)

func connectBus() (*dbus.Conn, error) {
	if os.Getenv("DBUS_SYSTEM_BUS_ADDRESS") == "" {
		os.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/run/dbus/system_bus_socket")
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		conn, err := dbus.ConnectSystemBus()
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("sinty-timedated: ")

	conn, err := connectBus()
	if err != nil {
		log.Fatalf("system bus: %v", err)
	}
	defer conn.Close()

	svc := timedated.New(conn, timedated.System{})
	if err := svc.Export(); err != nil {
		log.Fatalf("export: %v", err)
	}
	reply, err := conn.RequestName(timedated.BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatalf("request name %s: %v", timedated.BusName, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("%s already owned (systemd-timedated running?) - not starting", timedated.BusName)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
