/*
Copyright 2021 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workloadgate

import (
	"reflect"
	"testing"
)

func TestIsWorkloadEnable(t *testing.T) {
	cases := []struct {
		workloads       string
		expectEnables   map[string]bool
		expectEnableAll bool
	}{
		{
			workloads:       "",
			expectEnables:   map[string]bool{},
			expectEnableAll: false,
		},
		{
			workloads:       "*",
			expectEnables:   map[string]bool{},
			expectEnableAll: true,
		},
		{
			workloads:       "*,foo",
			expectEnables:   map[string]bool{"foo": true},
			expectEnableAll: true,
		},
		{
			workloads:       "foo,*",
			expectEnables:   map[string]bool{"foo": true},
			expectEnableAll: true,
		},
		{
			workloads:       "foo,a",
			expectEnables:   map[string]bool{"foo": true, "a": true},
			expectEnableAll: false,
		},
		{
			workloads:       "foo,-a",
			expectEnables:   map[string]bool{"foo": true, "a": false},
			expectEnableAll: false,
		},
		{
			workloads:       "-foo,a",
			expectEnables:   map[string]bool{"foo": false, "a": true},
			expectEnableAll: false,
		},
		{
			workloads:       "foo,-*",
			expectEnables:   map[string]bool{"foo": true},
			expectEnableAll: false,
		},
	}
	for _, c := range cases {
		enables, enableAll := parseWorkloadsEnabled(c.workloads)
		if !reflect.DeepEqual(enables, c.expectEnables) {
			t.Fatalf("workloads: %s, expected: %v, got: %v", c.workloads, c.expectEnables, enables)
		}
		if enableAll != c.expectEnableAll {
			t.Fatalf("workloads %s, expected: %v, got: %v", c.workloads, c.expectEnableAll, enableAll)
		}
	}
}
