// Package servers provides common servers starting code for all SaaS components.
package servers

import (
	"context"

	"github.com/google/uuid"

	"github.com/percona-platform/dbaas-controller/logger"
)

// getCtxForRequest returns derived context with request-scoped logger set, and the logger itself.
func getCtxForRequest(ctx context.Context) (context.Context, logger.Logger) {
	// UUID version 1: first 8 characters are time-based and lexicography sorted,
	// which is a useful property there
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	l := logger.Get(ctx).WithField("request", u.String())
	return logger.GetCtxWithLogger(ctx, l), l
}
