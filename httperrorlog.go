// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"strings"

	"github.com/gcash/bchlog"
)

// httpClientErrorPrefixes are the starts of the lines net/http logs for what
// a client does to its own connection: a TLS handshake it drops, as a port
// check does, or an HTTP/2 connection it breaks, times out or ends.
var httpClientErrorPrefixes = []string{
	"http: TLS handshake error",
	"http2: server: error reading preface",
	"http2: server connection error",
	"http2: server closing client connection",
	"http2: received GOAWAY",
	"timeout waiting for SETTINGS frames",
	"timeout waiting for PING response",
}

// httpErrorLog returns the ErrorLog for an HTTP server of bchd, which writes
// what net/http logs through logger instead of to standard error in Go's own
// format.  The lines a client causes on its own connection are logged at
// debug level, as BCHN logs its HTTP server's connection events only in its
// http debug category (httpserver.cpp:257-274); every other line, such as a
// handler's recovered panic or a failed accept, is logged as an error.
func httpErrorLog(logger bchlog.Logger) *log.Logger {
	return log.New(httpErrorWriter{logger: logger}, "", 0)
}

// httpErrorWriter writes each line of an HTTP server's ErrorLog to its
// logger.  log.Logger hands it one whole line per write.
type httpErrorWriter struct {
	logger bchlog.Logger
}

func (w httpErrorWriter) Write(p []byte) (int, error) {
	line := strings.TrimSuffix(string(p), "\n")
	for _, prefix := range httpClientErrorPrefixes {
		if strings.HasPrefix(line, prefix) {
			w.logger.Debug(line)
			return len(p), nil
		}
	}
	w.logger.Error(line)
	return len(p), nil
}
