# timefall

![ci](https://github.com/yuchanns/timefall/actions/workflows/ci.yaml/badge.svg?branch=main)
[![Go Reference](https://pkg.go.dev/badge/go.yuchanns.xyz/timefall)](https://pkg.go.dev/go.yuchanns.xyz/timefall)
[![](https://badge.fury.io/go/go.yuchanns.xyz%2Ftimefall.svg)](https://pkg.go.dev/go.yuchanns.xyz/timefall)
[![License](https://img.shields.io/github/license/yuchanns/timefall)](https://github.com/yuchanns/timefall/blob/main/LICENSE)

`timefall` is a generic, high-performance hierarchical timer wheel implementation for Go.

It is suitable for scenarios that require efficient management of a large number of timers with varying expiration times.

The library is **externally driven** — it does not run its own background loop or goroutine.

Instead, the user is responsible for periodically advancing the timer wheel by calling `Update`.

When `Update` is called, all expired timers for the current tick are dispatched to the provided callback.


<details>
<summary>Benchmark</summary>

```bash
❯ go test -bench=. -benchmem -count=6 -run=$$ -v
goos: linux
goarch: amd64
pkg: go.yuchanns.xyz/timefall
cpu: Intel(R) Core(TM) i5-10500 CPU @ 3.10GHz
BenchmarkTimerMassive
BenchmarkTimerMassive-12            1977            509834 ns/op               0 B/op          0 allocs/op
BenchmarkTimerMassive-12            2451            550316 ns/op               0 B/op          0 allocs/op
BenchmarkTimerMassive-12            2028            585764 ns/op               0 B/op          0 allocs/op
BenchmarkTimerMassive-12            2188            567332 ns/op               0 B/op          0 allocs/op
BenchmarkTimerMassive-12            2107            509195 ns/op               0 B/op          0 allocs/op
BenchmarkTimerMassive-12            2427            548307 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive
BenchmarkStdTimerMassive-12           72          21694044 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive-12           76          18379989 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive-12           74          24394464 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive-12           74          22217037 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive-12           70          22525907 ns/op               0 B/op          0 allocs/op
BenchmarkStdTimerMassive-12           78          23944621 ns/op               0 B/op          0 allocs/op
PASS
ok      go.yuchanns.xyz/timefall        307.905s
```
</details>

## Features

- **Hierarchical timer wheel** with 1 near wheel and 4 higher-level wheels.
- **Configurable precision** (tick duration).
- **Generic type support** for any event payload.
- **Thread-safe** for concurrent Add/Update calls.
- **Optional GC-free operation** via a user-provided allocator.
- **Low overhead** for large numbers of timers.

## Installation

```bash
go get go.yuchanns.xyz/timefall
```

## Usage

### Basic Example

```go
package main

import (
	"fmt"
	"time"

	"go.yuchanns.xyz/timefall"
)

type Event struct {
	ID int
}

func main() {
	// Create a new timer wheel with a precision of 10ms
	tf := timefall.New[Event](10 * time.Millisecond)

	// Add a timer that expires after 500ms
	tf.Add(&Event{ID: 1}, 500*time.Millisecond)

	// Drive the timer wheel using a ticker
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		tf.Update(func(e *Event) {
			fmt.Printf("Timer fired: %+v\n", *e)
		})
	}
}
```

### Multiple Timers

```go
tf := timefall.New[string](time.Millisecond * 5)

tf.Add(newString("A"), 100*time.Millisecond)
tf.Add(newString("B"), 250*time.Millisecond)
tf.Add(newString("C"), 1*time.Second)

ticker := time.NewTicker(5 * time.Millisecond)
defer ticker.Stop()

for range ticker.C {
	tf.Update(func(s *string) {
		fmt.Println("Expired:", *s)
	})
}
```

### Custom Precision

```go
// Precision of 1ms
tf := timefall.New[int](time.Millisecond * 1)
```

### Long-Duration Timers

The hierarchical structure efficiently handles timers scheduled far in the future.

```go
tf := timefall.New[string](10 * time.Millisecond)
tf.Add(newString("far future"), 10*time.Second)
```

### GC-Free Operation

By default, timer nodes are allocated from the Go heap.  
For GC-free operation, a custom allocator can be set via `timefall.SetAllocator`.

```go
type Allocator interface {
	Alloc(size uint) unsafe.Pointer
	Free(ptr unsafe.Pointer)
}

func main() {
	var myAlloc Allocator = NewMyAllocator()
	timefall.SetAllocator(myAlloc)

	tf := timefall.New[int](time.Millisecond * 10)
	// Timers will now use the custom allocator for node memory
}
```

## Important Notes

- `timefall` **does not schedule or run timers itself**.  
  You must periodically call `Update` to process expired timers.
- The library is safe for concurrent use.

## License

Licensed under the Apache License, Version 2.0.

## Contributing

Contributions are welcome. Please ensure that any changes maintain the zero-allocation guarantee and thread-safety properties.
