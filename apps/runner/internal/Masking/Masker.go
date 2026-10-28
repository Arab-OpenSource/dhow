package masking

import (
	"sync"
	"sync/atomic"
)

const (
	MinLength   = 4
	Replacement = "***"
	LongSecret  = 4 << 10
)

type compiled struct {
	ac  *automaton
	gen uint64
}

type Masker struct {
	mu       sync.Mutex
	secrets  map[string]bool
	patterns map[string]bool
	gen      uint64
	current  atomic.Pointer[compiled]
}

func New() *Masker {
	m := &Masker{secrets: map[string]bool{}, patterns: map[string]bool{}}
	m.current.Store(&compiled{ac: newAutomaton(nil)})
	return m
}

func (m *Masker) Add(secret []byte) bool { return m.AddAll([][]byte{secret})[0] }

func (m *Masker) AddAll(secrets [][]byte) []bool {
	added := make([]bool, len(secrets))
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	for i, secret := range secrets {
		if len(secret) < MinLength {
			continue
		}
		added[i] = true
		if m.secrets[string(secret)] {
			continue
		}
		m.secrets[string(secret)] = true
		for _, v := range Variants(secret) {
			m.patterns[string(v)] = true
		}
		changed = true
	}
	if changed {
		m.rebuild()
	}
	return added
}

func (m *Masker) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets = map[string]bool{}
	m.patterns = map[string]bool{}
	m.rebuild()
}

func (m *Masker) rebuild() {
	patterns := make([][]byte, 0, len(m.patterns))
	for p := range m.patterns {
		patterns = append(patterns, []byte(p))
	}
	m.gen++
	m.current.Store(&compiled{ac: newAutomaton(patterns), gen: m.gen})
}

func (m *Masker) Mask(b []byte) []byte {
	s := m.Stream()
	out := s.Write(b)
	return append(out, s.Flush()...)
}

func (m *Masker) MaskString(v string) string { return string(m.Mask([]byte(v))) }

func (m *Masker) MaxPatternLength() int { return m.current.Load().ac.maxLen }

func (m *Masker) Stream() *Stream {
	return &Stream{m: m, c: m.current.Load()}
}
