module github.com/percona-platform/dbaas-controller

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona/pmm => ../pmm
// replace github.com/percona-platform/dbaas-api => ../dbaas-api

// Update with:
// go get -v github.com/percona/pmm@PMM-2.0
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20200629201908-dfd511a99763
	github.com/percona-platform/saas v0.0.0-20200629200043-f77bafe09147
	github.com/percona/percona-xtradb-cluster-operator v1.4.0
	github.com/percona/pmm v2.8.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.15.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a // indirect
	golang.org/x/sys v0.0.0-20200622214017-ed371f2e16b4
	golang.org/x/text v0.3.3
	golang.org/x/tools v0.0.0-20200702044944-0cc1aa72b347 // indirect
	google.golang.org/genproto v0.0.0-20200311144346-b662892dd51b // indirect
	google.golang.org/grpc v1.30.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	sigs.k8s.io/controller-runtime v0.6.0 // indirect
)
