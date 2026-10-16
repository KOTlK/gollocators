// I did not write a single character in this file by hand.
package allocators

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

const testReserve = 256 * 1024 * 1024

func newTestArena(t *testing.T) *Arena {
	t.Helper()

	arena, err := NewArena(testReserve)
	if err != nil {
		t.Fatalf("NewArena: %v", err)
	}
	t.Cleanup(arena.Destroy)

	return arena
}

func newTestPool(t *testing.T) *Pool {
	t.Helper()

	pool, err := NewPool(16 * 1024 * 1024)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Destroy)

	return pool
}

func newListOf[T any](t *testing.T, capacity int, values ...T) *List[T] {
	t.Helper()

	list, err := MakeList[T](newTestArena(t), capacity)
	if err != nil {
		t.Fatalf("MakeList: %v", err)
	}
	for _, v := range values {
		list.Append(v)
	}

	return &list
}

func newIntList(t *testing.T, capacity int, values ...int) *List[int] {
	t.Helper()

	return newListOf[int](t, capacity, values...)
}

func contents[T any](list *List[T]) []T {
	return list.Items[:list.Count]
}

func preview[T any](s []T) string {
	const limit = 20
	if len(s) <= limit {
		return fmt.Sprint(s)
	}
	return fmt.Sprintf("%v ... (%d items)", s[:limit], len(s))
}

