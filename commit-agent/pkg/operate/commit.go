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

package operate

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/containerd"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/docker"
	_type "github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/type"
)

// RuntimeKind enumerates the supported container runtimes.
type RuntimeKind string

const (
	RuntimeDocker     RuntimeKind = "docker"
	RuntimeContainerd RuntimeKind = "containerd"

	// runtimeEnv allows operators to force a specific runtime, bypassing the
	// auto-detection that inspects the host sockets.
	runtimeEnv = "CONTAINER_RUNTIME"
)

// Default host-mounted runtime socket paths. Exposed as variables so tests can
// point selectRuntime at a tempdir.
var (
	defaultDockerSock     = "/host/run/docker.sock"
	defaultContainerdSock = "/host/run/containerd/containerd.sock"
)

// RuntimeOptions controls how the runtime singleton is constructed.
type RuntimeOptions struct {
	// Override forces a specific runtime ("docker" or "containerd"). When
	// empty, the package picks based on the presence of host sockets and the
	// CONTAINER_RUNTIME environment variable.
	Override string
	// ContainerdNamespace is the namespace used to look up containers in
	// containerd. Default "k8s.io".
	ContainerdNamespace string
	// PushRetry tunes the retry policy for push operations. Zero values use
	// sensible defaults; set MaxAttempts=1 to disable retries.
	PushRetry RetryPolicy
	// DockerSocket / ContainerdSocket override the auto-detection paths,
	// primarily for tests.
	DockerSocket     string
	ContainerdSocket string
}

// RetryPolicy describes a capped exponential backoff with jitter.
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.InitialDelay <= 0 {
		p.InitialDelay = 2 * time.Second
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 30 * time.Second
	}
	if p.Multiplier <= 1 {
		p.Multiplier = 2
	}
	return p
}

// Runtime is the long-lived handle to whichever container engine is in use.
// Clients are cached so each RPC reuses the same daemon connection.
type Runtime struct {
	kind RuntimeKind
	opts RuntimeOptions

	mu     sync.Mutex
	client _type.ContainerClient
}

// NewRuntime determines which runtime to talk to and validates the choice.
// Construction is eager but the underlying daemon client is created lazily on
// first use.
func NewRuntime(opts RuntimeOptions) (*Runtime, error) {
	if opts.ContainerdNamespace == "" {
		opts.ContainerdNamespace = "k8s.io"
	}
	if opts.DockerSocket == "" {
		opts.DockerSocket = defaultDockerSock
	}
	if opts.ContainerdSocket == "" {
		opts.ContainerdSocket = defaultContainerdSock
	}
	opts.PushRetry = opts.PushRetry.withDefaults()

	kind, err := selectRuntime(opts.Override, opts.DockerSocket, opts.ContainerdSocket)
	if err != nil {
		return nil, err
	}
	log.Infof("commit-agent will use %s runtime", kind)

	return &Runtime{kind: kind, opts: opts}, nil
}

// Kind returns the resolved runtime kind. Useful for diagnostics.
func (r *Runtime) Kind() RuntimeKind { return r.kind }

func (r *Runtime) ensureClient() (_type.ContainerClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		return r.client, nil
	}

	var (
		c   _type.ContainerClient
		err error
	)
	switch r.kind {
	case RuntimeDocker:
		c, err = docker.NewDockerClient()
	case RuntimeContainerd:
		c, err = containerd.NewContainerdClient(containerd.Options{
			Address:   r.opts.ContainerdSocket,
			Namespace: r.opts.ContainerdNamespace,
		})
	default:
		return nil, fmt.Errorf("unsupported runtime %q", r.kind)
	}
	if err != nil {
		return nil, fmt.Errorf("init %s client: %w", r.kind, err)
	}
	r.client = c
	return c, nil
}

