package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"
	"net/http"

	"go.sakib.dev/le/logger"
	"go.sakib.dev/le/pkg/utils"
)

type Server struct {
	Dir  string
	Port int

	server *http.Server

	subLock     sync.RWMutex
	subscribers []chan ServerEvent
}

func NewServer(dir string, port int) (*Server, error) {
	dir, err := utils.ValidAbsDir(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	slog.SetDefault(slog.New(logger.NewHandler()))

	slog.Info("Got directory:", "dir", dir)

	return &Server{
		Dir:  dir,
		Port: port, subLock: sync.RWMutex{},
	}, nil
}

func (s *Server) Subscribe(ch chan ServerEvent) {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	s.subscribers = append(s.subscribers, ch)
}

func (s *Server) Unsubscribe(ch chan ServerEvent) {
	s.subLock.Lock()
	defer s.subLock.Unlock()
	for i, subscriber := range s.subscribers {
		if subscriber == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			return
		}
	}
}

func (s *Server) Start() error {

	eventCh := make(chan ServerEvent, 100)
	handler := newHandler(s.Dir, eventCh)

	go func() {
		for event := range eventCh {
			s.publish(event)
		}
	}()

	s.PrintUrl()

	addr := fmt.Sprintf(":%d", s.Port)
	s.server = &http.Server{Addr: addr, Handler: handler}
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
}

func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}
	slog.Info("Closing server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) PrintUrl() {
	localIP, err := utils.GetLocalIP()
	if err != nil {
		slog.Error("Error getting local IP", "error", err)
		localIP = "localhost"
	}

	url := fmt.Sprintf("http://%s:%d", localIP, s.Port)
	slog.Info("Serving files from", "directory", s.Dir)
	slog.Info("File server is running on", "url", url)

	s.publish(&EventAddrUpdated{Addr: url, Dir: s.Dir, Time: time.Now()})

}

func (s *Server) publish(event ServerEvent) {
	s.subLock.RLock()
	defer s.subLock.RUnlock()
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
