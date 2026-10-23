package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gcash/bchd/bchrpc"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/traefik/grpc-web/go/grpcweb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// AuthenticationTokenKey is the key used in the context to authenticate clients.
// If this is set to anything other than "" in the config, then the server expects
// the client to set a key value in the context metadata to 'AuthenticationToken: cfg.AuthToken'
const AuthenticationTokenKey = "AuthenticationToken"

// grpcReadHeaderTimeout and grpcIdleTimeout bound how long the gRPC HTTP
// server waits for request headers and keeps idle connections.  There are no
// whole-request read or write timeouts because streams are long-lived.
var (
	grpcReadHeaderTimeout = 10 * time.Second
	grpcIdleTimeout       = 5 * time.Minute
)

// grpcWriteTimeout is how long a gRPC client may take none of a reply being
// written to it.  A stream whose client takes nothing for longer is reset, and
// a connection to which nothing can be written for longer is closed, so a
// client that stops reading cannot hold a call's goroutines and the reply for
// good, as BCHN's -rpcservertimeout does for its HTTP clients.  It only runs
// while a reply is being written, so a subscription with nothing to send is
// kept however long it waits.
var grpcWriteTimeout = 30 * time.Second

// grpcWritePartSize is how much of a reply is written to a gRPC stream each
// time its write deadline is renewed: a client must take this much in each
// grpcWriteTimeout.
const grpcWritePartSize = 64 * 1024

// grpcReadTimeout is how long a gRPC client may take to send each
// grpcWritePartSize bytes of a call's request, as BCHN's HTTP server ends a
// request it does not receive within -rpcservertimeout, 30 s by default
// (httpserver.cpp:434-435, httpserver.h:29).  It runs from the request's
// headers until its body ends, so a client that sends the headers and nothing
// more cannot hold the call's goroutines, which the call limit counts only
// once the request has been read.  A call whose client sends messages when it
// chooses has its request once its handler starts.
var grpcReadTimeout = 30 * time.Second

// grpcMaxReplyBytes is how much memory the gRPC replies being written may
// hold before those whose clients have stalled are reset to make room.  A
// call's place is freed when its handler returns, so without a bound,
// clients that take their replies slowly or not at all could make the node
// hold any number of them, each until its write deadline ends it.  No reply
// waits for room, and none whose client takes a part of it in each
// grpcStallTimeout is reset for room, so while no other has stalled, a reply
// is written over the bound.  BCHN has no such bound: its replies wait in its
// HTTP server's output buffers, but every RPC request needs its password.
var grpcMaxReplyBytes int64 = 512 << 20

// grpcStallTimeout is how long a gRPC client may take none of a part of a
// reply while the replies being written hold more than grpcMaxReplyBytes.
// The writes of clients that have taken nothing for longer are reset then,
// those stalled longest first, until the others fit.  A reset is written
// behind the frame being written, so the connection of a client whose reset
// has not ended the write grpcStallTimeout later is closed, as a client that
// does not read its socket takes neither.
var grpcStallTimeout = 2 * time.Second

// grpcResetDeadline is a write deadline in the past, which resets a stream at
// once.
var grpcResetDeadline = time.Unix(1, 0)

// grpcReplyBudget counts the memory that the gRPC replies being written hold.
// A reply counts from its first write until its flush, by the length in its
// message prefix, as gRPC holds the whole message until all of it has been
// written, and at least grpcWritePartSize, for its stream's goroutines and
// buffers.
type grpcReplyBudget struct {
	mu      sync.Mutex
	max     int64
	used    int64 // the bytes the replies counted hold
	resets  int64 // the bytes of those reset, until their writes return
	writers map[*grpcDeadlineWriter]struct{}
}

// newGrpcReplyBudget returns a budget of max bytes.
func newGrpcReplyBudget(max int64) *grpcReplyBudget {
	return &grpcReplyBudget{max: max,
		writers: make(map[*grpcDeadlineWriter]struct{})}
}

