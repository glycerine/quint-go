package tlc

import (
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestGeneratedTLAVariantsNamespace(t *testing.T) {
	for _, tc := range []struct {
		source string
		needed bool
	}{
		{`v == Variants!Variant("Some",1)`, true},
		{`v == OtherVariants!Variant("Some",1)`, false},
		{`v == "Variants!Variant"`, false},
		{`v == "escaped \" Variants!Variant"`, false},
		{"\\* Variants!Variant\nv==TRUE", false},
		{`(* outer (* Variants!Variant *) end *) v==TRUE`, false},
		{"(* Variants!Variant *)\nv==Variants!Variant(\"Some\",1)", true},
	} {
		source := "---- MODULE M ----\nEXTENDS Integers, Variants\n" + tc.source + "\n===="
		got, err := nativeGeneratedTLA(source, quint.CompilationResult{})
		if err != nil || strings.Contains(got, "Variants == INSTANCE Variants") != tc.needed || !strings.Contains(got, tc.source) {
			t.Fatal(got, err, tc)
		}
	}
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{{Kind: "var", Name: "Variants"}}}}
	if _, err := nativeGeneratedTLA("\nEXTENDS Variants\nv==Variants!Variant(\"Some\",1)\n", compiled); err == nil {
		t.Fatal("accepted namespace collision")
	}
	compiled.MainModule.Declarations[0].Kind = "typedef"
	if _, err := nativeGeneratedTLA("\nEXTENDS Variants\nv==Variants!Variant(\"Some\",1)\n", compiled); err != nil {
		t.Fatal("typedef is not emitted as a TLA declaration", err)
	}
	for _, source := range []string{`x==Variants!Variant("Some",1)`, "\nEXTENDS Variants!"} {
		if _, err := nativeGeneratedTLA(source, quint.CompilationResult{}); err == nil {
			t.Fatal("accepted malformed generated module")
		}
	}
}
