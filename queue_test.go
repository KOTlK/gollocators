package allocators

import "testing"

const queueTestReserve uint = 64 * 1024 * 1024

func newQueueTestArena(t *testing.T) *Arena {
	t.Helper()

	arena, err := NewArena(queueTestReserve)

	if err != nil {
		t.Fatalf("NewArena failed: %v", err)
	}

	t.Cleanup(arena.Destroy)

	return arena
}

func TestMakeQueue(t *testing.T) {
	arena := newQueueTestArena(t)

	queue, err := MakeQueue[int](arena, 8)

	if err != nil {
		t.Fatalf("MakeQueue returned error: %v", err)
	}

	if queue.Allocator != Allocator(arena) {
		t.Errorf("Allocator not stored")
	}
	if queue.Capacity != 8 {
		t.Errorf("Capacity = %d, want 8", queue.Capacity)
	}
	if len(queue.Items) != 8 {
		t.Errorf("len(Items) = %d, want 8", len(queue.Items))
	}
	if queue.Count != 0 || queue.Head != 0 || queue.Tail != 0 {
		t.Errorf("Count/Head/Tail = %d/%d/%d, want 0/0/0", queue.Count, queue.Head, queue.Tail)
	}
}

func TestMakeQueueOutOfMemory(t *testing.T) {
	arena, err := NewArena(4096)

	if err != nil {
		t.Fatalf("NewArena failed: %v", err)
	}

	t.Cleanup(arena.Destroy)

	_, err = MakeQueue[int](arena, 1024*1024)

	if err == nil {
		t.Fatalf("expected error when arena cannot satisfy allocation")
	}
}

func TestQueuePopEmpty(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	item, ok := queue.Pop()

	if ok {
		t.Errorf("Pop on empty queue returned ok = true")
	}
	if item != 0 {
		t.Errorf("Pop on empty queue returned %d, want zero value", item)
	}
}

func TestQueuePushPopSingle(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(42)

	if queue.Count != 1 {
		t.Errorf("Count = %d, want 1", queue.Count)
	}

	item, ok := queue.Pop()

	if !ok || item != 42 {
		t.Errorf("Pop = (%d, %v), want (42, true)", item, ok)
	}
	if queue.Count != 0 {
		t.Errorf("Count = %d, want 0", queue.Count)
	}

	_, ok = queue.Pop()

	if ok {
		t.Errorf("Pop after draining returned ok = true")
	}
}

func TestQueueFIFOOrder(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 16)

	for i := 0; i < 10; i++ {
		queue.Push(i)
	}

	for i := 0; i < 10; i++ {
		item, ok := queue.Pop()

		if !ok || item != i {
			t.Fatalf("Pop #%d = (%d, %v), want (%d, true)", i, item, ok, i)
		}
	}
}

func TestQueueCountTracking(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 8)

	for i := 1; i <= 5; i++ {
		queue.Push(i)

		if queue.Count != i {
			t.Errorf("after %d pushes Count = %d", i, queue.Count)
		}
	}

	for i := 4; i >= 0; i-- {
		queue.Pop()

		if queue.Count != i {
			t.Errorf("Count = %d, want %d", queue.Count, i)
		}
	}
}

func TestQueueWrapAround(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 8)

	// Advance head/tail so that subsequent pushes wrap around the buffer.
	for i := 0; i < 5; i++ {
		queue.Push(i)
	}
	for i := 0; i < 3; i++ {
		queue.Pop()
	}

	// Queue now holds 3, 4. Push more to wrap the tail.
	for i := 5; i < 9; i++ {
		queue.Push(i)
	}

	want := []int{3, 4, 5, 6, 7, 8}

	for _, w := range want {
		item, ok := queue.Pop()

		if !ok || item != w {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, w)
		}
	}
}

func TestQueueGrowthPreservesOrder(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 2)

	const n = 100

	for i := 0; i < n; i++ {
		queue.Push(i)
	}

	if queue.Count != n {
		t.Errorf("Count = %d, want %d", queue.Count, n)
	}
	if queue.Capacity < n {
		t.Errorf("Capacity = %d, want at least %d", queue.Capacity, n)
	}
	if len(queue.Items) != queue.Capacity {
		t.Errorf("len(Items) = %d, Capacity = %d", len(queue.Items), queue.Capacity)
	}

	for i := 0; i < n; i++ {
		item, ok := queue.Pop()

		if !ok || item != i {
			t.Fatalf("Pop #%d = (%d, %v), want (%d, true)", i, item, ok, i)
		}
	}
}

func TestQueueGrowthAfterWrapAround(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)
	queue.Pop()
	queue.Pop()
	queue.Push(4)
	queue.Push(5)
	queue.Push(6)
	// Force growth while head is not at index 0.
	for i := 7; i <= 20; i++ {
		queue.Push(i)
	}

	for want := 3; want <= 20; want++ {
		item, ok := queue.Pop()

		if !ok || item != want {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, want)
		}
	}

	if _, ok := queue.Pop(); ok {
		t.Errorf("queue should be empty")
	}
}

