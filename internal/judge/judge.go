package judge

import "context"

type Verdict string

const (
	Approve Verdict = "approve"
	Reject  Verdict = "reject"
	Doubt   Verdict = "doubt"
)

type Content struct {
	Title string
	Body  string
}

type Result struct {
	Reason  string
	Judge   string
	Verdict Verdict
	Scores  map[string]float64
}

type Judge interface {
	Name() string
	Evaluate(ctx context.Context, content Content) (Result, error)
}
