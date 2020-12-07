module github.com/percona-platform/dbaas-controller/tests

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona-platform/dbaas-api => ../dbaas-api

// Update with:
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/percona-platform/dbaas-api v0.0.0-20201120134348-35cb67a169d7
	github.com/percona-platform/dbaas-controller v0.1.2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.0.0-20201029080932-201ba4db2418
	google.golang.org/grpc v1.34.0
)
