package allocators

import (
	"fmt"
	"errors"
	"strings"
	"testing"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testAllocator hands out zeroed, 8-byte aligned memory from the Go heap and
// keeps every buffer alive so the GC never reclaims memory the table still uses.
type testAllocator struct {
	temp        bool
	bufs        [][]uint64
	mallocCalls int
	freeCalls   int
	failAfter   int // fail every Malloc once mallocCalls > failAfter (if >= 0)
}

func newTestAllocator(temp bool) *testAllocator {
	return &testAllocator{temp: temp, failAfter: -1}
}

func (a *testAllocator) Malloc(size uint) (unsafe.Pointer, error) {
	a.mallocCalls++

	if a.failAfter >= 0 && a.mallocCalls > a.failAfter {
		return nil, errors.New("testAllocator: forced failure")
	}

	words := (size + 7) / 8
	if words == 0 {
		words = 1
	}

	buf := make([]uint64, words)
	a.bufs = append(a.bufs, buf)

	return unsafe.Pointer(&buf[0]), nil
}

func (a *testAllocator) Free(data unsafe.Pointer) error {
	a.freeCalls++
	return nil
}

func (a *testAllocator) FreeAll()      {}
func (a *testAllocator) IsTemp() bool  { return a.temp }

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	f()
}

func mustNotPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: unexpected panic: %v", name, r)
		}
	}()
	f()
}

func newIntTable(t *testing.T, capacity uint) (HashTable[int, int], *testAllocator) {
	t.Helper()
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[int, int](alloc, capacity)
	if err != nil {
		t.Fatalf("MakeHashTable: %v", err)
	}
	return table, alloc
}

// Hashable key type.
var getHashCalls int

type hashableKey struct{ id int }

func (k hashableKey) GetHash() uint64 {
	getHashCalls++
	return uint64(k.id) + 100
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestMakeHashTable_InitialState(t *testing.T) {
	table, _ := newIntTable(t, 16)

	if table.Capacity != 16 {
		t.Errorf("Capacity = %d, want 16", table.Capacity)
	}
	if table.Count != 0 {
		t.Errorf("Count = %d, want 0", table.Count)
	}
	if len(table.Items) != 16 {
		t.Errorf("len(Items) = %d, want 16", len(table.Items))
	}
	if table.Hashfunc == nil {
		t.Error("Hashfunc is nil")
	}
	for i, item := range table.Items {
		if item.hash != 0 {
			t.Errorf("Items[%d].hash = %d, want 0 (free)", i, item.hash)
		}
	}
}

func TestMakeHashTable_AllocatorError(t *testing.T) {
	alloc := newTestAllocator(true)
	alloc.failAfter = 0

	if _, err := MakeHashTable[int, int](alloc, 8); err == nil {
		t.Error("MakeHashTable: expected error from failing allocator")
	}
	if _, err := MakeHashTableWithCustomHashFunc[int, int](alloc, 8, hashint); err == nil {
		t.Error("MakeHashTableWithCustomHashFunc: expected error from failing allocator")
	}
}

func TestMakeHashTableWithCustomHashFunc_UsesGivenFunc(t *testing.T) {
	alloc := newTestAllocator(true)
	calls := 0
	fn := func(k int) uint64 {
		calls++
		return uint64(k) + 2
	}

	table, err := MakeHashTableWithCustomHashFunc[int, int](alloc, 8, fn)
	if err != nil {
		t.Fatal(err)
	}

	table.Insert(1, 10)
	table.Get(1)
	table.Remove(1)

	if calls < 3 {
		t.Errorf("custom hash func called %d times, want at least 3", calls)
	}
}

func TestMakeHashTable_HashableKeyUsesGetHash(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[hashableKey, int](alloc, 8)
	if err != nil {
		t.Fatal(err)
	}

	getHashCalls = 0
	table.Insert(hashableKey{1}, 11)

	if getHashCalls == 0 {
		t.Error("GetHash was not called for a Hashable key")
	}
	if v, ok := table.Get(hashableKey{1}); !ok || v != 11 {
		t.Errorf("Get = (%d, %v), want (11, true)", v, ok)
	}
}

// ---------------------------------------------------------------------------
// Insert / Get
// ---------------------------------------------------------------------------

func TestInsertAndGet(t *testing.T) {
	table, _ := newIntTable(t, 16)

	for i := 1; i <= 5; i++ {
		if !table.Insert(i, i*10) {
			t.Fatalf("Insert(%d) returned false", i)
		}
	}

	if table.Count != 5 {
		t.Errorf("Count = %d, want 5", table.Count)
	}

	for i := 1; i <= 5; i++ {
		v, ok := table.Get(i)
		if !ok || v != i*10 {
			t.Errorf("Get(%d) = (%d, %v), want (%d, true)", i, v, ok, i*10)
		}
	}
}

func TestGet_MissingKey(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)

	v, ok := table.Get(999)
	if ok || v != 0 {
		t.Errorf("Get(missing) = (%d, %v), want (0, false)", v, ok)
	}
}

