build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/upstream-echo

build_docker:
	docker build -t nicholasjackson/upstream-echo:latest .

run_downstream:
	UPSTREAM_CALL=true go run main.go

run_upstream:
	MESSAGE="Hello from upstream" LISTEN_ADDR=localhost:9091 go run main.go

call_downstream:
	curl localhost:9090
