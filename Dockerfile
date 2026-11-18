FROM node:22-bookworm-slim AS frontend
WORKDIR /web
COPY web/package.json web/tsconfig.json web/vite.config.ts web/index.html ./
COPY web/src ./src
COPY web/public ./public
RUN npm install --no-audit --no-fund && npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /usr/local/bin/miniapp-server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /usr/local/bin/miniapp-testbot ./cmd/testbot

# Bothost may bind-mount /app. Keep executables and built frontend outside it.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates bash
WORKDIR /opt/miniapp
COPY --from=backend /usr/local/bin/miniapp-server /usr/local/bin/miniapp-server
COPY --from=backend /usr/local/bin/miniapp-testbot /usr/local/bin/miniapp-testbot
COPY --from=frontend /web/dist /opt/miniapp/web
COPY config/seed.json /opt/miniapp/config/seed.json
COPY scripts/start-bothost.sh /usr/local/bin/start-bothost
RUN chmod +x /usr/local/bin/start-bothost \
 && test -f /opt/miniapp/web/index.html \
 && bash -n /usr/local/bin/start-bothost

ENV PORT=8080 DATA_DIR=/app/runtime
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/start-bothost"]
