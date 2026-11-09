# syntax=docker/dockerfile:1

FROM golang:1.27.3-bookworm AS build

# confluent-kafka-go statically links librdkafka but still links against the
# system TLS, SASL, compression, and HTTP libraries.
RUN apt-get update && apt-get install -y --no-install-recommends \
    libssl-dev libsasl2-dev zlib1g-dev libzstd-dev liblz4-dev libcurl4-openssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=1 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/stream-bridge .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libssl3 libsasl2-2 zlib1g libzstd1 liblz4-1 libcurl4 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home stream

COPY --from=build /out/stream-bridge /usr/local/bin/stream-bridge

USER stream
WORKDIR /home/stream
ENTRYPOINT ["stream-bridge"]
