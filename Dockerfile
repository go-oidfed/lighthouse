FROM golang:1.25-bookworm AS builder
WORKDIR /app


COPY ./ ./
RUN go mod download

RUN CGO_ENABLED=1 go build -o /lighthouse github.com/go-oidfed/lighthouse/cmd/lighthouse
RUN go build -o /lhcli github.com/go-oidfed/lighthouse/cmd/lhcli

FROM debian:stable
RUN apt-get update && apt-get install -y ca-certificates && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/*

COPY --from=builder /lighthouse .
COPY --from=builder /lhcli .
COPY docker-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
