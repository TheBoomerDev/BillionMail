# MCP Server Integration

## What is MCP Support?

This fork includes **MCP (Model Context Protocol)** server support, allowing AI agents like Hermes Agent to interact with BillionMail programmatically.

## Quick Start

```bash
# Build and deploy MCP server
./deploy-mcp.sh 9090

# Or using Docker Compose
docker compose -f docker-compose.mcp.yml up -d
```

## Available MCP Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `list_domains` | List all configured mail domains | `{}` |
| `get_smtp_status` | Get SMTP server status | `{}` |
| `create_domain` | Create a new mail domain | `domain`, `a_record`, `active`, `auto_dkim` |
| `create_mailbox` | Create a new mailbox | `username`, `domain`, `password` |
| `get_domain` | Get domain details by name | `domain` |

## Example Usage with Hermes Agent

```python
from mcp_client import MCPClient

client = MCPClient("http://mail.cargoffer.com:9090/mcp")

# List domains
domains = client.call_tool("list_domains")

# Create new domain with DKIM
domain = client.call_tool("create_domain", {
    "domain": "example.com",
    "a_record": "192.168.1.100",
    "active": True,
    "auto_dkim": True
})

# Create mailbox
mailbox = client.call_tool("create_mailbox", {
    "username": "john",
    "domain": "example.com",
    "password": "secure_password"
})
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_PORT` | `9090` | Port for MCP server |
| `BILLIONMAIL_HOSTNAME` | `""` | Default mail hostname |

## API Endpoints

- **MCP Server**: `http://<host>:9090/mcp`
- **Core API**: `http://<host>:8888/api/`

## Deployment

The MCP server can be deployed standalone or integrated with existing BillionMail installation.

### Standalone Deployment

```bash
# Clone this fork
git clone https://github.com/TheBoomerDev/BillionMail.git
cd BillionMail

# Deploy MCP server only
./deploy-mcp.sh 9090
```

### Integrated with Core

Add the MCP server as a service in your existing docker-compose.yml:

```yaml
  mcp-server:
    build:
      context: .
      dockerfile: Dockerfile.mcp
    ports:
      - "9090:9090"
    environment:
      - BILLIONMAIL_HOSTNAME=mail.yourdomain.com
```

## Building from Source

```bash
# From the core directory
cd core
go build -o /opt/billionmail/mcp-server ./cmd/mcp
```

## License

AGPLv3 - See LICENSE file for details.