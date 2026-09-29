// Package mcp provides an MCP (Model Context Protocol) server for BillionMail.
// It is self-contained: it talks to the BillionMail core REST API over HTTP,
// so it can run beside the core container without importing internal packages.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config for connecting to the BillionMail core API
type Config struct {
	BaseURL  string // e.g. http://core:80 or http://127.0.0.1:8888
	Username string
	Password string
}

// Client holds an authenticated session against the core API
type Client struct {
	cfg    Config
	http   *http.Client
	token  string
	cookie string
}

// NewClientFromEnv builds a client from BILLIONMAIL_URL, BM_USERNAME, BM_PASSWORD env vars
func NewClientFromEnv() *Client {
	return &Client{
		cfg: Config{
			BaseURL:  envOr("BILLIONMAIL_URL", "http://core:80"),
			Username: envOr("BM_USERNAME", "billion"),
			Password: os.Getenv("BM_PASSWORD"),
		},
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Login authenticates and stores the JWT token
func (c *Client) Login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	})
	// Visit safepath first to obtain the session cookie (core requirement).
	// Use a non-redirecting request so we read Set-Cookie from the 302 itself.
	jar := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, _ := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/bm-admin", nil)
	resp, err := jar.Do(req)
	if err == nil {
		for _, ck := range resp.Cookies() {
			if ck.Name == "billion_mail" {
				c.cookie = ck.Name + "=" + ck.Value
			}
		}
		resp.Body.Close()
	}

	req, _ = http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/api/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	resp, err = c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("login decode failed: %w", err)
	}
	if !out.Success || out.Data.Token == "" {
		return fmt.Errorf("login failed: %s", out.Msg)
	}
	c.token = out.Data.Token
	return nil
}

// api performs an authenticated request against the core API
func (c *Client) api(ctx context.Context, method, path string, payload any, out any) error {
	if c.token == "" {
		if err := c.Login(ctx); err != nil {
			return err
		}
	}
	var rd io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		// token expired: re-login once and retry
		c.token = ""
		if err := c.Login(ctx); err != nil {
			return err
		}
		return c.api(ctx, method, path, payload, out)
	}
	body, _ := io.ReadAll(resp.Body)
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

// DomainInfo mirrors a domain row from the core API
type DomainInfo struct {
	Domain    string `json:"domain"`
	ARecord   string `json:"a_record"`
	Active    int    `json:"active"`
	Mailboxes int    `json:"mailboxes"`
}

// ListDomains lists all configured mail domains
func (c *Client) ListDomains(ctx context.Context) ([]DomainInfo, error) {
	var out struct {
		Success bool `json:"success"`
		Data    []struct {
			Domain  string `json:"domain"`
			ARecord string `json:"a_record"`
			Active  int    `json:"active"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := c.api(ctx, "GET", "/api/domains/all", nil, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("list domains failed: %s", out.Msg)
	}
	domains := make([]DomainInfo, 0, len(out.Data))
	for _, d := range out.Data {
		domains = append(domains, DomainInfo{Domain: d.Domain, ARecord: d.ARecord, Active: d.Active})
	}
	return domains, nil
}

// CreateDomain creates a new mail domain
func (c *Client) CreateDomain(ctx context.Context, domain, aRecord string) (map[string]any, error) {
	payload := map[string]any{"domain": domain}
	if aRecord != "" {
		payload["a_record"] = aRecord
	}
	var out map[string]any
	if err := c.api(ctx, "POST", "/api/domains/create", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteDomain removes a mail domain
func (c *Client) DeleteDomain(ctx context.Context, domain string) (map[string]any, error) {
	var out map[string]any
	if err := c.api(ctx, "POST", "/api/domains/delete", map[string]any{"domain": domain}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateMailbox creates a new mailbox under a domain
func (c *Client) CreateMailbox(ctx context.Context, username, domain, password string) (map[string]any, error) {
	payload := map[string]any{
		"local_part":   username,
		"domain":       domain,
		"password":     password,
		"full_name":    username,
		"active":       1,
		"isAdmin":      0,
		"quota":        5242880,
		"quota_active": 1,
	}
	var out map[string]any
	if err := c.api(ctx, "POST", "/api/mailbox/create", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListMailboxes lists mailboxes, optionally filtered by domain
func (c *Client) ListMailboxes(ctx context.Context, domain string) (map[string]any, error) {
	payload := map[string]any{}
	if strings.TrimSpace(domain) != "" {
		payload["domain"] = domain
	}
	var out map[string]any
	if err := c.api(ctx, "GET", "/api/mailbox/all", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetServiceStatus returns mail service status (SMTP/IMAP/POP)
func (c *Client) GetServiceStatus(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.api(ctx, "GET", "/api/overview", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetOverview returns platform overview stats
func (c *Client) GetOverview(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.api(ctx, "GET", "/api/overview", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
