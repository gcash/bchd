// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/txscript"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
	"github.com/gorilla/websocket"
)

const (
	testRPCAdminLogin   = "admin:adminpass"
	testRPCLimitedLogin = "limited:limitedpass"
)

// basicAuth returns the HTTP basic authorization header value for login.
func basicAuth(login string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
}

var testRPCGlobalsOnce sync.Once

// testRPCLog collects what the test RPC server logs.
var testRPCLog syncBuffer

// syncBuffer is a bytes.Buffer that is safe for concurrent use.
type syncBuffer struct {
	mtx sync.Mutex
	buf bytes.Buffer
}

// Write appends p to the buffer.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.buf.Write(p)
}

// Len returns the number of bytes written so far.
func (b *syncBuffer) Len() int {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.buf.Len()
}

// Since returns what was written after the first n bytes.
func (b *syncBuffer) Since(n int) string {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return string(b.buf.Bytes()[n:])
}

// setTestRPCGlobals sets the globals the test RPC server uses.  Client
// goroutines may outlive Stop, so the globals are set once and not restored.
func setTestRPCGlobals() {
	testRPCGlobalsOnce.Do(func() {
		cfg = &config{
			RPCMaxClients:        10,
			RPCMaxWebsockets:     25,
			RPCMaxConcurrentReqs: 20,
			RPCAuthTimeout:       1,
		}
		rpcsLog = bchlog.NewBackend(&testRPCLog).Logger("RPCS")
	})
}

// startTestRPCServer starts the RPC HTTP and websocket server on a loopback
// listener with an admin and a limited user.  It returns the listen address.
// opts can change the server before it starts.
func startTestRPCServer(t *testing.T, opts ...func(*rpcServer)) (*rpcServer, string) {
	t.Helper()

	setTestRPCGlobals()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &rpcServer{
		statusLines:            make(map[int]string),
		helpCacher:             newHelpCacher(),
		requestProcessShutdown: make(chan struct{}),
		quit:                   make(chan int),
	}
	s.cfg.Listeners = []net.Listener{ln}
	s.cfg.RPCAuthTimeout = 1
	s.authsha = sha256.Sum256([]byte(basicAuth(testRPCAdminLogin)))
	s.limitauthsha = sha256.Sum256([]byte(basicAuth(testRPCLimitedLogin)))
	s.ntfnMgr = newWsNotificationManager(s)
	for _, opt := range opts {
		opt(s)
	}
	s.Start()
	t.Cleanup(func() {
		_ = s.Stop()
	})
	return s, ln.Addr().String()
}

// dialTestWebsocket opens a websocket to the test server with the given
// request headers.
func dialTestWebsocket(t *testing.T, addr string, hdr http.Header) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/ws", hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// wsRoundTrip sends req and returns the reply, or an error when the server
// closed the connection.
func wsRoundTrip(conn *websocket.Conn, req string) (string, error) {
	if err := conn.WriteMessage(websocket.TextMessage, []byte(req)); err != nil {
		return "", err
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	return string(msg), err
}

// withPanickingHandler replaces the getblockcount handler with one that
// panics for the duration of the test.
func withPanickingHandler(t *testing.T) {
	t.Helper()
	saved := rpcHandlers["getblockcount"]
	rpcHandlers["getblockcount"] = func(*rpcServer, interface{}, <-chan struct{}) (interface{}, error) {
		panic("test handler panic")
	}
	t.Cleanup(func() { rpcHandlers["getblockcount"] = saved })
}

// TestWebsocketHandlerPanicRecovered ensures a panicking RPC handler run over
// the websocket answers with an internal error instead of stopping the
// process, for single and batched requests.
func TestWebsocketHandlerPanicRecovered(t *testing.T) {
	withPanickingHandler(t)
	_, addr := startTestRPCServer(t)
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	conn := dialTestWebsocket(t, addr, hdr)

	for _, req := range []string{
		`{"jsonrpc":"1.0","id":1,"method":"getblockcount","params":[]}`,
		`[{"jsonrpc":"2.0","id":2,"method":"getblockcount","params":[]}]`,
	} {
		reply, err := wsRoundTrip(conn, req)
		if err != nil {
			t.Fatalf("%s: %v", req, err)
		}
		if !strings.Contains(reply, `"code":-32603`) {
			t.Errorf("%s: unexpected reply %s", req, reply)
		}
	}
}

// TestHTTPHandlerPanicRecovered ensures a panicking RPC handler run over HTTP
// answers with an internal error instead of dropping the connection.
func TestHTTPHandlerPanicRecovered(t *testing.T) {
	withPanickingHandler(t)
	_, addr := startTestRPCServer(t)
	body := `{"jsonrpc":"1.0","id":1,"method":"getblockcount","params":[]}`
	req, _ := http.NewRequest("POST", "http://"+addr+"/", strings.NewReader(body))
	req.Header.Set("Authorization", basicAuth(testRPCAdminLogin))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	reply, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(reply), `"code":-32603`) {
		t.Errorf("unexpected reply %s", reply)
	}
}

