FROM golang:1.25.5-bookworm AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN make build

FROM alpine:3.20
ARG RESTIC_VERSION=0.18.1
ARG RESTIC_SHA256=5c0059e24d9e7c6c68acf34a1fd8e49da6eeb0c2c6fbc17c48c3cab28c1d23fc
RUN apk add --no-cache ca-certificates curl bzip2 \
    && curl -L "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2" -o /tmp/restic.bz2 \
    && echo "${RESTIC_SHA256}  /tmp/restic.bz2" | sha256sum -c - \
    && bzip2 -d /tmp/restic.bz2 \
    && install -m755 /tmp/restic /usr/local/bin/restic \
    && rm /tmp/restic

COPY --from=builder /src/bin/restic-exporter /usr/local/bin/restic-exporter

EXPOSE 8001
ENTRYPOINT ["/usr/local/bin/restic-exporter"]
