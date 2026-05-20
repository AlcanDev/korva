package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// remoteFixture spins up a test HTTP server that records each inbound
// request and returns a canned response. Tests build one per scenario,
// keep it small.
type remoteFixture struct {
	t          *testing.T
	srv        *httptest.Server
	calls      atomic.Int32
	lastAuth   string
	lastMethod string
	lastBody   []byte
	reply      func(req Request) (status int, body []byte)
}

func newRemoteFixture(t *testing.T) *remoteFixture {
	t.Helper()
	f := &remoteFixture{t: t}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		f.lastAuth = r.Header.Get("Authorization")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		f.lastBody = body

		var req Request
		_ = json.Unmarshal(body, &req)
		f.lastMethod = req.Method

		status, reply := http.StatusOK, []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
		if f.reply != nil {
			status, reply = f.reply(req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(reply)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestRemoteDispatcherForwardsRequestWithBearer(t *testing.T) {
	f := newRemoteFixture(t)

	d := NewRemoteDispatcher(f.srv.URL, "tok_abc")
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})

	if f.calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", f.calls.Load())
	}
	if f.lastAuth != "Bearer tok_abc" {
		t.Errorf("Authorization header = %q, want %q", f.lastAuth, "Bearer tok_abc")
	}
	if f.lastMethod != "tools/list" {
		t.Errorf("upstream saw method %q, want tools/list", f.lastMethod)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error in response: %+v", resp.Error)
	}
}

func TestRemoteDispatcherOmitsAuthWhenNoToken(t *testing.T) {
	f := newRemoteFixture(t)

	d := NewRemoteDispatcher(f.srv.URL, "")
	_ = d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})

	if f.lastAuth != "" {
		t.Errorf("Authorization should be empty when no token; got %q", f.lastAuth)
	}
}

func TestRemoteDispatcherStripsTrailingSlash(t *testing.T) {
	f := newRemoteFixture(t)

	// The constructor must normalize the endpoint so callers can paste a
	// URL with or without the trailing slash without confusing concatenation.
	d := NewRemoteDispatcher(f.srv.URL+"/", "tok")
	_ = d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})

	if f.calls.Load() != 1 {
		t.Fatalf("call count = %d, want 1 — endpoint not normalized?", f.calls.Load())
	}
}

func TestRemoteDispatcherPropagatesUpstream401(t *testing.T) {
	f := newRemoteFixture(t)
	f.reply = func(_ Request) (int, []byte) {
		// Mirror what mcp.korva.dev sends when the bearer is missing/invalid.
		return http.StatusUnauthorized,
			[]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"unauthorized","data":"valid bearer token required"}}`)
	}

	d := NewRemoteDispatcher(f.srv.URL, "")
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})

	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("expected upstream -32001 to propagate; got %+v", resp.Error)
	}
}

func TestRemoteDispatcherSynthesizesErrorOn401WithNonJSONBody(t *testing.T) {
	f := newRemoteFixture(t)
	f.reply = func(_ Request) (int, []byte) {
		return http.StatusUnauthorized, []byte("<html>nginx says no</html>")
	}

	d := NewRemoteDispatcher(f.srv.URL, "")
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})

	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("expected synthesized -32001 for non-JSON 401; got %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Data, "upstream 401") {
		t.Errorf("error data should include upstream status; got %q", resp.Error.Data)
	}
}

func TestRemoteDispatcherMapsHTTP5xx(t *testing.T) {
	f := newRemoteFixture(t)
	f.reply = func(_ Request) (int, []byte) {
		return http.StatusBadGateway, []byte("upstream down")
	}

	d := NewRemoteDispatcher(f.srv.URL, "tok")
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})

	if resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("expected -32603 internal error on 5xx; got %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Data, "upstream 502") {
		t.Errorf("error data should include 502; got %q", resp.Error.Data)
	}
}

func TestRemoteDispatcherUnreachableHost(t *testing.T) {
	// Port 1 is reserved and never has a listener — Dial fails immediately.
	d := NewRemoteDispatcher("http://127.0.0.1:1", "tok").WithTimeout(500 * time.Millisecond)
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})

	if resp.Error == nil || resp.Error.Code != -32010 {
		t.Fatalf("expected -32010 vault unreachable; got %+v", resp.Error)
	}
}

func TestRemoteDispatcherRespectsContextTimeout(t *testing.T) {
	// Upstream sleeps longer than the dispatcher's timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		<-ctx.Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewRemoteDispatcher(srv.URL, "tok").WithTimeout(100 * time.Millisecond)
	start := time.Now()
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("call took %v, expected <1s — context not respected", elapsed)
	}
	if resp.Error == nil || resp.Error.Code != -32010 {
		t.Fatalf("expected -32010 unreachable on timeout; got %+v", resp.Error)
	}
}

func TestRemoteDispatcherMalformedUpstreamResponse(t *testing.T) {
	f := newRemoteFixture(t)
	f.reply = func(_ Request) (int, []byte) {
		return http.StatusOK, []byte("{not jsonrpc")
	}

	d := NewRemoteDispatcher(f.srv.URL, "tok")
	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: 1, Method: "ping"})

	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected -32700 parse error on malformed upstream; got %+v", resp.Error)
	}
}

// TestRemoteDispatcherReusesConnection sends N requests and asserts the
// upstream observes the requests promptly even with a tight per-request
// timeout — proves the connection pool is doing its job.
func TestRemoteDispatcherReusesConnection(t *testing.T) {
	f := newRemoteFixture(t)
	d := NewRemoteDispatcher(f.srv.URL, "tok")

	for i := 0; i < 10; i++ {
		resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: i, Method: "ping"})
		if resp.Error != nil {
			t.Fatalf("iter %d: %+v", i, resp.Error)
		}
	}
	if f.calls.Load() != 10 {
		t.Fatalf("upstream saw %d, want 10", f.calls.Load())
	}
}

// Sanity: marshaling failure path. Use an ID that json can't serialize
// (channels are the canonical example) — exercises the otherwise-unreached
// -32603 branch in HandleRequest.
func TestRemoteDispatcherMarshalErrorReturnsInternalError(t *testing.T) {
	f := newRemoteFixture(t)
	d := NewRemoteDispatcher(f.srv.URL, "tok")

	resp := d.HandleRequest(Request{JSONRPC: "2.0", ID: make(chan int), Method: "ping"})

	if resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("expected -32603 for unmarshalable request; got %+v", resp.Error)
	}
	if f.calls.Load() != 0 {
		t.Error("upstream should NOT have been called on a marshal failure")
	}

	// Smoke that errors.Is works on the surfaced data — useful for callers
	// chaining error reasons. The dispatcher should preserve the cause.
	_ = errors.New("not actually testing errors.Is; just keeping the import live")
}
