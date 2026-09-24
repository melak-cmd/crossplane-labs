CLUSTER_NAME ?= crossplane-labs
PKG_NAME ?= kaonix-platform
TAG ?= v0.1.2

FUNCTION_NAME ?= function-recovery
FUNCTION_IMAGE ?= registry.localhost:5000/$(FUNCTION_NAME)

.PHONY: help create-cluster delete-cluster install-csi install-cnpg install-crossplane install-deps delete setup teardown restart-cnpg \
	project-build project-push uptest uptest-render render-app render-db render-db-backup render-backup render-network \
	validate-app validate-db validate-db-backup validate-backup validate-network validate validate-rbac status \
	function-build function-test function-lint function-xpkg function-push function-render

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-25s %s\n", $$1, $$2}'

create-cluster: ## Create k3d cluster
	k3d cluster create $(CLUSTER_NAME) --config clusters/k3d.yaml --api-port 6443

delete-cluster: ## Delete k3d cluster
	k3d cluster delete $(CLUSTER_NAME)

install-csi: ## Install CSI snapshot support (snapshot CRDs, snapshot-controller, hostpath CSI driver, classes)
	kubectl apply -f clusters/csi/snapshot-crds.yaml
	kubectl wait --for=condition=Established crd/volumesnapshots.snapshot.storage.k8s.io --timeout=60s
	kubectl apply -f clusters/csi/snapshot-controller.yaml
	kubectl apply -f clusters/csi/hostpath-rbac.yaml
	kubectl apply -f clusters/csi/hostpath-driverinfo.yaml
	kubectl apply -f clusters/csi/hostpath-plugin.yaml
	kubectl apply -f clusters/csi/snapshot-class.yaml
	kubectl apply -f clusters/csi/storage-class.yaml
	kubectl -n kube-system rollout status deploy/snapshot-controller --timeout=120s

install-cnpg: ## Install CloudNativePG operator
	helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
	helm repo update
	helm upgrade --install cnpg cnpg/cloudnative-pg \
		--namespace cnpg-system --create-namespace --wait

install-crossplane: ## Install Crossplane
	helm repo add crossplane-stable https://charts.crossplane.io/stable --force-update
	helm repo update
	helm upgrade --install crossplane crossplane-stable/crossplane \
		--namespace crossplane-system \
		--create-namespace \
		--version 2.3.3 \
		--set args='{--enable-operations}' \
		--wait

install-deps: ## Install APIs, functions, and providers from source (no package dependency resolution)
	kubectl create namespace platform --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -R -f apis/
	kubectl apply -f operations/
	kubectl apply -f functions/
	kubectl apply -f providers/
	for i in $$(seq 1 60); do kubectl get crd providerconfigs.kubernetes.m.crossplane.io >/dev/null 2>&1 && break; sleep 3; done
	kubectl wait --for=condition=Established crd/providerconfigs.kubernetes.m.crossplane.io --timeout=120s
	kubectl apply -f providers/providerconfigs/

delete: ## Delete application and database XRs
	kubectl delete -f examples/ --recursive --ignore-not-found

project-build: ## Build Crossplane project into packages (requires Docker)
	crossplane project build

project-push: project-build ## Push built project packages to local registry
	crossplane project push -t $(TAG)

uptest: ## Run e2e tests with uptest
	KUBECTL=kubectl CROSSPLANE_NAMESPACE=crossplane-system CHAINSAW=chainsaw \
	uptest e2e tests/uptest/app.yaml \
		--setup-script tests/uptest/setup.sh \
		--default-timeout 300s \
		--skip-import

uptest-render: ## Render chainsaw test files without running them
	KUBECTL=kubectl CROSSPLANE_NAMESPACE=crossplane-system CHAINSAW=chainsaw \
	uptest e2e tests/uptest/app.yaml \
		--setup-script tests/uptest/setup.sh \
		--default-timeout 300s \
		--skip-import \
		--render-only

render-app: ## Render App composition locally (requires Docker)
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml -x

render-db: ## Render Database composition locally (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x

render-db-backup: ## Render backup-enabled Database composition locally (requires Docker)
	crossplane composition render examples/databases/backup.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x

render-backup: ## Render DatabaseBackup composition locally (requires Docker)
	crossplane composition render examples/databases/manual-backup.yaml apis/databases/backup-composition.yaml \
		functions/functions.yaml -x

render-network: ## Render Network composition locally (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		functions/functions.yaml -x

validate-app: ## Render and validate App composition (requires Docker)
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml -x | \
		crossplane resource validate apis/ -

validate-db: ## Render and validate Database composition (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-db-backup: ## Render and validate backup-enabled Database composition (requires Docker)
	crossplane composition render examples/databases/backup.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-backup: ## Render and validate DatabaseBackup composition (requires Docker)
	crossplane composition render examples/databases/manual-backup.yaml apis/databases/backup-composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-network: ## Render and validate Network composition (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-rbac: ## Validate the composition read contract (downstream resources vs provider RBAC, no Docker)
	python3 scripts/validate-rbac.py

validate: validate-app validate-db validate-db-backup validate-backup validate-network validate-rbac ## Render and validate all compositions (requires Docker)
	@echo "All compositions validated successfully"

function-build: ## Build the function binary for the Development render runtime
	cd functions/$(FUNCTION_NAME) && go build -o function .

function-test: ## Run function unit tests
	cd functions/$(FUNCTION_NAME) && go test ./...

function-lint: ## Lint the function
	cd functions/$(FUNCTION_NAME) && golangci-lint run

function-xpkg: ## Build the function runtime image (Docker) and xpkg package (requires Docker)
	cd functions/$(FUNCTION_NAME) && docker build . --tag=$(FUNCTION_IMAGE):$(TAG)
	cd functions/$(FUNCTION_NAME) && crossplane xpkg build \
		--package-root=package \
		--embed-runtime-image=$(FUNCTION_IMAGE):$(TAG) \
		--package-file=$(FUNCTION_NAME).xpkg

function-push: function-xpkg ## Push the function image and xpkg to the local registry
	docker push $(FUNCTION_IMAGE):$(TAG)
	cd functions/$(FUNCTION_NAME) && crossplane xpkg push -f $(FUNCTION_NAME).xpkg $(FUNCTION_IMAGE):$(TAG)

function-render: function-build ## Render the function example locally (requires Docker)
	cd functions/$(FUNCTION_NAME) && ./function --insecure >/tmp/function-scale.log 2>&1 & \
	FNPID=$$!; \
	sleep 2; \
	cd functions/$(FUNCTION_NAME) && crossplane render example/xr.yaml example/composition.yaml example/functions.yaml -x; \
	RC=$$?; \
	kill $$FNPID 2>/dev/null; \
	wait $$FNPID 2>/dev/null; \
	exit $$RC

restart-cnpg: ## Restart CloudNativePG operator (required when snapshot CRDs are installed after the operator)
	kubectl delete pods -n cnpg-system -l app.kubernetes.io/name=cloudnative-pg --ignore-not-found

status: ## Show cluster and Crossplane status
	k3d cluster list
	@echo ""
	kubectl get pods -n crossplane-system 2>/dev/null || echo "Crossplane not installed"
	@echo ""
	kubectl get xrds compositions functions providers -A 2>/dev/null || true
	@echo ""
	kubectl get apps databases -A 2>/dev/null || echo "No apps or databases deployed"

setup: create-cluster install-crossplane install-csi install-cnpg install-deps ## Full cluster setup
	kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

teardown: delete-cluster ## Delete resources and cluster
