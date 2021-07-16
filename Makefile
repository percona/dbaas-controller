default: help

help:                             ## Display this help message
	@echo "Please use \`make <target>\` where <target> is one of:"
	@grep '^[a-zA-Z]' $(MAKEFILE_LIST) | \
		awk -F ':.*?## ' 'NF==2 {printf "  %-26s%s\n", $$1, $$2}'
	@echo " To deploy operators in an EKS cluster:"
	@echo ""
	@echo " Steps:"
	@echo " - Install AWS CLI https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html"
	@echo " - Use \`aws configure\` and add \`AWS_ACCESS_KEY_ID\` and \`AWS_SECRET_ACCESS_KEY\` with your credentials."
	@echo " - Create a cluster with the following command:"
	@echo " - eksctl create cluster --write-kubeconfig —name=your-cluster-name —zones=us-west-2a,us-west-2b --kubeconfig <PATH_TO_KUBECONFIG>"
	@echo " - Add your ACCESS and SECRET key from stage 3 to env section of your kube config file"
	@echo " "
	@echo "  	env:"
	@echo "   	- name: AWS_STS_REGIONAL_ENDPOINTS"
	@echo "     	value: regional"
	@echo "   	- name: AWS_DEFAULT_REGION"
	@echo "     	value: us-west-2"
	@echo "   	- name: AWS_ACCESS_KEY_ID"
	@echo "     	value: XXXXXXXXXXXXXXXXXXXXXXXX"
	@echo "   	- name: AWS_SECRET_ACCESS_KEY"
	@echo "     	value: XXXXXXXXXXXXXXXXXXXXXXXX"
	@echo " "
	@echo " - Replace aws to aws-iam-authenticator in users.user.exec.command of your kube config file and replace in args"
	@echo "         - eks"
	@echo "         - get-token"
	@echo "         - --cluster-name"
	@echo "         - <cluster-name>"
	@echo " with"
	@echo "        - token"
	@echo "         - -i"
	@echo ""

KUBERNETES_VERSION ?= 1.16.8

# `cut` is used to remove first `v` from `git describe` output
# PMM_RELEASE_XXX variables are overwritten during PMM Server build
PMM_RELEASE_PATH ?= bin
COMPONENT_VERSION ?= $(shell git describe --always --dirty | cut -b2-)
PMM_RELEASE_VERSION ?=
PMM_RELEASE_TIMESTAMP ?= $(shell date '+%s')
PMM_RELEASE_FULLCOMMIT ?= $(shell git rev-parse HEAD)
PMM_RELEASE_BRANCH ?= $(shell git describe --always --contains --all)
PMM_CONTAINER ?= pmm-managed-server
PATH_TO_KUBECONFIG ?= ${HOME}/.kube/config
KUBECTL_ARGS ?= --kubeconfig ${PATH_TO_KUBECONFIG}
PMM_LD_FLAGS = -ldflags " \
			-X 'github.com/percona/pmm/version.ProjectName=dbaas-controller' \
			-X 'github.com/percona/pmm/version.Version=$(COMPONENT_VERSION)' \
			-X 'github.com/percona/pmm/version.PMMVersion=$(PMM_RELEASE_VERSION)' \
			-X 'github.com/percona/pmm/version.Timestamp=$(PMM_RELEASE_TIMESTAMP)' \
			-X 'github.com/percona/pmm/version.FullCommit=$(PMM_RELEASE_FULLCOMMIT)' \
			-X 'github.com/percona/pmm/version.Branch=$(PMM_RELEASE_BRANCH)' \
			"

release-component-version:
	@echo $(COMPONENT_VERSION)

release:                          ## Build dbaas-controller release binaries.
	env CGO_ENABLED=0 go build -mod=readonly -v $(PMM_LD_FLAGS) -o $(PMM_RELEASE_PATH)/dbaas-controller ./cmd/dbaas-controller
	$(PMM_RELEASE_PATH)/dbaas-controller --version

init:                             ## Install development tools
	go build -o bin/check-license ./.github/check-license.go

	go build -modfile=tools/go.mod -o bin/go-consistent github.com/quasilyte/go-consistent
	go build -modfile=tools/go.mod -o bin/gofumports mvdan.cc/gofumpt/gofumports
	go build -modfile=tools/go.mod -o bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint
	go build -modfile=tools/go.mod -o bin/gotext golang.org/x/text/cmd/gotext
	go build -modfile=tools/go.mod -o bin/reviewdog github.com/reviewdog/reviewdog/cmd/reviewdog

