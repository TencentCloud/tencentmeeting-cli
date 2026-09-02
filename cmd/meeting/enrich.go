package meeting

import (
	"context"
	middleWare "tmeet/internal/cmdutil/middleware"
)

// compactFieldsWithAllEnrichments returns the remote compact whitelist from
// ctx with both recordEnrichmentFields and minuteEnrichmentFields appended,
// so the client-side injected record and minute subtrees survive
// output.WithCompact trimming (the remote schema does not know about those
// fields).
//
// The result is always a freshly allocated slice: the slice returned by
// middleware.GetCompactFields is backed by ctx and shared across callers, so
// appending to it in place could corrupt other readers.
func compactFieldsWithAllEnrichments(ctx context.Context) []string {
	base := middleWare.GetCompactFields(ctx)
	merged := make([]string, 0, len(base)+len(recordEnrichmentFields)+len(minuteEnrichmentFields))
	merged = append(merged, base...)
	merged = append(merged, recordEnrichmentFields...)
	merged = append(merged, minuteEnrichmentFields...)
	return merged
}
