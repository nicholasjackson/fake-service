FROM alpine:latest

RUN mkdir app
COPY ./bin/upstream-echo /app/upstream-echo

ENTRYPOINT ["/app/upstream-echo"]
