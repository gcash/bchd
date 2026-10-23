// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// readyGrpcMethod is a method of a service that is always ready.
const readyGrpcMethod = "/grpc.reflection.v1alpha.ServerReflection/X"

// setGrpcTestConfig sets the auth token and whether metrics are enabled for
// the duration of the test.
func setGrpcTestConfig(t *testing.T, token string, prometheus bool) {
	t.Helper()
	savedCfg, savedMetrics, savedLog := cfg, grpcMetrics, grpcLog
	t.Cleanup(func() { cfg, grpcMetrics, grpcLog = savedCfg, savedMetrics, savedLog })
	cfg = &config{GrpcAuthToken: token}
	grpcMetrics = nil
	if prometheus {
		grpcMetrics = newGrpcServerMetrics()
	}
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
// client ended, an SLP transaction refused with Aborted, a call needing an
// index the node does not run, a limit the call reached or its deadline
// passing, logs at debug rather than as an error, so a client cannot flood
// the log, while other failures still log errors.
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
		{status.Error(codes.ResourceExhausted, "x"), "[DBG]"},
		{status.Error(codes.DeadlineExceeded, "x"), "[DBG]"},
		{context.DeadlineExceeded, "[DBG]"},
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

// grpcWriteTestService answers GetRawBlock with a block of replySize bytes,
// and SubscribeBlocks with a notification of replySize bytes at once and
// another each time notify receives.
type grpcWriteTestService struct {
	pb.UnimplementedBchrpcServer
	replySize int
	notify    chan struct{}
}

func (s *grpcWriteTestService) GetRawBlock(context.Context, *pb.GetRawBlockRequest) (*pb.GetRawBlockResponse, error) {
	return &pb.GetRawBlockResponse{Block: make([]byte, s.replySize)}, nil
}

func (s *grpcWriteTestService) SubscribeBlocks(_ *pb.SubscribeBlocksRequest, stream pb.Bchrpc_SubscribeBlocksServer) error {
	for {
		err := stream.Send(&pb.BlockNotification{
			Block: &pb.BlockNotification_SerializedBlock{
				SerializedBlock: make([]byte, s.replySize),
			},
		})
		if err != nil {
			return err
		}
		select {
		case <-s.notify:
		case <-stream.Context().Done():
			return nil
		}
	}
}

// grpcBufferListener sets small socket buffers on the connections it
// accepts, so that a client that stops reading stalls the server's writes
// soon.
type grpcBufferListener struct {
	net.Listener
}

func (l grpcBufferListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(16 << 10)
	}
	return conn, err
}

// startGrpcWriteTest serves svc over TLS with the gRPC listener's HTTP server
// and handler, with grpcWriteTimeout and grpcReadTimeout set to timeout, and
// returns the server's address and the number of requests its handler is
// serving.
func startGrpcWriteTest(t *testing.T, svc pb.BchrpcServer,
	timeout time.Duration) (string, *atomic.Int32) {

	t.Helper()
	return startGrpcTest(t, timeout, func(gs *grpc.Server) {
		pb.RegisterBchrpcServer(gs, svc)
	})
}

// startGrpcTest serves the services register registers on a gRPC server
// with opts as startGrpcWriteTest does.
func startGrpcTest(t *testing.T, timeout time.Duration,
	register func(*grpc.Server), opts ...grpc.ServerOption) (string, *atomic.Int32) {

	t.Helper()
	return startGrpcRepliesTest(t, timeout,
		newGrpcReplyBudget(grpcMaxReplyBytes), register, opts...)
}

// startGrpcRepliesTest is startGrpcTest with the replies written within the
// budget replies.
func startGrpcRepliesTest(t *testing.T, timeout time.Duration,
	replies *grpcReplyBudget, register func(*grpc.Server),
	opts ...grpc.ServerOption) (string, *atomic.Int32) {

	t.Helper()
	saved, savedRead := grpcWriteTimeout, grpcReadTimeout
	t.Cleanup(func() { grpcWriteTimeout, grpcReadTimeout = saved, savedRead })
	grpcWriteTimeout, grpcReadTimeout = timeout, timeout

	dir := t.TempDir()
	cert, key, err := bchutil.NewTLSCertPair("bchd test",
		time.Now().Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	certFile := filepath.Join(dir, "rpc.cert")
	keyFile := filepath.Join(dir, "rpc.key")
	if err := os.WriteFile(certFile, cert, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		t.Fatal(err)
	}

	gs := grpc.NewServer(opts...)
	register(gs)
	handler := grpcHTTPHandler(gs, replies)
	var active atomic.Int32
	srv := newGrpcHTTPServer("", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			active.Add(1)
			defer active.Add(-1)
			handler.ServeHTTP(w, r)
		}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.ServeTLS(grpcBufferListener{ln}, certFile, keyFile) }()
	t.Cleanup(func() {
		srv.Close()
		gs.Stop()
		// No handler may read the timeouts once they are restored.
		waitForGrpc(t, "the handlers to return", func() bool {
			return active.Load() == 0
		})
	})
	return ln.Addr().String(), &active
}

// recvGrpc waits up to 20 seconds for a value from ch.
func recvGrpc[T any](t *testing.T, what string, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

// waitForGrpc waits up to 20 seconds for cond.
func waitForGrpc(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// rawH2Client is an HTTP/2 client connection whose reads and flow control
// the test drives.
type rawH2Client struct {
	t    *testing.T
	conn *tls.Conn
	fr   *http2.Framer
}

// dialRawH2 connects to addr and sends the client preface and settings.
func dialRawH2(t *testing.T, addr string, settings ...http2.Setting) *rawH2Client {
	t.Helper()
	tcp, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = tcp.(*net.TCPConn).SetReadBuffer(16 << 10)
	conn := tls.Client(tcp, &tls.Config{InsecureSkipVerify: true,
		NextProtos: []string{"h2"}})
	t.Cleanup(func() { conn.Close() })
	if err := conn.Handshake(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
		t.Fatal(err)
	}
	c := &rawH2Client{t: t, conn: conn, fr: http2.NewFramer(conn, conn)}
	c.fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if err := c.fr.WriteSettings(settings...); err != nil {
		t.Fatal(err)
	}
	return c
}

// call starts a gRPC call of method on the stream with the request.
func (c *rawH2Client) call(stream uint32, method string, req proto.Message) {
	c.t.Helper()
	c.start(stream, method)
	msg, err := proto.Marshal(req)
	if err != nil {
		c.t.Fatal(err)
	}
	body := make([]byte, 5+len(msg))
	binary.BigEndian.PutUint32(body[1:5], uint32(len(msg)))
	copy(body[5:], msg)
	if err := c.fr.WriteData(stream, true, body); err != nil {
		c.t.Fatal(err)
	}
}

// start starts a gRPC call of method on the stream, sending its headers only.
func (c *rawH2Client) start(stream uint32, method string) {
	c.t.Helper()
	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, f := range [][2]string{{":method", "POST"}, {":scheme", "https"},
		{":authority", "localhost"}, {":path", method},
		{"content-type", "application/grpc"}, {"te", "trailers"}} {

		_ = enc.WriteField(hpack.HeaderField{Name: f[0], Value: f[1]})
	}
	err := c.fr.WriteHeaders(http2.HeadersFrameParam{StreamID: stream,
		BlockFragment: block.Bytes(), EndHeaders: true})
	if err != nil {
		c.t.Fatal(err)
	}
}

// next returns the next frame read before the deadline, or nil once the
// deadline passes, answering the server's settings and pings.  The frame is
// valid until the next read.
func (c *rawH2Client) next(deadline time.Time) http2.Frame {
	c.t.Helper()
	for {
		_ = c.conn.SetReadDeadline(deadline)
		f, err := c.fr.ReadFrame()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil
		}
		if err != nil {
			c.t.Fatalf("reading a frame: %v", err)
		}
		switch f := f.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				if err := c.fr.WriteSettingsAck(); err != nil {
					c.t.Fatal(err)
				}
			}
			continue
		case *http2.PingFrame:
			if !f.IsAck() {
				if err := c.fr.WritePing(true, f.Data); err != nil {
					c.t.Fatal(err)
				}
			}
			continue
		case *http2.WindowUpdateFrame:
			continue
		}
		return f
	}
}

