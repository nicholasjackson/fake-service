version=v0.22.6

protos:
	protoc -I grpc/protos/ grpc/protos/api.proto --go_out=plugins=grpc:grpc/api

# Requires Yarn and Node
build_ui:
	cd ui && REACT_APP_API_URI=/ PUBLIC_URL=/ui yarn build

# Requires Packr to bundle assets
build_linux: build_ui
	packr2
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/amd64/fake-service
	packr2 clean

build_darwin: build_ui
	packr2
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/darwin/fake-service
	packr2 clean

build_arm6: build_ui
	packr2
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o bin/arm/6/fake-service
	packr2 clean

build_arm7: build_ui
	packr2
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/arm/7/fake-service
	packr2 clean

build_arm64: build_ui
	packr2
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/arm64/fake-service
	packr2 clean

build_windows: build_ui
	packr2
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/windows/fake-service.exe
	packr2 clean

build_local: build_ui
	packr2
	go build -o bin/fake-service
	packr2 clean

build_docker_vm:	build_linux
	docker build -t nmnellis/fake-service:vm-${version} -f Dockerfile-VM ./

build_docker_multi: build_linux build_arm7 build_arm6 build_arm64
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
	docker buildx create --name multi || true
	docker buildx use multi
	docker buildx inspect --bootstrap
	docker buildx build --platform linux/arm/v6,linux/arm/v7,linux/arm64,linux/amd64 \
		-t nicholasjackson/fake-service:${version} \
    -f ./Dockerfile \
    ./bin \
		--push
	docker buildx rm multi

run_downstream:
	TRACING_ZIPKIN=/dev/null NAME=web HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,grpc://localhost:9094" go run main.go

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
