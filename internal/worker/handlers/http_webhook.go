package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const maxOutputBytes = 4 * 1024 // 4 KB cap on stored response body

// HTTPWebhookHandler calls an HTTP/HTTPS URL with the job payload.
type HTTPWebhookHandler struct {
	client *http.Client
}

// NewHTTPWebhookHandler builds an HTTPWebhookHandler with SSRF protection.
func NewHTTPWebhookHandler() *HTTPWebhookHandler {
	transport := &http.Transport{
		DialContext: ssrfGuardDialer(),
		// Disable keep-alives to avoid connection pooling issues in tests.
		DisableKeepAlives: false,
	}
	return &HTTPWebhookHandler{
		client: &http.Client{
			Transport: transport,
			// No global timeout here; the caller passes a context with deadline.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Re-validate after each redirect to catch DNS rebinding.
				if err := guardHost(req.URL.Hostname()); err != nil {
					return err
				}
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// Execute calls the webhook URL specified in the payload.
// The runID is sent as Idempotency-Key so the target can deduplicate.
func (h *HTTPWebhookHandler) Execute(ctx context.Context, runID string, payload map[string]any) (Result, error) {
	rawURL, _ := payload["url"].(string)
	method, _ := payload["method"].(string)
	if method == "" {
		method = "POST"
	}

	if rawURL == "" {
		return Result{}, fmt.Errorf("payload.url is required")
	}

	var bodyReader io.Reader
	if body, ok := payload["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), rawURL, bodyReader)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}

	// Standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "JobScheduler/1.0")
	req.Header.Set("Idempotency-Key", runID) // dedup for the target service

	// Custom headers from payload
	if headers, ok := payload["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	start := time.Now()
	resp, err := h.client.Do(req)
	_ = start
	if err != nil {
		return Result{}, fmt.Errorf("http call failed: %w", err)
	}
	defer resp.Body.Close()

	// Read up to maxOutputBytes of the response body.
	limited := io.LimitReader(resp.Body, maxOutputBytes)
	bodyBytes, _ := io.ReadAll(limited)
	output := string(bodyBytes)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Result{
			Output:     output,
			HTTPStatus: resp.StatusCode,
		}, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	return Result{
		Output:     output,
		HTTPStatus: resp.StatusCode,
	}, nil
}

// ssrfGuardDialer returns a DialContext that blocks private/loopback IPs.
func ssrfGuardDialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if err := guardHost(host); err != nil {
			return nil, err
		}
		return d.DialContext(ctx, network, addr)
	}
}

// guardHost resolves the host and rejects private/loopback IPs.
func guardHost(host string) error {
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed for %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return fmt.Errorf("SSRF: target IP %s is not allowed", addr)
		}
	}
	return nil
}