func assertContents[T comparable](t *testing.T, list *List[T], want ...T) {
	t.Helper()

	got := contents(list)

	if list.Count != len(want) {
		t.Fatalf("Count = %d, want %d (items: %s)", list.Count, len(want), preview(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("items[%d] = %v, want %v\n got:  %s\n want: %s",
				i, got[i], want[i], preview(got), preview(want))
		}
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func assertNoPanic(t *testing.T, name string, fn func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: unexpected panic: %v", name, r)
		}
	}()
	fn()
}

// ---------------------------------------------------------------- MakeList

func TestMakeList(t *testing.T) {
	list, err := MakeList[int](newTestArena(t), 8)
	if err != nil {
		t.Fatalf("MakeList: %v", err)
	}

	if list.Count != 0 {
		t.Errorf("Count = %d, want 0", list.Count)
	}
	if list.Capacity != 8 {
		t.Errorf("Capacity = %d, want 8", list.Capacity)
	}
	if len(list.Items) != 8 {
		t.Errorf("len(Items) = %d, want 8", len(list.Items))
	}
	if list.Allocator == nil {
		t.Error("Allocator not set")
	}
}

func TestMakeListZeroCapacity(t *testing.T) {
	var list List[int]

	assertNoPanic(t, "MakeList with capacity 0", func() {
		list, _ = MakeList[int](newTestArena(t), 0)
	})

	if list.Count != 0 || list.Capacity != 0 || len(list.Items) != 0 {
		t.Fatalf("Count, Capacity, len(Items) = %d, %d, %d; want all 0",
			list.Count, list.Capacity, len(list.Items))
	}
}

func TestMakeListOutOfMemoryReturnError(t *testing.T) {
	arena, err := NewArena(4096)
	if err != nil {
		t.Fatalf("NewArena: %v", err)
	}
	t.Cleanup(arena.Destroy)
}

// ------------------------------------------------------------------ Append

func TestAppendReturnsIndexes(t *testing.T) {
	list := newIntList(t, 4)

	for i := 0; i < 10; i++ {
		if idx := list.Append(i * 10); idx != i {
			t.Fatalf("Append #%d returned index %d", i, idx)
		}
	}
}

func TestAppendStoresValues(t *testing.T) {
	list := newIntList(t, 16, 5, 6, 7)
	assertContents(t, list, 5, 6, 7)
}

func TestAppendGrowsAndKeepsData(t *testing.T) {
	const n = 5000
	list := newIntList(t, 2)

	for i := 0; i < n; i++ {
		list.Append(i)
	}

	if list.Count != n {
		t.Fatalf("Count = %d, want %d", list.Count, n)
	}
	if list.Capacity < n {
		t.Fatalf("Capacity = %d, want >= %d", list.Capacity, n)
	}
	if len(list.Items) != list.Capacity {
		t.Fatalf("len(Items) = %d, Capacity = %d", len(list.Items), list.Capacity)
	}
	for i := 0; i < n; i++ {
		if list.Items[i] != i {
			t.Fatalf("Items[%d] = %d, want %d", i, list.Items[i], i)
		}
	}
}

func TestAppendStructs(t *testing.T) {
	type vec struct {
		X, Y, Z float64
		Tag     string
	}

	list, err := MakeList[vec](newTestArena(t), 1)
	if err != nil {
		t.Fatal(err)
	}

	// NOTE: Tag holds a pointer, but arena memory is invisible to the GC.
	// Use a string literal so the data stays alive in the binary.
	for i := 0; i < 100; i++ {
		list.Append(vec{float64(i), float64(i) * 2, float64(i) * 3, "v"})
	}

	for i := 0; i < 100; i++ {
		want := vec{float64(i), float64(i) * 2, float64(i) * 3, "v"}
		if list.Items[i] != want {
			t.Fatalf("Items[%d] = %+v, want %+v", i, list.Items[i], want)
		}
	}
}

func TestAppendToZeroCapacityList(t *testing.T) {
	list := newIntList(t, 0)

	assertNoPanic(t, "Append on zero-capacity list", func() {
		list.Append(1)
		list.Append(2)
		list.Append(3)
	})

	assertContents(t, list, 1, 2, 3)
}

// ------------------------------------------------------------- AppendSlice

func TestAppendSlice(t *testing.T) {
	list := newIntList(t, 16, 1, 2)

	start, count := list.AppendSlice([]int{3, 4, 5})

	if start != 2 || count != 3 {
		t.Fatalf("start, count = %d, %d; want 2, 3", start, count)
	}
	assertContents(t, list, 1, 2, 3, 4, 5)
}

func TestAppendSliceEmpty(t *testing.T) {
	list := newIntList(t, 4, 1, 2)

	start, count := list.AppendSlice(nil)

	if start != 2 || count != 0 {
		t.Fatalf("start, count = %d, %d; want 2, 0", start, count)
	}
	assertContents(t, list, 1, 2)
}

func TestAppendSliceTriggersGrowth(t *testing.T) {
	list := newIntList(t, 4, 1, 2, 3)

	assertNoPanic(t, "AppendSlice past capacity", func() {
		list.AppendSlice([]int{4, 5, 6})
	})

	if list.Capacity < 6 {
		t.Fatalf("Capacity = %d, want >= 6", list.Capacity)
	}
	assertContents(t, list, 1, 2, 3, 4, 5, 6)
}

func TestAppendSliceLargerThanCapacity(t *testing.T) {
	list := newIntList(t, 4)

	big := make([]int, 100)
	for i := range big {
		big[i] = i
	}

	assertNoPanic(t, "AppendSlice much larger than capacity", func() {
		list.AppendSlice(big)
	})

	assertContents(t, list, big...)
}

func TestAppendSliceToZeroCapacityList(t *testing.T) {
	list := newIntList(t, 0)

	assertNoPanic(t, "AppendSlice(nil) on zero-capacity list", func() {
		list.AppendSlice(nil)
	})
	assertNoPanic(t, "AppendSlice on zero-capacity list", func() {
		list.AppendSlice([]int{1, 2, 3})
	})

	assertContents(t, list, 1, 2, 3)
}

func TestAppendSliceThenAppend(t *testing.T) {
	list := newIntList(t, 2)

	list.AppendSlice([]int{1, 2, 3})
	idx := list.Append(4)

	if idx != 3 {
		t.Fatalf("Append index = %d, want 3", idx)
	}
	assertContents(t, list, 1, 2, 3, 4)
}

// ---------------------------------------------------------------- Contains

func TestContains(t *testing.T) {
	list := newIntList(t, 8, 10, 20, 30, 20)

	tests := []struct {
		name      string
		item      int
		wantFound bool
		wantIndex int
	}{
		{"first", 10, true, 0},
		{"middle", 20, true, 1}, // first occurrence
		{"last unique", 30, true, 2},
		{"missing", 99, false, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found, idx := Contains(list, tc.item)
			if found != tc.wantFound || idx != tc.wantIndex {
				t.Fatalf("Contains(%d) = %v, %d; want %v, %d",
					tc.item, found, idx, tc.wantFound, tc.wantIndex)
			}
		})
	}
}

func TestContainsIgnoresStaleItemsPastCount(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)
	list.RemoveAt(2) // 3 still sits in Items[2], but Count is 2

	if found, _ := Contains(list, 3); found {
		t.Fatal("Contains found an element past Count")
	}
}

func TestContainsEmpty(t *testing.T) {
	list := newIntList(t, 4)

	if found, _ := Contains(list, 1); found {
		t.Fatal("Contains found element in empty list")
	}
}

func TestContainsStrings(t *testing.T) {
	list, err := MakeList[string](newTestArena(t), 4)
	if err != nil {
		t.Fatal(err)
	}
	list.Append("a")
	list.Append("b")

	if found, idx := Contains(&list, "b"); !found || idx != 1 {
		t.Fatalf("Contains(\"b\") = %v, %d; want true, 1", found, idx)
	}
}

