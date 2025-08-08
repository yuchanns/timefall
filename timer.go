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

// Package timefall provides a high-performance, hierarchical timer wheel
// implementation for scheduling and executing time-based events in Go.
//
// It is designed for scenarios that require:
//   - High-frequency timer updates with low CPU overhead.
//   - Large numbers of timers with varying expiration times.
//   - Optional GC-free operation via a user-provided memory allocator.
//
// The timer wheel uses a multi-level bucketed structure to efficiently
// manage timers that may be scheduled far in the future. Timers "cascade"
// down from higher-level buckets to lower-level buckets as time advances,
// until they reach the lowest level (the "near" wheel) where they are
// dispatched.
//
// The implementation is generic and supports any event payload type via Go
// generics.
//
// # Precision
//
// Timers operate with a fixed precision (tick duration), specified when
// creating a new timer with New. The default precision is 10 milliseconds.
// All timer durations are quantized to this precision.
//
// # Concurrency
//
// Timer operations are safe for concurrent use by multiple goroutines.
// Internal locking is implemented via a lightweight spinlock.
//
// # Memory allocation
//
// By default, timer nodes are allocated from the Go heap. For GC-free
// operation, a user-provided allocator can be injected via the package-level
// variable `malloc`, which must implement Alloc(size) and Free(ptr) methods.
//
// # Example
//
//	import "time"
//	type Event struct { ID int }
//
//	tf := timefall.New[Event](10*time.Millisecond)
//	tf.Add(&Event{ID: 1}, 500*time.Millisecond)
//
//	// In a loop or ticker:
//	tf.Update(func(e *Event) {
//	    fmt.Println("Timer fired:", e.ID)
//	})
package timefall

import (
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/aristanetworks/goarista/monotime"
)

// Timer is the core hierarchical timer wheel structure.
type Timer[T any] struct {
	n            [timeNear]linkList[T]
	t            [4][timeLevel]linkList[T]
	l            int32
	time         uint32
	starttime    int64
	current      int64
	currentPoint uint64

	precis time.Duration
}

// New creates and initializes a new timer wheel instance for events of type T.
//
// The optional precision parameter specifies the tick duration (minimum timer
// resolution). If omitted or non-positive, the default precision is 10 milliseconds.
// All timer durations will be rounded to the nearest multiple of this precision.
//
// If a package-level allocator `malloc` is set, the timer object will be allocated
// using malloc.Alloc instead of the Go heap.
//
// The returned timer must be periodically advanced by calling Update, typically
// from a ticker or game loop.
//
// Example:
//
//	tf := timefall.New[string](time.Millisecond * 5)
//	tf.Add(new(string), time.Second)
func New[T any](precision ...time.Duration) (t *Timer[T]) {
	t = &Timer[T]{}
	if malloc != nil {
		ptr := malloc.Alloc(uint(unsafe.Sizeof(*t)))
		t = (*Timer[T])(ptr)
	}

	for i := range timeNear {
		t.n[i].clear()
	}

	for i := range 4 {
		for j := range timeLevel {
			t.t[i][j].clear()
		}
	}

	// default precision is centiseconds
	precis := time.Microsecond * 10
	if len(precision) > 0 && precision[0] > 0 {
		precis = precision[0]
	}
	t.precis = precis

	t.init()

	return t
}

// Destroy stops the timer wheel, releases all pending timers, and frees
// internal resources.
//
// Any remaining timers are dispatched with a nil callback (i.e., discarded).
// If a package-level allocator is set, the timer object itself is freed
// via malloc.Free.
//
// It is safe to call Destroy multiple times or on a nil timer.
func (t *Timer[T]) Destroy() {
	if t == nil {
		return
	}
	t.acquireLock()
	defer func() {
		t.releaseLock()
		if malloc != nil {
			malloc.Free(unsafe.Pointer(t))
		}
	}()
	for i := range timeNear {
		current := t.n[i].clear()
		if current != nil {
			t.dispatchList(current, nil)
		}
	}
	for i := range 4 {
		for j := range timeLevel {
			current := t.t[i][j].clear()
			if current != nil {
				t.dispatchList(current, nil)
			}
		}
	}
}

// Add schedules a new timer event with the specified duration until expiration.
//
// The event payload is copied from the provided pointer and stored internally.
// The timer will fire after at least 'duration' has elapsed, quantized to the
// timer's precision.
//
// This method is safe for concurrent use.
func (t *Timer[T]) Add(event *T, duration time.Duration) {
	t.acquireLock()
	defer t.releaseLock()

	time := int32(duration.Nanoseconds() / t.precis.Nanoseconds())

	node := &timerNode[T]{}
	if malloc != nil {
		ptr := malloc.Alloc(uint(unsafe.Sizeof(*node)))
		node = (*timerNode[T])(ptr)
	}
	node.next = nil
	node.event = *event
	node.expire = time + int32(t.time)
	t.addNode(node)
}