func TestGet_EmptyTable(t *testing.T) {
	table, _ := newIntTable(t, 16)

	if _, ok := table.Get(1); ok {
		t.Error("Get on empty table returned ok = true")
	}
}

func TestInsert_DuplicateKeyUpdatesValue(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(7, 1)
	table.Insert(7, 2)

	if v, ok := table.Get(7); !ok || v != 2 {
		t.Errorf("Get(7) = (%d, %v), want (2, true)", v, ok)
	}
	if table.Count != 1 {
		t.Errorf("Count = %d after inserting the same key twice, want 1", table.Count)
	}
}

func TestInsert_DuplicateKeyDoesNotCreateSecondEntry(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(7, 1)
	table.Insert(7, 2)

	live := 0
	for _, item := range table.Items {
		if item.hash > 1 {
			live++
		}
	}
	if live != 1 {
		t.Errorf("%d live slots for a single key, want 1", live)
	}
}

func TestInsert_ZeroKeyAndZeroValue(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(0, 0)

	if v, ok := table.Get(0); !ok || v != 0 {
		t.Errorf("Get(0) = (%d, %v), want (0, true)", v, ok)
	}
	if table.Count != 1 {
		t.Errorf("Count = %d, want 1", table.Count)
	}
}

func TestInsert_NegativeKeys(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(-1, 100)
	table.Insert(-50, 200)

	if v, ok := table.Get(-1); !ok || v != 100 {
		t.Errorf("Get(-1) = (%d, %v)", v, ok)
	}
	if v, ok := table.Get(-50); !ok || v != 200 {
		t.Errorf("Get(-50) = (%d, %v)", v, ok)
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove_Existing(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)
	table.Insert(2, 20)

	if !table.Remove(1) {
		t.Fatal("Remove(1) returned false")
	}
	if table.Count != 1 {
		t.Errorf("Count = %d, want 1", table.Count)
	}
	if _, ok := table.Get(1); ok {
		t.Error("Get(1) still found after Remove")
	}
	if v, ok := table.Get(2); !ok || v != 20 {
		t.Errorf("Get(2) = (%d, %v), want (20, true)", v, ok)
	}
}

func TestRemove_Missing(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)

	if table.Remove(42) {
		t.Error("Remove(missing) returned true")
	}
	if table.Count != 1 {
		t.Errorf("Count = %d, want 1", table.Count)
	}
}

func TestRemove_Twice(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)

	if !table.Remove(1) {
		t.Fatal("first Remove returned false")
	}
	if table.Remove(1) {
		t.Error("second Remove returned true")
	}
	if table.Count != 0 {
		t.Errorf("Count = %d, want 0", table.Count)
	}
}

func TestRemove_FromEmptyTable(t *testing.T) {
	table, _ := newIntTable(t, 16)

	if table.Remove(1) {
		t.Error("Remove on empty table returned true")
	}
}

