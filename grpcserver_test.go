// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
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
// and handler, with grpcWriteTimeout set to timeout, and returns the server's
// address and the number of requests its handler is serving.
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
	saved := grpcWriteTimeout
	t.Cleanup(func() { grpcWriteTimeout = saved })
	grpcWriteTimeout = timeout

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
	handler := grpcHTTPHandler(gs)
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
		// No handler may read grpcWriteTimeout once it is restored.
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

// grpcLimitTestService serves a unary method X, which answers with a block of
// replySize bytes once release receives or is closed, or its call ends,
// telling started first, and a streaming method Y, which sends one
// notification and waits for its call to end.  It is registered under the
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
