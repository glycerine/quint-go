package quint

import (
	"encoding/json"
	"testing"
)

func BenchmarkExpressionJSONDecode(b *testing.B) {
	for _, sample := range []struct {
		name string
		data string
	}{
		{"scalar", `{"id":123456789012345678901234567890,"kind":"int","value":999999999999999999999999999}`},
		{"application", `{"id":1,"kind":"app","opcode":"iadd","args":[{"id":2,"kind":"int","value":1},{"id":3,"kind":"app","opcode":"imul","args":[{"id":4,"kind":"int","value":2},{"id":5,"kind":"name","name":"x"}]}]}`},
		{"lambda", `{"id":1,"kind":"lambda","qualifier":"def","params":[{"id":2,"name":"x"}],"expr":{"id":3,"kind":"app","opcode":"iadd","args":[{"id":4,"kind":"name","name":"x"},{"id":5,"kind":"int","value":1}]}}`},
	} {
		b.Run(sample.name, func(b *testing.B) {
			data := []byte(sample.data)
			b.ReportAllocs()
			for b.Loop() {
				var expression Expr
				if err := json.Unmarshal(data, &expression); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
