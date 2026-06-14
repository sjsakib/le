package server

import (
	az "archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.sakib.dev/le/pkg/cfg"
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
	handler := newTestHandler(t, dir, eventCh)

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

func TestHandlerReturnsContentLengthForFileDownload(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "example.txt")
	body := strings.Repeat("download contents\n", 256)

	if err := os.WriteFile(filePath, []byte(body), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	dir, err := utils.ValidAbsDir(dir)
	if err != nil {
		t.Fatalf("resolve test dir: %v", err)
	}

	eventCh := make(chan ServerEvent, 10)
	handler := newTestHandler(t, dir, eventCh)

	req := httptest.NewRequest(http.MethodGet, "/example.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	if rr.Body.String() != body {
		t.Fatalf("body = %q, want %q", rr.Body.String(), body)
	}

	expectedContentLength := fmt.Sprintf("%d", len(body))
	if got := rr.Header().Get("Content-Length"); got != expectedContentLength {
		t.Fatalf("Content-Length = %q, want %q", got, expectedContentLength)
	}
}

func TestHandlerHandlesHeadRequestForFileDownload(t *testing.T) {
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
	handler := newTestHandler(t, dir, eventCh)

	req := httptest.NewRequest(http.MethodHead, "/example.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	if got := rr.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty body", got)
	}

	if got, want := rr.Header().Get("Content-Length"), fmt.Sprintf("%d", len(body)); got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}

	if got, want := rr.Header().Get("Accept-Ranges"), "bytes"; got != want {
		t.Fatalf("Accept-Ranges = %q, want %q", got, want)
	}
}

func TestHandlerDownloadsZipArchiveWithRange(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	writeTestFile(t, dir, "alpha.txt", 2048, time.Unix(1700000000, 0))
	writeTestFile(t, dir, "nested/beta.txt", 4096, time.Unix(1700000060, 0))

	dir, err := utils.ValidAbsDir(dir)
	if err != nil {
		t.Fatalf("resolve test dir: %v", err)
	}

	eventCh := make(chan ServerEvent, 20)
	done := make(chan struct{})
	go drainEvents(eventCh, done)
	defer close(done)

	handler := newTestHandler(t, dir, eventCh)

	fullReq := httptest.NewRequest(http.MethodGet, "/?archive=true&compressed=false", nil)
	fullRR := httptest.NewRecorder()
	handler.ServeHTTP(fullRR, fullReq)
	fullBody := fullRR.Body.Bytes()

	if fullRR.Code != http.StatusOK {
		t.Fatalf("full archive status = %d, want %d", fullRR.Code, http.StatusOK)
	}

	if _, err := az.NewReader(bytes.NewReader(fullBody), int64(len(fullBody))); err != nil {
		t.Fatalf("full archive is not a valid zip: %v", err)
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/?archive=true&compressed=false", nil)
	rangeReq.Header.Set("Range", "bytes=10-99")
	rangeRR := httptest.NewRecorder()
	handler.ServeHTTP(rangeRR, rangeReq)
	rangeBody := rangeRR.Body.Bytes()

	if rangeRR.Code != http.StatusPartialContent {
		t.Fatalf("range archive status = %d, want %d", rangeRR.Code, http.StatusPartialContent)
	}

	if got, want := rangeRR.Header().Get("Accept-Ranges"), "bytes"; got != want {
		t.Fatalf("Accept-Ranges = %q, want %q", got, want)
	}

	if got, want := rangeRR.Header().Get("Content-Range"), fmt.Sprintf("bytes 10-99/%d", len(fullBody)); got != want {
		t.Fatalf("Content-Range = %q, want %q", got, want)
	}

	if got, want := rangeRR.Header().Get("Content-Length"), "90"; got != want {
		t.Fatalf("Content-Length = %q, want %q", got, want)
	}

	if !bytes.Equal(rangeBody, fullBody[10:100]) {
		t.Fatalf("range body does not match full archive bytes 10-99")
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
	if !strings.Contains(body, `<a class="file-header-modified active" href="?order=asc&amp;sort=date">Modified<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-desc"`) {
		t.Fatalf("expected date header to be active with descending indicator, body:\n%s", body)
	}
}

func TestServeDirectorySortsFilesByDateAscending(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "old.txt", 2048, oldTime)
	writeTestFile(t, dir, "new.txt", 2048, newTime)

	body := renderDirectory(t, dir, "sort=date&order=asc")

	assertFileOrder(t, body, "old.txt", "new.txt")
	if !strings.Contains(body, `<a class="file-header-modified active" href="?order=desc&amp;sort=date">Modified<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-asc"`) {
		t.Fatalf("expected date header to be active with ascending indicator, body:\n%s", body)
	}
}

func TestServeDirectorySortsFilesBySize(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "small.txt", 512, modTime)
	writeTestFile(t, dir, "large.txt", 4096, modTime)
	writeTestFile(t, dir, "medium.txt", 2048, modTime)

	body := renderDirectory(t, dir, "sort=size")

	assertFileOrder(t, body, "large.txt", "medium.txt", "small.txt")
	if !strings.Contains(body, `<a class="file-header-size active" href="?order=asc&amp;sort=size">Size<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-desc"`) {
		t.Fatalf("expected size header to be active with descending indicator, body:\n%s", body)
	}
}