// TestGrpcStalledReaderReset ensures that a call whose client stops granting
// flow control while its reply is written, unary or streaming, large or small,
// is reset once the client has taken none of it for grpcWriteTimeout, so that
// the call's goroutines and the reply are not held for good.
func TestGrpcStalledReaderReset(t *testing.T) {
	const timeout = 500 * time.Millisecond
	for _, test := range []struct {
		method string
		size   int
	}{
		{"/pb.bchrpc/GetRawBlock", 1 << 20},
		// A small reply waits in the writer's buffer until it is flushed.
		{"/pb.bchrpc/GetRawBlock", 100},
		{"/pb.bchrpc/SubscribeBlocks", 1 << 20},
	} {
		method := test.method
		svc := &grpcWriteTestService{replySize: test.size,
			notify: make(chan struct{})}
		addr, active := startGrpcWriteTest(t, svc, timeout)

		// The client grants no window, so none of the reply is sent.
		c := dialRawH2(t, addr, http2.Setting{
			ID: http2.SettingInitialWindowSize, Val: 0})
		var req proto.Message = &pb.GetRawBlockRequest{}
		if method == "/pb.bchrpc/SubscribeBlocks" {
			req = &pb.SubscribeBlocksRequest{}
		}
		c.call(1, method, req)
		start := time.Now()
		reset := false
		for !reset {
			f := c.next(start.Add(10 * timeout))
			if f == nil {
				break
			}
			if _, ok := f.(*http2.DataFrame); ok {
				t.Fatalf("%s, %d bytes: data sent without a window",
					method, test.size)
			}
			_, reset = f.(*http2.RSTStreamFrame)
		}
		if !reset {
			t.Errorf("%s, %d bytes: stream not reset after %v", method,
				test.size, time.Since(start))
			continue
		}
		if took := time.Since(start); took < timeout {
			t.Errorf("%s, %d bytes: reset after %v, before the timeout",
				method, test.size, took)
		}
		waitForGrpc(t, method+" to end", func() bool {
			return active.Load() == 0
		})
	}
}

// TestGrpcSlowReaderKeepsCall ensures that a client that takes a reply slowly
// but steadily gets all of it, although it takes several times
// grpcWriteTimeout, since the deadline is renewed as the reply goes out.
func TestGrpcSlowReaderKeepsCall(t *testing.T) {
	const timeout = 500 * time.Millisecond
	const size = 1 << 20
	svc := &grpcWriteTestService{replySize: size}
	addr, _ := startGrpcWriteTest(t, svc, timeout)

	// The client takes 16 KiB every 40 ms, so 64 KiB in 160 ms and the
	// whole reply in about 2.6 s.
	const window = 16 << 10
	c := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: window})
	c.call(1, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
	start := time.Now()
	received := 0
	for {
		f := c.next(time.Now().Add(10 * timeout))
		switch f := f.(type) {
		case nil:
			t.Fatalf("no frame after %d bytes", received)
		case *http2.RSTStreamFrame:
			t.Fatalf("stream reset after %d bytes in %v", received,
				time.Since(start))
		case *http2.DataFrame:
			n := len(f.Data())
			received += n
			if n == 0 {
				continue
			}
			time.Sleep(time.Duration(n) * 40 * time.Millisecond / window)
			if err := c.fr.WriteWindowUpdate(1, uint32(n)); err != nil {
				t.Fatal(err)
			}
			if err := c.fr.WriteWindowUpdate(0, uint32(n)); err != nil {
				t.Fatal(err)
			}
		case *http2.MetaHeadersFrame:
			if !f.StreamEnded() {
				continue
			}
			if got := f.PseudoValue("status"); got != "" {
				t.Fatalf("trailers with status %q", got)
			}
			var grpcStatus string
			for _, hf := range f.Fields {
				if hf.Name == "grpc-status" {
					grpcStatus = hf.Value
				}
			}
			if grpcStatus != "0" || received < size {
				t.Fatalf("call ended with grpc-status %q after %d bytes",
					grpcStatus, received)
			}
			if took := time.Since(start); took < 2*timeout {
				t.Fatalf("the reply took %v, not long enough to test",
					took)
			}
			return
		}
	}
}

// TestGrpcIdleStreamKept ensures that a subscription that has nothing to send
// for longer than grpcWriteTimeout is kept, since the deadline only runs
// while a write is in progress.
func TestGrpcIdleStreamKept(t *testing.T) {
	const timeout = 300 * time.Millisecond
	svc := &grpcWriteTestService{replySize: 256 << 10,
		notify: make(chan struct{})}
	addr, _ := startGrpcWriteTest(t, svc, timeout)

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stream, err := pb.NewBchrpcClient(cc).SubscribeBlocks(ctx,
		&pb.SubscribeBlocksRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := stream.Recv(); err != nil {
			t.Fatalf("notification %d: %v", i, err)
		}
		time.Sleep(3 * timeout)
		svc.notify <- struct{}{}
	}
}

// TestGrpcStalledConnectionClosed ensures that a connection to which nothing
// can be written for grpcWriteTimeout, as when the client stops reading from
// its socket, is closed, so that its calls end.
func TestGrpcStalledConnectionClosed(t *testing.T) {
	const timeout = 500 * time.Millisecond
	svc := &grpcWriteTestService{replySize: 32 << 20}
	addr, active := startGrpcWriteTest(t, svc, timeout)

	// Flow control lets the whole reply out at once, so only the socket
	// holds it back.
	c := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: 1<<31 - 1})
	if err := c.fr.WriteWindowUpdate(0, 1<<31-1-65535); err != nil {
		t.Fatal(err)
	}
	c.call(1, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
	for {
		f := c.next(time.Now().Add(10 * time.Second))
		if f == nil {
			t.Fatal("no reply headers")
		}
		if _, ok := f.(*http2.MetaHeadersFrame); ok {
			break
		}
	}

	// From here on the client reads nothing.  Closing the connection then
	// takes up to 5 s more, as crypto/tls gives the close_notify alert that
	// long to be written.
	waitForGrpc(t, "the call to end", func() bool {
		return active.Load() == 0
	})
}

