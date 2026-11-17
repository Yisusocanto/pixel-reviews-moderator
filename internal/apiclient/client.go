package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Yisusocanto/pixel-reviews-moderator/internal/judge"
)

var (
	ErrNotFound     = errors.New("target not found")
	ErrUnauthorized = errors.New("invalid service key")
)

type VerdictRequest struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Version int                `json:"version"`
	Reason  string             `json:"reason,omitempty"`
	Verdict judge.Verdict      `json:"verdict"`
	Judge   string             `json:"judge"`
	Scores  map[string]float64 `json:"scores"`
}

type VerdictResponse struct {
	Result string `json:"result"`
}

type Client struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(baseURL, serviceKey string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		serviceKey: serviceKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger.With(slog.String("component", "api_client")),
	}
}

func (c *Client) SendVerdict(ctx context.Context, request VerdictRequest) (*VerdictResponse, error) {
	bodyBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshaling verdict request: %w", err)
	}

	endpoint := c.baseURL + "/internal/moderation/verdicts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating verdict request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Key", c.serviceKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending verdict request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close verdict response body", slog.Any("error", closeErr))
		}
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		out := &VerdictResponse{}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return nil, fmt.Errorf("decoding verdict response: %w", err)
		}
		return out, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	default:
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(errBody))
	}
}
