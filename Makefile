# Makefile for Anheyu App

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')

# 构建参数
LDFLAGS = -X 'github.com/anzhiyu-c/anheyu-app/internal/pkg/version.Version=$(VERSION)' \
          -X 'github.com/anzhiyu-c/anheyu-app/internal/pkg/version.Commit=$(COMMIT)' \
          -X 'github.com/anzhiyu-c/anheyu-app/internal/pkg/version.Date=$(DATE)'

# 默认目标
.PHONY: build
build:
	@echo "Building anheyu-app with version $(VERSION)"
	go build -ldflags "$(LDFLAGS)" -o anheyu-app

# Linux AMD64 构建
.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building anheyu-app-linux-amd64 with version $(VERSION)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o anheyu-app-linux-amd64

# Linux ARM64 构建
.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building anheyu-app-linux-arm64 with version $(VERSION)"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o anheyu-app-linux-arm64

# 构建所有平台
.PHONY: build-all
build-all: build build-linux-amd64 build-linux-arm64

# 清理构建文件
.PHONY: clean
clean:
	@echo "Cleaning build artifacts"
	rm -f anheyu-app anheyu-app-linux-amd64 anheyu-app-linux-arm64

# 显示版本信息
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

# 运行测试
.PHONY: test
test:
	go test ./...

# 格式化代码
.PHONY: fmt
fmt:
	go fmt ./...

# 静态检查
.PHONY: vet
vet:
	go vet ./...

# 构建 Docker 镜像
.PHONY: docker
docker: build-linux-amd64
	@echo "Building Docker image"
	docker build -t anheyu/anheyu-backend:latest .

# 开发环境快速启动（原有方式，保持兼容）
.PHONY: dev
dev: build-linux-arm64
	@echo "Starting development environment"
	docker compose down
	docker compose up -d --build

# Docker 开发环境（等效于用户的开发流程）
.PHONY: dev-docker
dev-docker:
	@echo "🚀 Starting Docker development workflow..."
	@echo "📦 Stopping existing services..."
	docker compose down
	@echo "🔨 Building ARM64 binary for Docker..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o anheyu-app
	@echo "🐳 Building and starting Docker services..."
	docker compose up -d --build
	@echo "✅ Development environment ready!"
	@echo "🌐 Application: http://localhost:8091"
	@echo "📊 Version API: http://localhost:8091/api/version"
	@echo "📝 View logs: docker logs anheyu-backend -f"

# GoReleaser 目标
.PHONY: goreleaser-check
goreleaser-check:
	@echo "Checking GoReleaser configuration"
	goreleaser check

.PHONY: goreleaser-build
goreleaser-build: frontend-build
	@echo "Building with GoReleaser (snapshot mode)"
	goreleaser build --snapshot --clean

.PHONY: goreleaser-release
goreleaser-release: frontend-build
	@echo "Creating release with GoReleaser"
	goreleaser release --clean

.PHONY: goreleaser-release-dry
goreleaser-release-dry: frontend-build
	@echo "Dry run release with GoReleaser"
	goreleaser release --skip=publish --clean

# 前端构建
.PHONY: frontend-build
frontend-build:
	@echo "Building frontend assets"
	cd assets && pnpm install && pnpm run build

# 安装 GoReleaser
.PHONY: install-goreleaser
install-goreleaser:
	@echo "Installing GoReleaser"
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "Installing goreleaser..."; \
		go install github.com/goreleaser/goreleaser/v2@latest; \
	else \
		echo "GoReleaser is already installed"; \
		goreleaser --version; \
	fi

# 工具安装检查
.PHONY: check-tools
check-tools:
	@echo "Checking required tools..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed"; exit 1; }
	@command -v pnpm >/dev/null 2>&1 || { echo "pnpm is not installed"; exit 1; }
	@command -v goreleaser >/dev/null 2>&1 || { echo "GoReleaser is not installed, run 'make install-goreleaser'"; exit 1; }
	@echo "✅ All tools are available"

# Swagger 文档生成
.PHONY: swagger
swagger:
	@echo "🔄 Generating Swagger documentation files..."
	@command -v swag >/dev/null 2>&1 || { echo "❌ swag is not installed. Run: make install-swag"; exit 1; }
	swag init --parseDependency --parseInternal
	@echo "✅ Swagger documentation files generated successfully!"
	@echo "📄 Generated files:"
	@echo "   - docs/swagger.json  (OpenAPI JSON format)"
	@echo "   - docs/swagger.yaml  (OpenAPI YAML format)"
	@echo "   - docs/docs.go       (Go embedded docs)"
	@echo ""
	@echo "💡 Import swagger.json or swagger.yaml to your API management tool"

# 安装 Swagger 工具
.PHONY: install-swag
install-swag:
	@echo "📦 Installing swag CLI tool..."
	@echo "ℹ️  swag is used to generate Swagger documentation files from Go annotations"
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✅ swag installed successfully!"
	@echo ""
	@echo "Usage: make swagger"

# 帮助信息
.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "🏗️  Building:"
	@echo "  build              - Build for current platform"
	@echo "  build-linux-amd64  - Build for Linux AMD64"
	@echo "  build-linux-arm64  - Build for Linux ARM64"
	@echo "  build-all          - Build for all platforms"
	@echo "  frontend-build     - Build frontend assets only"
	@echo ""
	@echo "🚀 GoReleaser:"
	@echo "  goreleaser-check   - Check GoReleaser configuration"
	@echo "  goreleaser-build   - Build with GoReleaser (snapshot)"
	@echo "  goreleaser-release - Create release with GoReleaser"
	@echo "  goreleaser-release-dry - Dry run release"
	@echo ""
	@echo "🔧 Tools:"
	@echo "  install-goreleaser - Install GoReleaser"
	@echo "  install-swag       - Install Swagger CLI tool"
	@echo "  check-tools        - Check if required tools are installed"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  swagger            - Generate Swagger documentation files (JSON/YAML)"
	@echo ""
	@echo "🧪 Development:"
	@echo "  test               - Run tests"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run static analysis"
	@echo "  clean              - Clean build artifacts"
	@echo "  version            - Show version information"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  docker             - Build Docker image"
	@echo "  dev                - Start development environment (ARM64)"
	@echo "  dev-docker         - Docker development workflow (ARM64 build + compose)"
	@echo ""
	@echo "❓ Help:"
	@echo "  help               - Show this help"
