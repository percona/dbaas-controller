package logger

import "context"

// key is unexported to prevent collisions - it is different from any other type in other packages
//nolint:gochecknoglobals
var key = struct{}{}

// Get returns logger from given context produced by GetCtxWithLogger.
func Get(ctx context.Context) Logger {
	v := ctx.Value(key)
	if v == nil {
		l := NewLogger()
		return l
	}

	return v.(Logger)
}

// GetCtxWithLogger returns derived context with given logger set.
// If logger is already present, it will be shadowed.
func GetCtxWithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, key, l)
}
