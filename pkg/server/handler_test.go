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
