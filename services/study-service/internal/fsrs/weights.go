package fsrs

import (
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

func DefaultParams() gofsrs.Parameters {
	return gofsrs.DefaultParam()
}

func ParamsFromWeights(weights []float64) gofsrs.Parameters {
	p := gofsrs.DefaultParam()
	if len(weights) == 21 {
		var w gofsrs.Weights
		copy(w[:], weights)
		p.W = w
	}
	return p
}
