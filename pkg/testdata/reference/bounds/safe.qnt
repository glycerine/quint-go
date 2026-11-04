module safe {
  var x: int
  action init = x' = 0
  action step = x' = (x + 1) % 4
  val inv = x >= 0 and x < 4
}