func TestRemove_ThenReinsert(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(5, 50)
	table.Remove(5)
	table.Insert(5, 55)

	if v, ok := table.Get(5); !ok || v != 55 {
		t.Errorf("Get(5) = (%d, %v), want (55, true)", v, ok)
	}
	if table.Count != 1 {
		t.Errorf("Count = %d, want 1", table.Count)
	}
}

func TestRemove_DoesNotBreakProbeChain(t *testing.T) {
	alloc := newTestAllocator(true)
	// Constant hash forces every key into the same probe sequence.
	table, err := MakeHashTableWithCustomHashFunc[int, int](alloc, 32, func(int) uint64 { return 42 })
	if err != nil {
		t.Fatal(err)
	}

	table.Insert(1, 10)
	table.Insert(2, 20)
	table.Insert(3, 30)

	table.Remove(2)

	if v, ok := table.Get(3); !ok || v != 30 {
		t.Errorf("Get(3) after removing middle of chain = (%d, %v), want (30, true)", v, ok)
	}
	if v, ok := table.Get(1); !ok || v != 10 {
		t.Errorf("Get(1) = (%d, %v), want (10, true)", v, ok)
	}
}

// ---------------------------------------------------------------------------
// Collisions
// ---------------------------------------------------------------------------

func TestCollisions_ConstantHash(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTableWithCustomHashFunc[int, int](alloc, 64, func(int) uint64 { return 1234 })
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		if !table.Insert(i, i+1000) {
			t.Fatalf("Insert(%d) returned false", i)
		}
	}
	for i := 0; i < 20; i++ {
		if v, ok := table.Get(i); !ok || v != i+1000 {
			t.Errorf("Get(%d) = (%d, %v), want (%d, true)", i, v, ok, i+1000)
		}
	}
}

// ---------------------------------------------------------------------------
// Growth / reallocation
// ---------------------------------------------------------------------------

func TestGrowth_PreservesAllItems(t *testing.T) {
	table, _ := newIntTable(t, 4)
	const n = 1000

	for i := 1; i <= n; i++ {
		if !table.Insert(i, i*2) {
			t.Fatalf("Insert(%d) returned false", i)
		}
	}

	if table.Count != n {
		t.Errorf("Count = %d, want %d", table.Count, n)
	}
	if table.Capacity <= 4 {
		t.Errorf("Capacity = %d, expected growth beyond 4", table.Capacity)
	}
	for i := 1; i <= n; i++ {
		if v, ok := table.Get(i); !ok || v != i*2 {
			t.Fatalf("Get(%d) = (%d, %v), want (%d, true)", i, v, ok, i*2)
		}
	}
}

func TestGrowth_CapacityMatchesItemsLength(t *testing.T) {
	table, _ := newIntTable(t, 4)

	for i := 1; i <= 100; i++ {
		table.Insert(i, i)
	}

	if uint(len(table.Items)) != table.Capacity {
		t.Errorf("len(Items) = %d, Capacity = %d; should match", len(table.Items), table.Capacity)
	}
}

func TestGrowth_KeepsLoadFactorBelowThreshold(t *testing.T) {
	table, _ := newIntTable(t, 8)

	for i := 1; i <= 500; i++ {
		table.Insert(i, i)
		if table.Count*100/table.Capacity >= 100 {
			t.Fatalf("table full after %d inserts (count=%d cap=%d)", i, table.Count, table.Capacity)
		}
	}
}

func TestGrowth_FreesOldItemsForNonTempAllocator(t *testing.T) {
	alloc := newTestAllocator(false)
	table, err := MakeHashTable[int, int](alloc, 4)
	if err != nil {
		t.Fatal(err)
	}

	mallocsBefore := alloc.mallocCalls
	for i := 1; i <= 100; i++ {
		table.Insert(i, i)
	}

	reallocs := alloc.mallocCalls - mallocsBefore
	if reallocs == 0 {
		t.Fatal("expected at least one reallocation")
	}
	if alloc.freeCalls != reallocs {
		t.Errorf("Free called %d times, want %d (once per reallocation)", alloc.freeCalls, reallocs)
	}
}

