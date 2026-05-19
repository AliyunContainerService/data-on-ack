/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package server

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

func TestValidateCommit(t *testing.T) {
	cases := map[string]struct {
		req     *v1beta1.CommitRequest
		wantErr codes.Code
	}{
		"nil":         {req: nil, wantErr: codes.InvalidArgument},
		"empty id":    {req: &v1beta1.CommitRequest{ContainerID: " ", Image: "x:y"}, wantErr: codes.InvalidArgument},
		"empty image": {req: &v1beta1.CommitRequest{ContainerID: "abc", Image: ""}, wantErr: codes.InvalidArgument},
		"ok":          {req: &v1beta1.CommitRequest{ContainerID: "abc", Image: "x:y"}, wantErr: codes.OK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateCommit(tc.req)
			got := status.Code(err)
			if got != tc.wantErr {
				t.Fatalf("got code=%s, want %s (err=%v)", got, tc.wantErr, err)
			}
		})
	}
}

func TestValidatePush(t *testing.T) {
	if err := validatePush(nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil: got %v", err)
	}
	if err := validatePush(&v1beta1.PushRequest{Image: "  "}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty: got %v", err)
	}
	if err := validatePush(&v1beta1.PushRequest{Image: "ok:tag"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestAcquire_RespectsCancel(t *testing.T) {
	sem := make(chan struct{}, 1)
	sem <- struct{}{} // saturate

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	if _, err := acquire(ctx, sem); err == nil {
		t.Fatalf("expected ctx error, got nil")
	}
}

func TestLockContainer_Cleanup(t *testing.T) {
	s := &ImageServer{perContainer: map[string]*containerLock{}}
	unlock := s.lockContainer("abc")
	if got := s.perContainer["abc"]; got == nil || got.refcount != 1 {
		t.Fatalf("expected refcount=1, got %+v", got)
	}
	unlock()
	if _, ok := s.perContainer["abc"]; ok {
		t.Fatalf("expected map entry to be deleted after release")
	}
}

func TestLockContainer_SerializesSameID(t *testing.T) {
	s := &ImageServer{perContainer: map[string]*containerLock{}}
	var (
		mu  sync.Mutex
		log []string
	)
	rec := func(s string) {
		mu.Lock()
		log = append(log, s)
		mu.Unlock()
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	start := make(chan struct{})

	go func() {
		defer wg.Done()
		<-start
		release := s.lockContainer("abc")
		rec("a-acquired")
		time.Sleep(50 * time.Millisecond)
		rec("a-releasing")
		release()
	}()
	go func() {
		defer wg.Done()
		<-start
		time.Sleep(10 * time.Millisecond) // arrive after A locks
		release := s.lockContainer("abc")
		rec("b-acquired")
		release()
	}()

	close(start)
	wg.Wait()

	if got := strings.Join(log, ","); got != "a-acquired,a-releasing,b-acquired" {
		t.Fatalf("ordering wrong: %s", got)
	}
}

func TestRecoveryInterceptor(t *testing.T) {
	s := &ImageServer{}
	intc := s.recoveryInterceptor()
	_, err := intc(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/x/y"}, func(ctx context.Context, req any) (any, error) {
		panic("boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestTimeoutInterceptor_AddsDeadline(t *testing.T) {
	s := &ImageServer{}
	intc := s.timeoutInterceptor(50 * time.Millisecond)
	_, err := intc(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/x/y"}, func(ctx context.Context, _ any) (any, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("no deadline injected")
		}
		if time.Until(dl) > 100*time.Millisecond {
			return nil, errors.New("deadline too far")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTimeoutInterceptor_PreservesExistingDeadline(t *testing.T) {
	s := &ImageServer{}
	intc := s.timeoutInterceptor(time.Hour)
	caller, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := intc(caller, nil, &grpc.UnaryServerInfo{FullMethod: "/x/y"}, func(ctx context.Context, _ any) (any, error) {
		dl, _ := ctx.Deadline()
		if time.Until(dl) > 100*time.Millisecond {
			return nil, errors.New("interceptor extended caller's deadline")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestShortMethod(t *testing.T) {
	if got := shortMethod("/v1beta1.ImageService/CommitImage"); got != "CommitImage" {
		t.Fatalf("got %q", got)
	}
	if got := shortMethod("CommitImage"); got != "CommitImage" {
		t.Fatalf("got %q", got)
	}
}
