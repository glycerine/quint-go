module sum {
  var x: int
  var y: int
  action init = all { x' = 0, y' = 0 }
  action step = all { x' = x + 1, y' = y + 1 }
  val inv = x + y < 10
}
