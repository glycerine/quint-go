package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestTemporalGeneratedTableauNames(t *testing.T) {
	for _, name := range []string{"__temporal_t_1", "__temporal_t_z", "__temporal_t_10", "__temporal_t_3w5e11264sgsf", "__temporal_t_1_unroll", "__temporal_t_a_unroll_prev", "__saved___temporal_t_1"} {
		if !temporalGeneratedTableauName(name) {
			t.Fatal("generated name was not recognized", name)
		}
	}
	for _, name := range []string{"__user", "__temporal_custom", "__temporal_t_0", "__temporal_t_01", "__temporal_t_A", "__temporal_t_1_prev", "__temporal_t_3w5e11264sgsg", "__temporal_t_1_other", "I::__temporal_t_1", "__saved___temporal_t_1_unroll", "__saved___temporal_t_1_unroll_prev"} {
		if temporalGeneratedTableauName(name) {
			t.Fatal("unrelated name was reserved", name)
		}
	}
}

func TestTemporalGeneratedNameValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decls    []quint.Declaration
		temporal []string
		bad      []string
	}{
		{"fixed variable", []quint.Declaration{{Kind: "var", Name: "__InLoop"}}, []string{"live"}, []string{"__InLoop"}},
		{"fixed operator", []quint.Declaration{{Kind: "def", Name: "__LoopOK"}}, []string{"live"}, []string{"__LoopOK"}},
		{"named assumption", []quint.Declaration{{Kind: "assume", Name: "__LoopOK"}}, []string{"live"}, []string{"__LoopOK"}},
		{"saved variable", []quint.Declaration{{Kind: "var", Name: "x"}, {Kind: "var", Name: "__saved_x"}}, []string{"live"}, []string{"__saved_x"}},
		{"saved namespaced variable", []quint.Declaration{{Kind: "var", Name: "M::I::x"}, {Kind: "const", Name: "__saved_M::I::x"}}, []string{"live"}, []string{"__saved_M::I::x"}},
		{"selected property initial", []quint.Declaration{{Kind: "var", Name: "__live_init"}}, []string{"live"}, []string{"__live_init"}},
		{"wrapper property initial", []quint.Declaration{{Kind: "def", Name: "__q::temporalProps_init"}}, []string{"q::temporalProps"}, []string{"__q::temporalProps_init"}},
		{"all conflicts retain declaration order", []quint.Declaration{{Kind: "var", Name: "__temporal_t_1_unroll_prev"}, {Kind: "var", Name: "__InLoop"}, {Kind: "const", Name: "__saved___temporal_t_2"}}, []string{"live"}, []string{"__temporal_t_1_unroll_prev", "__InLoop", "__saved___temporal_t_2"}},
		{"safety is unchanged", []quint.Declaration{{Kind: "var", Name: "__InLoop"}, {Kind: "var", Name: "__saved_x"}, {Kind: "var", Name: "x"}}, nil, nil},
		{"unrelated private and imported names", []quint.Declaration{{Kind: "var", Name: "M::I::__InLoop"}, {Kind: "var", Name: "__saved_x"}, {Kind: "var", Name: "__temporal_custom"}, {Kind: "var", Name: "__different_init"}}, []string{"live"}, nil},
		{"omitted declarations", []quint.Declaration{{Kind: "typedef", Name: "__InLoop"}, {Kind: "def", Qualifier: quint.Run, Name: "__LoopOK"}}, []string{"live"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := &quint.IDGenerator{}
			for i := range tc.decls {
				tc.decls[i].ID = ids.Next()
			}
			compiled := quint.CompilationResult{MainModule: quint.Module{Name: "M", Declarations: tc.decls}}
			before, _ := json.Marshal(compiled.MainModule)
			err := validateTemporalNames(compiled, CheckOptions{Temporal: tc.temporal})
			after, _ := json.Marshal(compiled.MainModule)
			if string(before) != string(after) {
				t.Fatal("validation mutated compiled IR")
			}
			if len(tc.bad) == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Explanation != "Cannot prepare temporal verification" || len(err.Diagnostics) != len(tc.bad) {
				t.Fatal(err)
			}
			for i, diagnostic := range err.Diagnostics {
				if diagnostic.Code != quint.QNT000 || diagnostic.Reference == nil || !strings.Contains(diagnostic.Message, tc.bad[i]) {
					t.Fatal(diagnostic)
				}
				found := false
				for _, declaration := range tc.decls {
					found = found || declaration.Name == tc.bad[i] && *diagnostic.Reference == declaration.ID
				}
				if !found {
					t.Fatal("diagnostic did not refer to original declaration", diagnostic)
				}
			}
		})
	}
}

func TestTemporalNameCollisionCannotReportSafety(t *testing.T) {
	// Before name validation, this source reported success at bound three:
	// sharing __InLoop prevents selecting the true -> true closing transition.
	// The concrete false,true,true lasso violates the requested property.
	compiled := compileSource(t, `module M {
  var __InLoop:bool action init=__InLoop'=false action step=__InLoop'=true
  val safe=true temporal live=always(not(__InLoop))
}`)
	for _, property := range []string{"live", "q::temporalProps"} {
		result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 3, Temporal: []string{property}})
		var preparation *PreparationError
		if !errors.As(err, &preparation) || result.Outcome != "" || len(preparation.Diagnostics) != 1 {
			t.Fatal("reserved name reached checker or produced a verdict", result, err)
		}
		id := preparation.Diagnostics[0].Reference
		location := quint.Location{Source: "collision.qnt"}
		report := ReportCheck(result, err, quint.SourceMap{*id: location})
		if report.Explanation != "Cannot prepare temporal verification" || len(report.Errors) != 1 || len(report.Errors[0].Locations) != 1 || report.Errors[0].Locations[0].Source != "collision.qnt" {
			t.Fatal("lost source diagnostic", report)
		}
	}
}
