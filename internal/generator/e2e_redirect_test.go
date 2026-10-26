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
  /search:
    get:
      operationId: search
      requestBody:
        content:
          application/json: { schema: { type: string } }
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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

type received struct {
	header atomic.Value
	hits   atomic.Int32
}

func (rc *received) Header() http.Header {
	h, _ := rc.header.Load().(http.Header)
	return h
}

func recording(rc *received) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc.hits.Add(1)
		rc.header.Store(r.Header.Clone())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`"moved"`+"`"+`))
	}
}

// elsewhere is another origin on the same host, to which net/http itself would carry Authorization.
func elsewhere(t *testing.T) (*httptest.Server, *received) {
	var rc received
	srv := httptest.NewServer(recording(&rc))
	t.Cleanup(srv.Close)
	return srv, &rc
}

func redirecting(t *testing.T, target string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func withHeaders(req *http.Request, next RoundTripFunc) (*http.Response, error) {
	req.Header.Set("X-Api-Key", "secret")
	req.Header["x-secret"] = []string{"secret"}
	return next(req)
}

func TestAGetFollowsARedirectElsewhereWithoutItsCredentials(t *testing.T) {
	other, rc := elsewhere(t)
	client := NewClient(redirecting(t, other.URL).URL, WithAuth(&BearerAuth{Token: "secret"}), WithMiddleware(withHeaders), WithHTTPClient(&http.Client{}))

	got, err := client.GetThing(t.Context())
	if err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if *got != "moved" {
		t.Errorf("got %q", *got)
	}
	for _, name := range []string{"Authorization", "X-Api-Key", "X-Secret"} {
		if v := rc.Header().Get(name); v != "" {
			t.Errorf("the other origin received %s %q", name, v)
		}
	}
	if rc.Header().Get("Accept") == "" {
		t.Error("the other origin did not receive Accept")
	}
}

func TestAPostStopsAtARedirectElsewhere(t *testing.T) {
	other, rc := elsewhere(t)
	client := NewClient(redirecting(t, other.URL).URL, WithAuth(&BearerAuth{Token: "secret"}))

	_, err := client.CreateThing(t.Context())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("got %v, want the redirect handed back", err)
	}
	if rc.hits.Load() != 0 {
		t.Error("the request was sent to the other origin")
	}
}

func TestAPostSeeOtherElsewhereFollowsAsAGetWithoutItsCredentials(t *testing.T) {
	other, rc := elsewhere(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusSeeOther)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, WithAuth(&BearerAuth{Token: "secret"}), WithMiddleware(withHeaders))

	got, err := client.CreateThing(t.Context())
	if err != nil {
		t.Fatalf("CreateThing: %v", err)
	}
	if *got != "moved" {
		t.Errorf("got %q", *got)
	}
	for _, name := range []string{"Authorization", "X-Api-Key", "X-Secret"} {
		if v := rc.Header().Get(name); v != "" {
			t.Errorf("the other origin received %s %q", name, v)
		}
	}
}

func TestAPostRedirectedElsewhereBecomesAGetOnlyForASeeOther(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound} {
		other, rc := elsewhere(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+r.URL.Path, status)
		}))
		t.Cleanup(srv.Close)
		client := NewClient(srv.URL, WithAuth(&BearerAuth{Token: "secret"}))

		_, err := client.CreateThing(t.Context())
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
			t.Fatalf("a POST answered %d: got %v, want the redirect handed back", status, err)
		}
		if rc.hits.Load() != 0 {
			t.Errorf("a POST answered %d took another origin's page for its answer", status)
		}
	}
}

func TestARangedGetKeepsItsRangeElsewhere(t *testing.T) {
	other, rc := elsewhere(t)
	ranged := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		req.Header.Set("Range", "bytes=0-1")
		req.Header.Set("If-Range", `+"`"+`"v1"`+"`"+`)
		req.Header.Set("If-None-Match", `+"`"+`"v0"`+"`"+`)
		return next(req)
	}
	client := NewClient(redirecting(t, other.URL).URL, WithAuth(&BearerAuth{Token: "secret"}), WithMiddleware(ranged))

	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	for name, want := range map[string]string{"Range": "bytes=0-1", "If-Range": `+"`"+`"v1"`+"`"+`, "If-None-Match": `+"`"+`"v0"`+"`"+`} {
		if got := rc.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if a := rc.Header().Get("Authorization"); a != "" {
		t.Errorf("the other origin received Authorization %q", a)
	}
}

func TestAClientsOwnRedirectRefusalIsNotRetried(t *testing.T) {
	for _, refusal := range []error{
		errors.New("stopped after a redirect this caller refuses"),
		errors.New("redirect refused"),
		fmt.Errorf("refusing: %w", ErrInsecureRedirect),
	} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}))
		t.Cleanup(srv.Close)
		own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return refusal }}
		client := NewClient(srv.URL, fastRetry(), WithHTTPClient(own))

		if _, err := client.GetThing(t.Context()); !errors.Is(err, refusal) {
			t.Fatalf("got %v, want the caller's refusal %q", err, refusal)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("calls = %d for %q, want a single attempt", n, refusal)
		}
	}
}

func TestAGetWithABodyStopsAtARedirectElsewhere(t *testing.T) {
	other, rc := elsewhere(t)
	client := NewClient(redirecting(t, other.URL).URL)

	query := "secret"
	_, err := client.Search(t.Context(), &query)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("got %v, want the redirect handed back", err)
	}
	if rc.hits.Load() != 0 {
		t.Error("the body was sent to the other origin")
	}
}

func TestAPostFollowsARedirectOnItsOrigin(t *testing.T) {
	var rc received
	mux := http.NewServeMux()
	mux.HandleFunc("POST /thing", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/moved", recording(&rc))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for i, base := range []string{srv.URL, strings.Replace(srv.URL, "://", "://user:pass@", 1)} {
		client := NewClient(base, WithAuth(&BearerAuth{Token: "secret"}))
		if _, err := client.CreateThing(t.Context()); err != nil {
			t.Fatalf("CreateThing on %s: %v", base, err)
		}
		if n := rc.hits.Load(); n != int32(i+1) {
			t.Fatalf("the redirect on %s was not followed", base)
		}
		if a := rc.Header().Get("Authorization"); a != "Bearer secret" {
			t.Errorf("Authorization = %q on %s, want the credential kept on its origin", a, base)
		}
	}
}

func TestAnUpgradeToHTTPSOnAnotherPortIsAnotherOrigin(t *testing.T) {
	var rc received
	secure := httptest.NewTLSServer(recording(&rc))
	t.Cleanup(secure.Close)
	client := NewClient(redirecting(t, secure.URL).URL, WithAuth(&BearerAuth{Token: "secret"}), WithHTTPClient(secure.Client()))

	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if a := rc.Header().Get("Authorization"); a != "" {
		t.Errorf("another port received Authorization %q", a)
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

func TestARedirectRefusalAMiddlewareDropsTheResponseOfIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	dropping := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		resp, err := next(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return nil, err
		}
		return resp, nil
	}
	client := NewClient(srv.URL, fastRetry(), WithMiddleware(dropping))

	if _, err := client.GetThing(t.Context()); !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("got %v, want ErrTooManyRedirects", err)
	}
	if n := calls.Load(); n != 10 {
		t.Errorf("calls = %d, want one chain of redirects", n)
	}
}

func TestACallersRefusalAMiddlewareDropsTheResponseOfIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	refusal := errors.New("redirect refused")
	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return refusal }}
	dropping := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		resp, err := next(req)
		if resp != nil && err != nil {
			resp.Body.Close()
			resp = nil
		}
		return resp, err
	}
	client := NewClient(srv.URL, fastRetry(), WithHTTPClient(own), WithMiddleware(dropping))

	_, err := client.GetThing(t.Context())
	var refused *RedirectRefusedError
	if !errors.As(err, &refused) || !errors.Is(err, refusal) {
		t.Fatalf("got %v, want a *RedirectRefusedError wrapping the caller's refusal", err)
	}
	if !strings.HasSuffix(err.Error(), "redirect refused") {
		t.Errorf("message = %q, want the caller's own", err.Error())
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want a single attempt", n)
	}
}

func TestTheSDKsRefusalsAreRedirectRefusedErrors(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(plain.Close)
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(secure.Close)
	looping := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(looping.Close)

	for _, tc := range []struct {
		client *Client
		want   error
	}{
		{NewClient(secure.URL, WithHTTPClient(secure.Client())), ErrInsecureRedirect},
		{NewClient(looping.URL), ErrTooManyRedirects},
		{NewClient(secure.URL, WithHTTPClient(SafeRedirects(secure.Client()))), ErrInsecureRedirect},
	} {
		_, err := tc.client.GetThing(t.Context())
		var refused *RedirectRefusedError
		if !errors.As(err, &refused) || refused.Err != tc.want {
			t.Errorf("got %v, want a single *RedirectRefusedError wrapping %v", err, tc.want)
		}
	}
}

func TestACallersPolicyFollowsWhatTheSDKsWouldStop(t *testing.T) {
	other, rc := elsewhere(t)
	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	client := NewClient(redirecting(t, other.URL).URL, WithHTTPClient(own))

	if _, err := client.CreateThing(t.Context()); err != nil {
		t.Fatalf("CreateThing: %v", err)
	}
	if rc.hits.Load() != 1 {
		t.Error("the caller's policy did not decide the redirect")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestATransientErrorAMiddlewareReturnsWithAResponseIsRetried(t *testing.T) {
	other, rc := elsewhere(t)
	var calls atomic.Int32
	flaky := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{StatusCode: http.StatusBadGateway}, io.ErrUnexpectedEOF
		}
		return next(req)
	}
	client := NewClient(other.URL, fastRetry(), WithMiddleware(flaky))

	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if n := calls.Load(); n != 2 || rc.hits.Load() != 1 {
		t.Errorf("calls = %d, hits = %d, want the transient failure retried once", n, rc.hits.Load())
	}
}

func TestARetryAfterAMiddlewareReturnsWithAnErrorIsHonored(t *testing.T) {
	other, rc := elsewhere(t)
	var calls atomic.Int32
	busy := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Retry-After": {"120"}}}, io.ErrUnexpectedEOF
		}
		return next(req)
	}
	client := NewClient(other.URL, fastRetry(), WithMiddleware(busy))

	if _, err := client.GetThing(t.Context()); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v, want the error handed back rather than a retry sooner than the server asked", err)
	}
	if n := calls.Load(); n != 1 || rc.hits.Load() != 0 {
		t.Errorf("calls = %d, hits = %d, want no retry before the Retry-After", n, rc.hits.Load())
	}
}

func TestARefusalAMiddlewaresOwnClientReturnsWithTheResponseIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	own := func(req *http.Request, next RoundTripFunc) (*http.Response, error) {
		return (&http.Client{}).Do(req)
	}
	client := NewClient(srv.URL, fastRetry(), WithMiddleware(own))

	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected net/http's refusal")
	}
	if n := calls.Load(); n != 10 {
		t.Errorf("calls = %d, want one chain of redirects", n)
	}
}

func TestAnUntypedRedirectRefusalFromATransportIsNotRetried(t *testing.T) {
	for _, refusal := range []error{ErrInsecureRedirect, ErrTooManyRedirects} {
		var calls atomic.Int32
		inner := transportFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: refusal}
		})
		client := NewClient("http://example.test", fastRetry(), WithHTTPClient(&http.Client{Transport: inner}))

		_, err := client.GetThing(t.Context())
		if !errors.Is(err, refusal) {
			t.Fatalf("got %v, want %v", err, refusal)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("calls = %d for %v, want a single attempt", n, refusal)
		}
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestARedirectRefusedErrorAnswersForItsError(t *testing.T) {
	var empty RedirectRefusedError
	if empty.Error() == "" || empty.Timeout() || empty.Temporary() {
		t.Errorf("an empty RedirectRefusedError says %q, timeout %v, temporary %v", empty.Error(), empty.Timeout(), empty.Temporary())
	}

	var refusal net.Error = &RedirectRefusedError{Err: timeoutError{}}
	if !refusal.Timeout() || !refusal.Temporary() {
		t.Error("a refusal of a timeout does not report one")
	}
	if wrapped := (&RedirectRefusedError{Err: fmt.Errorf("policy: %w", timeoutError{})}); !wrapped.Timeout() || !wrapped.Temporary() {
		t.Error("a refusal of a wrapped timeout does not report one")
	}
	if !(&url.Error{Op: "Get", URL: "https://example.test", Err: refusal}).Timeout() {
		t.Error("url.Error.Timeout does not see the refusal's timeout")
	}

	plain := &RedirectRefusedError{Err: ErrInsecureRedirect}
	if plain.Timeout() || plain.Temporary() {
		t.Error("a refusal of a plain error reports a timeout")
	}
}

func TestAMiddlewareThatReturnsNothingIsAnErrorAndNotRetried(t *testing.T) {
	var calls atomic.Int32
	empty := func(*http.Request, RoundTripFunc) (*http.Response, error) {
		calls.Add(1)
		return nil, nil
	}
	client := NewClient("http://example.test", fastRetry(), WithMiddleware(empty))

	_, err := client.GetThing(t.Context())
	if err == nil || !strings.Contains(err.Error(), "middleware returned no response and no error") {
		t.Fatalf("got %v, want the middleware's missing response", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("calls = %d, want a single attempt", n)
	}
}

func TestAClientsOwnRedirectPolicyIsKept(t *testing.T) {
	other, rc := elsewhere(t)
	own := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	client := NewClient(redirecting(t, other.URL).URL, WithHTTPClient(own))

	if _, err := client.GetThing(t.Context()); err == nil {
		t.Fatal("expected the redirect to be handed back")
	}
	if rc.hits.Load() != 0 {
		t.Error("the client's own policy was replaced")
	}
}

type countingTransport struct{ n atomic.Int32 }

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return http.DefaultTransport.RoundTrip(r)
}

func TestTheHTTPClientIsReadAtEachRequest(t *testing.T) {
	other, _ := elsewhere(t)
	hc := &http.Client{}
	client := NewClient(other.URL, WithHTTPClient(hc))
	var transport countingTransport
	hc.Transport = &transport

	if _, err := client.GetThing(t.Context()); err != nil {
		t.Fatalf("GetThing: %v", err)
	}
	if transport.n.Load() != 1 {
		t.Error("a Transport set after NewClient was not used")
	}
}
`)
}
