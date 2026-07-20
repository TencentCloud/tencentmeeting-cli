// Package apicmdctx stores the resolved ApiCmd identifier on a
// context.Context so downstream layers (notably the REST proxy) can read it
// without depending on cobra command annotations.
//
// It intentionally has no dependency on other internal packages so both
// internal/cmdutil sub-packages and internal/proxy/rest-proxy can import it
// safely.
package apicmdctx

import "context"

// apiCmdCtxKey is the unexported key type used to stash the resolved ApiCmd
// into a context.Context.
type apiCmdCtxKey struct{}

// Inject returns a copy of ctx that carries the given ApiCmd name.
func Inject(ctx context.Context, apiCmdName string) context.Context {
	return context.WithValue(ctx, apiCmdCtxKey{}, apiCmdName)
}

// Get returns the ApiCmd stored in ctx, or an empty string when no ApiCmd
// is bound.
func Get(ctx context.Context) string {
	if name, ok := ctx.Value(apiCmdCtxKey{}).(string); ok {
		return name
	}
	return ""
}
