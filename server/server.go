package server

import (
	"fmt"

	"log/slog"
	"net/http"

	"go.sakib.dev/le/logger"
	"go.sakib.dev/le/pkg/utils"
)

type Server struct {
	Dir     string
	Port    int
	state   ServerState
	eventCh chan ServerEventName
}

func NewServer(dir string, port int, ch chan ServerEventName) (*Server, error) {
	dir, err := utils.ValidAbsDir(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	slog.SetDefault(slog.New(logger.NewHandler()))

	slog.Info("Got directory:", "dir", dir)

	return &Server{
		Dir:     dir,
		Port:    port,
		eventCh: ch,
		state: ServerState{
			Dir:       utils.ReplaceHome(dir),
			Conns:     make(map[string]*Conn),
			Downloads: make(map[string]*Download),
			Clients:   make(map[string]*Client),
		},
	}, nil
}

func (s *Server) Start() error {
	ch := make(chan ServerEvent, 100)
	handler := newHandler(s.Dir, ch)

	s.PrintUrl()

	go s.listenForData(ch)

	addr := fmt.Sprintf(":%d", s.Port)
	err := http.ListenAndServe(addr, handler)

	if err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}

	return nil
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

	s.state.Addr = &url
	s.publish(EvNameAddrUpdated)

}

func (s *Server) publish(event ServerEventName) {
	if s.eventCh != nil {
		s.eventCh <- event
	}
}

func (s *Server) GetState() *ServerState {
	return &s.state
}

func (s *Server) listenForData(ch <-chan ServerEvent) {
	for data := range ch {
		switch data := data.(type) {
		case EventConnClose:
			s.handleConnClose(&data)
		case EventDownloadProgress:
			s.handleDownloadProgress(&data)
		case EventDownloadStart:
			s.handleDownloadStart(&data)
		default:
			slog.Warn("Unknown server event", "event", data)
		}
	}
}

func (s *Server) handleConnClose(event *EventConnClose) {
	if _, exists := s.state.Conns[event.ConnID]; exists {
		delete(s.state.Conns, event.ConnID)
		s.publish(EvNameConnClose)
	} else {
		slog.Warn("Connection close event for unknown connection", "conn_id", event.ConnID)
	}
}

func (s *Server) handleDownloadStart(event *EventDownloadStart) {
	s.state.HandleDownloadStart(event)
	s.publish(EvNameDownloadProgress)
}

func (s *Server) handleDownloadProgress(event *EventDownloadProgress) {
	s.state.HandleDownloadProgress(event)
	s.publish(EvNameDownloadProgress)
}
