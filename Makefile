# Crossplane Labs Makefile
# Usage: make help

# Variables
CLUSTER_NAME ?= crossplane-labs
K3D_CONFIG ?= clusters/k3d.yaml
CROSSPLANE_VERSION ?= 1.20.5
KUBERNETES_PROVIDER_VERSION ?= v0.18.0
HELM_PROVIDER_VERSION ?= v0.14.0

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m

.PHONY: help create-cluster delete-cluster install-crossplane install-providers deploy-claim delete-claim status

help: ## Show this help
	@echo "$(GREEN)Crossplane Labs$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(YELLOW)%-25s$(NC) %s\n", $$1, $$2}'

create-cluster: ## Create k3d cluster
	@echo "$(GREEN)Creating k3d cluster $(CLUSTER_NAME)...$(NC)"
	k3d cluster create $(CLUSTER_NAME) --config $(K3D_CONFIG)

delete-cluster: ## Delete k3d cluster
	@echo "$(YELLOW)Deleting k3d cluster $(CLUSTER_NAME)...$(NC)"
	k3d cluster delete $(CLUSTER_NAME)

install-crossplane: ## Install Crossplane via Helm
	@echo "$(GREEN)Installing Crossplane $(CROSSPLANE_VERSION)...$(NC)"
	helm repo add crossplane-stable https://charts.crossplane.io/stable --force-update
	helm repo update
	helm upgrade --install crossplane crossplane-stable/crossplane \
		--namespace crossplane-system \
		--create-namespace \
		--version $(CROSSPLANE_VERSION) \
		--wait

install-providers: ## Apply provider configurations
	@echo "$(GREEN)Applying provider configurations...$(NC)"
	kubectl apply -f crossplane/providers/

deploy-xrds: ## Deploy CompositeResourceDefinitions
	@echo "$(GREEN)Deploying XRDs...$(NC)"
	kubectl apply -f crossplane/xrds/

deploy-compositions: ## Deploy Compositions
	@echo "$(GREEN)Deploying Compositions...$(NC)"
	kubectl apply -f crossplane/compositions/

deploy-claim: deploy-xrds deploy-compositions ## Deploy application claim
	@echo "$(GREEN)Deploying claim...$(NC)"
	kubectl apply -f apps/

delete-claim: ## Delete application claim
	@echo "$(YELLOW)Deleting claim...$(NC)"
	kubectl delete -f apps/ --ignore-not-found

status: ## Show cluster and Crossplane status
	@echo "$(GREEN)Cluster Status:$(NC)"
	k3d cluster list
	@echo ""
	@echo "$(GREEN)Pods:$(NC)"
	kubectl get pods -n crossplane-system 2>/dev/null || echo "Crossplane not installed"
	@echo ""
	@echo "$(GREEN)Providers:$(NC)"
	kubectl get providers 2>/dev/null || echo "No providers installed"
	@echo ""
	@echo "$(GREEN)XRDs:$(NC)"
	kubectl get xrds 2>/dev/null || echo "No XRDs deployed"
	@echo ""
	@echo "$(GREEN)Compositions:$(NC)"
	kubectl get compositions 2>/dev/null || echo "No Compositions deployed"
	@echo ""
	@echo "$(GREEN)Claims:$(NC)"
	kubectl get appclaims 2>/dev/null || echo "No Claims deployed"

setup: create-cluster install-crossplane install-providers ## Full setup: cluster + Crossplane + providers
	@echo "$(GREEN)Setup complete!$(NC)"

teardown: delete-claim delete-cluster ## Delete claim and cluster
	@echo "$(GREEN)Teardown complete!$(NC)"
