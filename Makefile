CLUSTER_NAME ?= crossplane-labs
PKG_NAME ?= kaonix-platform
PKG_TAG ?= v0.1.0

.PHONY: help create-cluster delete-cluster install-cnpg install-crossplane install-deps delete setup teardown \
	project-build project-push uptest uptest-render render-app render-db render-network \
	validate-app validate-db validate-network validate status

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-25s %s\n", $$1, $$2}'

create-cluster: ## Create k3d cluster
	k3d cluster create $(CLUSTER_NAME) --config clusters/k3d.yaml --api-port 6443

delete-cluster: ## Delete k3d cluster
	k3d cluster delete $(CLUSTER_NAME)

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
		--wait

install-deps: ## Install APIs, functions, and providers from source (no package dependency resolution)
	kubectl create namespace platform --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -R -f apis/
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
	crossplane project push -t $(PKG_TAG)

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
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml \
		functions/functions.yaml -x

render-db: ## Render Database composition locally (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x

render-network: ## Render Network composition locally (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		functions/functions.yaml -x

validate-app: ## Render and validate App composition (requires Docker)
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-db: ## Render and validate Database composition (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-network: ## Render and validate Network composition (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate: validate-app validate-db validate-network ## Render and validate all compositions (requires Docker)
	@echo "All compositions validated successfully"

status: ## Show cluster and Crossplane status
	k3d cluster list
	@echo ""
	kubectl get pods -n crossplane-system 2>/dev/null || echo "Crossplane not installed"
	@echo ""
	kubectl get xrds compositions functions providers -A 2>/dev/null || true
	@echo ""
	kubectl get apps databases -A 2>/dev/null || echo "No apps or databases deployed"

setup: create-cluster install-cnpg install-crossplane install-deps ## Full cluster setup
	kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

teardown: delete-cluster ## Delete resources and cluster