// count counts w's reply in the budget as at least n bytes until release,
// with took set when its client has just taken a part.  While the replies
// counted, less those reset, hold more than the budget, it resets the writes
// of others whose clients have taken none of a part for grpcStallTimeout,
// those stalled longest first, and closes the connection of one whose write
// has not returned grpcStallTimeout later.  It returns false once w has been
// reset.
func (b *grpcReplyBudget) count(w *grpcDeadlineWriter, n int64, took bool) bool {
	type victim struct {
		w       *grpcDeadlineWriter
		stalled time.Duration
	}
	b.mu.Lock()
	if w.reset.Load() {
		b.mu.Unlock()
		return false
	}
	now := time.Now()
	if _, ok := b.writers[w]; !ok {
		b.writers[w] = struct{}{}
		w.held, w.since = 0, now
	}
	if took {
		w.since = now
	}
	if n = max(n, grpcWritePartSize); n > w.held {
		b.used += n - w.held
		w.held = n
	}
	var victims []victim
	for b.used-b.resets > b.max {
		var v *grpcDeadlineWriter
		for o := range b.writers {
			if o != w && !o.reset.Load() &&
				now.Sub(o.since) >= grpcStallTimeout &&
				(v == nil || o.since.Before(v.since)) {

				v = o
			}
		}
		if v == nil {
			break
		}
		v.reset.Store(true)
		v.counted = true
		b.resets += v.held
		after := grpcStallTimeout
		v.closer = time.AfterFunc(after, func() { b.closeStalled(v, after) })
		victims = append(victims, victim{v, now.Sub(v.since)})
	}
	used := b.used
	b.mu.Unlock()

	for _, v := range victims {
		v.w.resetStream()
		grpcLog.Debugf("Reset a gRPC reply to %s, which took none of it for "+
			"%v while the replies being written held %d bytes", v.w.remote,
			v.stalled.Round(time.Millisecond), used)
	}
	return true
}

// release ends the count of w's reply.
func (b *grpcReplyBudget) release(w *grpcDeadlineWriter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.writers[w]; !ok {
		return
	}
	delete(b.writers, w)
	b.used -= w.held
	if w.counted {
		b.resets -= w.held
		w.counted = false
	}
	w.held = 0
	if w.closer != nil {
		w.closer.Stop()
		w.closer = nil
	}
}

// closeStalled closes w's connection when the write of its reply, reset
// after ago, has not returned, as the reset then waits behind a frame its
// client does not take.
func (b *grpcReplyBudget) closeStalled(w *grpcDeadlineWriter, after time.Duration) {
	b.mu.Lock()
	_, held := b.writers[w]
	b.mu.Unlock()
	if held {
		w.closeConn(after)
	}
}

func newGrpcServer(netAddrs []net.Addr, rpcCfg *bchrpc.GrpcServerConfig, svr *server) (*bchrpc.GrpcServer, error) {
	if len(netAddrs) == 0 {
		return nil, nil
	}
	// Only the first configured listen address is wired up; the function
	// returns a single server instance.
	addr := netAddrs[0]
	rpcCfg.NetMgr = svr
	opts := grpcServerOptions(newGrpcCallLimiter(cfg.GrpcMaxConcurrentCalls))
	creds, err := credentials.NewServerTLSFromFile(cfg.RPCCert, cfg.RPCKey)
	if err != nil {
		return nil, err
	}
	opts = append(opts, grpc.Creds(creds))
	server := grpc.NewServer(opts...)

	rpcCfg.Server = server

	httpServer := newGrpcHTTPServer(addr.String(), grpcHTTPHandler(server,
		newGrpcReplyBudget(grpcMaxReplyBytes)))

	rpcCfg.HTTPServer = httpServer

	gRPCServer := bchrpc.NewGrpcServer(rpcCfg)

	if len(cfg.PrometheusListen) != 0 {
		metrics := newGrpcServerMetrics()
		if err := metrics.register(prometheus.DefaultRegisterer, server); err != nil {
			return nil, err
		}

		router := mux.NewRouter()
		router.Handle("/metrics", promhttp.Handler())

		prometheusHTTPServer := &http.Server{
			Addr:         cfg.PrometheusListen,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		grpcMetrics = metrics

		go func() {
			if err := prometheusHTTPServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
				grpcLog.Tracef("Finished serving Prometheus metrics %v", err)
			}
		}()
	}

	// Serve only after metrics are set up, since the interceptors read
	// grpcMetrics.
	grpcLog.Infof("Experimental gRPC server listening on %s", addr)
	go func() {
		if err := httpServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
			grpcLog.Tracef("Finished serving expimental gRPC: %v", err)
		}
	}()

	return gRPCServer, nil
}

