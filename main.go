package main

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// a shameless copy of consts from net/http package
// Common HTTP methods.
//
// Unless otherwise noted, these are defined in RFC 7231 section 4.3.
//const (
//	MethodGet     = "GET"
//	MethodHead    = "HEAD"
//	MethodPost    = "POST"
//	MethodPut     = "PUT"
//	MethodPatch   = "PATCH" // RFC 5789
//	MethodDelete  = "DELETE"
//	MethodConnect = "CONNECT"
//	MethodOptions = "OPTIONS"
//	MethodTrace   = "TRACE"
//)

type Request struct {
	Method   string
	URL      *url.URL
	Proto    string
	ProtoMin int
	ProtoMaj int
	// yes yes there is the canonical header format, but this is just a basic example.
	Header map[string][]string
}

var (
	delim = []byte("\r\n")
)

func main() {
	const addr = "0.0.0.0:8080"
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	})
	slog.SetDefault(slog.New(h))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "addr", addr)
	}
	defer ln.Close()
	slog.Info("server started")
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("cannot accept connection", "remote", conn.RemoteAddr())
			continue
		}
		slog.Info("got connection", "remote", conn.RemoteAddr())
		handleConn(conn)
	}
}

// TODO: handle errors and read larger bodies
func readReq(c net.Conn) []byte {
	const bLen = 4096
	buf := make([]byte, bLen)
	// yes, I like danger
	n, _ := c.Read(buf)
	if n <= bLen {
		buf = buf[:n]
	}
	return buf
}

// body="GET / HTTP/1.1\r\nHost: localhost:8080\r\n
// User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:143.0) Gecko/20100101 Firefox/143.0\r\nAccept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate, br, zstd\r\nSec-GPC: 1\r\nConnection: keep-alive\r\nUpgrade-Insecure-Requests: 1\r\nSec-Fetch-Dest: document\r\nSec-Fetch-Mode: navigate\r\nSec-Fetch-Site: none\r\nSec-Fetch-User: ?1\r\nPriority: u=0, i\r\n\r\n"

func parseHTTPReq(req []byte) (*Request, error) {
	request := &Request{}
	space := []byte(" ")
	splits := bytes.Split(req, delim)
	ln1 := bytes.Split(splits[0], space)
	request.Method = string(ln1[0])
	uri := string(ln1[1])
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	request.URL = u
	request.Proto = string(ln1[2])
	maj, minV, ok := parseHTTPVer(request.Proto)
	if !ok {
		return nil, errors.New("invalid HTTP version string")
	}
	request.ProtoMaj = maj
	request.ProtoMin = minV
	ln2 := splits[1:]
	headerLen := calculateHeaderLen(ln2)
	incomingHeaders := splits[1:headerLen]
	headers := parseHeaders(incomingHeaders)
	request.Header = headers
	entityBody := splits[headerLen:]

	_ = entityBody
	return request, nil
}

func parseHTTPVer(vers string) (int, int, bool) {
	switch vers {
	case "HTTP/1.1":
		return 1, 1, true
	case "HTTP/1.0":
		return 1, 0, true
	}
	if !strings.HasPrefix(vers, "HTTP/") {
		return 0, 0, false
	}
	if len(vers) != len("HTTP/X.Y") {
		return 0, 0, false
	}
	maj, err := strconv.ParseUint(vers[5:6], 10, 0)
	if err != nil {
		return 0, 0, false
	}
	min, err := strconv.ParseUint(vers[7:8], 10, 0)
	if err != nil {
		return 0, 0, false
	}
	return int(maj), int(min), true
}

func calculateHeaderLen(remainingRequest [][]byte) int {
	for i, line := range remainingRequest {
		if string(line) == "" {
			return i + 1
		}
	}
	return -1
}

func parseHeaders(h [][]byte) map[string][]string {
	m := map[string][]string{}
	for _, item := range h {
		kvStr := string(item)
		splits := strings.SplitN(kvStr, ":", 2)
		k := splits[0]
		vals := strings.Split(strings.TrimSpace(splits[1]), ",")
		for i := 0; i < len(vals); i++ {
			vals[i] = strings.TrimSpace(vals[i])
		}
		m[k] = vals
	}
	return m
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	req := readReq(conn)
	// I will obviously handle the error (or maybe not)
	httpReq, _ := parseHTTPReq(req)
	slog.Info("request read", "body", httpReq)
	const msg = "HTTP/1.1 200 OK\r\n\r\nPong\r\n"
	// respond in kind
	l, err := conn.Write([]byte(msg))
	if err != nil {
		slog.Error("cannot serve request", "remote", conn.RemoteAddr())
	}
	if l != len(msg) {
		slog.Error("cannot write response, improper length", "expected", len(msg), "got", l)
	}
	slog.Info("response sent", "remote", conn.RemoteAddr())
}
