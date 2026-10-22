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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gcash/bchd/bchrpc/pb"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// grpcWebWaitDone receives the error the Wait method returns with.
var grpcWebWaitDone = make(chan error, 1)

// grpcWebTestDesc describes a unary Echo method, which fails for the value
// "fail" and returns the x-data-bin request metadata as a header, a
// server-streaming Count method, which sends one message per count and fails
// after the first message for counts over 10, and a Wait method, which sends
// one message and waits for the call to end.  It is registered under a ready
// service, so that the calls go through the real interceptors.
var grpcWebTestDesc = grpc.ServiceDesc{
	ServiceName: readyGrpcService,
	HandlerType: (*interface{})(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Echo",
		Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, intercept grpc.UnaryServerInterceptor) (interface{}, error) {
			in := new(wrapperspb.StringValue)
			if err := dec(in); err != nil {
				return nil, err
			}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				v := req.(*wrapperspb.StringValue).GetValue()
				if v == "fail" {
					return nil, status.Error(codes.InvalidArgument, "bad value")
				}
				md, _ := metadata.FromIncomingContext(ctx)
				header := metadata.Pairs("x-test-header", "h")
				header["x-data-bin"] = md["x-data-bin"]
				if err := grpc.SetHeader(ctx, header); err != nil {
					return nil, err
				}
				if err := grpc.SetTrailer(ctx, metadata.Pairs("x-test-trailer", "t")); err != nil {
					return nil, err
				}
				return wrapperspb.String("echo " + v), nil
			}
			info := &grpc.UnaryServerInfo{
				Server:     srv,
				FullMethod: "/" + readyGrpcService + "/Echo",
			}
			return intercept(ctx, in, info, handler)
		},
	}},
	Streams: []grpc.StreamDesc{{
		StreamName:    "Count",
		ServerStreams: true,
		Handler: func(srv interface{}, stream grpc.ServerStream) error {
			in := new(wrapperspb.UInt32Value)
			if err := stream.RecvMsg(in); err != nil {
				return err
			}
			for i := uint32(0); i < in.GetValue(); i++ {
				if err := stream.SendMsg(wrapperspb.String(fmt.Sprint(i))); err != nil {
					return err
				}
				if in.GetValue() > 10 {
					return status.Error(codes.OutOfRange, "too many")
				}
			}
			return nil
		},
	}, {
		StreamName:    "Wait",
		ServerStreams: true,
		Handler: func(srv interface{}, stream grpc.ServerStream) error {
			if err := stream.SendMsg(wrapperspb.String("waiting")); err != nil {
				return err
			}
			<-stream.Context().Done()
			// An error would be logged after the test has moved on.
			grpcWebWaitDone <- stream.Context().Err()
			return nil
		},
	}},
}

// newGrpcWebTestServer returns HTTP/1.1 and HTTP/2 test servers serving the
// test service through the gRPC listener's handler and the node's
// interceptors and call limit.
func newGrpcWebTestServer(t *testing.T) (h1, h2 *httptest.Server) {
	t.Helper()
	setGrpcTestConfig(t, "token", false)

	server := grpc.NewServer(grpcServerOptions(newGrpcCallLimiter(16))...)
	server.RegisterService(&grpcWebTestDesc, struct{}{})
	handler := grpcHTTPHandler(server)

	h1 = httptest.NewServer(handler)
	t.Cleanup(h1.Close)
	h2 = httptest.NewUnstartedServer(handler)
	h2.EnableHTTP2 = true
	h2.StartTLS()
	t.Cleanup(h2.Close)
	return h1, h2
}

// grpcWebFrame returns msg as a data frame.
func grpcWebFrame(t *testing.T, msg proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, 5, 5+len(b))
	binary.BigEndian.PutUint32(frame[1:], uint32(len(b)))
	return append(frame, b...)
}

type grpcWebTestFrame struct {
	flag    byte
	payload []byte
}