func TestGrowth_DoesNotFreeForTempAllocator(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[int, int](alloc, 4)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 100; i++ {
		table.Insert(i, i)
	}

	if alloc.freeCalls != 0 {
		t.Errorf("Free called %d times on a temp allocator, want 0", alloc.freeCalls)
	}
}

func TestGrowth_PanicsWhenAllocatorFails(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[int, int](alloc, 4)
	if err != nil {
		t.Fatal(err)
	}

	alloc.failAfter = alloc.mallocCalls // every further Malloc fails

	mustPanic(t, "Insert triggering failed realloc", func() {
		for i := 1; i <= 100; i++ {
			table.Insert(i, i)
		}
	})
}

func TestGrowth_AfterRemovals(t *testing.T) {
	table, _ := newIntTable(t, 8)

	for i := 1; i <= 200; i++ {
		table.Insert(i, i)
	}

	for i := 1; i <= 200; i += 2 {
		table.Remove(i)
	}

	for i := 201; i <= 400; i++ {
		table.Insert(i, i)
	}

	for i := 1; i <= 400; i++ {
		v, ok := table.Get(i)
		shouldExist := i%2 == 0 || i > 200
		if ok != shouldExist {
			t.Errorf("Get(%d) ok = %v, want %v", i, ok, shouldExist)
		}
		if ok && v != i {
			t.Errorf("Get(%d) = %d, want %d", i, v, i)
		}
	}
}

// ---------------------------------------------------------------------------
// Tombstones / churn
// ---------------------------------------------------------------------------

func TestChurn_InsertRemoveManyDistinctKeys(t *testing.T) {
	table, _ := newIntTable(t, 10)

	for i := 1; i <= 200; i++ {
		if !table.Insert(i, i) {
			t.Fatalf("Insert(%d) returned false (Count=%d, Capacity=%d)", i, table.Count, table.Capacity)
		}
		if !table.Remove(i) {
			t.Fatalf("Remove(%d) returned false", i)
		}
	}

	if table.Count != 0 {
		t.Errorf("Count = %d, want 0", table.Count)
	}
}

func TestChurn_TombstonesAreReusedOnInsert(t *testing.T) {
	table, _ := newIntTable(t, 16)

	table.Insert(1, 1)
	table.Remove(1)
	table.Insert(1, 2)

	tombstones, live := 0, 0
	for _, item := range table.Items {
		switch {
		case item.hash == 1:
			tombstones++
		case item.hash > 1:
			live++
		}
	}
	if live != 1 || tombstones != 0 {
		t.Errorf("live = %d, tombstones = %d; want 1 and 0 (tombstone should be reused)", live, tombstones)
	}
}

func TestChurn_GetOnTableFullOfTombstonesTerminates(t *testing.T) {
	table, _ := newIntTable(t, 8)

	for i := 1; i <= 5; i++ {
		table.Insert(i, i)
		table.Remove(i)
	}

	mustNotPanic(t, "Get", func() { table.Get(12345) })
}

// ---------------------------------------------------------------------------
// Edge-case capacities
// ---------------------------------------------------------------------------

func TestZeroCapacity_InsertDoesNotPanic(t *testing.T) {
	table, _ := newIntTable(t, 0)

	mustNotPanic(t, "Insert on capacity 0", func() {
		table.Insert(1, 1)
	})
}

func TestCapacityOne_InsertAndGet(t *testing.T) {
	table, _ := newIntTable(t, 1)

	mustNotPanic(t, "Insert/Get on capacity 1", func() {
		table.Insert(1, 10)
		if v, ok := table.Get(1); !ok || v != 10 {
			t.Errorf("Get(1) = (%d, %v), want (10, true)", v, ok)
		}
	})
}

