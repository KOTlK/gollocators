package allocators

import "testing"

const stackTestReserve uint = 64 * 1024 * 1024

func newStackTestArena(t *testing.T) *Arena {
	t.Helper()

	arena, err := NewArena(stackTestReserve)

	if err != nil {
		t.Fatalf("NewArena failed: %v", err)
	}

	t.Cleanup(arena.Destroy)

	return arena
}

func TestMakeStack(t *testing.T) {
	arena := newStackTestArena(t)

	stack, err := MakeStack[int](arena, 8)

	if err != nil {
		t.Fatalf("MakeStack returned error: %v", err)
	}

	if stack.Allocator != Allocator(arena) {
		t.Errorf("Allocator not stored")
	}
	if stack.Capacity != 8 {
		t.Errorf("Capacity = %d, want 8", stack.Capacity)
	}
	if len(stack.Items) != 8 {
		t.Errorf("len(Items) = %d, want 8", len(stack.Items))
	}
	if stack.Count != 0 {
		t.Errorf("Count = %d, want 0", stack.Count)
	}
}

func TestMakeStackOutOfMemory(t *testing.T) {
	arena, err := NewArena(4096)

	if err != nil {
		t.Fatalf("NewArena failed: %v", err)
	}

	t.Cleanup(arena.Destroy)

	_, err = MakeStack[int](arena, 1024*1024)

	if err == nil {
		t.Fatalf("expected error when arena cannot satisfy allocation")
	}
}

func TestStackPopEmpty(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	item, ok := stack.Pop()

	if ok {
		t.Errorf("Pop on empty stack returned ok = true")
	}
	if item != 0 {
		t.Errorf("Pop on empty stack returned %d, want zero value", item)
	}
	if stack.Count != 0 {
		t.Errorf("Count = %d, want 0", stack.Count)
	}
}

func TestStackPushPopSingle(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	stack.Push(42)

	if stack.Count != 1 {
		t.Errorf("Count = %d, want 1", stack.Count)
	}

	item, ok := stack.Pop()

	if !ok || item != 42 {
		t.Errorf("Pop = (%d, %v), want (42, true)", item, ok)
	}
	if stack.Count != 0 {
		t.Errorf("Count = %d, want 0", stack.Count)
	}

	if _, ok = stack.Pop(); ok {
		t.Errorf("Pop after draining returned ok = true")
	}
}

func TestStackLIFOOrder(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 16)

	for i := 0; i < 10; i++ {
		stack.Push(i)
	}

	for i := 9; i >= 0; i-- {
		item, ok := stack.Pop()

		if !ok || item != i {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, i)
		}
	}
}

func TestStackCountTracking(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 8)

	for i := 1; i <= 5; i++ {
		stack.Push(i)

		if stack.Count != i {
			t.Errorf("after %d pushes Count = %d", i, stack.Count)
		}
	}

	for i := 4; i >= 0; i-- {
		stack.Pop()

		if stack.Count != i {
			t.Errorf("Count = %d, want %d", stack.Count, i)
		}
	}
}

func TestStackFillToExactCapacity(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	for i := 0; i < 4; i++ {
		stack.Push(i)
	}

	if stack.Capacity != 4 {
		t.Errorf("Capacity = %d, want 4 (no growth expected when exactly full)", stack.Capacity)
	}
	if stack.Count != 4 {
		t.Errorf("Count = %d, want 4", stack.Count)
	}
}

func TestStackGrowthPreservesItems(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 2)

	const n = 100

	for i := 0; i < n; i++ {
		stack.Push(i)
	}

	if stack.Count != n {
		t.Errorf("Count = %d, want %d", stack.Count, n)
	}
	if stack.Capacity < n {
		t.Errorf("Capacity = %d, want at least %d", stack.Capacity, n)
	}
	if len(stack.Items) != stack.Capacity {
		t.Errorf("len(Items) = %d, Capacity = %d", len(stack.Items), stack.Capacity)
	}

	for i := n - 1; i >= 0; i-- {
		item, ok := stack.Pop()

		if !ok || item != i {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, i)
		}
	}
}

