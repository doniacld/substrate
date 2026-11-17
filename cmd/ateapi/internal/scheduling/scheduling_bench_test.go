// Copyright 2026 The Agent Substrate Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheduling

import (
	"context"
	"fmt"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// benchFleet builds n workers shaped like a real pool: every fourth worker is
// microvm rather than gvisor, labels alternate between two tiers, and
// allocation varies so that some workers are full and the rest differ in load.
// Schedule therefore exercises every filter and the load comparison.
func benchFleet(n int) fleet {
	f := make(fleet, 0, n)
	for i := range n {
		class := "gvisor"
		if i%4 == 3 {
			class = "microvm"
		}
		tier := fmt.Sprintf("%d", i%2)
		w := worker(fmt.Sprintf("w-%d", i), class, fmt.Sprintf("node-%d", i%16),
			map[string]string{"tier": tier, "pool": "default"},
			withMaxActors(32), withCapacity(16_000, 64<<30))
		// i%33 in [0,32]: 32 marks a full worker.
		for a := range i % 33 {
			assignedFor("demo", fmt.Sprintf("a-%d-%d", i, a),
				&ateapi.Resources{CpuMillicores: 250, MemoryBytes: 1 << 30})(w)
		}
		f = append(f, w)
	}
	return f
}

func BenchmarkSchedule(b *testing.B) {
	sizes := []int{100, 1_000, 10_000}
	cases := []struct {
		name        string
		constraints Constraints
	}{
		{
			name:        "class-only",
			constraints: Constraints{SandboxClass: "gvisor"},
		},
		{
			name: "selectors-and-limits",
			constraints: Constraints{
				SandboxClass:     "gvisor",
				TemplateSelector: labels.SelectorFromSet(labels.Set{"pool": "default"}),
				ActorSelector:    labels.SelectorFromSet(labels.Set{"tier": "1"}),
				Limits:           &ateapi.Resources{CpuMillicores: 500, MemoryBytes: 2 << 30},
			},
		},
	}
	for _, n := range sizes {
		f := benchFleet(n)
		for _, tc := range cases {
			b.Run(fmt.Sprintf("%s/workers=%d", tc.name, n), func(b *testing.B) {
				s := New(f)
				ctx := context.Background()
				b.ReportAllocs()
				for b.Loop() {
					if _, err := s.Schedule(ctx, tc.constraints); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
