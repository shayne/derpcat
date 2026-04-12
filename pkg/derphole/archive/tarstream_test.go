package archive

import (
	"archive/tar"
	"bytes"
	"testing"
)

func TestExtractTarRejectsParentTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0600, Size: int64(len("x"))}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := ExtractTar(bytes.NewReader(buf.Bytes()), t.TempDir(), "photos"); err == nil {
		t.Fatal("ExtractTar() error = nil, want traversal rejection")
	}
}
