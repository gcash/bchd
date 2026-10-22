// Copyright (c) 2026 The btcsuite developers
// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package socks

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testTimeout bounds how long a test waits for the test proxy.
const testTimeout = 5 * time.Second

// proxyRequest is what a client sent to the test proxy.
type proxyRequest struct {
	// user and pass are the credentials the client authenticated with, if
	// any.
	user, pass string

	// addrType is the SOCKS address type of the destination.
	addrType byte

	// host and port are the requested destination.
	host string
	port int
}

// successReply is the reply of a SOCKS5 proxy that connected to the
// destination and reports 0.0.0.0:0 as the bound address, as Tor does.
var successReply = []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}

// serveProxy runs handle for every connection to a test proxy and returns the
// proxy address.
func serveProxy(t *testing.T, handle func(conn net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				handle(conn)
			}()
		}
	}()

	return ln.Addr().String()
}

// startProxy runs a minimal SOCKS5 proxy that sends reply to every connect
// request, then keeps the connection open.  It returns the proxy address and
// a channel that receives each request.
func startProxy(t *testing.T, reply []byte) (string, chan proxyRequest) {
	t.Helper()

	requests := make(chan proxyRequest, 10)
	addr := serveProxy(t, func(conn net.Conn) {
		req, err := readRequest(conn)
		if err != nil {
			return
		}
		requests <- req
		if _, err := conn.Write(reply); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, conn)
	})

	return addr, requests
}

// readRequest reads the method negotiation, the optional username/password
// authentication and the connect request of a SOCKS5 client.
func readRequest(conn net.Conn) (proxyRequest, error) {
	var req proxyRequest

	// Greeting: version, number of methods, methods.
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return req, err
	}
	methods := make([]byte, header[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return req, err
	}

	// Pick username/password authentication when offered.
	if !bytes.Contains(methods, []byte{2}) {
		if _, err := conn.Write([]byte{5, 0}); err != nil {
			return req, err
		}
	} else {
		if _, err := conn.Write([]byte{5, 2}); err != nil {
			return req, err
		}
		user, pass, err := readCredentials(conn)
		if err != nil {
			return req, err
		}
		req.user, req.pass = user, pass
		if _, err := conn.Write([]byte{1, 0}); err != nil {
			return req, err
		}
	}

	// Request: version, command, reserved, address type, address, port.
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return req, err
	}
	req.addrType = head[3]

	var addr []byte
	switch req.addrType {
	case 1:
		addr = make([]byte, net.IPv4len)
	case 4:
		addr = make([]byte, net.IPv6len)
	case 3:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return req, err
		}
		addr = make([]byte, length[0])
	}
	if _, err := io.ReadFull(conn, addr); err != nil {
		return req, err
	}
	if req.addrType == 3 {
		req.host = string(addr)
	} else {
		req.host = net.IP(addr).String()
	}

	port := make([]byte, 2)
	if _, err := io.ReadFull(conn, port); err != nil {
		return req, err
	}
	req.port = int(binary.BigEndian.Uint16(port))

	return req, nil
}

// readCredentials reads a username/password authentication request.
func readCredentials(conn net.Conn) (string, string, error) {
	var b [1]byte
	field := func() (string, error) {
		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return "", err
		}
		value := make([]byte, b[0])
		_, err := io.ReadFull(conn, value)

		return string(value), err
	}

	// Version of the subnegotiation.
	if _, err := io.ReadFull(conn, b[:]); err != nil {
		return "", "", err
	}
	user, err := field()
	if err != nil {
		return "", "", err
	}
	pass, err := field()

	return user, pass, err
}

// receive returns the next request the test proxy received.
func receive(t *testing.T, requests chan proxyRequest) proxyRequest {
	t.Helper()

	select {
	case req := <-requests:
		return req

	case <-time.After(testTimeout):
		t.Fatal("the proxy received no request")
		return proxyRequest{}
	}
}

