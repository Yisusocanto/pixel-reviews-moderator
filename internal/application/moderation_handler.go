package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/Yisusocanto/pixel-reviews-moderator/internal/apiclient"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/judge"
)

var (
	ErrDiscard = errors.New("permanent error: discard message")
	ErrRequeue = errors.New("temporary error: requeue message")
)

type ModerationEvent struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
}

type ModerationHandler struct {
	pipeline  *judge.Pipeline
	apiClient *apiclient.Client
	logger    *slog.Logger
}

func NewModerationHandler(
	pipeline *judge.Pipeline,
	apiClient *apiclient.Client,
	logger *slog.Logger,
) *ModerationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ModerationHandler{
		pipeline:  pipeline,
		apiClient: apiClient,
		logger:    logger.With(slog.String("component", "moderation_handler")),
	}
}

func (m *ModerationHandler) Handle(ctx context.Context, data []byte) error {
	var event ModerationEvent
	if err := json.Unmarshal(data, &event); err != nil {
		m.logger.Error("invalid moderation event json", slog.Any("error", err))
		return ErrDiscard
	}

	log := m.logger.With(
		slog.String("target_type", event.Type),
		slog.String("target_id", event.ID),
		slog.Int("version", event.Version),
	)

	content := judge.Content{
		Title: event.Title,
		Body:  event.Content,
	}

	res, err := m.pipeline.Evaluate(ctx, content)
	if err != nil {
		log.Error("moderation pipeline failed", slog.Any("error", err))
		return ErrRequeue
	}

	verdictReq := apiclient.VerdictRequest{
		ID:      event.ID,
		Type:    event.Type,
		Version: event.Version,
		Reason:  res.Reason,
		Verdict: res.Verdict,
		Judge:   res.Judge,
		Scores:  res.Scores,
	}

	apiRes, err := m.apiClient.SendVerdict(ctx, verdictReq)
	if err != nil {
		if errors.Is(err, apiclient.ErrNotFound) {
			log.Warn("target no longer exists in api; discarding message")
			return ErrDiscard
		}
		if errors.Is(err, apiclient.ErrUnauthorized) {
			log.Error("api rejected service key; check MODERATION_SERVICE_KEY")
			return ErrRequeue
		}
		log.Error("failed to send verdict to api", slog.Any("error", err))
		return ErrRequeue
	}

	log.Info(
		"moderation verdict delivered",
		slog.String("verdict", string(res.Verdict)),
		slog.String("judge", res.Judge),
		slog.String("reason", res.Reason),
		slog.String("api_result", apiRes.Result),
	)

	return nil
}
