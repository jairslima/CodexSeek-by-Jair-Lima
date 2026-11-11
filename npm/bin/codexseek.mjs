#!/usr/bin/env node
/**
 * CodexSeek by Jair Lima — Cross-platform Node.js ESM Launcher
 * Copyright (c) 2026 Jair Lima. Licensed under MIT & Apache-2.0.
 */
import { spawn, execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const APP_NAME = "CodexSeek by Jair Lima";
const APP_VERSION = "1.0.0";
const ENV_KEY = "DEEPSEEK_API_KEY";
const DEFAULT_MODEL = "deepseek-flash";
const PRO_MODEL = "deepseek-v4-pro";
const DEFAULT_BASE_URL = "https://api.deepseek.com/v1";

const MODEL_ALIASES = {
  flash: "deepseek-flash",
  "deepseek-flash": "deepseek-flash",
  pro: "deepseek-v4-pro",
  "deepseek-pro": "deepseek-v4-pro",
  "deepseek-v4-pro": "deepseek-v4-pro",
  chat: "deepseek-chat",
  "deepseek-chat": "deepseek-chat",
  reasoner: "deepseek-reasoner",
  r1: "deepseek-reasoner",
};

function resolveAlias(m) {
  const k = String(m || "").trim().toLowerCase();
  return MODEL_ALIASES[k] || m;
}

function getStateDir() {
  return process.env.CODEXSEEK_HOME || path.join(os.homedir(), ".codexseek");
}

function resolveApiKey() {
  if (process.env[ENV_KEY] && process.env[ENV_KEY].trim()) {
    return process.env[ENV_KEY].trim();
  }
  const candidates = [
    path.join(os.homedir(), ".codexseek", ".env"),
    path.join(os.homedir(), ".codexseek", "credentials.env"),
    path.join(os.homedir(), ".api_keys", "credentials.env"),
  ];
  for (const file of candidates) {
    if (fs.existsSync(file)) {
      const lines = fs.readFileSync(file, "utf8").split(/\r?\n/);
      for (let line of lines) {
        line = line.trim().replace(/^export\s+/, "");
        if (!line || line.startsWith("#")) continue;
        const idx = line.indexOf("=");
        if (idx === -1) continue;
        const k = line.slice(0, idx).trim();
        const v = line.slice(idx + 1).trim().replace(/^['"]|['"]$/g, "");
        if (k === ENV_KEY && v) return v;
      }
    }
  }
  return "";
}

function findCodexBinary() {
  if (process.env.CODEXSEEK_UPSTREAM_BIN && fs.existsSync(process.env.CODEXSEEK_UPSTREAM_BIN)) {
    return process.env.CODEXSEEK_UPSTREAM_BIN;
  }
  if (process.platform === "win32") {
    const appData = process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming");
    const c = path.join(
      appData,
      "npm",
      "node_modules",
      "@openai",
      "codex",
      "node_modules",
      "@openai",
      "codex-win32-x64",
      "vendor",
      "x86_64-pc-windows-msvc",
      "bin",
      "codex.exe"
    );
    if (fs.existsSync(c)) return c;
  }
  return process.platform === "win32" ? "codex.cmd" : "codex";
}

const rawArgs = process.argv.slice(2);
if (rawArgs[0] === "-V" || rawArgs[0] === "--version" || rawArgs[0] === "version") {
  console.log(`${APP_NAME} v${APP_VERSION}`);
  process.exit(0);
}

const apiKey = resolveApiKey();
if (!apiKey) {
  console.error(`[CodexSeek] Erro: variável ${ENV_KEY} não configurada.`);
  process.exit(1);
}

const codexBin = findCodexBinary();
const stateDir = getStateDir();
const catalogPath = path.join(stateDir, "deepseek_catalog.json");

let selectedModel = process.env.CODEXSEEK_MODEL
  ? resolveAlias(process.env.CODEXSEEK_MODEL)
  : DEFAULT_MODEL;

const cleaned = [];
let isExec = false;
let execIdx = -1;

for (let i = 0; i < rawArgs.length; i++) {
  const a = rawArgs[i];
  if (a === "--pro") {
    selectedModel = PRO_MODEL;
    continue;
  }
  if (a === "--flash") {
    selectedModel = DEFAULT_MODEL;
    continue;
  }
  if ((a === "-m" || a === "--model") && i + 1 < rawArgs.length) {
    selectedModel = resolveAlias(rawArgs[++i]);
    continue;
  }
  if ((a === "exec" || a === "e") && !isExec) {
    isExec = true;
    execIdx = cleaned.length;
  }
  cleaned.push(a);
}

if (isExec && !cleaned.includes("--ignore-user-config")) {
  cleaned.splice(execIdx + 1, 0, "--ignore-user-config");
}

const overlay = [
  "--no-daemon",
  "-m",
  selectedModel,
  "-c",
  'model_provider="deepseek"',
  "-c",
  'model_providers.deepseek.name="DeepSeek"',
  "-c",
  `model_providers.deepseek.base_url="${DEFAULT_BASE_URL}"`,
  "-c",
  `model_providers.deepseek.env_key="${ENV_KEY}"`,
  "-c",
  'model_providers.deepseek.wire_api="responses"',
];

if (fs.existsSync(catalogPath)) {
  overlay.push("-c", `model_catalog_json="${catalogPath.replace(/\\/g, "\\\\")}"`);
}
if (process.platform === "win32") {
  overlay.push("-c", 'windows.sandbox="unelevated"');
}

const child = spawn(codexBin, [...overlay, ...cleaned], {
  stdio: "inherit",
  env: { ...process.env, [ENV_KEY]: apiKey },
});

child.on("exit", (code) => process.exit(code ?? 1));
