package allocators

import (
	"fmt"
	"strings"
	"unsafe"
	"strconv"
)

const is64Bit 			= strconv.IntSize == 64
const hashTableFillRate = 70 // realloc after reaching 70% fill rate

type Hashable interface {
	GetHash() uint64
}

type pair[TKey comparable, TValue any] struct {
	key   TKey
	value TValue
	hash  uint64 // 0 - is free, 1 - is tombstone, everything else is valid hash
}

type HashTable[TKey comparable, TValue any] struct {
	Allocator Allocator
	Items   []pair[TKey, TValue]
	Hashfunc  func(TKey) uint64
	Capacity  uint
	Count     uint
	Occupied  uint
}

func MakeHashTable[TKey comparable, TValue any](allocator Allocator, capacity uint)  (HashTable[TKey, TValue], error){
	table := HashTable[TKey, TValue]{}

	if capacity == 0 {
		capacity++
	}

	capacity = nextPowerOf2(capacity)

	table.Allocator = allocator
	table.Capacity  = capacity
	table.Count     = 0
	table.Occupied  = 0

	items, error := CallocSliced[pair[TKey, TValue]](allocator, capacity)

	if error != nil {
		var z HashTable[TKey, TValue]
		return z, error
	}

	for i := range(capacity) {
		var zkey TKey
		var zvalue TValue
		
		items[i] = pair[TKey, TValue]{ zkey, zvalue, 0 }
	}

	table.Items = items

	var zero TKey

	_, ok := any(zero).(Hashable)

	if ok {
		table.Hashfunc = func(k TKey) uint64 { 
			return any(k).(Hashable).GetHash() 
		}
	} else {
		table.Hashfunc = stableHash[TKey]
	}

	return table, nil
}

func MakeHashTableWithCustomHashFunc[TKey comparable, TValue any](allocator Allocator, capacity uint, hashFunc func(TKey) uint64) (HashTable[TKey, TValue], error) {
	table := HashTable[TKey, TValue]{}

	if capacity == 0 {
		capacity++
	}

	capacity = nextPowerOf2(capacity)

	table.Allocator = allocator
	table.Capacity  = capacity
	table.Hashfunc  = hashFunc
	table.Count     = 0
	table.Occupied  = 0

	items, error := CallocSliced[pair[TKey, TValue]](allocator, uint(capacity))

	if error != nil {
		var z HashTable[TKey, TValue]
		return z, error
	}

	for i := range(capacity) {
		var zkey TKey
		var zvalue TValue
		
		items[i] = pair[TKey, TValue]{ zkey, zvalue, 0 }
	}

	table.Items = items

	return table, nil
}

func (table *HashTable[TKey, TValue]) Destroy() {
	if table.Allocator.IsTemp() == false {
		table.Allocator.Free(unsafe.Pointer(unsafe.SliceData(table.Items)))
		return
	}
	
	table.Items     = nil
	table.Count     = 0
	table.Capacity  = 0
	table.Allocator = nil
	table.Occupied  = 0
	table.Hashfunc  = nil
}

// Insert item into a table.
// Returns true on success, false if item have been inserted before or for some reason insertion is impossible.
func (table *HashTable[TKey, TValue]) Insert(key TKey, value TValue) bool {
	table.maybeRealloc(1)

	hash := table.Hashfunc(key)

	var capacity uint64 = uint64(table.Capacity)

	inserted, fresh := insertIntoTable(table.Items, hash, key, value, capacity)

	if inserted {
		table.Count++
		if fresh {
			table.Occupied++
		}
	}
	
	return inserted
}

func (table *HashTable[TKey, TValue]) Remove(key TKey) bool {
		hash 	 := table.Hashfunc(key)
		capacity := uint64(table.Capacity)
	var iteration uint64 = 0

	for iteration < capacity {
		index := doubleHash(hash, capacity, iteration)
		item  := table.Items[index]

		if item.hash == hash && item.key == key {
			var zv TValue
			var zk TKey
			item.hash  = 1
			item.value = zv
			item.key   = zk

			table.Items[index] = item

			table.Count--

			return true
		}

		if item.hash == 0 {
			break
		}

		iteration++
	}

	return false
}

func (table *HashTable[TKey, TValue]) Get(key TKey) (TValue, bool) {
		hash 	 := table.Hashfunc(key)
		capacity := uint64(table.Capacity)
	var iteration uint64 = 0

	for iteration < capacity {
		index := doubleHash(hash, capacity, iteration)
		item  := table.Items[index]

		if item.hash == hash && item.key == key {
			return item.value, true
		}

		if item.hash == 0 {
			break
		}

		iteration++
	}

	var z TValue
	return z, false
}

func (table HashTable[TKey, TValue]) Has(key TKey) bool {
		hash 	 := table.Hashfunc(key)
		capacity := uint64(table.Capacity)
	var iteration uint64 = 0

	for iteration < capacity {
		index := doubleHash(hash, capacity, iteration)
		item  := table.Items[index]

		if item.hash == hash && item.key == key {
			return true
		}

		if item.hash == 0 {
			break
		}

		iteration++
	}

	return false
}

