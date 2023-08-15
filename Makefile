DOCKER_REGISTRY ?= docker.io/nicholasjackson
VERSION=v0.23.1
CONSULBASE=v1.12.2

protos:
	 protoc --proto_path grpc/protos --go_out=grpc/api --go_opt=paths=source_relative \
    --go-grpc_out=grpc/api --go-grpc_opt=paths=source_relative \
    api.proto

# Requires Yarn and Node
build_ui:
	cd ui && DOCKER_BUILDKIT=1 docker build -f Dockerfile.build -o build .

build_linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/linux/amd64/fake-service

build_darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/darwin/amd64/fake-service
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/darwin/arm64/fake-service

build_arm6:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o bin/linux/arm6/fake-service

build_arm7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/linux/arm7/fake-service

build_arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/linux/arm64/fake-service

build_windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/windows/fake-service.exe

build_local: build_ui
	go build -o bin/fake-service

build_docker_vm:	build_ui build_linux build_arm64
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	docker buildx create --name multi || true
	docker buildx use multi
	docker buildx inspect --bootstrap
	docker buildx build --platform linux/arm64,linux/amd64 \
		-t ${DOCKER_REGISTRY}/fake-service:vm-${CONSULBASE}-${VERSION} \
		-f ./Dockerfile-VM \
    . \
		--push
	docker buildx rm multi

build_docker_multi: build_linux build_arm64
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	docker buildx create --name multi || true
	docker buildx use multi
	docker buildx inspect --bootstrap
	docker buildx build --platform linux/arm64,linux/amd64 \
		-t ${DOCKER_REGISTRY}/fake-service:${VERSION} \
    -f ./Dockerfile \
    ./bin \
		--push
	docker buildx rm multi

run_downstream:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9094" MESSAGE="This is some text<br/>Some more text too" go run main.go

run_downstream_errors:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false ERROR_RATE=1 ERROR_CODE=500 UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9093" go run main.go

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

test:
	filewatcher --idle-timeout 24h -x **/ui gotestsum

run_functional_ddog: build_docker
	cd examples/docker-compose-datadog && docker-compose up

run_functional_ddog_consul: build_docker
	cd examples/docker-compose-datadog && docker-compose -f docker-compose-consul.yml up

run_functional_jaeger: build_docker
	cd examples/docker-compose-jaeger && docker-compose up
