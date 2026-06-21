# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.4-bookworm AS build

WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -trimpath \
      -ldflags="-s -w -buildid= -X github.com/isyuricunha/discord-news-rss-bot/internal/version.Version=$VERSION -X github.com/isyuricunha/discord-news-rss-bot/internal/version.Commit=$COMMIT -X github.com/isyuricunha/discord-news-rss-bot/internal/version.Date=$BUILD_DATE" \
      -o /out/discord-rss-bot ./cmd/discord-rss-bot

RUN mkdir -p /out/rootfs/app/data /out/rootfs/tmp && chown -R 65532:65532 /out/rootfs/app /out/rootfs/tmp

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/rootfs/app /app
COPY --from=build --chown=65532:65532 /out/rootfs/tmp /tmp
COPY --from=build /out/discord-rss-bot /discord-rss-bot

WORKDIR /app

ENV RSS_BOT_DATA=/app/data \
    LOG_LEVEL=info \
    LOG_FORMAT=text

USER 65532:65532
ENTRYPOINT ["/discord-rss-bot"]
CMD ["run"]

HEALTHCHECK --interval=5m --timeout=10s --start-period=2m --retries=3 \
  CMD ["/discord-rss-bot", "healthcheck"]
