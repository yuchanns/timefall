package timefall

import (
	"sync/atomic"
	"unsafe"
)

type Allocator interface {
	Alloc(size uint) unsafe.Pointer
	Free(ptr unsafe.Pointer)
}

var malloc Allocator

var allocInit atomic.Int32

func SetAllocator(alloc Allocator) {
	if alloc == nil {
		panic("allocator cannot be nil")
	}
	if allocInit.Add(1) != 1 {
		panic("allocator can only be set once")
	}
	malloc = alloc
}
