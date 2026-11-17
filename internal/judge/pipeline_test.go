package judge

import (
	"context"
	"errors"
	"testing"
)

type mockJudge struct {
	name      string
	result    Result
	err       error
	callCount int
}

func (m *mockJudge) Name() string {
	return m.name
}

func (m *mockJudge) Evaluate(_ context.Context, _ Content) (Result, error) {
	m.callCount++
	if m.err != nil {
		return Result{}, m.err
	}
	return m.result, nil
}

func TestPipeline_Evaluate(t *testing.T) {
	ctx := context.Background()
	content := Content{Title: "Title", Body: "Body"}

	t.Run("empty pipeline approves", func(t *testing.T) {
		p := NewPipeline()
		res, err := p.Evaluate(ctx, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Verdict != Approve {
			t.Errorf("Verdict = %v, want %v", res.Verdict, Approve)
		}
	})

	t.Run("all judges approve", func(t *testing.T) {
		j1 := &mockJudge{name: "j1", result: Result{Verdict: Approve, Judge: "j1"}}
		j2 := &mockJudge{name: "j2", result: Result{Verdict: Approve, Judge: "j2"}}

		p := NewPipeline(j1, j2)
		res, err := p.Evaluate(ctx, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Verdict != Approve {
			t.Errorf("Verdict = %v, want %v", res.Verdict, Approve)
		}
		if j1.callCount != 1 || j2.callCount != 1 {
			t.Errorf("expected both judges called once, got j1=%d, j2=%d", j1.callCount, j2.callCount)
		}
	})

	t.Run("early reject short-circuits remaining judges", func(t *testing.T) {
		j1 := &mockJudge{name: "j1", result: Result{Verdict: Reject, Reason: "spam", Judge: "j1"}}
		j2 := &mockJudge{name: "j2", result: Result{Verdict: Approve, Judge: "j2"}}

		p := NewPipeline(j1, j2)
		res, err := p.Evaluate(ctx, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Verdict != Reject || res.Reason != "spam" {
			t.Errorf("expected Reject/spam, got %v/%v", res.Verdict, res.Reason)
		}
		if j1.callCount != 1 {
			t.Errorf("expected j1 called once, got %d", j1.callCount)
		}
		if j2.callCount != 0 {
			t.Errorf("expected j2 NOT called, got %d", j2.callCount)
		}
	})

	t.Run("first doubt kept if subsequent judges approve", func(t *testing.T) {
		j1 := &mockJudge{name: "j1", result: Result{Verdict: Doubt, Reason: "caps", Judge: "j1"}}
		j2 := &mockJudge{name: "j2", result: Result{Verdict: Approve, Judge: "j2"}}

		p := NewPipeline(j1, j2)
		res, err := p.Evaluate(ctx, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Verdict != Doubt || res.Reason != "caps" {
			t.Errorf("expected Doubt/caps, got %v/%v", res.Verdict, res.Reason)
		}
		if j1.callCount != 1 || j2.callCount != 1 {
			t.Errorf("expected both judges called, got j1=%d, j2=%d", j1.callCount, j2.callCount)
		}
	})

	t.Run("subsequent reject overrides prior doubt", func(t *testing.T) {
		j1 := &mockJudge{name: "j1", result: Result{Verdict: Doubt, Reason: "caps", Judge: "j1"}}
		j2 := &mockJudge{name: "j2", result: Result{Verdict: Reject, Reason: "hate", Judge: "j2"}}

		p := NewPipeline(j1, j2)
		res, err := p.Evaluate(ctx, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Verdict != Reject || res.Reason != "hate" {
			t.Errorf("expected Reject/hate, got %v/%v", res.Verdict, res.Reason)
		}
	})

	t.Run("judge error halts pipeline", func(t *testing.T) {
		j1 := &mockJudge{name: "j1", err: errors.New("gemini down")}
		j2 := &mockJudge{name: "j2", result: Result{Verdict: Approve, Judge: "j2"}}

		p := NewPipeline(j1, j2)
		_, err := p.Evaluate(ctx, content)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if j2.callCount != 0 {
			t.Errorf("expected j2 NOT called after error, got %d", j2.callCount)
		}
	})
}
