package judge

import "context"

type Pipeline struct {
	judges []Judge
}

func NewPipeline(judges ...Judge) *Pipeline {
	return &Pipeline{judges: judges}
}

func (p *Pipeline) Evaluate(ctx context.Context, content Content) (*Result, error) {
	var firstDoubt *Result
	lastResult := Result{Verdict: Approve, Judge: "system"}

	for _, j := range p.judges {
		result, err := j.Evaluate(ctx, content)
		if err != nil {
			return nil, err
		}

		switch result.Verdict {
		case Reject:
			return &result, nil
		case Doubt:
			if firstDoubt == nil {
				r := result
				firstDoubt = &r
			}
		case Approve:
			lastResult = result
		}
	}

	if firstDoubt != nil {
		return firstDoubt, nil
	}

	return &lastResult, nil
}