// TestGrpcWebStalledReaderEnds ensures that a gRPC-Web call over HTTP/1.1
// whose client stops reading its reply ends once the client has taken none of
// it for grpcWriteTimeout, as gRPC-Web replies go through the same deadlines.
func TestGrpcWebStalledReaderEnds(t *testing.T) {
	const timeout = 500 * time.Millisecond
	svc := &grpcWriteTestService{replySize: 32 << 20}
	addr, active := startGrpcWriteTest(t, svc, timeout)

	tcp, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = tcp.(*net.TCPConn).SetReadBuffer(16 << 10)
	conn := tls.Client(tcp, &tls.Config{InsecureSkipVerify: true,
		NextProtos: []string{"http/1.1"}})
	defer conn.Close()
	body := []byte{0, 0, 0, 0, 0} // an empty GetRawBlockRequest
	req, err := http.NewRequest("POST", "https://localhost/pb.bchrpc/GetRawBlock",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/grpc-web+proto")
	req.Header.Set("X-Grpc-Web", "1")
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK ||
		resp.Header.Get("Content-Type") != "application/grpc-web+proto" {

		t.Fatalf("status %d, content type %q", resp.StatusCode,
			resp.Header.Get("Content-Type"))
	}

	// From here on the client reads nothing.  Closing the connection then
	// takes up to 5 s more, as crypto/tls gives the close_notify alert that
	// long to be written.
	waitForGrpc(t, "the call to end", func() bool {
		return active.Load() == 0
	})
}

// TestGrpcWebCalls ensures that the gRPC-Web wrapper serves calls as a
// browser makes them, in the binary and the text format: a unary call gets
// its reply and its status in gRPC-Web frames, a failed call gets its status
// in the headers, a server stream gets each message as it is sent, every
// reply carries the CORS headers a browser needs to read it, and a CORS
// preflight is answered for any origin.
func TestGrpcWebCalls(t *testing.T) {
	const origin = "https://wallet.example"
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		// HTTP/1.1, where only the wrapper's flushes get a stream's
		// messages out as they are sent.
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
	}}

	// post sends method an empty request in the given format from origin,
	// checks the reply's status and headers, and returns its body, as
	// gRPC-Web frames, and its headers.  The body is closed when the test
	// ends.
	post := func(addr, method, contentType string) (io.Reader, http.Header) {
		t.Helper()
		var body io.Reader = bytes.NewReader([]byte{0, 0, 0, 0, 0})
		text := contentType == "application/grpc-web-text"
		if text {
			body = strings.NewReader("AAAAAAA=") // the same five bytes
		}
		req, err := http.NewRequest("POST", "https://"+addr+"/pb.bchrpc/"+method,
			body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Grpc-Web", "1")
		req.Header.Set("Origin", origin)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		if resp.StatusCode != http.StatusOK ||
			resp.Header.Get("Content-Type") != contentType {

			t.Fatalf("%s: status %d, content type %q", method,
				resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		exposed := strings.ToLower(resp.Header.Get("Access-Control-Expose-Headers"))
		if resp.Header.Get("Access-Control-Allow-Origin") != origin ||
			!strings.Contains(exposed, "grpc-status") {

			t.Fatalf("%s: Access-Control-Allow-Origin %q, "+
				"Access-Control-Expose-Headers %q", method,
				resp.Header.Get("Access-Control-Allow-Origin"), exposed)
		}
		if text {
			return &grpcWebTextReader{r: resp.Body}, resp.Header
		}
		return resp.Body, resp.Header
	}

	// readFrame reads one gRPC-Web frame: its flags and its payload.
	readFrame := func(r io.Reader) (byte, []byte) {
		t.Helper()
		var header [5]byte
		if _, err := io.ReadFull(r, header[:]); err != nil {
			t.Fatalf("frame header: %v", err)
		}
		payload := make([]byte, binary.BigEndian.Uint32(header[1:]))
		if _, err := io.ReadFull(r, payload); err != nil {
			t.Fatalf("frame payload: %v", err)
		}
		return header[0], payload
	}

	for _, contentType := range []string{"application/grpc-web+proto",
		"application/grpc-web-text"} {

		// A service and server per format, so that one format's stream
		// cannot take the other's notifications.
		svc := &grpcWriteTestService{replySize: 100, notify: make(chan struct{})}
		addr, _ := startGrpcWriteTest(t, svc, 30*time.Second)

		body, _ := post(addr, "GetRawBlock", contentType)
		reply := &pb.GetRawBlockResponse{}
		flags, payload := readFrame(body)
		if flags != 0 || proto.Unmarshal(payload, reply) != nil ||
			len(reply.Block) != svc.replySize {

			t.Fatalf("%s: message frame with flags %#x, %d bytes",
				contentType, flags, len(payload))
		}
		flags, payload = readFrame(body)
		if flags != 0x80 || !strings.Contains(string(payload), "grpc-status: 0") {
			t.Fatalf("%s: trailer frame with flags %#x: %q", contentType,
				flags, payload)
		}

		// grpcWriteTestService doesn't implement GetBlock, so the call
		// fails before any message, with its status in the headers.
		if _, header := post(addr, "GetBlock", contentType); header.Get("Grpc-Status") != "12" {
			t.Fatalf("%s: GetBlock's grpc-status %q, want 12 (Unimplemented)",
				contentType, header.Get("Grpc-Status"))
		}

		// The first notification arrives before the service sends the
		// second, so the stream is flushed message by message.
		body, _ = post(addr, "SubscribeBlocks", contentType)
		for i := 0; i < 2; i++ {
			flags, payload := readFrame(body)
			notification := &pb.BlockNotification{}
			if flags != 0 || proto.Unmarshal(payload, notification) != nil ||
				len(notification.GetSerializedBlock()) != svc.replySize {

				t.Fatalf("%s: notification %d: frame with flags %#x, "+
					"%d bytes", contentType, i, flags, len(payload))
			}
			if i == 0 {
				svc.notify <- struct{}{}
			}
		}

		// A browser's CORS preflight is answered for any origin, and lets
		// the call send the gRPC-Web headers.
		req, err := http.NewRequest("OPTIONS",
			"https://"+addr+"/pb.bchrpc/GetRawBlock", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type,x-grpc-web")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		allowed := strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers"))
		if resp.Header.Get("Access-Control-Allow-Origin") != origin ||
			!strings.Contains(allowed, "x-grpc-web") {

			t.Fatalf("preflight: status %d, Access-Control-Allow-Origin %q, "+
				"Access-Control-Allow-Headers %q", resp.StatusCode,
				resp.Header.Get("Access-Control-Allow-Origin"), allowed)
		}
	}
}

// grpcWebTextReader decodes a gRPC-Web text reply as it arrives.  The text
// format may send one padded base64 part per flush, so it decodes a
// 4-character group at a time.
type grpcWebTextReader struct {
	r   io.Reader
	buf []byte
}

func (g *grpcWebTextReader) Read(p []byte) (int, error) {
	for len(g.buf) == 0 {
		var group [4]byte
		if _, err := io.ReadFull(g.r, group[:]); err != nil {
			return 0, err
		}
		raw, err := base64.StdEncoding.DecodeString(string(group[:]))
		if err != nil {
			return 0, err
		}
		g.buf = raw
	}
	n := copy(p, g.buf)
	g.buf = g.buf[n:]
	return n, nil
}

// grpcLimitTestService serves a unary method X, which answers with a block of
// replySize bytes once release receives or is closed, or its call ends,
// telling started first, a streaming method Y, which sends one notification
// and waits for its call to end, and a method Z streaming both ways, which
// answers each message it receives.  It is registered under the
// name of the reflection service, which ServiceReady always finds ready, so
// that its calls pass the interceptors as bchrpc's do once the node runs.
type grpcLimitTestService struct {
	replySize int
	started   chan struct{}
	release   chan struct{}
}

// grpcLimitTestStream is Y's method name.
const grpcLimitTestStream = "/grpc.reflection.v1alpha.ServerReflection/Y"

func (s *grpcLimitTestService) register(gs *grpc.Server) {
	x := func(ctx context.Context, _ interface{}) (interface{}, error) {
		if s.started != nil {
			s.started <- struct{}{}
			select {
			case <-s.release:
			case <-ctx.Done():
			}
		}
		return &pb.GetRawBlockResponse{Block: make([]byte, s.replySize)}, nil
	}
	gs.RegisterService(&grpc.ServiceDesc{
		ServiceName: "grpc.reflection.v1alpha.ServerReflection",
		HandlerType: (*interface{})(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "X",
			Handler: func(srv interface{}, ctx context.Context,
				dec func(interface{}) error,
				intercept grpc.UnaryServerInterceptor) (interface{}, error) {

				req := new(pb.GetRawBlockRequest)
				if err := dec(req); err != nil {
					return nil, err
				}
				info := &grpc.UnaryServerInfo{Server: srv,
					FullMethod: readyGrpcMethod}
				return intercept(ctx, req, info, x)
			},
		}},
		Streams: []grpc.StreamDesc{{
			StreamName:    "Y",
			ServerStreams: true,
			Handler: func(_ interface{}, stream grpc.ServerStream) error {
				if err := stream.RecvMsg(new(pb.SubscribeBlocksRequest)); err != nil {
					return err
				}
				if err := stream.SendMsg(&pb.BlockNotification{}); err != nil {
					return err
				}
				<-stream.Context().Done()
				return nil
			},
		}, {
			StreamName:    "Z",
			ServerStreams: true,
			ClientStreams: true,
			Handler: func(_ interface{}, stream grpc.ServerStream) error {
				for {
					err := stream.RecvMsg(new(pb.SubscribeBlocksRequest))
					if err == io.EOF {
						return nil
					}
					if err != nil {
						return err
					}
					if err := stream.SendMsg(&pb.BlockNotification{}); err != nil {
						return err
					}
				}
			},
		}},
	}, s)
}

// dialGrpcTest returns a connection to the test server at addr.
func dialGrpcTest(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cc.Close() })
	return cc
}

