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

package common

import (
	"encoding/json"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const containerStateTestInput string = `
{
 "containerStatuses": [
     {
         "containerID": "docker://dac1c01c439e8fa873679b13e22bc75baa59129e2c4e3282e36bf950b5c7bc53",
         "image": "perconalab/pmm-client:dev-latest",
         "imageID": "docker-pullable://perconalab/pmm-client@sha256:1697b99e10e50ce62637c6073f6ff70ab96cfbc287e487c554ce1bb72a5126fe",
         "lastState": {
             "terminated": {
                 "containerID": "docker://dac1c01c439e8fa873679b13e22bc75baa59129e2c4e3282e36bf950b5c7bc53",
                 "exitCode": 1,
                 "finishedAt": "2021-02-19T15:19:57Z",
                 "reason": "Error",
                 "startedAt": "2021-02-19T15:19:57Z"
             }
         },
         "name": "pmm-client",
         "ready": false,
         "restartCount": 3,
         "started": false,
         "state": {
             "waiting": {
                 "message": "back-off 40s restarting failed container=pmm-client pod=newclusterinsane-proxysql-0_default(efda5403-ff22-46e7-9930-4366d7eec910)",
                 "reason": "CrashLoopBackOff"
             }
         }
     }
  ]        
}
`

func TestIsContainerInState(t *testing.T) {
	t.Parallel()
	ps := new(PodStatus)
	require.NoError(t, json.Unmarshal([]byte(containerStateTestInput), ps))
	assert.True(t, IsContainerInState(ps.ContainerStatuses, ContainerStateWaiting, "pmm-client"), "pmm-client is waiting but reported otherwise")
	assert.False(t, IsContainerInState(ps.ContainerStatuses, ContainerState("fakestate"), "pmm-client"), "check for non-existing state should return false")
}