// grpcServerOptions returns the gRPC server's interceptors, with calls
// bounding the unary calls once they are authorized.
func grpcServerOptions(calls *grpcCallLimiter) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StreamInterceptor(interceptStreaming),
		grpc.ChainUnaryInterceptor(interceptUnary, calls.intercept),
	}
}

// grpcCallQueueFactor is how many unary gRPC calls may wait for a place, as a
// multiple of how many may be in progress.
const grpcCallQueueFactor = 4

// grpcCallLimiter bounds the unary gRPC calls in progress, from the time a
// call's request has been read and its caller authorized until its handler has
// made the reply.  Any client may call when no auth token is set, and a call
// can make the node read a block and build a reply larger still.  A call over
// the limit waits for a place, while fewer than grpcCallQueueFactor times the
// limit wait, and is refused with ResourceExhausted otherwise, as BCHN's HTTP
// server runs 16 requests at once and queues 64 more.  The reply is sent
// without a place, as BCHN's worker threads hand theirs to the connection, so
// a client that takes a reply slowly does not keep other calls waiting, and
// the write deadline ends one that takes none of it.  Subscriptions have their
// own limit.
type grpcCallLimiter struct {
	places     chan struct{} // a value for each call in progress
	waiting    atomic.Int64  // calls waiting for a place
	maxWaiting int64
}

// newGrpcCallLimiter returns a limiter of max unary calls in progress.
func newGrpcCallLimiter(max int) *grpcCallLimiter {
	return &grpcCallLimiter{
		places:     make(chan struct{}, max),
		maxWaiting: grpcCallQueueFactor * int64(max),
	}
}

// intercept runs a unary call once it holds a place, waiting for one until
// its context ends.  The place is freed when the handler returns or panics,
// before the reply is sent.
func (l *grpcCallLimiter) intercept(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	select {
	case l.places <- struct{}{}:
	default:
		if l.waiting.Add(1) > l.maxWaiting {
			l.waiting.Add(-1)
			return nil, status.Errorf(codes.ResourceExhausted,
				"too many calls, the limit is %d", cap(l.places))
		}
		select {
		case l.places <- struct{}{}:
			l.waiting.Add(-1)
		case <-ctx.Done():
			l.waiting.Add(-1)
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
	defer func() { <-l.places }()
	return handler(ctx, req)
}

// grpcHTTPHandler returns the handler of the gRPC listener, which passes
// gRPC-Web requests and their CORS preflights to the gRPC-Web wrapper of
// server and the others to server, and writes the replies of both with write
// deadlines, within the replies budget.
func grpcHTTPHandler(server *grpc.Server, replies *grpcReplyBudget) http.Handler {
	allowAllOrigins := grpcweb.WithOriginFunc(func(origin string) bool {
		return true
	})
	wrappedGrpc := grpcweb.WrapServer(server, allowAllOrigins)

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		conn, _ := req.Context().Value(grpcConnKey{}).(net.Conn)
		w := &grpcDeadlineWriter{ResponseWriter: resp,
			rc: http.NewResponseController(resp), replies: replies,
			conn: conn, remote: req.RemoteAddr,
			text: wrappedGrpc.IsGrpcWebRequest(req) && strings.HasPrefix(
				req.Header.Get("Content-Type"), grpcWebTextContentType)}
		defer w.close()
		if req.Body != http.NoBody {
			d := &grpcReadDeadline{rc: w.rc}
			defer d.end()
			d.renew()
			req = req.WithContext(context.WithValue(req.Context(),
				grpcReadDeadlineKey{}, d))
			req.Body = &grpcDeadlineBody{ReadCloser: req.Body, deadline: d}
		}
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(w, req)
		} else {
			server.ServeHTTP(w, req)
		}
	})
}

// grpcReadDeadlineKey is the context key of a request's grpcReadDeadline.
type grpcReadDeadlineKey struct{}

// grpcReadDeadline is the read deadline of a gRPC request's body, renewed for
// each grpcWritePartSize bytes received until the call has its request.
type grpcReadDeadline struct {
	rc *http.ResponseController

	mu    sync.Mutex
	got   int  // bytes received since the deadline was renewed
	done  bool // the call has its request
	ended bool // the request has ended
}

