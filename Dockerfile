# syntax=docker/dockerfile:1

FROM golang:alpine AS build

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.10.0
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/carrel \
    ./cmd/carrel \
 && mkdir -p /out/data

FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/carrel /carrel
# Empty named volumes are seeded from this path, including owner and mode.
# Owned by the image user (nonroot), mode 0700 — not world-writable, not a
# numeric uid of our choosing.
COPY --from=build --chown=nonroot:nonroot --chmod=0700 /out/data /var/lib/carrel

EXPOSE 8080

ENV CARREL_DATA_DIR=/var/lib/carrel

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/carrel", "healthcheck"]

ENTRYPOINT ["/carrel"]
