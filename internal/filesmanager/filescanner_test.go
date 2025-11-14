package filesmanager

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode"
)

type nopCloser struct {
	*bytes.Reader
}

func (n *nopCloser) Close() error { return nil }

func newInMemoryScanner(t *testing.T, data string, readSize KiB) *fileScanner {
	t.Helper()
	if readSize == 0 {
		t.Fatalf("readSize must be > 0 for helper")
	}
	reader := &nopCloser{Reader: bytes.NewReader([]byte(data))}
	return &fileScanner{
		reader:    bufio.NewReaderSize(reader, int(readSize)*1024),
		closeFile: func() error { return reader.Close() },
		readSize:  int(readSize) * 1024,
	}
}

func TestNewFileScannerZeroSize(t *testing.T) {
	t.Parallel()

	if _, err := newFileScanner("ignored", 0); err == nil {
		t.Fatal("expected error when readSize is zero")
	}
}

func TestFileScannerFindAndReadWhile(t *testing.T) {
	t.Parallel()

	sc := newInMemoryScanner(t, "package main\nversion: 1.2.3\n", 1)

	if err := sc.Find([]byte("version: ")); err != nil {
		t.Fatalf("Find failed: %v", err)
	}

	got, err := sc.ReadWhile(1, func(b byte) bool {
		return unicode.IsDigit(rune(b)) || b == '.'
	})
	if err != nil {
		t.Fatalf("ReadWhile failed: %v", err)
	}
	if string(got) != "1.2.3" {
		t.Fatalf("expected version token, got %q", got)
	}

	next, err := sc.ReadWhile(1, func(b byte) bool { return b == '\n' })
	if err != nil {
		t.Fatalf("ReadWhile for newline failed: %v", err)
	}
	if string(next) != "\n" {
		t.Fatalf("expected newline, got %q", next)
	}
}

func TestFileScannerFindMissingPrefix(t *testing.T) {
	t.Parallel()

	sc := newInMemoryScanner(t, "abc", 1)

	if err := sc.Find([]byte("zzz")); err == nil {
		t.Fatal("expected error when prefix is missing")
	}
}

func TestFileScannerReadWhileLimitExceeded(t *testing.T) {
	t.Parallel()

	sc := newInMemoryScanner(t, strings.Repeat("a", 2048), 1)
	if err := sc.Find([]byte("")); err != nil {
		t.Fatalf("Find with empty prefix failed: %v", err)
	}

	if _, err := sc.ReadWhile(1, func(byte) bool { return true }); err == nil {
		t.Fatal("expected error when read exceeds limit")
	}
}

func TestFileScannerReadWhileZeroLimit(t *testing.T) {
	t.Parallel()

	sc := newInMemoryScanner(t, "abc", 1)
	if _, err := sc.ReadWhile(0, func(byte) bool { return true }); err == nil {
		t.Fatal("expected error when max is zero")
	}
}

func TestFileScannerCloseIdempotent(t *testing.T) {
	t.Parallel()

	sc := newInMemoryScanner(t, "data", 1)
	if err := sc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := sc.Close(); err != nil {
		t.Fatalf("Close should be idempotent, got %v", err)
	}
}

func TestFileScannerFindPropagatesIOErrors(t *testing.T) {
	t.Parallel()

	broken := &errorReader{err: errors.New("boom")}
	sc := &fileScanner{
		reader:    bufio.NewReaderSize(broken, 1024),
		closeFile: func() error { return broken.Close() },
		readSize:  1024,
	}

	if err := sc.Find([]byte("anything")); !errors.Is(err, broken.err) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func (e *errorReader) Close() error {
	return nil
}
