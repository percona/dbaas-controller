package monitoring

import "github.com/percona-platform/dbaas-controller/service/k8sclient/common"

type BasicAuth struct {
	// The secret in the service scrape namespace that contains the username
	// for authentication.
	Username common.SecretKeySelector `json:"username,omitempty"`
	// The secret in the service scrape namespace that contains the password
	// for authentication.
	Password common.SecretKeySelector `json:"password,omitempty"`
}

type RemoteWriteSpec struct {
	URL       string     `json:"url"`
	BasicAuth *BasicAuth `json:"basicAuth"`
}

type VMAgentSpec struct {
	ServiceScrapeNamespaceSelector *common.LabelSelector `json:"serviceScrapeNamespaceSelector"`
	ServiceScrapeSelector          *common.LabelSelector `json:"serviceScrapeSelector"`
	PodScrapeNamespaceSelector     *common.LabelSelector `json:"podScrapeNamespaceSelector"`
	PodScrapeSelector              *common.LabelSelector `json:"podScrapeSelector"`
	ProbeSelector                  *common.LabelSelector `json:"probeSelector"`
	ProbeNamespaceSelector         *common.LabelSelector `json:"probeNamespaceSelector"`
	StaticScrapeSelector           *common.LabelSelector `json:"staticScrapeSelector"`
	StaticScrapeNamespaceSelector  *common.LabelSelector `json:"staticScrapeNamespaceSelector"`
	ReplicaCount                   int                   `json:"replicaCount"`
	Resources                      *common.PodResources  `json:"resources"`
	AdditionalArgs                 map[string]string     `json:"additionalArgs"`
	RemoteWrite                    []RemoteWriteSpec     `json:"remoteWrite"`
}

type VMAgent struct {
	common.TypeMeta // anonymous for embedding

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	common.ObjectMeta `json:"metadata,omitempty"`

	Spec VMAgentSpec `json:"spec"`
}
