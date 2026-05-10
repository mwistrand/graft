package render

import (
	"github.com/mwistrand/graft/internal/provider"
)

// deltaRenderer renders the AI summary/ordering panels. Per-file diff
// rendering happens in the TUI (which spawns delta directly), so for the
// pre-review prose output it delegates to the same formatter as the fallback.
type deltaRenderer struct {
	fallback *fallbackRenderer
}

func newDeltaRenderer(_ string, opts Options) *deltaRenderer {
	return &deltaRenderer{
		fallback: newFallbackRenderer(opts),
	}
}

// RenderSummary displays the AI-generated summary.
func (r *deltaRenderer) RenderSummary(summary *provider.SummarizeResponse) error {
	return r.fallback.RenderSummary(summary)
}

// RenderOrdering displays the file ordering with reasoning.
func (r *deltaRenderer) RenderOrdering(order *provider.OrderResponse) error {
	return r.fallback.RenderOrdering(order)
}
