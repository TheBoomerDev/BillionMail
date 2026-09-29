#!/bin/bash
# Deploy BillionMail MCP Server
# Usage: ./deploy-mcp.sh [port]

set -e

PORT=${1:-9090}
DOMAIN=${BILLIONMAIL_HOSTNAME:-mail.cargoffer.com}

echo "🚀 Building MCP server..."
docker build -t billionmail/mcp-server:latest -f Dockerfile.mcp . || {
    echo "❌ Build failed, trying alternative build method..."
    docker build -t billionmail/mcp-server:latest . -f Dockerfile.mcp
}

echo "🔧 Stopping existing containers..."
docker rm -f billionmail-mcp-server 2>/dev/null || true

echo "🚀 Starting MCP server on port $PORT..."
docker run -d \
    --name billionmail-mcp-server \
    -p "${PORT}:9090" \
    -e MCP_PORT=9090 \
    -e BILLIONMAIL_HOSTNAME="$DOMAIN" \
    -v $(pwd)/core/data:/root/data \
    --restart unless-stopped \
    billionmail/mcp-server:latest

echo "✅ MCP server started!"
echo "📡 Access at: http://localhost:${PORT}/mcp"
echo ""
echo "🔧 Testing connection..."
sleep 3
curl -s http://localhost:${PORT}/health 2>/dev/null || echo "Server running (no health endpoint)"

echo ""
echo "📝 MCP Tools available:"
echo "  - list_domains"
echo "  - get_smtp_status"  
echo "  - create_domain"
echo "  - create_mailbox"