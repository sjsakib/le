package state

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.sakib.dev/le/pkg/server"
)

type Conn struct {
	ID        string
	ClientID  *string
	CreatedAt time.Time
	UpdatedAt time.Time

	DownloadChunk *DownloadChunk
}

type DownloadChunk struct {
	ConnID    string
	Start     int64
	End       int64
	Sent      int64
	StartedAt time.Time
	UpdatedAt time.Time
}

type Download struct {
	ID              string
	ClientID        string
	FileDisplayPath string
	TotalSize       int64
	StartedAt       time.Time
	Chunks          []*DownloadChunk
}

type ServerState struct {
	Dir       string
	Addr      *string
	Conns     map[string]*Conn
	Clients   map[string]*server.Client
	Downloads map[string]*Download
	mx        sync.RWMutex
}

func New() *ServerState {
	return &ServerState{
		Dir:       "",
		Addr:      nil,
		Conns:     make(map[string]*Conn),
		Clients:   make(map[string]*server.Client),
		Downloads: make(map[string]*Download),
		mx:        sync.RWMutex{},
	}
}

func (s *ServerState) HandleEvent(event server.ServerEvent) {
	s.mx.Lock()
	defer s.mx.Unlock()
	switch event := event.(type) {
	case *server.EventAddrUpdated:
		s.Addr = &event.Addr
		s.Dir = event.Dir
	case *server.EventConnClose:
		s.HandleConnClose(event)
	case *server.EventDownloadProgress:
		s.HandleDownloadProgress(event)
	case *server.EventDownloadStart:
		s.HandleDownloadStart(event)
	default:
		slog.Warn("Unknown server event", "event", event)
	}

}

func (s *ServerState) RLock() {
	s.mx.RLock()
}

func (s *ServerState) RUnlock() {
	s.mx.RUnlock()
}

func (s *ServerState) HandleConnClose(event *server.EventConnClose) {
	if _, exists := s.Conns[event.ConnID]; exists {
		delete(s.Conns, event.ConnID)
	} else {
		slog.Warn("Connection close event for unknown connection", "conn_id", event.ConnID)
	}
}

func GetDownloadID(client *server.Client, fileDisplayPath string) string {
	return fmt.Sprintf("%s-%s", client.GetID(), fileDisplayPath)
}

func (s *ServerState) HandleDownloadStart(event *server.EventDownloadStart) {
	_, exists := s.Conns[event.ConnID]

	if !exists {
		clientID := event.Client.GetID()
		conn := &Conn{
			ID:        event.ConnID,
			ClientID:  &clientID,
			CreatedAt: event.Time,
			UpdatedAt: event.Time,
		}

		downloadID := GetDownloadID(event.Client, event.FileDisplayPath)

		download, exists := s.Downloads[downloadID]

		if !exists {
			download = &Download{
				ID:              downloadID,
				ClientID:        event.Client.GetID(),
				FileDisplayPath: event.FileDisplayPath,
				TotalSize:       event.TotalSize,
				StartedAt:       event.Time,
				Chunks:          make([]*DownloadChunk, 0),
			}
			s.Downloads[download.ID] = download
		}

		chunk := &DownloadChunk{
			ConnID:    event.ConnID,
			Start:     event.Range.Start,
			End:       event.Range.End,
			StartedAt: event.Time,
		}

		download.Chunks = append(download.Chunks, chunk)
		conn.DownloadChunk = chunk

		slog.Debug("Download after assign chunk", "chunk", download)

		s.Conns[event.ConnID] = conn
		s.Clients[clientID] = event.Client
		return
	}
}

func (s *ServerState) HandleDownloadProgress(event *server.EventDownloadProgress) {
	conn, exists := s.Conns[event.ConnID]

	if !exists {
		slog.Warn("Got file progress on closed connection", "connId", event.ConnID)
		return
	}

	if conn.DownloadChunk == nil {
		slog.Warn("Got download progress, but download chunk not found in connection", "connId", event.ConnID)
		return
	}

	conn.DownloadChunk.Sent += event.Sent
}