// Commit takes a snapshot of the named container and tags it as `image`.
func (r *Runtime) Commit(ctx context.Context, containerID, image string) (string, error) {
	if strings.TrimSpace(containerID) == "" {
		return "container ID is empty", errors.New("container ID is empty")
	}
	if strings.TrimSpace(image) == "" {
		return "image is empty", errors.New("image is empty")
	}

	c, err := r.ensureClient()
	if err != nil {
		return fmt.Sprintf("%s client init error", r.kind), err
	}
	if err := c.CommitImageFromSelf(ctx, containerID, image); err != nil {
		return fmt.Sprintf("commit failed: %v", err), err
	}
	msg := fmt.Sprintf("Container committed successfully, image: %s", image)
	log.Infoln(msg)
	return msg, nil
}

// Push uploads the local image to its remote registry, authenticating with the
// supplied credentials. The push is retried with capped exponential backoff if
// the underlying registry returns a transient error.
func (r *Runtime) Push(ctx context.Context, image, username, password string) (string, error) {
	if strings.TrimSpace(image) == "" {
		return "image is empty", errors.New("image is empty")
	}
	c, err := r.ensureClient()
	if err != nil {
		return fmt.Sprintf("%s client init error", r.kind), err
	}

	policy := r.opts.PushRetry
	var lastErr error
	delay := policy.InitialDelay
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "context canceled before push", err
		}
		err := c.PushImageFromSelf(ctx, image, username, password)
		if err == nil {
			msg := fmt.Sprintf("Image pushed successfully: %s", image)
			if attempt > 1 {
				msg = fmt.Sprintf("%s (after %d attempts)", msg, attempt)
			}
			log.Infoln(msg)
			return msg, nil
		}
		lastErr = err
		if !isRetryablePushError(err) || attempt == policy.MaxAttempts {
			break
		}
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		wait := delay + jitter
		log.Warnf("push attempt %d/%d for %s failed: %v; retrying in %s",
			attempt, policy.MaxAttempts, image, err, wait)

		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return "context canceled during retry backoff", ctx.Err()
		case <-t.C:
		}
		if next := time.Duration(float64(delay) * policy.Multiplier); next > policy.MaxDelay {
			delay = policy.MaxDelay
		} else {
			delay = next
		}
	}
	return fmt.Sprintf("push failed: %v", lastErr), lastErr
}

// isRetryablePushError reports whether `err` looks transient enough to warrant
// another attempt. We're conservative: client-side errors (bad credentials,
// invalid image reference, etc.) must not trigger retries.
func isRetryablePushError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "denied"),
		strings.Contains(msg, "manifest invalid"),
		strings.Contains(msg, "blob unknown"),
		strings.Contains(msg, "name invalid"),
		strings.Contains(msg, "tag invalid"),
		strings.Contains(msg, "name unknown"),
		strings.Contains(msg, "no basic auth credentials"):
		return false
	}
	// Heuristic: 5xx, EOF, connection reset, timeout, temporary DNS issues.
	for _, marker := range []string{
		"500", "502", "503", "504",
		"timeout", "timed out",
		"connection refused", "connection reset",
		"eof", "no such host", "broken pipe",
		"i/o timeout", "temporary failure",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// selectRuntime resolves the runtime kind in the following precedence:
//  1. explicit `override` argument (if non-empty)
//  2. CONTAINER_RUNTIME environment variable
//  3. presence of dockerSock
//  4. presence of containerdSock
func selectRuntime(override, dockerSock, containerdSock string) (RuntimeKind, error) {
	if override == "" {
		override = os.Getenv(runtimeEnv)
	}
	switch strings.ToLower(strings.TrimSpace(override)) {
	case "docker":
		return RuntimeDocker, nil
	case "containerd":
		return RuntimeContainerd, nil
	case "":
		// fall through to auto-detection
	default:
		return "", fmt.Errorf("unsupported runtime override %q", override)
	}

	if pathExists(dockerSock) {
		return RuntimeDocker, nil
	}
	if pathExists(containerdSock) {
		return RuntimeContainerd, nil
	}
	return "", fmt.Errorf("no container runtime socket found at %s or %s; mount the host /run into the agent", dockerSock, containerdSock)
}

// pathExists reports whether the file/socket exists. Errors other than
// not-exist (e.g. permission denied) are treated as "exists" so that
// privileged-only sockets still trigger the right code path.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return !os.IsNotExist(err)
}
