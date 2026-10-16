package allocators

import . "unsafe"
import "errors"

var _ Allocator = (*Pool)(nil)

type pool_region struct {
	arena *Arena
	next  *pool_region
}

type Pool struct {
	first 		*pool_region
	minCapacity  uint
}

func NewPool(minCapacity uint) (*Pool, error) {
	pool := new(Pool)

	pool.minCapacity = minCapacity
	region, err := newRegion(minCapacity)

	if err != nil {
		return nil, err
	}

	pool.first = region

	return pool, nil
}

func (pool *Pool) Malloc(size uint) (Pointer, error) {
	region, err := pool.findBestFit(size)

	if err != nil {
		return nil, err
	}

	data, err := region.arena.Malloc(size)

	if err != nil {
		return nil, err
	}

	return data, err
}

func (pool *Pool) Free(data Pointer) error {
	return errors.New("Cannot free individual data with Pool allocator.")
}

func (pool *Pool) FreeAll() {
	region := pool.first

	for region != nil {
		region.arena.FreeAll()
		region = region.next
	}
}

func (pool *Pool) IsTemp() bool {
	return true
}

func (pool *Pool) Destroy() {
	region := pool.first

	for region != nil {
		region.arena.Destroy()
		region.arena = nil

		region = region.next
	}

	pool.first = nil
}

func newRegion(reserve uint) (*pool_region, error) {
	arena, err := NewArena(reserve)

	if err != nil {
		return nil, err
	}

	region := new(pool_region{arena: arena, next: nil})

	return region, nil
}

func (pool *Pool) findBestFit(size uint) (*pool_region, error) {
	region := pool.first

	for region != nil {
		if (region.arena.canFit(size)) {
			break
		}

		if region.next == nil {
			next, err := newRegion(max(pool.minCapacity, size + 1024))

			if err != nil {
				return nil, err
			}

			region.next = next
		}

		region = region.next
	}

	return region, nil
}