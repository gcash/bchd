// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcMetricsTestStream is a stream that receives recv messages and accepts
// every message sent.
type grpcMetricsTestStream struct {
	grpcTestStream
	recv int
}

func (s *grpcMetricsTestStream) SendMsg(interface{}) error { return nil }

func (s *grpcMetricsTestStream) RecvMsg(interface{}) error {
	if s.recv == 0 {
		return io.EOF
	}
	s.recv--
	return nil
}

// grpcMetricsTestServer returns a server with a unary, a server streaming, a
// client streaming and a bidirectional method on a service that is always
// ready.
func grpcMetricsTestServer() *grpc.Server {
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "grpc.reflection.v1alpha.ServerReflection",
		HandlerType: (*interface{})(nil),
		Methods:     []grpc.MethodDesc{{MethodName: "Check"}},
		Streams: []grpc.StreamDesc{
			{StreamName: "Watch", ServerStreams: true},
			{StreamName: "Upload", ClientStreams: true},
			{StreamName: "Chat", ServerStreams: true, ClientStreams: true},
		},
	}, struct{}{})
	return server
}

// runGrpcMetricsCalls makes unary and streaming calls that succeed, fail,
// panic, or are refused for a wrong token or by a full call limiter.
func runGrpcMetricsCalls() {
	const service = "/grpc.reflection.v1alpha.ServerReflection/"
	ctx := tokenContext("tok")
	unary := func(ctx context.Context, method string, err error) {
		info := &grpc.UnaryServerInfo{FullMethod: service + method}
		_, _ = interceptUnary(ctx, nil, info,
			func(context.Context, interface{}) (interface{}, error) {
				if method == "Panic" {
					panic("handler failure")
				}
				return "ok", err
			})
	}
	unary(ctx, "Check", nil)
	unary(ctx, "Check", nil)
	unary(ctx, "Check", status.Error(codes.NotFound, "x"))
	unary(ctx, "Check", errors.New("x"))
	unary(tokenContext("wrong"), "Check", nil)
	unary(ctx, "Panic", nil)

	// The call limiter runs inside the metrics, as grpcServerOptions chains
	// it, so a call it refuses is counted.
	full := newGrpcCallLimiter(1)
	full.places <- struct{}{}
	full.waiting.Store(full.maxWaiting)
	info := &grpc.UnaryServerInfo{FullMethod: service + "Check"}
	_, _ = interceptUnary(ctx, nil, info,
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return full.intercept(ctx, req, info,
				func(context.Context, interface{}) (interface{}, error) {
					return "ok", nil
				})
		})

	stream := func(method string, client, server bool, recv, send int, err error) {
		info := &grpc.StreamServerInfo{FullMethod: service + method,
			IsClientStream: client, IsServerStream: server}
		ss := &grpcMetricsTestStream{grpcTestStream{ctx: ctx}, recv}
		_ = interceptStreaming(nil, ss, info,
			func(_ interface{}, ss grpc.ServerStream) error {
				for {
					if ss.RecvMsg(nil) != nil {
						break
					}
				}
				for i := 0; i < send; i++ {
					_ = ss.SendMsg(nil)
				}
				return err
			})
	}
	stream("Watch", false, true, 1, 3, nil)
	stream("Upload", true, false, 4, 0, status.Error(codes.InvalidArgument, "x"))
	stream("Chat", true, true, 2, 2, status.Error(codes.Canceled, "x"))
}

// grpcMetricsText returns the gRPC metrics of g in the text format, with
// only the sample count of histograms, since their buckets and sums depend on
// timing.
func grpcMetricsText(t *testing.T, g prometheus.Gatherer) string {
	t.Helper()
	families, err := g.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, mf := range families {
		name := mf.GetName()
		if !strings.HasPrefix(name, "grpc_") {
			continue
		}
		typ := strings.ToLower(mf.GetType().String())
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, mf.GetHelp(), name, typ)
		for _, m := range mf.GetMetric() {
			var labels []string
			for _, l := range m.GetLabel() {
				labels = append(labels, fmt.Sprintf("%s=%q", l.GetName(), l.GetValue()))
			}
			sample, value := name, m.GetCounter().GetValue()
			if m.GetHistogram() != nil {
				sample, value = name+"_count", float64(m.GetHistogram().GetSampleCount())
			}
			fmt.Fprintf(&b, "%s{%s} %v\n", sample, strings.Join(labels, ","), value)
		}
	}
	return b.String()
}

