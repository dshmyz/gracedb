package semanticrouter

import "context"

// HybridRouter combines semantic and lexical routing.
type HybridRouter struct {
	semantic   *Router
	lexical    *LexicalRouter
	lambda     float64 // weight for semantic (0-1), 1-lambda for lexical
}

// NewHybridRouter creates a hybrid router.
func NewHybridRouter(semantic *Router, lexical *LexicalRouter, lambda float64) *HybridRouter {
	if lambda < 0 || lambda > 1 {
		lambda = 0.7
	}
	return &HybridRouter{
		semantic: semantic,
		lexical:  lexical,
		lambda:   lambda,
	}
}

// Route performs hybrid routing combining both semantic and lexical scores.
func (hr *HybridRouter) Route(ctx context.Context, text string) (*RouteResult, error) {
	var semanticResult, lexicalResult *RouteResult
	var err error

	if hr.semantic != nil {
		semanticResult, err = hr.semantic.Route(ctx, text)
		if err != nil {
			return nil, err
		}
	}
	if hr.lexical != nil {
		lexicalResult, err = hr.lexical.Route(ctx, text)
		if err != nil {
			return nil, err
		}
	}

	// Combine scores.
	semanticScore := 0.0
	semanticName := ""
	if semanticResult != nil && semanticResult.Matched {
		semanticScore = semanticResult.Score
		semanticName = semanticResult.RouteName
	}

	lexicalScore := 0.0
	lexicalName := ""
	if lexicalResult != nil && lexicalResult.Matched {
		lexicalScore = lexicalResult.Score
		lexicalName = lexicalResult.RouteName
	}

	// If both agree, boost the score.
	if semanticName == lexicalName && semanticName != "" {
		combined := hr.lambda*semanticScore + (1-hr.lambda)*lexicalScore
		return &RouteResult{
			RouteName: semanticName,
			Score:     combined,
			Matched:   combined >= hr.semantic.cfg.Threshold,
		}, nil
	}

	// Pick the higher-scored route.
	if semanticScore*hr.lambda >= lexicalScore*(1-hr.lambda) {
		return semanticResult, nil
	}
	return lexicalResult, nil
}
