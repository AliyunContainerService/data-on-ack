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

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/operate"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

const (
	// netProtocol is the Unix Domain Socket network type used for the listener.
	netProtocol = "unix"
	// fallbackVersion is reported when the binary was built without -ldflags.
	fallbackVersion = "0.1.0"
	// requestIDKey is the metadata key clients can set to correlate logs.
	requestIDKey = "x-request-id"
	// healthServiceName is the service name reported in gRPC health checks.
	healthServiceName = "v1beta1.ImageService"
)

// Config drives runtime behaviour of the gRPC image server.
type Config struct {
	SocketAddress       string
	Version             string
	GitCommit           string
	RequestTimeout      time.Duration
	MaxConcurrentCommit int
	MaxConcurrentPush   int
	ContainerdNamespace string
	RuntimeOverride     string
	// AllowedUIDs restricts which Unix peer UIDs may invoke the agent.
	// Empty/nil means "allow everyone with access to the socket" (the
	// pre-existing behaviour).
	AllowedUIDs []uint32
	// Metrics, if non-nil, enables observation of RPC counts/latencies.
	Metrics *Registry
	// PushRetry overrides the runtime push retry policy. Zero values pick
	// sensible defaults inside operate.NewRuntime.
	PushRetry operate.RetryPolicy
}

// ImageServer is the gRPC handler implementing v1beta1.ImageServiceServer.
type ImageServer struct {
	v1beta1.UnimplementedImageServiceServer

	cfg          Config
	listener     net.Listener
	grpc         *grpc.Server
	health       *healthService
	commitSem    chan struct{}
	pushSem      chan struct{}
	containerMu  sync.Mutex
	perContainer map[string]*containerLock
	runtime      *operate.Runtime
	metrics      *Registry
}

// containerLock is a refcounted mutex so we can drop entries from
// perContainer once the last waiter releases.
type containerLock struct {
	mu       sync.Mutex
	refcount int
}

// New validates configuration and returns an ImageServer ready to start.
func New(cfg Config) (*ImageServer, error) {
	if cfg.SocketAddress == "" {
		return nil, errors.New("socket address is required")
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Minute
	}
	if cfg.MaxConcurrentCommit <= 0 {
		cfg.MaxConcurrentCommit = 4
	}
	if cfg.MaxConcurrentPush <= 0 {
		cfg.MaxConcurrentPush = 4
	}
	if cfg.Version == "" {
		cfg.Version = fallbackVersion
	}
	if cfg.ContainerdNamespace == "" {
		cfg.ContainerdNamespace = "k8s.io"
	}

	rt, err := operate.NewRuntime(operate.RuntimeOptions{
		Override:            cfg.RuntimeOverride,
		ContainerdNamespace: cfg.ContainerdNamespace,
		PushRetry:           cfg.PushRetry,
	})
	if err != nil {
		return nil, fmt.Errorf("init container runtime: %w", err)
	}

	registerMetricHelp(cfg.Metrics)

	return &ImageServer{
		cfg:          cfg,
		commitSem:    make(chan struct{}, cfg.MaxConcurrentCommit),
		pushSem:      make(chan struct{}, cfg.MaxConcurrentPush),
		perContainer: make(map[string]*containerLock),
		runtime:      rt,
		metrics:      cfg.Metrics,
	}, nil
}

func registerMetricHelp(r *Registry) {
	if r == nil {
		return
	}
	r.docHelp("commit_agent_rpc_total", "Total number of gRPC requests by method and result code.")
	r.docHelp("commit_agent_rpc_duration_seconds", "Latency of gRPC handlers in seconds.")
	r.docHelp("commit_agent_rpc_inflight", "Number of in-flight gRPC requests.")
	r.docHelp("commit_agent_runtime_info", "Resolved container runtime kind (1 = active).")
}

