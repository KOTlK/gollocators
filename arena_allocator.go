package allocators

import . "unsafe"
import "errors"

var _ Allocator = (*Arena)(nil)

const minCommitPerAllocation uint = 1024 * 1024 * 4 // 4 mb minimal commit
const pageSize     = 4096
const defaultAlign = 16

type Arena struct {
    data     uintptr
    reserved uint
    commited uint
    used     uint
}

func NewArena(reserve uint) (*Arena, error) {
    mem, err := MemReserve(reserve)

    if err != nil {
        return nil, err
    }

    arena := new(Arena {data: mem, reserved: alignUp(reserve, pageSize), commited: 0, used: 0});

    return arena, nil;
}

func (arena *Arena) Malloc(size uint) (Pointer, error) {
    start := alignUp(arena.used, defaultAlign)

    if size > arena.reserved || start > arena.reserved - size {
        return nil, errors.New("Arena: out of reserved memory.")
    }

    newUsed := start + size

    if newUsed > arena.commited {
        target := min(alignUp(newUsed + minCommitPerAllocation, pageSize), arena.reserved)
        err    := MemCommit(arena.data + uintptr(arena.commited), target - arena.commited)

        if err != nil {
            return nil, err
        }

        arena.commited = target
    }

    arena.used = newUsed

    return Pointer(arena.data + uintptr(start)), nil
}

func (arena *Arena) Free(data Pointer) error {
    return errors.New("Cannot free individual data with Arena allocator.")
}

func (arena *Arena) FreeAll() {
    arena.used = 0
}

func (arena *Arena) IsTemp() bool {
    return true
}

func (arena *Arena) Destroy() {
    MemRelease(arena.data)
    arena.data     = 0
    arena.used     = 0
    arena.reserved = 0
    arena.commited = 0
}

func (arena *Arena) canFit(size uint) bool {
    start := alignUp(arena.used, defaultAlign)

    if size > arena.reserved || start > arena.reserved-size {
        return false
    }

    return true
}

func alignUp(mem, align uint) uint { 
    return (mem + align - 1) &^ (align - 1) 
}