// renew sets the read deadline grpcReadTimeout ahead, unless the call has its
// request or the request has ended.
func (d *grpcReadDeadline) renew() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.done && !d.ended {
		_ = d.rc.SetReadDeadline(time.Now().Add(grpcReadTimeout))
	}
}

// read counts n bytes of the body received, renewing the deadline once a part
// has come, and clears it once the body has ended or failed.
func (d *grpcReadDeadline) read(n int, err error) {
	if err != nil {
		d.clear()
		return
	}
	d.mu.Lock()
	d.got += n
	full := d.got >= grpcWritePartSize
	if full {
		d.got = 0
	}
	d.mu.Unlock()
	if full {
		d.renew()
	}
}

// clear clears the read deadline, as the call has its request.
func (d *grpcReadDeadline) clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.done && !d.ended {
		d.done = true
		_ = d.rc.SetReadDeadline(time.Time{})
	}
}

// end marks the request ended, after which its ResponseController may no
// longer be used.
func (d *grpcReadDeadline) end() {
	d.mu.Lock()
	d.ended = true
	d.mu.Unlock()
}

// grpcDeadlineBody is the body of a gRPC request, read under its read
// deadline.
type grpcDeadlineBody struct {
	io.ReadCloser
	deadline *grpcReadDeadline
}

func (b *grpcDeadlineBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.deadline.read(n, err)
	return n, err
}

// grpcConnKey is the context key of a gRPC request's connection.
type grpcConnKey struct{}

// grpcWebTextContentType is the content type of gRPC-Web's text format, whose
// replies are base64 encoded.
const grpcWebTextContentType = "application/grpc-web-text"

// errGrpcReplyReset is returned for the writes of a reply reset to make room
// for others.
var errGrpcReplyReset = errors.New("reply reset to make room for others")

// grpcDeadlineWriter is the ResponseWriter of a gRPC request.  It sets the
// write deadline of the request's stream at the first write of each reply,
// renews it each time grpcWritePartSize bytes more of the reply have been
// written, and clears it once the reply's flush returns, so a stream is reset
// only when its client takes none of a part for grpcWriteTimeout, however
// large the reply, however it is split into writes and however long the
// stream is idle between replies.  Its replies are counted in the replies
// budget, which may reset them sooner.
type grpcDeadlineWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	replies *grpcReplyBudget
	conn    net.Conn // the request's connection, if known
	remote  string   // the client's address, for the log
	text    bool     // the replies are base64, in gRPC-Web's text format

	// The reply being written, from its first write until its flush.
	// gRPC writes a stream's replies one at a time, from one goroutine.
	open    bool
	prefix  [8]byte // the reply's first bytes, until its length is read
	nprefix int
	sized   bool  // the prefix has been read
	known   bool  // and gave the reply's length
	written int64 // the bytes of the reply written
	part    int   // those written since the deadline was renewed

	// held, since, counted and closer are guarded by the budget's lock: the
	// bytes the reply counts, when its client last took a part of it,
	// whether the budget reset it and counts its bytes in its resets, and
	// the timer that then closes the connection.
	held    int64
	since   time.Time
	counted bool
	closer  *time.Timer

	// reset is set once the budget has reset the stream, and mu orders the
	// deadlines set by the writes and by the reset, and the reset after
	// the request's end, closed.
	reset  atomic.Bool
	mu     sync.Mutex
	closed bool
}

// Write writes p as part of the reply its first write began, renewing the
// write deadline each time grpcWritePartSize bytes more have been written.
// The writes of a writer that has been reset are refused.
func (w *grpcDeadlineWriter) Write(p []byte) (int, error) {
	if !w.open {
		w.begin()
	}
	if !w.sized {
		w.readPrefix(p)
	}
	n := 0
	for {
		if w.reset.Load() {
			w.replies.release(w)
			return n, errGrpcReplyReset
		}
		if n == len(p) {
			return n, nil
		}
		m, err := w.ResponseWriter.Write(
			p[n:min(len(p), n+grpcWritePartSize-w.part)])
		n += m
		w.part += m
		w.written += int64(m)
		if err != nil {
			if w.reset.Load() {
				w.replies.release(w)
			}
			return n, err
		}
		if w.part == grpcWritePartSize {
			w.part = 0
			var grown int64
			if !w.known {
				grown = w.written
			}
			w.replies.count(w, grown, true)
			w.renewDeadline()
		}
	}
}

