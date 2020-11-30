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
	metav1 "github.com/percona-platform/dbaas-controller/k8s_api/apimachinery/pkg/apis/meta/v1"
)

const (
	// NamespaceDefault means the object is in the default namespace which is applied when not specified by clients.
	NamespaceDefault string = "default"
	// NamespaceAll is the default argument to specify on a context when you want to list or filter resources across all namespaces.
	NamespaceAll string = ""
	// NamespaceNodeLease is the namespace where we place node lease objects (used for node heartbeats).
	NamespaceNodeLease string = "kube-node-lease"
	// TopologyKeyAny is the service topology key that matches any node.
	TopologyKeyAny string = "*"
)

// Represents a host path mapped into a pod.
// Host path volumes do not support ownership management or SELinux relabeling.
type HostPathVolumeSource struct {
	// Path of the directory on the host.
	// If the path is a symlink, it will follow the link to the real path.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	Path string `json:"path"`
}

// Represents an empty directory for a pod.
// Empty directory volumes support ownership management and SELinux relabeling.
type EmptyDirVolumeSource struct{}

// PodSpec is a description of a pod.
type PodSpec struct {
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	NodeName string `json:"nodeName,omitempty"`

	// Specifies the hostname of the Pod
	// If not specified, the pod's hostname will be set to a system-defined value.
	Hostname string `json:"hostname,omitempty"`
}

// Pod is a collection of containers that can run on a host. This resource is created
// by clients and scheduled onto hosts.
type Pod struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec PodSpec `json:"spec,omitempty"`
}
