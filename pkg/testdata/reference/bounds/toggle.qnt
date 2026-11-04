module toggle {
  var b: bool
  action init = b' = false
  action step = b' = not(b)
  val inv = not(b)
}
