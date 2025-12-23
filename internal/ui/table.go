package ui

import (
	"io"
	"strings"
	"unicode/utf8"
)

type Align int

const (
	AlignLeft Align = iota
	AlignRight
)

type TruncateMode int

const (
	TruncateNone   TruncateMode = iota
	TruncateEnd                 // "hello wo…"
	TruncateMiddle              // "hel…rld"
	TruncateStart               // "…o world"
)

// Column configures a column in the table.
type Column struct {
	Header       string
	Align        Align        // default: AlignLeft
	MaxWidth     int          // 0 = unlimited
	Truncate     TruncateMode // default: TruncateEnd when MaxWidth > 0
	Ellipsis     string       // default: "…"
	PaddingRight int          // default: 2 spaces
}

type Table struct {
	columns []Column
	rows    [][]string

	// style options (still no deps, simple)
	ShowHeader    bool
	ShowSeparator bool
}

func NewTable(columns ...Column) *Table {
	// apply defaults
	for i := range columns {
		if columns[i].PaddingRight == 0 {
			columns[i].PaddingRight = 2
		}
		if columns[i].Ellipsis == "" {
			columns[i].Ellipsis = "…"
		}
		if columns[i].Align == 0 {
			columns[i].Align = AlignLeft
		}
		if columns[i].MaxWidth > 0 && columns[i].Truncate == TruncateNone {
			// keep as none explicitly; caller choice
		} else if columns[i].MaxWidth > 0 && columns[i].Truncate == 0 {
			// zero value is TruncateNone; default to end truncation when MaxWidth is set
			columns[i].Truncate = TruncateEnd
		}
	}

	return &Table{
		columns:       columns,
		ShowHeader:    true,
		ShowSeparator: true,
	}
}

func (t *Table) AddRow(cells ...string) {
	// normalize row length
	row := make([]string, len(t.columns))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
	}
	t.rows = append(t.rows, row)
}

func (t *Table) Render(w io.Writer) error {
	if len(t.columns) == 0 {
		return nil
	}

	widths := t.computeWidths()

	// header
	if t.ShowHeader {
		headerCells := make([]string, len(t.columns))
		for i, c := range t.columns {
			headerCells[i] = c.Header
		}
		if err := t.writeRow(w, headerCells, widths); err != nil {
			return err
		}
		if t.ShowSeparator {
			if err := t.writeSeparator(w, widths); err != nil {
				return err
			}
		}
	}

	// rows
	for _, row := range t.rows {
		if err := t.writeRow(w, row, widths); err != nil {
			return err
		}
	}
	return nil
}

func (t *Table) computeWidths() []int {
	widths := make([]int, len(t.columns))

	// include headers
	for i, col := range t.columns {
		w := runeLen(col.Header)
		if col.MaxWidth > 0 && w > col.MaxWidth {
			w = col.MaxWidth
		}
		widths[i] = max(widths[i], w)
	}

	// include rows
	for _, row := range t.rows {
		for i, cell := range row {
			col := t.columns[i]
			cell = t.applyTruncation(col, cell)
			w := runeLen(cell)
			if col.MaxWidth > 0 && w > col.MaxWidth {
				w = col.MaxWidth
			}
			widths[i] = max(widths[i], w)
		}
	}

	// ensure widths respect MaxWidth
	for i, col := range t.columns {
		if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
			widths[i] = col.MaxWidth
		}
	}

	return widths
}

func (t *Table) writeRow(w io.Writer, cells []string, widths []int) error {
	for i, raw := range cells {
		col := t.columns[i]
		cell := t.applyTruncation(col, raw)

		// pad/align to computed width
		out := align(cell, widths[i], col.Align)

		// add spacing between columns
		if col.PaddingRight > 0 {
			out += strings.Repeat(" ", col.PaddingRight)
		}

		if _, err := io.WriteString(w, out); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (t *Table) writeSeparator(w io.Writer, widths []int) error {
	for i, col := range t.columns {
		dashes := strings.Repeat("-", widths[i])
		if col.PaddingRight > 0 {
			dashes += strings.Repeat(" ", col.PaddingRight)
		}
		if _, err := io.WriteString(w, dashes); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (t *Table) applyTruncation(col Column, s string) string {
	if col.MaxWidth <= 0 || col.Truncate == TruncateNone {
		return s
	}
	if runeLen(s) <= col.MaxWidth {
		return s
	}
	ell := col.Ellipsis
	if ell == "" {
		ell = "…"
	}

	// If max width is too small to fit ellipsis + content, hard cut.
	if col.MaxWidth <= runeLen(ell) {
		return takeRunes(s, col.MaxWidth)
	}

	avail := col.MaxWidth - runeLen(ell)

	switch col.Truncate {
	case TruncateStart:
		// "…suffix"
		suffix := takeRunesFromEnd(s, avail)
		return ell + suffix
	case TruncateMiddle:
		// "prefix…suffix"
		left := avail / 2
		right := avail - left
		return takeRunes(s, left) + ell + takeRunesFromEnd(s, right)
	case TruncateEnd:
		fallthrough
	default:
		// "prefix…"
		return takeRunes(s, avail) + ell
	}
}

func align(s string, width int, a Align) string {
	l := runeLen(s)
	if l >= width {
		return s
	}
	pad := strings.Repeat(" ", width-l)
	if a == AlignRight {
		return pad + s
	}
	return s + pad
}

func runeLen(s string) int {
	// rune count (Unicode-safe), not terminal cell width (wcwidth)
	return utf8.RuneCountInString(s)
}

func takeRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	// slice by scanning runes
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

func takeRunesFromEnd(s string, n int) string {
	if n <= 0 {
		return ""
	}
	total := utf8.RuneCountInString(s)
	if total <= n {
		return s
	}
	// need last n runes => skip total-n runes from start
	skip := total - n
	i := 0
	for pos := range s {
		if i == skip {
			return s[pos:]
		}
		i++
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
