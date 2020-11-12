build:
	go build -trimpath -mod=vendor -ldflags "-X k8s.io/client-go/pkg/version.gitCommit=$$(git rev-parse HEAD) -X k8s.io/client-go/pkg/version.gitVersion=v1.0.0+$$(git rev-parse --short=7 HEAD)" -o bin/insights-operator ./cmd/insights-operator

.PHONY: build

test-unit:
	go test $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)
.PHONY: test-unit

test-e2e:
	go test ./test/integration -v -run ^\(TestIsIOHealthy\)$$ ^\(TestPullSecretExists\)$$ -timeout 1m
	test/integration/resource_samples/apply.sh
	go test ./test/integration -v -timeout 20m $(TEST_OPTIONS)
.PHONY: test-e2e

vet:
	go vet $$(go list ./... | grep -v /vendor/)

lint:
	golint $$(go list ./... | grep -v /vendor/)

gen-doc:
	go run cmd/gendoc/main.go --out=docs/gathered-data.md

vendor:
	go mod tidy
	go mod vendor
	go mod verify
.PHONY: vendor

build-plugin:
	go build -trimpath -mod=vendor -buildmode=plugin -ldflags "-X k8s.io/client-go/pkg/version.gitCommit=$$(git rev-parse HEAD) -X k8s.io/client-go/pkg/version.gitVersion=v1.0.0+$$(git rev-parse --short=7 HEAD)" -o bin/container_runtime_configs.so ./pkg/plugins/container_runtime_configs/gather.go
	go build -trimpath -mod=vendor -buildmode=plugin -ldflags "-X k8s.io/client-go/pkg/version.gitCommit=$$(git rev-parse HEAD) -X k8s.io/client-go/pkg/version.gitVersion=v1.0.0+$$(git rev-parse --short=7 HEAD)" -o bin/pod_disruption_budgets.so ./pkg/plugins/pod_disruption_budgets/gather.go