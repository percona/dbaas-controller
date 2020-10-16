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

package v1

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestReconcileAffinity(t *testing.T) {
	cases := []struct {
		name     string
		pod      *PodSpec
		desiered *PodSpec
	}{
		{
			name: "no affinity set",
			pod:  &PodSpec{},
			desiered: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: &defaultAffinityTopologyKey,
				},
			},
		},
		{
			name: "wrong antiAffinityTopologyKey",
			pod: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: func(s string) *string { return &s }("beta.kubernetes.io/instance-type"),
				},
			},
			desiered: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: &defaultAffinityTopologyKey,
				},
			},
		},
		{
			name: "valid antiAffinityTopologyKey",
			pod: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: func(s string) *string { return &s }("kubernetes.io/hostname"),
				},
			},
			desiered: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: func(s string) *string { return &s }("kubernetes.io/hostname"),
				},
			},
		},
		{
			name: "valid antiAffinityTopologyKey with Advanced",
			pod: &PodSpec{
				Affinity: &PodAffinity{
					TopologyKey: func(s string) *string { return &s }("kubernetes.io/hostname"),
					Advanced: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{},
					},
				},
			},
			desiered: &PodSpec{
				Affinity: &PodAffinity{
					Advanced: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{},
					},
				},
			},
		},
	}

	for _, c := range cases {
		c.pod.reconcileAffinityOpts()
		if !reflect.DeepEqual(c.desiered.Affinity, c.pod.Affinity) {
			t.Errorf("case %q:\n want: %#v\n have: %#v", c.name, c.desiered.Affinity, c.pod.Affinity)
		}
	}
}
