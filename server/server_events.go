package server

import "time"

type ServerEventName string

const (
	EvNameDownloadStart    ServerEventName = "download_start"
	EvNameDownloadProgress ServerEventName = "download_progress"
	EvNameConnClose        ServerEventName = "conn_close"
	EvNameAddrUpdated      ServerEventName = "addr_updated"
)

type Range struct {
	Start int64
	End   int64
}

type EventDownloadStart struct {
	ConnID          string
	Client          *Client
	FileDisplayPath string
	TotalSize       int64
	Range           Range
	Time            time.Time
}

type EventConnClose struct {
	ConnID string
	Time   time.Time
}

type EventDownloadProgress struct {
	ConnID string
	Sent   int64
	Time   time.Time
}

type ServerEvent interface {
	EventName() ServerEventName
}

func (e EventConnClose) EventName() ServerEventName {
	return EvNameConnClose
}
func (e EventDownloadProgress) EventName() ServerEventName {
	return EvNameDownloadProgress
}
func (e EventDownloadStart) EventName() ServerEventName {
	return EvNameDownloadStart
}
