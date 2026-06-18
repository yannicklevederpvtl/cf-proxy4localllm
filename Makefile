# Proto codegen (Task 17). Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
PROTO_DIR := proto/llmbridge/v1
GEN_DIR := gen/llmbridge/v1

.PHONY: proto
proto:
	PATH="$$(go env GOPATH)/bin:$$PATH" protoc \
		--go_out=. --go_opt=module=github.com/cf-webhook-service/cf-proxy4localllm \
		--go-grpc_out=. --go-grpc_opt=module=github.com/cf-webhook-service/cf-proxy4localllm \
		$(PROTO_DIR)/bridge.proto

.PHONY: vendor
vendor:
	cd hub && go mod vendor && go mod tidy

.PHONY: build
build:
	cd hub && go build -o ../bin/hub .
	cd bridge && go build -o ../bin/bridge .

.PHONY: verify
verify: proto
	cd gen/llmbridge && go mod tidy && go mod verify && go build ./...
	cd hub && go mod tidy && go mod verify && go test ./...
	cd bridge && go mod tidy && go mod verify && go test ./...
