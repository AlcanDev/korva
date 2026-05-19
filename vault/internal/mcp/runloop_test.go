package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
)

// fakeDispatcher records every HandleRequest call and returns a configurable
// response so tests can verify both the request shape Serve forwards and the
// response shape Serve emits.
type fakeDispatcher struct {
	calls []Request
	reply func(Request) Response
}

func (f *fakeDispatcher) HandleRequest(req Request) Response {
	f.calls = append(f.calls, req)
	if f.reply != nil {
		return f.reply(req)
	}
	return makeResult(req.ID, map[string]string{"ok": "true"})
}

func TestServeForwardsRequestsAndEmitsResponses(t *testing.T) {
	d := &fakeDispatcher{}

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := Serve(d, strings.NewReader(input), &out, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	if got := len(d.calls); got != 2 {
		t.Fatalf("dispatcher saw %d requests, want 2", got)
	}
	if d.calls[0].Method != "initialize" {
		t.Errorf("first call method = %q, want initialize", d.calls[0].Method)
	}
	if d.calls[1].Method != "tools/list" {
		t.Errorf("second call method = %q, want tools/list", d.calls[1].Method)
	}

	// Output: one JSON envelope per line, both with ID matching the request.
	dec := json.NewDecoder(&out)
	var got []Response
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("decode: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 2 {
		t.Fatalf("emitted %d responses, want 2", len(got))
	}
	if got[0].ID.(float64) != 1 || got[1].ID.(float64) != 2 {
		t.Errorf("response IDs = %v, %v; want 1, 2", got[0].ID, got[1].ID)
	}
}

// TestServeSurvivesParseError documents the resilience invariant: a single
// malformed line must NOT terminate the loop — Serve replies with a
// -32700 envelope and continues to the next request. This matches the
// behavior of the original Server.Run() before the refactor.
func TestServeSurvivesParseError(t *testing.T) {
	d := &fakeDispatcher{}
	input := "{not json\n" +
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"

	var out bytes.Buffer
	if err := Serve(d, strings.NewReader(input), &out, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	if len(d.calls) != 1 || d.calls[0].Method != "ping" {
		t.Fatalf("dispatcher should see only ping after parse-error recovery, saw %v", d.calls)
	}

	dec := json.NewDecoder(&out)
	var first Response
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if first.Error == nil || first.Error.Code != -32700 {
		t.Errorf("first response should be parse error -32700, got %+v", first.Error)
	}
}

// TestServeBlankLinesSkipped verifies a stray blank line (common when an
// editor pipes through a buffering layer) does not produce an empty
// response envelope.
func TestServeBlankLinesSkipped(t *testing.T) {
	d := &fakeDispatcher{}
	input := "\n\n" + `{"jsonrpc":"2.0","id":7,"method":"ping"}` + "\n" + "\n"

	var out bytes.Buffer
	if err := Serve(d, strings.NewReader(input), &out, log.New(io.Discard, "", 0)); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	if len(d.calls) != 1 {
		t.Fatalf("dispatcher saw %d calls; blank lines should not dispatch", len(d.calls))
	}
}

// TestServeNilLoggerOK guards against the common mistake of passing a nil
// logger from a refactored call site. Serve must fall back to an io.Discard
// logger so the loop never panics on the first error path.
func TestServeNilLoggerOK(t *testing.T) {
	d := &fakeDispatcher{}
	var out bytes.Buffer

	// nil logger; a parse error forces the only code path that touches it.
	err := Serve(d, strings.NewReader("not json\n"), &out, nil)
	if err != nil {
		t.Fatalf("Serve with nil logger: %v", err)
	}
}

// TestRunStillWorksWithLegacyTestFields confirms the backward-compatibility
// guarantee documented on Server.Run(): if existing tests pre-seed
// s.reader/s.writer, Run() honors them instead of touching os.Stdin. The
// established helpers in server_test.go rely on this — breaking them was
// the easy mistake this guard prevents.
func TestRunStillWorksWithLegacyTestFields(t *testing.T) {
	var out bytes.Buffer
	srv := &Server{
		reader: bufio.NewReader(strings.NewReader("not json\n")),
		writer: &out,
		logger: log.New(io.Discard, "", 0),
	}

	if err := srv.Run(); err != nil {
		t.Fatalf("Run with legacy reader/writer: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected parse-error response written to legacy writer")
	}
}
