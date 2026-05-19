/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
 */

package utils

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// containerIDPattern matches a 64-hex-char container ID (the canonical form
// emitted by both docker/containerd) so it can be plucked out of a cgroup
// path regardless of the surrounding scope/slice format.
var containerIDPattern = regexp.MustCompile(`[0-9a-f]{64}`)

// ReadCgroupLine reads /proc/self/cgroup and returns the line that most
// likely identifies the current container.
//
//   - cgroup v1: prefer the "name=systemd" controller line, falling back to
//     any line whose path contains a 64-hex container ID.
//   - cgroup v2: there is exactly one line of the form `0::/...` so we just
//     return it.
//
// An error is returned only when the file can't be opened or no usable line
// is found.
func ReadCgroupLine(fileName string) (string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", fileName, err)
	}
	defer f.Close()

	var (
		systemdLine string
		v2Line      string
		anyHexLine  string
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "0::"):
			v2Line = line
		case strings.Contains(line, "name=systemd"):
			systemdLine = line
		default:
			if anyHexLine == "" && containerIDPattern.MatchString(line) {
				anyHexLine = line
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan %s: %w", fileName, err)
	}

	switch {
	case systemdLine != "":
		return systemdLine, nil
	case v2Line != "":
		return v2Line, nil
	case anyHexLine != "":
		return anyHexLine, nil
	}
	return "", errors.New("can't get container id from cgroup file")
}

// GetContainerID extracts the 64-hex container ID from a cgroup path. It is
// resilient to v1/v2 layout differences and to systemd's `docker-<id>.scope`,
// `cri-containerd-<id>.scope`, and `crio-<id>.scope` naming conventions.
func GetContainerID(cgroupContent string) string {
	if id := containerIDPattern.FindString(cgroupContent); id != "" {
		return id
	}
	// Last-resort fallback: keep the legacy parsing in case the runtime
	// emits a non-hex ID. This preserves previous behaviour rather than
	// returning empty.
	parts := strings.Split(cgroupContent, "/")
	tail := parts[len(parts)-1]
	tail = strings.TrimSuffix(tail, ".scope")
	if idx := strings.LastIndex(tail, "-"); idx >= 0 {
		tail = tail[idx+1:]
	}
	return tail
}
