//go:build windows

package allocators

import "syscall"

var (
    kernel32     = syscall.NewLazyDLL("kernel32.dll")
    VirtualAlloc = kernel32.NewProc("VirtualAlloc")
    VirtualFree  = kernel32.NewProc("VirtualFree")
)

const (
    memCommit     = 0x1000
    memReserve    = 0x2000
    memRelease    = 0x8000
    pageNoAccess  = 0x01
    pageReadWrite = 0x04
)

func MemReserve(size uint) (uintptr, error) {
    mem, _, err := VirtualAlloc.Call(0, uintptr(size), memReserve, pageNoAccess)

    if mem == 0 {
        return 0, err
    }

    return mem, nil
}

func MemCommit(ptr uintptr, size uint) error {
    mem, _, err := VirtualAlloc.Call(ptr, uintptr(size), memCommit, pageReadWrite)

    if mem == 0 {
        return err
    }

    return nil
}

func MemRelease(data uintptr) {
    VirtualFree.Call(data, 0, memRelease)
}