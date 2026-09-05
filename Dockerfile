FROM node:22-bookworm-slim AS frontend-builder

WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-bookworm AS backend-builder

ARG DANMAKU_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${DANMAKU_VERSION}" -o /output/danmaku ./cmd/danmaku

FROM debian:bookworm-slim AS runner

ENV TZ=Asia/Shanghai
WORKDIR /usr/local/danmaku

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend-builder /output/danmaku ./danmaku
COPY --from=frontend-builder /src/frontend/dist/ ./wwwroot/
COPY appsettings.json ./appsettings.json

EXPOSE 80

CMD ["./danmaku"]