// dial dials addr through p and closes the connection when the test ends.
func dial(t *testing.T, p *Proxy, addr string) net.Conn {
	t.Helper()

	conn, err := p.DialTimeout("tcp", addr, testTimeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

// TestDialAddresses checks that a connection reports the destination as its
// remote address and the address bound by the proxy as its local address.
func TestDialAddresses(t *testing.T) {
	t.Parallel()

	proxyAddr, requests := startProxy(t, successReply)
	conn := dial(t, &Proxy{Addr: proxyAddr}, "1.2.3.4:8333")

	req := receive(t, requests)
	if req.addrType != 1 || req.host != "1.2.3.4" || req.port != 8333 {
		t.Fatalf("unexpected request %+v", req)
	}
	if req.user != "" {
		t.Fatalf("unexpected user %q", req.user)
	}

	want := &ProxiedAddr{Net: "tcp", Host: "1.2.3.4", Port: 8333}
	if !reflect.DeepEqual(conn.RemoteAddr(), want) {
		t.Fatalf("RemoteAddr: got %v, want %v", conn.RemoteAddr(), want)
	}
	if got := conn.LocalAddr().String(); got != "0.0.0.0:0" {
		t.Fatalf("LocalAddr: got %q, want 0.0.0.0:0", got)
	}
}

// TestDialBoundAddr checks that the IPv4 and IPv6 addresses the proxy bound
// are reported as the local address.
func TestDialBoundAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reply []byte
		want  string
	}{{
		reply: []byte{5, 0, 0, 1, 10, 0, 0, 1, 0x20, 0x8d},
		want:  "10.0.0.1:8333",
	}, {
		reply: append(append([]byte{5, 0, 0, 4},
			net.ParseIP("2001:db8::1")...), 0x20, 0x8d),
		want: "[2001:db8::1]:8333",
	}}

	for _, test := range tests {
		proxyAddr, _ := startProxy(t, test.reply)
		conn := dial(t, &Proxy{Addr: proxyAddr}, "1.2.3.4:8333")
		if got := conn.LocalAddr().String(); got != test.want {
			t.Fatalf("LocalAddr: got %q, want %q", got, test.want)
		}
	}
}

// TestDialHostname checks that a hostname is sent to the proxy unresolved and
// reported as the remote address, also with the "onion" network, which bchd
// uses to dial onion addresses.
func TestDialHostname(t *testing.T) {
	t.Parallel()

	host := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	addr := net.JoinHostPort(host, "8333")

	for _, network := range []string{"tcp", "onion"} {
		proxyAddr, requests := startProxy(t, successReply)
		p := &Proxy{Addr: proxyAddr}
		conn, err := p.DialTimeout(network, addr, testTimeout)
		if err != nil {
			t.Fatalf("%s: %v", network, err)
		}
		t.Cleanup(func() {
			_ = conn.Close()
		})

		req := receive(t, requests)
		if req.addrType != 3 || req.host != host {
			t.Fatalf("%s: unexpected request %+v", network, req)
		}
		want := &ProxiedAddr{Net: network, Host: host, Port: 8333}
		if !reflect.DeepEqual(conn.RemoteAddr(), want) {
			t.Fatalf("%s: RemoteAddr: got %v, want %v", network,
				conn.RemoteAddr(), want)
		}
	}
}

// TestDialCredentials checks that the configured credentials are sent to the
// proxy.
func TestDialCredentials(t *testing.T) {
	t.Parallel()

	proxyAddr, requests := startProxy(t, successReply)
	p := &Proxy{Addr: proxyAddr, Username: "user", Password: "pass"}
	dial(t, p, "1.2.3.4:8333")

	req := receive(t, requests)
	if req.user != "user" || req.pass != "pass" {
		t.Fatalf("unexpected credentials %q %q", req.user, req.pass)
	}
}

// TestDialTorIsolation checks that with Tor stream isolation, every
// connection uses new random credentials instead of the configured ones.
func TestDialTorIsolation(t *testing.T) {
	t.Parallel()

	proxyAddr, requests := startProxy(t, successReply)
	p := &Proxy{
		Addr:         proxyAddr,
		Username:     "user",
		Password:     "pass",
		TorIsolation: true,
	}

	dial(t, p, "1.2.3.4:8333")
	first := receive(t, requests)
	dial(t, p, "1.2.3.4:8333")
	second := receive(t, requests)

	for _, req := range []proxyRequest{first, second} {
		if len(req.user) != 16 || len(req.pass) != 16 {
			t.Fatalf("unexpected credentials %q %q", req.user, req.pass)
		}
		if req.user == "user" {
			t.Fatal("configured username sent with Tor isolation")
		}
	}
	if first.user == second.user || first.pass == second.pass {
		t.Fatal("credentials reused across connections")
	}
}

// TestDialLongBoundHostname checks that a long hostname as the bound address
// in the proxy's reply is handled.
func TestDialLongBoundHostname(t *testing.T) {
	t.Parallel()

	name := strings.Repeat("a", 255)
	reply := append([]byte{5, 0, 0, 3, byte(len(name))}, name...)
	reply = append(reply, 0x20, 0x8d)

	proxyAddr, _ := startProxy(t, reply)
	conn := dial(t, &Proxy{Addr: proxyAddr}, "1.2.3.4:8333")

	want := net.JoinHostPort(name, strconv.Itoa(8333))
	if got := conn.LocalAddr().String(); got != want {
		t.Fatalf("LocalAddr: got %q, want %q", got, want)
	}
}

