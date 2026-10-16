package allocators

import . "unsafe"

type Allocator interface {
    Malloc(size uint) (Pointer, error)
    Free(data Pointer) error
    FreeAll()
    IsTemp() bool
    Destroy()
}

func Calloc[T any](allocator Allocator, count uint) (uintptr, error) {
    var t T
    data, err := allocator.Malloc(uint(Sizeof(t)) * count)

    if err != nil {
        return 0, err
    }

    return uintptr(data), nil
}

func CallocSliced[T any](allocator Allocator, count uint) ([]T, error) {
    var t T
    data, err := allocator.Malloc(uint(Sizeof(t)) * count)

    if err != nil {
        return nil, err
    }

    return Slice((*T)(data), count), nil
}

func AppendStruct[T any](allocator Allocator) (*T, error) {
    var t T
    data, err := allocator.Malloc(uint(Sizeof(t)))

    if err != nil {
        return &t, err
    }

    return (*T)(data), nil
}