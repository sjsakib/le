package main

import (
	"log"
	"sync"

	"go.sakib.dev/le/pkg/config"
	"go.sakib.dev/le/pkg/server"
	"go.sakib.dev/le/pkg/tui"
)

func main() {

	config, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	srvr, err := server.NewServer(config.Dir, config.Port)

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	wg := sync.WaitGroup{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		err = tui.Start(srvr)
		if err != nil {
			log.Fatalf("Failed to start TUI: %v", err)
		}

	}()


	go func() {
		defer wg.Done()
		if err := srvr.Start(); err != nil {
			log.Fatalf("Failed to start srvr: %v", err)
		}
	}()

	wg.Wait()
}