ci-init:                ## Initialize CI environment
	# nothing there yet

gen:                              ## Generate code
	go generate ./catalog
	mv catalog/locales/en/out.gotext.json catalog/locales/en/messages.gotext.json
	# add blank line at EOF
	echo >> catalog/locales/en/messages.gotext.json
	make format

format:                           ## Format source code
	gofmt -w -s .
	bin/gofumports -local github.com/percona-platform/dbaas-controller -l -w .

check:                            ## Run checks/linters for the whole project
	bin/check-license
	bin/go-consistent -pedantic -exclude "tests" ./...
	cd tests && ../bin/go-consistent -pedantic ./...
	bin/golangci-lint run

install:                          ## Install binaries
	go build $(PMM_LD_FLAGS) -race -o bin/dbaas-controller ./cmd/dbaas-controller

test:                             ## Run tests
	go test -race -timeout=30m ./...

test-cover:                       ## Run tests and collect per-package coverage information
	go test -race -timeout=30m -count=1 -coverprofile=cover.out -covermode=atomic ./...

test-crosscover:                  ## Run tests and collect cross-package coverage information
	go test -race -timeout=30m -count=1 -coverprofile=crosscover.out -covermode=atomic -p=1 -coverpkg=./... ./...

test-api-build:                   ## Check that API tests can be built
	cd tests && go test -count=0 ./...

run: install                      ## Run dbaas-controller
	bin/dbaas-controller

env-up:                           ## Start development environment
	make env-up-start

env-up-start:
	minikube config set kubernetes-version $(KUBERNETES_VERSION)
	minikube config view
	minikube start
	minikube status
	minikube profile list
	minikube addons list
	minikube kubectl -- version
	minikube kubectl -- get nodes
	minikube kubectl -- get pods


env-down:
	#
	# Please use `minikube stop` to stop Kubernetes cluster, or `minikube delete` to fully delete it.
	# Not picking one for you.
	#

collect-debugdata:                ## Collect debugdata
	rm -fr debugdata
	mkdir debugdata
	minikube logs --length=100 > ./debugdata/minikube.txt
	minikube kubectl -- describe pods > ./debugdata/pods.txt
	minikube kubectl -- describe pv,pvc > ./debugdata/pv-pvc.txt
	minikube kubectl -- get events --sort-by=lastTimestamp > ./debugdata/events.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='name=percona-xtradb-cluster-operator' > ./debugdata/pxc-operators.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='app.kubernetes.io/name=percona-xtradb-cluster' > ./debugdata/pxc-clusters.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='name=percona-server-mongodb-operator' > ./debugdata/psmdb-operators.txt
	minikube kubectl -- logs --all-containers --timestamps --selector='app.kubernetes.io/name=percona-server-mongodb' > ./debugdata/psmdb-clusters.txt

eks-setup-test-namespace:
	kubectl ${KUBECTL_ARGS} create ns "${NAMESPACE}"
	kubectl ${KUBECTL_ARGS} config set-context --current --namespace="${NAMESPACE}"

eks-cleanup-namespace:
	kubectl ${KUBECTL_ARGS} delete perconaxtradbcluster --all
	kubectl ${KUBECTL_ARGS} delete perconaxtradbclusterbackup --all
	kubectl ${KUBECTL_ARGS} delete perconaservermongodb --all
	kubectl ${KUBECTL_ARGS} delete perconaservermongodbbackup --all

eks-delete-operators:             ## Delete Kubernetes operators from EKS. Run this before deleting the cluster to not to leave garbage.
	# Delete the PXC operator
	kubectl ${KUBECTL_ARGS} delete deployment percona-xtradb-cluster-operator
	# Delete the PSMDB operator
	kubectl ${KUBECTL_ARGS} delete deployment percona-server-mongodb-operator

eks-delete-current-namespace:
	NAMESPACE=$$(kubectl config view --minify --output 'jsonpath={..namespace}'); \
	if [ "$$NAMESPACE" != "default" ]; then kubectl delete ns "$$NAMESPACE"; fi

deploy-to-pmm-server:
	docker cp bin/dbaas-controller ${PMM_CONTAINER}:/usr/sbin/dbaas-controller
	docker exec ${PMM_CONTAINER} supervisorctl restart dbaas-controller
