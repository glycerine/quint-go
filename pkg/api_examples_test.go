package quint_test

import (
	"context"
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
)

func ExampleEvaluatePureModules() {
	parsed, err := quint.ParseModulesWithLocations("answer.qnt", "module main { pure val answer = 6 * 7 }")
	if err != nil {
		panic(err)
	}
	value, err := quint.EvaluatePureModules(context.Background(), parsed.Modules, parsed.IDs, "main", "answer", nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(value.String())
	// Output: 42
}
