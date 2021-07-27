FROM golang:1.12-alpine as builder
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates
WORKDIR /app
COPY . .
RUN GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -mod=vendor -ldflags="-w -s" -o grayproxy

FROM scratch as production
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/grayproxy /grayproxy
EXPOSE 12201/udp
ENTRYPOINT ["/grayproxy"]
CMD ["-in", "tcp://:12201", "-v", "-out", "http://loki:3100/api/prom/push"]
