// Package hindsight provides a memoryflow recall strategy plugin that enriches
// recall requests with entity and keyword cues from a memory bank.
package hindsight

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/dshmyz/gracedb/pkg/memoryflow"
	"github.com/dshmyz/gracedb/pkg/types"
)

// StrategyOptions configures the Hindsight recall strategy.
type StrategyOptions struct {
	BankID          string
	Namespace       string
	EntityNames     []string
	Keywords        []string
	RetrievalMode   string
	UseKG           bool
}

// Strategy is a memoryflow recall strategy plugin. It enriches recall requests
// with Hindsight-style bank, entity, and lexical cues, then delegates to the
// normal memoryflow recall path.
type Strategy struct {
	opts StrategyOptions
}

// NewStrategy creates a Hindsight recall strategy.
func NewStrategy(opts StrategyOptions) *Strategy {
	opts.BankID = strings.TrimSpace(opts.BankID)
	opts.Namespace = strings.TrimSpace(opts.Namespace)
	if opts.Namespace == "" && opts.BankID != "" {
		opts.Namespace = hindsightMemoryNamespace(opts.BankID)
	}
	return &Strategy{opts: opts}
}

// Recall enriches the request with Hindsight strategy cues and delegates to next.
func (s *Strategy) Recall(ctx context.Context, req memoryflow.RecallRequest, next memoryflow.RecallFunc) (*memoryflow.RecallResponse, error) {
	if next == nil {
		return nil, types.ErrEmptyText
	}
	req = s.enrichRecallRequest(req)
	return next(ctx, req)
}

func (s *Strategy) enrichRecallRequest(req memoryflow.RecallRequest) memoryflow.RecallRequest {
	if strings.TrimSpace(req.Namespace) == "" && s.opts.Namespace != "" {
		req.Namespace = s.opts.Namespace
	}

	plan := memoryflow.RetrievalPlan{Query: req.Query}
	if req.Plan != nil {
		plan = *req.Plan
		if strings.TrimSpace(plan.Query) == "" {
			plan.Query = req.Query
		}
	}
	plan.Keywords = mergeStrings(plan.Keywords, s.opts.Keywords)
	plan.Keywords = mergeStrings(plan.Keywords, req.State.Tags)
	plan.EntityNames = mergeStrings(plan.EntityNames, s.opts.EntityNames)

	if s.opts.BankID != "" {
		plan.Keywords = mergeStrings(plan.Keywords, []string{s.opts.BankID})
	}
	if req.State.Namespace != "" {
		plan.Keywords = mergeStrings(plan.Keywords, []string{req.State.Namespace})
	}

	if s.opts.RetrievalMode != "" {
		plan.RetrievalMode = s.opts.RetrievalMode
	} else if s.opts.UseKG && len(plan.EntityNames) > 0 {
		plan.RetrievalMode = "graph"
	} else if plan.RetrievalMode == "" {
		plan.RetrievalMode = "lexical"
	}

	req.Plan = &plan
	if req.RetrievalMode == "" {
		req.RetrievalMode = plan.RetrievalMode
	}
	return req
}

func hindsightMemoryNamespace(bankID string) string {
	return "hindsight:" + sanitizeBankID(bankID)
}

func sanitizeBankID(bankID string) string {
	bankID = strings.TrimSpace(strings.ToLower(bankID))
	if bankID == "" {
		return "default"
	}
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	value := strings.Trim(re.ReplaceAllString(bankID, "-"), "-_")
	if value == "" {
		value = "bank"
	}
	if len(value) <= 48 {
		return value
	}
	hash := sha1.Sum([]byte(bankID))
	return value[:40] + "-" + hex.EncodeToString(hash[:])[:8]
}

func mergeStrings(a, b []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, v)
		}
	}
	for _, v := range b {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
