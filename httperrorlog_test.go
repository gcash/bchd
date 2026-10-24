// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/tls"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gcash/bchlog"
	"github.com/gcash/bchutil"
)

// stdLogCapture redirects the standard logger, which net/http writes to when
// a server has no ErrorLog, into a buffer for the rest of the test.
func stdLogCapture(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

// dropTLSHandshake connects to addr and closes the connection without a TLS
// handshake, as a port check does.
func dropTLSHandshake(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

// waitForLog waits until buf holds want, failing the test after 5 s.
func waitForLog(t *testing.T, buf *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.Since(0), want) {
		if time.Now().After(deadline) {
			t.Fatalf("log %q, want a line with %q", buf.Since(0), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestGrpcTLSHandshakeErrorLogged ensures that a TLS handshake a client
// drops on the gRPC listener is logged at debug level through the GRPC
// logger, in bchd's format, and not written to standard error by net/http.
func TestGrpcTLSHandshakeErrorLogged(t *testing.T) {
	std := stdLogCapture(t)
	logged := &syncBuffer{}
	logger := bchlog.NewBackend(logged).Logger("GRPC")
	logger.SetLevel(bchlog.LevelDebug)
	saved := grpcLog
	grpcLog = logger
	t.Cleanup(func() { grpcLog = saved })

	addr, _ := startGrpcWriteTest(t, &grpcWriteTestService{}, 30*time.Second)
	dropTLSHandshake(t, addr)
	waitForLog(t, logged, "[DBG] GRPC: http: TLS handshake error from 127.0.0.1:")
	if std.Len() != 0 {
		t.Fatalf("standard logger got %q", std.Since(0))
	}
}

// TestRPCTLSHandshakeErrorLogged ensures the same for the JSON-RPC server,
// through the RPCS logger.
func TestRPCTLSHandshakeErrorLogged(t *testing.T) {
	std := stdLogCapture(t)
	cert, key, err := bchutil.NewTLSCertPair("bchd test",
		time.Now().Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}

	setTestRPCGlobals()
	rpcsLog.SetLevel(bchlog.LevelDebug)
	t.Cleanup(func() { rpcsLog.SetLevel(bchlog.LevelInfo) })
	start := testRPCLog.Len()

	// Serve TLS as the RPC listeners do, by wrapping the test listener.
	_, addr := startTestRPCServer(t, func(s *rpcServer) {
		s.cfg.Listeners[0] = tls.NewListener(s.cfg.Listeners[0],
			&tls.Config{Certificates: []tls.Certificate{keyPair}})
	})
	dropTLSHandshake(t, addr)
	deadline := time.Now().Add(5 * time.Second)
	want := "[DBG] RPCS: http: TLS handshake error from 127.0.0.1:"
	for !strings.Contains(testRPCLog.Since(start), want) {
		if time.Now().After(deadline) {
			t.Fatalf("RPCS log %q, want a line with %q",
				testRPCLog.Since(start), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if std.Len() != 0 {
		t.Fatalf("standard logger got %q", std.Since(0))
	}
}

// TestHTTPErrorLogLevels ensures that httpErrorLog logs the lines a client
// causes on its own connection at debug level and every other line, such as
// a recovered panic or one net/http may add later, as an error, each as one
// line in bchd's format.
func TestHTTPErrorLogLevels(t *testing.T) {
	var logged bytes.Buffer
	logger := bchlog.NewBackend(&logged).Logger("TEST")
	logger.SetLevel(bchlog.LevelDebug)
	errorLog := httpErrorLog(logger)

	tests := []struct {
		line  string
		level string
	}{
		{"http: TLS handshake error from 127.0.0.1:53250: EOF", "DBG"},
		{"http2: server: error reading preface from client 127.0.0.1:1: EOF", "DBG"},
		{"http2: server connection error from 127.0.0.1:1: PROTOCOL_ERROR", "DBG"},
		{"http2: server closing client connection: EOF", "DBG"},
		{"http2: received GOAWAY [FrameHeader GOAWAY len=8], starting graceful shutdown", "DBG"},
		{"timeout waiting for SETTINGS frames from 127.0.0.1:1", "DBG"},
		{"timeout waiting for PING response", "DBG"},
		{"http: panic serving 127.0.0.1:1: boom", "ERR"},
		// A real error that mentions client noise stays an error.
		{"http: panic serving 127.0.0.1:1: timeout waiting for PING response", "ERR"},
		{"http: Accept error: too many open files; retrying in 5ms", "ERR"},
		{"http: superfluous response.WriteHeader call from main.f (x.go:1)", "ERR"},
		{"a line net/http does not log today", "ERR"},
	}
	for _, test := range tests {
		logged.Reset()
		errorLog.Print(test.line)
		got := logged.String()
		want := "[" + test.level + "] TEST: " + test.line + "\n"
		if !strings.HasSuffix(got, want) || strings.Count(got, "\n") != 1 {
			t.Errorf("%q: logged %q, want a line ending in %q", test.line,
				got, want)
		}
	}
}
