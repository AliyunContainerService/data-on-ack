/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package server

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryCounter(t *testing.T) {
	r := NewRegistry()
	r.docHelp("foo_total", "demo")
	r.IncCounter("foo_total", map[string]string{"a": "1"})
	r.AddCounter("foo_total", map[string]string{"a": "1"}, 4)
	r.IncCounter("foo_total", map[string]string{"a": "2"})

	var buf bytes.Buffer
	if err := r.write(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`# HELP foo_total demo`,
		`# TYPE foo_total counter`,
		`foo_total{a="1"} 5`,
		`foo_total{a="2"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRegistryGauge(t *testing.T) {
	r := NewRegistry()
	r.IncGauge("inflight", nil)
	r.IncGauge("inflight", nil)
	r.DecGauge("inflight", nil)

	var buf bytes.Buffer
	_ = r.write(&buf)
	if !strings.Contains(buf.String(), "inflight 1") {
		t.Fatalf("gauge value wrong:\n%s", buf.String())
	}
}

func TestRegistryHistogram(t *testing.T) {
	r := NewRegistry()
	r.docHelp("dur_seconds", "demo")
	r.ObserveSeconds("dur_seconds", map[string]string{"m": "X"}, 0.05)
	r.ObserveSeconds("dur_seconds", map[string]string{"m": "X"}, 3)

	var buf bytes.Buffer
	_ = r.write(&buf)
	out := buf.String()
	for _, want := range []string{
		`# TYPE dur_seconds histogram`,
		`dur_seconds_bucket{m="X",le="0.1"} 1`,
		`dur_seconds_bucket{m="X",le="+Inf"} 2`,
		`dur_seconds_count{m="X"} 2`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestEscapeLabelValue(t *testing.T) {
	in := `weird "value" with \backslash` + "\nnewline"
	got := escapeLabelValue(in)
	want := `weird \"value\" with \\backslash\nnewline`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHTTPHandler_Format(t *testing.T) {
	r := NewRegistry()
	r.IncCounter("rpc_total", map[string]string{"code": "OK"})

	rec := httptest.NewRecorder()
	r.HTTPHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("unexpected content type %q", got)
	}
	if !strings.Contains(rec.Body.String(), `rpc_total{code="OK"} 1`) {
		t.Fatalf("body missing sample: %s", rec.Body.String())
	}
}