// grpcWebCall posts body to method and returns the response with its body
// split into frames.  In text mode the body is base64 encoded in chunks of
// two bytes, and the response is decoded.
func grpcWebCall(t *testing.T, srv *httptest.Server, method, contentType, token string, body []byte) (*http.Response, []grpcWebTestFrame) {
	t.Helper()
	text := strings.HasPrefix(contentType, grpcWebTextContentType)
	if text {
		var enc []byte
		for len(body) > 0 {
			n := min(2, len(body))
			enc = base64.StdEncoding.AppendEncode(enc, body[:n])
			body = body[n:]
		}
		body = enc
	}

	req, err := http.NewRequest(http.MethodPost,
		srv.URL+"/"+readyGrpcService+"/"+method, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set(AuthenticationTokenKey, token)
	req.Header.Set("X-Data-Bin", base64.StdEncoding.EncodeToString([]byte{0, 1, 0xff}))
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, splitGrpcWebFrames(t, respBody, text)
}

// splitGrpcWebFrames splits a response body into frames, decoding it first in
// text mode.
func splitGrpcWebFrames(t *testing.T, body []byte, text bool) []grpcWebTestFrame {
	t.Helper()
	if text {
		if len(body)%4 != 0 {
			t.Fatalf("text response length %d", len(body))
		}
		decoded := make([]byte, len(body))
		n, err := decodeBase64Chunks(decoded, body)
		if err != nil {
			t.Fatalf("decode text response %.100q: %v", body, err)
		}
		body = decoded[:n]
	}

	var frames []grpcWebTestFrame
	for len(body) > 0 {
		if len(body) < 5 {
			t.Fatalf("short frame header %x", body)
		}
		n := binary.BigEndian.Uint32(body[1:5])
		if uint32(len(body)-5) < n {
			t.Fatalf("short frame of %d bytes, %d left", n, len(body)-5)
		}
		frames = append(frames, grpcWebTestFrame{body[0], body[5 : 5+n]})
		body = body[5+n:]
	}
	return frames
}

// checkGrpcWebHeaders checks the CORS and content type headers of a
// gRPC-Web response.
func checkGrpcWebHeaders(t *testing.T, resp *http.Response, contentType string, exposed ...string) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != contentType {
		t.Errorf("content type %q, want %q", got, contentType)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allowed origin %q", got)
	}
	if _, ok := resp.Header["Access-Control-Allow-Credentials"]; ok {
		t.Error("response allows credentials")
	}
	expose := resp.Header.Get("Access-Control-Expose-Headers")
	for _, h := range append(exposed, "Grpc-Status", "Grpc-Message") {
		if !strings.Contains(expose, h) {
			t.Errorf("exposed headers %q lack %s", expose, h)
		}
	}
}

var grpcWebTestTypes = []struct {
	request, response string
}{
	{"application/grpc-web", "application/grpc-web"},
	{"application/grpc-web+proto", "application/grpc-web+proto"},
	{"application/grpc-web-text", "application/grpc-web-text"},
	{"application/grpc-web-text+proto", "application/grpc-web-text+proto"},
}

// TestGrpcWebUnary ensures a unary call returns its headers, one data frame
// and a trailer frame in every content type, over HTTP/1.1 and HTTP/2.
func TestGrpcWebUnary(t *testing.T) {
	h1, h2 := newGrpcWebTestServer(t)
	for _, srv := range []*httptest.Server{h1, h2} {
		for _, ct := range grpcWebTestTypes {
			body := grpcWebFrame(t, wrapperspb.String("hi"))
			resp, frames := grpcWebCall(t, srv, "Echo", ct.request, "token", body)
			checkGrpcWebHeaders(t, resp, ct.response, "X-Test-Header", "X-Data-Bin")
			if resp.ProtoMajor != 1 && srv == h1 || resp.ProtoMajor != 2 && srv == h2 {
				t.Errorf("response protocol %s", resp.Proto)
			}
			if got := resp.Header.Get("X-Test-Header"); got != "h" {
				t.Errorf("%s: header %q", ct.request, got)
			}
			if got := resp.Header.Get("X-Data-Bin"); got != "AAH/" {
				t.Errorf("%s: binary header %q", ct.request, got)
			}

			want := []grpcWebTestFrame{
				{0, grpcWebFrame(t, wrapperspb.String("echo hi"))[5:]},
				{0x80, []byte("grpc-status: 0\r\nx-test-trailer: t\r\n")},
			}
			if fmt.Sprint(frames) != fmt.Sprint(want) {
				t.Errorf("%s: frames %q, want %q", ct.request, frames, want)
			}
		}
	}
}

