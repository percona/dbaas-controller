module github.com/percona-platform/dbaas-controller

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona/pmm => ../pmm

// Update with:
// go get -v github.com/percona/pmm@PMM-2.0

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/percona-platform/saas v0.0.0-20200629200043-f77bafe09147
	github.com/percona/percona-xtradb-cluster-operator v1.4.0
	github.com/percona/pmm v2.7.1-0.20200610194542-2785bb4d1a6b+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	google.golang.org/grpc v1.30.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.5
	sigs.k8s.io/controller-runtime v0.6.0 // indirect
)
