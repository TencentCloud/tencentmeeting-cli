#!/usr/bin/env node

"use strict";

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

const platform = os.platform();
const arch     = os.arch();
const key = `${platform}-${arch}`;
const binaryName = PLATFORM_MAP[key];

if (!binaryName) {
    process.exit(1);
}

const binaryPath = path.join(__dirname, "..", "dist", binaryName);

if (!fs.existsSync(binaryPath)) {
    // 二进制不存在，无需清理
    process.exit(0);
}

// 确保可执行权限（Windows 不需要）
if (platform !== "win32") {
    fs.chmodSync(binaryPath, 0o755);
}

// 执行 tmeet auth logout 清理登录态
try {
    execFileSync(binaryPath, ["auth", "logout"], { stdio: "inherit" });
} catch (err) {
    process.exit(0);
}
