package quint

import "testing"

func TestDocumentationWhitespace(t *testing.T) {
	for _, tc := range []struct{ comment, want string }{
		{"///  indented  \n", " indented  "}, {"///nospace\n", "ospace"},
		{"/// text\r\n", "text\r"}, {"/// \n", ""}, {"///\n", ""},
		{"/// first\n/// \n/// third\n", "first\n\nthird"},
	} {
		ms, err := ParseModules(tc.comment + "module M {\n" + tc.comment + "val x = 1 }")
		if err != nil {
			t.Fatal(err)
		}
		for _, doc := range []*string{ms[0].Doc, ms[0].Declarations[0].Doc} {
			if tc.want == "" {
				if doc != nil {
					t.Errorf("empty documentation: %q", *doc)
				}
			} else if doc == nil || *doc != tc.want {
				t.Errorf("%q: %v want %q", tc.comment, doc, tc.want)
			}
		}
	}
}

func TestDocumentationOnExpandedDeclarations(t *testing.T) {
	for _, source := range []string{"type T = A | B", "val (a,b) = pair", "val {a,b} = record", "val (_,_) = pair"} {
		ds, err := ParseDeclaration("/// docs\n" + source)
		if err != nil {
			t.Fatal(err)
		}
		for i, d := range ds {
			if i == len(ds)-1 {
				if d.Doc == nil || *d.Doc != "docs" {
					t.Fatal("missing final declaration docs")
				}
			} else if d.Doc != nil {
				t.Fatal("docs attached to earlier generated declaration")
			}
		}
	}
	ms, err := ParseModules("module M { /// orphan\n } module N { val x = 1 }")
	if err != nil || ms[1].Declarations[0].Doc != nil {
		t.Fatalf("documentation leaked between modules: %v", err)
	}
}
