// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcServerMetrics holds the Prometheus metrics of the gRPC server.  The
// names, help strings and labels match those of go-grpc-prometheus, so
// existing dashboards keep working.
type grpcServerMetrics struct {
	started  *prometheus.CounterVec
	handled  *prometheus.CounterVec
	received *prometheus.CounterVec
	sent     *prometheus.CounterVec
	seconds  *prometheus.HistogramVec
}

// grpcMetrics is nil unless metrics are enabled.
var grpcMetrics *grpcServerMetrics

var grpcMethodLabels = []string{"grpc_type", "grpc_service", "grpc_method"}

// grpcCodes lists the codes whose handled series are created up front.
var grpcCodes = []codes.Code{
	codes.OK, codes.Canceled, codes.Unknown, codes.InvalidArgument,
	codes.DeadlineExceeded, codes.NotFound, codes.AlreadyExists,
	codes.PermissionDenied, codes.Unauthenticated, codes.ResourceExhausted,
	codes.FailedPrecondition, codes.Aborted, codes.OutOfRange,
	codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss,
}

func newGrpcServerMetrics() *grpcServerMetrics {
	counter := func(name, help string, labels []string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: name, Help: help}, labels)
	}
	return &grpcServerMetrics{
		started: counter("grpc_server_started_total",
			"Total number of RPCs started on the server.", grpcMethodLabels),
		handled: counter("grpc_server_handled_total",
			"Total number of RPCs completed on the server, regardless of success or failure.",
			append(grpcMethodLabels[:3:3], "grpc_code")),
		received: counter("grpc_server_msg_received_total",
			"Total number of RPC stream messages received on the server.", grpcMethodLabels),
		sent: counter("grpc_server_msg_sent_total",
			"Total number of gRPC stream messages sent by the server.", grpcMethodLabels),
		seconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_server_handling_seconds",
			Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
			Buckets: prometheus.DefBuckets,
		}, grpcMethodLabels),
	}
}

// register adds the metrics to reg and creates the series of every method of
// server, so they are exported with zero values before the first call.
func (m *grpcServerMetrics) register(reg prometheus.Registerer, server *grpc.Server) error {
	for _, c := range []prometheus.Collector{m.started, m.handled,
		m.received, m.sent, m.seconds} {

		if err := reg.Register(c); err != nil {
			return err
		}
	}
	for service, info := range server.GetServiceInfo() {
		for _, method := range info.Methods {
			typ := grpcMethodType(method.IsClientStream, method.IsServerStream)
			m.started.WithLabelValues(typ, service, method.Name)
			m.received.WithLabelValues(typ, service, method.Name)
			m.sent.WithLabelValues(typ, service, method.Name)
			m.seconds.WithLabelValues(typ, service, method.Name)
			for _, code := range grpcCodes {
				m.handled.WithLabelValues(typ, service, method.Name, code.String())
			}
		}
	}
	return nil
}

func grpcMethodType(clientStream, serverStream bool) string {
	switch {
	case clientStream && serverStream:
		return "bidi_stream"
	case clientStream:
		return "client_stream"
	case serverStream:
		return "server_stream"
	}
	return "unary"
}

// grpcCallMetrics records the metrics of one call.
type grpcCallMetrics struct {
	m      *grpcServerMetrics
	labels []string
	start  time.Time
}

func (m *grpcServerMetrics) startCall(typ, fullMethod string) *grpcCallMetrics {
	service, method := "unknown", "unknown"
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	if i := strings.Index(fullMethod, "/"); i >= 0 {
		service, method = fullMethod[:i], fullMethod[i+1:]
	}
	c := &grpcCallMetrics{m: m, labels: []string{typ, service, method},
		start: time.Now()}
	m.started.WithLabelValues(c.labels...).Inc()
	return c
}

func (c *grpcCallMetrics) handled(err error) {
	code := status.Code(err).String()
	c.m.handled.WithLabelValues(append(c.labels[:3:3], code)...).Inc()
	c.m.seconds.WithLabelValues(c.labels...).Observe(time.Since(c.start).Seconds())
}

func (m *grpcServerMetrics) interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	c := m.startCall("unary", info.FullMethod)
	m.received.WithLabelValues(c.labels...).Inc()
	resp, err := handler(ctx, req)
	c.handled(err)
	if err == nil {
		m.sent.WithLabelValues(c.labels...).Inc()
	}
	return resp, err
}

func (m *grpcServerMetrics) interceptStreaming(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// A stream with neither flag counts as bidi_stream, as before.
	typ := "bidi_stream"
	if info.IsClientStream != info.IsServerStream {
		typ = grpcMethodType(info.IsClientStream, info.IsServerStream)
	}
	c := m.startCall(typ, info.FullMethod)
	err := handler(srv, &grpcMetricsStream{ss, c})
	c.handled(err)
	return err
}

// grpcMetricsStream counts the messages of a stream.
type grpcMetricsStream struct {
	grpc.ServerStream
	c *grpcCallMetrics
}

func (s *grpcMetricsStream) SendMsg(msg interface{}) error {
	err := s.ServerStream.SendMsg(msg)
	if err == nil {
		s.c.m.sent.WithLabelValues(s.c.labels...).Inc()
	}
	return err
}

func (s *grpcMetricsStream) RecvMsg(msg interface{}) error {
	err := s.ServerStream.RecvMsg(msg)
	if err == nil {
		s.c.m.received.WithLabelValues(s.c.labels...).Inc()
	}
	return err
}
