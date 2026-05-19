/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package operate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectRuntime_Override(t *testing.T) {
	cases := map[string]struct {
		override string
		want     RuntimeKind
		wantErr  bool
	}{
		"docker":     {override: "docker", want: RuntimeDocker},
		"containerd": {override: "containerd", want: RuntimeContainerd},
		"upper":      {override: "DOCKER", want: RuntimeDocker},
		"bogus":      {override: "podman", wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := selectRuntime(tc.override, "/no", "/no")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSelectRuntime_FromEnv(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "containerd")
	got, err := selectRuntime("", "/no", "/no")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != RuntimeContainerd {
		t.Fatalf("got %s, want containerd", got)
	}
}

func TestSelectRuntime_AutoDetect(t *testing.T) {
	t.Setenv("CONTAINER_RUNTIME", "")
	dir := t.TempDir()
	dock := filepath.Join(dir, "docker.sock")
	cont := filepath.Join(dir, "containerd.sock")

	// Neither present -> error.
	if _, err := selectRuntime("", dock, cont); err == nil {
		t.Fatalf("expected error when no sockets exist")
	}

	// Only containerd present -> containerd.
	if err := os.WriteFile(cont, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := selectRuntime("", dock, cont)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != RuntimeContainerd {
		t.Fatalf("got %s, want containerd", got)
	}

	// Both present -> docker wins (precedence rule).
	if err := os.WriteFile(dock, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = selectRuntime("", dock, cont)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != RuntimeDocker {
		t.Fatalf("got %s, want docker", got)
	}
}

func TestIsRetryablePushError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nil":             {err: nil, want: false},
		"unauthorized":    {err: errString("unauthorized: token expired"), want: false},
		"denied":          {err: errString("denied: insufficient_scope"), want: false},
		"manifest":        {err: errString("manifest invalid"), want: false},
		"500":             {err: errString("registry returned status 500"), want: true},
		"503":             {err: errString("503 Service Unavailable"), want: true},
		"timeout":         {err: errString("net/http: request canceled (Client.Timeout exceeded)"), want: true},
		"conn reset":      {err: errString("connection reset by peer"), want: true},
		"random":          {err: errString("something weird"), want: false},
		"unauthorized503": {err: errString("503 then unauthorized"), want: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isRetryablePushError(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v (err=%v)", got, tc.want, tc.err)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestRetryPolicyDefaults(t *testing.T) {
	p := RetryPolicy{}.withDefaults()
	if p.MaxAttempts < 1 || p.InitialDelay <= 0 || p.MaxDelay <= 0 || p.Multiplier <= 1 {
		t.Fatalf("bad defaults: %+v", p)
	}
}
