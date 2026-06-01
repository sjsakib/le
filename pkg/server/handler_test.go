package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.sakib.dev/le/pkg/utils"
)

func TestHandlerDownloadsFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "example.txt")
	const body = "download contents"

	if err := os.WriteFile(filePath, []byte(body), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	dir, err := utils.ValidAbsDir(dir)
	if err != nil {
		t.Fatalf("resolve test dir: %v", err)
	}

	eventCh := make(chan ServerEvent, 10)
	server := httptest.NewServer(newHandler(dir, eventCh))
	defer server.Close()

	client := http.Client{Timeout: time.Second}
	resp, err := client.Get(server.URL + "/example.txt")
	if err != nil {
		t.Fatalf("download file: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}
