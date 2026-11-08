# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24

# =========================
# Build Stage
# =========================
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o server ./cmd/server


# =========================
# Runtime Stage
# =========================
FROM alpine:3.23

RUN apk add --no-cache \
    ca-certificates \
    postgresql18-client \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    tzdata \
    && update-ca-certificates

RUN pg_dump --version

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV TZ=Asia/Jakarta

RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone

WORKDIR /app

COPY --from=build /app/server /app/server
COPY --from=build /app/migrations /app/db/migrations
COPY --from=build /app/assets /app/assets

ENV PORT=8080
EXPOSE 8080

CMD ["/app/server"]
