FROM alpine:latest as base

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

# Copy AMD binaries
FROM base AS image-amd64-

COPY amd64/fake-service /app/fake-service
RUN chmod +x /app/fake-service

# Copy Arm 6 binaries
FROM base AS image-arm-v6

COPY arm/6/fake-service /app/fake-service
RUN chmod +x /app/fake-service

# Copy Arm 7 binaries
FROM base AS image-arm-v7

COPY arm/7/fake-service /app/fake-service
RUN chmod +x /app/fake-service

FROM image-${TARGETARCH}-${TARGETVARIANT}

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG BUILDPLATFORM
ARG BUILDARCH

RUN echo "I am running on $BUILDPLATFORM, building for $TARGETPLATFORM $TARGETARCH $TARGETVARIANT"  

ENTRYPOINT ["/app/fake-service"]
