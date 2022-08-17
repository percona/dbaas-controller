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

	corev1 "k8s.io/api/core/v1"
)

// Extracted from https://pkg.go.dev/k8s.io/api/core/v1

// HostPathVolumeSource represents a host path mapped into a pod.
// Host path volumes do not support ownership management or SELinux relabeling.
//
// https://pkg.go.dev/k8s.io/api/core/v1#HostPathVolumeSource
type HostPathVolumeSource struct {
	// Path of the directory on the host.
	// If the path is a symlink, it will follow the link to the real path.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	Path string `json:"path"`
}

// EmptyDirVolumeSource represents an empty directory for a pod.
// Empty directory volumes support ownership management and SELinux relabeling.
//
// https://pkg.go.dev/k8s.io/api/core/v1#EmptyDirVolumeSource
type EmptyDirVolumeSource struct{}

// PodVolumePersistentVolumeClaim represents PVC of volume mounted to a pod.
type PodVolumePersistentVolumeClaim struct{}

// ContainerState describes container's state - waiting, running, terminated.
type ContainerState string

const (
	// ContainerStateWaiting represents a state when container requires some
	// operations being done in order to complete start up.
	ContainerStateWaiting ContainerState = "waiting"
	// ContainerStateTerminated indicates that container began execution and
	// then either ran to completion or failed for some reason.
	ContainerStateTerminated ContainerState = "terminated"
)

// IsContainerInState returns true if container is in give state, otherwise false.
func IsContainerInState(containerStatuses []corev1.ContainerStatus, state ContainerState, containerName string) bool {
	containerState := make(map[string]interface{})
	for _, status := range containerStatuses {
		data, _ := json.Marshal(status.State)
		json.Unmarshal(data, &containerState)
		if _, ok := containerState[string(state)]; ok {
			return true
		}
	}
	return false
}

// DeploymentTemplate is a template for creating pods.
type DeploymentTemplate struct {
	ObjectMeta `json:"metadata,omitempty"`
	Spec       PodSpec `json:"spec,omitempty"`
}

// DeploymentSpec details deployment specification.
type DeploymentSpec struct {
	Selector LabelSelector      `json:"selector,omitempty"`
	Template DeploymentTemplate `json:"template,omitempty"`
}

// Deployment is a higher abstraction based on pods. It's basically a group of pods.
type Deployment struct {
	TypeMeta
	ObjectMeta `json:"metadata,omitempty"`
	Spec       DeploymentSpec `json:"spec,omitempty"`
}

// IsNodeInCondition returns true if node's condition given as an argument has
// status "True". Otherwise it returns false.
func IsNodeInCondition(node corev1.Node, conditionType corev1.NodeConditionType) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Type == conditionType {
			return true
		}
	}
	return false
}

// Image holds continaer image names and image size.
type Image struct {
	Names     []string `json:"names,omitempty"`
	SizeBytes int64    `json:"sizeBytes,omitempty"`
}

// PersistentVolumeCapacity holds string representation of storage size.
type PersistentVolumeCapacity struct {
	// Storage size as string.
	Storage string `json:"storage,omitempty"`
}

// PersistentVolumeSpec holds PV specs.
type PersistentVolumeSpec struct {
	// Capacity of the volume.
	Capacity PersistentVolumeCapacity `json:"capacity,omitempty"`
}

// PersistentVolume holds information about PV.
type PersistentVolume struct {
	TypeMeta
	// Specification of the volume.
	Spec PersistentVolumeSpec `json:"spec,omitempty"`
}

// PodSpec is a description of a pod.
type PodSpec struct {
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	NodeName string `json:"nodeName,omitempty"`

	// Specifies the hostname of the Pod
	// If not specified, the pod's hostname will be set to a system-defined value.
	Hostname string `json:"hostname,omitempty"`

	// List of containers.
	Containers []ContainerSpec `json:"containers,omitempty"`

	// List of init containers.
	InitContainers []ContainerSpec `json:"initContainers,omitempty"`

	// Volumes stores list of volumes used by pod.
	Volumes []PodVolume `json:"volumes,omitempty"`
}

// ContainerSpec represents a container definition.
type ContainerSpec struct {
	Name      string               `json:"name,omitempty"`
	Image     string               `json:"image,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// PodVolume holds info about volume attached to pod.
type PodVolume struct {
	PersistentVolumeClaim *PodVolumePersistentVolumeClaim `json:"persistentVolumeClaim,omitempty"`
}
