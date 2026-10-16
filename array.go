package allocators

import "fmt"
import "strings"
import "unsafe"

type Array[T any] struct {
	Allocator  Allocator
	Items    []T
	Capacity int
}

func MakeArray[T any](allocator Allocator, capacity int) (Array[T], error) {
	array := Array[T]{}

	array.Allocator = allocator
	array.Capacity  = capacity

	data, err := CallocSliced[T](allocator, uint(capacity))

	if err != nil {
		return array, err
	}

	array.Items = data

	return array, nil
}

func (arr *Array[T]) Destroy() {
	if arr.Allocator.IsTemp() {
		return
	}

	arr.Allocator.Free(unsafe.Pointer(unsafe.SliceData(arr.Items)))
	arr.Capacity  = 0
	arr.Allocator = nil
}

func (arr *Array[T]) Get(index int) T {
	if (index < 0 || index > arr.Capacity) {
		panic("Index outside of bounds of the array.")
	}
	
	return arr.Items[index]
}

func (arr *Array[T]) Set(index int, item T) {
	if (index < 0 || index > arr.Capacity) {
		panic("Index outside of bounds of the array.")
	}

	arr.Items[index] = item
}

func (arr Array[T]) String() string {
	if arr.Capacity == 0 {
		return "{ }"
	}

	var sb strings.Builder

	sb.WriteString("[ ")

	for i := range(arr.Capacity) {
		name := fmt.Sprintf("%v", arr.Items[i])

		sb.WriteString(name)

		if (i != arr.Capacity - 1) {
			sb.WriteString(", ")
		}
	}

	sb.WriteString(" ]")

	return sb.String()
}

func SortArray[T quick_sort_compatible](arr *Array[T]) {
	quickSort(arr.Items, 0, arr.Capacity)
}