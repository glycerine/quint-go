module counter3 {
  var x: int
  action init = x' = 0
  action step = x' = x + 1
  val inv = x < 3
}
