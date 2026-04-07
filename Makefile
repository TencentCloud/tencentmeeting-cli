
# ── 配置 ──────────────────────────────────────────────────────────────────────
APP_NAME   := tmeet
OUTPUT_DIR := dist
MAIN_PKG   := .

# 版本号：通过 make install VERSION=v1.0.0 传入
VERSION    ?=
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -s -w -X tmeet/cmd.Version=$(VERSION) -X tmeet/cmd.BuildTime=$(BUILD_TIME)

# 编译目标列表：GOOS/GOARCH/平台友好名称
TARGETS := \
	darwin/amd64/macOS-Intel \
	darwin/arm64/macOS-AppleSilicon \
	linux/amd64/Linux-x86_64 \
	linux/arm64/Linux-ARM64 \
	windows/amd64/Windows-x86_64

# ── 默认目标 ──────────────────────────────────────────────────────────────────
.PHONY: install clean build sync-version

# ── 编译 ──────────────────────────────────────────────────────────────────────
build:
ifndef VERSION
	$(error ❌ 请传入版本号，例如：make install VERSION=v1.0.0)
endif
	@rm -rf $(OUTPUT_DIR)
	@mkdir -p $(OUTPUT_DIR)
	@echo "版本: $(VERSION)  构建时间: $(BUILD_TIME)"
	@echo "────────────────────────────────────────"
	@$(foreach target,$(TARGETS), \
		$(eval GOOS   := $(word 1,$(subst /, ,$(target)))) \
		$(eval GOARCH := $(word 2,$(subst /, ,$(target)))) \
		$(eval PLAT   := $(word 3,$(subst /, ,$(target)))) \
		$(eval EXT    := $(if $(filter windows,$(GOOS)),.exe,)) \
		$(eval OUT    := $(OUTPUT_DIR)/$(APP_NAME)-$(PLAT)$(EXT)) \
		printf "编译 %-30s → %s\n" "$(GOOS)/$(GOARCH) ($(PLAT))" "$(OUT)" && \
		CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT) $(MAIN_PKG) && \
	) true
	@echo "────────────────────────────────────────"
	@echo "✅ 全部编译完成，产物位于 ./$(OUTPUT_DIR)/"
	@ls -lh $(OUTPUT_DIR)/

# ── 同步版本号到 package.json ─────────────────────────────────────────────────
sync-version:
ifndef VERSION
	$(error ❌ 请传入版本号，例如：make install VERSION=v1.0.0)
endif
	@echo ""
	@echo "🔖 同步版本号到 package.json: $(VERSION)"
ifeq ($(shell uname),Darwin)
	@sed -i '' 's/"version": ".*"/"version": "$(VERSION)"/' package.json
else
	@sed -i 's/"version": ".*"/"version": "$(VERSION)"/' package.json
endif

# ── 清理 ──────────────────────────────────────────────────────────────────────
clean:
	@rm -rf $(OUTPUT_DIR)
	@echo "🧹 已清理 $(OUTPUT_DIR)/"
