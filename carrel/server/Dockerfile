# syntax=docker/dockerfile:1

# The context is the directory that holds this repository as carrel/ and the
# clone of Typecase beside it as typecase/ (docs/development.md): the binary
# wires in the client (carrel/client), and the client takes Typecase by a
# replace onto the neighbouring clone. From that directory:
#
#   docker build -f carrel/carrel/server/Dockerfile -t carrel .
#
# Only the server, the client and Typecase go in (Dockerfile.dockerignore
# beside this file); dev/, docs/ and the rest of the repository never do.
# The paths inside mirror the context, so the replaces of go.mod resolve
# as they do on a developer's machine.
FROM golang:1.27-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src/carrel/carrel/server
COPY typecase/go.mod typecase/go.sum /src/typecase/
COPY carrel/carrel/client/go.mod carrel/carrel/client/go.sum /src/carrel/carrel/client/
COPY carrel/carrel/server/go.mod carrel/carrel/server/go.sum ./
RUN go mod download
COPY typecase /src/typecase
COPY carrel/carrel/client /src/carrel/carrel/client
COPY carrel/carrel/server ./
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/carrel ./cmd/carrel

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 1000 -h /home/carrel carrel \
    && mkdir /data && chown carrel:carrel /data
COPY --from=build /out/carrel /usr/local/bin/carrel
USER carrel
ENV CARREL_BIND=0.0.0.0 \
    CARREL_PORT=8080 \
    CARREL_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/carrel"]
