#!/usr/bin/env bash
set -euo pipefail

# ── 配置 ──────────────────────────────────────────────────────────────────────
APP_NAME="tmeet"
OUTPUT_DIR="dist"
MAIN_PKG="."

# 版本号：必须通过第一个参数传入
if [ $# -ge 1 ] && [ -n "$1" ]; then
  VERSION="$1"
else
  echo "❌ 请传入版本号，例如：bash build.sh v1.0.0"
  exit 1
fi
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X tmeet/cmd.Version=${VERSION} -X tmeet/cmd.BuildTime=${BUILD_TIME}"

# ── 编译目标 ──────────────────────────────────────────────────────────────────
# 格式：GOOS/GOARCH/平台友好名称
TARGETS=(
  "darwin/amd64/macOS-Intel"
  "darwin/arm64/macOS-AppleSilicon"
  "linux/amd64/Linux-x86_64"
  "linux/arm64/Linux-ARM64"
  "windows/amd64/Windows-x86_64"
)

# ── 清理并创建输出目录 ────────────────────────────────────────────────────────
rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

echo "版本: ${VERSION}  构建时间: ${BUILD_TIME}"
echo "────────────────────────────────────────"

# ── 开始编译 ──────────────────────────────────────────────────────────────────
for TARGET in "${TARGETS[@]}"; do
  GOOS=$(echo "${TARGET}" | cut -d/ -f1)
  GOARCH=$(echo "${TARGET}" | cut -d/ -f2)
  PLATFORM=$(echo "${TARGET}" | cut -d/ -f3)

  # Windows 可执行文件加 .exe 后缀
  EXT=""
  if [ "${GOOS}" = "windows" ]; then
    EXT=".exe"
  fi

  OUTPUT="${OUTPUT_DIR}/${APP_NAME}-${PLATFORM}${EXT}"

  printf "编译 %-30s → %s\n" "${GOOS}/${GOARCH} (${PLATFORM})" "${OUTPUT}"

  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${OUTPUT}" "${MAIN_PKG}"
done

echo "────────────────────────────────────────"
echo "✅ 全部编译完成，产物位于 ./${OUTPUT_DIR}/"
ls -lh "${OUTPUT_DIR}/"

# ── 同步版本号到 package.json ─────────────────────────────────────────────────
echo ""
echo "🔖 同步版本号到 package.json: ${VERSION}"
# macOS sed 需要 -i '' 参数，Linux 直接 -i
if [[ "$(uname)" == "Darwin" ]]; then
  sed -i '' "s/\"version\": \".*\"/\"version\": \"${VERSION}\"/" package.json
else
  sed -i "s/\"version\": \".*\"/\"version\": \"${VERSION}\"/" package.json
fi

