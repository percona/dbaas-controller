module github.com/percona-platform/dbaas-controller/tests

go 1.17

// Use for local development, but do not commit:
// replace github.com/percona-platform/dbaas-api => ../../dbaas-api

// Update with:
// go get -v github.com/percona-platform/dbaas-api@main

require (
	github.com/percona-platform/dbaas-api v0.0.0-20211221135741-2b281cf414aa
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.0
	golang.org/x/sys v0.5.0
	google.golang.org/grpc v1.49.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/mwitkow/go-proto-validators v0.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
