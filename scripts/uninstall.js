#!/usr/bin/env node

"use strict";

// Explicit uninstall helper for `@tencentcloud/tmeet`.
//
// npm's own uninstall / preuninstall hooks are not reliably triggered on
// `npm uninstall -g` across OSes and package managers, so we do NOT wire
// this file into package.json. Users (or docs) invoke it directly:
//
//     node $(npm root -g)/@tencentcloud/tmeet/scripts/uninstall.js
//     npm uninstall -g @tencentcloud/tmeet
//
// The script performs three idempotent steps; failures in any one step
// are logged but never abort the next step, so a half-broken install can
// still be fully cleaned up with a single invocation.

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

// Must match scripts/cleanup.js so platform detection stays in sync.
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
const binaryPath = binaryName
    ? path.join(__dirname, "..", "dist", binaryName)
    : "";

// ── Step 1: stop long-lived processes (event bus daemon). ─────────────────
// Must happen before auth logout because logout triggers resource-release
// hooks that assume the bus is already down.
if (binaryPath && fs.existsSync(binaryPath)) {
    if (platform !== "win32") {
        try { fs.chmodSync(binaryPath, 0o755); } catch (_) {}
    }
    try {
        execFileSync(binaryPath, ["event", "stop", "--force"], { stdio: "inherit" });
    } catch (e) {
        console.log(`[tmeet] uninstall: event stop warning: ${e.message}`);
    }

    // ── Step 2: wipe credentials via the public logout path. ──────────────
    // Using the binary (rather than deleting the keychain entry directly)
    // ensures ResourceReleaseHook contracts are honoured.
    try {
        execFileSync(binaryPath, ["auth", "logout"], { stdio: "inherit" });
    } catch (e) {
        console.log(`[tmeet] uninstall: auth logout warning: ${e.message}`);
    }
} else {
    console.log("[tmeet] uninstall: binary already removed, skipping event stop / auth logout");
}

// ── Step 3: nuke the config directory. ────────────────────────────────────
// This covers anything the Go binary could not reach itself (e.g. the
// per-account data dir under ~/.tmeet/data/<open_id>/ created by
// internal/notice). The env var mirrors internal/config.GetConfigDir()
// so a custom-deployed CLI still cleans up the right directory.
try {
    const dir = process.env.TMEET_CLI_CONFIG_DIR
        || path.join(process.env.HOME || os.homedir(), ".tmeet");
    fs.rmSync(dir, { recursive: true, force: true });
    console.log(`[tmeet] uninstall: removed ${dir}`);
} catch (e) {
    console.log(`[tmeet] uninstall: cleanup warning: ${e.message}`);
}
