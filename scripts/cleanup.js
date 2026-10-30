#!/usr/bin/env node

"use strict";

// postinstall hook (overlay-install / upgrade).
//
// Behaviour change:
//   • We no longer run `tmeet auth logout` on overlay install — the user's
//     login state is now preserved across upgrades.
//   • We DO still need to stop the previous version's long-lived processes
//     (event-bus daemon in particular) so new-vs-old binaries don't end up
//     speaking slightly different protocols against the same socket. The
//     stop command is idempotent: no bus running → exit 0 with
//     state=stopped.
//
// Any failure here is intentionally swallowed — a crashed postinstall must
// not break the user's npm install. The worst case (stale bus survives
// the upgrade) is handled on next `tmeet event *` invocation anyway.

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
    // Unsupported platform: nothing we can call. Exit 0 to not block npm.
    process.exit(0);
}

const binaryPath = path.join(__dirname, "..", "dist", binaryName);

if (!fs.existsSync(binaryPath)) {
    // 二进制不存在，无需清理
    process.exit(0);
}

// 确保可执行权限（Windows 不需要）
if (platform !== "win32") {
    try { fs.chmodSync(binaryPath, 0o755); } catch (_) { /* best-effort */ }
}

// 结束上一个版本仍在跑的长进程（event-bus 等）。
// --force 保证就算 bus 死锁也能被强制清理。
try {
    execFileSync(binaryPath, ["event", "stop", "--force"], { stdio: "inherit" });
} catch (_) {
    // 失败不阻塞 npm 安装流程。
    process.exit(0);
}
