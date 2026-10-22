// Copyright (c) 2026 The bchd developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"google.golang.org/grpc"
)

const (
	grpcContentType        = "application/grpc"
	grpcWebContentType     = "application/grpc-web"
	grpcWebTextContentType = "application/grpc-web-text"
)

// grpcWebHandler serves gRPC-Web requests and their CORS preflights by
// translating them for the gRPC server, and passes every other request to
// the gRPC server unchanged.
//
// The wire format is described in doc/PROTOCOL-WEB.md of grpc/grpc.
type grpcWebHandler struct {
	server *grpc.Server

	methodsOnce sync.Once
	methods     map[string]struct{}
}

// newGrpcWebHandler returns a handler that serves gRPC and gRPC-Web requests
// with the passed server.
func newGrpcWebHandler(server *grpc.Server) *grpcWebHandler {
	return &grpcWebHandler{server: server}
}

// ServeHTTP implements http.Handler.
func (h *grpcWebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(
		r.Header.Get("Content-Type"), grpcWebContentType):

		h.serveGrpcWeb(w, r)

	case h.isGrpcWebPreflight(r):
		hdr := w.Header()
		hdr.Set("Access-Control-Allow-Origin", "*")
		hdr.Set("Access-Control-Allow-Methods", http.MethodPost)
		if reqHeaders := r.Header.Values("Access-Control-Request-Headers"); len(reqHeaders) > 0 {
			hdr["Access-Control-Allow-Headers"] = reqHeaders
		}
		hdr.Set("Access-Control-Max-Age", "600")
		hdr.Set("Vary", "Access-Control-Request-Headers")
		w.WriteHeader(http.StatusNoContent)

	default:
		h.server.ServeHTTP(w, r)
	}
}

// isGrpcWebPreflight returns whether r is a CORS preflight for a gRPC-Web
// call of a method the server has.
func (h *grpcWebHandler) isGrpcWebPreflight(r *http.Request) bool {
	if r.Method != http.MethodOptions || !strings.Contains(strings.ToLower(
		r.Header.Get("Access-Control-Request-Headers")), "x-grpc-web") {

		return false
	}

	// Services are all registered before the server starts listening.
	h.methodsOnce.Do(func() {
		h.methods = make(map[string]struct{})
		for name, info := range h.server.GetServiceInfo() {
			for _, m := range info.Methods {
				h.methods["/"+name+"/"+m.Name] = struct{}{}
			}
		}
	})
	_, ok := h.methods[r.URL.Path]
	return ok
}

// serveGrpcWeb serves a gRPC-Web call by passing a gRPC version of the request
// to the server and converting the response back.
func (h *grpcWebHandler) serveGrpcWeb(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	webType := grpcWebContentType
	text := strings.HasPrefix(contentType, grpcWebTextContentType)
	if text {
		webType = grpcWebTextContentType
	}

	// The server only accepts HTTP/2 requests, so present it with one.  The
	// length is that of the HTTP/1.1 body, which differs in text mode.
	req := r.Clone(r.Context())
	req.ProtoMajor, req.ProtoMinor = 2, 0
	req.Header.Set("Content-Type", strings.Replace(contentType, webType,
		grpcContentType, 1))
	req.Header.Del("Content-Length")
	if text {
		req.Body = &base64Body{body: r.Body}
	}

	// Calls carry no cookies or HTTP auth, so any origin may make them.
	w.Header().Set("Access-Control-Allow-Origin", "*")

	resp := &grpcWebResponse{
		w:           w,
		header:      make(http.Header),
		contentType: webType,
	}
	if text {
		resp.enc = base64.NewEncoder(base64.StdEncoding, w)
	}
	h.server.ServeHTTP(resp, req)
	resp.finish()
}

// grpcWebResponse is the http.ResponseWriter handed to the gRPC server.  It
// sends the headers the server sets before its first write as HTTP headers,
// and those it sets afterwards, the trailers, as a final trailer frame.
type grpcWebResponse struct {
	w           http.ResponseWriter
	header      http.Header
	sent        map[string]struct{}
	contentType string
	enc         io.WriteCloser
	wroteHeader bool
}

// Header implements http.ResponseWriter.
func (g *grpcWebResponse) Header() http.Header {
	return g.header
}

