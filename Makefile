CLUSTER_NAME ?= crossplane-labs
REGISTRY ?= registry.localhost:5000
PKG_NAME ?= kaonix-platform
PKG_TAG ?= v0.1.0

.PHONY: help create-cluster delete-cluster install-cnpg install-crossplane delete setup teardown \
	build-xpkg push-xpkg install-xpkg uptest uptest-render render-app render-db render-network \
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

delete: ## Delete application and database XRs
	kubectl delete -f examples/ --recursive --ignore-not-found

build-xpkg: ## Build Crossplane Configuration Package
	rm -rf .build && mkdir -p .build/apis .build/examples
	cp crossplane.yaml .build/
	cp -r apis/* .build/apis/
	cp -r examples/* .build/examples/
	crossplane xpkg build --package-root=.build --examples-root=".build/examples" \
		--package-file=xpkg.yaml --verbose
	rm -rf .build

push-xpkg: build-xpkg ## Push package to local registry
	crossplane xpkg push localhost:5000/$(PKG_NAME):$(PKG_TAG) -f xpkg.yaml

install-xpkg: push-xpkg ## Install package from local registry
	crossplane xpkg install configuration $(REGISTRY)/$(PKG_NAME):$(PKG_TAG) $(PKG_NAME) --wait=3m

uptest: ## Run e2e tests with uptest
	KUBECTL=kubectl CROSSPLANE_NAMESPACE=crossplane-system CHAINSAW=chainsaw \
	uptest e2e test/uptest/app.yaml \
		--setup-script test/uptest/setup.sh \
		--default-timeout 300s \
		--skip-import

uptest-render: ## Render chainsaw test files without running them
	KUBECTL=kubectl CROSSPLANE_NAMESPACE=crossplane-system CHAINSAW=chainsaw \
	uptest e2e test/uptest/app.yaml \
		--setup-script test/uptest/setup.sh \
		--default-timeout 300s \
		--skip-import \
		--render-only

render-app: ## Render App composition locally (requires Docker)
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml \
		crossplane/functions/functions.yaml -x

render-db: ## Render Database composition locally (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		crossplane/functions/functions.yaml -x

render-network: ## Render Network composition locally (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		crossplane/functions/functions.yaml -x

validate-app: ## Render and validate App composition (requires Docker)
	crossplane composition render examples/apps/app.yaml apis/apps/composition.yaml \
		crossplane/functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-db: ## Render and validate Database composition (requires Docker)
	crossplane composition render examples/databases/postgres.yaml apis/databases/composition.yaml \
		crossplane/functions/functions.yaml -x | \
		crossplane resource validate apis/ -

validate-network: ## Render and validate Network composition (requires Docker)
	crossplane composition render examples/networks/network.yaml apis/networks/composition.yaml \
		crossplane/functions/functions.yaml -x | \
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

setup: create-cluster install-cnpg install-crossplane install-xpkg ## Full cluster setup
	kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

teardown: delete delete-cluster ## Delete resources and cluster
