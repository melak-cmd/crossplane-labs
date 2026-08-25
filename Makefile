CLUSTER_NAME ?= crossplane-labs

.PHONY: help create-cluster delete-cluster install-crossplane install-cnpg install-providers \
	install-functions deploy-apis create-namespace deploy-app deploy-database \
	delete-app delete-database status setup teardown build-xpkg uptest uptest-render

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-25s %s\n", $$1, $$2}'

create-cluster: ## Create k3d cluster
	k3d cluster create $(CLUSTER_NAME) --config clusters/k3d.yaml

delete-cluster: ## Delete k3d cluster
	k3d cluster delete $(CLUSTER_NAME)

install-crossplane: ## Install Crossplane via Helm
	helm repo add crossplane-stable https://charts.crossplane.io/stable --force-update
	helm repo update
	helm upgrade --install crossplane crossplane-stable/crossplane \
		--namespace crossplane-system --create-namespace --wait

install-cnpg: ## Install CloudNativePG operator
	helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update
	helm repo update
	helm upgrade --install cnpg cnpg/cloudnative-pg \
		--namespace cnpg-system --create-namespace --wait

install-providers: ## Apply provider configurations
	kubectl apply -f crossplane/providers/

install-functions: ## Apply function configurations
	kubectl apply -f crossplane/functions/

deploy-apis: ## Deploy XRDs and Compositions
	kubectl apply -R -f apis/

create-namespace: ## Create a-team namespace and default ProviderConfig
	kubectl create namespace a-team --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f crossplane/providerconfigs/

deploy-app: deploy-apis create-namespace ## Deploy application XR
	kubectl apply -f examples/apps/

deploy-database: deploy-apis create-namespace ## Deploy database XR
	kubectl apply -f examples/databases/

delete-app: ## Delete application XR
	kubectl delete -f examples/apps/ --ignore-not-found

delete-database: ## Delete database XR
	kubectl delete -f examples/databases/ --ignore-not-found

build-xpkg: ## Build Crossplane Configuration Package
	crossplane xpkg build --package-root=. --examples-root="./examples" \
		--ignore=".github/*,clusters/*,test/*,Makefile,.gitignore,.git" --verbose

status: ## Show cluster and Crossplane status
	k3d cluster list
	@echo ""
	kubectl get pods -n crossplane-system 2>/dev/null || echo "Crossplane not installed"
	@echo ""
	kubectl get xrds compositions functions providers -A 2>/dev/null || true
	@echo ""
	kubectl get apps databases -A 2>/dev/null || echo "No apps or databases deployed"

setup: create-cluster install-crossplane install-cnpg install-providers install-functions deploy-apis create-namespace ## Full setup

teardown: delete-app delete-database delete-cluster ## Delete resources and cluster

uptest: ## Run e2e tests with uptest
	KUBECTL=kubectl CROSSPLANE_NAMESPACE=crossplane-system \
	CHAINSAW=$$(which chainsaw) \
	uptest e2e test/uptest/app.yaml \
		--setup-script test/uptest/setup.sh \
		--default-timeout 300s \
		--skip-import

uptest-render: ## Render uptest test cases without running
	uptest e2e test/uptest/app.yaml \
		--setup-script test/uptest/setup.sh \
		--render-only
