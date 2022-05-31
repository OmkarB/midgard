# Build Image
FROM golang:1.18-alpine AS build

# ca-certificates pull in default CAs, without this https fetch from blockstore will fail
RUN apk add --no-cache make musl-dev gcc ca-certificates && update-ca-certificates

WORKDIR /tmp/midgard

# Cache Go dependencies like this:
COPY go.mod go.sum ./
RUN go mod download

COPY cmd cmd
COPY config config
COPY internal internal
COPY openapi openapi

# Compile.
ENV CC=/usr/bin/gcc
ENV CGO_ENABLED=1
RUN go build -v --ldflags '-linkmode external -extldflags=-static' -installsuffix cgo ./cmd/blockstore/dump
RUN go build -v --ldflags '-linkmode external -extldflags=-static' -installsuffix cgo ./cmd/midgard
RUN go build -v --ldflags '-linkmode external -extldflags=-static' -installsuffix cgo ./cmd/trimdb
RUN go build -v --ldflags '-linkmode external -extldflags=-static' -installsuffix cgo ./cmd/statechecks

# Main Image
FROM busybox

RUN mkdir -p openapi/generated
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /tmp/midgard/openapi/generated/doc.html ./openapi/generated/doc.html
COPY --from=build /tmp/midgard/dump .
COPY --from=build /tmp/midgard/midgard .
COPY --from=build /tmp/midgard/statechecks .
COPY --from=build /tmp/midgard/trimdb .
COPY config/config.json .
COPY resources /resources

RUN mkdir -p /tmp/midgard/blockstore

CMD [ "./midgard", "config.json" ]
