// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/websocket"
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
