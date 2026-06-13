package zip

import (
	az "archive/zip"
	"bytes"
	"io"
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
