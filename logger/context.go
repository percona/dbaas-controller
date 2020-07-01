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