// TestGrpcCallLimit ensures that at most the limit of unary calls are in
// progress at once, that up to four times as many more wait for a place
// until their context ends, that calls beyond those are refused with
// ResourceExhausted, that a caller without the auth token is refused before
// it takes a place, and that streaming calls are not counted.
func TestGrpcCallLimit(t *testing.T) {
	setGrpcTestConfig(t, "tok", false)
	svc := &grpcLimitTestService{replySize: 10,
		started: make(chan struct{}, 100), release: make(chan struct{})}
	calls := newGrpcCallLimiter(2)
	addr, _ := startGrpcTest(t, time.Minute, svc.register,
		grpcServerOptions(calls)...)
	cc := dialGrpcTest(t, addr)
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		AuthenticationTokenKey, "tok")
	call := func(ctx context.Context) error {
		return cc.Invoke(ctx, readyGrpcMethod, &pb.GetRawBlockRequest{},
			new(pb.GetRawBlockResponse))
	}

	// Two calls take the places.
	errs := make(chan error, 10)
	for i := 0; i < 2; i++ {
		go func() { errs <- call(ctx) }()
	}
	for i := 0; i < 2; i++ {
		recvGrpc(t, "a call to start", svc.started)
	}

	// A waiting call leaves the queue when its deadline passes, or when its
	// client cancels it.
	dctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err := call(dctx); status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("call past its deadline: got %v, want DeadlineExceeded", err)
	}
	waitForGrpc(t, "the queue to empty", func() bool {
		return calls.waiting.Load() == 0
	})
	cctx, cancel := context.WithCancel(ctx)
	canceled := make(chan error, 1)
	go func() { canceled <- call(cctx) }()
	waitForGrpc(t, "a call to wait", func() bool {
		return calls.waiting.Load() == 1
	})
	cancel()
	if err := recvGrpc(t, "the canceled call", canceled); status.Code(err) != codes.Canceled {
		t.Errorf("canceled call: got %v, want Canceled", err)
	}
	waitForGrpc(t, "the queue to empty", func() bool {
		return calls.waiting.Load() == 0
	})

	// Eight more wait, and none of them runs.
	for i := 0; i < 8; i++ {
		go func() { errs <- call(ctx) }()
	}
	waitForGrpc(t, "eight calls to wait", func() bool {
		return calls.waiting.Load() == 8
	})
	if n := len(svc.started); n != 0 {
		t.Fatalf("%d calls ran past the limit", n)
	}

	// The next is refused at once, one without the token is refused for
	// that, and a streaming call still runs.
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := call(rctx)
	if status.Code(err) != codes.ResourceExhausted ||
		!strings.Contains(err.Error(), "the limit is 2") {

		t.Errorf("call over the queue: got %v, want ResourceExhausted", err)
	}
	if err := call(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "invalid authentication token") {

		t.Errorf("call without the token: got %v, want the auth error", err)
	}
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := cc.NewStream(sctx, &grpc.StreamDesc{ServerStreams: true},
		grpcLimitTestStream)
	if err == nil {
		err = stream.SendMsg(&pb.SubscribeBlocksRequest{})
	}
	if err == nil {
		err = stream.CloseSend()
	}
	if err == nil {
		err = stream.RecvMsg(new(pb.BlockNotification))
	}
	if err != nil {
		t.Errorf("streaming call with the places taken: %v", err)
	}
	cancel()

	// A place freed goes to one waiting call.
	svc.release <- struct{}{}
	recvGrpc(t, "a waiting call to start", svc.started)
	waitForGrpc(t, "a waiting call to start", func() bool {
		return calls.waiting.Load() == 7
	})
	if n := len(svc.started); n != 0 {
		t.Errorf("%d more calls started", n)
	}

	close(svc.release)
	for i := 0; i < 10; i++ {
		if err := recvGrpc(t, "the calls to end", errs); err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
}

// TestGrpcCallLimiterContextEnds ensures that a call whose context ends while
// it waits for a place returns Canceled or DeadlineExceeded, which are logged
// at debug, as the client is gone or has given up.
func TestGrpcCallLimiterContextEnds(t *testing.T) {
	calls := newGrpcCallLimiter(1)
	calls.places <- struct{}{}
	handler := func(context.Context, interface{}) (interface{}, error) {
		t.Error("handler ran without a place")
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := calls.intercept(ctx, nil, nil, handler)
	if status.Code(err) != codes.Canceled {
		t.Errorf("canceled: got %v, want Canceled", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	_, err = calls.intercept(ctx, nil, nil, handler)
	if status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("deadline passed: got %v, want DeadlineExceeded", err)
	}
	if n := calls.waiting.Load(); n != 0 {
		t.Errorf("%d calls left waiting", n)
	}
}

// TestGrpcCallLimitZero ensures that a limit of 0 refuses every unary call,
// as a limit of 0 subscriptions refuses every subscription.
func TestGrpcCallLimitZero(t *testing.T) {
	setGrpcTestConfig(t, "", false)
	svc := &grpcLimitTestService{replySize: 10}
	addr, _ := startGrpcTest(t, time.Minute, svc.register,
		grpcServerOptions(newGrpcCallLimiter(0))...)
	err := dialGrpcTest(t, addr).Invoke(context.Background(),
		readyGrpcMethod, &pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse))
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("got %v, want ResourceExhausted", err)
	}
}

// TestGrpcCallPlaceFreedWhenHandlerReturns ensures that a call frees its place
// once its handler has made the reply, as BCHN's HTTP server frees its worker
// thread and leaves the reply to the connection, so that a client that takes
// a large reply slowly, or not at all, does not keep other calls waiting for
// as long as the reply takes to send.
func TestGrpcCallPlaceFreedWhenHandlerReturns(t *testing.T) {
	setGrpcTestConfig(t, "", false)
	svc := &grpcLimitTestService{replySize: 1 << 20}
	calls := newGrpcCallLimiter(1)
	addr, _ := startGrpcTest(t, time.Minute, svc.register,
		grpcServerOptions(calls)...)

	// The first call's client grants no window for its reply, so the reply
	// waits to be sent until the write deadline, a minute later.
	c := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: 0})
	c.call(1, readyGrpcMethod, &pb.GetRawBlockRequest{})
	for deadline := time.Now().Add(10 * time.Second); ; {
		f := c.next(deadline)
		if f == nil {
			t.Fatal("no reply headers")
		}
		if _, ok := f.(*http2.MetaHeadersFrame); ok {
			break
		}
	}

	// The second runs meanwhile.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := dialGrpcTest(t, addr).Invoke(ctx, readyGrpcMethod,
		&pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse))
	if err != nil {
		t.Fatalf("second call while the first's reply waits: %v", err)
	}
	if f := c.next(time.Now().Add(100 * time.Millisecond)); f != nil {
		t.Errorf("the first call got a %T frame before its client took any "+
			"of its reply", f)
	}
}

// TestGrpcCallLimiterPanicFreesPlace ensures that a call whose handler panics
// frees its place, as interceptUnary recovers the panic and answers the call.
func TestGrpcCallLimiterPanicFreesPlace(t *testing.T) {
	calls := newGrpcCallLimiter(1)
	func() {
		defer func() { _ = recover() }()
		_, _ = calls.intercept(context.Background(), nil, nil,
			func(context.Context, interface{}) (interface{}, error) {
				panic("handler")
			})
	}()
	if n := len(calls.places); n != 0 {
		t.Errorf("%d places held after the handler panicked", n)
	}
}

