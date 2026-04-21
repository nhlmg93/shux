package shux

// OrderedIDList keeps IDs in stable insertion order while offering a small
// vocabulary for the common index/remove/copy operations used by sessions and
// windows.
type OrderedIDList []uint32

func (ids *OrderedIDList) Add(id uint32) {
	*ids = append(*ids, id)
}

func (ids *OrderedIDList) Remove(target uint32) bool {
	index := ids.IndexOf(target)
	if index < 0 {
		return false
	}
	current := *ids
	*ids = append(current[:index], current[index+1:]...)
	return true
}

func (ids OrderedIDList) IndexOf(target uint32) int {
	for i, id := range ids {
		if id == target {
			return i
		}
	}
	return -1
}

func (ids OrderedIDList) Clone() []uint32 {
	return append([]uint32(nil), ids...)
}

func (ids OrderedIDList) First() (uint32, bool) {
	if len(ids) == 0 {
		return 0, false
	}
	return ids[0], true
}
