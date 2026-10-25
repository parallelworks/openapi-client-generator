package generator

import "testing"

const redirectSpec = `openapi: 3.1.0
info: { title: redirects, version: "1" }
components:
  securitySchemes:
    bearer: { type: http, scheme: bearer }
security:
  - bearer: []
paths:
  /thing:
    get:
      operationId: getThing
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { type: string } }
    post:
      operationId: createThing
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { type: string } }
`

// TestE2E_Redirects pins the generated client's redirect policy: no credential
// leaves the origin that was asked, and nothing is sent over plain http.
func TestE2E_Redirects(t *testing.T) {
	files, _ := generateFromSpec(t, redirectSpec, "redirectapi")

	runGeneratedWireTest(t, files, "redirects", `package redirectapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetry() ClientOption {
	cfg := DefaultRetryConfig()
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = time.Millisecond
	return WithRetry(cfg)
}

// elsewhere is another origin on the same host, to which net/http itself would carry Authorization.
func elsewhere(t *testing.T) (*httptest.Server, *atomic.Value, *atomic.Int32) {
	var authorization atomic.Value
	authorization.Store("")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		authorization.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`"moved"`+"`"+`))
	}))
	t.Cleanup(srv.Close)
	return srv, &authorization, &hits
}

func redirecting(t *testing.T, target string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAGetFollowsARedirectElsewhereWithoutItsCredential(t *testing.T) {
	other, authorization, _ := elsewhere(t)
	client := NewClient(redirecting(t, other.URL).URL, WithAuth(&BearerAuth{Token: "secret"}), WithHTTPClient(&http.Client{}))

	got, err := client.GetThing(t.Context())
	if err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if *got != "moved" {
		t.Errorf("got %q", *got)
	}
	if a := authorization.Load(); a != "" {
		t.Errorf("the other origin received Authorization %q", a)
	}
}

func TestAPostStopsAtARedirectElsewhere(t *testing.T) {
	other, _, hits := elsewhere(t)
	client := NewClient(redirecting(t, other.URL).URL, WithAuth(&BearerAuth{Token: "secret"}))

	_, err := client.CreateThing(t.Context())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("got %v, want the redirect handed back", err)
	}
	if hits.Load() != 0 {
		t.Error("the request was sent to the other origin")
	}
}

func TestARedirectToPlainHTTPIsRefusedAndNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "http://"+r.Host+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, fastRetry(), WithHTTPClient(srv.Client()))

	_, err := client.GetThing(t.Context())
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("got %v, want ErrInsecureRedirect", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want a single attempt", n)
	}
}

func TestTooManyRedirectsAreNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, fastRetry())

	_, err := client.GetThing(t.Context())
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("got %v, want ErrTooManyRedirects", err)
	}
	if n := calls.Load(); n != 10 {
		t.Errorf("calls = %d, want one chain of redirects", n)
	}
}

func TestAClientsOwnRedirectPolicyIsKept(t *testing.T) {
	other, _, hits := elsewhere(t)
	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	client := NewClient(redirecting(t, other.URL).URL, WithHTTPClient(own))

	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected the redirect to be handed back")
	}
	if hits.Load() != 0 {
		t.Error("the client's own policy was replaced")
	}
}
`)
}
