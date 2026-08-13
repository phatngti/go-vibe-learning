package gatewayauth

import "context"

type userContextKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}
