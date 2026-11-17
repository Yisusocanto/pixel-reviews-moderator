package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yisusocanto/pixel-reviews-moderator/internal/judge"
)

func TestClient_SendVerdict(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	sampleReq := VerdictRequest{
		ID:      "01a1ea85-e118-72b8-bff6-15ac1418c625",
		Type:    "review",
		Version: 1,
		Verdict: judge.Approve,
		Judge:   "ai_model",
		Scores:  map[string]float64{"spam": 0.01},
	}

	t.Run("success returns verdict response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/internal/moderation/verdicts" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("X-Service-Key") != "secret123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
			}

			var req VerdictRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decoding request: %v", err)
			}
			if req.ID != sampleReq.ID || req.Verdict != sampleReq.Verdict {
				t.Errorf("unexpected body: %+v", req)
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(VerdictResponse{Result: "applied"})
		}))
		defer server.Close()

		client := NewClient(server.URL, "secret123", logger)
		resp, err := client.SendVerdict(ctx, sampleReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Result != "applied" {
			t.Errorf("Result = %q, want %q", resp.Result, "applied")
		}
	})

	t.Run("returns ErrNotFound on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient(server.URL, "secret123", logger)
		_, err := client.SendVerdict(ctx, sampleReq)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("returns ErrUnauthorized on 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewClient(server.URL, "wrong-key", logger)
		_, err := client.SendVerdict(ctx, sampleReq)
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("returns generic error on 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("database connection lost"))
		}))
		defer server.Close()

		client := NewClient(server.URL, "secret123", logger)
		_, err := client.SendVerdict(ctx, sampleReq)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
