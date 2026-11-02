package quint

import "github.com/glycerine/insdict"

// RuntimeRegister is a stable cell shared by compiled variable reads/writes.
// Invalid Value means unset; Read supplies the reference QNT502 diagnostic.
type RuntimeRegister struct {
	Name  string
	Value RuntimeValue
}

func (r *RuntimeRegister) Read() (RuntimeValue, error) {
	if !r.Value.Valid() {
		return RuntimeValue{}, Diagnostic{Code: "QNT502", Message: "Variable " + r.Name + " not set"}
	}
	return r.Value, nil
}

// RuntimeCache holds a memoized evaluation and is cleared on state shifts.
type RuntimeCache struct {
	Value RuntimeValue
	Err   error
}

// VariableStorage owns current/next registers for one evaluator. Keys identify
// declarations; display names are kept separately for diagnostics and records.
// It is not safe for concurrent use.
type VariableStorage struct {
	current     *insdict.Dict[string, *RuntimeRegister]
	next        *insdict.Dict[string, *RuntimeRegister]
	caches      []*RuntimeCache
	metadata    bool
	ActionTaken string
	NondetPicks *insdict.Dict[string, RuntimeValue]
}

func NewVariableStorage() *VariableStorage {
	return NewVariableStorageWithMetadata(false)
}

// NewVariableStorageWithMetadata enables action/pick fields in exported records.
func NewVariableStorageWithMetadata(enabled bool) *VariableStorage {
	return &VariableStorage{current: insdict.NewDict[string, *RuntimeRegister](), next: insdict.NewDict[string, *RuntimeRegister](), metadata: enabled, NondetPicks: insdict.NewDict[string, RuntimeValue]()}
}

// ClearMetadata preserves registered pick names while marking their values unset.
func (s *VariableStorage) ClearMetadata() {
	if s.metadata {
		s.ActionTaken = ""
		for name := range s.NondetPicks.All() {
			s.NondetPicks.Set(name, RuntimeValue{})
		}
	}
}

// Register returns the same register pointers on repeated registration of a key.
func (s *VariableStorage) Register(key, name string) (*RuntimeRegister, *RuntimeRegister) {
	if current, ok := s.current.Get2(key); ok {
		return current, s.next.Get(key)
	}
	current, next := &RuntimeRegister{Name: name}, &RuntimeRegister{Name: name}
	s.current.Set(key, current)
	s.next.Set(key, next)
	return current, next
}
func (s *VariableStorage) TrackCache(cache *RuntimeCache) { s.caches = append(s.caches, cache) }

// Shift replaces every current value with its corresponding next value, then
// clears next registers and tracked caches. Missing assignments become unset;
// they do not silently carry the previous state's value forward.
func (s *VariableStorage) Shift() {
	for key, current := range s.current.All() {
		current.Value = s.next.Get(key).Value
	}
	for _, next := range s.next.All() {
		next.Value = RuntimeValue{}
	}
	s.clearCaches()
}

// Reset clears registers; like upstream reset, it leaves tracked caches alone.
func (s *VariableStorage) Reset() {
	for _, register := range s.current.All() {
		register.Value = RuntimeValue{}
	}
	for _, register := range s.next.All() {
		register.Value = RuntimeValue{}
	}
	s.ClearMetadata()
}

func (s *VariableStorage) clearCaches() {
	for _, cache := range s.caches {
		cache.Value = RuntimeValue{}
		cache.Err = nil
	}
}

// VariableSnapshot stores next-state values and metadata for action backtracking.
// It deliberately excludes current registers and caches.
type VariableSnapshot struct {
	next   *insdict.Dict[string, RuntimeValue]
	picks  *insdict.Dict[string, RuntimeValue]
	action string
}