// TestHTTPRequestBodyLimit ensures an HTTP request body larger than
// maxRPCRequestSize is refused with 413 instead of being read in full, and
// that a body of exactly that size is still answered.
func TestHTTPRequestBodyLimit(t *testing.T) {
	// Use a small excessive block size to keep the limit low.
	wire.SetLimits(1000000)
	t.Cleanup(func() { wire.SetLimits(32000000) })

	s := &rpcServer{}
	req := `{"jsonrpc":"1.0","id":1,"method":"uptime","params":[]}`
	paddedRequest := func(size int64) *http.Request {
		body := io.MultiReader(strings.NewReader(req),
			io.LimitReader(spaceReader{}, size-int64(len(req))))
		return httptest.NewRequest("POST", "/", body)
	}

	w := httptest.NewRecorder()
	s.jsonRPCRead(w, paddedRequest(maxRPCRequestSize()), true)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"id":1`) {
		t.Errorf("size at the limit: got status %d, reply %s", w.Code,
			w.Body.String())
	}

	for _, size := range []int64{maxRPCRequestSize() + 1, 2 * maxRPCRequestSize()} {
		w := httptest.NewRecorder()
		s.jsonRPCRead(w, paddedRequest(size), true)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("size %d: got status %d, want %d", size, w.Code,
				http.StatusRequestEntityTooLarge)
		}
	}
}

// spaceReader is an endless source of spaces.
type spaceReader struct{}

func (spaceReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// TestWebsocketWellFormedRequests ensures valid requests from admin and
// limited clients are answered, with credentials sent in the HTTP request or
// with the authenticate command.
func TestWebsocketWellFormedRequests(t *testing.T) {
	_, addr := startTestRPCServer(t)

	type exchange struct{ req, want string }
	limitedChecks := []exchange{
		{`{"jsonrpc":"1.0","id":1,"method":"uptime","params":[]}`, `"id":1`},
		{`{"jsonrpc":"2.0","id":"a","method":"session","params":[]}`, `"id":"a"`},
		// A notification gets no reply, so the next one is the batch reply.
		{`{"jsonrpc":"2.0","id":null,"method":"uptime","params":[]}`, ""},
		{`[{"jsonrpc":"2.0","id":2,"method":"uptime","params":[]},` +
			`{"jsonrpc":"2.0","id":"b","method":"session"}]`, `"id":"b"`},
		{`{"jsonrpc":"1.0","id":3,"method":"stop","params":[]}`,
			`limited user not authorized`},
	}
	adminChecks := []exchange{
		{`{"jsonrpc":"1.0","id":1,"method":"uptime","params":[]}`, `"id":1`},
		{`[{"jsonrpc":"2.0","id":"a","method":"uptime","params":[]}]`, `"id":"a"`},
		{`{"jsonrpc":"1.0","id":3,"method":"stop","params":["x"]}`, `"id":3`},
	}
	authCmd := func(login string) string {
		user, pass, _ := strings.Cut(login, ":")
		return `{"jsonrpc":"1.0","id":0,"method":"authenticate","params":["` +
			user + `","` + pass + `"]}`
	}

	for _, test := range []struct {
		name   string
		login  string
		header bool
		checks []exchange
	}{
		{"admin header", testRPCAdminLogin, true, adminChecks},
		{"limited header", testRPCLimitedLogin, true, limitedChecks},
		{"admin authenticate", testRPCAdminLogin, false, adminChecks},
		{"limited authenticate", testRPCLimitedLogin, false, limitedChecks},
	} {
		var hdr http.Header
		if test.header {
			hdr = http.Header{"Authorization": []string{basicAuth(test.login)}}
		}
		conn := dialTestWebsocket(t, addr, hdr)
		if !test.header {
			reply, err := wsRoundTrip(conn, authCmd(test.login))
			if err != nil || !strings.Contains(reply, `"error":null`) {
				t.Fatalf("%s: authenticate reply %s, err %v",
					test.name, reply, err)
			}
		}
		for _, check := range test.checks {
			if check.want == "" {
				err := conn.WriteMessage(websocket.TextMessage, []byte(check.req))
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				continue
			}
			reply, err := wsRoundTrip(conn, check.req)
			if err != nil || !strings.Contains(reply, check.want) {
				t.Errorf("%s: %s: reply %s, err %v", test.name,
					check.req, reply, err)
			}
		}
	}
}

// expectWsClosed asserts the server closes the connection within a few
// seconds without sending a reply.
func expectWsClosed(t *testing.T, conn *websocket.Conn, what string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err == nil {
		t.Fatalf("%s: connection still open, got reply %s", what, msg)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("%s: connection still open after 5s", what)
	}
}

// TestWebsocketPreAuthTimeout ensures an unauthenticated websocket is closed
// once the auth timeout passes.
func TestWebsocketPreAuthTimeout(t *testing.T) {
	_, addr := startTestRPCServer(t)
	conn := dialTestWebsocket(t, addr, nil)
	expectWsClosed(t, conn, "idle unauthenticated client")
}

// TestWebsocketPreAuthMalformed ensures malformed requests from an
// unauthenticated client close the connection instead of being answered.
func TestWebsocketPreAuthMalformed(t *testing.T) {
	_, addr := startTestRPCServer(t)
	for _, req := range []string{
		`{"jsonrpc":"1.0","id":1,"method":"","params":[]}`,
		`{"jsonrpc":"1.0","id":1,"method":"getblockcount","params":{}}`,
		`[{"jsonrpc":"1.0","id":1,"method":"","params":[]}]`,
	} {
		conn := dialTestWebsocket(t, addr, nil)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(req)); err != nil {
			t.Fatal(err)
		}
		expectWsClosed(t, conn, req)
	}
}

