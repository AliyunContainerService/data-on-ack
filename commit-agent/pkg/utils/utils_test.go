/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetContainerID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "cgroup v1 systemd docker",
			in:   "12:cpu,cpuacct:/kubepods/besteffort/podabc/docker-1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b.scope",
			want: "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		},
		{
			name: "cgroup v1 cri-containerd",
			in:   "11:name=systemd:/kubepods.slice/kubepods-burstable.slice/cri-containerd-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.scope",
			want: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name: "cgroup v2",
			in:   "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podxx.slice/cri-containerd-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.scope",
			want: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		{
			name: "no hex id, fallback to legacy parsing",
			in:   "0::/system.slice/something-shorthex.scope",
			want: "shorthex",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetContainerID(tc.in)
			if got != tc.want {
				t.Errorf("GetContainerID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestReadCgroupLine(t *testing.T) {
	dir := t.TempDir()

	cases := map[string]struct {
		body    string
		wantSub string
	}{
		"v1": {
			body: `12:cpu,cpuacct:/kubepods/pod-a/docker-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.scope
11:name=systemd:/kubepods/pod-a/docker-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.scope
`,
			wantSub: "name=systemd",
		},
		"v2": {
			body:    "0::/kubepods.slice/kubepods-burstable.slice/cri-containerd-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.scope\n",
			wantSub: "0::/",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			line, err := ReadCgroupLine(p)
			if err != nil {
				t.Fatalf("ReadCgroupLine: %v", err)
			}
			if !contains(line, tc.wantSub) {
				t.Errorf("line %q missing substring %q", line, tc.wantSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (s[:len(sub)] == sub || contains(s[1:], sub)))))
}
