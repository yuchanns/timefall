package timefall_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.yuchanns.xyz/timefall"
)

func TestTimerDefault(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	type testData struct {
		time     time.Time
		expected time.Duration
	}

	// The precision is centiseconds
	precision := 10 * time.Millisecond

	tf := timefall.New[testData]()

	testTable := []time.Duration{
		30 * time.Millisecond,
		100 * time.Millisecond,
		250 * time.Millisecond,
		500 * time.Millisecond,
		750 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}

	wg := sync.WaitGroup{}
	for _, expected := range testTable {
		wg.Add(1)
		tf.Add(&testData{
			time:     time.Now(),
			expected: expected,
		}, expected)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(precision)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tf.Update(func(arg *testData) {
					defer wg.Done()
					assert.InDelta(
						time.Since(arg.time).Nanoseconds(),
						arg.expected.Nanoseconds(),
						float64(precision.Nanoseconds()),
					)
				})
			}
		}
	}()

	wg.Wait()
	cancel()
}
