package generator

import "testing"

const networkRetrySpec = `openapi: 3.1.0
info: { title: net, version: "1" }
paths:
  /thing:
    get:
      operationId: getThing
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { type: string } }
`

// TestE2E_NetworkRetry drives the generated client over a hand-rolled HTTP/2
// connection, since net/http reports a reset stream through an unexported type
// that no shape check can name, and pins the failures that must not be retried.
func TestE2E_NetworkRetry(t *testing.T) {
	files, _ := generateFromSpec(t, networkRetrySpec, "netapi")

	runGeneratedWireTest(t, files, "networkretry", `package netapi

import (
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const (
	frameData        = 0x0
	frameHeaders     = 0x1
	frameRSTStream   = 0x3
	frameSettings    = 0x4
	flagEndStream    = 0x1
	flagEndHeaders   = 0x4
	flagSettingsAck  = 0x1
	errCodeInternal  = 0x2
	clientPreface    = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
)

func writeFrame(conn net.Conn, frameType, flags byte, streamID uint32, payload []byte) {
	header := make([]byte, 9)
	header[0], header[1], header[2] = byte(len(payload)>>16), byte(len(payload)>>8), byte(len(payload))
	header[3] = frameType
	header[4] = flags
	binary.BigEndian.PutUint32(header[5:], streamID)
	conn.Write(append(header, payload...))
}

// serveH2C answers the first request stream with RST_STREAM INTERNAL_ERROR and
// every later one with a 200 JSON body.
func serveH2C(t *testing.T, conn net.Conn, streams *atomic.Int32) {
	defer conn.Close()
	if _, err := io.ReadFull(conn, make([]byte, len(clientPreface))); err != nil {
		return
	}
	writeFrame(conn, frameSettings, 0, 0, nil)
	header := make([]byte, 9)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		length := int(header[0])<<16 | int(header[1])<<8 | int(header[2])
		streamID := binary.BigEndian.Uint32(header[5:]) & 0x7fffffff
		payload := make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}
		switch header[3] {
		case frameSettings:
			if header[4]&flagSettingsAck == 0 {
				writeFrame(conn, frameSettings, flagSettingsAck, 0, nil)
			}
		case frameHeaders:
			if streams.Add(1) == 1 {
				code := make([]byte, 4)
				binary.BigEndian.PutUint32(code, errCodeInternal)
				writeFrame(conn, frameRSTStream, 0, streamID, code)
				continue
			}
			// :status 200 from the static table, then content-type as a literal on static name 31.
			status := append([]byte{0x88, 0x0f, 0x10, 0x10}, "application/json"...)
			writeFrame(conn, frameHeaders, flagEndHeaders, streamID, status)
			writeFrame(conn, frameData, flagEndStream, streamID, []byte(`+"`"+`"ok"`+"`"+`))
		}
	}
}

func h2cClient(t *testing.T) (*Client, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var streams atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveH2C(t, conn, &streams)
		}
	}()
	transport := &http.Transport{Protocols: new(http.Protocols)}
	transport.Protocols.SetUnencryptedHTTP2(true)
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.MaxDelay = 10 * time.Millisecond
	return NewClient("http://"+ln.Addr().String(), WithRetry(cfg), WithHTTPClient(&http.Client{Transport: transport})), &streams
}

// A server-side RST_STREAM that is not REFUSED_STREAM is handed back by
// net/http rather than retried internally, so the client has to do it.
func TestResetStreamIsRetried(t *testing.T) {
	client, streams := h2cClient(t)

	got, err := client.GetThing(t.Context())
	if err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if got == nil || *got != "ok" {
		t.Errorf("result = %v", got)
	}
	if n := streams.Load(); n != 2 {
		t.Errorf("streams = %d, want the reset stream and one retry", n)
	}
}

// http.Client.Timeout reports as context.DeadlineExceeded since Go 1.23, but it
// is a per-attempt limit, not the caller's deadline, so the next attempt is due.
func TestClientTimeoutIsRetried(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-release
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.MaxDelay = 10 * time.Millisecond
	client := NewClient(srv.URL, WithRetry(cfg), WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))

	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected the timeout to surface")
	}
	if n := calls.Load(); n != int32(cfg.MaxRetries+1) {
		t.Errorf("calls = %d, want every attempt", n)
	}
}

type countingListener struct {
	net.Listener
	accepts atomic.Int32
}

func (l *countingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepts.Add(1)
	}
	return conn, err
}

func TestUntrustedCertificateIsNotRetried(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ln := &countingListener{Listener: srv.Listener}
	srv.Listener = ln
	srv.StartTLS()
	t.Cleanup(srv.Close)
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond
	client := NewClient(srv.URL, WithRetry(cfg))

	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected the certificate to be rejected")
	}
	if n := ln.accepts.Load(); n != 1 {
		t.Errorf("connections = %d, want a single attempt", n)
	}
}

func TestUnsupportedSchemeIsNotRetried(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = time.Second
	client := NewClient("ftp://127.0.0.1:1", WithRetry(cfg))

	start := time.Now()
	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected the scheme to be rejected")
	}
	if waited := time.Since(start); waited >= cfg.BaseDelay {
		t.Errorf("waited %v, want no retry", waited)
	}
}
`)
}
