FROM alpine:latest

RUN mkdir app
COPY ./bin/fake-service-linux /app/fake-service

ENTRYPOINT ["/app/fake-service"]