func TestQueueGrowthResetsHead(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(1)
	queue.Push(2)
	queue.Pop()

	for i := 3; i <= 10; i++ {
		queue.Push(i)
	}

	if queue.Head != 0 {
		t.Errorf("Head = %d after growth, want 0", queue.Head)
	}
}

func TestQueueResetIndicesWhenEmpty(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 8)

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)
	queue.Pop()
	queue.Pop()
	queue.Pop()

	if queue.Head != 0 || queue.Tail != 0 {
		t.Errorf("Head/Tail = %d/%d after draining, want 0/0", queue.Head, queue.Tail)
	}
}

func TestQueueReuseAfterDrain(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	for round := 0; round < 5; round++ {
		for i := 0; i < 3; i++ {
			queue.Push(round*10 + i)
		}
		for i := 0; i < 3; i++ {
			item, ok := queue.Pop()

			if !ok || item != round*10+i {
				t.Fatalf("round %d: Pop = (%d, %v), want (%d, true)", round, item, ok, round*10+i)
			}
		}
	}
}

func TestQueueInterleavedPushPop(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	next := 0
	expected := 0

	for i := 0; i < 200; i++ {
		queue.Push(next)
		next++
		queue.Push(next)
		next++

		item, ok := queue.Pop()

		if !ok || item != expected {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, expected)
		}

		expected++
	}

	for queue.Count > 0 {
		item, ok := queue.Pop()

		if !ok || item != expected {
			t.Fatalf("drain Pop = (%d, %v), want (%d, true)", item, ok, expected)
		}

		expected++
	}

	if expected != next {
		t.Errorf("popped %d items, pushed %d", expected, next)
	}
}

func TestQueueStructItems(t *testing.T) {
	type point struct {
		X, Y int32
		Tag  uint8
	}

	arena := newQueueTestArena(t)
	queue, err := MakeQueue[point](arena, 2)

	if err != nil {
		t.Fatalf("MakeQueue failed: %v", err)
	}

	in := []point{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}

	for _, p := range in {
		queue.Push(p)
	}

	for _, want := range in {
		got, ok := queue.Pop()

		if !ok || got != want {
			t.Fatalf("Pop = (%+v, %v), want (%+v, true)", got, ok, want)
		}
	}
}

func TestQueueFloatItems(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[float64](arena, 2)

	in := []float64{1.5, -2.25, 3.125, 0, 99.75}

	for _, v := range in {
		queue.Push(v)
	}

	for _, want := range in {
		got, ok := queue.Pop()

		if !ok || got != want {
			t.Fatalf("Pop = (%v, %v), want (%v, true)", got, ok, want)
		}
	}
}

func TestQueueWithPoolAllocator(t *testing.T) {
	pool, err := NewPool(1024 * 1024)

	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	t.Cleanup(pool.Destroy)

	queue, err := MakeQueue[int](pool, 2)

	if err != nil {
		t.Fatalf("MakeQueue failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		queue.Push(i)
	}

	for i := 0; i < 50; i++ {
		item, ok := queue.Pop()

		if !ok || item != i {
			t.Fatalf("Pop #%d = (%d, %v), want (%d, true)", i, item, ok, i)
		}
	}
}

func TestQueueStringEmpty(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	if got := queue.String(); got != "{ }" {
		t.Errorf("String() = %q, want %q", got, "{ }")
	}
}

func TestQueueStringSingle(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(7)

	if got := queue.String(); got != "[ 7 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 7 ]")
	}
}

func TestQueueStringMultiple(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 8)

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)

	if got := queue.String(); got != "[ 1, 2, 3 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 1, 2, 3 ]")
	}
}

func TestQueueStringAfterPop(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 8)

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)
	queue.Pop()

	if got := queue.String(); got != "[ 2, 3 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 2, 3 ]")
	}
}

func TestQueueStringAfterWrapAround(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(1)
	queue.Push(2)
	queue.Pop()
	queue.Pop()
	queue.Push(3)
	queue.Push(4)

	if got := queue.String(); got != "[ 3, 4 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 3, 4 ]")
	}
}

func TestQueueDestroyTempAllocator(t *testing.T) {
	arena := newQueueTestArena(t)
	queue, _ := MakeQueue[int](arena, 4)

	queue.Push(1)
	queue.Push(2)
	queue.Destroy()

	if queue.Allocator != nil {
		t.Errorf("Allocator not cleared")
	}
	if queue.Count != 0 || queue.Capacity != 0 || queue.Head != 0 || queue.Tail != 0 {
		t.Errorf("fields not reset: Count=%d Capacity=%d Head=%d Tail=%d",
			queue.Count, queue.Capacity, queue.Head, queue.Tail)
	}
}