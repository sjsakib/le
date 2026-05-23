package server

import (
	"fmt"
	"log/slog"
	"time"
)

type Client struct {
	IP        string
	Host      string
	UserAgent string
}

func (c *Client) GetID() string {
	return fmt.Sprintf("%s-%s", c.IP, c.UserAgent)
}

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
	Clients   map[string]*Client
	Downloads map[string]*Download
}

func (s *ServerState) HandleConnClose(event EventConnClose) {
	if _, exists := s.Conns[event.ConnID]; exists {
		delete(s.Conns, event.ConnID)
	} else {
		slog.Warn("Connection close event for unknown connection", "conn_id", event.ConnID)
	}
}

func GetDownloadID(client *Client, fileDisplayPath string) string {
	return fmt.Sprintf("%s-%s", client.GetID(), fileDisplayPath)
}

func (s *ServerState) HandleDownloadStart(event *EventDownloadStart) {
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

func (s *ServerState) HandleDownloadProgress(event *EventDownloadProgress) {
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
