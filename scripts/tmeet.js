#!/usr/bin/env node

"use strict";

// ── Node.js 最低版本检测（必须用 ES5 语法，确保低版本也能解析）────────────
var nodeMajor = parseInt(process.versions.node.split(".")[0], 10);
if (nodeMajor < 14) {
    var _platform = process.platform;
    var upgradeHint = "";
    if (_platform === "darwin") {
        upgradeHint =
            "  # 推荐使用 nvm（Node 版本管理器）：\n" +
            "  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash\n" +
            "  nvm install 18\n" +
            "  nvm use 18\n\n" +
            "  # 或使用 Homebrew：\n" +
            "  brew install node";
    } else if (_platform === "linux") {
        upgradeHint =
            "  # 推荐使用 nvm（Node 版本管理器）：\n" +
            "  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash\n" +
            "  nvm install 18\n" +
            "  nvm use 18";
    } else {
        upgradeHint = "  请访问 https://nodejs.org/zh-cn/download 下载最新版本";
    }
    console.error(
        "[tmeet] Node.js 版本过低：当前 v" + process.versions.node +
        "，需要 v14 或更高版本。\n\n" +
        "升级方式：\n" + upgradeHint
    );
    process.exit(1);
}

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

// ── 平台 → 产物文件名映射 ────────────────────────────────────────────────────
const PLATFORM_MAP = {
    "darwin-x64":   "tmeet-macOS-Intel",
    "darwin-arm64": "tmeet-macOS-AppleSilicon",
    "linux-x64":    "tmeet-Linux-x86_64",
    "linux-arm64":  "tmeet-Linux-ARM64",
    "win32-x64":    "tmeet-Windows-x86_64.exe",
};

const platform = os.platform();   // darwin | linux | win32
const arch     = os.arch();       // x64 | arm64

const key = `${platform}-${arch}`;
const binaryName = PLATFORM_MAP[key];

if (!binaryName) {
    console.error(`[tmeet] 不支持的平台: ${platform}/${arch}`);
    console.error(`支持的平台: ${Object.keys(PLATFORM_MAP).join(", ")}`);
    process.exit(1);
}

const binaryPath = path.join(__dirname, "..", "dist", binaryName);

if (!fs.existsSync(binaryPath)) {
    console.error(`[tmeet] 找不到可执行文件: ${binaryPath}`);
    console.error("请先运行 bash build.sh 编译产物");
    process.exit(1);
}

// 确保可执行权限（Windows 不需要）
if (platform !== "win32") {
    fs.chmodSync(binaryPath, 0o755);
}

// 透传所有参数并继承 stdio，保持与直接调用二进制完全一致的体验
try {
    execFileSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
    process.exit(err.status != null ? err.status : 1);
}