// ------------------------------------------------------------------ Remove

func TestRemoveSwapsLastIntoPlace(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3, 4)

	if !Remove(list, 2) {
		t.Fatal("Remove returned false")
	}

	assertContents(t, list, 1, 4, 3)
}

func TestRemoveLastElement(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	if !Remove(list, 3) {
		t.Fatal("Remove returned false")
	}

	assertContents(t, list, 1, 2)
}

func TestRemoveOnlyElement(t *testing.T) {
	list := newIntList(t, 4, 7)

	if !Remove(list, 7) {
		t.Fatal("Remove returned false")
	}

	assertContents(t, list)
}

func TestRemoveMissing(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	if Remove(list, 99) {
		t.Fatal("Remove returned true for missing item")
	}

	assertContents(t, list, 1, 2, 3)
}

func TestRemoveEmpty(t *testing.T) {
	list := newIntList(t, 4)

	if Remove(list, 1) {
		t.Fatal("Remove returned true on empty list")
	}
}

func TestRemoveOnlyFirstOccurrence(t *testing.T) {
	list := newIntList(t, 8, 5, 1, 5, 2)

	Remove(list, 5)

	// first 5 replaced by last element (2)
	assertContents(t, list, 2, 1, 5)
}

// --------------------------------------------------------------- RemoveAll

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name        string
		input       []int
		item        int
		wantRemoved int
		wantItems   []int // order is unspecified (swap-remove), so compared sorted
	}{
		{"none present", []int{1, 2, 3}, 9, 0, []int{1, 2, 3}},
		{"one", []int{1, 2, 3}, 2, 1, []int{1, 3}},
		{"several scattered", []int{2, 1, 2, 3, 2, 4}, 2, 3, []int{1, 3, 4}},
		{"adjacent at end", []int{1, 2, 2, 2}, 2, 3, []int{1}},
		{"adjacent at start", []int{2, 2, 2, 1}, 2, 3, []int{1}},
		{"all elements", []int{2, 2, 2}, 2, 3, []int{}},
		{"single match", []int{2}, 2, 1, []int{}},
		{"empty list", []int{}, 2, 0, []int{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list := newIntList(t, 8, tc.input...)

			ok, removed := RemoveAll(list, tc.item)

			if removed != tc.wantRemoved {
				t.Fatalf("removed = %d, want %d", removed, tc.wantRemoved)
			}
			if ok != (tc.wantRemoved > 0) {
				t.Fatalf("ok = %v, want %v", ok, tc.wantRemoved > 0)
			}

			got := slices.Clone(contents(list))
			slices.Sort(got)
			want := slices.Clone(tc.wantItems)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Fatalf("items = %v, want %v", got, want)
			}
			if f, _ := Contains(list, tc.item); f && tc.wantRemoved > 0 {
				t.Fatalf("item %d still present after RemoveAll", tc.item)
			}
		})
	}
}

// ---------------------------------------------------------------- RemoveAt

func TestRemoveAtMiddle(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3, 4)

	list.RemoveAt(1)

	assertContents(t, list, 1, 4, 3)
}

func TestRemoveAtFirst(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	list.RemoveAt(0)

	assertContents(t, list, 3, 2)
}

func TestRemoveAtLast(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	list.RemoveAt(2)

	assertContents(t, list, 1, 2)
}

func TestRemoveAtOnlyElement(t *testing.T) {
	list := newIntList(t, 4, 1)

	list.RemoveAt(0)

	assertContents(t, list)
}

func TestRemoveAtOutOfRangePanics(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	assertPanics(t, "index == Count", func() { list.RemoveAt(3) })
	assertPanics(t, "index > Count", func() { list.RemoveAt(100) })
	assertContents(t, list, 1, 2, 3)
}

func TestRemoveAtNegativeIndexPanicsWithoutCorruptingList(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	assertPanics(t, "negative index", func() { list.RemoveAt(-1) })

	// A rejected index must leave the list untouched.
	assertContents(t, list, 1, 2, 3)
}

func TestRemoveAtEmptyPanics(t *testing.T) {
	list := newIntList(t, 4)

	assertPanics(t, "RemoveAt on empty list", func() { list.RemoveAt(0) })
}

// -------------------------------------------------------------------- Sort

