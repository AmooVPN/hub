FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hub ./cmd/hub

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && adduser -D -H hub

WORKDIR /app

COPY --from=builder /out/hub /app/hub
COPY web /app/web

RUN mkdir -p /app/data /app/backups && chown -R hub:hub /app

USER hub

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD ["/app/hub", "healthcheck"]

CMD ["/app/hub"]