// TestGrpcRequestNotSentEnds ensures that a call whose client sends the
// request's headers and nothing more, unary or streaming, ends once
// grpcReadTimeout has passed, so that the call's goroutines are not held for
// as long as the client keeps its connection, and that this is logged at
// debug.
func TestGrpcRequestNotSentEnds(t *testing.T) {
	const timeout = 300 * time.Millisecond
	setGrpcTestConfig(t, "", false)
	lines := make(grpcLogLines, 100)
	logger := bchlog.NewBackend(lines).Logger("GRPC")
	logger.SetLevel(bchlog.LevelDebug)
	grpcLog = logger
	svc := &grpcLimitTestService{replySize: 10}
	addr, active := startGrpcTest(t, timeout, svc.register,
		grpcServerOptions(newGrpcCallLimiter(1))...)
	c := dialRawH2(t, addr)
	c.start(1, readyGrpcMethod)
	c.start(3, grpcLimitTestStream)
	start := time.Now()
	waitForGrpc(t, "the calls to start", func() bool {
		return active.Load() == 2
	})
	for deadline := start.Add(5 * time.Second); active.Load() != 0; {
		if time.Now().After(deadline) {
			t.Fatalf("%d calls held after %v", active.Load(),
				time.Since(start))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if took := time.Since(start); took < timeout {
		t.Errorf("the calls ended after %v, before the timeout", took)
	}

	// The streaming call's error is logged at debug, so such clients
	// cannot flood the log, and before the test restores the log.
	for {
		line := recvGrpc(t, "the streaming call's error", lines)
		if strings.Contains(line, "errored") {
			if !strings.Contains(line, "[DBG]") {
				t.Errorf("logged %q", line)
			}
			return
		}
	}
}

// grpcLogLines passes on the lines logged to it.
type grpcLogLines chan string

func (c grpcLogLines) Write(p []byte) (int, error) {
	c <- string(p)
	return len(p), nil
}

// TestGrpcSlowRequestKept ensures that a request whose client sends it slowly
// but steadily is read whole, although it takes several times
// grpcReadTimeout, since the deadline is renewed as the request comes.
func TestGrpcSlowRequestKept(t *testing.T) {
	const timeout = 500 * time.Millisecond
	svc := &grpcWriteTestService{replySize: 10}
	addr, _ := startGrpcWriteTest(t, svc, timeout)
	c := dialRawH2(t, addr)
	c.start(1, "/pb.bchrpc/GetRawBlock")
	msg, err := proto.Marshal(&pb.GetRawBlockRequest{
		HashOrHeight: &pb.GetRawBlockRequest_Hash{Hash: make([]byte, 512<<10)}})
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 5+len(msg))
	binary.BigEndian.PutUint32(body[1:5], uint32(len(msg)))
	copy(body[5:], msg)

	// 16 KiB every 40 ms, so 64 KiB in 160 ms and the whole request in
	// about 1.3 s.
	start := time.Now()
	for len(body) > 0 {
		n := min(len(body), 16<<10)
		if err := c.fr.WriteData(1, n == len(body), body[:n]); err != nil {
			t.Fatal(err)
		}
		body = body[n:]
		time.Sleep(40 * time.Millisecond)
	}
	if took := time.Since(start); took < 2*timeout {
		t.Fatalf("the request took %v, not long enough to test", took)
	}
	for {
		switch f := c.next(time.Now().Add(10 * time.Second)).(type) {
		case nil:
			t.Fatal("no reply")
		case *http2.RSTStreamFrame:
			t.Fatal("stream reset")
		case *http2.MetaHeadersFrame:
			if !f.StreamEnded() {
				continue
			}
			for _, hf := range f.Fields {
				if hf.Name == "grpc-status" && hf.Value != "0" {
					t.Fatalf("grpc-status %s", hf.Value)
				}
			}
			return
		}
	}
}

// TestGrpcClientStreamNotTimed ensures that a call whose client streams can
// send its messages as far apart as it chooses, as a subscription updating
// its filter does.
func TestGrpcClientStreamNotTimed(t *testing.T) {
	const timeout = 300 * time.Millisecond
	setGrpcTestConfig(t, "", false)
	svc := &grpcLimitTestService{}
	addr, _ := startGrpcTest(t, timeout, svc.register,
		grpcServerOptions(newGrpcCallLimiter(1))...)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stream, err := dialGrpcTest(t, addr).NewStream(ctx, &grpc.StreamDesc{
		ServerStreams: true, ClientStreams: true},
		"/grpc.reflection.v1alpha.ServerReflection/Z")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := stream.SendMsg(&pb.SubscribeBlocksRequest{}); err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if err := stream.RecvMsg(new(pb.BlockNotification)); err != nil {
			t.Fatalf("answer %d: %v", i, err)
		}
		time.Sleep(3 * timeout)
	}

	// The call ends once its client is done.
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	if err := stream.RecvMsg(new(pb.BlockNotification)); err != io.EOF {
		t.Errorf("the end of the call: %v", err)
	}
}

// readDeadlineRecorder is a ResponseWriter that records the read deadlines set
// on it.
type readDeadlineRecorder struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (r *readDeadlineRecorder) SetReadDeadline(t time.Time) error {
	r.deadlines = append(r.deadlines, t)
	return nil
}

// TestGrpcReadDeadline ensures a request's read deadline is set
// grpcReadTimeout ahead, renewed once each part has come, and cleared once its
// body ends, after which, or once the request has ended, it is not set again.
func TestGrpcReadDeadline(t *testing.T) {
	saved := grpcReadTimeout
	t.Cleanup(func() { grpcReadTimeout = saved })
	grpcReadTimeout = 7 * time.Minute
	rec := &readDeadlineRecorder{}
	d := &grpcReadDeadline{rc: http.NewResponseController(rec)}
	check := func(what string, n int, last time.Time) {
		t.Helper()
		if len(rec.deadlines) != n {
			t.Fatalf("%s: deadlines %v, want %d", what, rec.deadlines, n)
		}
		if got := rec.deadlines[n-1]; got.IsZero() != last.IsZero() {
			t.Errorf("%s: last deadline %v", what, got)
		}
	}
	d.renew()
	check("the headers", 1, time.Now())
	if ahead := time.Until(rec.deadlines[0]); ahead < 6*time.Minute ||
		ahead > grpcReadTimeout {

		t.Errorf("the deadline is %v ahead, want %v", ahead, grpcReadTimeout)
	}
	d.read(grpcWritePartSize-1, nil)
	check("under a part", 1, time.Now())
	d.read(1, nil)
	check("a part", 2, time.Now())
	d.read(grpcWritePartSize-1, nil)
	check("under the next part", 2, time.Now())
	d.read(0, io.EOF)
	check("the end of the body", 3, time.Time{})
	d.read(grpcWritePartSize, nil)
	d.renew()
	d.clear()
	check("after the end of the body", 3, time.Time{})

	rec.deadlines = nil
	d = &grpcReadDeadline{rc: http.NewResponseController(rec)}
	d.renew()
	d.end()
	d.renew()
	d.clear()
	check("after the request ended", 1, time.Now())
}

// grpcRepliesHeld returns the number of replies the budget counts and the
// bytes they hold.
func grpcRepliesHeld(b *grpcReplyBudget) (int, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.writers), b.used
}

// setGrpcStallTimeout sets grpcStallTimeout for the duration of the test.  It
// is restored after the cleanups of the servers started later, which wait for
// their handlers to return.
func setGrpcStallTimeout(t *testing.T, timeout time.Duration) {
	saved := grpcStallTimeout
	t.Cleanup(func() { grpcStallTimeout = saved })
	grpcStallTimeout = timeout
}

// headers reads the client's frames until the reply headers of the stream.
func (c *rawH2Client) headers(stream uint32) {
	c.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; {
		f := c.next(deadline)
		if f == nil {
			c.t.Fatalf("no reply headers for stream %d", stream)
		}
		if h, ok := f.(*http2.MetaHeadersFrame); ok && h.StreamID == stream {
			return
		}
	}
}

// resets returns the streams the client sees reset until the deadline.
func (c *rawH2Client) resets(deadline time.Time) map[uint32]bool {
	c.t.Helper()
	reset := make(map[uint32]bool)
	for {
		switch f := c.next(deadline).(type) {
		case nil:
			return reset
		case *http2.RSTStreamFrame:
			reset[f.StreamID] = true
		case *http2.DataFrame:
			c.t.Fatalf("data sent on stream %d without a window", f.StreamID)
		}
	}
}

// takeSteadily takes at least size bytes of the stream's reply or
// notification, 16 KiB every 40 ms, so a part in each 160 ms, granting flow
// control for each frame once it has come, and tells done how it went:
// "taken whole" once size bytes have come.
func (c *rawH2Client) takeSteadily(stream uint32, size int, done chan<- string) {
	const window = 16 << 10
	received := 0
	for received < size {
		switch f := c.next(time.Now().Add(10 * time.Second)).(type) {
		case nil:
			done <- fmt.Sprintf("no frame after %d bytes", received)
			return
		case *http2.RSTStreamFrame:
			done <- fmt.Sprintf("reset after %d bytes", received)
			return
		case *http2.DataFrame:
			n := len(f.Data())
			received += n
			if n == 0 {
				continue
			}
			time.Sleep(time.Duration(n) * 40 * time.Millisecond / window)
			_ = c.fr.WriteWindowUpdate(stream, uint32(n))
			_ = c.fr.WriteWindowUpdate(0, uint32(n))
		case *http2.MetaHeadersFrame:
			if f.StreamEnded() {
				done <- fmt.Sprintf("ended after %d bytes", received)
				return
			}
		}
	}
	done <- "taken whole"
}

