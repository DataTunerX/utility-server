# 使用 Go 镜像作为基础镜像
FROM golang:1.17 as builder

# 设置工作目录
WORKDIR /app

# 复制项目文件到工作目录
COPY . .

# 构建可执行文件
RUN go build -o patch-k8s-server

# 创建最终的镜像，使用轻量的 alpine 镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从构建镜像中拷贝可执行文件
COPY --from=builder /app/patch-k8s-server .

# 暴露应用程序端口
EXPOSE 8080

# 启动应用程序
CMD ["./patch-k8s-server"]
