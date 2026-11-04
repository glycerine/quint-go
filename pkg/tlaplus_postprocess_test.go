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

func TestFixAppliedLetOperatorsRewritesOnlyApplicationsOfDefinedOperators(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, want string }{
		{
			name: "single-line applied lambda",
			in:   `v == (LET __QUINT_LAMBDA_1(old) == IF a THEN "black" ELSE old IN __QUINT_LAMBDA_1)(y[i])`,
			want: `v == (LET __QUINT_LAMBDA_1(old) == IF a THEN "black" ELSE old IN __QUINT_LAMBDA_1(y[i]))`,
		},
		{
			name: "multi-line definition with nested parentheses and two arguments",
			in:   "v == (LET f(a, b) == (a + b)\n  IN f)(g(1), 2)\n",
			want: "v == (LET f(a, b) == (a + b)\n  IN f(g(1), 2))\n",
		},
		{
			name: "defined name is not an operator with parameters",
			in:   `v == (LET d == 1 IN d)(2)`,
			want: `v == (LET d == 1 IN d)(2)`,
		},
		{
			name: "applied name is not the defined operator",
			in:   `v == (LET f(a) == a IN g)(1)`,
			want: `v == (LET f(a) == a IN g)(1)`,
		},
		{
			name: "empty argument list is left unchanged",
			in:   `v == (LET f(a) == a IN f)()`,
			want: `v == (LET f(a) == a IN f)()`,
		},
		{
			name: "comments and strings are not rewritten",
			in:   "\\* (LET f(a) == a IN f)(1)\n(* (LET f(a) == a IN f)(1) *)\nv == \"(LET f(a) == a IN f)(1)\"",
			want: "\\* (LET f(a) == a IN f)(1)\n(* (LET f(a) == a IN f)(1) *)\nv == \"(LET f(a) == a IN f)(1)\"",
		},
		{
			name: "identifier that merely ends with IN is not a LET",
			in:   `v == (BLOCKLET f(a) == a IN f)(1)`,
			want: `v == (BLOCKLET f(a) == a IN f)(1)`,
		},
	} {
		if got := FixAppliedLetOperators(tc.in); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

func TestPostprocessTLAAppliesBothRewrites(t *testing.T) {
	t.Parallel()
	in := "A_v == <A>_v\nv == (LET f(a) == a IN f)(1)\n"
	want := "A_v == <<A>>_v\nv == (LET f(a) == a IN f(1))\n"
	if got := PostprocessTLA(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
