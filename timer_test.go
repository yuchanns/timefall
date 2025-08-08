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

func TestTimerVariousPrecisions(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	type testData struct {
		start    time.Time
		expected time.Duration
	}

	precisions := []time.Duration{
		time.Millisecond,
		5 * time.Millisecond,
		20 * time.Millisecond,
		50 * time.Millisecond,
	}

	for _, prec := range precisions {
		t.Run(prec.String(), func(t *testing.T) {
			t.Parallel()
			tf := timefall.New[testData](prec)

			wg := sync.WaitGroup{}
			expected := 200 * time.Millisecond
			wg.Add(1)
			tf.Add(&testData{
				start:    time.Now(),
				expected: expected,
			}, expected)

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				ticker := time.NewTicker(prec)
				for {
					select {
					case <-ctx.Done():
						ticker.Stop()
						return
					case <-ticker.C:
						tf.Update(func(arg *testData) {
							defer wg.Done()
							diff := time.Since(arg.start)
							assert.InDelta(
								diff.Nanoseconds(),
								arg.expected.Nanoseconds(),
								float64(prec.Nanoseconds()),
							)
						})
					}
				}
			}()

			wg.Wait()
			cancel()
		})
	}
}

func TestTimerDestroy(t *testing.T) {
	t.Parallel()
	tf := timefall.New[int]()
	tf.Add(new(int), 100*time.Millisecond)
	require.NotPanics(t, func() {
		tf.Destroy()
	})
}

func TestTimerConcurrentAddUpdate(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	type testData struct {
		start    time.Time
		expected time.Duration
	}

	const precision = 5 * time.Millisecond
	tf := timefall.New[testData](precision)

	wg := sync.WaitGroup{}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				wg.Add(1)
				tf.Add(&testData{
					start:    time.Now(),
					expected: 50 * time.Millisecond,
				}, 50*time.Millisecond)
				time.Sleep(time.Millisecond)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(precision)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tf.Update(func(arg *testData) {
					diff := time.Since(arg.start)
					assert.LessOrEqual(diff.Milliseconds(), int64(arg.expected.Milliseconds()+precision.Milliseconds()))
					wg.Done()
				})
			}
		}
	}()

	wg.Wait()
}

func TestTimerLongDelay(t *testing.T) {
	t.Parallel()
	assert := require.New(t)

	type testData struct {
		start    time.Time
		expected time.Duration
	}

	const precision = 10 * time.Millisecond
	tf := timefall.New[testData](precision)

	expected := 5 * time.Second
	wg := sync.WaitGroup{}
	wg.Add(1)

	tf.Add(&testData{
		start:    time.Now(),
		expected: expected,
	}, expected)

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
					diff := time.Since(arg.start)
					assert.InDelta(
						diff.Nanoseconds(),
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