func TestServeDirectorySortsFilesBySizeAscending(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "small.txt", 512, modTime)
	writeTestFile(t, dir, "large.txt", 4096, modTime)
	writeTestFile(t, dir, "medium.txt", 2048, modTime)

	body := renderDirectory(t, dir, "sort=size&order=asc")

	assertFileOrder(t, body, "small.txt", "medium.txt", "large.txt")
	if !strings.Contains(body, `<a class="file-header-size active" href="?order=desc&amp;sort=size">Size<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-asc"`) {
		t.Fatalf("expected size header to be active with ascending indicator, body:\n%s", body)
	}
}

func TestServeDirectorySortsFilesByName(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "beta.txt", 2048, modTime)
	writeTestFile(t, dir, "alpha.txt", 2048, modTime)
	writeTestFile(t, dir, "gamma.txt", 2048, modTime)

	body := renderDirectory(t, dir, "sort=name")

	assertFileOrder(t, body, "alpha.txt", "beta.txt", "gamma.txt")
	if !strings.Contains(body, `<a class="file-header-name active" href="?order=desc&amp;sort=name">Name<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-asc"`) {
		t.Fatalf("expected name header to be active with ascending indicator, body:\n%s", body)
	}
}

func TestServeDirectorySortsFilesByNameDescending(t *testing.T) {
	dir := t.TempDir()
	modTime := time.Now().Add(-1 * time.Hour)

	writeTestFile(t, dir, "beta.txt", 2048, modTime)
	writeTestFile(t, dir, "alpha.txt", 2048, modTime)
	writeTestFile(t, dir, "gamma.txt", 2048, modTime)

	body := renderDirectory(t, dir, "sort=name&order=desc")

	assertFileOrder(t, body, "gamma.txt", "beta.txt", "alpha.txt")
	if !strings.Contains(body, `<a class="file-header-name active" href="?order=asc&amp;sort=name">Name<span class="sort-indicator" aria-hidden="true"><svg class="sort-icon sort-icon-desc"`) {
		t.Fatalf("expected name header to be active with descending indicator, body:\n%s", body)
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
	if !strings.Contains(body, `href="?order=desc&amp;q=REPORT&amp;sort=size"`) {
		t.Fatalf("expected size sort link to preserve search query, body:\n%s", body)
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

func newTestHandler(t *testing.T, dir string, eventCh chan<- ServerEvent) http.Handler {
	t.Helper()

	handler, err := newHandler(&cfg.Config{
		Dir:            dir,
		StaticSiteMode: cfg.StaticSiteModeOff,
	}, eventCh)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	return handler
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

func drainEvents(ch <-chan ServerEvent, done <-chan struct{}) {
	for {
		select {
		case <-ch:
		case <-done:
			return
		}
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
