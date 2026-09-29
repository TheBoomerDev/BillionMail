// BillionMail MCP Server
// Exposes BillionMail management operations as MCP tools over Streamable HTTP.
// It is a thin HTTP client of the BillionMail core REST API (no internal imports),
// so it builds standalone and can run beside the core container.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	bm "billionmail-core/mcp"
)

// toolRequest is the minimal JSON-RPC shape we need to serve MCP tool calls.
type toolRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"params"`
}

type toolResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// textResult wraps output as MCP content blocks
func textResult(v any) map[string]any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(b)},
		},
	}
}

func main() {
	ctx := context.Background()
	client := bm.NewClientFromEnv()

	// Fail fast if the core is unreachable / credentials are wrong.
	if err := client.Login(ctx); err != nil {
		log.Printf("warning: initial login failed (%v) — will retry on first tool call", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req toolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, nil, -32700, "parse error: "+err.Error())
			return
		}

		switch req.Method {
		case "initialize":
			writeOK(w, req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "billionmail-mcp", "version": "1.0.0"},
			})
		case "tools/list":
			writeOK(w, req.ID, map[string]any{"tools": toolsSchema()})
		case "tools/call":
			result, err := dispatch(ctx, client, req.Params.Name, req.Params.Arguments)
			if err != nil {
				writeErr(w, req.ID, -32000, err.Error())
				return
			}
			writeOK(w, req.ID, textResult(result))
		case "notifications/initialized":
			writeOK(w, req.ID, map[string]any{})
		default:
			writeErr(w, req.ID, -32601, "method not supported: "+req.Method)
		}
	})

	port := 9090
	if p := os.Getenv("MCP_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	log.Printf("BillionMail MCP server listening on :%d/mcp (core: %s)", port, os.Getenv("BILLIONMAIL_URL"))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

func dispatch(ctx context.Context, client *bm.Client, name string, args json.RawMessage) (any, error) {
	var a map[string]any
	if len(args) > 0 {
		_ = json.Unmarshal(args, &a)
	}
	str := func(k string) string {
		if v, ok := a[k].(string); ok {
			return v
		}
		return ""
	}

	switch name {
	case "list_domains":
		return client.ListDomains(ctx)
	case "create_domain":
		d := str("domain")
		if d == "" {
			return nil, fmt.Errorf("missing required argument: domain")
		}
		return client.CreateDomain(ctx, d, str("a_record"))
	case "delete_domain":
		d := str("domain")
		if d == "" {
			return nil, fmt.Errorf("missing required argument: domain")
		}
		return client.DeleteDomain(ctx, d)
	case "create_mailbox":
		u, d := str("username"), str("domain")
		if u == "" || d == "" {
			return nil, fmt.Errorf("missing required arguments: username, domain")
		}
		return client.CreateMailbox(ctx, u, d, str("password"))
	case "list_mailboxes":
		return client.ListMailboxes(ctx, str("domain"))
	case "get_service_status":
		return client.GetServiceStatus(ctx)
	case "get_overview":
		return client.GetOverview(ctx)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func toolsSchema() []map[string]any {
	schema := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	s := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

	return []map[string]any{
		{"name": "list_domains", "description": "List all configured mail domains", "inputSchema": schema(map[string]any{})},
		{"name": "create_domain", "description": "Create a new mail domain", "inputSchema": schema(
			map[string]any{"domain": s("Domain name"), "a_record": s("A record IP (optional)")}, "domain")},
		{"name": "delete_domain", "description": "Delete a mail domain", "inputSchema": schema(
			map[string]any{"domain": s("Domain name")}, "domain")},
		{"name": "create_mailbox", "description": "Create a mailbox user@domain", "inputSchema": schema(
			map[string]any{"username": s("Mailbox username"), "domain": s("Domain name"), "password": s("Password")},
			"username", "domain")},
		{"name": "list_mailboxes", "description": "List mailboxes, optionally filtered by domain", "inputSchema": schema(
			map[string]any{"domain": s("Filter by domain (optional)")})},
		{"name": "get_service_status", "description": "Get SMTP/IMAP/POP service status", "inputSchema": schema(map[string]any{})},
		{"name": "get_overview", "description": "Get platform overview stats", "inputSchema": schema(map[string]any{})},
	}
}

func writeOK(w http.ResponseWriter, id json.RawMessage, result any) {
	writeJSON(w, toolResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func writeErr(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	writeJSON(w, toolResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
