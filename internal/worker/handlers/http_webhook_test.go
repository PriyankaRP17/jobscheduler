package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestHandler returns an HTTPWebhookHandler with the SSRF guard disabled.
// Only for use in tests that spin up httptest.Server on 127.0.0.1.
func newTestHandler() *HTTPWebhookHandler {
	return &HTTPWebhookHandler{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func TestHTTPWebhookHandler_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing Idempotency-Key header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	h := newTestHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.Execute(ctx, "run_test_001", map[string]any{
		"url":    srv.URL + "/webhook",
		"method": "POST",
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.HTTPStatus != 200 {
		t.Errorf("expected 200, got %d", result.HTTPStatus)
	}
	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestHTTPWebhookHandler_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`service down`))
	}))
	defer srv.Close()

	h := newTestHandler()
	ctx := context.Background()

	_, err := h.Execute(ctx, "run_002", map[string]any{"url": srv.URL, "method": "GET"})
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
}

func TestHTTPWebhookHandler_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newTestHandler()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := h.Execute(ctx, "run_003", map[string]any{"url": srv.URL, "method": "GET"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPWebhookHandler_MissingURL(t *testing.T) {
	h := newTestHandler()
	_, err := h.Execute(context.Background(), "run_004", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestHTTPWebhookHandler_InvalidScheme(t *testing.T) {
	// Use the real handler — ftp:// fails at http.NewRequest, before SSRF check.
	h := NewHTTPWebhookHandler()
	_, err := h.Execute(context.Background(), "run_005", map[string]any{
		"url": "ftp://example.com/file",
	})
	if err == nil {
		t.Fatal("expected error for ftp:// scheme")
	}
}

// TestSSRFGuard_BlocksPrivateIP confirms the production handler rejects private IPs.
func TestSSRFGuard_BlocksPrivateIP(t *testing.T) {
	h := NewHTTPWebhookHandler()
	ctx := context.Background()

	privateURLs := []string{
		"http://127.0.0.1/secret",
		"http://192.168.1.1/admin",
		"http://10.0.0.1/internal",
		"http://169.254.169.254/latest/meta-data/",
	}
	for _, u := range privateURLs {
		_, err := h.Execute(ctx, "run_ssrf", map[string]any{"url": u, "method": "GET"})
		if err == nil {
			t.Errorf("expected SSRF block for %s, got nil error", u)
		}
	}
}