// TestGrpcWebStream ensures a server-streaming call returns a data frame per
// message and then a trailer frame, including when it ends with an error.
func TestGrpcWebStream(t *testing.T) {
	h1, h2 := newGrpcWebTestServer(t)
	for _, srv := range []*httptest.Server{h1, h2} {
		for _, ct := range grpcWebTestTypes {
			body := grpcWebFrame(t, wrapperspb.UInt32(3))
			resp, frames := grpcWebCall(t, srv, "Count", ct.request, "token", body)
			checkGrpcWebHeaders(t, resp, ct.response)
			var want []grpcWebTestFrame
			for _, s := range []string{"0", "1", "2"} {
				want = append(want, grpcWebTestFrame{0, grpcWebFrame(t, wrapperspb.String(s))[5:]})
			}
			want = append(want, grpcWebTestFrame{0x80, []byte("grpc-status: 0\r\n")})
			if fmt.Sprint(frames) != fmt.Sprint(want) {
				t.Errorf("%s: frames %q, want %q", ct.request, frames, want)
			}

			body = grpcWebFrame(t, wrapperspb.UInt32(11))
			resp, frames = grpcWebCall(t, srv, "Count", ct.request, "token", body)
			checkGrpcWebHeaders(t, resp, ct.response)
			want = []grpcWebTestFrame{
				{0, grpcWebFrame(t, wrapperspb.String("0"))[5:]},
				{0x80, []byte("grpc-message: too many\r\ngrpc-status: 11\r\n")},
			}
			if fmt.Sprint(frames) != fmt.Sprint(want) {
				t.Errorf("%s: frames %q, want %q", ct.request, frames, want)
			}
		}
	}
}

