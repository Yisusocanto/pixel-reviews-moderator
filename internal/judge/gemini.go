package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type DecisionRequest struct {
	Model     string                     `json:"model"`
	Input     string                     `json:"input"`
	Questions map[string]QuestionRequest `json:"questions"`
}

type QuestionRequest struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
}

type ModelResponse struct {
	Answers map[string]QuestionResponse `json:"answers"`
}

type QuestionResponse struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Noul       float64 `json:"noul"`
}

type categoryThreshold struct {
	reject float64
	doubt  float64
}

var defaultThresholds = map[string]categoryThreshold{
	"self_harm_or_threat": {reject: 0.50, doubt: 0.20},
	"hate_or_harassment":  {reject: 0.70, doubt: 0.35},
	"spam_or_scam":        {reject: 0.75, doubt: 0.45},
}

type GeminiJudge struct {
	httpClient *http.Client
	apiKey     string
	apiURL     string
	modelName  string
	logger     *slog.Logger
}

func NewGeminiJudge(apiKey, apiURL, modelName string, logger *slog.Logger) *GeminiJudge {
	if logger == nil {
		logger = slog.Default()
	}
	return &GeminiJudge{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		apiURL:     apiURL,
		modelName:  modelName,
		logger:     logger.With(slog.String("component", "gemini_judge")),
	}
}

func (g *GeminiJudge) Name() string {
	return "ai_model"
}

func (g *GeminiJudge) Evaluate(ctx context.Context, content Content) (Result, error) {
	input := content.Body
	if content.Title != "" {
		input = fmt.Sprintf("Review Title: %s\nReview Body: %s\n", content.Title, content.Body)
	}

	requestBody := DecisionRequest{
		Model: g.modelName,
		Input: input,
		Questions: map[string]QuestionRequest{
			"hate_or_harassment": {
				Type:         "noul",
				Instructions: "Does the text contain hate speech, slurs, discrimination, personal harassment, or direct attacks against people, players, developers, or protected groups? Criticizing or disliking the game mechanics, story, or content itself is acceptable.",
			},
			"self_harm_or_threat": {
				Type:         "noul",
				Instructions: "Does the text express personal suicidal thoughts, intent or desire to commit suicide or self-harm (such as 'me quiero suicidar', 'ojala morir', 'un arma en la cabeza', 'want to kill myself'), encourage suicide, or make violent threats against real people? Fictional video game events (e.g., 'my character died in the dungeon') are acceptable, but any personal expression of wanting to die, commit suicide, or harm oneself must be answered YES.",
			},
			"spam_or_scam": {
				Type:         "noul",
				Instructions: "Is the text commercial advertising, promotional spam, selling accounts or currency, phishing, suspicious links, or nonsensical gibberish completely unrelated to reviewing the game?",
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return Result{}, fmt.Errorf("marshaling gemini request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return Result{}, fmt.Errorf("creating gemini request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	res, err := g.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("calling gemini api: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			g.logger.Warn("failed to close gemini response body", slog.Any("error", closeErr))
		}
	}()

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return Result{}, fmt.Errorf("gemini api returned status %d: %s", res.StatusCode, string(errBody))
	}

	out := &ModelResponse{}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return Result{}, fmt.Errorf("decoding gemini response: %w", err)
	}

	scores := make(map[string]float64, len(out.Answers))
	for question, answer := range out.Answers {
		scores[question] = answer.Noul
	}

	g.logger.Info("gemini evaluation completed", slog.Any("scores", scores))

	questionOrder := []string{"self_harm_or_threat", "hate_or_harassment", "spam_or_scam"}
	var doubtReason string

	for _, question := range questionOrder {
		answer, exists := out.Answers[question]
		if !exists {
			continue
		}

		th, hasThreshold := defaultThresholds[question]
		if !hasThreshold {
			th = categoryThreshold{reject: 0.75, doubt: 0.45}
		}

		if answer.Noul >= th.reject {
			return Result{
				Verdict: Reject,
				Judge:   g.Name(),
				Reason:  question,
				Scores:  scores,
			}, nil
		}

		if answer.Noul >= th.doubt && doubtReason == "" {
			doubtReason = question
		}
	}

	if doubtReason != "" {
		return Result{
			Reason:  doubtReason,
			Judge:   g.Name(),
			Verdict: Doubt,
			Scores:  scores,
		}, nil
	}

	return Result{
		Reason:  "",
		Judge:   g.Name(),
		Verdict: Approve,
		Scores:  scores,
	}, nil
}