func TestCapacityTwo_InsertMany(t *testing.T) {
	table, _ := newIntTable(t, 2)

	mustNotPanic(t, "Insert on capacity 2", func() {
		for i := 1; i <= 50; i++ {
			table.Insert(i, i)
		}
	})
}

// ---------------------------------------------------------------------------
// Reserved hash values (0 = free, 1 = tombstone)
// ---------------------------------------------------------------------------


func TestStableHash_NeverReturnsReservedValues(t *testing.T) {
	if h := stableHash(uint8(23)); h <= 1 {
		t.Errorf("stableHash(uint8(23)) = %d, want > 1", h)
	}
	if h := stableHash(int8(23)); h <= 1 {
		t.Errorf("stableHash(int8(23)) = %d, want > 1", h)
	}
	if h := stableHash(uint16(9234)); h <= 1 {
		t.Errorf("stableHash(uint16(9234)) = %d, want > 1", h)
	}
	if h := stableHash(int(982753)); h <= 1 {
		t.Errorf("stableHash(int(982753)) = %d, want > 1", h)
	}
	if h := stableHash(uint(478995)); h <= 1 {
		t.Errorf("stableHash(uint(478995)) = %d, want > 1", h)
	}
	if h := stableHash(uint32(54867)); h <= 1 {
		t.Errorf("stableHash(uint32(54867)) = %d, want > 1", h)
	}
}

func TestInsertGet_KeysWhoseStableHashIsReserved(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[uint8, int](alloc, 16)
	if err != nil {
		t.Fatal(err)
	}

	table.Insert(23, 1) // stable hash of 23 is 1 (tombstone marker)
	table.Insert(24, 2)

	if v, ok := table.Get(23); !ok || v != 1 {
		t.Errorf("Get(23) = (%d, %v), want (1, true)", v, ok)
	}
	if table.Count != 1+1 {
		t.Errorf("Count = %d, want 2", table.Count)
	}
}

// ---------------------------------------------------------------------------
// stableHash
// ---------------------------------------------------------------------------

func TestStableHash_Deterministic(t *testing.T) {
	if stableHash(42) != stableHash(42) {
		t.Error("stableHash(int) is not deterministic")
	}
	if stableHash("hello") != stableHash("hello") {
		t.Error("stableHash(string) is not deterministic")
	}
}

func TestStableHash_StringsDifferentContentDifferentHash(t *testing.T) {
	pairs := [][2]string{
		{"abc", "abd"},
		{"cat", "dog"},
		{"hello", "world"},
		{"a", "b"},
	}
	for _, p := range pairs {
		if stableHash(p[0]) == stableHash(p[1]) {
			t.Errorf("stableHash(%q) == stableHash(%q)", p[0], p[1])
		}
	}
}

func TestStableHash_StringsDifferentLengthDifferentHash(t *testing.T) {
	if stableHash("a") == stableHash("aa") {
		t.Error("stableHash(\"a\") == stableHash(\"aa\")")
	}
}

func TestStableHash_EmptyString(t *testing.T) {
	if h := stableHash(""); h <= 1 {
		t.Errorf("stableHash(\"\") = %d, want > 1", h)
	}
}

func TestStableHash_AllIntegerTypesSupported(t *testing.T) {
	mustNotPanic(t, "stableHash integer types", func() {
		stableHash(int(1))
		stableHash(uint(1))
		stableHash(int8(1))
		stableHash(uint8(1))
		stableHash(int16(1))
		stableHash(uint16(1))
		stableHash(int32(1))
		stableHash(uint32(1))
		stableHash(int64(1))
		stableHash(uint64(1))
	})
}

func TestStableHash_UnsupportedTypesPanic(t *testing.T) {
	mustPanic(t, "float64", func() { stableHash(1.5) })
	mustPanic(t, "bool", func() { stableHash(true) })
	mustPanic(t, "struct", func() { stableHash(struct{ a int }{1}) })
}

