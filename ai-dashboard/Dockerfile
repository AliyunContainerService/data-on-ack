# ---- Stage 1: Build frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --legacy-peer-deps || npm install --legacy-peer-deps
COPY frontend/ ./
RUN npm run build

# ---- Stage 2: Build Go backend ----
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY backend/ ./backend/
RUN CGO_ENABLED=0 GOOS=linux go build -o /ai-dashboard ./backend/cmd/server/

# ---- Stage 3: Runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/UTC /etc/localtime && \
    echo "UTC" > /etc/timezone
WORKDIR /app
COPY --from=backend-builder /ai-dashboard ./ai-dashboard
COPY --from=frontend-builder /app/frontend/dist ./dist
EXPOSE 8080
ENTRYPOINT ["./ai-dashboard"]
