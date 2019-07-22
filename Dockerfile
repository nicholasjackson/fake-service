FROM alpine:latest

RUN mkdir app
COPY upstream-echo /app/upstream-echo

ENTRYPOINT ["/app/upstream-echo"]
