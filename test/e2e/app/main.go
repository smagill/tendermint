package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tendermint/tendermint/abci/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	app, err := NewApplication()
	if err != nil {
		return err
	}
	server, err := server.NewServer("tcp://0.0.0.0:27000", "socket", app)
	if err != nil {
		return err
	}
	err = server.Start()
	if err != nil {
		return err
	}
	// Apparently there's no way to wait for the server, so we just sleep
	for {
		time.Sleep(1 * time.Hour)
	}
}
