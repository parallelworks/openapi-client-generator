package generator

import "testing"

const retryAfterSpec = `openapi: 3.1.0
info: { title: rate, version: "1" }
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

// TestE2E_RetryAfter covers both forms of the header and the wait a client did
// not agree to: a date was ignored entirely, and a long delta blocked the call
// for its full length regardless of MaxDelay.
func TestE2E_RetryAfter(t *testing.T) {
	files, _ := generateFromSpec(t, retryAfterSpec, "rateapi")

	runGeneratedWireTest(t, files, "retryafter", `package rateapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// limited answers 429 with the given Retry-After until the last attempt.
func limited(t *testing.T, header string, failures int) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= failures {
			w.Header().Set("Retry-After", header)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`"ok"`+"`"+`))
	}))
	t.Cleanup(srv.Close)
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.MaxDelay = 5 * time.Second
	return NewClient(srv.URL, WithRetry(cfg)), &calls
}

func TestDeltaSecondsIsHonored(t *testing.T) {
	client, calls := limited(t, "1", 1)

	start := time.Now()
	got, err := client.GetThing(t.Context())
	if err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if got == nil || *got != "ok" {
		t.Errorf("result = %v", got)
	}
	if *calls != 2 {
		t.Errorf("calls = %d, want a retry", *calls)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("waited %v, want at least the second the server asked for", waited)
	}
}

// The date form is the other half of RFC 9110, and was falling through to
// backoff, which retries sooner than the server allowed.
func TestHTTPDateIsHonored(t *testing.T) {
	// HTTP dates carry whole seconds, so asking for two guarantees at least one
	// survives the truncation.
	client, calls := limited(t, time.Now().Add(2*time.Second).UTC().Format(http.TimeFormat), 1)

	start := time.Now()
	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if *calls != 2 {
		t.Errorf("calls = %d, want a retry", *calls)
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("waited %v, want the client to respect the date it was given", waited)
	}
}

// A wait past MaxDelay is not something the caller agreed to, so the call
// returns rather than sleeping it out.
func TestWaitBeyondMaxDelayGivesUp(t *testing.T) {
	client, calls := limited(t, "3600", 5)

	start := time.Now()
	_, err := client.GetThing(t.Context())
	if err == nil {
		t.Fatal("expected the rate limit error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("err = %v, want the 429", err)
	}
	if *calls != 1 {
		t.Errorf("calls = %d, want no retry", *calls)
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("waited %v, want the call to return rather than sleep", waited)
	}

	// The header is still on the response, so the caller can schedule it.
	if apiErr.Status == "" {
		t.Error("the error should carry the response status")
	}
}

// A retryable status with no Retry-After still backs off as before.
func TestNoHeaderStillBacksOff(t *testing.T) {
	client, calls := limited(t, "", 1)

	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if *calls != 2 {
		t.Errorf("calls = %d, want a retry", *calls)
	}
}
`)
}
