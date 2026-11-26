package quickinput

func (s *State) Move(delta, n int) {
	s.move(delta, n)
}

func (s *State) Reset(query string) {
	*s = State{Query: query}
}
