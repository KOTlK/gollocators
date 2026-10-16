package allocators

import "unsafe"
import "strings"
import "fmt"

type Queue[T any] struct {
	Allocator Allocator
	Items   []T
	Count     int
	Capacity  int
	Head      int
	Tail      int
}

func MakeQueue[T any](allocator Allocator, capacity int) (Queue[T], error) {
	queue := Queue[T]{}

	queue.Allocator = allocator
	queue.Capacity  = capacity
	queue.Count     = 0
	queue.Head      = 0
	queue.Tail      = 0
	data, err      := CallocSliced[T](allocator, uint(capacity))

	if err != nil {
		return queue, err
	}

	queue.Items = data

	return queue, nil
}

func (queue *Queue[T]) Destroy() {
	if queue.Allocator.IsTemp() {
		queue.Allocator = nil
		queue.Count     = 0
		queue.Capacity  = 0
		queue.Head      = 0
		queue.Tail      = 0
		return
	}

	queue.Allocator.Free(unsafe.Pointer(unsafe.SliceData(queue.Items)))
	queue.Count     = 0
	queue.Capacity  = 0
	queue.Head      = 0
	queue.Tail      = 0
	queue.Allocator = nil
	queue.Items     = nil
}

func (queue *Queue[T]) Push(item T) {
	queue.maybeRealloc(1)

	queue.Items[queue.Tail] = item;
	queue.Tail = (queue.Tail + 1) % queue.Capacity
	queue.Count++;
}

func (queue *Queue[T]) Pop() (T, bool) {
	if queue.Count == 0 {
		var zero T
		return zero, false
	}

	item := queue.Items[queue.Head]

	queue.Head = (queue.Head + 1) % queue.Capacity

	queue.Count--

	if queue.Head == queue.Tail && queue.Count == 0 {
		queue.Head = 0
		queue.Tail = 0
	}

	return item, true
}

func (queue *Queue[T]) String() string {
	if queue.Count == 0 {
		return "{ }"
	}

	var sb strings.Builder

	sb.WriteString("[ ")

	for i := queue.Head; i != queue.Tail; i = (i + 1) % queue.Capacity {
		name := fmt.Sprintf("%v", queue.Items[i])

		sb.WriteString(name)

		if (i != (queue.Tail - 1) % queue.Capacity) {
			sb.WriteString(", ")
		}
	}

	sb.WriteString(" ]")

	return sb.String()
}

func (queue *Queue[T]) maybeRealloc(addCount int) {
	if queue.Count + addCount >= queue.Capacity {
		newCapacity := (queue.Capacity + addCount) * 2

		newItems, err := CallocSliced[T](queue.Allocator, uint(newCapacity))

		if err != nil {
			panic("Cannot reallocate memory for queue.")
		}

		i := 0

		for j := queue.Head; j != queue.Tail; {
			newItems[i] = queue.Items[j]
			i++
			j = (j + 1) % queue.Capacity
		}

		if queue.Allocator.IsTemp() == false {
			queue.Allocator.Free(unsafe.Pointer(unsafe.SliceData(queue.Items)))
		}

		queue.Items    = newItems
		queue.Capacity = newCapacity
		queue.Head     = 0
		queue.Tail     = i
	}
}