// TestWebsocketPreAuthReadLimit ensures an unauthenticated client can't send
// large messages, while an authenticated one can.
func TestWebsocketPreAuthReadLimit(t *testing.T) {
	_, addr := startTestRPCServer(t)
	auth := `{"jsonrpc":"1.0","id":1,"method":"authenticate","params":["admin","adminpass"]}`
	padding := strings.Repeat(" ", 1<<20)

	// The server may close the connection before the write completes.
	conn := dialTestWebsocket(t, addr, nil)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(auth+padding)); err == nil {
		expectWsClosed(t, conn, "large unauthenticated message")
	}

	conn = dialTestWebsocket(t, addr, nil)
	reply, err := wsRoundTrip(conn, auth)
	if err != nil || !strings.Contains(reply, `"error":null`) {
		t.Fatalf("authenticate: reply %s, err %v", reply, err)
	}
	// Past the auth timeout the authenticated client stays connected.
	time.Sleep(1500 * time.Millisecond)
	req := `{"jsonrpc":"1.0","id":2,"method":"getbestblockhash","params":["x"]}`
	reply, err = wsRoundTrip(conn, req+padding)
	if err != nil || !strings.Contains(reply, `"id":2`) {
		t.Fatalf("large authenticated message: reply %s, err %v", reply, err)
	}
}

// wsFrameHeader returns the header of a masked client frame with a zero mask
// key, so the payload that follows is sent as is.
func wsFrameHeader(opcode byte, final bool, length uint64) []byte {
	b0 := opcode
	if final {
		b0 |= 0x80
	}
	var hdr []byte
	switch {
	case length < 126:
		hdr = []byte{b0, 0x80 | byte(length)}
	case length <= 0xffff:
		hdr = binary.BigEndian.AppendUint16([]byte{b0, 0x80 | 126}, uint16(length))
	default:
		hdr = binary.BigEndian.AppendUint64([]byte{b0, 0x80 | 127}, length)
	}
	return append(hdr, 0, 0, 0, 0)
}

// TestWebsocketPreAuthReadLimitFrameLength ensures the pre-auth read limit
// also holds for a fragmented message whose first frame declares a 64-bit
// length with the top bit set, which RFC 6455 forbids.
func TestWebsocketPreAuthReadLimitFrameLength(t *testing.T) {
	_, addr := startTestRPCServer(t)
	auth := `{"jsonrpc":"1.0","id":1,"method":"authenticate","params":["admin","adminpass"]}`
	payload := []byte(auth + strings.Repeat(" ", 1<<20))

	msg := wsFrameHeader(websocket.TextMessage, false, 1<<63)
	for len(payload) > 0 {
		n := min(len(payload), 32*1024)
		msg = append(msg, wsFrameHeader(0, n == len(payload), uint64(n))...)
		msg = append(msg, payload[:n]...)
		payload = payload[n:]
	}

	// The server may close the connection before the write completes.
	conn := dialTestWebsocket(t, addr, nil)
	_, _ = conn.UnderlyingConn().Write(msg)
	expectWsClosed(t, conn, "fragmented message over the pre-auth limit")
}

