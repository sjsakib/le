package utils

import (
	"net"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Skipf("GetLocalIP requires UDP dial support: %v", err)
	}
	if ip == "" {
		t.Error("GetLocalIP returned empty IP string")
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		t.Errorf("GetLocalIP returned invalid IP %q", ip)
	}
}

func TestParseRangeHeader(t *testing.T) {
	tests := []struct {
		header    string
		size      int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{"bytes=0-99", 100, 0, 99, false},
		{"bytes=10-20", 50, 10, 20, false},
		{"bytes=10-", 50, 10, 49, false},  // end not specified, should be last byte
		{"bytes=-20", 100, 80, 99, false}, // last 20 bytes
		{"bytes=0-0", 1, 0, 0, false},
		{"bytes=0-", 100, 0, 99, false},
		{"bytes=0-150", 100, 0, 0, true},  // end out of bounds
		{"bytes=-10-20", 100, 0, 0, true}, // negative start
		{"", 100, 0, 0, false},            // empty header
	}
	for _, tt := range tests {
		start, end, err := ParseRangeHeader(tt.header, tt.size)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRangeHeader(%q, %d) error = %v, wantErr %v", tt.header, tt.size, err, tt.wantErr)
		}
		if !tt.wantErr {
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("ParseRangeHeader(%q, %d) = (%d, %d), want (%d, %d)", tt.header, tt.size, start, end, tt.wantStart, tt.wantEnd)
			}
		}
	}
}
