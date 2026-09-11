# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/carrel ./cmd/carrel

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 1000 -h /home/carrel carrel
COPY --from=build /out/carrel /usr/local/bin/carrel
USER carrel
ENV CARREL_BIND=0.0.0.0 \
    CARREL_PORT=8080 \
    CARREL_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/carrel"]
