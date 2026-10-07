FROM golang:1.26 AS builder
WORKDIR /app

ARG BUILD_VERSION
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV BUILD_VERSION=${BUILD_VERSION}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Not `make init`: that also runs `buf dep update`, which would build the
# image against unpinned dependencies instead of the committed buf.lock.
RUN make install && make gen/all && make build/${TARGETOS}/${TARGETARCH}

FROM gcr.io/distroless/static
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY --from=builder /app/build/${TARGETOS}_${TARGETARCH}/app /main
ENV GIN_MODE=release
ENTRYPOINT ["/main"]
CMD ["--config", "/config/config.json"]
