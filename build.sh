
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/amd64/fake-service
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o bin/arm/6/fake-service
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o bin/arm/7/fake-service
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/darwin/fake-service

docker buildx build --platform linux/arm/v6,linux/arm/v7,linux/amd64 \
    -t nmnellis/fake-service:v2 -f ./Dockerfile ./bin --push