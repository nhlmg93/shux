package actor

import "fmt"

// Children is the lifecycle API for keyed child actor refs: Init adds, Delete removes,
// Must looks up when presence is guaranteed by internal bookkeeping.
type Children[K comparable, M any] interface {
	Init(id K, ref Ref[M])
	Delete(id K)
	Must(id K) Ref[M]
}

// Lifecycle tracks child refs in a map. Init panics on duplicate id; Delete panics if
// id is missing when teardown should be paired with Init; Must panics on internal miss.
type Lifecycle[K comparable, M any] struct {
	pkg  string
	kind string
	m    map[K]Ref[M]
}

// NewLifecycle returns an empty lifecycle tracker. pkg and kind appear in panic messages
// (e.g. pkg "supervisor", kind "session").
func NewLifecycle[K comparable, M any](pkg, kind string) *Lifecycle[K, M] {
	return &Lifecycle[K, M]{
		pkg:  pkg,
		kind: kind,
		m:    make(map[K]Ref[M]),
	}
}

func (l *Lifecycle[K, M]) Init(id K, ref Ref[M]) {
	if _, exists := l.m[id]; exists {
		panic(fmt.Sprintf("%s: duplicate %s id %v", l.pkg, l.kind, id))
	}
	l.m[id] = ref
}

func (l *Lifecycle[K, M]) Delete(id K) {
	if _, ok := l.m[id]; !ok {
		panic(fmt.Sprintf("%s: Delete: unknown %s id %v", l.pkg, l.kind, id))
	}
	delete(l.m, id)
}

func (l *Lifecycle[K, M]) Must(id K) Ref[M] {
	ref, ok := l.m[id]
	if !ok {
		panic(fmt.Sprintf("%s: internal lookup missing %s %v", l.pkg, l.kind, id))
	}
	return ref
}