func (s *VariableStorage) Snapshot() VariableSnapshot {
	values := insdict.NewDict[string, RuntimeValue]()
	for key, register := range s.next.All() {
		values.Set(key, register.Value)
	}
	picks := insdict.NewDict[string, RuntimeValue]()
	for name, value := range s.NondetPicks.All() {
		picks.Set(name, value)
	}
	return VariableSnapshot{next: values, picks: picks, action: s.ActionTaken}
}

// Recover restores values into existing cells, preserving compiled references.
// Registers introduced after the snapshot are left as they are, as upstream.
func (s *VariableStorage) Recover(snapshot VariableSnapshot) {
	if snapshot.next == nil {
		return
	}
	for key, register := range s.next.All() {
		if value, ok := snapshot.next.Get2(key); ok {
			register.Value = value
		}
	}
	s.ActionTaken = snapshot.action
	s.NondetPicks = insdict.NewDict[string, RuntimeValue]()
	if snapshot.picks != nil {
		for name, value := range snapshot.picks.All() {
			s.NondetPicks.Set(name, value)
		}
	}
}

// Record exports set current variables and optional action/pick metadata.
func (s *VariableStorage) Record() (RuntimeValue, error) {
	args := []Expr{}
	for _, register := range s.current.All() {
		if register.Value.Valid() {
			args = append(args, Expr{Kind: "str", Str: register.Name}, register.Value.source)
		}
	}
	if s.metadata {
		picks := []Expr{}
		for name, value := range s.NondetPicks.All() {
			label, payload := "None", Expr{Kind: "app", Opcode: "Tup", Args: []Expr{}}
			if value.Valid() {
				label, payload = "Some", value.source
			}
			picks = append(picks, Expr{Kind: "str", Str: name}, Expr{Kind: "app", Opcode: "variant", Args: []Expr{{Kind: "str", Str: label}, payload}})
		}
		args = append(args, Expr{Kind: "str", Str: ITFNondetPicks}, Expr{Kind: "app", Opcode: "Rec", Args: picks}, Expr{Kind: "str", Str: ITFActionTaken}, Expr{Kind: "str", Str: s.ActionTaken})
	}
	return NewRuntimeValue(Expr{Kind: "app", Opcode: "Rec", Args: args})
}

// FromRecord resets registers, restores known variable names and optional
// metadata, and clears tracked caches. Unknown fields are ignored. As upstream,
// Some pick values are retained as variants, rather than unwrapped.
func (s *VariableStorage) FromRecord(record RuntimeValue) error {
	fail := func(message string) error { return Diagnostic{Code: "QNT501", Message: message} }
	if !record.Valid() || record.expression.Kind != "app" || record.expression.Opcode != "Rec" {
		return fail("Expected a record value")
	}
	s.Reset()
	fields := insdict.NewDict[string, Expr]()
	for i := 0; i < len(record.source.Args); i += 2 {
		fields.Set(record.source.Args[i].Str, record.source.Args[i+1])
	}
	for name, expression := range fields.All() {
		value, err := NewRuntimeValue(expression)
		if err != nil {
			return err
		}
		found := false
		for _, register := range s.current.All() {
			if register.Name == name {
				register.Value = value
				found = true
				break
			}
		}
		if found || !s.metadata {
			continue
		}
		switch name {
		case ITFActionTaken:
			if expression.Kind != "str" {
				return fail("Expected a string action name")
			}
			s.ActionTaken = expression.Str
		case ITFNondetPicks:
			if expression.Kind != "app" || expression.Opcode != "Rec" {
				return fail("Expected a record of nondeterministic picks")
			}
			for i := 0; i < len(expression.Args); i += 2 {
				pick, err := NewRuntimeValue(expression.Args[i+1])
				if err != nil {
					return err
				}
				if pick.expression.Kind != "app" || pick.expression.Opcode != "variant" {
					return fail("Expected a variant pick value")
				}
				if pick.expression.Args[0].Str == "None" {
					pick = RuntimeValue{}
				}
				s.NondetPicks.Set(expression.Args[i].Str, pick)
			}
		}
	}
	s.clearCaches()
	return nil
}
