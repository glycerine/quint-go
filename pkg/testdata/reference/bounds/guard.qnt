module guard {
  var x: int
  action init = x' = 0
  action step = all { x < 5, x' = x + 1 }
  val inv = x != 4
}