func TestDefaultHash_UnsupportedKeyTypePanicsOnInsert(t *testing.T) {
	alloc := newTestAllocator(true)
	table, err := MakeHashTable[float64, int](alloc, 8)
	if err != nil {
		t.Fatal(err)
	}

	mustPanic(t, "Insert with float64 key", func() { table.Insert(1.5, 1) })
}

// ---------------------------------------------------------------------------
// Different key / value types
// ---------------------------------------------------------------------------

func TestVariousIntegerKeyTypes(t *testing.T) {
	t.Run("uint64", func(t *testing.T) {
		table, _ := MakeHashTable[uint64, int](newTestAllocator(true), 16)
		for i := uint64(1); i <= 50; i++ {
			table.Insert(i, int(i))
		}
		for i := uint64(1); i <= 50; i++ {
			if v, ok := table.Get(i); !ok || v != int(i) {
				t.Fatalf("Get(%d) = (%d, %v)", i, v, ok)
			}
		}
	})

	t.Run("int32", func(t *testing.T) {
		table, _ := MakeHashTable[int32, int](newTestAllocator(true), 16)
		for i := int32(-25); i <= 25; i++ {
			table.Insert(i, int(i)*2)
		}
		for i := int32(-25); i <= 25; i++ {
			if v, ok := table.Get(i); !ok || v != int(i)*2 {
				t.Fatalf("Get(%d) = (%d, %v)", i, v, ok)
			}
		}
	})

	t.Run("uint16", func(t *testing.T) {
		table, _ := MakeHashTable[uint16, int](newTestAllocator(true), 16)
		for i := uint16(0); i < 50; i++ {
			table.Insert(i, int(i))
		}
		for i := uint16(0); i < 50; i++ {
			if v, ok := table.Get(i); !ok || v != int(i) {
				t.Fatalf("Get(%d) = (%d, %v)", i, v, ok)
			}
		}
	})
}

func TestStructValues(t *testing.T) {
	type vec struct{ x, y, z float64 }

	table, err := MakeHashTable[int, vec](newTestAllocator(true), 8)
	if err != nil {
		t.Fatal(err)
	}

	table.Insert(1, vec{1, 2, 3})
	table.Insert(2, vec{4, 5, 6})

	if v, ok := table.Get(2); !ok || v != (vec{4, 5, 6}) {
		t.Errorf("Get(2) = (%v, %v)", v, ok)
	}
}

func TestStructKeysWithCustomHash(t *testing.T) {
	type point struct{ x, y int32 }

	hashFn := func(p point) uint64 { return 2 + uint64(uint32(p.x))*31 + uint64(uint32(p.y)) }
	table, err := MakeHashTableWithCustomHashFunc[point, int](newTestAllocator(true), 16, hashFn)
	if err != nil {
		t.Fatal(err)
	}

	table.Insert(point{1, 2}, 12)
	table.Insert(point{2, 1}, 21)

	if v, ok := table.Get(point{1, 2}); !ok || v != 12 {
		t.Errorf("Get({1,2}) = (%d, %v)", v, ok)
	}
	if v, ok := table.Get(point{2, 1}); !ok || v != 21 {
		t.Errorf("Get({2,1}) = (%d, %v)", v, ok)
	}
	if _, ok := table.Get(point{9, 9}); ok {
		t.Error("Get({9,9}) found a missing key")
	}
}

// ---------------------------------------------------------------------------
// doubleHash
// ---------------------------------------------------------------------------

func TestDoubleHash_StaysInRange(t *testing.T) {
	for _, length := range []uint64{2, 3, 7, 16, 100, 1021} {
		for _, hash := range []uint64{2, 3, 17, 1 << 20, 1<<63 + 5} {
			for iter := uint64(0); iter < 50; iter++ {
				if idx := doubleHash(hash, length, iter); idx >= length {
					t.Fatalf("doubleHash(%d, %d, %d) = %d, out of range", hash, length, iter, idx)
				}
			}
		}
	}
}

