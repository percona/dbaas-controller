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
package monitoring

import (
	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
)

// BasicAuth contains basic auth credentials to connect to Victoria Metrics.
type BasicAuth struct {
	// The secret in the service scrape namespace that contains the username
	// for authentication.
	Username common.SecretKeySelector `json:"username,omitempty"`
	// The secret in the service scrape namespace that contains the password
	// for authentication.
	Password common.SecretKeySelector `json:"password,omitempty"`
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
	ServiceScrapeNamespaceSelector *common.LabelSelector    `json:"serviceScrapeNamespaceSelector"`
	ServiceScrapeSelector          *common.LabelSelector    `json:"serviceScrapeSelector"`
	PodScrapeNamespaceSelector     *common.LabelSelector    `json:"podScrapeNamespaceSelector"`
	PodScrapeSelector              *common.LabelSelector    `json:"podScrapeSelector"`
	ProbeSelector                  *common.LabelSelector    `json:"probeSelector"`
	ProbeNamespaceSelector         *common.LabelSelector    `json:"probeNamespaceSelector"`
	StaticScrapeSelector           *common.LabelSelector    `json:"staticScrapeSelector"`
	StaticScrapeNamespaceSelector  *common.LabelSelector    `json:"staticScrapeNamespaceSelector"`
	ReplicaCount                   int                      `json:"replicaCount"`
	Resources                      *common.PodResources     `json:"resources"`
	AdditionalArgs                 map[string]string        `json:"additionalArgs"`
	RemoteWrite                    []VMAgentRemoteWriteSpec `json:"remoteWrite"`
}

// VMAgent contains CR for VM Agent.
type VMAgent struct {
	common.TypeMeta // anonymous for embedding

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	common.ObjectMeta `json:"metadata,omitempty"`

	Spec VMAgentSpec `json:"spec"`
}

// TLSConfig specifies TLSConfig configuration parameters.
type TLSConfig struct {
	// Path to the CA cert in the container to use for the targets.
	CAFile string `json:"caFile,omitempty"`
	// Stuct containing the CA cert to use for the targets.
	CA *common.SecretKeySelector `json:"ca,omitempty"`

	// Path to the client cert file in the container for the targets.
	CertFile string `json:"certFile,omitempty"`
	// Struct containing the client cert file for the targets.
	Cert *common.SecretKeySelector `json:"cert,omitempty"`

	// Path to the client key file in the container for the targets.
	KeyFile string `json:"keyFile,omitempty"`
	// Secret containing the client key file for the targets.
	KeySecret *common.SecretKeySelector `json:"keySecret,omitempty"`

	// Used to verify the hostname for the targets.
	ServerName string `json:"serverName,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}
