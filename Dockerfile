# syntax=docker/dockerfile:1.11

############################
# STAGE 1: Build with cache
############################
FROM docker.1ms.run/golang:1.24 AS builder

WORKDIR /app

ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.org
#ENV GOPRIVATE=""
#ENV GONOSUMDB=""

# 单独复制 mod 文件以优化缓存
COPY go.mod go.sum ./

# 下载依赖，使用缓存挂载（BuildKit）
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# 复制源代码
COPY . .

# 编译程序，使用构建缓存
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o /go/bin/app ./main.go

############################
# STAGE 2: Minimal run image
############################
FROM docker.1ms.run/alpine:3.18

COPY --from=builder /go/bin/app /app

USER root:root

ENTRYPOINT ["/app"]

