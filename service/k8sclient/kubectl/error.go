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

package kubectl

import (
	"fmt"
)

type kubeCtlError struct {
	err    error
	cmd    string
	stderr string
}

func (e *kubeCtlError) Error() string {
	return fmt.Sprintf("%s\ncmd: %s\nstderr: %s", e.err, e.cmd, e.stderr)
}

// TODO Cause, Unwrap methods?
// TODO Tests
// https://jira.percona.com/browse/PMM-6349
