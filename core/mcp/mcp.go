// Package mcp provides an MCP (Model Context Protocol) server for BillionMail
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"billionmail-core/internal/controller/domains"
	"billionmail-core/internal/controller/mail_boxes"
	"billionmail-core/internal/controller/mail_services"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

// MCPServer wraps BillionMail services for MCP protocol
type MCPServer struct {
	domains    domains.IDomainsV1
	mailBoxes  mail_boxes.IMailBoxesV1
	mailSvcs   mail_services.IMailServicesV1
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() *MCPServer {
	return &MCPServer{
		domains:   domains.NewV1(),
		mailBoxes: mail_boxes.NewV1(),
		mailSvcs:  mail_services.NewV1(),
	}
}

// ListDomains lists all configured mail domains
func (m *MCPServer) ListDomains(ctx context.Context) ([]DomainInfo, error) {
	res, err := m.domains.GetDomainAll(ctx, &domains.GetDomainAllReq{})
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}
	var domains []DomainInfo
	if res.Data != nil {
		data, _ := json.Marshal(res.Data)
		json.Unmarshal(data, &domains)
	}
	return domains, nil
}

// DomainInfo represents domain information for MCP
type DomainInfo struct {
	Domain     string `json:"domain"`
	A_Record   string `json:"a_record"`
	Active     bool   `json:"active"`
	Mailboxes  int    `json:"mailboxes_count"`
	DKIMStatus string `json:"dkim_status"`
}

// MailboxInfo represents mailbox information
type MailboxInfo struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

// CreateDomainParams parameters for creating a domain
type CreateDomainParams struct {
	Domain    string `json:"domain"`
	A_Record  string `json:"a_record"`
	Active    bool   `json:"active"`
	AutoDKIM  bool   `json:"auto_dkim"` // Generate DKIM automatically
}

// CreateDomain creates a new mail domain
func (m *MCPServer) CreateDomain(ctx context.Context, params CreateDomainParams) (DomainInfo, error) {
	req := &domains.AddDomainReq{
		Domain: params.Domain,
		A_Record: params.A_Record,
		Active: params.Active,
	}
	res, err := m.domains.AddDomain(ctx, req)
	if err != nil {
		return DomainInfo{}, fmt.Errorf("failed to create domain: %w", err)
	}
	return DomainInfo{
		Domain:   params.Domain,
		A_Record: params.A_Record,
		Active:   params.Active,
	}, nil
}

// CreateMailbox creates a new mailbox
func (m *MCPServer) CreateMailbox(ctx context.Context, username, domain, password string) (MailboxInfo, error) {
	req := &mail_boxes.AddOneReq{
		Username: username,
		Domain:   domain,
		Password: password,
	}
	res, err := m.mailBoxes.AddOne(ctx, req)
	if err != nil {
		return MailboxInfo{}, fmt.Errorf("failed to create mailbox: %w", err)
	}
	return MailboxInfo{
		Username: username,
		Domain:   domain,
		Email:    fmt.Sprintf("%s@%s", username, domain),
		Status:   "active",
	}, nil
}

// GetSMTPStatus returns SMTP server status
func (m *MCPServer) GetSMTPStatus(ctx context.Context) (*SMTPStatus, error) {
	res, err := m.mailSvcs.GetSMTPStatus(ctx, &mail_services.GetSMTPStatusReq{})
	if err != nil {
		return nil, fmt.Errorf("failed to get SMTP status: %w", err)
	}
	return &SMTPStatus{
		Port:         25,
		Status:       "listening",
		ActiveDomains: res.Data.ActiveDomains,
	}, nil
}

// SMTPStatus represents SMTP server status
type SMTPStatus struct {
	Port          int `json:"port"`
	Status        string `json:"status"`
	ActiveDomains int   `json:"active_domains"`
}