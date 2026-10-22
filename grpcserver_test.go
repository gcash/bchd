// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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

// TestGrpcHTTPServerTimeouts ensures the gRPC HTTP server closes connections
// that stall while sending headers and keeps long-lived streams usable.
func TestGrpcHTTPServerTimeouts(t *testing.T) {
	savedHeader, savedIdle := grpcReadHeaderTimeout, grpcIdleTimeout
	t.Cleanup(func() { grpcReadHeaderTimeout, grpcIdleTimeout = savedHeader, savedIdle })
	grpcReadHeaderTimeout = 200 * time.Millisecond
	grpcIdleTimeout = 200 * time.Millisecond

	srv := newGrpcHTTPServer("127.0.0.1:0", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {}))
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatal("whole-request timeouts would cut off streams")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { srv.Close() })

	for _, partial := range []string{"", "GET / HTTP/1.1\r\n"} {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = conn.Write([]byte(partial))
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, err = io.ReadAll(conn)
		conn.Close()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			t.Errorf("connection after %q still open", partial)
		}
	}
}

// TestGrpcErrorLogLevel ensures a call that fails with an answer for the
// caller, such as NotFound for a proof a client polls for, a subscription the
// client ended, an SLP transaction refused with Aborted or a call needing an
// index the node does not run, logs at debug rather than as an error, so a
// client cannot flood the log, while other failures still log errors.
func TestGrpcErrorLogLevel(t *testing.T) {
	setGrpcTestConfig(t, "tok", false)
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	logger.SetLevel(bchlog.LevelDebug)
	grpcLog = logger
	ctx := peer.NewContext(tokenContext("tok"), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
	})

	for _, test := range []struct {
		err   error
		level string
	}{
		{status.Error(codes.NotFound, "x"), "[DBG]"},
		{status.Error(codes.InvalidArgument, "x"), "[DBG]"},
		{status.Error(codes.Canceled, "x"), "[DBG]"},
		{context.Canceled, "[DBG]"},
		{status.Error(codes.Aborted, "x"), "[DBG]"},
		{status.Error(codes.Unavailable, "x"), "[DBG]"},
		{status.Error(codes.Internal, "x"), "[ERR]"},
		{errors.New("x"), "[ERR]"},
	} {
		buf.Reset()
		unary := func(context.Context, interface{}) (interface{}, error) {
			return nil, test.err
		}
		info := &grpc.UnaryServerInfo{FullMethod: readyGrpcMethod}
		_, _ = interceptUnary(ctx, nil, info, unary)
		want := test.level + " GRPC: Unary method " + readyGrpcMethod +
			" invoked by 127.0.0.1:1 errored"
		if !strings.Contains(buf.String(), want) {
			t.Errorf("%v: log %q, want a line with %q", test.err,
				buf.String(), want)
		}

		buf.Reset()
		stream := func(interface{}, grpc.ServerStream) error { return test.err }
		sinfo := &grpc.StreamServerInfo{FullMethod: readyGrpcMethod,
			IsServerStream: true}
		_ = interceptStreaming(nil, &grpcTestStream{ctx: ctx}, sinfo, stream)
		want = test.level + " GRPC: Streaming method " + readyGrpcMethod +
			" invoked by 127.0.0.1:1 errored"
		if !strings.Contains(buf.String(), want) {
			t.Errorf("%v: stream log %q, want a line with %q", test.err,
				buf.String(), want)
		}
	}
}

// TestGrpcCallLogLevel ensures the line naming each call and its caller is
// logged at debug, not info, so that at the default level the log does not
// grow with every call, made before the caller is authenticated.
func TestGrpcCallLogLevel(t *testing.T) {
	setGrpcTestConfig(t, "", false)
	var buf bytes.Buffer
	logger := bchlog.NewBackend(&buf).Logger("GRPC")
	grpcLog = logger
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
	})
	unary := func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: readyGrpcMethod}
	stream := func(interface{}, grpc.ServerStream) error { return nil }
	sinfo := &grpc.StreamServerInfo{FullMethod: readyGrpcMethod,
		IsServerStream: true}

	logger.SetLevel(bchlog.LevelInfo)
	_, _ = interceptUnary(ctx, nil, info, unary)
	_ = interceptStreaming(nil, &grpcTestStream{ctx: ctx}, sinfo, stream)
	if buf.Len() != 0 {
		t.Errorf("calls logged at the info level: %q", buf.String())
	}

	logger.SetLevel(bchlog.LevelDebug)
	_, _ = interceptUnary(ctx, nil, info, unary)
	_ = interceptStreaming(nil, &grpcTestStream{ctx: ctx}, sinfo, stream)
	for _, kind := range []string{"Unary", "Streaming"} {
		want := "[DBG] GRPC: " + kind + " method " + readyGrpcMethod +
			" invoked by 127.0.0.1:1\n"
		if !strings.Contains(buf.String(), want) {
			t.Errorf("debug log %q, want a line with %q", buf.String(), want)
		}
	}
}