// TestGrpcMetrics ensures the gRPC metrics keep the names, help strings and
// labels of go-grpc-prometheus, are created for every method up front, and
// count only calls that pass authentication, including those the call limiter
// refuses.
func TestGrpcMetrics(t *testing.T) {
	setGrpcTestConfig(t, "tok", true)
	reg := prometheus.NewRegistry()
	if err := grpcMetrics.register(reg, grpcMetricsTestServer()); err != nil {
		t.Fatal(err)
	}
	runGrpcMetricsCalls()
	text := grpcMetricsText(t, reg)

	const labels = `grpc_service="grpc.reflection.v1alpha.ServerReflection",`
	for _, want := range []string{
		"# HELP grpc_server_started_total Total number of RPCs started on the server.",
		"# TYPE grpc_server_started_total counter",
		"# HELP grpc_server_handled_total Total number of RPCs completed on the server, regardless of success or failure.",
		"# TYPE grpc_server_handled_total counter",
		"# HELP grpc_server_msg_received_total Total number of RPC stream messages received on the server.",
		"# TYPE grpc_server_msg_received_total counter",
		"# HELP grpc_server_msg_sent_total Total number of gRPC stream messages sent by the server.",
		"# TYPE grpc_server_msg_sent_total counter",
		"# HELP grpc_server_handling_seconds Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
		"# TYPE grpc_server_handling_seconds histogram",

		`grpc_server_started_total{grpc_method="Check",` + labels + `grpc_type="unary"} 5`,
		`grpc_server_handled_total{grpc_code="OK",grpc_method="Check",` + labels + `grpc_type="unary"} 2`,
		`grpc_server_handled_total{grpc_code="NotFound",grpc_method="Check",` + labels + `grpc_type="unary"} 1`,
		`grpc_server_handled_total{grpc_code="Unknown",grpc_method="Check",` + labels + `grpc_type="unary"} 1`,
		`grpc_server_handled_total{grpc_code="ResourceExhausted",grpc_method="Check",` + labels + `grpc_type="unary"} 1`,
		`grpc_server_msg_received_total{grpc_method="Check",` + labels + `grpc_type="unary"} 5`,
		`grpc_server_msg_sent_total{grpc_method="Check",` + labels + `grpc_type="unary"} 2`,
		`grpc_server_handling_seconds_count{grpc_method="Check",` + labels + `grpc_type="unary"} 5`,

		// A panicking call starts but is not handled.
		`grpc_server_started_total{grpc_method="Panic",` + labels + `grpc_type="unary"} 1`,
		`grpc_server_msg_received_total{grpc_method="Panic",` + labels + `grpc_type="unary"} 1`,

		`grpc_server_started_total{grpc_method="Watch",` + labels + `grpc_type="server_stream"} 1`,
		`grpc_server_handled_total{grpc_code="OK",grpc_method="Watch",` + labels + `grpc_type="server_stream"} 1`,
		`grpc_server_msg_received_total{grpc_method="Watch",` + labels + `grpc_type="server_stream"} 1`,
		`grpc_server_msg_sent_total{grpc_method="Watch",` + labels + `grpc_type="server_stream"} 3`,
		`grpc_server_handled_total{grpc_code="InvalidArgument",grpc_method="Upload",` + labels + `grpc_type="client_stream"} 1`,
		`grpc_server_msg_received_total{grpc_method="Upload",` + labels + `grpc_type="client_stream"} 4`,
		`grpc_server_msg_sent_total{grpc_method="Upload",` + labels + `grpc_type="client_stream"} 0`,
		`grpc_server_handled_total{grpc_code="Canceled",grpc_method="Chat",` + labels + `grpc_type="bidi_stream"} 1`,
		`grpc_server_msg_received_total{grpc_method="Chat",` + labels + `grpc_type="bidi_stream"} 2`,
		`grpc_server_msg_sent_total{grpc_method="Chat",` + labels + `grpc_type="bidi_stream"} 2`,
		`grpc_server_handling_seconds_count{grpc_method="Chat",` + labels + `grpc_type="bidi_stream"} 1`,
	} {
		if !strings.Contains(text, want+"\n") {
			t.Errorf("missing %q", want)
		}
	}

	// Every registered method has a handled series for each of the 17
	// codes, and the panicking call has none.
	if n := strings.Count(text, "grpc_server_handled_total{"); n != 4*17 {
		t.Errorf("%d handled series, want %d", n, 4*17)
	}
}