// TestDialMalformedReply checks that a malformed or failed connect reply from
// the proxy returns an error.
func TestDialMalformedReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reply []byte
	}{
		{"wrong version", []byte{4, 0, 0, 1, 0, 0, 0, 0, 0, 0}},
		{"failure status", []byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0}},
		{"unknown address type", []byte{5, 0, 0, 9, 0, 0, 0, 0, 0, 0}},
		{"truncated header", []byte{5, 0}},
		{"truncated IPv4", []byte{5, 0, 0, 1, 10, 0}},
		{"truncated IPv6", []byte{5, 0, 0, 4, 0x20, 0x01, 0x0d}},
		{"truncated hostname", []byte{5, 0, 0, 3, 255, 'a', 'b'}},
		{"missing port", []byte{5, 0, 0, 1, 10, 0, 0, 1}},
	}

	for _, test := range tests {
		// The proxy closes the connection after its reply, so a short
		// reply ends in EOF rather than in the timeout.
		proxyAddr := serveProxy(t, func(conn net.Conn) {
			if _, err := readRequest(conn); err != nil {
				return
			}
			_, _ = conn.Write(test.reply)
		})

		p := &Proxy{Addr: proxyAddr}
		conn, err := p.DialTimeout("tcp", "1.2.3.4:8333", testTimeout)
		if err == nil {
			conn.Close()
			t.Fatalf("%s: dial succeeded", test.name)
		}
	}
}

// TestDialMalformedMethodReply checks that a malformed or refused method
// selection from the proxy returns an error.
func TestDialMalformedMethodReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reply []byte
	}{
		{"wrong version", []byte{4, 0}},
		{"no acceptable method", []byte{5, 0xff}},
		{"truncated", []byte{5}},
	}

	for _, test := range tests {
		proxyAddr := serveProxy(t, func(conn net.Conn) {
			header := make([]byte, 2)
			if _, err := io.ReadFull(conn, header); err != nil {
				return
			}
			methods := make([]byte, header[1])
			if _, err := io.ReadFull(conn, methods); err != nil {
				return
			}
			_, _ = conn.Write(test.reply)
		})

		p := &Proxy{Addr: proxyAddr}
		conn, err := p.DialTimeout("tcp", "1.2.3.4:8333", testTimeout)
		if err == nil {
			conn.Close()
			t.Fatalf("%s: dial succeeded", test.name)
		}
	}
}

// TestDialTimeout checks that the timeout covers the SOCKS handshake, so a
// proxy that never answers the greeting or the connect request can't hang
// the dial.
func TestDialTimeout(t *testing.T) {
	t.Parallel()

	silent := serveProxy(t, func(conn net.Conn) {
		_, _ = io.Copy(io.Discard, conn)
	})
	noReply, _ := startProxy(t, nil)

	for _, proxyAddr := range []string{silent, noReply} {
		p := &Proxy{Addr: proxyAddr}
		start := time.Now()
		_, err := p.DialTimeout("tcp", "1.2.3.4:8333",
			100*time.Millisecond)
		if err == nil {
			t.Fatal("dial succeeded")
		}
		if elapsed := time.Since(start); elapsed >= testTimeout {
			t.Fatalf("dial took %v", elapsed)
		}
	}
}

// TestDialContextDeadline checks that the context's deadline covers the
// SOCKS handshake.
func TestDialContextDeadline(t *testing.T) {
	t.Parallel()

	proxyAddr, _ := startProxy(t, nil)
	p := &Proxy{Addr: proxyAddr}

	ctx, cancel := context.WithTimeout(context.Background(),
		100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := p.DialContext(ctx, "tcp", "1.2.3.4:8333")
	if err == nil {
		t.Fatal("dial succeeded")
	}
	if elapsed := time.Since(start); elapsed >= testTimeout {
		t.Fatalf("dial took %v", elapsed)
	}
}

// TestDialDeadlineCleared checks that the dial timeout no longer applies to a
// connection once the handshake is done.
func TestDialDeadlineCleared(t *testing.T) {
	t.Parallel()

	const timeout = 100 * time.Millisecond
	proxyAddr := serveProxy(t, func(conn net.Conn) {
		if _, err := readRequest(conn); err != nil {
			return
		}
		if _, err := conn.Write(successReply); err != nil {
			return
		}
		time.Sleep(3 * timeout)
		if _, err := conn.Write([]byte("hi")); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, conn)
	})

	p := &Proxy{Addr: proxyAddr}
	conn, err := p.DialTimeout("tcp", "1.2.3.4:8333", timeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	b := make([]byte, 2)
	if _, err := io.ReadFull(conn, b); err != nil {
		t.Fatalf("read after the dial timeout: %v", err)
	}
}
