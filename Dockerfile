FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hub ./cmd/hub

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -H hub

WORKDIR /app

COPY --from=builder /out/hub /app/hub

RUN mkdir -p /app/data /app/backups && chown -R hub:hub /app

USER hub

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -qO- http://127.0.0.1:8080/health/live >/dev/null || exit 1

CMD ["/app/hub"]