// Update advances the timer wheel to the current time and dispatches any
// expired timers.
//
// The provided callback fn is invoked for each expired timer event.
// If fn is nil, expired timers are discarded.
//
// Update should be called periodically, typically from a ticker running
// at the timer's precision. Multiple calls within the same tick are ignored.
//
// This method is safe for concurrent use.
func (t *Timer[T]) Update(fn TimerExecuteFunc[T]) {
	t.assert()
	cp := uint64(time.Unix(0, int64(monotime.Now())).UnixNano() / t.precis.Nanoseconds())
	if cp < t.currentPoint {
		t.currentPoint = cp
	} else if cp == t.currentPoint {
		return
	}
	diff := int64(cp - t.currentPoint)
	t.currentPoint = cp
	t.current += diff
	for range diff {
		t.tick(fn)
	}
}

// Start returns the start time of the timer wheel in units of the
// timer's precision.
func (t *Timer[T]) Start() int64 {
	t.assert()
	return t.starttime
}

// Now returns the current time of the timer wheel in units of the
// timer's precision.
func (t *Timer[T]) Now() int64 {
	t.assert()
	return t.current
}

const (
	timeNearShift  = 8
	timeNear       = 1 << timeNearShift
	timeLevelShift = 6
	timeLevel      = 1 << timeLevelShift
	timeNearMask   = timeNear - 1
	timeLevelMask  = timeLevel - 1
)

type timerNode[T any] struct {
	next   *timerNode[T]
	expire int32
	event  T
}

type linkList[T any] struct {
	head timerNode[T]
	tail *timerNode[T]
}

func (l *linkList[T]) link(node *timerNode[T]) {
	l.tail.next = node
	l.tail = node
	node.next = nil
}

func (l *linkList[T]) clear() (ret *timerNode[T]) {
	ret = l.head.next
	l.head.next = nil
	l.tail = &l.head
	return
}

func (t *Timer[T]) assert() {
	if t == nil {
		panic("timer is nil")
	}
}

func (t *Timer[T]) acquireLock() {
	t.assert()
	for !atomic.CompareAndSwapInt32(&t.l, 0, 1) {
		runtime.Gosched()
	}
}

func (t *Timer[T]) releaseLock() {
	t.assert()
	atomic.StoreInt32(&t.l, 0)
}

func (t *Timer[T]) init() {
	t.assert()
	now := time.Now()
	sec := now.UnixNano() / t.precis.Nanoseconds()
	t.time = 0
	t.starttime = sec / 100
	t.current = sec % 100
	t.currentPoint = uint64(time.Unix(0, int64(monotime.Now())).UnixNano() / t.precis.Nanoseconds())
	atomic.StoreInt32(&t.l, 0)
}

// TimerExecuteFunc is the type of callback function invoked by Update
// for each expired timer event.
//
// The argument is a pointer to the event payload stored in the timer node.
type TimerExecuteFunc[T any] func(arg *T)

func (t *Timer[T]) tick(fn TimerExecuteFunc[T]) {
	t.acquireLock()
	defer t.releaseLock()

	// try to dispatch timeout 0 (rare condition)
	t.execute(fn)

	// shift time first, and then dispatch timer message
	t.shift()

	t.execute(fn)
}

func (t *Timer[T]) dispatchList(current *timerNode[T], fn TimerExecuteFunc[T]) {
	for {
		if fn != nil {
			fn(&current.event)
		}
		temp := current
		current = current.next
		if malloc != nil {
			malloc.Free(unsafe.Pointer(temp))
		}
		if current == nil {
			return
		}
	}
}

func (t *Timer[T]) execute(fn TimerExecuteFunc[T]) {
	t.assert()
	idx := t.time & timeNearMask

	for t.n[idx].head.next != nil {
		current := t.n[idx].clear()
		t.releaseLock()
		// dispatchList don't need lock
		t.dispatchList(current, fn)
		t.acquireLock()
	}
}

func (t *Timer[T]) shift() {
	t.assert()
	mask := uint32(timeNear)
	t.time++
	ct := t.time
	if ct == 0 {
		t.moveList(3, 0)
		return
	}
	time := ct >> timeNearShift
	var i int
	for (ct & (mask - 1)) == 0 {
		idx := time & timeLevelMask
		if idx != 0 {
			t.moveList(i, int(idx))
			break
		}
		mask <<= timeLevelShift
		time >>= timeLevelShift
		i++
	}
}

func (t *Timer[T]) moveList(level int, idx int) {
	t.assert()

	current := t.t[level][idx].clear()
	for current != nil {
		temp := current.next
		t.addNode(current)
		current = temp
	}
}

func (t *Timer[T]) addNode(node *timerNode[T]) {
	t.assert()

	time := node.expire
	current_time := int32(t.time)

	if (time | timeNearMask) == (current_time | timeNearMask) {
		t.n[time&timeNearMask].link(node)
		return
	}
	mask := int32(timeNear << timeLevelShift)
	var i = 0
	for ; i < 3; i++ {
		if (time | (mask - 1)) == (current_time | (mask - 1)) {
			break
		}
		mask <<= timeLevelShift
	}
	t.t[i][(time>>(timeNearShift+i*timeLevelShift))&timeLevelMask].link(node)
}
