package zip

import (
	az "archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestZip(t *testing.T) {
	z := New("../../test-files", false)

	buf, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}

	_, err = az.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}

}

func TestCompressedZip(t *testing.T) {
	z := New("../../test-files", true)

	buf, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}

	r, err := az.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}

	if len(r.File) == 0 {
		t.Fatal("archive contains no files")
	}

	for _, f := range r.File {
		if f.Method != az.Deflate {
			t.Errorf("%s: expected Deflate method, got %d", f.Name, f.Method)
		}

		rc, err := f.Open()
		if err != nil {
			t.Fatalf("%s: open: %v", f.Name, err)
		}

		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: read: %v", f.Name, err)
		}

		want, err := os.ReadFile(filepath.Join("../../test-files", f.Name))
		if err != nil {
			t.Fatalf("%s: read original: %v", f.Name, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("%s: decompressed content does not match original", f.Name)
		}
	}
}