func TestSort(t *testing.T) {
	tests := []struct {
		name  string
		input []int
	}{
		{"empty", nil},
		{"single", []int{1}},
		{"two sorted", []int{1, 2}},
		{"two reversed", []int{2, 1}},
		{"already sorted", []int{1, 2, 3, 4, 5}},
		{"reversed", []int{5, 4, 3, 2, 1}},
		{"duplicates", []int{3, 1, 3, 2, 1, 3}},
		{"all equal", []int{7, 7, 7, 7}},
		{"negatives", []int{-3, 5, -1, 0, -10, 4}},
		{"extremes", []int{math.MaxInt, 0, math.MinInt, -1, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list := newIntList(t, 4, tc.input...)

			SortList(list)

			want := slices.Clone(tc.input)
			slices.Sort(want)
			assertContents(t, list, want...)
		})
	}
}

func TestSortOnlyTouchesLiveItems(t *testing.T) {
	list := newIntList(t, 8, 5, 6, 1)
	list.RemoveAt(2) // Count is 2, but Items[2] still holds a stale 1

	SortList(list)

	assertContents(t, list, 5, 6)
	if list.Items[2] != 1 {
		t.Fatalf("Sort modified Items[Count]: got %d, want 1", list.Items[2])
	}
}

func TestSortOnZeroCapacityList(t *testing.T) {
	list := newIntList(t, 0)

	assertNoPanic(t, "Sort on empty zero-capacity list", func() {
		SortList(list)
	})
}

func TestSortKeepsCountAndCapacity(t *testing.T) {
	list := newIntList(t, 16, 3, 1, 2)
	count, capacity := list.Count, list.Capacity

	SortList(list)

	if list.Count != count || list.Capacity != capacity {
		t.Fatalf("Count, Capacity = %d, %d; want %d, %d",
			list.Count, list.Capacity, count, capacity)
	}
}

func TestSortIsIdempotent(t *testing.T) {
	list := newIntList(t, 4, 4, 2, 9, 1, 2)

	SortList(list)
	first := slices.Clone(contents(list))
	SortList(list)

	assertContents(t, list, first...)
}

func TestSortRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))

	values := make([]int, 10000)
	for i := range values {
		values[i] = r.IntN(2000) - 1000 // plenty of duplicates and negatives
	}

	list := newIntList(t, 4, values...)
	SortList(list)

	slices.Sort(values)
	assertContents(t, list, values...)
}

func TestSortPatterns(t *testing.T) {
	const n = 2000

	patterns := map[string]func(i int) int{
		"ascending":  func(i int) int { return i },
		"descending": func(i int) int { return n - i },
		"all equal":  func(i int) int { return 42 },
		"sawtooth":   func(i int) int { return i % 10 },
	}

	for name, gen := range patterns {
		t.Run(name, func(t *testing.T) {
			values := make([]int, n)
			for i := range values {
				values[i] = gen(i)
			}

			list := newIntList(t, 4, values...)
			SortList(list)

			slices.Sort(values)
			assertContents(t, list, values...)
		})
	}
}

func TestSortAfterRemovals(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	list := newIntList(t, 4)

	for i := 0; i < 500; i++ {
		list.Append(r.IntN(100))
	}
	RemoveAll(list, 50)
	for i := 0; i < 20; i++ {
		list.RemoveAt(0)
	}

	want := slices.Clone(contents(list))
	slices.Sort(want)

	SortList(list)

	assertContents(t, list, want...)
}

func checkSortsLikeSlices[T quick_sort_compatible](t *testing.T, name string, values []T) {
	t.Run(name, func(t *testing.T) {
		list := newListOf[T](t, 4, values...)

		SortList(list)

		want := slices.Clone(values)
		slices.Sort(want)
		assertContents(t, list, want...)
	})
}

func TestSortAllSupportedTypes(t *testing.T) {
	checkSortsLikeSlices(t, "int8", []int8{127, -128, 0, -1, 1, 5, -5, 127})
	checkSortsLikeSlices(t, "uint8", []uint8{255, 0, 128, 1, 254, 7, 255})
	checkSortsLikeSlices(t, "int16", []int16{math.MaxInt16, math.MinInt16, 0, -1, 300, -300})
	checkSortsLikeSlices(t, "uint16", []uint16{math.MaxUint16, 0, 300, 1, 65534})
	checkSortsLikeSlices(t, "int32", []int32{math.MaxInt32, math.MinInt32, 0, -1, 70000, -70000})
	checkSortsLikeSlices(t, "uint32", []uint32{math.MaxUint32, 0, 70000, 1, 4294967294})
	checkSortsLikeSlices(t, "int64", []int64{math.MaxInt64, math.MinInt64, 0, -1, 1 << 40, -(1 << 40)})
	checkSortsLikeSlices(t, "uint64", []uint64{math.MaxUint64, 0, 1 << 40, 1, math.MaxUint64 - 1})
	checkSortsLikeSlices(t, "int", []int{math.MaxInt, math.MinInt, 0, -1, 1, 9, -9})
	checkSortsLikeSlices(t, "uint", []uint{math.MaxUint, 0, 5, 1, 4, math.MaxUint - 1})
	checkSortsLikeSlices(t, "uintptr", []uintptr{5, 1, 3, 0, 4, 2})
}

