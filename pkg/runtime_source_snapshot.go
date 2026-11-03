package quint

// recoverSourceSnapshot follows the evaluator's mutable pick-map recovery.
// Unlike the public Recover API, a later pick update can change this snapshot's
// metadata. Repeated actionAny attempts observe that alias in the source runtime.
func (s *VariableStorage) recoverSourceSnapshot(snapshot VariableSnapshot) {
	if snapshot.next == nil {
		return
	}
	for key, register := range s.next.All() {
		if value, exists := snapshot.next.Get2(key); exists {
			register.Value = value
		}
	}
	s.ActionTaken = snapshot.action
	s.NondetPicks = snapshot.picks
}