// TestGrpcWebCancel ensures a client that goes away ends its server stream.
func TestGrpcWebCancel(t *testing.T) {
	h1, h2 := newGrpcWebTestServer(t)
	for _, srv := range []*httptest.Server{h1, h2} {
		for _, ct := range []string{grpcWebContentType, grpcWebTextContentType} {
			body := grpcWebFrame(t, wrapperspb.String(""))
			if ct == grpcWebTextContentType {
				body = []byte(base64.StdEncoding.EncodeToString(body))
			}
			ctx, cancel := context.WithCancel(context.Background())
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				srv.URL+"/"+readyGrpcService+"/Wait", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", ct)
			req.Header.Set(AuthenticationTokenKey, "token")
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(resp.Body, make([]byte, 5)); err != nil {
				t.Fatalf("%s: first message: %v", ct, err)
			}
			cancel()
			resp.Body.Close()

			select {
			case err := <-grpcWebWaitDone:
				if err != context.Canceled {
					t.Errorf("%s: stream ended with %v", ct, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s: stream still running after cancel", ct)
			}
		}
	}
}

// TestGrpcWebTrailersOnly ensures a call that fails before sending anything
// returns its status in the headers and an empty body.
func TestGrpcWebTrailersOnly(t *testing.T) {
	h1, _ := newGrpcWebTestServer(t)
	big := wrapperspb.String(strings.Repeat("x", 5<<20))
	tests := []struct {
		name, method, token string
		msg                 proto.Message
		code                codes.Code
		message             string
	}{
		{"error", "Echo", "token", wrapperspb.String("fail"), codes.InvalidArgument, "bad value"},
		{"bad token", "Echo", "wrong", wrapperspb.String("hi"), codes.Unknown, "invalid authentication token"},
		{"bad stream token", "Count", "", wrapperspb.UInt32(1), codes.Unknown, "invalid authentication token"},
		{"too large", "Echo", "token", big, codes.ResourceExhausted, ""},
	}
	for _, test := range tests {
		for _, ct := range grpcWebTestTypes {
			body := grpcWebFrame(t, test.msg)
			resp, frames := grpcWebCall(t, h1, test.method, ct.request, test.token, body)
			checkGrpcWebHeaders(t, resp, ct.response)
			if len(frames) != 0 {
				t.Errorf("%s %s: frames %q", test.name, ct.request, frames)
			}
			if got := resp.Header.Get("Grpc-Status"); got != fmt.Sprint(int(test.code)) {
				t.Errorf("%s %s: status %q, want %d", test.name, ct.request, got, test.code)
			}
			got := resp.Header.Get("Grpc-Message")
			if !strings.HasPrefix(got, test.message) {
				t.Errorf("%s %s: message %q", test.name, ct.request, got)
			}
		}
	}
}

// TestGrpcWebPreflight ensures only preflights for gRPC-Web calls of known
// methods are answered, without allowing credentials.
func TestGrpcWebPreflight(t *testing.T) {
	h1, _ := newGrpcWebTestServer(t)
	tests := []struct {
		path, headers string
		want          int
	}{
		{"/" + readyGrpcService + "/Echo", "content-type,x-grpc-web,x-user-agent", http.StatusNoContent},
		{"/" + readyGrpcService + "/Count", "X-Grpc-Web", http.StatusNoContent},
		{"/" + readyGrpcService + "/Missing", "x-grpc-web", http.StatusMethodNotAllowed},
		{"/" + readyGrpcService + "/Echo", "content-type", http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		req, err := http.NewRequest(http.MethodOptions, h1.URL+test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", test.headers)
		resp, err := h1.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != test.want {
			t.Errorf("%s %s: status %d, want %d", test.path, test.headers,
				resp.StatusCode, test.want)
		}
		if test.want != http.StatusNoContent {
			continue
		}
		want := map[string]string{
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST",
			"Access-Control-Allow-Headers": test.headers,
			"Access-Control-Max-Age":       "600",
			"Vary":                         "Access-Control-Request-Headers",
		}
		for k, v := range want {
			if got := resp.Header.Get(k); got != v {
				t.Errorf("%s: %s %q, want %q", test.path, k, got, v)
			}
		}
		if _, ok := resp.Header["Access-Control-Allow-Credentials"]; ok {
			t.Errorf("%s: preflight allows credentials", test.path)
		}
	}
}

// TestGrpcWebOtherRequests ensures requests that are not gRPC-Web reach the
// gRPC server unchanged, and that native gRPC calls work.
func TestGrpcWebOtherRequests(t *testing.T) {
	h1, h2 := newGrpcWebTestServer(t)
	tests := []struct {
		method, contentType string
		want                int
	}{
		{http.MethodGet, "", http.StatusMethodNotAllowed},
		{http.MethodGet, "application/grpc-web", http.StatusMethodNotAllowed},
		{http.MethodPost, "application/json", http.StatusUnsupportedMediaType},
		{http.MethodPost, "application/grpc", http.StatusHTTPVersionNotSupported},
	}
	for _, test := range tests {
		req, err := http.NewRequest(test.method, h1.URL+"/"+readyGrpcService+"/Echo", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", test.contentType)
		resp, err := h1.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != test.want {
			t.Errorf("%s %q: status %d, want %d", test.method, test.contentType,
				resp.StatusCode, test.want)
		}
		if _, ok := resp.Header["Access-Control-Allow-Origin"]; ok {
			t.Errorf("%s %q: CORS headers on a gRPC response", test.method, test.contentType)
		}
	}

	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	conn, err := grpc.NewClient(strings.TrimPrefix(h2.URL, "https://"),
		grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		AuthenticationTokenKey, "token")
	out := new(wrapperspb.StringValue)
	err = conn.Invoke(ctx, "/"+readyGrpcService+"/Echo", wrapperspb.String("hi"), out)
	if err != nil || out.GetValue() != "echo hi" {
		t.Fatalf("native call: %q, %v", out.GetValue(), err)
	}
	err = conn.Invoke(ctx, "/"+readyGrpcService+"/Echo", wrapperspb.String("fail"), out)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("native call error %v", err)
	}
}

// TestBase64Body ensures text request bodies decode across separately padded
// chunks, line breaks and short reads, and that bad input is an error.
func TestBase64Body(t *testing.T) {
	tests := []struct {
		in, want string
		err      bool
	}{
		{"", "", false},
		{"aGVsbG8=", "hello", false},
		{"aGVsbG8gd29ybGQ=", "hello world", false},
		{"aA==ZQ==bGxv", "hello", false},
		{"aGU=\r\nbGxv", "hello", false},
		{"aGVsbG8", "hel", true},
		{"aGVs*G8=", "", true},
		{"aA=a", "", true},
	}
	for _, test := range tests {
		for _, oneByte := range []bool{false, true} {
			var r io.Reader = strings.NewReader(test.in)
			if oneByte {
				r = iotest.OneByteReader(r)
			}
			b := &base64Body{body: io.NopCloser(r)}
			got, err := io.ReadAll(b)
			if (err != nil) != test.err {
				t.Errorf("%q: error %v", test.in, err)
			}
			if !test.err && string(got) != test.want {
				t.Errorf("%q: got %q, want %q", test.in, got, test.want)
			}
		}
	}

	long := bytes.Repeat([]byte("0123456789"), 2000)
	b := &base64Body{body: io.NopCloser(strings.NewReader(
		base64.StdEncoding.EncodeToString(long)))}
	got, err := io.ReadAll(b)
	if err != nil || !bytes.Equal(got, long) {
		t.Errorf("long body: %d bytes, %v", len(got), err)
	}
}

// grpcWebModes are a binary and a text gRPC-Web content type.
var grpcWebModes = []string{"application/grpc-web+proto",
	"application/grpc-web-text+proto"}

// postGrpcWeb sends a gRPC-Web call of method with req over a new HTTP/1.1
// connection to addr with a socket read buffer of readBuffer bytes, and
// returns the response once its headers have been read.  Its body is read
// from the connection only as the caller reads it.
func postGrpcWeb(t *testing.T, addr, method, contentType string, req proto.Message, readBuffer int) *http.Response {
	t.Helper()
	tcp, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = tcp.(*net.TCPConn).SetReadBuffer(readBuffer)
	conn := tls.Client(tcp, &tls.Config{InsecureSkipVerify: true,
		NextProtos: []string{"http/1.1"}})
	t.Cleanup(func() { conn.Close() })

	body := grpcWebFrame(t, req)
	if strings.HasPrefix(contentType, grpcWebTextContentType) {
		body = base64.StdEncoding.AppendEncode(nil, body)
	}
	hreq, err := http.NewRequest(http.MethodPost, "https://localhost"+method,
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	hreq.Header.Set("Content-Type", contentType)
	hreq.Header.Set("X-Grpc-Web", "1")
	if err := hreq.Write(conn); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), hreq)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK ||
		resp.Header.Get("Content-Type") != contentType {

		t.Fatalf("%s: status %d, content type %q", contentType,
			resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return resp
}

// TestGrpcWebStalledReaderReset ensures that a gRPC-Web call over HTTP/2
// whose client grants no flow control, binary or text, unary or streaming, is
// reset once the client has taken none of its reply for grpcWriteTimeout, as
// a native gRPC call is.
func TestGrpcWebStalledReaderReset(t *testing.T) {
	const timeout = 500 * time.Millisecond
	for _, ct := range grpcWebModes {
		for _, method := range []string{"/pb.bchrpc/GetRawBlock",
			"/pb.bchrpc/SubscribeBlocks"} {

			svc := &grpcWriteTestService{replySize: 1 << 20,
				notify: make(chan struct{})}
			addr, active := startGrpcWriteTest(t, svc, timeout)

			c := dialRawH2(t, addr, http2.Setting{
				ID: http2.SettingInitialWindowSize, Val: 0})
			var req proto.Message = &pb.GetRawBlockRequest{}
			if method == "/pb.bchrpc/SubscribeBlocks" {
				req = &pb.SubscribeBlocksRequest{}
			}
			c.callAs(1, method, ct, req)
			start := time.Now()
			reset := false
			for !reset {
				f := c.next(start.Add(10 * timeout))
				if f == nil {
					break
				}
				if _, ok := f.(*http2.DataFrame); ok {
					t.Fatalf("%s %s: data sent without a window", ct, method)
				}
				_, reset = f.(*http2.RSTStreamFrame)
			}
			if !reset {
				t.Errorf("%s %s: stream not reset after %v", ct, method,
					time.Since(start))
				continue
			}
			if took := time.Since(start); took < timeout {
				t.Errorf("%s %s: reset after %v, before the timeout", ct,
					method, took)
			}
			waitForGrpc(t, method+" to end", func() bool {
				return active.Load() == 0
			})
		}
	}
}

// TestGrpcWebSlowReaderKeepsCall ensures that a gRPC-Web client over
// HTTP/1.1 that takes a text reply slowly but steadily gets all of it,
// although it takes several times grpcWriteTimeout.  The text reply goes
// through the base64 encoder, which writes it in small parts.
func TestGrpcWebSlowReaderKeepsCall(t *testing.T) {
	const timeout = 500 * time.Millisecond
	const size = 2 << 20
	svc := &grpcWriteTestService{replySize: size}
	addr, _ := startGrpcWriteTest(t, svc, timeout)

	// The socket buffer is fixed, so that the reply cannot all wait in it,
	// and larger than a TCP segment on loopback, as a smaller one stalls
	// TCP itself.
	ct := grpcWebModes[1]
	resp := postGrpcWeb(t, addr, "/pb.bchrpc/GetRawBlock", ct,
		&pb.GetRawBlockRequest{}, 256<<10)
	start := time.Now()

	// The client takes 64 KiB every 40 ms, so the whole reply in about
	// 1.7 s.
	var body []byte
	buf := make([]byte, 64<<10)
	for {
		n, err := io.ReadFull(resp.Body, buf)
		body = append(body, buf[:n]...)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			t.Fatalf("after %d bytes in %v: %v", len(body),
				time.Since(start), err)
		}
		time.Sleep(40 * time.Millisecond)
	}
	took := time.Since(start)

	frames := splitGrpcWebFrames(t, body, true)
	if len(frames) != 2 || len(frames[0].payload) < size ||
		string(frames[1].payload) != "grpc-status: 0\r\n" {

		t.Fatalf("%d frames", len(frames))
	}
	if took < 2*timeout {
		t.Fatalf("the reply took %v, not long enough to test", took)
	}
}

// TestGrpcWebIdleStreamKept ensures that a text gRPC-Web subscription over
// HTTP/1.1 that has nothing to send for longer than grpcWriteTimeout is
// kept.
func TestGrpcWebIdleStreamKept(t *testing.T) {
	const timeout = 300 * time.Millisecond
	svc := &grpcWriteTestService{replySize: 64 << 10,
		notify: make(chan struct{})}
	addr, _ := startGrpcWriteTest(t, svc, timeout)
	resp := postGrpcWeb(t, addr, "/pb.bchrpc/SubscribeBlocks", grpcWebModes[1],
		&pb.SubscribeBlocksRequest{}, 256<<10)
	msg, err := proto.Marshal(&pb.BlockNotification{
		Block: &pb.BlockNotification_SerializedBlock{
			SerializedBlock: make([]byte, svc.replySize),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	n := base64.StdEncoding.EncodedLen(5 + len(msg))
	for i := 0; i < 3; i++ {
		buf := make([]byte, n)
		if _, err := io.ReadFull(resp.Body, buf); err != nil {
			t.Fatalf("notification %d: %v", i, err)
		}
		frames := splitGrpcWebFrames(t, buf, true)
		if len(frames) != 1 || frames[0].flag != 0 {
			t.Fatalf("notification %d: %d frames", i, len(frames))
		}
		time.Sleep(3 * timeout)
		svc.notify <- struct{}{}
	}
}