// TestGrpcStalledRepliesMakeRoom ensures that once the replies being written
// hold more than the budget, a reply that needs room resets the writes whose
// clients have taken none of a part for grpcStallTimeout, the longest stalled
// first, until the others fit, and is sent at once, so that clients that take
// none of their replies can neither make the node hold more nor keep other
// calls waiting, and that the room is given back as the writes end.  A small
// reply waits in its flush, which counts a part.
func TestGrpcStalledRepliesMakeRoom(t *testing.T) {
	setGrpcStallTimeout(t, 100*time.Millisecond)
	for _, test := range []struct {
		size int
		room int64
	}{
		{1 << 20, 3<<20 + 1<<19},
		{100, 3*grpcWritePartSize + grpcWritePartSize/2},
	} {
		svc := &grpcWriteTestService{replySize: test.size}
		replies := newGrpcReplyBudget(test.room)
		addr, _ := startGrpcRepliesTest(t, time.Minute, replies,
			func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

		// A client takes none of three replies, called one after the
		// other, and then none for grpcStallTimeout more.
		c := dialRawH2(t, addr, http2.Setting{
			ID: http2.SettingInitialWindowSize, Val: 0})
		for _, stream := range []uint32{1, 3, 5} {
			c.call(stream, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
			c.headers(stream)
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(grpcStallTimeout)

		// Another client's call gets its reply, and the first of the three
		// is reset to make room for it.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := dialGrpcTest(t, addr).Invoke(ctx, "/pb.bchrpc/GetRawBlock",
			&pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse))
		cancel()
		if err != nil {
			t.Fatalf("%d bytes: a call with the room taken: %v", test.size, err)
		}
		reset := c.resets(time.Now().Add(500 * time.Millisecond))
		if !reset[1] || reset[3] || reset[5] {
			t.Errorf("%d bytes: streams reset %v, want only 1", test.size, reset)
		}
		if n, used := grpcRepliesHeld(replies); n != 2 || used > test.room {
			t.Errorf("%d bytes: %d replies hold %d bytes, want 2 within %d",
				test.size, n, used, test.room)
		}

		// The room is given back once the client goes.
		c.conn.Close()
		waitForGrpc(t, "the room to be given back", func() bool {
			n, used := grpcRepliesHeld(replies)
			return n == 0 && used == 0
		})
	}
}

// TestGrpcSteadyReplyKeptForRoom ensures that a reply whose client takes it
// steadily is not reset to make room while one whose client has taken none of
// it for grpcStallTimeout is written, though the first started before.
func TestGrpcSteadyReplyKeptForRoom(t *testing.T) {
	setGrpcStallTimeout(t, 300*time.Millisecond)
	const size = 1 << 20
	svc := &grpcWriteTestService{replySize: size}
	replies := newGrpcReplyBudget(2*size + size/2)
	addr, _ := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// The steady client takes the whole reply in about 2.6 s.
	steady := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: 16 << 10})
	steady.call(1, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
	done := make(chan string, 1)
	go steady.takeSteadily(1, size, done)

	// Then a client takes none of its reply, for longer than
	// grpcStallTimeout.
	time.Sleep(200 * time.Millisecond)
	stalled := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: 0})
	stalled.call(1, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
	stalled.headers(1)
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := dialGrpcTest(t, addr).Invoke(ctx, "/pb.bchrpc/GetRawBlock",
		&pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse))
	if err != nil {
		t.Fatalf("a call with the room taken: %v", err)
	}
	if reset := stalled.resets(time.Now().Add(500 * time.Millisecond)); !reset[1] {
		t.Error("the stalled reply was not reset")
	}
	if got := recvGrpc(t, "the steady reply", done); got != "taken whole" {
		t.Errorf("the steady reply: %s", got)
	}
}

