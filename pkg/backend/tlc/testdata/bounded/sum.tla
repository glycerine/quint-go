---------------------------------- MODULE sum ----------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Int;
  *)
  x

VARIABLE
  (*
    @type: Int;
  *)
  y

(*
  @type: (() => Bool);
*)
init == x = 0 /\ y = 0

(*
  @type: (() => Bool);
*)
step == x' := x + 1 /\ y' := y + 1

(*
  @type: (() => Bool);
*)
inv == x + y < 10

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
