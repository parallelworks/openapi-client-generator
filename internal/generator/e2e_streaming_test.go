package generator

import "testing"

const streamingSpec = `openapi: 3.1.0
info: { title: chat, version: "1" }
paths:
  /chat:
    post:
      operationId: chat
      requestBody:
        required: true
        content:
          application/json: { schema: { $ref: "#/components/schemas/Request" } }
      responses:
        "200":
          description: whole or in parts
          content:
            application/json: { schema: { $ref: "#/components/schemas/Reply" } }
            text/event-stream: { schema: { $ref: "#/components/schemas/Chunk" } }
  /logs:
    get:
      operationId: tailLogs
      responses:
        "200":
          description: text events
          content:
            text/event-stream: { schema: { type: string } }
components:
  schemas:
    Request:
      type: object
      properties:
        prompt: { type: string }
    Reply:
      type: object
      properties:
        text: { type: string }
    Chunk:
      type: object
      properties:
        delta: { type: string }
        index: { type: integer }
`

// TestE2E_EventStreams covers a response offering text/event-stream. The client
// read every response whole, so a stream was reachable only as one lump after
// the server finished, which for a chat completion is the opposite of the point.
func TestE2E_EventStreams(t *testing.T) {
	files, _ := generateFromSpec(t, streamingSpec, "chatapi")

	runGeneratedWireTest(t, files, "eventstream", `package chatapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// frames serves the given SSE text, flushing each write so the client sees the
// events as they are written rather than at the end.
func frames(t *testing.T, chunks ...string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q, want text/event-stream", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, chunk := range chunks {
			w.Write([]byte(chunk))
			w.(http.Flusher).Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestChunksArriveTyped(t *testing.T) {
	client := frames(t,
		": keep-alive\n\n",
		"data: {\"delta\":\"He\",\"index\":0}\n\n",
		"data: {\"delta\":\"llo\",\"index\":1}\n\n",
		"data: [DONE]\n\n",
	)

	stream, err := client.ChatStream(t.Context(), Request{})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer stream.Close()

	var text string
	var count int
	for {
		chunk, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		count++
		if chunk.Delta != nil {
			text += *chunk.Delta
		}
	}

	// The comment is a keep-alive and the sentinel is not an event, so neither
	// is delivered as one.
	if count != 2 || text != "Hello" {
		t.Errorf("got %d chunks spelling %q, want 2 spelling Hello", count, text)
	}
}

// Events arrive as the server writes them, which is the whole reason for the
// method: a reader blocked until the response completed would time out here.
func TestEventsArriveBeforeTheStreamEnds(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"delta\":\"first\"}\n\n"))
		w.(http.Flusher).Flush()
		<-release
		w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()
	defer close(release)

	stream, err := NewClient(srv.URL).ChatStream(t.Context(), Request{})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer stream.Close()

	done := make(chan string, 1)
	go func() {
		chunk, err := stream.Next()
		if err != nil {
			done <- "error: " + err.Error()
			return
		}
		done <- *chunk.Delta
	}()

	select {
	case got := <-done:
		if got != "first" {
			t.Errorf("first event = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first event did not arrive while the server was still writing")
	}
}

func TestMultilineDataAndEventFields(t *testing.T) {
	client := frames(t, "event: tick\nid: 7\ndata: line one\ndata: line two\n\n")

	stream, err := client.TailLogsStream(t.Context())
	if err != nil {
		t.Fatalf("TailLogsStream: %v", err)
	}
	defer stream.Close()

	// A stream of text declares string, where the data is the value rather than
	// a document to decode.
	line, err := stream.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if line != "line one\nline two" {
		t.Errorf("payload = %q, want the data lines joined", line)
	}
	if stream.EventName() != "tick" || stream.EventID() != "7" {
		t.Errorf("event name = %q id = %q, want tick and 7", stream.EventName(), stream.EventID())
	}
}

// A stream that ends without the blank line that would dispatch its last event
// still delivers it.
func TestUnterminatedFinalEventIsDelivered(t *testing.T) {
	client := frames(t, "data: {\"delta\":\"tail\"}")

	stream, err := client.ChatStream(t.Context(), Request{})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if chunk.Delta == nil || *chunk.Delta != "tail" {
		t.Errorf("chunk = %+v", chunk)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("second Next = %v, want io.EOF", err)
	}
}

func TestErrorStatusIsAnErrorRatherThanAStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("slow down"))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).ChatStream(t.Context(), Request{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("err = %v, want the 429", err)
	}
	if string(apiErr.Body) != "slow down" {
		t.Errorf("body = %q, want the error payload", apiErr.Body)
	}
}

// The buffered method is still there, and still decodes the JSON alternative.
func TestBufferedMethodIsUnaffected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"text\":\"whole\"}"))
	}))
	defer srv.Close()

	reply, err := NewClient(srv.URL).Chat(t.Context(), Request{})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if reply == nil || reply.Text == nil || *reply.Text != "whole" {
		t.Errorf("reply = %+v", reply)
	}
}
`)
}
