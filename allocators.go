package allocators

import . "unsafe"

type Allocator interface {
    Malloc(size uint) (Pointer, error)
    Free(data Pointer) error
    FreeAll()
    IsTemp() bool
}

func Calloc[T any](allocator Allocator, count uint) (*T, error) {
    var t T
    data, error := allocator.Malloc(uint(Sizeof(t)) * count)

    if error != nil {
        return nil, error
    }

    return (*T)(data), nil
}

func CallocSliced[T any](allocator Allocator, count uint) ([]T, error) {
    var t T
    data, error := allocator.Malloc(uint(Sizeof(t)) * count)

    if error != nil {
        return nil, error
    }

    return Slice((*T)(data), count), nil
}

func AppendStruct[T any](allocator Allocator) (*T, error) {
    return Calloc[T](allocator, 1)
}