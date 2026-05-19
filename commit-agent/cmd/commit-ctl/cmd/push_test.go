/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package cmd

import (
	"strings"
	"testing"
)

func TestReadPasswordStdin(t *testing.T) {
	cases := map[string]struct {
		input string
		want  string
		err   bool
	}{
		"with newline":    {input: "secret\n", want: "secret"},
		"with crlf":       {input: "secret\r\n", want: "secret"},
		"without newline": {input: "secret", want: "secret"},
		"empty":           {input: "\n", err: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pw, err := readPasswordStdin(strings.NewReader(tc.input))
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, wantErr=%v", err, tc.err)
			}
			if !tc.err && pw != tc.want {
				t.Fatalf("got %q, want %q", pw, tc.want)
			}
		})
	}
}
