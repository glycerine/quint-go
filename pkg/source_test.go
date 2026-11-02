package quint

import "testing"

func TestSourceCoordinateConversions(t *testing.T) {
	f, err := NewSourceFile("unicode.qnt", "aé😀\r\nb\n")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name() != "unicode.qnt" || f.Text() != "aé😀\r\nb\n" {
		t.Fatal("source identity changed")
	}
	cases := []struct {
		offset int
		p      Position
	}{{0, Position{0, 0, 0}}, {1, Position{0, 1, 1}}, {3, Position{0, 2, 2}}, {7, Position{0, 4, 4}}, {8, Position{0, 5, 5}}, {9, Position{1, 0, 6}}, {10, Position{1, 1, 7}}, {11, Position{2, 0, 8}}}
	for runeOffset, c := range cases {
		p, err := f.PositionAt(c.offset)
		if err != nil || p != c.p {
			t.Fatalf("offset %d: %v %v", c.offset, p, err)
		}
		offset, err := f.ByteOffset(p.Line, p.Col)
		if err != nil || offset != c.offset {
			t.Fatalf("reverse %v: %d %v", p, offset, err)
		}
		r, err := f.RuneOffset(c.offset)
		if err != nil || r != runeOffset {
			t.Fatalf("rune %d: %d %v", c.offset, r, err)
		}
	}
	for _, offset := range []int{-1, 2, 4, 5, 6, 12} {
		if _, err := f.PositionAt(offset); err == nil {
			t.Errorf("accepted byte %d", offset)
		}
		if _, err := f.RuneOffset(offset); err == nil {
			t.Errorf("accepted rune byte %d", offset)
		}
	}
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {0, 3}, {1, 2}, {3, 0}} {
		if _, err := f.ByteOffset(p[0], p[1]); err == nil {
			t.Errorf("accepted position %v", p)
		}
	}
	loc, err := f.Location(1, 7)
	if err != nil || loc.Start != cases[1].p || *loc.End != cases[3].p || loc.Source != f.Name() {
		t.Fatalf("span: %v %v", loc, err)
	}
	for _, span := range [][2]int{{7, 1}, {2, 7}, {1, 6}} {
		if _, err := f.Location(span[0], span[1]); err == nil {
			t.Errorf("accepted span %v", span)
		}
	}
}
func TestEmptyAndInvalidSources(t *testing.T) {
	if _, err := NewSourceFile("bad", string([]byte{255})); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
	f, err := NewSourceFile("empty", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.PositionAt(0)
	if err != nil || p != (Position{}) {
		t.Fatalf("empty position: %v %v", p, err)
	}
	if _, err := f.Location(0, 0); err != nil {
		t.Fatal(err)
	}
}