// ------------------------------------------------------------------ String

func TestString(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  string
	}{
		// Empty uses braces while non-empty uses brackets; this pins the current output.
		{"empty", nil, "{ }"},
		{"single", []int{1}, "[ 1 ]"},
		{"several", []int{1, 2, 3}, "[ 1, 2, 3 ]"},
		{"negatives", []int{-1, 2, -3}, "[ -1, 2, -3 ]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list := newIntList(t, 4, tc.input...)

			if got := list.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStringIgnoresStaleItemsPastCount(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)
	list.RemoveAt(2)

	if got, want := list.String(), "[ 1, 2 ]"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringOfStrings(t *testing.T) {
	list := newListOf[string](t, 4, "a", "b")

	if got, want := list.String(), "[ a, b ]"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringWorksWithFmt(t *testing.T) {
	list := newIntList(t, 4, 1, 2, 3)
	want := "[ 1, 2, 3 ]"

	// if got := fmt.Sprint(*list); got != want {
	// 	t.Errorf("Sprint(value) = %q, want %q", got, want)
	// }
	if got := fmt.Sprint(list); got != want {
		t.Errorf("Sprint(pointer) = %q, want %q", got, want)
	}
	// if got := fmt.Sprintf("%v", *list); got != want {
	// 	t.Errorf("%%v = %q, want %q", got, want)
	// }
	if got := fmt.Sprintf("%s", list); got != want {
		t.Errorf("%%s = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------- Integration

func TestAppendAfterRemoveReusesSlots(t *testing.T) {
	list := newIntList(t, 8, 1, 2, 3)

	list.RemoveAt(0)
	idx := list.Append(9)

	if idx != 2 {
		t.Fatalf("Append index = %d, want 2", idx)
	}
	assertContents(t, list, 3, 2, 9)
}

func TestGrowthAfterRemovals(t *testing.T) {
	list := newIntList(t, 4)

	for round := 0; round < 50; round++ {
		for i := 0; i < 20; i++ {
			list.Append(i)
		}
		for i := 0; i < 10; i++ {
			list.RemoveAt(0)
		}
	}

	if list.Count != 500 {
		t.Fatalf("Count = %d, want 500", list.Count)
	}
}

// ------------------------------------------------------------- With a Pool

func TestListWithPool(t *testing.T) {
	list, err := MakeList[int](newTestPool(t), 4)
	if err != nil {
		t.Fatalf("MakeList: %v", err)
	}

	for i := 0; i < 10000; i++ {
		list.Append(i)
	}

	if list.Count != 10000 {
		t.Fatalf("Count = %d", list.Count)
	}
	for i := 0; i < 10000; i++ {
		if list.Items[i] != i {
			t.Fatalf("Items[%d] = %d", i, list.Items[i])
		}
	}

	if found, idx := Contains(&list, 9999); !found || idx != 9999 {
		t.Fatalf("Contains(9999) = %v, %d", found, idx)
	}

	ok, removed := RemoveAll(&list, 5)
	if !ok || removed != 1 {
		t.Fatalf("RemoveAll = %v, %d", ok, removed)
	}
}

func TestSortWithPool(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))

	list, err := MakeList[int](newTestPool(t), 4)
	if err != nil {
		t.Fatalf("MakeList: %v", err)
	}

	values := make([]int, 5000)
	for i := range values {
		values[i] = r.IntN(1000)
		list.Append(values[i])
	}

	SortList(&list)

	slices.Sort(values)
	assertContents(t, &list, values...)
}

func TestTwoListsShareOneArena(t *testing.T) {
	arena := newTestArena(t)

	a, err := MakeList[int](arena, 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := MakeList[int](arena, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Interleaved growth must not make the lists overwrite each other.
	for i := 0; i < 1000; i++ {
		a.Append(i)
		b.Append(-i)
	}

	for i := 0; i < 1000; i++ {
		if a.Items[i] != i || b.Items[i] != -i {
			t.Fatalf("lists corrupted at %d: a=%d b=%d", i, a.Items[i], b.Items[i])
		}
	}
}