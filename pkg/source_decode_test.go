package quint

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestNodeSourceDecodingFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/utf8.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Hex, Text string }
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		input, err := hex.DecodeString(tc.Hex)
		if err != nil {
			t.Fatal(err)
		}
		got := decodeSourceUTF8(input)
		if got != tc.Text || !utf8.ValidString(got) {
			t.Fatalf("%s: got %q, Node %q", tc.Hex, got, tc.Text)
		}
	}
}
func TestFileSourceMalformedUTF8(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "bad.qnt")
	if err := os.WriteFile(file, []byte{'a', 0xe2, 0x82, '\r', '\n', 0x80}, 0600); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	r := NewFileSourceResolver(sources, nil)
	got, err := r.Load(r.LookupPath(root, "bad.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\ufffd\n\ufffd" || sources[file] != got {
		t.Fatalf("loaded %q", got)
	}
	if _, err := NewSourceFile(file, got); err != nil {
		t.Fatal("decoded source must support position indexing", err)
	}
}
