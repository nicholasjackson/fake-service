build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/upstream-echo

build_docker:
	docker build -t nicholasjackson/upstream-echo:latest .

run_downstream:
	HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_WORKERS=2 UPSTREAM_URIS="http://localhost:9091,http://localhost:9092" go run main.go

run_upstream_1:
	MESSAGE="Hello from upstream 1" LISTEN_ADDR=localhost:9091 go run main.go

run_upstream_2:
	MESSAGE="Hello from upstream 2" LISTEN_ADDR=localhost:9092 go run main.go

call_downstream:
	curl localhost:9090
