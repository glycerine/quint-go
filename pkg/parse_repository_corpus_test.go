package quint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This separately checks actual TypeScript output. The prediction-strategy
// corpus tests compare two Go parsers and intentionally remain independent.
func TestUpstreamRepositoryParserCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for the complete upstream source corpus")
	}
	data, err := os.ReadFile("testdata/parser/parser_repository_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct{ Name, SourceSHA256, SnapshotSHA256 string }
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 197 {
		t.Fatal("repository parser inventory", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", fixture.Name))
			if err != nil {
				t.Fatal(err)
			}
			if got := parserCorpusHash(string(source)); got != fixture.SourceSHA256 {
				t.Fatal("source changed; regenerate the reference oracle", got)
			}
			result, err := ParseModulesRecovering(fixture.Name, string(source))
			if err != nil {
				t.Fatal(err)
			}
			snapshot := struct {
				Modules         []Module          `json:"modules"`
				Diagnostics     []Diagnostic      `json:"diagnostics"`
				UpstreamSources UpstreamSourceMap `json:"upstreamSources"`
				NextID          ID                `json:"nextID"`
			}{result.Modules, result.Diagnostics, result.UpstreamSources, result.IDs.Next()}
			wire, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			var value any
			decoder := json.NewDecoder(bytes.NewReader(wire))
			decoder.UseNumber()
			if err = decoder.Decode(&value); err != nil {
				t.Fatal(err)
			}
			var canonical strings.Builder
			parserCorpusCanonical(&canonical, value)
			if got := parserCorpusHash(canonical.String()); got != fixture.SnapshotSHA256 {
				t.Fatalf("raw IR/diagnostic/source-map/next-ID hash got%s want%s", got, fixture.SnapshotSHA256)
			}
		})
	}
}

func parserCorpusHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func parserCorpusCanonical(out *strings.Builder, value any) {
	switch value := value.(type) {
	case nil:
		out.WriteString("n;")
	case bool:
		if value {
			out.WriteString("b1;")
		} else {
			out.WriteString("b0;")
		}
	case string:
		out.WriteByte('s')
		out.WriteString(strconv.Itoa(len(value)))
		out.WriteByte(':')
		out.WriteString(value)
	case json.Number:
		out.WriteByte('d')
		out.WriteString(string(value))
		out.WriteByte(';')
	case []any:
		out.WriteByte('[')
		for _, child := range value {
			parserCorpusCanonical(out, child)
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for _, key := range keys {
			parserCorpusCanonical(out, key)
			parserCorpusCanonical(out, value[key])
		}
		out.WriteByte('}')
	default:
		panic(fmt.Sprintf("unexpected parser corpus value %T", value))
	}
}
