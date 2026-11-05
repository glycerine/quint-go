--------------------------------- MODULE safe ---------------------------------

EXTENDS Integers, Sequences, FiniteSets, TLC, Apalache, Variants

VARIABLE
  (*
    @type: Int;
  *)
  x

(*
  @type: (() => Bool);
*)
step == x' := (x + 1) - (IF (x + 1 < 0 /\ 4 > 0) \/ (x + 1 > 0 /\ 4 < 0) THEN -(IF x + 1 < 0 THEN -(x + 1) ELSE x + 1) \div (IF 4 < 0 THEN -4 ELSE 4) ELSE (IF x + 1 < 0 THEN -(x + 1) ELSE x + 1) \div (IF 4 < 0 THEN -4 ELSE 4)) * 4

(*
  @type: (() => Bool);
*)
inv == x >= 0 /\ x < 4

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
