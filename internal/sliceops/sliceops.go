package sliceops

import "fmt"

// Remove removes the item at the given index. If the index does not exist, an
// error is returned. -1 denotes the element at the end of the list.
func sliceRemove[E any](sl []E, index int) ([]E, error) {
	if index == -1 {
		index = len(sl) - 1
	}

	if index < -1 || index >= len(sl) {
		return sl, fmt.Errorf("index does not exist")
	}

	if len(sl) == 1 {
		sl = make([]E, 0)
		return sl, nil
	}

	newItems := make([]E, len(sl)-1)
	copy(newItems, sl[:index])
	copy(newItems[index:], sl[index+1:])
	sl = newItems

	return sl, nil
}

// Insert inserts the given item at the given index. If the index is larger than
// possible or -1, the item is inserted at the end of the list via append. All
// items at list.Items[index] are moved up by one to make room for the new item.
func sliceInsert[E any](sl []E, index int, item E) ([]E, error) {
	if index == -1 {
		index = len(sl)
	}

	if index < -1 {
		return sl, fmt.Errorf("index does not exist")
	}
	if index >= len(sl) {
		sl = append(sl, item)
		return sl, nil
	}
	updated := make([]E, len(sl)+1)

	copy(updated, sl[:index])
	updated[index] = item
	if index < len(updated) {
		copy(updated[index+1:], sl[index:])
	}

	sl = updated

	return sl, nil
}

// Move moves the item at index from to index to. If from is out of bounds, an
// error is returned. If to is negative or greater than allowed index, the item
// is moved to the end of the list.
func Move[E any](sl []E, from, to int) ([]E, error) {
	if from < 0 || from >= len(sl) {
		return sl, fmt.Errorf("from-index does not exist")
	}
	if to < -1 {
		return sl, fmt.Errorf("to-index does not exist")
	}
	if to == -1 || to >= len(sl) {
		to = len(sl) - 1
	}

	moved := sl[from]

	var err error
	sl, err = sliceRemove(sl, from)
	if err != nil {
		return sl, err
	}

	return sliceInsert(sl, to, moved)
}

// RealIndex returns the given index if it is valid. If the index is -1, it is
// converted to the highest valid index. If the index is greater than the
// highest valid, the behavior depends on clampMax; if clampMax is true, the
// index is converted to the highest valid index, and if clampMax is false, an
// error is returned. If a valid index is not possible, an error is returned. If
// the returned error is nil, the returned index is guaranteed to be able to be
// used in sl without panicking in its current state. Not thread-safe.
func RealIndex[E any](sl []E, idx int, clampMax bool) (int, error) {
	if idx == -1 {

	}
}
