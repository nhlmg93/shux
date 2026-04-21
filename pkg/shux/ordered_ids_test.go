package shux

import "testing"

func TestOrderedIDList(t *testing.T) {
	var ids OrderedIDList
	ids.Add(10)
	ids.Add(20)
	ids.Add(30)

	if got := ids.IndexOf(20); got != 1 {
		t.Fatalf("IndexOf(20) = %d, want 1", got)
	}
	if got := ids.IndexOf(99); got != -1 {
		t.Fatalf("IndexOf(99) = %d, want -1", got)
	}

	first, ok := ids.First()
	if !ok || first != 10 {
		t.Fatalf("First() = (%d, %t), want (10, true)", first, ok)
	}

	clone := ids.Clone()
	clone[0] = 99
	if ids[0] != 10 {
		t.Fatalf("Clone() did not return an independent copy")
	}

	if !ids.Remove(20) {
		t.Fatalf("Remove(20) = false, want true")
	}
	if got := []uint32(ids); len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("after Remove(20) = %v, want [10 30]", got)
	}
	if ids.Remove(99) {
		t.Fatalf("Remove(99) = true, want false")
	}
}
