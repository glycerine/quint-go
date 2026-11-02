package quint

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// Position matches Quint's zero-based source coordinates. Col and Index count
// UTF-16 code units, matching JavaScript strings and LSP's default encoding.
type Position struct {
	Line  int `json:"line"`
	Col   int `json:"col"`
	Index int `json:"index"`
}
type Location struct {
	Source string    `json:"source"`
	Start  Position  `json:"start"`
	End    *Position `json:"end,omitempty"`
}
type SourceMap map[ID]Location

// SourceFile stores immutable UTF-8 text and indexes all rune boundaries.
// NewSourceFile rejects invalid UTF-8 rather than silently replacing bytes.
type SourceFile struct {
	name      string
	text      string
	offsets   []int
	positions []Position
}

func NewSourceFile(name, text string) (*SourceFile, error) {
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("source %q is not valid UTF-8", name)
	}
	f := &SourceFile{name: name, text: text}
	p := Position{}
	for offset, r := range text {
		f.offsets = append(f.offsets, offset)
		f.positions = append(f.positions, p)
		units := 1
		if r > 0xffff {
			units = 2
		}
		p.Index += units
		if r == '\n' {
			p.Line++
			p.Col = 0
		} else {
			p.Col += units
		}
	}
	f.offsets = append(f.offsets, len(text))
	f.positions = append(f.positions, p)
	return f, nil
}
func (f *SourceFile) Name() string { return f.name }
func (f *SourceFile) Text() string { return f.text }
func (f *SourceFile) PositionAt(byteOffset int) (Position, error) {
	i := sort.SearchInts(f.offsets, byteOffset)
	if i == len(f.offsets) || f.offsets[i] != byteOffset {
		return Position{}, fmt.Errorf("byte offset %d is not a source boundary", byteOffset)
	}
	return f.positions[i], nil
}
func (f *SourceFile) ByteOffset(line, column int) (int, error) {
	i := sort.Search(len(f.positions), func(i int) bool { p := f.positions[i]; return p.Line > line || (p.Line == line && p.Col >= column) })
	if i == len(f.positions) || f.positions[i].Line != line || f.positions[i].Col != column {
		return 0, fmt.Errorf("invalid UTF-16 position %d:%d", line, column)
	}
	return f.offsets[i], nil
}
func (f *SourceFile) RuneOffset(byteOffset int) (int, error) {
	i := sort.SearchInts(f.offsets, byteOffset)
	if i == len(f.offsets) || f.offsets[i] != byteOffset {
		return 0, fmt.Errorf("invalid byte offset %d", byteOffset)
	}
	return i, nil
}

// Location returns a half-open span; parser adapters must convert inclusive
// token end coordinates explicitly rather than changing this convention.
func (f *SourceFile) Location(start, end int) (Location, error) {
	if start > end {
		return Location{}, fmt.Errorf("reversed source span")
	}
	a, err := f.PositionAt(start)
	if err != nil {
		return Location{}, err
	}
	b, err := f.PositionAt(end)
	if err != nil {
		return Location{}, err
	}
	return Location{Source: f.name, Start: a, End: &b}, nil
}
