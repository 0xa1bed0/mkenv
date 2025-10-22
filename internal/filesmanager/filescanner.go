package filesmanager

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

type KiB = uint8

// KMP table builder for efficient prefix search.
func kmpTable(pat []byte) []int {
	lps := make([]int, len(pat))
	length := 0
	for i := 1; i < len(pat); i++ {
		for length > 0 && pat[i] != pat[length] {
			length = lps[length-1]
		}
		if pat[i] == pat[length] {
			length++
			lps[i] = length
		}
	}
	return lps
}

// FileScanner allows streaming search and token extraction without loading
// the entire file into memory. It’s designed to be safe for arbitrarily large files.
//
// Example usage:
//
//	sc, _ := newFileScanner("go.mod", 32)
//	defer sc.Close()
//	sc.Find([]byte("go "))
//	version, _ := sc.ReadWhile(32, func(b byte) bool {
//	    return unicode.IsDigit(rune(b)) || b == '.'
//	})
//	fmt.Println(string(version))
type FileScanner interface {
	// Find consumes bytes until the prefix is found.
	// After returning nil, the scanner is positioned just *after* the prefix.
	Find(prefix []byte) error

	// ReadWhile collects bytes satisfying accept(b)==true, up to max bytes.
	// Stops at the first non-accepting byte (which is left unread).
	ReadWhile(max KiB, accept func(b byte) bool) ([]byte, error)

	// Close closes the underlying file.
	Close() error
}

// fileScanner implements FileScanner in a memory-safe, chunked manner.
type fileScanner struct {
	reader    *bufio.Reader
	closeFile func() error
	hasStash  bool // whether we have a stashed byte to unread
	stash     byte // single-byte stash
	readSize  int  // internal chunk size (bounds per-read allocation)
}

// NewFileScanner opens filePath for reading and returns a streaming Scanner.
// readSize controls the size of the internal read buffer in KiB.
func newFileScanner(filePath string, readSize KiB) (FileScanner, error) {
	if readSize == 0 {
		return nil, errors.New("readSize must be > 0")
	}
	bufLength := int(readSize) * 1024
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return &fileScanner{
		reader: bufio.NewReaderSize(file, bufLength),
		closeFile: func() error {
			if file == nil {
				return nil
			}
			err := file.Close()
			file = nil
			return err
		},
		readSize: bufLength,
	}, nil
}

// Close closes the file.
func (s *fileScanner) Close() error {
	return s.closeFile()
}

// Find locates the first occurrence of prefix and positions the reader
// right after it. Safe for arbitrarily large files.
//
// MEMORY SAFETY NOTES:
// - Uses KMP to avoid regex backtracking or large temporary buffers.
// - Reads through bufio.Reader, which bounds per-read allocations (≤ readSize).
func (s *fileScanner) Find(prefix []byte) error {
	if len(prefix) == 0 {
		return nil
	}

	// Precompute KMP prefix table
	lps := kmpTable(prefix)
	j := 0 // index into prefix

	for {
		b, err := s.readByte()
		if err != nil {
			if err == io.EOF {
				return errors.New("prefix not found")
			}
			return err
		}

		for j > 0 && b != prefix[j] {
			j = lps[j-1]
		}
		if b == prefix[j] {
			j++
			if j == len(prefix) {
				// found full prefix, positioned right after
				return nil
			}
		}
	}
}

// ReadWhile reads bytes while accept(b) returns true, up to 'max' total bytes.
// It stops at the first non-accepting byte (which is “unread” internally).
//
// MEMORY SAFETY NOTES:
// - Each byte read is bounds-checked by Go’s runtime.
// - Total accumulation is capped by 'max' to prevent unbounded growth.
// - bufio.Reader ensures per-read allocations stay within readSize.
func (s *fileScanner) ReadWhile(max KiB, accept func(b byte) bool) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("max must be > 0")
	}

	maxBytes := int(max) * 1024

	out := make([]byte, 0, min(maxBytes, 64)) // preallocate small buffer

	for {
		b, err := s.readByte()
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}

		if !accept(b) {
			s.unreadByte(b) // safe one-byte stash
			return out, nil
		}

		if len(out) == maxBytes {
			return nil, fmt.Errorf("token exceeds maximum size (%d KiB)", max)
		}
		out = append(out, b) // bounds-checked append
	}
}

// readByte returns the next byte from the stream or stash.
func (s *fileScanner) readByte() (byte, error) {
	if s.hasStash {
		s.hasStash = false
		return s.stash, nil
	}
	return s.reader.ReadByte() // bounded by readSize
}

// unreadByte stashes a single byte for the next read.
func (s *fileScanner) unreadByte(b byte) {
	s.hasStash = true
	s.stash = b
}
