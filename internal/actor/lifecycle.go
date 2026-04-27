package actor

import "fmt"

// Lifecycle tracks child refs in a map. Init panics on duplicate id; Delete panics if
// id is missing when teardown should be paired with Init; Must panics on internal miss.
type Lifecycle[K comparable, M any] struct {
	pkg     string
	kind    string
	validID func(K) bool
	m       map[K]Ref[M]
}

// NewLifecycle returns an empty lifecycle tracker. pkg and kind appear in panic messages
// (e.g. pkg "supervisor", kind "session"). validID rejects zero/invalid ids.
func NewLifecycle[K comparable, M any](pkg, kind string, validID func(K) bool) *Lifecycle[K, M] {
	if pkg == "" {
		panic("actor: NewLifecycle: empty pkg")
	}
	if kind == "" {
		panic("actor: NewLifecycle: empty kind")
	}
	if validID == nil {
		panic("actor: NewLifecycle: nil validID")
	}
	return &Lifecycle[K, M]{
		pkg:     pkg,
		kind:    kind,
		validID: validID,
		m:       make(map[K]Ref[M]),
	}
}

func (l *Lifecycle[K, M]) Init(id K, ref Ref[M]) {
	if !l.validID(id) {
		l.panicf("Init", "invalid id %v", id)
	}
	if !ref.Valid() {
		l.panicf("Init", "invalid ref for id %v", id)
	}
	if _, exists := l.m[id]; exists {
		l.panicf("Init", "duplicate id %v", id)
	}
	l.m[id] = ref
}

func (l *Lifecycle[K, M]) Delete(id K) {
	if !l.validID(id) {
		l.panicf("Delete", "invalid id %v", id)
	}
	if _, ok := l.m[id]; !ok {
		l.panicf("Delete", "unknown id %v", id)
	}
	delete(l.m, id)
}

func (l *Lifecycle[K, M]) Must(id K) Ref[M] {
	if !l.validID(id) {
		l.panicf("Must", "invalid id %v", id)
	}
	ref, ok := l.m[id]
	if !ok {
		l.panicf("Must", "missing id %v", id)
	}
	return ref
}

func (l *Lifecycle[K, M]) panicf(op, format string, args ...any) {
	panic(fmt.Sprintf("%s: %s %s: ", l.pkg, l.kind, op) + fmt.Sprintf(format, args...))
}
