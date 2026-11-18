FROM node:22-alpine AS frontend
WORKDIR /web
COPY web/package.json web/tsconfig.json web/vite.config.ts web/index.html ./
COPY web/src ./src
COPY web/public ./public
RUN npm install --no-audit --no-fund && npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=backend /server /server
COPY --from=frontend /web/dist ./web/dist
ENV ADDR=:8080 WEB_DIST=web/dist
EXPOSE 8080
CMD ["/server"]