func (s *ImageServer) setupRPCServer() error {
	if err := s.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, s.cfg.SocketAddress)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	// Loosen permissions so clients running as the notebook user can dial.
	if err := os.Chmod(s.cfg.SocketAddress, 0o666); err != nil {
		log.Warnf("could not chmod socket %s: %v", s.cfg.SocketAddress, err)
	}
	s.listener = listener
	log.Infof("registered unix domain socket: %s", s.cfg.SocketAddress)

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			s.recoveryInterceptor(),
			s.requestIDInterceptor(),
			s.loggingInterceptor(),
			s.metricsInterceptor(),
			s.authInterceptor(),
			s.timeoutInterceptor(s.cfg.RequestTimeout),
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              30 * time.Second,
			Timeout:           10 * time.Second,
		}),
	}
	if creds := newPeerCredentials(); creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	srv := grpc.NewServer(opts...)
	v1beta1.RegisterImageServiceServer(srv, s)

	hs := newHealthService()
	hs.set("", healthpb.HealthCheckResponse_SERVING)
	hs.set(healthServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)

	if s.metrics != nil {
		s.metrics.IncGauge("commit_agent_runtime_info", map[string]string{
			"runtime": string(s.runtime.Kind()),
		})
	}

	s.grpc = srv
	s.health = hs
	return nil
}

// StartRPCServer starts serving in a goroutine and returns the underlying
// *grpc.Server so callers can perform graceful shutdown. The error channel is
// closed when Serve returns.
func (s *ImageServer) StartRPCServer() (*grpc.Server, chan error) {
	errorChan := make(chan error, 1)
	if err := s.setupRPCServer(); err != nil {
		errorChan <- err
		close(errorChan)
		return nil, errorChan
	}

	go func() {
		defer close(errorChan)
		if err := s.grpc.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errorChan <- err
		}
	}()
	log.Infof("commit-agent gRPC server started on %s (version=%s commit=%s runtime=%s)",
		s.cfg.SocketAddress, s.cfg.Version, s.cfg.GitCommit, s.runtime.Kind())
	return s.grpc, errorChan
}

func (s *ImageServer) cleanSockFile() error {
	if err := unix.Unlink(s.cfg.SocketAddress); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete socket file: %w", err)
	}
	return nil
}

// Version implements the gRPC method.
func (s *ImageServer) Version(_ context.Context, _ *v1beta1.VersionRequest) (*v1beta1.VersionResponse, error) {
	return &v1beta1.VersionResponse{Version: s.cfg.Version}, nil
}

// CommitImage implements the gRPC method.
func (s *ImageServer) CommitImage(ctx context.Context, req *v1beta1.CommitRequest) (*v1beta1.CommitResponse, error) {
	if err := validateCommit(req); err != nil {
		return nil, err
	}

	release, err := acquire(ctx, s.commitSem)
	if err != nil {
		return nil, err
	}
	defer release()

	unlock := s.lockContainer(req.ContainerID)
	defer unlock()

	result, err := s.runtime.Commit(ctx, req.ContainerID, req.Image)
	if err != nil {
		return &v1beta1.CommitResponse{Result: result}, status.Errorf(codes.Internal, "commit failed: %v", err)
	}
	return &v1beta1.CommitResponse{Result: result}, nil
}

// PushImage implements the gRPC method.
func (s *ImageServer) PushImage(ctx context.Context, req *v1beta1.PushRequest) (*v1beta1.PushResponse, error) {
	if err := validatePush(req); err != nil {
		return nil, err
	}

	release, err := acquire(ctx, s.pushSem)
	if err != nil {
		return nil, err
	}
	defer release()

	result, err := s.runtime.Push(ctx, req.Image, req.Username, req.Password)
	if err != nil {
		return &v1beta1.PushResponse{Result: result}, status.Errorf(codes.Internal, "push failed: %v", err)
	}
	return &v1beta1.PushResponse{Result: result}, nil
}

func validateCommit(req *v1beta1.CommitRequest) error {
	if req == nil || strings.TrimSpace(req.ContainerID) == "" {
		return status.Error(codes.InvalidArgument, "containerID must not be empty")
	}
	if strings.TrimSpace(req.Image) == "" {
		return status.Error(codes.InvalidArgument, "image must not be empty")
	}
	return nil
}

func validatePush(req *v1beta1.PushRequest) error {
	if req == nil || strings.TrimSpace(req.Image) == "" {
		return status.Error(codes.InvalidArgument, "image must not be empty")
	}
	return nil
}

