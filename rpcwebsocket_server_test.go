// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/websocket"
	"github.com/gcash/bchd/btcjson"
	"github.com/gcash/bchd/wire"
	"github.com/gcash/bchlog"
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

// startTestRPCServer starts the RPC HTTP and websocket server on a loopback
// listener with an admin and a limited user.  It returns the listen address.
func startTestRPCServer(t *testing.T) (*rpcServer, string) {
	t.Helper()

	// Client goroutines may outlive Stop, so the globals are set once and
	// not restored.
	testRPCGlobalsOnce.Do(func() {
		cfg = &config{
			RPCMaxClients:        10,
			RPCMaxWebsockets:     25,
			RPCMaxConcurrentReqs: 20,
			RPCAuthTimeout:       1,
		}
		rpcsLog = bchlog.Disabled
	})

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
