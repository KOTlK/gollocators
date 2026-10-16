package allocators

import "unsafe"
import "strings"
import "fmt"

type Stack[T any] struct {
	Allocator Allocator
	Items   []T
	Count     int
	Capacity  int
}

func MakeStack[T any](allocator Allocator, capacity int) (Stack[T], error) {
	stack := Stack[T]{}

	stack.Allocator = allocator
	stack.Capacity  = capacity
	stack.Count     = 0
	data, err      := CallocSliced[T](allocator, uint(capacity))

	if err != nil {
		return stack, err
	}

	stack.Items = data

	return stack, nil
}

func (stack *Stack[T]) Destroy() {
	if stack.Allocator.IsTemp() {
		stack.Count     = 0
		stack.Capacity  = 0
		stack.Allocator = nil
		return
	}

	stack.Allocator.Free(unsafe.Pointer(unsafe.SliceData(stack.Items)))
	stack.Count     = 0
	stack.Capacity  = 0
	stack.Allocator = nil
}

func (stack *Stack[T]) Push(item T) {
	stack.maybeRealloc(1)

	stack.Items[stack.Count] = item
	stack.Count++
}

func (stack *Stack[T]) Pop() (T, bool) {
	if stack.Count == 0 {
		var zero T
		return zero, false
	}

	stack.Count--

	return stack.Items[stack.Count], true
}

func (stack *Stack[T]) String() string {
	if stack.Count == 0 {
		return "{ }"
	}

	var sb strings.Builder

	sb.WriteString("[ ")

	for i := range(stack.Count) {
		name := fmt.Sprintf("%v", stack.Items[i])

		sb.WriteString(name)

		if (i != stack.Count - 1) {
			sb.WriteString(", ")
		}
	}

	sb.WriteString(" ]")

	return sb.String()
}

func (stack *Stack[T]) maybeRealloc(addCount int) {
	if stack.Count + addCount > stack.Capacity {
		newCapacity := (stack.Capacity + addCount) * 2

		newItems, err := CallocSliced[T](stack.Allocator, uint(newCapacity))

		if err != nil {
			panic("Cannot reallocate memory for stack.")
		}

		copy(newItems, stack.Items)

		if stack.Allocator.IsTemp() == false {
			stack.Allocator.Free(unsafe.Pointer(unsafe.SliceData(stack.Items)))
		}

		stack.Items    = newItems
		stack.Capacity = newCapacity
	}
}