// TestWebsocketNormalCloseNotLogged ensures a client that closes the
// websocket normally is not logged as a receive error.
func TestWebsocketNormalCloseNotLogged(t *testing.T) {
	_, addr := startTestRPCServer(t)
	logStart := testRPCLog.Len()
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	conn := dialTestWebsocket(t, addr, hdr)
	client := conn.LocalAddr().String()

	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		t.Fatal(err)
	}
	disconnected := "Disconnected websocket client " + client
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(testRPCLog.Since(logStart), disconnected) {
		if time.Now().After(deadline) {
			t.Fatal("server did not disconnect the client")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Contains(testRPCLog.Since(logStart), "Websocket receive error from "+client) {
		t.Fatal("normal close logged as a receive error")
	}
}

// withIndexEchoHandler makes getblockhash answer with the index it was
// passed, for the rest of the test.
func withIndexEchoHandler(t *testing.T) {
	t.Helper()
	saved := rpcHandlers["getblockhash"]
	rpcHandlers["getblockhash"] = func(_ *rpcServer, cmd interface{}, _ <-chan struct{}) (interface{}, error) {
		return cmd.(*btcjson.GetBlockHashCmd).Index, nil
	}
	t.Cleanup(func() { rpcHandlers["getblockhash"] = saved })
}

// TestBatchedRequestParams ensures the params of a batched request reach the
// command as sent, over HTTP and the websocket, as those of a single request
// do: an integer above 2^53 keeps its value, and 0.0 is refused where an
// integer is expected.
func TestBatchedRequestParams(t *testing.T) {
	withIndexEchoHandler(t)
	_, addr := startTestRPCServer(t)
	batch := `[{"jsonrpc":"2.0","id":1,"method":"getblockhash","params":[9007199254740993]},` +
		`{"jsonrpc":"2.0","id":2,"method":"getblockhash","params":[0.0]}]`
	check := func(path string, reply []byte) {
		t.Helper()
		var replies []struct {
			ID     int
			Result json.RawMessage
			Error  *btcjson.RPCError
		}
		if err := json.Unmarshal(reply, &replies); err != nil || len(replies) != 2 {
			t.Fatalf("%s: unexpected reply %s", path, reply)
		}
		for _, r := range replies {
			switch r.ID {
			case 1:
				if string(r.Result) != "9007199254740993" || r.Error != nil {
					t.Errorf("%s: big integer came through as %s, error %v",
						path, r.Result, r.Error)
				}
			case 2:
				if r.Error == nil {
					t.Errorf("%s: 0.0 was taken as an integer: %s", path,
						r.Result)
				}
			}
		}
	}

	req, err := http.NewRequest("POST", "http://"+addr+"/", strings.NewReader(batch))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", basicAuth(testRPCAdminLogin))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	reply, _ := io.ReadAll(resp.Body)
	check("http", reply)

	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	conn := dialTestWebsocket(t, addr, hdr)
	wsReply, err := wsRoundTrip(conn, batch)
	if err != nil {
		t.Fatal(err)
	}
	check("websocket", []byte(wsReply))
}

// smallBufferListener gives each connection it accepts a small send buffer,
// so writes to a client that stops reading stall after kilobytes rather than
// after the megabytes a loopback connection can hold.
type smallBufferListener struct{ net.Listener }

func (l smallBufferListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(4096)
	}
	return conn, err
}

// withSmallSendBuffers is a startTestRPCServer option that gives the server's
// connections small send buffers.
func withSmallSendBuffers(s *rpcServer) {
	s.cfg.Listeners[0] = smallBufferListener{s.cfg.Listeners[0]}
}

