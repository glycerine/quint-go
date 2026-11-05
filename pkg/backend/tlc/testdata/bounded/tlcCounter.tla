------------------------------ MODULE tlcCounter ------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Int;
  *)
  counter

(*
  @type: (() => Bool);
*)
step == counter < 3 /\ counter' := counter + 1

(*
  @type: (() => Bool);
*)
inv == counter >= 0

(*
  @type: (() => Bool);
*)
init == counter = 0

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
