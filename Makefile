CLUSTER_NAME ?= crossplane-labs
NAMESPACE ?= platform

.PHONY: help create-cluster delete-cluster install-cnpg deploy delete setup teardown \
	build-xpkg uptest uptest-render status

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-25s %s\n", $$1, $$2}'

create-cluster: ## Create k3d cluster
	k3d cluster create $(CLUSTER_NAME) --config clusters/k3d.yaml

delete-cluster: ## Delete k3d cluster
	k3d cluster delete $(CLUSTER_NAME)

install-cnpg: ## Install CloudNativePG operator
	helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
	helm repo update
	helm upgrade --install cnpg cnpg/cloudnative-pg \
		--namespace cnpg-system --create-namespace --wait

deploy: ## Deploy XRDs, Compositions, functions, namespace and ProviderConfig
	kubectl apply -R -f apis/
	kubectl apply -f crossplane/functions/
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f crossplane/providerconfigs/

delete: ## Delete application and database XRs
	kubectl delete -f examples/ --recursive --ignore-not-found

build-xpkg: ## Build Crossplane Configuration Package
	crossplane xpkg build --package-root=. --examples-root="./examples" \
		--ignore=".github/*,clusters/*,test/*,Makefile,.gitignore,.git" --verbose

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

status: ## Show cluster and Crossplane status
	k3d cluster list
	@echo ""
	kubectl get pods -n crossplane-system 2>/dev/null || echo "Crossplane not installed"
	@echo ""
	kubectl get xrds compositions functions providers -A 2>/dev/null || true
	@echo ""
	kubectl get apps databases -A 2>/dev/null || echo "No apps or databases deployed"

setup: create-cluster install-cnpg deploy ## Full cluster setup
	kubectl wait --for=condition=Ready pods --all -n crossplane-system --timeout=180s

teardown: delete delete-cluster ## Delete resources and cluster
