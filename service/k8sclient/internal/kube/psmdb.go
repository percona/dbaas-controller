package kube

import "time"

// ObjectHeader is the kubectl get response header. It is a partial PSMDB object only used to
// parse the CR version so we can decode the response into the appropriate stuct type.
type MinimumObjectListSpec struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Annotations struct {
				KubectlKubernetesIoLastAppliedConfiguration string `json:"kubectl.kubernetes.io/last-applied-configuration"`
			} `json:"annotations"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Finalizers        []string  `json:"finalizers"`
			Generation        int       `json:"generation"`
			Name              string    `json:"name"`
			Namespace         string    `json:"namespace"`
			ResourceVersion   string    `json:"resourceVersion"`
			UID               string    `json:"uid"`
		} `json:"metadata"`
		Spec struct {
			CrVersion string `json:"crVersion"`
		} `json:"spec"`
	} `json:"items"`
}