// acquire reserves one slot from the supplied semaphore, respecting context
// cancellation.
func acquire(ctx context.Context, sem chan struct{}) (func(), error) {
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

// lockContainer returns an unlock function that holds a mutex unique to this
// container ID. Concurrent commits on the same container serialise; different
// containers proceed in parallel. The map entry is reference-counted and
// removed when the final waiter releases — this keeps memory bounded on
// long-lived nodes.
func (s *ImageServer) lockContainer(id string) func() {
	s.containerMu.Lock()
	cl, ok := s.perContainer[id]
	if !ok {
		cl = &containerLock{}
		s.perContainer[id] = cl
	}
	cl.refcount++
	s.containerMu.Unlock()

	cl.mu.Lock()

	return func() {
		cl.mu.Unlock()
		s.containerMu.Lock()
		cl.refcount--
		if cl.refcount == 0 {
			delete(s.perContainer, id)
		}
		s.containerMu.Unlock()
	}
}

// --- Interceptors ----------------------------------------------------------

func (s *ImageServer) recoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.WithFields(log.Fields{
					"method": info.FullMethod,
					"panic":  fmt.Sprint(r),
					"stack":  string(debug.Stack()),
				}).Error("recovered from panic in grpc handler")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func (s *ImageServer) requestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var rid string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := md.Get(requestIDKey); len(v) > 0 {
				rid = v[0]
			}
		}
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx = context.WithValue(ctx, requestIDCtxKey{}, rid)
		return handler(ctx, req)
	}
}

func (s *ImageServer) loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		entry := log.WithFields(log.Fields{
			"method":     info.FullMethod,
			"request_id": requestIDFromCtx(ctx),
		})
		entry.Debug("rpc started")

		resp, err := handler(ctx, req)

		fields := log.Fields{
			"latency_ms": time.Since(started).Milliseconds(),
			"code":       status.Code(err).String(),
		}
		switch code := status.Code(err); {
		case err == nil:
			entry.WithFields(fields).Info("rpc completed")
		case code == codes.Canceled, code == codes.DeadlineExceeded:
			entry.WithFields(fields).WithError(err).Info("rpc canceled")
		default:
			entry.WithFields(fields).WithError(err).Warn("rpc failed")
		}
		return resp, err
	}
}

// metricsInterceptor records request count + latency to the registry.
func (s *ImageServer) metricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if s.metrics == nil {
			return handler(ctx, req)
		}
		method := shortMethod(info.FullMethod)
		s.metrics.IncGauge("commit_agent_rpc_inflight", map[string]string{"method": method})
		defer s.metrics.DecGauge("commit_agent_rpc_inflight", map[string]string{"method": method})

		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		s.metrics.IncCounter("commit_agent_rpc_total", map[string]string{
			"method": method,
			"code":   code,
		})
		s.metrics.ObserveSeconds("commit_agent_rpc_duration_seconds",
			map[string]string{"method": method},
			time.Since(start).Seconds())
		return resp, err
	}
}

// authInterceptor enforces AllowedUIDs when configured. It pulls peer
// credentials stashed by the SO_PEERCRED transport wrapper.
func (s *ImageServer) authInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if len(s.cfg.AllowedUIDs) == 0 {
			return handler(ctx, req)
		}
		p, ok := peer.FromContext(ctx)
		if !ok || p.AuthInfo == nil {
			return nil, status.Error(codes.Unauthenticated, "no peer credentials present")
		}
		creds, ok := p.AuthInfo.(peerCredAuthInfo)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing peer credentials")
		}
		for _, allowed := range s.cfg.AllowedUIDs {
			if creds.UID == allowed {
				return handler(ctx, req)
			}
		}
		log.WithFields(log.Fields{
			"peer_uid": creds.UID,
			"peer_pid": creds.PID,
			"method":   info.FullMethod,
		}).Warn("rejecting RPC: caller UID not allowlisted")
		return nil, status.Error(codes.PermissionDenied, "caller UID not allowlisted")
	}
}

func (s *ImageServer) timeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

func shortMethod(full string) string {
	// /v1beta1.ImageService/CommitImage -> CommitImage
	if i := strings.LastIndex(full, "/"); i >= 0 && i < len(full)-1 {
		return full[i+1:]
	}
	return full
}

type requestIDCtxKey struct{}

func requestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}
