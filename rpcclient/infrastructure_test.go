// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"errors"
	"io"
	"testing"

	"github.com/gorilla/websocket"
)

// TestShouldLogReadError ensures a normal close by the server or a client
// shutdown is not logged as a websocket read error, while other errors are.
func TestShouldLogReadError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"EOF", io.EOF, false},
		{"normal close", &websocket.CloseError{Code: websocket.CloseNormalClosure}, false},
		{"going away", &websocket.CloseError{Code: websocket.CloseGoingAway}, false},
		{"abnormal close", &websocket.CloseError{Code: websocket.CloseAbnormalClosure}, true},
		{"other error", errors.New("read failed"), true},
	}
	for _, test := range tests {
		c := &Client{shutdown: make(chan struct{})}
		if got := c.shouldLogReadError(test.err); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
		close(c.shutdown)
		if c.shouldLogReadError(test.err) {
			t.Errorf("%s: logged after shutdown", test.name)
		}
	}
}
