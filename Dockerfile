FROM golang:1.23-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/AmooVPM/hub/internal/version.Version=${VERSION} -X github.com/AmooVPM/hub/internal/version.Commit=${COMMIT} -X github.com/AmooVPM/hub/internal/version.Date=${DATE}" -o /out/hub ./cmd/hub

FROM alpine:3.20

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

LABEL org.opencontainers.image.title="hub" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${DATE}"

RUN apk add --no-cache ca-certificates tzdata && adduser -D -H hub

WORKDIR /app

COPY --from=builder /out/hub /app/hub
COPY web /app/web

RUN mkdir -p /app/data /app/backups && chown -R hub:hub /app

USER hub

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD ["/app/hub", "healthcheck"]

CMD ["/app/hub"]