func (table HashTable[TKey, TValue]) String() string {
	if table.Count == 0 {
		return "{ }"
	}

	var sb strings.Builder

	for i := range(table.Capacity) {
		item := table.Items[i]

		if item.hash > 1 {
			key   := fmt.Sprintf("%v", item.key)
			value := fmt.Sprintf("%v", item.value)
			str   := fmt.Sprintf("(%s - %s),\n", key, value)

			sb.WriteString(str)
		}
	}

	return sb.String()
}

func (table *HashTable[TKey, TValue]) maybeRealloc(appendCount uint) {
	if ((table.Occupied + appendCount) * 100) / table.Capacity < hashTableFillRate {
		return
	}

	newCapacity   := nextPowerOf2(table.Capacity + appendCount)
	newItems, err := CallocSliced[pair[TKey, TValue]](table.Allocator, newCapacity)

	if err != nil {
		panic("Cannot reallocate hash table. Not enough space.")
	}

	for i := range(newCapacity) {
		var zkey TKey
		var zvalue TValue

		newItems[i] = pair[TKey, TValue]{ zkey, zvalue, 0 }
	}

	for i := range(table.Capacity) {
		item := table.Items[i]

		if item.hash > 1 {
			insertIntoTable(newItems, item.hash, item.key, item.value, uint64(newCapacity))
		}
	}

	if table.Allocator.IsTemp() == false {
		table.Allocator.Free(unsafe.Pointer(unsafe.SliceData(table.Items)))
	}

	table.Items    = newItems
	table.Capacity = newCapacity
	table.Occupied = table.Count
}

func doubleHash(hash uint64, length uint64, iteration uint64) uint64 {
	h1   := hash & (length - 1)
	step := (hash >> 32) | 1
	return (h1 + iteration * step) & (length - 1)
}

func insertIntoTable[TKey comparable, TValue any](table []pair[TKey, TValue], 
												  hash uint64, 
												  key TKey, 
												  value TValue, 
												  capacity uint64) (bool, bool) {
	var iteration 	   uint64 = 0
	var hitTombstone   bool   = false
	var firstTombstone uint64 = 0

	for iteration < capacity {
		index := doubleHash(hash, capacity, iteration)
		item  := table[index]

		switch {
		case item.hash == hash && item.key == key :
			item.value = value
			table[index] = item
			return false, false
		case item.hash == 0 :
			if hitTombstone {
				item  = table[firstTombstone]
				index = firstTombstone
			}

			item.key   = key
			item.value = value
			item.hash  = hash

			table[index] = item

			return true, !hitTombstone
		case item.hash == 1 :
			if !hitTombstone {
				hitTombstone   = true
				firstTombstone = index
			}
		}

		iteration++
	}

	return false, false
}

func stableHash[K comparable](k K) uint64 {
	switch v := any(k).(type) {
	case string:
		return hashstring(v)
	case int:
		return hashint(v)
	case uint:
		return hashuint(v)
	case int8:
		return hashs8(v)
	case uint8:
		return hashu8(v)
	case int16:
		return hashs16(v)
	case uint16:
		return hashu16(v)
	case int32:
		return hashs32(v)
	case uint32:
		return hashu32(v)
	case int64:
		return hashs64(v)
	case uint64:
		return hashu64(v)
	case uintptr:
		return hashuintptr(v)
	default:
		panic("What the fuck are you trying to do?")
	}
}

func nextPowerOf2(v uint) uint {
	v--;
	v |= v >> 1;
	v |= v >> 2;
	v |= v >> 4;
	v |= v >> 8;
	v |= v >> 16;
	if is64Bit {
		v |= v >> 32;
	}
	v++;

	return v
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return 2 + k
}

func hashu8 (x uint8)  uint64 { return fmix64(uint64(x)) }
func hashu16(x uint16) uint64 { return fmix64(uint64(x)) }
func hashu32(x uint32) uint64 { return fmix64(uint64(x)) }
func hashu64(x uint64) uint64 { return fmix64(x) }

func hashs8 (x int8)  uint64 { return fmix64(uint64(x)) }
func hashs16(x int16) uint64 { return fmix64(uint64(x)) }
func hashs32(x int32) uint64 { return fmix64(uint64(x)) }
func hashs64(x int64) uint64 { return fmix64(uint64(x)) }

func hashint    (x int)     uint64 { return fmix64(uint64(x)) }
func hashuint   (x uint)    uint64 { return fmix64(uint64(x)) }
func hashuintptr(x uintptr) uint64 { return fmix64(uint64(x)) }

func hashstring(a string) uint64 {
	hash := uint64(5381)

	for i := 0; i < len(a); i++ {
		hash = ((hash << 5) + hash) + uint64(a[i])
	}

    return fmix64(hash)
}