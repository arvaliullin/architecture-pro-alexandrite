task3_dir := $(patsubst %/,%,$(dir $(lastword $(MAKEFILE_LIST))))

CERT_MANAGER_VERSION ?= v1.13.3
JAEGER_OPERATOR_VERSION ?= v1.51.0

.PHONY: task3-cert-manager
task3-cert-manager:
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	kubectl rollout status deployment/cert-manager -n cert-manager --timeout=180s
	kubectl rollout status deployment/cert-manager-cainjector -n cert-manager --timeout=180s
	kubectl rollout status deployment/cert-manager-webhook -n cert-manager --timeout=180s

.PHONY: task3-jaeger
task3-jaeger:
	kubectl get ns observability >/dev/null 2>&1 || kubectl create namespace observability
	curl -fsSL https://github.com/jaegertracing/jaeger-operator/releases/download/$(JAEGER_OPERATOR_VERSION)/jaeger-operator.yaml \
	  | sed 's|gcr.io/kubebuilder/kube-rbac-proxy|quay.io/brancz/kube-rbac-proxy|g' \
	  | kubectl apply --server-side -f - -n observability
	kubectl rollout status deployment/jaeger-operator -n observability --timeout=180s
	kubectl apply -f $(task3_dir)/k8s/jaeger-instance.yaml
	for _ in $$(seq 1 30); do \
	  kubectl get deployment/simplest >/dev/null 2>&1 && break; \
	  sleep 2; \
	done
	kubectl rollout status deployment/simplest --timeout=180s

.PHONY: task3-build
task3-build:
	docker build -t service-a:latest $(task3_dir)/services/service-a/
	docker build -t service-b:latest $(task3_dir)/services/service-b/

.PHONY: task3-deploy
task3-deploy:
	kubectl apply -f $(task3_dir)/k8s/services.yaml
	kubectl rollout status deployment/service-a --timeout=180s
	kubectl rollout status deployment/service-b --timeout=180s

.PHONY: task3-test
task3-test:
	kubectl exec $$(kubectl get pods -l app=service-a -o jsonpath='{.items[0].metadata.name}') -- wget -qO- http://service-a:8080

.PHONY: task3-ui
task3-ui:
	kubectl port-forward svc/simplest-query 16686:16686

.PHONY: task3-all
task3-all: task3-cert-manager task3-jaeger task3-build task3-deploy task3-test

.PHONY: task3-clean
task3-clean:
	kubectl delete -f $(task3_dir)/k8s/services.yaml --ignore-not-found
	kubectl delete -f $(task3_dir)/k8s/jaeger-instance.yaml --ignore-not-found
