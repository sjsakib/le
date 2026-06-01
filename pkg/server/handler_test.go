package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	handler := newHandler(dir, eventCh)

	req := httptest.NewRequest(http.MethodGet, "/example.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	if rr.Body.String() != body {
		t.Fatalf("body = %q, want %q", rr.Body.String(), body)
	}
}

func TestServeDirectoryDefaultsToLatestFirst(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "old.txt", 2048, oldTime)
	writeTestFile(t, dir, "new.txt", 2048, newTime)

	body := renderDirectory(t, dir, "")

	assertFileOrder(t, body, "new.txt", "old.txt")
}

func TestServeDirectorySortsFilesBySize(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "small.txt", 512, modTime)
	writeTestFile(t, dir, "large.txt", 4096, modTime)
	writeTestFile(t, dir, "medium.txt", 2048, modTime)

	body := renderDirectory(t, dir, "sort=size")

	assertFileOrder(t, body, "large.txt", "medium.txt", "small.txt")
	if !strings.Contains(body, `<option value="size" selected>Largest first</option>`) {
		t.Fatalf("expected size sort option to be selected, body:\n%s", body)
	}
}

func TestServeDirectorySearchFiltersFileNames(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "Report.pdf", 2048, modTime)
	writeTestFile(t, dir, "notes.txt", 2048, modTime)

	body := renderDirectory(t, dir, "q=REPORT")

	if !strings.Contains(body, fileNameSpan("Report.pdf")) {
		t.Fatalf("expected search result to include Report.pdf, body:\n%s", body)
	}
	if strings.Contains(body, fileNameSpan("notes.txt")) {
		t.Fatalf("expected search result to exclude notes.txt, body:\n%s", body)
	}
	if !strings.Contains(body, `value="REPORT"`) {
		t.Fatalf("expected search query to be preserved in input, body:\n%s", body)
	}
}

func TestServeDirectoryShowsSmallFileSizeInBytes(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "small.txt", 512, modTime)

	body := renderDirectory(t, dir, "")

	if !strings.Contains(body, `<span class="file-size">512 B</span>`) {
		t.Fatalf("expected small file size to render in bytes, body:\n%s", body)
	}
}

func renderDirectory(t *testing.T, dir string, rawQuery string) string {
	t.Helper()

	target := "/"
	if rawQuery != "" {
		target += "?" + rawQuery
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	h := &handler{}
	h.serveDirectory(rr, req, dir)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body:\n%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	return rr.Body.String()
}

func writeTestFile(t *testing.T, dir, name string, size int, modTime time.Time) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set mod time for %s: %v", name, err)
	}
}

func assertFileOrder(t *testing.T, body string, names ...string) {
	t.Helper()

	lastIndex := -1
	for _, name := range names {
		index := strings.Index(body, fileNameSpan(name))
		if index == -1 {
			t.Fatalf("expected %s in body:\n%s", name, body)
		}
		if index <= lastIndex {
			t.Fatalf("expected files in order %v, body:\n%s", names, body)
		}
		lastIndex = index
	}
}

func fileNameSpan(name string) string {
	return `<span class="file-name">` + name + `</span>`
}
