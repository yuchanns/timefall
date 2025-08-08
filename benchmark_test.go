// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package timefall_test

import (
	"testing"
	"time"

	"go.yuchanns.xyz/timefall"
)

func BenchmarkTimerMassive(b *testing.B) {
	type testData struct {
		start    time.Time
		expected time.Duration
	}

	const precision = 10 * time.Millisecond
	const nodeCount = 100_000

	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		tf := timefall.New[testData](precision)
		for range nodeCount {
			tf.Add(&testData{
				start:    time.Now(),
				expected: precision,
			}, precision)
		}
		time.Sleep(precision)

		b.StartTimer()

		tf.Update(func(arg *testData) {})

		b.StopTimer()

		tf.Destroy()

		b.StartTimer()
	}
}

func BenchmarkStdTimerMassive(b *testing.B) {
	const precision = 10 * time.Millisecond
	const nodeCount = 100_000

	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		timers := make([]*time.Timer, nodeCount)
		for i := range nodeCount {
			timers[i] = time.NewTimer(precision)
		}
		time.Sleep(precision)

		b.StartTimer()
		for _, t := range timers {
			<-t.C
		}
		b.StopTimer()

		for _, t := range timers {
			t.Stop()
		}
		b.StartTimer()
	}
}
