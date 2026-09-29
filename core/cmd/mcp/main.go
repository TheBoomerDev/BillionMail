package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"billionmail-core/mcp"
	"github.com/modelcontextprotocol/go-sdk"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

var server *mcp.MCPServer

func main() {
	// Initialize BillionMail MCP server
	server = mcp.NewMCPServer()

	// Create MCP server
	mcpServer := sdk.NewServer("billionmail-server", "1.0.0", &sdk.ServerOptions{
		Capabilities: sdk.Capabilities{
			Tools: &sdk.ToolCapabilities{ListChanged: true},
		},
	})

	// Register tools
	mcpServer.AddTool("list_domains", "List all configured mail domains",
		jsonschema.Object{"type": "object"},
		func(ctx context.Context, _ *sdk.ToolParams) (any, error) {
			return server.ListDomains(ctx)
		})

	mcpServer.AddTool("get_smtp_status", "Get SMTP server status",
		jsonschema.Object{"type": "object"},
		func(ctx context.Context, _ *sdk.ToolParams) (any, error) {
			return server.GetSMTPStatus(ctx)
		})

	mcpServer.AddTool("create_domain", "Create a new mail domain",
		jsonschema.Object{
			"properties": map[string]any{
				"domain": map[string]any{"type": "string", "description": "Domain name to create"},
				"a_record": map[string]any{"type": "string", "description": "A record IP address"},
				"active": map[string]any{"type": "boolean", "default": true},
				"auto_dkim": map[string]any{"type": "boolean", "description": "Generate DKIM automatically", "default": false},
			},
			"required": []any{"domain"},
		},
		func(ctx context.Context, p *sdk.ToolParams) (any, error) {
			var params mcp.CreateDomainParams
			params.Domain = p.Params.(map[string]any)["domain"].(string)
			if a, ok := p.Params.(map[string]any)["a_record"].(string); ok {
				params.A_Record = a
			}
			if a, ok := p.Params.(map[string]any)["active"].(bool); ok {
				params.Active = a
			}
			if a, ok := p.Params.(map[string]any)["auto_dkim"].(bool); ok {
				params.AutoDKIM = a
			}
			return server.CreateDomain(ctx, params)
		},
	)

	mcpServer.AddTool("create_mailbox", "Create a new mailbox",
		jsonschema.Object{
			"properties": map[string]any{
				"username": map[string]any{"type": "string", "description": "Mailbox username"},
				"domain": map[string]any{"type": "string", "description": "Domain name"},
				"password": map[string]any{"type": "string", "description": "Mailbox password"},
			},
			"required": []any{"username", "domain"},
		},
		func(ctx context.Context, p *sdk.ToolParams) (any, error) {
			params := p.Params.(map[string]any)
			username := params["username"].(string)
			domain := params["domain"].(string)
			password := ""
			if pw, ok := params["password"].(string); ok {
				password = pw
			}
			return server.CreateMailbox(ctx, username, domain, password)
		},
	)

	// Get port from env or default to 8081
	port := 8081
	if p := os.Getenv("MCP_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	// Start HTTP server for MCP
	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		mcpServer.ServeHTTP(w, r)
	})

	log.Printf("BillionMail MCP Server starting on :%d", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatalf("MCP server failed: %v", err)
	}
}