// begin starts a reply at its first write, counting it in the budget, and
// sets its write deadline.
func (w *grpcDeadlineWriter) begin() {
	w.open = true
	w.nprefix, w.sized, w.known, w.written, w.part = 0, false, false, 0, 0
	w.replies.count(w, 0, false)
	w.renewDeadline()
}

// readPrefix reads the length of the reply from the gRPC message prefix in
// its first 5 bytes, base64 encoded in its first 8 in gRPC-Web's text format,
// as the writes bring them, and counts the whole reply.  A reply without a
// valid prefix counts what has been written of it.
func (w *grpcDeadlineWriter) readPrefix(p []byte) {
	size := 5
	if w.text {
		size = 8
	}
	w.nprefix += copy(w.prefix[w.nprefix:size], p)
	if w.nprefix < size {
		return
	}
	w.sized = true
	prefix := w.prefix[:5]
	if w.text {
		var raw [6]byte
		n, err := base64.StdEncoding.Decode(raw[:], w.prefix[:8])
		if err != nil || n < 5 {
			return
		}
		prefix = raw[:5]
	}

	// The flag byte says whether the message is compressed, and in gRPC-Web
	// whether it holds the trailers.
	if prefix[0]&^0x81 != 0 {
		return
	}
	w.known = true
	w.replies.count(w, 5+int64(binary.BigEndian.Uint32(prefix[1:])), false)
}

// Flush sends what has been written of the reply, under its write deadline,
// and ends it.  gRPC requires the writer to be an http.Flusher, and flushes
// each message once it is written.  A flush with nothing written, of the
// headers or trailers alone, counts as a reply while it waits.
func (w *grpcDeadlineWriter) Flush() {
	if !w.open {
		w.begin()
	}
	if !w.reset.Load() {
		_ = w.rc.Flush()
	}
	w.open = false
	w.clearDeadline()
	w.replies.release(w)
}

// renewDeadline sets the write deadline grpcWriteTimeout ahead, unless the
// stream has been reset, which it returns false for.
func (w *grpcDeadlineWriter) renewDeadline() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.reset.Load() {
		return false
	}
	_ = w.rc.SetWriteDeadline(time.Now().Add(grpcWriteTimeout))
	return true
}

// clearDeadline clears the write deadline, unless the stream has been reset.
func (w *grpcDeadlineWriter) clearDeadline() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.reset.Load() {
		_ = w.rc.SetWriteDeadline(time.Time{})
	}
}

// resetStream sets a write deadline in the past, which resets the stream of
// an HTTP/2 request at once and fails the writes of an HTTP/1.1 request,
// unless the request has ended.
func (w *grpcDeadlineWriter) resetStream() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		_ = w.rc.SetWriteDeadline(grpcResetDeadline)
	}
}

// close marks the request ended, after which its ResponseController may no
// longer be used, and ends the count of a reply not flushed.
func (w *grpcDeadlineWriter) close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.replies.release(w)
}

// closeConn closes the request's connection, which ends every call on it,
// for a client that took none of its reset reply for after.  The socket of a
// TLS connection is closed directly, as closing the TLS connection first
// gives an alert up to 5 s to be written to the socket the client does not
// read.
func (w *grpcDeadlineWriter) closeConn(after time.Duration) {
	conn := w.conn
	if tc, ok := conn.(*tls.Conn); ok {
		conn = tc.NetConn()
	}
	if conn == nil {
		return
	}
	grpcLog.Debugf("Closing the gRPC connection of %s, which took none of a "+
		"reset reply for %v", w.remote, after)
	_ = conn.Close()
}

// newGrpcHTTPServer returns the HTTP server for the gRPC listener.  An HTTP/2
// connection to which nothing can be written for grpcWriteTimeout is closed,
// as a client that does not read its socket stalls every stream on it, which
// the deadline of each stream cannot end.  Each request's context holds its
// connection, which the replies budget closes sooner.
func newGrpcHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: grpcReadHeaderTimeout,
		IdleTimeout:       grpcIdleTimeout,
		HTTP2: &http.HTTP2Config{
			WriteByteTimeout: grpcWriteTimeout,
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return context.WithValue(ctx, grpcConnKey{}, c)
		},
	}
}

// serviceName returns the package.service segment from the full gRPC method
// name `/package.service/method`.
func serviceName(method string) string {
	// Slice off first /
	method = method[1:]
	// Keep everything before the next /
	return method[:strings.IndexRune(method, '/')]
}

