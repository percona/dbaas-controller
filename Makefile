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
PMM_USER ?= $(shell echo -n 'admin' | base64)
PMM_PASS ?= $(shell echo -n 'admin_password' | base64)
PATH_TO_KUBECONFIG ?= ${HOME}/.kube/config
KUBECTL_ARGS = --kubeconfig ${PATH_TO_KUBECONFIG}
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

eks-setup-test-namespace:
	kubectl ${KUBECTL_ARGS} create ns "${NAMESPACE}"
	kubectl ${KUBECTL_ARGS} config set-context --current --namespace="${NAMESPACE}"
	kubectl ${KUBECTL_ARGS} config get-contexts $(kubectl ${KUBECTL_ARGS} config current-context) | awk '{print $5}' | tail -1

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
	make env-up-wait

env-up-start:
	minikube config set kubernetes-version $(KUBERNETES_VERSION)
	minikube config view
	minikube start
	minikube status
	minikube profile list
	minikube addons list
	minikube kubectl -- version
	cat ./deploy/pxc-operator.yaml | minikube kubectl -- apply -f -
	minikube kubectl -- apply -f ./deploy/pxc-secrets.yaml
	cat ./deploy/psmdb-operator.yaml | minikube kubectl -- apply -f -
	minikube kubectl -- apply -f ./deploy/psmdb-secrets.yaml
	minikube kubectl -- get nodes
	minikube kubectl -- get pods

env-up-wait:
	minikube kubectl -- wait --for=condition=Available deployment percona-xtradb-cluster-operator
	minikube kubectl -- wait --for=condition=Available deployment percona-server-mongodb-operator

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

eks-install-operators:            ## Install Kubernetes operators in EKS.
	# Install the PXC operator
	cat ./deploy/pxc-operator.yaml | kubectl ${KUBECTL_ARGS} apply -f -
	cat ./deploy/pxc-secrets.yaml | sed "s/pmmserver:.*=/pmmserver: ${PMM_PASS}/g" | kubectl ${KUBECTL_ARGS} apply -f -
	# Install the PSMDB operator
	cat ./deploy/psmdb-operator.yaml | kubectl ${KUBECTL_ARGS} apply -f -
	cat ./deploy/psmdb-secrets.yaml | sed "s/PMM_SERVER_USER:.*/PMM_SERVER_USER: ${PMM_USER}/g;s/PMM_SERVER_PASSWORD:.*/PMM_SERVER_PASSWORD: ${PMM_PASS}/g;" | kubectl ${KUBECTL_ARGS} apply -f -
	kubectl ${KUBECTL_ARGS} wait --for=condition=Available deployment percona-xtradb-cluster-operator
	kubectl ${KUBECTL_ARGS} wait --for=condition=Available deployment percona-server-mongodb-operator

eks-delete-operators:             ## Delete Kubernetes operators from EKS. Run this before deleting the cluster to not to leave garbage.
	# Delete the PXC operator
	cat ./deploy/pxc-operator.yaml | kubectl ${KUBECTL_ARGS} delete -f -
	cat ./deploy/pxc-secrets.yaml | sed "s/pmmserver:.*/pmmserver: ${PMM_PASS}/g" | kubectl ${KUBECTL_ARGS} delete -f -
	# Delete the PSMDB operator
	cat ./deploy/psmdb-operator.yaml | kubectl ${KUBECTL_ARGS} delete -f -
	cat ./deploy/psmdb-secrets.yaml | sed "s/PMM_SERVER_USER:.*/PMM_SERVER_USER: ${PMM_USER}/g;s/PMM_SERVER_PASSWORD:.*/PMM_SERVER_PASSWORD: ${PMM_PASS}/g;" | kubectl ${KUBECTL_ARGS} delete -f -

eks-delete-current-namespace: eks-delete-operators
	export NAMESPACE="$(kubectl ${KUBECTL_ARGS} config get-contexts $(kubectl ${KUBECTL_ARGS} config current-context) | awk '{print $5}' | tail -1)"
	if [ "${NAMESPACE}" != "default" ]; then
		kubectl delete ns "${NAMESPACE}"
	fi
