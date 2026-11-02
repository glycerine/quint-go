package lsp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type Document struct {
	URI        string
	LanguageID string
	Version    int
	Text       string
}

// Snapshot belongs to one document version. Analysis and semantic requests
// share it read-only; the server replaces snapshots instead of mutating them.
type Snapshot struct {
	URI         string
	Version     int
	Text        string
	Frontend    quint.FrontendResult
	Analysis    quint.AnalysisResult
	Sources     *insdict.Dict[string, string]
	Diagnostics []Diagnostic
}

func sourceForURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" {
		return "", fmt.Errorf("expected an absolute document URI")
	}
	if u.Scheme != "file" {
		return uri, nil
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("remote file URIs are unsupported")
	}
	if u.Path == "" {
		return "", fmt.Errorf("empty file URI")
	}
	return filepath.Clean(filepath.FromSlash(u.Path)), nil
}
func uriForSource(source string) string {
	if u, err := url.Parse(source); err == nil && u.Scheme != "" {
		return source
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		absolute = source
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
}

type overlayResolver struct {
	ctx    context.Context
	files  *insdict.Dict[string, string]
	loaded *insdict.Dict[string, string]
	disk   *quint.FileSourceResolver
}

func (r *overlayResolver) LookupPath(stem, importPath string) quint.SourceLookupPath {
	return r.disk.LookupPath(stem, importPath)
}
func (r *overlayResolver) StemPath(path quint.SourceLookupPath) string { return r.disk.StemPath(path) }
func (r *overlayResolver) Load(path quint.SourceLookupPath) (string, error) {
	if err := r.ctx.Err(); err != nil {
		return "", err
	}
	text, ok := r.files.Get2(path.NormalizedPath)
	var err error
	if !ok {
		text, err = r.disk.Load(path)
	}
	if err == nil {
		r.loaded.Set(path.SourceName, text)
	}
	return text, err
}

func analyzeDocument(ctx context.Context, document Document, overlays *insdict.Dict[string, string]) (*Snapshot, error) {
	snapshot := &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text, Sources: insdict.NewDict[string, string](), Diagnostics: []Diagnostic{}}
	// Keep current open-buffer imports even if an incomplete main document
	// prevents ordinary parsing from reaching its imports. Completion recovery
	// can then analyze the same unsaved source graph without stale snapshots.
	for name, text := range overlays.All() {
		snapshot.Sources.Set(name, text)
	}
	path, err := sourceForURI(document.URI)
	if err != nil {
		return snapshot, err
	}
	resolver := &overlayResolver{ctx: ctx, files: overlays, loaded: snapshot.Sources, disk: quint.NewFileSourceResolver(nil, nil)}
	main := quint.SourceLookupPath{NormalizedPath: path, SourceName: path}
	snapshot.Frontend, err = quint.ParseProject(resolver, main)
	if err == nil && ctx.Err() == nil && snapshot.Frontend.Table != nil && len(snapshot.Frontend.Diagnostics) == 0 {
		snapshot.Analysis, err = quint.AnalyzeModules(snapshot.Frontend.Modules, snapshot.Frontend.Table)
	}
	if ctx.Err() != nil {
		return snapshot, ctx.Err()
	}
	for _, diagnostic := range snapshot.Frontend.Diagnostics {
		snapshot.Diagnostics = append(snapshot.Diagnostics, convertDiagnostic(path, snapshot.Frontend.Sources, diagnostic))
	}
	for _, diagnostic := range snapshot.Analysis.Diagnostics {
		snapshot.Diagnostics = append(snapshot.Diagnostics, convertDiagnostic(path, snapshot.Frontend.Sources, diagnostic))
	}
	if err != nil {
		var syntax *quint.SyntaxErrors
		var diagnostic quint.Diagnostic
		switch {
		case errors.As(err, &syntax):
			for _, issue := range syntax.Issues {
				position := Position{Line: issue.Position.Line, Character: issue.Position.Col}
				item := Diagnostic{Range: Range{Start: position, End: position}, Severity: 1, Code: issue.Code, Source: "quint", Message: issue.Message}
				if syntax.Source != "" && syntax.Source != path {
					item.Range = Range{}
					item.RelatedInformation = []DiagnosticRelatedInformation{{Location: Location{URI: uriForSource(syntax.Source), Range: Range{Start: position, End: position}}, Message: issue.Message}}
				}
				snapshot.Diagnostics = append(snapshot.Diagnostics, item)
			}
		case errors.As(err, &diagnostic):
			snapshot.Diagnostics = append(snapshot.Diagnostics, convertDiagnostic(path, snapshot.Frontend.Sources, diagnostic))
		default:
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Severity: 1, Source: "quint", Message: err.Error()})
		}
	}
	return snapshot, nil
}
func convertDiagnostic(path string, sources quint.SourceMap, diagnostic quint.Diagnostic) Diagnostic {
	out := Diagnostic{Severity: 1, Code: diagnostic.Code, Source: "quint", Message: diagnostic.Message}
	resolved := quint.ResolveDiagnostic(sources, diagnostic)
	for _, location := range append(resolved.Locations, resolved.TraceLocations...) {
		span := rangeForLocation(location)
		if location.Source == path && out.Range == (Range{}) {
			out.Range = span
			continue
		}
		out.RelatedInformation = append(out.RelatedInformation, DiagnosticRelatedInformation{Location: Location{URI: uriForSource(location.Source), Range: span}, Message: diagnostic.Message})
	}
	return out
}
func rangeForLocation(location quint.Location) Range {
	start := Position{Line: location.Start.Line, Character: location.Start.Col}
	end := start
	if location.End != nil {
		end = Position{Line: location.End.Line, Character: location.End.Col}
	}
	return Range{Start: start, End: end}
}

type contentChange struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

func applyChanges(text string, changes []contentChange) (string, error) {
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
			continue
		}
		file, err := quint.NewSourceFile("<document>", text)
		if err != nil {
			return "", err
		}
		start, err := file.ByteOffset(change.Range.Start.Line, change.Range.Start.Character)
		if err != nil {
			return "", err
		}
		end, err := file.ByteOffset(change.Range.End.Line, change.Range.End.Character)
		if err != nil {
			return "", err
		}
		if end < start {
			return "", fmt.Errorf("reversed document edit")
		}
		if change.RangeLength != nil {
			a, _ := file.PositionAt(start)
			b, _ := file.PositionAt(end)
			if *change.RangeLength != b.Index-a.Index {
				return "", fmt.Errorf("document edit rangeLength does not match UTF-16 span")
			}
		}
		text = text[:start] + change.Text + text[end:]
	}
	_, err := quint.NewSourceFile("<document>", text)
	return text, err
}
