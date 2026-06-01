package main

import (
	"flag"
	"log"
	"sync"

	"go.sakib.dev/le/pkg/server"
	"go.sakib.dev/le/pkg/tui"
)

func main() {
	dir := flag.String("dir", ".", "Directory to serve files from")
	port := flag.Int("port", 8080, "Port to run the file server on")

	flag.Parse()

	srvr, err := server.NewServer(*dir, *port)

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
