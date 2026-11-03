package quint

import "testing"

func TestUpstreamSeededIDGeneratorAPI(t *testing.T) {
	t.Parallel()
	for _, fixture := range readAPISourceCompactFixture(t).IDs {
		t.Run(fixture.Initial, func(t *testing.T) {
			t.Parallel()
			last, err := ParseID(fixture.Initial)
			if err != nil {
				t.Fatal(err)
			}
			left, right := NewIDGeneratorAfter(last), NewIDGeneratorAfter(last)
			for _, expected := range fixture.Next {
				if got := left.Next().String(); got != expected {
					t.Fatalf("ID after %s=%s; want %s", fixture.Initial, got, expected)
				}
			}
			if got := right.Next().String(); got != fixture.Next[0] || last.String() != fixture.Initial {
				t.Fatal("resumed generators share counters or mutate their seed", got, last)
			}
		})
	}
}

func TestResumedIDGeneratorFeedsIncrementalParsing(t *testing.T) {
	t.Parallel()
	last, err := ParseID("999999999999999999999999999999999999")
	if err != nil {
		t.Fatal(err)
	}
	ids := NewIDGeneratorAfter(last)
	for _, expected := range []string{"1000000000000000000000000000000000000", "1000000000000000000000000000000000001"} {
		result, err := ParseExpressionOrDeclaration("1", ids)
		if err != nil || result.Expr == nil || result.Expr.ID.String() != expected {
			t.Fatalf("resumed parse=%+v, %v; want ID %s", result, err, expected)
		}
	}
	if got := NewIDGeneratorAfter(ID{}).Next(); got != NewIDGenerator().Next() {
		t.Fatal("zero checkpoint must preserve default allocation", got)
	}
}
