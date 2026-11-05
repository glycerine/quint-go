--------------------------------- MODULE jump ---------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Int;
  *)
  x

(*
  @type: (() => Bool);
*)
step == x < 100 /\ x' := x * 2

(*
  @type: (() => Bool);
*)
inv == x /= 8

(*
  @type: (() => Bool);
*)
init == x = 1

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
