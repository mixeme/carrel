# syntax=docker/dockerfile:1

FROM golang:alpine AS build

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.1.0
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/carrel \
    ./cmd/carrel

FROM gcr.io/distroless/static:nonroot

COPY --from=build /out/carrel /carrel

EXPOSE 8080

ENV CARREL_DATA_DIR=/var/lib/carrel

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/carrel", "healthcheck"]

ENTRYPOINT ["/carrel"]
