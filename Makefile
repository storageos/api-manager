
# Image URL to use all building/pushing image targets
IMG ?= storageos/api-manager:test
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

OS=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)
KUBEBUILDER_VERSION=2.3.1
DEFAULT_KUBEBUILDER_PATH=/usr/local/kubebuilder/bin

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Install kubebuilder tools. This is required for running envtest.
kubebuilder:
	@if [ ! -d $(DEFAULT_KUBEBUILDER_PATH) ]; then \
		curl -L https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(OS)/$(ARCH) | tar -xz -C /tmp/; \
		sudo mv /tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)/ /usr/local/kubebuilder; \
	fi

# Run tests
test: kubebuilder generate fmt vet manifests
	go test -timeout 300s ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet tidy
	go build -o bin/manager main.go

# Prune, add and vendor go dependencies.
tidy:
	go mod tidy -v
	go mod vendor -v

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests secret
	go run ./main.go -api-secret-path=$(PWD)/.secret

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Create local secret, required for `run` target.
secret: .secret .secret/username .secret/password
.secret:
	mkdir .secret
.secret/username:
	echo "storageos" >.secret/username
.secret/password:
	echo "storageos" >.secret/password

