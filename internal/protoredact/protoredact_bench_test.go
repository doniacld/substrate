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

package protoredact_test

import (
	"fmt"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/internal/proto/ateletpb"
	"go.ate.dev/substrate/internal/protoredact"
	"google.golang.org/protobuf/proto"
)

// benchRunRequest is a RunRequest with several containers, each carrying a mix
// of plain and secret env vars, alongside a volume whose scalar map Redacted
// must skip.
func benchRunRequest() *ateletpb.RunRequest {
	spec := &ateletpb.WorkloadSpec{
		Volumes: []*ateletpb.Volume{{
			Name:     "data",
			External: &ateletpb.ExternalVolumeSource{VolumeContext: map[string]string{"fsType": "ext4"}},
		}},
	}
	for c := range 4 {
		ctr := &ateletpb.Container{Name: fmt.Sprintf("c-%d", c)}
		for e := range 16 {
			ctr.Env = append(ctr.Env, &ateletpb.EnvVar{Name: fmt.Sprintf("VAR_%d", e), Value: "value"})
		}
		spec.Containers = append(spec.Containers, ctr)
	}
	return &ateletpb.RunRequest{Spec: spec}
}

func BenchmarkNeedsRedaction(b *testing.B) {
	cases := []struct {
		name string
		msg  proto.Message
	}{
		// A type that cannot reach a debug_redact field: answered by the
		// memoized type check alone.
		{"unlabeled-type", &ateapi.Worker{WorkerPod: "w-0", NodeName: "node-a", Labels: map[string]string{"tier": "1"}}},
		// A type that can reach one, with secrets set: walks until the first hit.
		{"run-request", benchRunRequest()},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				protoredact.NeedsRedaction(tc.msg)
			}
		})
	}
}

func BenchmarkRedacted(b *testing.B) {
	msg := benchRunRequest()
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := protoredact.Redacted(msg); !ok {
			b.Fatal("Redacted() masked nothing")
		}
	}
}
