module github.com/percona-platform/dbaas-controller/tests

go 1.14

// Use for local development, but do not commit:
// replace github.com/percona-platform/dbaas-api => ../dbaas-api

// Update with:
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/percona-platform/dbaas-api v0.0.0-20201210100406-24be1f8f5ab6
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20200520004742-59133d7f0dd7 // indirect
	golang.org/x/sys v0.0.0-20201029080932-201ba4db2418
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/grpc v1.33.2
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)
