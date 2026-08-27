package lineage

import "context"

func abortFresh() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ctx.Err()
	if err != nil {
		return ctx
	}
	return context.Background()
}
