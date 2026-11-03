package quint

import "os"

// LoadPreservingLineEnds reads and records decoded UTF-8 without converting
// line endings. The upstream CLI uses this for its main input; imported files
// continue through Load and its existing LF normalization.
func (r *FileSourceResolver) LoadPreservingLineEnds(path SourceLookupPath) (string, error) {
	data, err := os.ReadFile(path.NormalizedPath)
	if err != nil {
		return "", &SourceLoadError{Path: path.NormalizedPath, Cause: err}
	}
	text := decodeSourceUTF8(data)
	r.sourceCode[path.NormalizedPath] = text
	return text, nil
}
