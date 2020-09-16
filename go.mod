module github.com/percona-platform/dbaas-controller

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona/pmm => ../pmm
// replace github.com/percona-platform/dbaas-api => ../dbaas-api
// replace github.com/percona-platform/saas => ../saas

// Update with:
// go get -v github.com/percona/pmm@PMM-2.0
// go get -v github.com/percona-platform/dbaas-api@main
// go get -v github.com/percona-platform/saas@main

require (
	github.com/AlekSi/pointer v1.1.0
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20200909154600-92f2ab9b8440
	github.com/percona-platform/saas v0.0.0-20200715163609-32e145816e31
	github.com/percona/percona-backup-mongodb v1.2.0 // indirect
	github.com/percona/percona-server-mongodb-operator v1.4.0
	github.com/percona/percona-xtradb-cluster-operator v1.4.0
	github.com/percona/pmm v2.9.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0
	golang.org/x/sys v0.0.0-20200724161237-0e2f3a69832c
	golang.org/x/text v0.3.3
	google.golang.org/grpc v1.32.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.19.1
	sigs.k8s.io/controller-runtime v0.6.1 // indirect
)

// Use the same versions as operators:
// * https://github.com/percona/percona-xtradb-cluster-operator/blob/master/go.mod
// * https://github.com/percona/percona-server-mongodb-operator/blob/master/go.mod
replace (
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
)