func TestStackGrowthCapacity(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 2)

	stack.Push(1)
	stack.Push(2)
	stack.Push(3) // triggers growth: (2 + 1) * 2 = 6

	if stack.Capacity != 6 {
		t.Errorf("Capacity = %d, want 6", stack.Capacity)
	}
}

func TestStackInterleavedPushPop(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 2)

	stack.Push(1)
	stack.Push(2)

	if item, _ := stack.Pop(); item != 2 {
		t.Errorf("Pop = %d, want 2", item)
	}

	stack.Push(3)
	stack.Push(4)
	stack.Push(5)

	want := []int{5, 4, 3, 1}

	for _, w := range want {
		item, ok := stack.Pop()

		if !ok || item != w {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, w)
		}
	}

	if _, ok := stack.Pop(); ok {
		t.Errorf("stack should be empty")
	}
}

func TestStackReuseAfterDrain(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	for round := 0; round < 5; round++ {
		for i := 0; i < 3; i++ {
			stack.Push(round*10 + i)
		}
		for i := 2; i >= 0; i-- {
			item, ok := stack.Pop()

			if !ok || item != round*10+i {
				t.Fatalf("round %d: Pop = (%d, %v), want (%d, true)", round, item, ok, round*10+i)
			}
		}
	}
}

func TestStackStructItems(t *testing.T) {
	type point struct {
		X, Y int32
		Tag  uint8
	}

	arena := newStackTestArena(t)
	stack, err := MakeStack[point](arena, 2)

	if err != nil {
		t.Fatalf("MakeStack failed: %v", err)
	}

	in := []point{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}

	for _, p := range in {
		stack.Push(p)
	}

	for i := len(in) - 1; i >= 0; i-- {
		got, ok := stack.Pop()

		if !ok || got != in[i] {
			t.Fatalf("Pop = (%+v, %v), want (%+v, true)", got, ok, in[i])
		}
	}
}

func TestStackFloatItems(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[float64](arena, 2)

	in := []float64{1.5, -2.25, 3.125, 0, 99.75}

	for _, v := range in {
		stack.Push(v)
	}

	for i := len(in) - 1; i >= 0; i-- {
		got, ok := stack.Pop()

		if !ok || got != in[i] {
			t.Fatalf("Pop = (%v, %v), want (%v, true)", got, ok, in[i])
		}
	}
}

func TestStackWithPoolAllocator(t *testing.T) {
	pool, err := NewPool(1024 * 1024)

	if err != nil {
		t.Fatalf("NewPool failed: %v", err)
	}

	t.Cleanup(pool.Destroy)

	stack, err := MakeStack[int](pool, 2)

	if err != nil {
		t.Fatalf("MakeStack failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		stack.Push(i)
	}

	for i := 49; i >= 0; i-- {
		item, ok := stack.Pop()

		if !ok || item != i {
			t.Fatalf("Pop = (%d, %v), want (%d, true)", item, ok, i)
		}
	}
}

func TestStackStringEmpty(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	if got := stack.String(); got != "{ }" {
		t.Errorf("String() = %q, want %q", got, "{ }")
	}
}

func TestStackStringSingle(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	stack.Push(7)

	if got := stack.String(); got != "[ 7 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 7 ]")
	}
}

func TestStackStringMultiple(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 8)

	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	if got := stack.String(); got != "[ 1, 2, 3 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 1, 2, 3 ]")
	}
}

func TestStackStringAfterPop(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 8)

	stack.Push(1)
	stack.Push(2)
	stack.Push(3)
	stack.Pop()

	if got := stack.String(); got != "[ 1, 2 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 1, 2 ]")
	}
}

func TestStackStringAfterGrowth(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 1)

	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	if got := stack.String(); got != "[ 1, 2, 3 ]" {
		t.Errorf("String() = %q, want %q", got, "[ 1, 2, 3 ]")
	}
}

func TestStackDestroyTempAllocator(t *testing.T) {
	arena := newStackTestArena(t)
	stack, _ := MakeStack[int](arena, 4)

	stack.Push(1)
	stack.Push(2)
	stack.Destroy()

	if stack.Allocator != nil {
		t.Errorf("Allocator not cleared")
	}
	if stack.Count != 0 || stack.Capacity != 0 {
		t.Errorf("fields not reset: Count=%d Capacity=%d", stack.Count, stack.Capacity)
	}
}