package quint

import "testing"

func TestFixAngleActions(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"name", "a == []<>(<Inc>_x)", "a == []<>(<<Inc>>_x)"},
		{"square", "e == [Inc]_x /\\ <Inc>_x", "e == [Inc]_x /\\ <<Inc>>_x"},
		{"call", "a == <A(i, (j))>_vars", "a == <<A(i, (j))>>_vars"},
		{"parentheses", "b == []<>(<(x' > x /\\ x < 2)>_<<x, y>>)", "b == []<>(<<(x' > x /\\ x < 2)>>_<<x, y>>)"},
		{"multiple", "a == <A>_x /\\ <(B)>_y", "a == <<A>>_x /\\ <<(B)>>_y"},
		{"correct", "a == []<>(<<Inc>>_x) /\\ <<(x' > x)>>_<<x, y>>", "a == []<>(<<Inc>>_x) /\\ <<(x' > x)>>_<<x, y>>"},
		{"comments", "(* <A>_x *)\n\\* <A>_x\ns == \"<A>_x\"\n", "(* <A>_x *)\n\\* <A>_x\ns == \"<A>_x\"\n"},
		{"operators", "a == x > _y /\\ [A]_x /\\ WF_x(A) /\\ x' >= x /\\ <<1, 2>>", "a == x > _y /\\ [A]_x /\\ WF_x(A) /\\ x' >= x /\\ <<1, 2>>"},
		{"nested-comment", "(* outer (* <A>_x *) <B>_x *) <C>_x", "(* outer (* <A>_x *) <B>_x *) <<C>>_x"},
		{"index", "<f[x]>_v", "<<f[x]>>_v"},
		{"unbalanced", "<A(i>_v", "<A(i>_v"},
		{"missing-open", "A(i)>_v", "A(i)>_v"},
		{"empty", "<>_v", "<>_v"},
		{"unicode", "(* λ <A>_x *) <A>_v", "(* λ <A>_x *) <<A>>_v"},
		{"escaped", `s == "\"<A>_v" <B>_v`, `s == "\"<A>_v" <<B>>_v`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FixAngleActions(tc.input)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if PostprocessTLA(got) != got {
				t.Fatal("not idempotent")
			}
		})
	}
	input := `d ==
  []<>(<(\A i \in { 1, 2 }:
    (x' > x + i \/ y' < y - i) \/ x' + y' >= i)>_<<x, y>>)`
	want := `d ==
  []<>(<<(\A i \in { 1, 2 }:
    (x' > x + i \/ y' < y - i) \/ x' + y' >= i)>>_<<x, y>>)`
	if got := PostprocessTLA(input); got != want {
		t.Fatal(got)
	}
}
