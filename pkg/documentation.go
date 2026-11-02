package quint

import (
	_ "embed"
	"encoding/json"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/glycerine/insdict"
)

// DocumentationEntry is a definition's source signature and optional Markdown
// documentation. Signatures use source annotations rather than inferred types.
type DocumentationEntry struct {
	Name          string  `json:"name"`
	Signature     string  `json:"signature"`
	Documentation *string `json:"documentation,omitempty"`
}

func documentationEntry(declaration Declaration) DocumentationEntry {
	entry := DocumentationEntry{Name: declaration.Name}
	if declaration.Doc != nil {
		doc := *declaration.Doc
		entry.Documentation = &doc
	}
	annotation := ""
	if declaration.TypeAnnotation != nil {
		annotation = ": " + TypeString(*declaration.TypeAnnotation)
	}
	switch declaration.Kind {
	case "def":
		entry.Signature = QualifierString(declaration.Qualifier) + " " + declaration.Name + annotation
	case "var", "const":
		entry.Signature = declaration.Kind + " " + declaration.Name + annotation
	case "assume":
		entry.Signature = "assume " + declaration.Name + " = " + ExpressionString(*declaration.Assumption)
	case "typedef":
		entry.Signature = "type " + declaration.Name
		if declaration.Type != nil {
			if len(declaration.Params) > 0 {
				entry.Signature += "[" + strings.Join(declaration.Params, ", ") + "]"
			}
			entry.Signature += " = " + TypeString(*declaration.Type)
		}
	}
	return entry
}

func isDocumentationDefinition(declaration Declaration) bool {
	switch declaration.Kind {
	case "def", "var", "const", "assume", "typedef":
		return true
	}
	return false
}

// ProduceDocs returns source-order documentation for every definition, including
// constants, variables, assumptions, and types. Imports/exports are excluded.
// A later duplicate name replaces its entry without moving the original key.
// The result and its optional documentation strings belong to the caller.
func ProduceDocs(module Module) *insdict.Dict[string, DocumentationEntry] {
	entries := insdict.NewDict[string, DocumentationEntry]()
	for _, declaration := range module.Declarations {
		if isDocumentationDefinition(declaration) {
			entries.Set(declaration.Name, documentationEntry(declaration))
		}
	}
	return entries
}

// ProduceDocsByID retains definitions with identical names under their source
// declaration IDs, matching the signature-help lookup used by editor clients.
func ProduceDocsByID(module Module) *insdict.Dict[ID, DocumentationEntry] {
	entries := insdict.NewDictFunc[ID, DocumentationEntry](func(id ID) uint64 {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(id.String()))
		return hash.Sum64()
	})
	for _, declaration := range module.Declarations {
		if isDocumentationDefinition(declaration) {
			entries.Set(declaration.ID, documentationEntry(declaration))
		}
	}
	return entries
}

// DocumentationMarkdown renders the reference CLI's entry format, including
// the empty documentation paragraph when no docstring was supplied.
func DocumentationMarkdown(entry DocumentationEntry) string {
	doc := ""
	if entry.Documentation != nil {
		doc = *entry.Documentation
	}
	return "## " + entry.Name + "\n\nSignature: `" + entry.Signature + "`\n\n" + doc
}

//go:embed documentation_assets/builtin.json
var builtinDocumentationJSON []byte

var builtinDocumentation struct {
	sync.Once
	entries *insdict.Dict[string, DocumentationEntry]
	err     error
}

// BuiltinDocs returns generated documentation from the upstream builtin
// specification. Decoding is cached; each call returns owned entries and strings.
// It does not read files relative to the executable or the working directory.
func BuiltinDocs() (*insdict.Dict[string, DocumentationEntry], error) {
	builtinDocumentation.Do(func() {
		var entries []DocumentationEntry
		if err := json.Unmarshal(builtinDocumentationJSON, &entries); err != nil {
			builtinDocumentation.err = err
			return
		}
		builtinDocumentation.entries = insdict.NewDict[string, DocumentationEntry]()
		for _, entry := range entries {
			builtinDocumentation.entries.Set(entry.Name, entry)
		}
	})
	if builtinDocumentation.err != nil {
		return nil, builtinDocumentation.err
	}
	entries := insdict.NewDict[string, DocumentationEntry]()
	for name, entry := range builtinDocumentation.entries.All() {
		if entry.Documentation != nil {
			doc := *entry.Documentation
			entry.Documentation = &doc
		}
		entries.Set(name, entry)
	}
	return entries, nil
}
