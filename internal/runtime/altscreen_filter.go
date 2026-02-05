package runtime

import "io"

// AltScreenFilter wraps a writer and filters out the "exit alternate screen"
// escape sequences (rmcup), preventing the terminal from restoring the
// pre-container screen when the container shell exits.
//
// This preserves the container's terminal output so users can see what
// happened after exiting.
type AltScreenFilter struct {
	w      io.Writer
	buf    []byte // partial escape sequence buffer
	maxSeq int    // max length of sequences we're looking for
}

// NewAltScreenFilter creates a writer that filters alternate screen exit sequences.
func NewAltScreenFilter(w io.Writer) *AltScreenFilter {
	return &AltScreenFilter{
		w:      w,
		buf:    make([]byte, 0, 16),
		maxSeq: 8, // longest sequence is \x1b[?1049l (8 bytes)
	}
}

func (f *AltScreenFilter) Write(p []byte) (n int, err error) {
	n = len(p) // we always "consume" all input from caller's perspective

	// Append to buffer for processing
	data := append(f.buf, p...)
	f.buf = f.buf[:0]

	// Process the data, filtering out rmcup sequences
	out := make([]byte, 0, len(data))
	i := 0

	for i < len(data) {
		// Check if we're at the start of an escape sequence
		if data[i] == 0x1b {
			// Check if we have enough data to identify the sequence
			remaining := data[i:]

			// Look for rmcup variants: \x1b[?1049l or \x1b[?47l
			if seq, match := matchRmcup(remaining); match {
				// Skip this sequence entirely
				i += len(seq)
				continue
			}

			// Check if this could be a partial rmcup sequence
			if couldBeRmcup(remaining) {
				// Buffer it for next write
				f.buf = append(f.buf, remaining...)
				break
			}
		}

		out = append(out, data[i])
		i++
	}

	if len(out) > 0 {
		_, err = f.w.Write(out)
	}
	return
}

// Flush writes any buffered partial sequence (on close).
func (f *AltScreenFilter) Flush() error {
	if len(f.buf) > 0 {
		_, err := f.w.Write(f.buf)
		f.buf = f.buf[:0]
		return err
	}
	return nil
}

// matchRmcup checks if data starts with an rmcup sequence.
// Returns the sequence and true if matched.
func matchRmcup(data []byte) ([]byte, bool) {
	// Common rmcup sequences that exit alternate screen:
	// \x1b[?1049l - most common (xterm)
	// \x1b[?47l   - older variant
	sequences := [][]byte{
		{0x1b, '[', '?', '1', '0', '4', '9', 'l'},
		{0x1b, '[', '?', '4', '7', 'l'},
	}

	for _, seq := range sequences {
		if len(data) >= len(seq) {
			match := true
			for j := 0; j < len(seq); j++ {
				if data[j] != seq[j] {
					match = false
					break
				}
			}
			if match {
				return seq, true
			}
		}
	}
	return nil, false
}

// couldBeRmcup checks if data could be the start of an rmcup sequence.
func couldBeRmcup(data []byte) bool {
	// Prefixes of rmcup sequences
	prefixes := [][]byte{
		{0x1b},
		{0x1b, '['},
		{0x1b, '[', '?'},
		{0x1b, '[', '?', '1'},
		{0x1b, '[', '?', '1', '0'},
		{0x1b, '[', '?', '1', '0', '4'},
		{0x1b, '[', '?', '1', '0', '4', '9'},
		{0x1b, '[', '?', '4'},
		{0x1b, '[', '?', '4', '7'},
	}

	for _, prefix := range prefixes {
		if len(data) <= len(prefix) {
			match := true
			for j := 0; j < len(data); j++ {
				if data[j] != prefix[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
