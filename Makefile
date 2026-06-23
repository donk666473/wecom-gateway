.PHONY: build run test clean lint

# 项目名称
APP_NAME := wecom-gateway
# 主入口
MAIN_PATH := ./cmd/server
# 输出目录
BUILD_DIR := ./build

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# 编译参数
LDFLAGS := -X 'github.com/wecom-gateway/version.GitCommitVersion=$(GIT_COMMIT)' \
           -X 'github.com/wecom-gateway/version.BuildTime=$(BUILD_TIME)' \
           -X 'github.com/wecom-gateway/version.Version=$(VERSION)'

# ============================================================================
# 构建
# ============================================================================

## build: 编译项目
build:
	@echo "Building $(APP_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

## build-linux: 交叉编译 Linux 版本
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux $(MAIN_PATH)

## build-windows: 交叉编译 Windows 版本
build-windows:
	@echo "Building $(APP_NAME) for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME).exe $(MAIN_PATH)

# ============================================================================
# 运行
# ============================================================================

## run: 运行项目（开发模式）
run:
	go run $(MAIN_PATH)/main.go --server.mode=debug --config=config/config.yaml

## run-example: 使用示例配置运行
run-example:
	go run $(MAIN_PATH)/main.go --server.mode=debug --config=config/config.example.yaml

# ============================================================================
# 测试
# ============================================================================

## test: 运行测试
test:
	go test -v -race -coverprofile=coverage.out ./...

## test-cover: 查看测试覆盖率
test-cover: test
	go tool cover -html=coverage.out -o coverage.html

# ============================================================================
# 代码质量
# ============================================================================

## lint: 代码检查
lint:
	golangci-lint run ./...

## fmt: 格式化代码
fmt:
	go fmt ./...

## vet: 代码审查
vet:
	go vet ./...

## tidy: 整理依赖
tidy:
	go mod tidy

# ============================================================================
# 工具
# ============================================================================

## clean: 清理构建产物
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## deps: 安装依赖
deps:
	go mod download

## help: 显示帮助信息
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