// interceptStreaming runs a streaming call through panic recovery, then
// authentication and the service readiness check, then metrics.  Metrics come
// last so that the handler never runs for an unauthenticated caller.
func interceptStreaming(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer recoverGrpcPanic(info.FullMethod, &err)

	// A call whose client streams sends its messages when it chooses, so
	// it has its request once it starts.
	if info.IsClientStream {
		if d, ok := ss.Context().Value(grpcReadDeadlineKey{}).(*grpcReadDeadline); ok {
			d.clear()
		}
	}

	// This line is logged for every call, before the caller is
	// authenticated, so it is at debug.
	p, ok := peer.FromContext(ss.Context())
	if ok {
		grpcLog.Debugf("Streaming method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	if err := authorizeGrpcCall(ss.Context(), info.FullMethod); err != nil {
		return err
	}

	if grpcMetrics != nil {
		err = grpcMetrics.interceptStreaming(srv, ss, info, handler)
	} else {
		err = handler(srv, ss)
	}
	if err != nil && ok {
		logGrpcError("Streaming", info.FullMethod, p.Addr.String(), err)
	}
	return err
}

// interceptUnary runs a unary call through panic recovery, then
// authentication and the service readiness check, then metrics.  Metrics come
// last so that the handler never runs for an unauthenticated caller.
func interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer recoverGrpcPanic(info.FullMethod, &err)

	// This line is logged for every call, before the caller is
	// authenticated, so it is at debug.
	p, ok := peer.FromContext(ctx)
	if ok {
		grpcLog.Debugf("Unary method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	if err := authorizeGrpcCall(ctx, info.FullMethod); err != nil {
		return nil, err
	}

	if grpcMetrics != nil {
		resp, err = grpcMetrics.interceptUnary(ctx, req, info, handler)
	} else {
		resp, err = handler(ctx, req)
	}
	if err != nil && ok {
		logGrpcError("Unary", info.FullMethod, p.Addr.String(), err)
	}
	return resp, err
}

// logGrpcError logs the error a call failed with.  An error that answers the
// caller, such as an unknown transaction, a bad argument, an SLP transaction
// refused with Aborted, an index the node does not run, a limit such as on
// subscriptions or proof searches, the end of a subscription it canceled or of
// the server, or its deadline passing, is logged at debug, so that a client
// polling for something cannot flood the log with errors.
func logGrpcError(kind, method, addr string, err error) {
	format := "%s method %s invoked by %s errored: %v"
	switch status.Code(err) {
	case codes.NotFound, codes.InvalidArgument, codes.OutOfRange,
		codes.AlreadyExists, codes.FailedPrecondition, codes.Canceled,
		codes.Aborted, codes.Unavailable, codes.ResourceExhausted,
		codes.DeadlineExceeded:

		grpcLog.Debugf(format, kind, method, addr, err)
		return
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {

		grpcLog.Debugf(format, kind, method, addr, err)
		return
	}
	grpcLog.Errorf(format, kind, method, addr, err)
}

// recoverGrpcPanic turns a panic in a gRPC handler into an Internal error, so
// a single call can't stop the process.  It must be deferred.
func recoverGrpcPanic(method string, err *error) {
	if r := recover(); r != nil {
		grpcLog.Errorf("Recovered from panic in gRPC method %s: %v\n%s",
			method, r, debug.Stack())
		*err = status.Error(codes.Internal, "internal error")
	}
}

// authorizeGrpcCall checks the auth token and that the called service is
// ready.
func authorizeGrpcCall(ctx context.Context, method string) error {
	if err := validateAuthenticationToken(ctx); err != nil {
		return err
	}
	return bchrpc.ServiceReady(serviceName(method))
}

func validateAuthenticationToken(ctx context.Context) error {
	if cfg.GrpcAuthToken == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("invalid authentication token")
	}
	tokens := md.Get(AuthenticationTokenKey)
	if len(tokens) == 0 {
		return errors.New("invalid authentication token")
	}
	// Compare in constant time, over hashes so the lengths always match.
	got := sha256.Sum256([]byte(tokens[0]))
	want := sha256.Sum256([]byte(cfg.GrpcAuthToken))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return errors.New("invalid authentication token")
	}
	return nil
}
