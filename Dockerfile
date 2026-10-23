# syntax=docker/dockerfile:1

FROM golang:1.27.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/paypact-api \
    ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S paypact \
    && adduser -S -G paypact paypact

WORKDIR /app

COPY --from=build --chown=paypact:paypact /out/paypact-api ./paypact-api

USER paypact

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget --quiet --spider http://127.0.0.1:8080/readyz || exit 1

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/paypact-api"]
