// dbaas-controller
// Copyright (C) 2020 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
