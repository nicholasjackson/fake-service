FROM alpine:latest

RUN mkdir app
COPY ./bin/fake-service /app/fake-service

ENTRYPOINT ["/app/fake-service"]
