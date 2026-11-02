package quint

import "testing"

func BenchmarkRuntimeSequenceCachedViews(b *testing.B) {
	values := runtimeSequenceCacheValues(b, true)
	for _, tc := range []struct {
		name string
		args []RuntimeValue
	}{
		{"empty", nil},
		{"scalar", values[:3]},
		{"nested", values[5:10]},
		{"wide", append(append([]RuntimeValue{}, values...), values...)},
	} {
		for _, implementation := range []string{"normalize", "cached"} {
			b.Run(tc.name+"/"+implementation, func(b *testing.B) {
				construct := runtimeSequenceNormalizeOracle
				if implementation == "cached" {
					construct = newRuntimeComposite
				}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := construct("List", tc.args); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
