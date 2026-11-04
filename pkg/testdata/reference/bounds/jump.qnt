module jump {
  var x: int
  action init = x' = 1
  action step = all { x < 100, x' = x * 2 }
  val inv = x != 8
}
