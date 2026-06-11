# ============================================================
# Stage 1: Build — 编译 Go 静态二进制
# ============================================================
FROM golang:1.24-alpine AS builder

# 安装 git（Go module 下载需要）
RUN apk --no-cache add git

WORKDIR /build

# ----------------------------------------
# Layer 1: 先拷贝 go.mod / go.sum，利用 Docker 层缓存加速依赖下载
# ----------------------------------------
COPY cmd/blog-agent/go.mod cmd/blog-agent/go.sum ./cmd/blog-agent/

# 拷贝共享模块 —— replace 指令指向 ../common/*
COPY cmd/common/ ./cmd/common/

# 为所有 replace 到 ./pkgs/ 的本地模块创建占位目录，
# 使 go mod download 能正常跳过本地模块、只下载外部依赖
RUN grep '=> \./pkgs/' cmd/blog-agent/go.mod \
    | sed 's|.*=> \./pkgs/\([^/]*\).*|\1|' \
    | sort -u \
    | xargs -I{} mkdir -p cmd/blog-agent/pkgs/{}

WORKDIR /build/cmd/blog-agent
RUN go mod download

# ----------------------------------------
# Layer 2: 拷贝全部源码并编译
# ----------------------------------------
WORKDIR /build
COPY cmd/blog-agent/ ./cmd/blog-agent/

WORKDIR /build/cmd/blog-agent
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /build/go_blog .

# ============================================================
# Stage 2: Runtime — 最小运行时镜像
# ============================================================
FROM alpine:3.21

# 安装运行时依赖：证书、时区、健康检查工具
RUN apk --no-cache add ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建非 root 用户
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

WORKDIR /app

# 从构建阶段拷贝二进制
COPY --from=builder /build/go_blog .

# 拷贝模板和静态资源（运行时从文件系统加载，非 embed）
COPY cmd/blog-agent/templates/ ./templates/
COPY cmd/blog-agent/statics/    ./statics/

# 创建数据和日志目录
RUN mkdir -p /app/blogs_txt /app/logs && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 8888

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -sf http://localhost:8888/ || exit 1

ENTRYPOINT ["./go_blog"]
CMD ["/app/blogs_txt/sys_conf.md"]
