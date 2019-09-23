version=v0.4.

protos:
	protoc -I grpc/protos/ grpc/protos/api.proto --go_out=plugins=grpc:grpc/api

build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/fake-service

build_docker: build_linux
	docker build -t nicholasjackson/fake-service:${version} .

run_downstream:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9093" go run main.go

run_upstream_1:
	NAME=payment MESSAGE="Hello from upstream 1" LISTEN_ADDR=localhost:9091  UPSTREAM_URIS=http://localhost:9092 go run main.go

run_upstream_2:
	NAME=currency MESSAGE="Hello from upstream 2" LISTEN_ADDR=localhost:9092 go run main.go

run_downstream_grpc:
	NAME=api HTTP_CLIENT_KEEP_ALIVES=false TRACING_ZIPKIN=/dev/stderr UPSTREAM_WORKERS=2 LISTEN_ADDR=localhost:9093 UPSTREAM_URIS="grpc://localhost:9094" go run main.go

run_upstream_grpc:
	NAME=accounts SERVER_TYPE=grpc TRACING_ZIPKIN=/dev/stderr MESSAGE="Hello from grpc upstream" LISTEN_ADDR=localhost:9094 go run main.go

call_downstream:
	curl localhost:9090
