package quint_test

import (
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
)

func ExampleIRTransformer_TransformType() {
	input, err := quint.ParseType("Set[int]")
	if err != nil {
		panic(err)
	}
	transformer := quint.IRTransformer{ExitType: func(typ quint.Type) (quint.Type, error) {
		if typ.Kind == "int" {
			return quint.Type{Kind: "bool", ID: typ.ID}, nil
		}
		return typ, nil
	}}
	output, err := transformer.TransformType(input)
	if err != nil {
		panic(err)
	}
	fmt.Println(quint.TypeString(input))
	fmt.Println(quint.TypeString(output))
	// Output:
	// Set[int]
	// Set[bool]
}
