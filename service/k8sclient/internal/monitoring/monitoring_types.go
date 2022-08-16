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

// Package monitoring contains all structs required to monitor kubernetes cluster.
// +k8s:deepcopy-gen=package,register
package monitoring

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BasicAuth contains basic auth credentials to connect to Victoria Metrics.
type BasicAuth struct {
	// The secret in the service scrape namespace that contains the username
	// for authentication.
	Username corev1.SecretKeySelector `json:"username,omitempty"`
	// The secret in the service scrape namespace that contains the password
	// for authentication.
	Password corev1.SecretKeySelector `json:"password,omitempty"`
}

// VMAgentRemoteWriteSpec defines the remote storage configuration for VmAgent.
type VMAgentRemoteWriteSpec struct {
	// URL of the endpoint to send samples to.
	URL string `json:"url"`
	// BasicAuth allow an endpoint to authenticate over basic authentication
	BasicAuth *BasicAuth `json:"basicAuth"`
	// TLSConfig describes tls configuration for remote write target.
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

// VMAgentSpec contains configuration for VM Agent pod.
type VMAgentSpec struct {
	ServiceScrapeNamespaceSelector *metav1.LabelSelector        `json:"serviceScrapeNamespaceSelector"`
	ServiceScrapeSelector          *metav1.LabelSelector        `json:"serviceScrapeSelector"`
	PodScrapeNamespaceSelector     *metav1.LabelSelector        `json:"podScrapeNamespaceSelector"`
	PodScrapeSelector              *metav1.LabelSelector        `json:"podScrapeSelector"`
	ProbeSelector                  *metav1.LabelSelector        `json:"probeSelector"`
	ProbeNamespaceSelector         *metav1.LabelSelector        `json:"probeNamespaceSelector"`
	StaticScrapeSelector           *metav1.LabelSelector        `json:"staticScrapeSelector"`
	StaticScrapeNamespaceSelector  *metav1.LabelSelector        `json:"staticScrapeNamespaceSelector"`
	ReplicaCount                   int                          `json:"replicaCount"`
	Resources                      *corev1.ResourceRequirements `json:"resources"`
	ExtraArgs                      map[string]string            `json:"extraArgs"`
	RemoteWrite                    []VMAgentRemoteWriteSpec     `json:"remoteWrite"`
	SelectAllByDefault             bool                         `json:"selectAllByDefault"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMAgent contains CR for VM Agent.
type VMAgent struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VMAgentSpec `json:"spec"`
}

// TLSConfig specifies TLSConfig configuration parameters.
type TLSConfig struct {
	// Path to the CA cert in the container to use for the targets.
	CAFile string `json:"caFile,omitempty"`
	// Stuct containing the CA cert to use for the targets.
	CA *corev1.SecretKeySelector `json:"ca,omitempty"`

	// Path to the client cert file in the container for the targets.
	CertFile string `json:"certFile,omitempty"`
	// Struct containing the client cert file for the targets.
	Cert *corev1.SecretKeySelector `json:"cert,omitempty"`

	// Path to the client key file in the container for the targets.
	KeyFile string `json:"keyFile,omitempty"`
	// Secret containing the client key file for the targets.
	KeySecret *corev1.SecretKeySelector `json:"keySecret,omitempty"`

	// Used to verify the hostname for the targets.
	ServerName string `json:"serverName,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
