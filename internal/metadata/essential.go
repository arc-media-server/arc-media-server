package metadata

import "context"

type essentialKey struct{}

// WithEssentialTV limits episode-order work during the playable-title phase.
// An empty source restores the complete provider-order behavior.
func WithEssentialTV(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, essentialKey{}, source)
}
func essentialSource(ctx context.Context) string {
	source, _ := ctx.Value(essentialKey{}).(string)
	return source
}
