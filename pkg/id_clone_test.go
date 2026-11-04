package quint

import "testing"

func TestIDGeneratorCloneOwnsCounter(t *testing.T) {
	t.Parallel()
	generator := NewIDGenerator()
	generator.next.SetString("900719925474099300000000000000000001", 10)
	fork := generator.Clone()
	want := "900719925474099300000000000000000001"
	if got := fork.Next().String(); got != want {
		t.Fatal(got)
	}
	fork.Next()
	if got := generator.Next().String(); got != want {
		t.Fatal("fork advanced its caller", got)
	}
	if got := generator.Clone().Next().String(); got != "900719925474099300000000000000000002" {
		t.Fatal(got)
	}
	if got := fork.Next().String(); got != "900719925474099300000000000000000003" {
		t.Fatal("caller advanced its fork", got)
	}
}
