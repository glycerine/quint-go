--------------------------------- MODULE guard ---------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Int;
  *)
  x

(*
  @type: (() => Bool);
*)
step == x < 5 /\ x' := x + 1

(*
  @type: (() => Bool);
*)
inv == x /= 4

(*
  @type: (() => Bool);
*)
init == x = 0

(*
  @type: (() => Bool);
*)
q_init == init

(*
  @type: (() => Bool);
*)
q_step == step

(*
  @type: (() => Bool);
*)
q_inv == inv

================================================================================
