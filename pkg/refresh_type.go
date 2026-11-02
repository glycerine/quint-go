package quint

// RefreshTypeIDs copies a type and refreshes every present type-node ID in
// preorder. Rows themselves have no IDs. Missing IDs remain absent.
func RefreshTypeIDs(typ Type, ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) (Type, error) {
	out, err := ApplySubstitution(nil, typ)
	if err != nil {
		return Type{}, err
	}
	err = WalkType(&out, func(t *Type) (bool, error) {
		if t.ID != nil {
			id := refreshAnalysisID(*t.ID, ids, sources, analysis)
			t.ID = &id
		}
		return true, nil
	})
	return out, err
}