func TestDoubleHash_ProbeSequenceVisitsEverySlot(t *testing.T) {
	const length = 16
	for hash := uint64(2); hash < 60; hash++ {
		seen := map[uint64]bool{}
		for iter := uint64(0); iter < length; iter++ {
			seen[doubleHash(hash, length, iter)] = true
		}
		if len(seen) != length {
			t.Errorf("hash %d visited %d/%d slots", hash, len(seen), length)
		}
	}
}

func TestInsert_ProbeSequenceReachesEverySlotForNonPrimeCapacity(t *testing.T) {
	// Capacity 16 and a hash whose secondary step is even would only reach half the slots.
	alloc := newTestAllocator(true)
	table, err := MakeHashTableWithCustomHashFunc[int, int](alloc, 16, func(k int) uint64 { return 2 })
	if err != nil {
		t.Fatal(err)
	}

	inserted := 0
	for i := 0; i < 11; i++ { // stays below 70% so no realloc is triggered
		if table.Insert(i, i) {
			inserted++
		}
	}
	if inserted != 11 {
		t.Errorf("inserted %d/11 keys", inserted)
	}
	for i := 0; i < 11; i++ {
		if _, ok := table.Get(i); !ok {
			t.Errorf("Get(%d) not found", i)
		}
	}
}

// ---------------------------------------------------------------------------
// String()
// ---------------------------------------------------------------------------

func TestString_Empty(t *testing.T) {
	table, _ := newIntTable(t, 8)

	if s := table.String(); s != "{ }" {
		t.Errorf("String() = %q, want %q", s, "{ }")
	}
}

func TestString_ContainsEntries(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)
	table.Insert(2, 20)

	s := table.String()

	for _, want := range []string{"(1 - 10)", "(2 - 20)"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

func TestString_OmitsRemovedEntries(t *testing.T) {
	table, _ := newIntTable(t, 16)
	table.Insert(1, 10)
	table.Insert(2, 20)
	table.Remove(1)

	s := table.String()

	if strings.Contains(s, "(1 - 10)") {
		t.Errorf("String() = %q still contains removed entry", s)
	}
	if !strings.Contains(s, "(2 - 20)") {
		t.Errorf("String() = %q missing remaining entry", s)
	}
}

func TestString_OneLinePerEntry(t *testing.T) {
	table, _ := newIntTable(t, 32)
	for i := 1; i <= 5; i++ {
		table.Insert(i, i)
	}

	if lines := strings.Count(table.String(), "\n"); lines != 5 {
		t.Errorf("String() has %d lines, want 5", lines)
	}
}

// ---------------------------------------------------------------------------
// Randomized comparison against a Go map
// ---------------------------------------------------------------------------

func TestAgainstGoMap(t *testing.T) {
	table, _ := newIntTable(t, 8)
	ref := map[int]int{}

	// Small deterministic LCG so the test is reproducible without math/rand.
	state := uint64(12345)
	next := func() uint64 {
		state = state*6364136223846793005 + 1442695040888963407
		return state >> 33
	}

	for step := 0; step < 5000; step++ {
		key := int(next() % 300)
		switch next() % 3 {
		case 0, 1:
			val := int(next() % 1000)
			fmt.Println(table.Get(key))
			fmt.Println(ref[key])
			table.Insert(key, val)
			ref[key] = val
		case 2:
			gotRemoved := table.Remove(key)
			_, wantRemoved := ref[key]
			delete(ref, key)
			if gotRemoved != wantRemoved {
				t.Fatalf("step %d: Remove(%d) = %v, want %v", step, key, gotRemoved, wantRemoved)
			}
		}

		if uint(len(ref)) != table.Count {
			t.Fatalf("step %d: Count = %d, want %d", step, table.Count, len(ref))
		}
	}

	for k, want := range ref {
		if got, ok := table.Get(k); !ok || got != want {
			t.Errorf("Get(%d) = (%d, %v), want (%d, true)", k, got, ok, want)
		}
	}
}