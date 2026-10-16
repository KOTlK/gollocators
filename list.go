package allocators

import "strings"
import "fmt"
import "unsafe"

type List[T any] struct {
	Allocator Allocator
	Items     []T
	Capacity  int
	Count     int
}

func MakeList[T any](allocator Allocator, capacity int) (List[T], error) {
	list := List[T]{}

	list.Allocator = allocator
	list.Capacity  = capacity
	list.Count     = 0
	data, err     := CallocSliced[T](allocator, uint(capacity))

	if err != nil {
		return list, err
	}

	list.Items = data

	return list, nil
}

func (list *List[T]) Destroy() {
	if list.Allocator.IsTemp() {
		list.Allocator = nil
		list.Count     = 0
		list.Capacity  = 0
		return
	}

	list.Allocator.Free(unsafe.Pointer(unsafe.SliceData(list.Items)))
	list.Count     = 0
	list.Capacity  = 0
	list.Allocator = nil
}

func (list *List[T]) Get(index int) T {
	if (index < 0 || index > list.Capacity) {
		panic("Index outside of bounds of the list.")
	}
	
	return list.Items[index]
}

func (list *List[T]) Set(index int, item T) {
	if (index < 0 || index > list.Capacity) {
		panic("Index outside of bounds of the list.")
	}

	list.Items[index] = item
}

func (list *List[T]) Append(item T) int {
	list.maybeRealloc(1)

	index := list.Count
	list.Items[index] = item
	list.Count++

	return index
}

func (list *List[T]) AppendSlice(items []T) (start int, count int) {
	start = list.Count
	count = int(len(items))

	list.maybeRealloc(count)

	for i := range(count) {
		list.Items[start + i] = items[i]
	}

	list.Count += count

	return start, count
}

func Contains[T comparable](list *List[T], item T) (bool, int) {
	for i := int(0); i < list.Count; i++ {
		if list.Items[i] == item {
			return true, i
		}
	}

	return false, 0
}

// Removes element and puts the last element at it's place
func Remove[T comparable](list *List[T], item T) bool {
	for i := int(0); i < list.Count; i++ {
		if list.Items[i] == item {
			list.Count--
			list.Items[i] = list.Items[list.Count]
			return true
		}
	}

	return false
}

func RemoveAll[T comparable](list *List[T], item T) (bool, int) {
	var removed int = 0;

	for i := int(0); i < list.Count; i++ {
		if list.Items[i] == item {
			list.Count--
			list.Items[i] = list.Items[list.Count]
			removed++
			i--
		}
	}

	if removed > 0 {
		return true, removed
	}

	return false, 0
}

func (list *List[T]) RemoveAt(index int) {
	if index >= list.Count || index < 0 {
		panic("Index out of range of the list.")
	}

	list.Count--
	list.Items[index] = list.Items[list.Count]
}

func SortList[T quick_sort_compatible](list *List[T]) {
	quickSort(list.Items, 0, list.Count - 1)
}

func (list *List[T]) String() string {
	if list.Count == 0 {
		return "{ }"
	}

	var sb strings.Builder

	sb.WriteString("[ ")

	for i := range(list.Count) {
		name := fmt.Sprintf("%v", list.Items[i])

		sb.WriteString(name)

		if (i != list.Count - 1) {
			sb.WriteString(", ")
		}
	}

	sb.WriteString(" ]")

	return sb.String()
}

type quick_sort_compatible interface {
	int8 | uint8 | int16 | uint16 | int32 | uint32 |
	int64 | uint64 | int | uint | uintptr
}

func quickSort[T quick_sort_compatible](slice []T, lo int, hi int) {
	if lo < hi {
		pi := partition(slice, lo, hi)

		quickSort(slice, lo, pi - 1)
		quickSort(slice, pi + 1, hi)
	}
}

func partition[T quick_sort_compatible](slice []T, lo int, hi int) int {
	pivot := slice[hi]
	i := lo - 1

	for j := lo; j < hi; j++ {
		if slice[j] <= pivot {
			i++
			slice[i], slice[j] = slice[j], slice[i]
		}
	}

	slice[i + 1], slice[hi] = slice[hi], slice[i + 1]

	return i + 1
}

func (list *List[T]) maybeRealloc(addCount int) {
	if list.Count + addCount > list.Capacity {
		newCapacity := (list.Capacity + addCount) * 2

		newItems, err := CallocSliced[T](list.Allocator, uint(newCapacity))

		if err != nil {
			panic("Cannot reallocate memory for list.")
		}

		copy(newItems, list.Items)

		if list.Allocator.IsTemp() == false {
			list.Allocator.Free(unsafe.Pointer(unsafe.SliceData(list.Items)))
		}

		list.Items    = newItems
		list.Capacity = newCapacity
	}
}