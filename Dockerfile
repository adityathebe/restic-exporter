FROM golang:1.25.5-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/restic-exporter .

FROM alpine:3.20
ARG RESTIC_VERSION=0.18.1
RUN apk add --no-cache ca-certificates curl bzip2 \
    && curl -L "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2" -o /tmp/restic.bz2 \
    && bzip2 -d /tmp/restic.bz2 \
    && install -m755 /tmp/restic /usr/local/bin/restic \
    && rm /tmp/restic

COPY --from=builder /out/restic-exporter /usr/local/bin/restic-exporter

EXPOSE 8001
ENTRYPOINT ["/usr/local/bin/restic-exporter"]
