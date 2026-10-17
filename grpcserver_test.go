// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/gcash/bchlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// readyGrpcMethod is a method of a service that is always ready.
const readyGrpcMethod = "/grpc.reflection.v1alpha.ServerReflection/X"

// setGrpcTestConfig sets the auth token and whether metrics are enabled for
// the duration of the test.
func setGrpcTestConfig(t *testing.T, token string, prometheus bool) {
	t.Helper()
	savedCfg, savedProm, savedLog := cfg, prometheusEnabled, grpcLog
	t.Cleanup(func() { cfg, prometheusEnabled, grpcLog = savedCfg, savedProm, savedLog })
	cfg = &config{GrpcAuthToken: token}
	prometheusEnabled = prometheus
	grpcLog = bchlog.Disabled
}

// tokenContext returns an incoming context carrying the given auth token.
func tokenContext(token string) context.Context {
	return metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(AuthenticationTokenKey, token))
}

type grpcTestStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *grpcTestStream) Context() context.Context { return s.ctx }

// TestGrpcHandlerNotRunBeforeAuth ensures unary and stream handlers never run
// for a caller without the right token, with and without metrics.
func TestGrpcHandlerNotRunBeforeAuth(t *testing.T) {
	for _, prometheus := range []bool{false, true} {
		setGrpcTestConfig(t, "correct-token", prometheus)

		var calls int32
		unary := func(ctx context.Context, req interface{}) (interface{}, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		}
		info := &grpc.UnaryServerInfo{FullMethod: readyGrpcMethod}
		_, err := interceptUnary(tokenContext("wrong-token"), nil, info, unary)
		if err == nil {
			t.Errorf("prometheus=%v: unary call with a wrong token succeeded", prometheus)
		}

		stream := func(srv interface{}, ss grpc.ServerStream) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		sinfo := &grpc.StreamServerInfo{FullMethod: readyGrpcMethod,
			IsServerStream: true}
		ss := &grpcTestStream{ctx: context.Background()}
		if err := interceptStreaming(nil, ss, sinfo, stream); err == nil {
			t.Errorf("prometheus=%v: stream call without a token succeeded", prometheus)
		}
		if n := atomic.LoadInt32(&calls); n != 0 {
			t.Errorf("prometheus=%v: handler ran %d times before auth", prometheus, n)
		}
	}
}

// TestGrpcHandlerRunsOnce ensures an authenticated call runs its handler
// exactly once, with and without metrics.
func TestGrpcHandlerRunsOnce(t *testing.T) {
	for _, prometheus := range []bool{false, true} {
		setGrpcTestConfig(t, "correct-token", prometheus)

		var calls int32
		unary := func(ctx context.Context, req interface{}) (interface{}, error) {
			atomic.AddInt32(&calls, 1)
			return "ok", nil
		}
		info := &grpc.UnaryServerInfo{FullMethod: readyGrpcMethod}
		resp, err := interceptUnary(tokenContext("correct-token"), nil, info, unary)
		if err != nil || resp != "ok" {
			t.Errorf("prometheus=%v: resp %v, err %v", prometheus, resp, err)
		}

		stream := func(srv interface{}, ss grpc.ServerStream) error {
			atomic.AddInt32(&calls, 1)
			return nil
		}
		sinfo := &grpc.StreamServerInfo{FullMethod: readyGrpcMethod,
			IsServerStream: true}
		ss := &grpcTestStream{ctx: tokenContext("correct-token")}
		if err := interceptStreaming(nil, ss, sinfo, stream); err != nil {
			t.Errorf("prometheus=%v: stream err %v", prometheus, err)
		}
		if n := atomic.LoadInt32(&calls); n != 2 {
			t.Errorf("prometheus=%v: handlers ran %d times, want 2", prometheus, n)
		}
	}
}

// TestGrpcHandlerPanicRecovered ensures a panicking handler returns an
// Internal error instead of stopping the process.
func TestGrpcHandlerPanicRecovered(t *testing.T) {
	setGrpcTestConfig(t, "", true)

	unary := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("handler failure")
	}
	info := &grpc.UnaryServerInfo{FullMethod: readyGrpcMethod}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("unary handler panic escaped: %v", r)
			}
		}()
		_, err = interceptUnary(context.Background(), nil, info, unary)
	}()
	if status.Code(err) != codes.Internal {
		t.Errorf("unary: err %v, want Internal", err)
	}

	stream := func(srv interface{}, ss grpc.ServerStream) error {
		var s []int
		_ = s[1]
		return nil
	}
	sinfo := &grpc.StreamServerInfo{FullMethod: readyGrpcMethod}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("stream handler panic escaped: %v", r)
			}
		}()
		err = interceptStreaming(nil, &grpcTestStream{ctx: context.Background()},
			sinfo, stream)
	}()
	if status.Code(err) != codes.Internal {
		t.Errorf("stream: err %v, want Internal", err)
	}
}