// WriteHeader implements http.ResponseWriter.
func (g *grpcWebResponse) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	g.sent = make(map[string]struct{}, len(g.header))

	hdr := g.w.Header()
	exposed := []string{"Grpc-Message", "Grpc-Status"}
	for k, vv := range g.header {
		g.sent[k] = struct{}{}
		if k == "Trailer" {
			continue
		}
		k = http.CanonicalHeaderKey(strings.TrimPrefix(k, http.TrailerPrefix))
		if k == "Content-Type" {
			types := make([]string, len(vv))
			for i, v := range vv {
				types[i] = strings.Replace(v, grpcContentType, g.contentType, 1)
			}
			vv = types
		}
		hdr[k] = vv
		if len(vv) > 0 && k != "Grpc-Message" && k != "Grpc-Status" {
			exposed = append(exposed, k)
		}
	}

	// Browsers hide response headers from scripts unless they are listed.
	sort.Strings(exposed)
	hdr.Set("Access-Control-Expose-Headers", strings.Join(exposed, ", "))
	g.w.WriteHeader(code)
}

// Write implements http.ResponseWriter.
func (g *grpcWebResponse) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.enc != nil {
		return g.enc.Write(b)
	}
	return g.w.Write(b)
}

// Flush implements http.Flusher.  Before the first write it does nothing, so
// that a status set right after it still goes out in the headers.
func (g *grpcWebResponse) Flush() {
	if g.wroteHeader {
		g.flush()
	}
}

// flush sends everything written so far.  In text mode this ends the base64
// chunk, which the protocol allows to be followed by more chunks.
func (g *grpcWebResponse) flush() {
	if g.enc != nil {
		g.enc.Close()
		g.enc = base64.NewEncoder(base64.StdEncoding, g.w)
	}
	if f, ok := g.w.(http.Flusher); ok {
		f.Flush()
	}
}

// finish ends the response once the server has returned.  When nothing was
// written, the status is still in the header map and is sent as headers.
func (g *grpcWebResponse) finish() {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
		g.flush()
		return
	}

	trailer := make(http.Header)
	for k, vv := range g.header {
		if _, ok := g.sent[k]; ok || k == "Trailer" {
			continue
		}
		k = strings.ToLower(strings.TrimPrefix(k, http.TrailerPrefix))
		trailer[k] = append(trailer[k], vv...)
	}
	var buf bytes.Buffer
	_ = trailer.Write(&buf)

	frame := make([]byte, 5, 5+buf.Len())
	frame[0] = 0x80
	binary.BigEndian.PutUint32(frame[1:], uint32(buf.Len()))
	_, _ = g.Write(append(frame, buf.Bytes()...))
	g.flush()
}

// base64Body decodes a grpc-web-text request body.  The body may be made of
// several base64 chunks, each with its own padding.
type base64Body struct {
	body io.ReadCloser
	in   [4096]byte
	n    int
	dec  [3072]byte
	out  []byte
	err  error
}

// Read implements io.Reader.
func (b *base64Body) Read(p []byte) (int, error) {
	for len(b.out) == 0 && b.err == nil {
		b.fill()
	}
	if len(b.out) == 0 {
		return 0, b.err
	}
	n := copy(p, b.out)
	b.out = b.out[n:]
	return n, nil
}

// fill reads more of the body and decodes all complete 4-byte groups.
func (b *base64Body) fill() {
	n, err := b.body.Read(b.in[b.n:])

	// Drop line breaks, as base64.NewDecoder does.
	in := b.in[:b.n]
	for _, c := range b.in[b.n : b.n+n] {
		if c != '\r' && c != '\n' {
			in = append(in, c)
		}
	}

	whole := len(in) &^ 3
	m, decErr := decodeBase64Chunks(b.dec[:], in[:whole])
	b.out = b.dec[:m]
	b.n = copy(b.in[:], in[whole:])

	switch {
	case decErr != nil:
		b.err = decErr
	case err == io.EOF && b.n != 0:
		b.err = io.ErrUnexpectedEOF
	default:
		b.err = err
	}
}

// Close implements io.Closer.
func (b *base64Body) Close() error {
	return b.body.Close()
}

// decodeBase64Chunks decodes src, whose length is a multiple of 4, into dst.
// A group that ends in padding ends a chunk.
func decodeBase64Chunks(dst, src []byte) (int, error) {
	var n int
	for len(src) > 0 {
		end := len(src)
		if i := bytes.IndexByte(src, '='); i >= 0 {
			end = i&^3 + 4
		}
		m, err := base64.StdEncoding.Decode(dst[n:], src[:end])
		n += m
		if err != nil {
			return n, err
		}
		src = src[end:]
	}
	return n, nil
}
