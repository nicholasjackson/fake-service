build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/upstream-echo

build_docker:
	docker build -t nicholasjackson/upstream-echo:latest .

run_downstream:
	HTTP_CLIENT_KEEP_ALIVES=false UPSTREAM_CALL=true go run main.go

run_upstream_1:
	UPSTREAM_CALL=true MESSAGE="Hello from upstream 1" UPSTREAM_URI=localhost:9092 LISTEN_ADDR=localhost:9091 go run main.go

run_upstream_2:
	MESSAGE="Hello from upstream 2" LISTEN_ADDR=localhost:9092 go run main.go

call_downstream:
	curl localhost:9090
