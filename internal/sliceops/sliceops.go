package sliceops

import "fmt"

// Index returns the first (lowest) index of the value within the given slice.
// If the value is not found, -1 is returned.
// slices is only in stdlib as of Go 1.21; need to have this for 1.20
func Index[E comparable](sl []E, v E) int {
	for i, item := range sl {
		if item == v {
			return i
		}
	}
	return -1
}

// Filter returns a new slice with only the items that the given function
// returns true for.
func Filter[E any](sl []E, fn func(E) bool) []E {
	var newItems []E
	for _, item := range sl {
		if fn(item) {
			newItems = append(newItems, item)
		}
	}
	return newItems
}

// Remove removes the item at the given index. If the index does not exist, an
// error is returned. -1 denotes the element at the end of the list.
func Remove[E any](sl []E, index int) ([]E, error) {
	var err error
	index, err = RealIndex(sl, index, false)
	if err != nil {
		return sl, fmt.Errorf("index %w", err)
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
func Insert[E any](sl []E, index int, item E) ([]E, error) {

	// for insertion we actually get real index as though the slice were one
	// item larger, so make the updated slice first so we can check against it.
	updated := make([]E, len(sl)+1)
	var err error
	index, err = RealIndex(updated, index, true)
	if err != nil {
		return sl, fmt.Errorf("index %w", err)
	}

	// if we are inserting at the end, just append
	if index >= len(sl) {
		sl = append(sl, item)
		return sl, nil
	}

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
	var err error
	from, err = RealIndex(sl, from, false)
	if err != nil {
		return sl, fmt.Errorf("from-index %w", err)
	}
	to, err = RealIndex(sl, to, true)
	if err != nil {
		return sl, fmt.Errorf("to-index %w", err)
	}

	moved := sl[from]

	sl, err = Remove(sl, from)
	if err != nil {
		return sl, err
	}

	return Insert(sl, to, moved)
}

// RealIndex returns the given index if it is valid. If the index is -1, it is
// converted to the highest valid index. If the index is greater than the
// highest valid, the behavior depends on clampMax; if clampMax is true, the
// index is converted to the highest valid index, and if clampMax is false, an
// error is returned. If a valid index is not possible, an error is returned. If
// the returned error is nil, the returned index is guaranteed to be able to be
// used in sl without panicking in its current state. Not thread-safe.
//
// Even if an error is returned, the index at its last value is also returned,
// which informs what the actual index was at the time the error was discoverd.
func RealIndex[E any](sl []E, idx int, clampMax bool) (int, error) {
	if len(sl) == 0 {
		return idx, fmt.Errorf("does not exist")
	}

	if idx == -1 {
		idx = len(sl) - 1
	}

	if idx < 0 {
		return idx, fmt.Errorf("does not exist")
	}

	if idx >= len(sl) {
		if clampMax {
			return len(sl) - 1, nil
		}
		return idx, fmt.Errorf("does not exist")
	}

	return idx, nil
}
