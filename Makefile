build_linux:
	CGO_ENABLED=0 GOOS=linux go build -o bin/upstream-echo

build_docker:
	docker build -t nicholasjackson/upstream-echo:latest .

run_downstream:
	go run main.go

run_upstream:
	UPSTREAM=true LISTEN_ADDR=localhost:9091 go run main.go

call_downstream:
	curl localhost:9090
