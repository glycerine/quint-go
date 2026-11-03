package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// InstantiateModule builds the uniquely named module for one resolved instance.
// The caller replaces the instance with the returned import and resolves names
// again after registering the new module.
func InstantiateModule(owner string, instance Declaration, prototype Module, table *insdict.Dict[ID, LookupDefinition], ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) (Module, Declaration, error) {
	if instance.Kind != "instance" {
		return Module{}, Declaration{}, fmt.Errorf("expected instance declaration")
	}
	qualifier := prototype.Name
	if instance.QualifiedName != nil {
		qualifier = *instance.QualifiedName
	}
	name := owner + "::" + qualifier
	declarations := []Declaration{}
	for _, o := range instance.Overrides {
		dependencies, err := DependentDefinitions(o.Expr, table)
		if err != nil {
			return Module{}, Declaration{}, err
		}
		declarations = append(declarations, dependencies...)
		body := o.Expr
		if prototype.runtimeSource != nil {
			body = sourceRuntimeExpression(body, instance.ID)
		}
		d := Declaration{Kind: "def", Qualifier: PureVal, ID: o.Parameter.ID, Name: name + "::" + o.Parameter.Name, Expr: &body}
		if def, ok := table.Get2(o.Parameter.ID); ok && def.Declaration != nil {
			d.TypeAnnotation = def.Declaration.TypeAnnotation
		}
		declarations = append(declarations, d)
	}
	for _, d := range prototype.Declarations {
		if d.Kind == "const" {
			continue
		}
		if d.Kind == "import" || d.Kind == "export" || d.Kind == "instance" {
			declarations = append(declarations, d)
			continue
		}
		d = instantiateRuntimeVariableProvenance(d, name)
		if prototype.runtimeSource != nil {
			d = sourceRuntimeDeclaration(d, instance.ID)
		}
		copied, err := AddNamespaceToDefinition(d, name, builtinResolutionNames)
		if err != nil {
			return Module{}, Declaration{}, err
		}
		copied, err = RefreshDefinitionIDs(copied, ids, sources, analysis)
		if err != nil {
			return Module{}, Declaration{}, err
		}
		declarations = append(declarations, copied)
	}
	out := prototype
	out.Name = name
	out.Declarations = declarations
	star := "*"
	return out, Declaration{Kind: "import", ID: instance.ID, ProtoName: name, DefName: &star}, nil
}
