package tlc

import (
	"fmt"
	"strings"

	checker "github.com/glycerine/apalache-go/pkg"
	quint "github.com/glycerine/quint-go/pkg"
)

// The Apalache printer uses qualified Variants! calls with EXTENDS Variants.
// Native TLC resolves qualified calls through named instances. Add the standard
// named instance to generated input without changing operators or string data.
func nativeGeneratedTLA(source string, compiled quint.CompilationResult) (string, error) {
	if !containsTLAQualifier(source, "Variants!") {
		return source, nil
	}
	for _, decl := range compiled.MainModule.Declarations {
		if decl.Kind == "typedef" || decl.Kind == "def" && decl.Qualifier == quint.Run {
			continue
		}
		if checker.ParseableTLAName(decl.Name) == "Variants" {
			return "", fmt.Errorf("Go TLC Variants namespace conflicts with declaration %q", decl.Name)
		}
	}
	start := strings.Index(source, "\nEXTENDS ")
	if start < 0 {
		return "", fmt.Errorf("Go TLC generated Variants input has no EXTENDS declaration")
	}
	end := strings.IndexByte(source[start+1:], '\n')
	if end < 0 {
		return "", fmt.Errorf("Go TLC generated EXTENDS declaration is incomplete")
	}
	end += start + 1
	return source[:end] + "\nVariants == INSTANCE Variants" + source[end:], nil
}

func containsTLAQualifier(source, qualifier string) bool {
	if !strings.Contains(source, qualifier) {
		return false
	}
	for i := 0; i < len(source); {
		switch {
		case strings.HasPrefix(source[i:], "\\*"):
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case strings.HasPrefix(source[i:], "(*"):
			i += 2
			for depth := 1; i < len(source) && depth > 0; {
				if strings.HasPrefix(source[i:], "(*") {
					depth++
					i += 2
				} else if strings.HasPrefix(source[i:], "*)") {
					depth--
					i += 2
				} else {
					i++
				}
			}
		case source[i] == '"':
			i++
			for i < len(source) {
				if source[i] == '\\' {
					i += 2
					continue
				}
				if source[i] == '"' {
					i++
					break
				}
				i++
			}
		default:
			if strings.HasPrefix(source[i:], qualifier) && (i == 0 || !tlaIdentifierByte(source[i-1])) {
				return true
			}
			i++
		}
	}
	return false
}

func tlaIdentifierByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 128
}
