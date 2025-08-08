// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package timefall

import (
	"sync/atomic"
	"unsafe"
)

// Allocator defines the interface for a custom memory allocator.
//
// A user-provided Allocator can be installed via SetAllocator to enable
// GC-free operation for timer node allocations.
//
// Implementations must be safe for concurrent use by multiple goroutines
// if timers are added or updated concurrently.
//
// Alloc returns a pointer to a newly allocated memory block of the given size.
// The returned pointer must be aligned and valid for use with unsafe.Pointer.
//
// Free releases the memory block pointed to by ptr. The pointer must have
// been previously returned by Alloc from the same Allocator implementation.
type Allocator interface {
	Alloc(size uint) unsafe.Pointer
	Free(ptr unsafe.Pointer)
}

var malloc Allocator

var allocInit atomic.Int32

// SetAllocator installs a custom Allocator for GC-free timer node allocation.
//
// The provided alloc must not be nil. SetAllocator can only be called once;
// subsequent calls will panic.
//
// Once set, the allocator will be used for all subsequent allocations of
// timer-related data structures. Existing timers allocated before setting
// the allocator will continue to use the Go heap.
//
// Example:
//
//	type MyAlloc struct{ /* ... */ }
//	func (m *MyAlloc) Alloc(size uint) unsafe.Pointer { /* ... */ }
//	func (m *MyAlloc) Free(ptr unsafe.Pointer) { /* ... */ }
//
//	func main() {
//	    timefall.SetAllocator(&MyAlloc{})
//	    tf := timefall.New[int](time.Millisecond * 10)
//	    // Timers will now use MyAlloc for node memory
//	}
//
// Panics:
//   - if alloc is nil.
//   - if called more than once.
func SetAllocator(alloc Allocator) {
	if alloc == nil {
		panic("allocator cannot be nil")
	}
	if allocInit.Add(1) != 1 {
		panic("allocator can only be set once")
	}
	malloc = alloc
}
