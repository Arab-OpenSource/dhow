package masking

type Stream struct {
	m       *Masker
	c       *compiled
	state   int32
	pending []byte
	covered []bool
	inMask  bool
}

func (s *Stream) refresh() {
	latest := s.m.current.Load()
	if latest == s.c {
		return
	}
	s.c = latest
	s.state = root
	old := s.pending
	oldCovered := s.covered
	s.pending, s.covered = s.pending[:0:0], s.covered[:0:0]
	for i, b := range old {
		s.feed(b)
		if oldCovered[i] {
			s.covered[len(s.covered)-1] = true
		}
	}
}

func (s *Stream) feed(b byte) {
	ac := s.c.ac
	s.pending = append(s.pending, b)
	s.covered = append(s.covered, false)
	s.state = ac.step(s.state, b)
	if n := int(ac.out[s.state]); n > 0 {
		for i := len(s.covered) - n; i < len(s.covered); i++ {
			s.covered[i] = true
		}
	}
}

func (s *Stream) Write(p []byte) []byte {
	s.refresh()
	if s.c.ac.empty() && len(s.pending) == 0 {
		return append([]byte(nil), p...)
	}
	for _, b := range p {
		s.feed(b)
	}
	keep := int(s.c.ac.keep[s.state])
	return s.emit(len(s.pending) - keep)
}

func (s *Stream) Flush() []byte {
	s.refresh()
	out := s.emit(len(s.pending))
	s.state = root
	return out
}

func (s *Stream) Pending() int { return len(s.pending) }

func (s *Stream) emit(n int) []byte {
	if n <= 0 {
		return nil
	}
	out := make([]byte, 0, n)
	for i := 0; i < n; {
		if !s.covered[i] {
			out = append(out, s.pending[i])
			i++
			continue
		}
		continues := i == 0 && s.inMask
		for i < n && s.covered[i] {
			i++
		}
		if !continues {
			out = append(out, Replacement...)
		}
	}
	s.inMask = s.covered[n-1]
	s.pending = append(s.pending[:0], s.pending[n:]...)
	s.covered = append(s.covered[:0], s.covered[n:]...)
	return out
}
