package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yisusocanto/pixel-reviews-moderator/internal/apiclient"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/judge"
)

type mockJudge struct {
	result judge.Result
	err    error
}

func (m *mockJudge) Name() string { return "mock" }
func (m *mockJudge) Evaluate(_ context.Context, _ judge.Content) (judge.Result, error) {
	if m.err != nil {
		return judge.Result{}, m.err
	}
	return m.result, nil
}

func TestModerationHandler_Handle(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	validPayload, _ := json.Marshal(ModerationEvent{
		ID:      "01a1ea85-e118-72b8-bff6-15ac1418c625",
		Version: 1,
		Type:    "review",
		Title:   "Juego genial",
		Content: "Me encanto todo",
	})

	t.Run("invalid json returns ErrDiscard", func(t *testing.T) {
		p := judge.NewPipeline(&mockJudge{result: judge.Result{Verdict: judge.Approve}})
		client := apiclient.NewClient("http://localhost", "secret", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, []byte("not valid json"))
		if !errors.Is(err, ErrDiscard) {
			t.Errorf("expected ErrDiscard, got %v", err)
		}
	})

	t.Run("pipeline failure returns ErrRequeue", func(t *testing.T) {
		p := judge.NewPipeline(&mockJudge{err: errors.New("pipeline crashed")})
		client := apiclient.NewClient("http://localhost", "secret", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, validPayload)
		if !errors.Is(err, ErrRequeue) {
			t.Errorf("expected ErrRequeue, got %v", err)
		}
	})

	t.Run("api not found returns ErrDiscard", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		p := judge.NewPipeline(&mockJudge{result: judge.Result{Verdict: judge.Approve, Judge: "mock"}})
		client := apiclient.NewClient(server.URL, "secret", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, validPayload)
		if !errors.Is(err, ErrDiscard) {
			t.Errorf("expected ErrDiscard, got %v", err)
		}
	})

	t.Run("api unauthorized returns ErrRequeue", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		p := judge.NewPipeline(&mockJudge{result: judge.Result{Verdict: judge.Approve, Judge: "mock"}})
		client := apiclient.NewClient(server.URL, "wrong-key", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, validPayload)
		if !errors.Is(err, ErrRequeue) {
			t.Errorf("expected ErrRequeue, got %v", err)
		}
	})

	t.Run("api 500 error returns ErrRequeue", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		p := judge.NewPipeline(&mockJudge{result: judge.Result{Verdict: judge.Approve, Judge: "mock"}})
		client := apiclient.NewClient(server.URL, "secret", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, validPayload)
		if !errors.Is(err, ErrRequeue) {
			t.Errorf("expected ErrRequeue, got %v", err)
		}
	})

	t.Run("successful processing returns nil", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(apiclient.VerdictResponse{Result: "applied"})
		}))
		defer server.Close()

		p := judge.NewPipeline(&mockJudge{result: judge.Result{Verdict: judge.Approve, Judge: "mock"}})
		client := apiclient.NewClient(server.URL, "secret", logger)
		h := NewModerationHandler(p, client, logger)

		err := h.Handle(ctx, validPayload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
