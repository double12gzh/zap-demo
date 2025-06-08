# --------------------------
# 变量定义
APP_NAME := myapp
VERSION  := latest
IMAGE    := $(APP_NAME):$(VERSION)
BINARY   := app
ENTRY    ?= ./cmd/main.go
TEST_FUNC ?= .*  # 默认运行所有测试

# 可配置变量
PKG := ./...
GOFILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

export DOCKER_BUILDKIT = 1

# 默认目标
.DEFAULT_GOAL := help

# ANSI 颜色
COLOR_GREEN  = \033[32m
COLOR_CYAN   = \033[36m
COLOR_YELLOW = \033[33m
COLOR_RESET  = \033[0m

# --------------------------
.PHONY: help
help: ## 显示所有支持的目标及参数说明
	@printf "Usage: make <target>\n\n"
	@printf "Targets:\n"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| sort \
		| awk -F ':.*?## ' '{printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@printf "\nVariables:\n"
	@printf "  %-10s = %-20s # 用于: image, image-no-cache, run, clean\n" "APP_NAME" "$(APP_NAME)"
	@printf "  %-10s = %-20s # 用于: image, image-no-cache, run, clean\n" "VERSION" "$(VERSION)"
	@printf "  %-10s = %-20s # 用于: image, image-no-cache, run, clean\n" "IMAGE" "$(IMAGE)"
	@printf "  %-10s = %-20s # 用于: build, clean\n" "BINARY" "$(BINARY)"
	@printf "  %-10s = %-20s # 用于: build\n" "ENTRY" "$(ENTRY)"
	@printf "  %-10s = %-20s # 用于: test\n" "TEST_FUNC" "$(TEST_FUNC)"

# --------------------------
.PHONY: build
build: ## 本地编译 Go 二进制文件
	@printf ">> ${COLOR_CYAN}Building Go binary${COLOR_RESET}\n"
	GO111MODULE=on CGO_ENABLED=0 go build -o bin/$(BINARY) $(ENTRY)
	@printf ">> ✅ ${COLOR_GREEN}Binary built: bin/%s${COLOR_RESET}\n" "$(BINARY)"

# --------------------------
.PHONY: image
image: ## 构建 Docker 镜像，使用缓存
	@printf ">> ${COLOR_CYAN}Building Docker image ${COLOR_GREEN}%s${COLOR_RESET} with cache...\n" "$(IMAGE)"
	docker build -t $(IMAGE) .
	@printf ">> ✅ ${COLOR_GREEN}Image built: %s${COLOR_RESET}\n" "$(IMAGE)"

# --------------------------
.PHONY: image-no-cache
image-no-cache: ## 重新构建 Docker 镜像，不使用缓存
	@printf ">> ${COLOR_YELLOW}Rebuilding Docker image ${COLOR_GREEN}%s${COLOR_RESET} without cache...\n" "$(IMAGE)"
	docker build --no-cache -t $(IMAGE) .
	@printf ">> ✅ ${COLOR_GREEN}Image rebuilt: %s${COLOR_RESET}\n" "$(IMAGE)"

# --------------------------
.PHONY: run
run: ## 启动构建好的 Docker 容器
	@printf ">> ${COLOR_CYAN}Running Docker container from image ${COLOR_GREEN}%s${COLOR_RESET}...\n" "$(IMAGE)"
	docker run --rm -it $(IMAGE)

# --------------------------
.PHONY: clean
clean: ## 清除本地二进制和镜像
	@printf ">> ${COLOR_YELLOW}Cleaning local binary and image...${COLOR_RESET}\n"
	@rm -rf bin/
	-docker rmi -f $(IMAGE) >/dev/null 2>&1 || true
	@printf ">> ✅ ${COLOR_GREEN}Clean done${COLOR_RESET}\n"

# --------------------------
.PHONY: clean-cache
clean-cache: ## 清除 Docker 构建缓存
	@printf ">> ${COLOR_YELLOW}Cleaning Docker build cache...${COLOR_RESET}\n"
	docker builder prune -f

# --------------------------
.PHONY: test
test: ## 运行测试 (用法: make test TEST_FUNC=TestConcurrentPing)
	@printf ">> ${COLOR_CYAN}Running tests for function: ${COLOR_YELLOW}%s${COLOR_RESET}\n" "$(TEST_FUNC)"
	go test -v -count=1 -cover $(PKG) -gcflags="all=-N -l" -run $(TEST_FUNC)
	@printf ">> ✅ ${COLOR_GREEN}Tests passed${COLOR_RESET}\n"

# --------------------------
.PHONY: fmt
fmt: ## 格式化代码（go fmt）
	go fmt $(PKG)

# --------------------------
.PHONY: imports
imports: ## 使用 goimports 自动组织 import 和格式化代码
	goimports -w $(GOFILES)

# --------------------------
.PHONY: lint
lint: ## 执行静态分析 lint
	golangci-lint run

# --------------------------
.PHONY: format
format: fmt imports ## 一键格式化所有内容

# --------------------------
.PHONY: check
check: fmt imports lint ## 一键检查所有问题

