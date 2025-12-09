FROM golang:1.25.5-bookworm AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN make build

ARG RESTIC_VERSION=0.18.1
RUN apt-get update && apt-get install -y bzip2 && rm -rf /var/lib/apt/lists/* \
    && curl -L "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2" -o /tmp/restic.bz2 \
    && bzip2 -d /tmp/restic.bz2 \
    && chmod 755 /tmp/restic

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /src/bin/restic-exporter /usr/local/bin/restic-exporter
COPY --from=builder /tmp/restic /usr/local/bin/restic

EXPOSE 8001
ENTRYPOINT ["/usr/local/bin/restic-exporter"]
