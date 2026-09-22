package stacktrace

import (
	"runtime"
	"sync"
)

const initialDepth = 64

var pool = sync.Pool{
	New: func() any {
		return &stack{storage: make([]uintptr, initialDepth)}
	},
}

type stack struct {
	frames  *runtime.Frames
	pcs     []uintptr
	storage []uintptr
}

func Capture(skip int) *stack {
	s, _ := pool.Get().(*stack)
	pcs := s.storage
	for {
		if n := runtime.Callers(skip+2, pcs); n < len(pcs) {
			s.pcs = pcs[:n]
			break
		}
		pcs = make([]uintptr, len(pcs)*2)
		s.storage = pcs
	}
	s.frames = runtime.CallersFrames(s.pcs)
	return s
}

func (s *stack) Free() {
	s.pcs = nil
	s.frames = nil

	pool.Put(s)
}

func (s *stack) Next() (runtime.Frame, bool) { return s.frames.Next() }
func (s *stack) Count() int                  { return len(s.pcs) }
