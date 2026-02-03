# 阶段 1: 编译环境
FROM golang:1.22-alpine AS builder

WORKDIR /src
# 开启 Go 模块代理，加速国内下载
ENV GOPROXY=https://goproxy.cn,direct

# 安装 Git
RUN apk add --no-cache git

# 复制文件
COPY go.mod ./
COPY . .

# 编译
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /app/video-merger main.go

# 阶段 2: 运行环境
FROM alpine:latest

# 安装 FFmpeg (核心依赖)
RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

# 从 builder 拷贝
COPY --from=builder /app/video-merger .
COPY --from=builder /src/static ./static

EXPOSE 8082

CMD ["./video-merger"]