# 阶段 1: 编译
FROM golang:1.22-alpine AS builder

WORKDIR /src
# 设置 GOPROXY 解决国内网络问题
ENV GOPROXY=https://goproxy.cn,direct

# 安装 git (以防万一)
RUN apk add --no-cache git

# 复制依赖配置
COPY go.mod ./
# 复制源码
COPY . .

# 整理依赖并编译
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /app/video-merger main.go

# 阶段 2: 运行
FROM alpine:latest

# 安装 FFmpeg
RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

# 从 builder 复制程序
COPY --from=builder /app/video-merger .
# 复制静态资源
COPY --from=builder /src/static ./static

EXPOSE 8082

CMD ["./video-merger"]