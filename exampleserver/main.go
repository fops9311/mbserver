package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	mbserver "github.com/fops9311/mbserver"
	"github.com/tarm/serial"
)

var port string

func main() {

	flag.StringVar(&port, "port", "COM8", "write port as in devices (COM8 - default)")

	flag.Parse()

	serv := mbserver.NewServer()

	serv.HoldingRegisters = make([]uint16, 45000)
	serv.InputRegisters = make([]uint16, 45000)
	serv.Coils = make([]byte, 500)
	serv.DiscreteInputs = make([]byte, 45000)
	serv.Debug = true
	for i := range serv.HoldingRegisters {
		serv.HoldingRegisters[i] = uint16(i)
	}
	err := serv.ListenRTU(&serial.Config{Name: port, Baud: 9600, StopBits: serial.Stop1})
	if err != nil {
		log.Fatalf("failed to listen, got %v\n", err)
	}
	f := func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("...end...")
				os.Exit(0)
				return nil
			case <-time.After(1 * time.Second):
				//fmt.Println("Hello in a loop de loop")
			}
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	f(ctx)
}
