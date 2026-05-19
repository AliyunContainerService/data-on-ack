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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/operate"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/server"
)

// Version / GitCommit are injected at build time via -ldflags.
var (
	Version   = "dev"
	GitCommit = "none"
)

var (
	socketAddress       = flag.String("socket-address", "/host/run/commit-agent/commit-agent.sock", "the socket address which was listened by commit-agent server")
	logLevel            = flag.String("log-level", "info", "log level: panic, fatal, error, warn, info, debug, trace")
	logFormat           = flag.String("log-format", "text", "log format: text or json")
	requestTimeout      = flag.Duration("request-timeout", 30*time.Minute, "default per-RPC timeout (image push can be long, set generously)")
	maxConcurrentCommit = flag.Int("max-concurrent-commit", 4, "maximum number of concurrent CommitImage RPCs handled by the agent")
	maxConcurrentPush   = flag.Int("max-concurrent-push", 4, "maximum number of concurrent PushImage RPCs handled by the agent")
	shutdownTimeout     = flag.Duration("shutdown-timeout", 30*time.Second, "graceful shutdown deadline before forcefully stopping the gRPC server")
	containerNamespace  = flag.String("containerd-namespace", "k8s.io", "containerd namespace used to look up containers")
	runtimeOverride     = flag.String("runtime", "", "force container runtime: docker | containerd. Empty = auto-detect")
	allowedUIDs         = flag.String("allowed-uids", "", "comma-separated list of caller UIDs allowed to invoke the agent (empty = allow all peers reaching the socket)")
	metricsAddress      = flag.String("metrics-address", "", "address to expose Prometheus /metrics on, e.g. \":9090\". Empty = disabled")
	pushRetryAttempts   = flag.Int("push-retry-attempts", 3, "maximum push attempts before giving up; 1 disables retries")
	pushRetryInitial    = flag.Duration("push-retry-initial-delay", 2*time.Second, "initial backoff between push retries")
	pushRetryMax        = flag.Duration("push-retry-max-delay", 30*time.Second, "maximum backoff between push retries")
	showVersion         = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Parse()

	if *showVersion {
		log.Infof("commit-agent %s (%s)", Version, GitCommit)
		return
	}

	configureLogger(*logLevel, *logFormat)
	mustValidateFlags(*socketAddress)

	uids, err := parseUIDList(*allowedUIDs)
	if err != nil {
		log.Fatalf("invalid --allowed-uids: %v", err)
	}

	var registry *server.Registry
	var metricsSrv *server.MetricsServer
	if *metricsAddress != "" {
		registry = server.NewRegistry()
		metricsSrv = &server.MetricsServer{Registry: registry, Address: *metricsAddress}
	}

	cfg := server.Config{
		SocketAddress:       *socketAddress,
		Version:             Version,
		GitCommit:           GitCommit,
		RequestTimeout:      *requestTimeout,
		MaxConcurrentCommit: *maxConcurrentCommit,
		MaxConcurrentPush:   *maxConcurrentPush,
		ContainerdNamespace: *containerNamespace,
		RuntimeOverride:     *runtimeOverride,
		AllowedUIDs:         uids,
		Metrics:             registry,
		PushRetry: operate.RetryPolicy{
			MaxAttempts:  *pushRetryAttempts,
			InitialDelay: *pushRetryInitial,
			MaxDelay:     *pushRetryMax,
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to init commit-agent: %v", err)
	}

	var metricsErrCh <-chan error
	if metricsSrv != nil {
		metricsErrCh = metricsSrv.Start()
	}

	grpcSrv, errChan := srv.StartRPCServer()
	if grpcSrv == nil {
		log.Fatalf("commit-agent failed to start: %v", <-errChan)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	shutdown := func(reason string) {
		log.Infof("%s; initiating graceful shutdown (deadline=%s)", reason, *shutdownTimeout)
		stopped := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
			log.Infoln("commit-agent shut down cleanly")
		case <-time.After(*shutdownTimeout):
			log.Warnln("graceful shutdown deadline exceeded, forcing stop")
			grpcSrv.Stop()
		}
		if metricsSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = metricsSrv.Shutdown(ctx)
			cancel()
		}
	}

	select {
	case sig := <-signals:
		shutdown("captured " + sig.String())
	case err := <-errChan:
		if err != nil {
			log.Errorf("commit-agent runtime error: %v", err)
		}
		shutdown("grpc server returned")
	case err := <-metricsErrCh:
		if err != nil {
			log.Errorf("metrics server runtime error: %v", err)
		}
		shutdown("metrics server returned")
	}
}

func configureLogger(level, format string) {
	if lvl, err := log.ParseLevel(level); err == nil {
		log.SetLevel(lvl)
	} else {
		log.Warnf("invalid log-level %q, falling back to info", level)
		log.SetLevel(log.InfoLevel)
	}
	if strings.EqualFold(format, "json") {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
}

func mustValidateFlags(pathToUnixSocket string) {
	log.Infof("Checking socket path %s", pathToUnixSocket)
	if !strings.HasPrefix(pathToUnixSocket, "@") {
		socketDir := filepath.Dir(pathToUnixSocket)
		_, err := os.Stat(socketDir)
		log.Infof("Unix Socket directory is %s", socketDir)
		if err != nil && os.IsNotExist(err) {
			log.Infof(" Directory %s portion of socket-address flag: %s does not exist, create it.", socketDir, pathToUnixSocket)
			if err = os.MkdirAll(socketDir, 0o755); err != nil {
				log.Fatalf(" Directory %s create failed, err: %s", socketDir, err)
			}
		}
	}
	log.Infof("Communication between commit-cli and commit-agent will be via %s", pathToUnixSocket)
}

func parseUIDList(s string) ([]uint32, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	out := []uint32{}
	for _, raw := range strings.Split(s, ",") {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, err
		}
		out = append(out, uint32(n))
	}
	return out, nil
}
