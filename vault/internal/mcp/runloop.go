package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/alcandev/korva/internal/version"
)

// Dispatcher is the minimum surface a JSON-RPC handler needs to drive the
// stdio loop. *Server satisfies it today; future implementations (e.g. a
// thin HTTP proxy when KORVA_VAULT_ENDPOINT points at a remote vault) will
// satisfy it without owning a local store.
type Dispatcher interface {
	HandleRequest(req Request) Response
}

// Serve runs the stdio JSON-RPC loop against the given Dispatcher.
//
// One JSON-RPC envelope per line on r. Each parsed Request is forwarded to
// d.HandleRequest and the resulting Response is written to w (also one line
// per envelope, newline-terminated). The loop blocks until r returns EOF or
// an unrecoverable read error, mirroring the behavior of the original
// Server.Run() so callers that piped into stdin/stdout see no change.
//
// Parse errors do NOT terminate the loop — they emit a -32700 JSON-RPC
// error response and the loop continues, same as before the refactor.
func Serve(d Dispatcher, r io.Reader, w io.Writer, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	logger.Printf("Korva Vault MCP stdio loop starting (%s)", version.String())

	br := bufio.NewReader(r)

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeJSONLine(w, logger, makeError(nil, -32700, "parse error", err.Error()))
			continue
		}

		writeJSONLine(w, logger, d.HandleRequest(req))
	}
}

// writeJSONLine marshals v and emits it as a single newline-terminated
// line on w. Marshal failures are logged but not propagated — the stdio
// loop must keep running so a single bad response doesn't end the session.
func writeJSONLine(w io.Writer, logger *log.Logger, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		logger.Printf("marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "%s\n", data)
}
