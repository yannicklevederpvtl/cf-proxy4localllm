module github.com/cf-webhook-service/cf-proxy4localllm/hub

go 1.22

require (
	github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge v0.0.0
	github.com/google/uuid v1.6.0
	github.com/soheilhy/cmux v0.1.5
	google.golang.org/grpc v1.64.0
)

require (
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace github.com/cf-webhook-service/cf-proxy4localllm/gen/llmbridge => ../gen/llmbridge