// TestGrpcReadersKeptOverBudget ensures that replies whose clients are taking
// them are never reset to make room, however much more than the budget they
// hold together, the replies of calls and the notifications of subscriptions
// alike, so that clients reading normally get what they got before the
// budget.
func TestGrpcReadersKeptOverBudget(t *testing.T) {
	setGrpcStallTimeout(t, time.Second)
	const size = 1 << 20
	svc := &grpcWriteTestService{replySize: size}
	replies := newGrpcReplyBudget(size + size/2)
	addr, _ := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// Three clients take their replies steadily, starting 200 ms apart, so
	// that the three are written at once, twice the budget.
	done := make(chan string, 3)
	for _, method := range []string{"/pb.bchrpc/GetRawBlock",
		"/pb.bchrpc/SubscribeBlocks", "/pb.bchrpc/GetRawBlock"} {

		c := dialRawH2(t, addr, http2.Setting{
			ID: http2.SettingInitialWindowSize, Val: 16 << 10})
		var req proto.Message = &pb.GetRawBlockRequest{}
		if method == "/pb.bchrpc/SubscribeBlocks" {
			req = &pb.SubscribeBlocksRequest{}
		}
		c.call(1, method, req)
		go c.takeSteadily(1, size, done)
		time.Sleep(200 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		if got := recvGrpc(t, "a reply", done); got != "taken whole" {
			t.Errorf("a reply: %s", got)
		}
	}
}

// TestGrpcStalledSocketReplyFreed ensures that a reply reset to make room on a
// connection whose client does not read its socket, where the reset waits
// behind the frame being written, stays counted until its write returns, and
// that the connection is closed once the reset has not ended the write within
// grpcStallTimeout, which ends the call and frees the reply.
func TestGrpcStalledSocketReplyFreed(t *testing.T) {
	setGrpcStallTimeout(t, 300*time.Millisecond)
	const size = 1 << 20
	svc := &grpcWriteTestService{replySize: size}
	replies := newGrpcReplyBudget(size + size/2)
	addr, active := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// Flow control lets the whole reply out at once, so only the socket
	// holds it back.  From the reply headers on, the client reads nothing.
	c := dialRawH2(t, addr, http2.Setting{
		ID: http2.SettingInitialWindowSize, Val: 1<<31 - 1})
	if err := c.fr.WriteWindowUpdate(0, 1<<31-1-65535); err != nil {
		t.Fatal(err)
	}
	c.call(1, "/pb.bchrpc/GetRawBlock", &pb.GetRawBlockRequest{})
	c.headers(1)
	time.Sleep(2 * grpcStallTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := dialGrpcTest(t, addr).Invoke(ctx, "/pb.bchrpc/GetRawBlock",
		&pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse))
	if err != nil {
		t.Fatalf("a call with the room taken: %v", err)
	}
	if n, used := grpcRepliesHeld(replies); n != 1 || used < size {
		t.Errorf("after the reset: %d replies hold %d bytes, want the "+
			"stalled one, still held", n, used)
	}
	for deadline := time.Now().Add(5 * time.Second); active.Load() != 0; {
		if time.Now().After(deadline) {
			t.Fatal("the stalled call still runs 5 s after its reset")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n, used := grpcRepliesHeld(replies); n != 0 || used != 0 {
		t.Errorf("after the call ended: %d replies hold %d bytes", n, used)
	}
}

// TestGrpcReplyLargerThanRoomSent ensures that a reply larger than the room
// of the budget is still sent whole when nothing else is written.
func TestGrpcReplyLargerThanRoomSent(t *testing.T) {
	svc := &grpcWriteTestService{replySize: 1 << 20}
	replies := newGrpcReplyBudget(256 << 10)
	addr, _ := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp := new(pb.GetRawBlockResponse)
	err := dialGrpcTest(t, addr).Invoke(ctx, "/pb.bchrpc/GetRawBlock",
		&pb.GetRawBlockRequest{}, resp)
	if err != nil || len(resp.Block) != 1<<20 {
		t.Errorf("got %d bytes, %v", len(resp.Block), err)
	}
}

// TestGrpcUnflushedReplyReleased ensures that a reply the request ends without
// flushing, as grpc-go's http.Error answer to a request it refuses before any
// call, is given back once the request ends, so that it can neither fill the
// budget for good nor, once stalled, have a later reset close whatever its
// connection then carries.
func TestGrpcUnflushedReplyReleased(t *testing.T) {
	svc := &grpcWriteTestService{replySize: 10}
	replies := newGrpcReplyBudget(grpcMaxReplyBytes)
	addr, active := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// grpc-go refuses a GET over HTTP/1.1 with an error body written by
	// http.Error, which nothing flushes.
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	t.Cleanup(client.CloseIdleConnections)
	resp, err := client.Get("https://" + addr + "/pb.bchrpc/GetRawBlock")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode == http.StatusOK || len(body) == 0 {
		t.Fatalf("the GET got %d, %q, %v; want an error body",
			resp.StatusCode, body, err)
	}
	waitForGrpc(t, "the request to end", func() bool {
		return active.Load() == 0
	})
	if n, used := grpcRepliesHeld(replies); n != 0 || used != 0 {
		t.Errorf("after the request ended, %d replies hold %d bytes", n, used)
	}
}

// grpcWebCall sends a gRPC-Web request for GetRawBlock over HTTP/1.1 with the
// content type and body given, and returns the connection once the reply
// headers have come, the client reading nothing more from it.
func grpcWebCall(t *testing.T, addr, contentType string, body []byte) net.Conn {
	t.Helper()
	tcp, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = tcp.(*net.TCPConn).SetReadBuffer(16 << 10)
	conn := tls.Client(tcp, &tls.Config{InsecureSkipVerify: true,
		NextProtos: []string{"http/1.1"}})
	t.Cleanup(func() { conn.Close() })
	req, err := http.NewRequest("POST", "https://localhost/pb.bchrpc/GetRawBlock",
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Grpc-Web", "1")
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Content-Type"); got != contentType {
		t.Fatalf("status %d, content type %q", resp.StatusCode, got)
	}
	return conn
}

// TestGrpcWebStalledReplyMakesRoom ensures that a gRPC-Web reply over HTTP/1.1
// whose client stops reading is reset to make room as a gRPC reply is.
func TestGrpcWebStalledReplyMakesRoom(t *testing.T) {
	setGrpcStallTimeout(t, 100*time.Millisecond)
	const size = 32 << 20
	svc := &grpcWriteTestService{replySize: size}
	replies := newGrpcReplyBudget(size + size/2)
	addr, active := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// An empty GetRawBlockRequest.  From the reply headers on, the client
	// reads nothing, and a gRPC call's reply needs the room.
	grpcWebCall(t, addr, "application/grpc-web+proto", []byte{0, 0, 0, 0, 0})
	time.Sleep(200 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := dialGrpcTest(t, addr).Invoke(ctx, "/pb.bchrpc/GetRawBlock",
		&pb.GetRawBlockRequest{}, new(pb.GetRawBlockResponse),
		grpc.MaxCallRecvMsgSize(2*size))
	if err != nil {
		t.Fatalf("a call with the room taken: %v", err)
	}
	waitForGrpc(t, "the gRPC-Web call to end", func() bool {
		return active.Load() == 0
	})
}

// TestGrpcWebTextReplyCounted ensures that a gRPC-Web reply in the text
// format, which the wrapper writes base64 encoded 1 KiB at a time, counts as
// its whole message while its client takes none of it, as gRPC holds the
// message until its last part is written.
func TestGrpcWebTextReplyCounted(t *testing.T) {
	const size = 8 << 20
	svc := &grpcWriteTestService{replySize: size}
	replies := newGrpcReplyBudget(grpcMaxReplyBytes)
	addr, _ := startGrpcRepliesTest(t, time.Minute, replies,
		func(gs *grpc.Server) { pb.RegisterBchrpcServer(gs, svc) })

	// A GetRawBlockRequest for height 1, base64 encoded.
	grpcWebCall(t, addr, "application/grpc-web-text", []byte("AAAAAAIQAQ=="))
	for deadline := time.Now().Add(5 * time.Second); ; {
		n, used := grpcRepliesHeld(replies)
		if n == 1 && used >= size {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d replies hold %d bytes, want the reply's %d", n,
				used, size)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// deadlineRecorder is a ResponseWriter that records the write deadlines set
// on it and takes what is written to it.
type deadlineRecorder struct {
	http.ResponseWriter
	deadlines []time.Time
	written   int
}

func (r *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	r.deadlines = append(r.deadlines, t)
	return nil
}

func (r *deadlineRecorder) Write(p []byte) (int, error) {
	r.written += len(p)
	return len(p), nil
}

// TestGrpcDeadlineWriterParts ensures that a reply's write deadline is set at
// its first write, renewed each time a part more has been written however the
// reply is split into writes, as the text format splits it into writes of
// 1 KiB, and cleared by its flush, so that a client must take a part, not a
// write, in each grpcWriteTimeout.
func TestGrpcDeadlineWriterParts(t *testing.T) {
	rec := &deadlineRecorder{}
	w := &grpcDeadlineWriter{ResponseWriter: rec,
		rc:      http.NewResponseController(rec),
		replies: newGrpcReplyBudget(grpcMaxReplyBytes)}
	piece := make([]byte, 1<<10)
	for i := 0; i < 200; i++ {
		if _, err := w.Write(piece); err != nil {
			t.Fatal(err)
		}
	}
	if len(rec.deadlines) != 4 {
		t.Errorf("200 writes of 1 KiB set %d deadlines, want the first and "+
			"3 renewals", len(rec.deadlines))
	}
	w.Flush()
	if n := len(rec.deadlines); n != 5 || !rec.deadlines[n-1].IsZero() {
		t.Fatalf("%d deadlines after the flush, the last %v, want 5, the "+
			"last cleared", n, rec.deadlines[n-1])
	}
	for i, d := range rec.deadlines[:4] {
		if d.IsZero() {
			t.Errorf("deadline %d cleared", i)
		}
	}
	if rec.written != 200<<10 {
		t.Errorf("%d bytes written, want %d", rec.written, 200<<10)
	}
}

// TestGrpcReplyCountedByPrefix ensures a reply counts the length its message
// prefix gives, from its first write, also when the prefix comes in pieces or
// base64 encoded as in the text format, and that a reply without a valid
// prefix counts what has been written, at least a part, until its flush.
func TestGrpcReplyCountedByPrefix(t *testing.T) {
	prefix := func(flag byte, length uint32) []byte {
		p := []byte{flag, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(p[1:], length)
		return p
	}
	text := func(p []byte) []byte {
		return []byte(base64.StdEncoding.EncodeToString(p))
	}
	message := prefix(0, 3<<20)
	encoded := text(append(message, 7))
	for _, test := range []struct {
		name   string
		text   bool
		writes [][]byte
		want   int64
	}{
		{"binary", false, [][]byte{message}, 5 + 3<<20},
		{"binary in pieces", false, [][]byte{message[:2], message[2:]},
			5 + 3<<20},
		{"compressed", false, [][]byte{prefix(1, 1<<20)}, 5 + 1<<20},
		{"text", true, [][]byte{encoded[:4], encoded[4:]}, 5 + 3<<20},
		{"text trailers", true, [][]byte{text(prefix(0x80, 1<<20))},
			5 + 1<<20},
		{"small", false, [][]byte{prefix(0, 10)}, grpcWritePartSize},
		{"no prefix", false, [][]byte{bytes.Repeat([]byte("x"), 200<<10)},
			192 << 10},
		{"not a message", false, [][]byte{[]byte("invalid gRPC request")},
			grpcWritePartSize},
		{"not base64", true, [][]byte{[]byte("invalid gRPC request")},
			grpcWritePartSize},
	} {
		b := newGrpcReplyBudget(grpcMaxReplyBytes)
		rec := &deadlineRecorder{}
		w := &grpcDeadlineWriter{ResponseWriter: rec,
			rc: http.NewResponseController(rec), replies: b, text: test.text}
		for _, p := range test.writes {
			if _, err := w.Write(p); err != nil {
				t.Fatalf("%s: %v", test.name, err)
			}
		}
		if n, used := grpcRepliesHeld(b); n != 1 || used != test.want {
			t.Errorf("%s: %d replies hold %d bytes, want 1 and %d",
				test.name, n, used, test.want)
		}
		w.Flush()
		if n, used := grpcRepliesHeld(b); n != 0 || used != 0 {
			t.Errorf("%s: after the flush %d replies hold %d bytes",
				test.name, n, used)
		}
	}
}

// closeRecorder is a connection that counts the times it is closed.
type closeRecorder struct {
	net.Conn
	closes atomic.Int32
}

func (c *closeRecorder) Close() error {
	c.closes.Add(1)
	return nil
}

// TestGrpcReplyBudget ensures the budget counts each reply as at least a
// part, and while the replies hold more than the budget resets the writes of
// others whose clients have taken none of a part for grpcStallTimeout, the
// longest stalled first, until the others fit, never one whose client is
// taking its reply nor the writer itself, so that a write goes ahead over the
// budget when no other has stalled.  A reset reply stays counted, without
// making room twice, until its write returns, the writes of a writer it reset
// are refused, and the connection of one whose write has not returned within
// grpcStallTimeout is closed.
func TestGrpcReplyBudget(t *testing.T) {
	setGrpcStallTimeout(t, 100*time.Millisecond)
	const k = 1 << 10
	b := newGrpcReplyBudget(350 * k)
	writer := func() (*grpcDeadlineWriter, *deadlineRecorder, *closeRecorder) {
		rec := &deadlineRecorder{}
		conn := &closeRecorder{}
		return &grpcDeadlineWriter{ResponseWriter: rec,
			rc: http.NewResponseController(rec), replies: b,
			conn: conn}, rec, conn
	}
	w1, rec1, conn1 := writer()
	w2, rec2, conn2 := writer()
	w3, rec3, conn3 := writer()
	check := func(what string, replies int, used int64, reset ...bool) {
		t.Helper()
		if n, u := grpcRepliesHeld(b); n != replies || u != used {
			t.Errorf("%s: %d replies hold %d bytes, want %d and %d", what,
				n, u, replies, used)
		}
		for i, w := range []*grpcDeadlineWriter{w1, w2, w3} {
			if w.reset.Load() != reset[i] {
				t.Errorf("%s: writer %d reset %v, want %v", what, i+1,
					w.reset.Load(), reset[i])
			}
		}
	}

	// A reply over the budget goes ahead while the others are young.
	b.count(w1, 100*k, false)
	time.Sleep(20 * time.Millisecond)
	b.count(w2, 100*k, false)
	b.count(w3, 200*k, false)
	check("over the budget", 3, 400*k, false, false, false)

	// Once both have stalled, the longest stalled makes room, and stays
	// counted until its write returns, but the room it makes is not made
	// again.
	time.Sleep(120 * time.Millisecond)
	b.count(w3, 0, true)
	check("w1 reset", 3, 400*k, true, false, false)
	b.count(w3, 240*k, false)
	check("within the room made", 3, 440*k, true, false, false)
	b.count(w3, 300*k, false)
	check("w2 reset", 3, 500*k, true, true, false)
	for i, rec := range []*deadlineRecorder{rec1, rec2} {
		if len(rec.deadlines) != 1 || !rec.deadlines[0].Equal(grpcResetDeadline) {
			t.Errorf("writer %d: deadlines %v, want the reset's", i+1,
				rec.deadlines)
		}
	}
	if len(rec3.deadlines) != 0 {
		t.Errorf("the writer making room had deadlines %v", rec3.deadlines)
	}

	// With no other reply stalled, w3's goes ahead over the budget, and a
	// reset writer's replies are refused.
	b.count(w3, 400*k, false)
	check("no reply stalled", 3, 600*k, true, true, false)
	if b.count(w1, k, false) {
		t.Error("a reset writer's reply was counted")
	}

	// w1's write returns in time and gives back its room, and w2's
	// connection is closed once its write has not returned within
	// grpcStallTimeout.
	b.release(w1)
	check("w1 released", 2, 500*k, true, true, false)
	waitForGrpc(t, "w2's connection to be closed", func() bool {
		return conn2.closes.Load() == 1
	})
	b.release(w2)
	check("w2 released", 1, 400*k, true, true, false)
	time.Sleep(120 * time.Millisecond)
	if conn1.closes.Load() != 0 || conn2.closes.Load() != 1 ||
		conn3.closes.Load() != 0 {

		t.Errorf("connections closed %d, %d and %d times, want only w2's "+
			"once", conn1.closes.Load(), conn2.closes.Load(),
			conn3.closes.Load())
	}

	// A reply larger than the budget never resets its own writer, and a
	// reply counts at least a part.
	b.count(w3, 500*k, false)
	b.release(w3)
	check("all released", 0, 0, true, true, false)
	w4, _, _ := writer()
	b.count(w4, 10, false)
	if n, used := grpcRepliesHeld(b); n != 1 || used != grpcWritePartSize {
		t.Errorf("a small reply: %d replies hold %d bytes", n, used)
	}
	b.release(w4)
	if n, used := grpcRepliesHeld(b); n != 0 || used != 0 {
		t.Errorf("the end: %d replies hold %d bytes", n, used)
	}
}

// TestGrpcReplyReleased ensures a reply gives back its room once its writer
// has been reset, at its next write rather than its flush, that a reply the
// request ends without flushing gives it back too, that a writer's next reply
// is counted and has its deadline set again after a flush, and that the timer
// of a reset reply closes nothing once the reply has been given back.
func TestGrpcReplyReleased(t *testing.T) {
	setGrpcStallTimeout(t, 50*time.Millisecond)
	b := newGrpcReplyBudget(grpcWritePartSize)
	writer := func() (*grpcDeadlineWriter, *deadlineRecorder, *closeRecorder) {
		rec := &deadlineRecorder{}
		conn := &closeRecorder{}
		return &grpcDeadlineWriter{ResponseWriter: rec,
			rc: http.NewResponseController(rec), replies: b,
			conn: conn}, rec, conn
	}
	held := func(what string, want int) {
		t.Helper()
		if n, _ := grpcRepliesHeld(b); n != want {
			t.Errorf("%s: %d replies held, want %d", what, n, want)
		}
	}
	prefix := []byte{0, 0, 0, 0, 10}

	// A writer's second reply is counted, and its deadline set, as its
	// first was.
	w1, rec1, conn1 := writer()
	for i := 0; i < 2; i++ {
		if _, err := w1.Write(prefix); err != nil {
			t.Fatal(err)
		}
		held("a reply being written", 1)
		w1.Flush()
		held("a reply flushed", 0)
	}
	if n := len(rec1.deadlines); n != 4 {
		t.Errorf("two replies set %d deadlines, want each set and cleared", n)
	}

	// w1's third reply stalls, and w2's makes room by resetting it: w1's
	// next write gives the room back at once.
	if _, err := w1.Write(prefix); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	w2, _, _ := writer()
	if _, err := w2.Write(prefix); err != nil {
		t.Fatal(err)
	}
	if !w1.reset.Load() {
		t.Fatal("the stalled reply was not reset")
	}
	held("after the reset", 2)
	if _, err := w1.Write(make([]byte, 10)); err != errGrpcReplyReset {
		t.Errorf("a reset writer's write: %v", err)
	}
	held("after the reset writer's write", 1)

	// The timer of w1's reset closes nothing, as w1's reply was given back,
	// and the end of w2's request gives back its reply, never flushed.
	b.closeStalled(w1, 0)
	time.Sleep(60 * time.Millisecond)
	if n := conn1.closes.Load(); n != 0 {
		t.Errorf("the reset reply's connection was closed %d times", n)
	}
	w2.close()
	held("after the request ended", 0)
}

// TestGrpcDeadlineWriterReset ensures that once a writer has been reset its
// writes do not renew or clear its write deadline, which the reset set in the
// past, and are refused, and that a reset after the request has ended sets
// no deadline.
func TestGrpcDeadlineWriterReset(t *testing.T) {
	rec := &deadlineRecorder{}
	w := &grpcDeadlineWriter{ResponseWriter: rec,
		rc: http.NewResponseController(rec), replies: newGrpcReplyBudget(1)}
	if !w.renewDeadline() {
		t.Fatal("deadline not renewed")
	}
	w.clearDeadline()
	if len(rec.deadlines) != 2 || rec.deadlines[0].IsZero() ||
		!rec.deadlines[1].IsZero() {

		t.Fatalf("deadlines %v, want one renewed and cleared", rec.deadlines)
	}

	w.reset.Store(true)
	w.resetStream()
	if w.renewDeadline() {
		t.Error("a reset writer renewed its deadline")
	}
	w.clearDeadline()
	if _, err := w.Write([]byte{1}); err != errGrpcReplyReset {
		t.Errorf("a reset writer's write: %v", err)
	}
	w.Flush()
	if len(rec.deadlines) != 3 || !rec.deadlines[2].Equal(grpcResetDeadline) {
		t.Errorf("deadlines %v, want the reset's last", rec.deadlines)
	}
	if rec.written != 0 {
		t.Errorf("a reset writer wrote %d bytes", rec.written)
	}

	rec.deadlines = nil
	w.close()
	w.resetStream()
	if len(rec.deadlines) != 0 {
		t.Errorf("a reset after the request ended set %v", rec.deadlines)
	}
}
