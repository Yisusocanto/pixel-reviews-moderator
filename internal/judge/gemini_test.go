package judge

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiJudge_Evaluate(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name         string
		serverStatus int
		serverResponse any
		wantVerdict  Verdict
		wantReason   string
		wantErr      bool
	}{
		{
			name:         "approves clean review with low scores",
			serverStatus: http.StatusOK,
			serverResponse: ModelResponse{
				Answers: map[string]QuestionResponse{
					"self_harm_or_threat": {Type: "noul", Noul: 0.02, Confidence: 0.95},
					"hate_or_harassment":  {Type: "noul", Noul: 0.05, Confidence: 0.90},
					"spam_or_scam":        {Type: "noul", Noul: 0.01, Confidence: 0.99},
				},
			},
			wantVerdict: Approve,
			wantReason:  "",
		},
		{
			name:         "rejects self harm when score exceeds reject threshold",
			serverStatus: http.StatusOK,
			serverResponse: ModelResponse{
				Answers: map[string]QuestionResponse{
					"self_harm_or_threat": {Type: "noul", Noul: 0.72, Confidence: 0.88},
					"hate_or_harassment":  {Type: "noul", Noul: 0.05, Confidence: 0.90},
					"spam_or_scam":        {Type: "noul", Noul: 0.01, Confidence: 0.99},
				},
			},
			wantVerdict: Reject,
			wantReason:  "self_harm_or_threat",
		},
		{
			name:         "flags doubt for self harm when score is between doubt and reject threshold",
			serverStatus: http.StatusOK,
			serverResponse: ModelResponse{
				Answers: map[string]QuestionResponse{
					"self_harm_or_threat": {Type: "noul", Noul: 0.35, Confidence: 0.80},
					"hate_or_harassment":  {Type: "noul", Noul: 0.02, Confidence: 0.95},
					"spam_or_scam":        {Type: "noul", Noul: 0.03, Confidence: 0.95},
				},
			},
			wantVerdict: Doubt,
			wantReason:  "self_harm_or_threat",
		},
		{
			name:         "rejects hate speech when exceeding threshold",
			serverStatus: http.StatusOK,
			serverResponse: ModelResponse{
				Answers: map[string]QuestionResponse{
					"self_harm_or_threat": {Type: "noul", Noul: 0.01, Confidence: 0.99},
					"hate_or_harassment":  {Type: "noul", Noul: 0.85, Confidence: 0.92},
					"spam_or_scam":        {Type: "noul", Noul: 0.05, Confidence: 0.90},
				},
			},
			wantVerdict: Reject,
			wantReason:  "hate_or_harassment",
		},
		{
			name:         "rejects spam when exceeding threshold",
			serverStatus: http.StatusOK,
			serverResponse: ModelResponse{
				Answers: map[string]QuestionResponse{
					"self_harm_or_threat": {Type: "noul", Noul: 0.01, Confidence: 0.99},
					"hate_or_harassment":  {Type: "noul", Noul: 0.02, Confidence: 0.95},
					"spam_or_scam":        {Type: "noul", Noul: 0.88, Confidence: 0.94},
				},
			},
			wantVerdict: Reject,
			wantReason:  "spam_or_scam",
		},
		{
			name:         "returns error on api failure 500",
			serverStatus: http.StatusInternalServerError,
			serverResponse: map[string]string{"error": "internal error"},
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-goog-api-key") != "test-key" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(tc.serverStatus)
				_ = json.NewEncoder(w).Encode(tc.serverResponse)
			}))
			defer server.Close()

			j := NewGeminiJudge("test-key", server.URL, "gemini-decision-1", logger)
			res, err := j.Evaluate(ctx, Content{Title: "Title", Body: "Review text"})

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %q, want %q", res.Verdict, tc.wantVerdict)
			}
			if res.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", res.Reason, tc.wantReason)
			}
		})
	}
}