// dialStalledWebsocket opens a websocket to the test server with a small
// receive buffer, for a client that stops reading.
func dialStalledWebsocket(t *testing.T, addr, login string) *websocket.Conn {
	t.Helper()
	dialer := &websocket.Dialer{NetDial: func(network, addr string) (net.Conn, error) {
		conn, err := net.Dial(network, addr)
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetReadBuffer(4096)
		}
		return conn, err
	}}
	hdr := http.Header{"Authorization": []string{basicAuth(login)}}
	conn, _, err := dialer.Dial("ws://"+addr+"/ws", hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// setWsLimits sets the websocket write timeout and pending notification bound
// for the rest of the test.  It is called before the server starts, and the
// test ends with closeWsClients, so no client reads them as they are restored.
func setWsLimits(t *testing.T, timeout time.Duration, maxPending int) {
	t.Helper()
	savedTimeout, savedMax := wsWriteTimeout, wsMaxPendingNtfnBytes
	t.Cleanup(func() { wsWriteTimeout, wsMaxPendingNtfnBytes = savedTimeout, savedMax })
	wsWriteTimeout, wsMaxPendingNtfnBytes = timeout, maxPending
}

// numWsClients returns the number of websocket clients the server serves,
// failing the test when the notification manager, which counts them, does not
// answer.
func numWsClients(t *testing.T, s *rpcServer) int {
	t.Helper()
	n := make(chan int, 1)
	go func() { n <- s.ntfnMgr.NumClients() }()
	select {
	case n := <-n:
		return n
	case <-time.After(5 * time.Second):
		t.Fatal("the notification manager did not answer NumClients")
	}
	return 0
}

// waitWsClients waits up to timeout for the server to serve n websocket
// clients.
func waitWsClients(t *testing.T, s *rpcServer, n int, timeout time.Duration, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for got := numWsClients(t, s); got != n; got = numWsClients(t, s) {
		if time.Now().After(deadline) {
			t.Fatalf("%s: %d websocket clients after %v, want %d", what,
				got, timeout, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// closeWsClients closes the connections and waits for the server to let
// their clients go.
func closeWsClients(t *testing.T, s *rpcServer, conns ...*websocket.Conn) {
	t.Helper()
	for _, conn := range conns {
		conn.Close()
	}
	waitWsClients(t, s, 0, 5*time.Second, "closed clients")
}

// wsNotify sends the notification request method on conn and checks the
// reply.
func wsNotify(t *testing.T, conn *websocket.Conn, method string) {
	t.Helper()
	reply, err := wsRoundTrip(conn, `{"jsonrpc":"1.0","id":1,"method":"`+method+`","params":[]}`)
	if err != nil || !strings.Contains(reply, `"error":null`) {
		t.Fatalf("%s: reply %s, err %v", method, reply, err)
	}
}

// notifyTestTx passes the notification manager the nth of a series of new
// memory pool transactions and returns its hash.
func notifyTestTx(s *rpcServer, n int) string {
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{PreviousOutPoint: wire.OutPoint{Index: uint32(n)}})
	tx.AddTxOut(wire.NewTxOut(1000, []byte{txscript.OP_TRUE}, wire.TokenData{}))
	btx := bchutil.NewTx(tx)
	hash := btx.Hash().String()
	s.ntfnMgr.NotifyMempoolTx(btx, true)
	return hash
}

// expectTxNtfn reads the txaccepted notification for hash from conn.
func expectTxNtfn(t *testing.T, conn *websocket.Conn, hash, what string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if !strings.Contains(string(msg), `"method":"txaccepted","params":["`+hash+`"`) {
		t.Fatalf("%s: unexpected message %s", what, msg)
	}
}

// TestWebsocketStalledClient ensures a websocket client that stops reading,
// with its send queue full of replies, holds up neither the notifications of
// another client nor the notification manager, which counts the clients for
// each new connection, and that it is disconnected once a write to it times
// out.
func TestWebsocketStalledClient(t *testing.T) {
	setWsLimits(t, 3*time.Second, wsMaxPendingNtfnBytes)
	s, addr := startTestRPCServer(t, withSmallSendBuffers)
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	reader := dialTestWebsocket(t, addr, hdr)
	stalled := dialStalledWebsocket(t, addr, testRPCAdminLogin)
	wsNotify(t, reader, "notifynewtransactions")
	wsNotify(t, stalled, "notifynewtransactions")

	// The stalled client asks for help more often than its connection
	// holds the answers, and reads none of them, so its replies fill its
	// send queue and its requests wait.  The sleep lets the server get
	// there.
	for i := range 300 {
		req := fmt.Sprintf(`{"jsonrpc":"1.0","id":%d,"method":"help","params":[]}`, i)
		if err := stalled.WriteMessage(websocket.TextMessage, []byte(req)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(300 * time.Millisecond)

	for i := range 5 {
		expectTxNtfn(t, reader, notifyTestTx(s, i), fmt.Sprintf(
			"notification %d to the reader, while a client is stalled", i))
	}
	if n := numWsClients(t, s); n != 2 {
		t.Fatalf("%d websocket clients while one is stalled, want 2", n)
	}
	waitWsClients(t, s, 1, 10*time.Second, "past the write timeout")
	expectTxNtfn(t, reader, notifyTestTx(s, 5), "the reader, past the write timeout")
	closeWsClients(t, s, reader, stalled)
}

// bufferListener gives each connection it accepts a send buffer of size
// bytes.
type bufferListener struct {
	net.Listener
	size int
}

func (l bufferListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(l.size)
	}
	return conn, err
}

// TestWebsocketSlowReaderKeepsConnection ensures a websocket client that
// takes a message more slowly than the write timeout, but steadily, is not
// disconnected: the timeout is for a client that takes nothing, as BCHN's
// -rpcservertimeout is, not for the time a whole message takes.
func TestWebsocketSlowReaderKeepsConnection(t *testing.T) {
	// The connection's buffers hold a small part of the reply, but are not
	// so small that the reader's pace leaves the server waiting long for
	// each window update.
	const bufSize = 16 * 1024
	setWsLimits(t, time.Second, wsMaxPendingNtfnBytes)
	s, addr := startTestRPCServer(t, func(s *rpcServer) {
		s.cfg.Listeners[0] = bufferListener{s.cfg.Listeners[0], bufSize}
	})
	dialer := &websocket.Dialer{NetDial: func(network, addr string) (net.Conn, error) {
		conn, err := net.Dial(network, addr)
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetReadBuffer(bufSize)
		}
		return conn, err
	}}
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	conn, _, err := dialer.Dial("ws://"+addr+"/ws", hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// One batch of help requests gets one reply, of over 512 KiB.
	reqs := make([]string, 200)
	for i := range reqs {
		reqs[i] = fmt.Sprintf(`{"jsonrpc":"1.0","id":%d,"method":"help","params":[]}`, i)
	}
	batch := "[" + strings.Join(reqs, ",") + "]"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(batch)); err != nil {
		t.Fatal(err)
	}

	// The reply is read 4 KiB at a time, at most 200 KB/s, so that it
	// takes over twice the write timeout, while each part the server
	// writes between renewals of its deadline is taken well within it.
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, r, err := conn.NextReader()
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	start := time.Now()
	buf := make([]byte, 4*1024)
	size := 0
	for {
		n, err := r.Read(buf)
		size += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reply after %d bytes and %v: %v", size,
				time.Since(start), err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if took := time.Since(start); size < 512<<10 || took < 2*wsWriteTimeout {
		t.Fatalf("a reply of %d bytes in %v is too small or too fast to "+
			"test the timeout", size, took)
	}

	if reply, err := wsRoundTrip(conn, `{"jsonrpc":"1.0","id":1,"method":"help","params":["help"]}`); err != nil ||
		!strings.Contains(reply, `"error":null`) {

		t.Fatalf("the slow reader after its reply: %s, %v", reply, err)
	}
	if n := numWsClients(t, s); n != 1 {
		t.Fatalf("%d websocket clients, want the slow reader", n)
	}
	closeWsClients(t, s, conn)
}

// TestWebsocketIdleClientKeepsConnection ensures a websocket client that
// received nothing for longer than the write timeout still gets its next
// messages, a notification and a reply larger than the server writes at once:
// the deadline is renewed as each message is written, so an idle client is
// not disconnected.
func TestWebsocketIdleClientKeepsConnection(t *testing.T) {
	setWsLimits(t, 300*time.Millisecond, wsMaxPendingNtfnBytes)
	s, addr := startTestRPCServer(t)
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	reader := dialTestWebsocket(t, addr, hdr)
	wsNotify(t, reader, "notifynewtransactions")

	time.Sleep(2 * wsWriteTimeout)
	expectTxNtfn(t, reader, notifyTestTx(s, 0),
		"the reader, idle for twice the write timeout")

	time.Sleep(2 * wsWriteTimeout)
	reqs := make([]string, 4)
	for i := range reqs {
		reqs[i] = fmt.Sprintf(`{"jsonrpc":"1.0","id":%d,"method":"help","params":[]}`, i)
	}
	reply, err := wsRoundTrip(reader, "["+strings.Join(reqs, ",")+"]")
	if err != nil || len(reply) < 8<<10 {
		t.Fatalf("a reply of %d bytes after idling, err %v; want over 8 KiB",
			len(reply), err)
	}
	if n := numWsClients(t, s); n != 1 {
		t.Fatalf("%d websocket clients, want the idle reader", n)
	}
	closeWsClients(t, s, reader)
}

// TestWebsocketPendingNotificationBound ensures a websocket client that falls
// further behind on its notifications than wsMaxPendingNtfnBytes is
// disconnected, while one that keeps reading is not: neither when each of its
// notifications alone is larger than the bound, nor when they come in bursts
// within the bound that add up to far more.
func TestWebsocketPendingNotificationBound(t *testing.T) {
	for _, test := range []struct {
		name       string
		maxPending int
		burst      int
	}{
		{"notifications larger than the bound", 64, 1},
		{"bursts within the bound", 1024, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			setWsLimits(t, wsWriteTimeout, test.maxPending)
			s, addr := startTestRPCServer(t, withSmallSendBuffers)
			hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
			reader := dialTestWebsocket(t, addr, hdr)
			stalled := dialStalledWebsocket(t, addr, testRPCAdminLogin)
			wsNotify(t, reader, "notifynewtransactions")
			wsNotify(t, stalled, "notifynewtransactions")

			// The stalled client reads none of its notifications,
			// so they wait once its connection is full.
			hashes := make([]string, test.burst)
			for i := 0; i < 5000; i += test.burst {
				for j := range hashes {
					hashes[j] = notifyTestTx(s, i+j)
				}
				for _, hash := range hashes {
					expectTxNtfn(t, reader, hash, "the reader")
				}
			}
			waitWsClients(t, s, 1, 5*time.Second,
				"a client behind on its notifications")
			expectTxNtfn(t, reader, notifyTestTx(s, 5000),
				"the reader, once the other client is gone")
			closeWsClients(t, s, reader, stalled)
		})
	}
}

// wsSession sends the session command on conn and returns an error unless
// the server answers it.
func wsSession(conn *websocket.Conn) error {
	reply, err := wsRoundTrip(conn, `{"jsonrpc":"1.0","id":1,"method":"session","params":[]}`)
	if err != nil {
		return err
	}
	if !strings.Contains(reply, `"sessionid"`) {
		return fmt.Errorf("unexpected reply %s", reply)
	}
	return nil
}

// TestWebsocketPreAuthSlots ensures websocket clients that have not
// authenticated have --rpcmaxwebsockets slots of their own, so they cannot
// take those of the clients that have; that when those slots are full a new
// client disconnects the oldest that has not authenticated, so they cannot
// keep out the clients that authenticate with the authenticate command; and
// that a client that authenticates with the command moves to an authenticated
// slot, or is disconnected when none is free.  The server counts a client
// after the client sees the connection open, so each step waits for a reply
// or a disconnection that shows the server has counted the clients before.
func TestWebsocketPreAuthSlots(t *testing.T) {
	s, addr := startTestRPCServer(t, func(s *rpcServer) {
		s.cfg.RPCAuthTimeout = 60
	})
	limit := cfg.RPCMaxWebsockets
	adminHdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	authenticate := `{"jsonrpc":"1.0","id":1,"method":"authenticate",` +
		`"params":["admin","adminpass"]}`
	var conns []*websocket.Conn
	dial := func(hdr http.Header) *websocket.Conn {
		conn := dialTestWebsocket(t, addr, hdr)
		conns = append(conns, conn)
		return conn
	}

	// Fill the slots of clients that have not authenticated, one at a
	// time, so the server counts them in that order.  A client that
	// authenticated with its request is still served.
	var waiting []*websocket.Conn
	for i := range limit {
		waiting = append(waiting, dial(nil))
		waitWsClients(t, s, i+1, 5*time.Second, "unauthenticated clients")
	}
	if err := wsSession(dial(adminHdr)); err != nil {
		t.Fatalf("authenticated client with all unauthenticated slots "+
			"taken: %v", err)
	}

	// One more client that has not authenticated disconnects the oldest
	// of them, and the next one the next oldest.  The newest can
	// authenticate with the command.
	older := dial(nil)
	expectWsClosed(t, waiting[0], "oldest unauthenticated client")
	newest := dial(nil)
	expectWsClosed(t, waiting[1], "next oldest unauthenticated client")
	reply, err := wsRoundTrip(newest, authenticate)
	if err != nil || !strings.Contains(reply, `"error":null`) {
		t.Fatalf("authenticate: reply %s, err %v", reply, err)
	}

	// With the authenticated slots full, another authenticated client is
	// refused, and so is a client that then authenticates with the
	// command, alone or in a batch.
	for range limit - 2 {
		if err := wsSession(dial(adminHdr)); err != nil {
			t.Fatalf("authenticated client: %v", err)
		}
	}
	expectWsClosed(t, dial(adminHdr), "authenticated client over the limit")
	if reply, err := wsRoundTrip(older, authenticate); err == nil {
		t.Fatalf("authenticate over the limit: reply %s", reply)
	}
	if reply, err := wsRoundTrip(waiting[2], "["+authenticate+"]"); err == nil {
		t.Fatalf("batched authenticate over the limit: reply %s", reply)
	}

	// A client that authenticated with the command keeps its connection
	// as more clients that have not authenticated arrive and take the
	// places of the older ones, the last of which is waiting[limit-1].
	for range limit {
		dial(nil)
	}
	expectWsClosed(t, waiting[limit-1], "unauthenticated client older "+
		"than the newer ones")
	if err := wsSession(newest); err != nil {
		t.Fatalf("client that authenticated with the command: %v", err)
	}

	// The client that authenticated with the command frees an
	// authenticated slot when it leaves.
	newest.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := wsSession(dial(adminHdr))
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("authenticated client after another left: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	closeWsClients(t, s, conns...)
}

// TestWebsocketClientSlots ensures the server counts websocket clients in
// their slots: none is served with --rpcmaxwebsockets 0, as there is no client
// that has not authenticated to disconnect to make room; the oldest client
// that has not authenticated is uncounted as soon as a new one takes its
// place; a client that is no longer waiting to authenticate, as when it was
// disconnected to make room, does not take an authenticated slot; and a client
// that leaves frees its slot.
func TestWebsocketClientSlots(t *testing.T) {
	setTestRPCGlobals()
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })

	cfg = &config{}
	s := &rpcServer{}
	for _, authenticated := range []bool{false, true} {
		if s.addWsClient(&wsClient{authenticated: authenticated}) {
			t.Errorf("client authenticated %v served with no slots",
				authenticated)
		}
	}

	// The oldest is marked disconnected, so it needs no connection to be
	// disconnected.
	cfg = &config{RPCMaxWebsockets: 2}
	oldest, waiting, other := &wsClient{disconnected: true}, &wsClient{},
		&wsClient{}
	for _, client := range []*wsClient{oldest, waiting, other} {
		if !s.addWsClient(client) {
			t.Fatal("client that has not authenticated refused")
		}
	}
	if !slices.Equal(s.wsPreAuthClients, []*wsClient{waiting, other}) {
		t.Fatalf("waiting clients %v, want %v", s.wsPreAuthClients,
			[]*wsClient{waiting, other})
	}
	s.removeWsClient(other)

	cfg = &config{RPCMaxWebsockets: 1}
	if s.authenticateWsClient(other) || s.wsAuthClients != 0 {
		t.Errorf("client not waiting authenticated: %d authenticated",
			s.wsAuthClients)
	}
	if !s.authenticateWsClient(waiting) || s.wsAuthClients != 1 ||
		len(s.wsPreAuthClients) != 0 {

		t.Errorf("waiting client authenticated: %d authenticated, %d "+
			"waiting", s.wsAuthClients, len(s.wsPreAuthClients))
	}
	waiting.authenticated = true

	if !s.addWsClient(other) {
		t.Fatal("client refused a free slot")
	}
	s.removeWsClient(other)
	s.removeWsClient(waiting)
	if s.wsAuthClients != 0 || len(s.wsPreAuthClients) != 0 {
		t.Errorf("clients left: %d authenticated, %d waiting",
			s.wsAuthClients, len(s.wsPreAuthClients))
	}
}

// TestRequestIDEchoedAsSent ensures the server gives a request id back
// exactly as sent, as BCHN does, over HTTP and the websocket, alone and in a
// batch: an integer id above 2^53 is not rounded, and an id that is a bool,
// an object or an array, which JSON-RPC 1.0 allows, gets a reply.
func TestRequestIDEchoedAsSent(t *testing.T) {
	_, addr := startTestRPCServer(t)
	hdr := http.Header{"Authorization": []string{basicAuth(testRPCAdminLogin)}}
	conn := dialTestWebsocket(t, addr, hdr)

	for _, id := range []string{`12345678901234567890`, `true`,
		`{"b":1,"a":[2,"x"]}`, `[1,"x"]`} {

		single := `{"jsonrpc":"1.0","id":` + id + `,"method":"uptime","params":[]}`
		batch := `[{"jsonrpc":"2.0","id":` + id + `,"method":"uptime","params":[]}]`
		want := `"id":` + id + `}`

		for _, req := range []string{single, batch} {
			httpReq, err := http.NewRequest("POST", "http://"+addr+"/",
				strings.NewReader(req))
			if err != nil {
				t.Fatal(err)
			}
			httpReq.Header.Set("Authorization", basicAuth(testRPCAdminLogin))
			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			reply, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || !strings.Contains(string(reply), want) {
				t.Errorf("http %s: reply %s, err %v", req, reply, err)
			}

			wsReply, err := wsRoundTrip(conn, req)
			if err != nil || !strings.Contains(wsReply, want) {
				t.Errorf("websocket %s: reply %s, err %v", req, wsReply,
					err)
			}
		}
	}
}
