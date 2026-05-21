package memoryflow

import "context"

// PassthroughRecallStrategy is the default recall strategy. It delegates to
// the next recall function unchanged.
type PassthroughRecallStrategy struct{}

// Recall delegates unchanged.
func (PassthroughRecallStrategy) Recall(ctx context.Context, req RecallRequest, next RecallFunc) (*RecallResponse, error) {
	return next(ctx, req)
}

// RecallFunc is the callback for the next recall step in the chain.
type RecallFunc func(ctx context.Context, req RecallRequest) (*RecallResponse, error)

// RecallStrategy wraps or enriches recall requests.
type RecallStrategy interface {
	Recall(ctx context.Context, req RecallRequest, next RecallFunc) (*RecallResponse, error)
}
