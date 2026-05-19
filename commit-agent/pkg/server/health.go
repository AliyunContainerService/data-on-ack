/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package server

import (
	"context"
	"sync"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// healthService is a small implementation of the gRPC Health Checking
// Protocol. The full upstream package (google.golang.org/grpc/health) is not
// vendored, so we implement just what we need: per-service status with a
// simple Watch loop driven by a broadcast channel.
type healthService struct {
	healthpb.UnimplementedHealthServer

	mu        sync.RWMutex
	statuses  map[string]healthpb.HealthCheckResponse_ServingStatus
	listeners map[string]map[chan healthpb.HealthCheckResponse_ServingStatus]struct{}
}

func newHealthService() *healthService {
	return &healthService{
		statuses:  map[string]healthpb.HealthCheckResponse_ServingStatus{},
		listeners: map[string]map[chan healthpb.HealthCheckResponse_ServingStatus]struct{}{},
	}
}

func (h *healthService) set(service string, st healthpb.HealthCheckResponse_ServingStatus) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statuses[service] = st
	for ch := range h.listeners[service] {
		select {
		case ch <- st:
		default:
		}
	}
}

// Check implements healthpb.HealthServer.
func (h *healthService) Check(_ context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if st, ok := h.statuses[req.Service]; ok {
		return &healthpb.HealthCheckResponse{Status: st}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

// Watch streams updates for the requested service. It honors stream
// cancellation and sends an immediate snapshot of the current status.
func (h *healthService) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	ch := make(chan healthpb.HealthCheckResponse_ServingStatus, 1)

	h.mu.Lock()
	if _, ok := h.listeners[req.Service]; !ok {
		h.listeners[req.Service] = map[chan healthpb.HealthCheckResponse_ServingStatus]struct{}{}
	}
	h.listeners[req.Service][ch] = struct{}{}
	st, ok := h.statuses[req.Service]
	h.mu.Unlock()

	if ok {
		ch <- st
	} else {
		ch <- healthpb.HealthCheckResponse_SERVICE_UNKNOWN
	}

	defer func() {
		h.mu.Lock()
		delete(h.listeners[req.Service], ch)
		h.mu.Unlock()
	}()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case st := <-ch:
			if err := stream.Send(&healthpb.HealthCheckResponse{Status: st}); err != nil {
				return err
			}
		}
	}
}
