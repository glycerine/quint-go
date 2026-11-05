-------------------------------- MODULE toggle --------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Bool;
  *)
  b

(*
  @type: (() => Bool);
*)
step == b' := (~b)

(*
  @type: (() => Bool);
*)
inv == ~b

(*
  @type: (() => Bool);
*)
init == b = FALSE

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
