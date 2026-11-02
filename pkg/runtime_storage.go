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
	current *insdict.Dict[string, *RuntimeRegister]
	next    *insdict.Dict[string, *RuntimeRegister]
	caches  []*RuntimeCache
}

func NewVariableStorage() *VariableStorage {
	return &VariableStorage{current: insdict.NewDict[string, *RuntimeRegister](), next: insdict.NewDict[string, *RuntimeRegister]()}
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
	for _, cache := range s.caches {
		cache.Value = RuntimeValue{}
		cache.Err = nil
	}
}

// Reset clears registers; like upstream reset, it leaves tracked caches alone.
func (s *VariableStorage) Reset() {
	for _, register := range s.current.All() {
		register.Value = RuntimeValue{}
	}
	for _, register := range s.next.All() {
		register.Value = RuntimeValue{}
	}
}

// VariableSnapshot stores the next-state values needed for action backtracking.
// It deliberately excludes current registers and caches.
type VariableSnapshot struct {
	next *insdict.Dict[string, RuntimeValue]
}

func (s *VariableStorage) Snapshot() VariableSnapshot {
	values := insdict.NewDict[string, RuntimeValue]()
	for key, register := range s.next.All() {
		values.Set(key, register.Value)
	}
	return VariableSnapshot{next: values}
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
}

// Record exports set current variables, omitting unset registers. Runtime
// metadata is added by the simulation layer separately.
func (s *VariableStorage) Record() (RuntimeValue, error) {
	args := []Expr{}
	for _, register := range s.current.All() {
		if register.Value.Valid() {
			args = append(args, Expr{Kind: "str", Str: register.Name}, register.Value.source)
		}
	}
	return NewRuntimeValue(Expr{Kind: "app", Opcode: "Rec", Args: args})
}
