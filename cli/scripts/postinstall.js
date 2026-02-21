#!/usr/bin/env node
/**
 * Postinstall script for @rep-protocol/cli
 * Downloads the rep-gateway binary from GitHub Releases for the current platform.
 * Falls back to a locally-built binary when working inside the monorepo.
 */

'use strict';

const fs = require('fs');
const path = require('path');
const https = require('https');
const os = require('os');
const { execSync } = require('child_process');

const pkg = require('../package.json');
const GATEWAY_VERSION = pkg.gatewayVersion;
const REPO = 'RuachTech/rep';

const GATEWAY_BIN_DIR = path.join(__dirname, '../bin/gateway');
const IS_WINDOWS = process.platform === 'win32';
const GATEWAY_BIN_NAME = IS_WINDOWS ? 'rep-gateway.exe' : 'rep-gateway';
const GATEWAY_BIN_PATH = path.join(GATEWAY_BIN_DIR, GATEWAY_BIN_NAME);

function detectPlatform() {
  const osMap = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
  const archMap = { x64: 'amd64', arm64: 'arm64' };
  const goos = osMap[process.platform];
  const goarch = archMap[process.arch];
  if (!goos || !goarch) return null;
  return { goos, goarch };
}

function findLocalBinary() {
  // When working inside the monorepo, prefer a locally-built binary.
  const local = path.join(__dirname, '../../gateway/bin/', GATEWAY_BIN_NAME);
  return fs.existsSync(local) ? local : null;
}

function installBinary(src) {
  fs.mkdirSync(GATEWAY_BIN_DIR, { recursive: true });
  fs.copyFileSync(src, GATEWAY_BIN_PATH);
  if (!IS_WINDOWS) fs.chmodSync(GATEWAY_BIN_PATH, 0o755);
}

/** Follow redirects and stream response to a file. */
function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      https
        .get(u, { headers: { 'User-Agent': 'rep-cli-postinstall' }, agent: false }, (res) => {
          if ([301, 302, 307, 308].includes(res.statusCode)) {
            follow(res.headers.location);
            return;
          }
          if (res.statusCode !== 200) {
            reject(new Error(`HTTP ${res.statusCode} for ${u}`));
            return;
          }
          const out = fs.createWriteStream(dest);
          res.pipe(out);
          out.on('finish', () => out.close(resolve));
          out.on('error', reject);
        })
        .on('error', reject);
    };
    follow(url);
  });
}

async function downloadBinary(target) {
  const ext = target.goos === 'windows' ? 'zip' : 'tar.gz';
  const archiveName = `rep-gateway_${GATEWAY_VERSION}_${target.goos}_${target.goarch}.${ext}`;
  const url = `https://github.com/${REPO}/releases/download/v${GATEWAY_VERSION}/${archiveName}`;

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'rep-gateway-'));
  const archivePath = path.join(tmpDir, archiveName);

  process.stdout.write(`  Downloading rep-gateway v${GATEWAY_VERSION} (${target.goos}/${target.goarch})... `);
  try {
    await download(url, archivePath);
    process.stdout.write('done\n');
  } catch (err) {
    process.stdout.write('failed\n');
    throw new Error(`Download failed: ${err.message}\nURL: ${url}`);
  }

  process.stdout.write('  Extracting... ');
  execSync(`tar -xf "${archivePath}" -C "${tmpDir}"`, { stdio: 'pipe' });
  process.stdout.write('done\n');

  const extracted = path.join(tmpDir, GATEWAY_BIN_NAME);
  if (!fs.existsSync(extracted)) {
    throw new Error(`Binary not found in archive after extraction: ${extracted}`);
  }

  installBinary(extracted);
  fs.rmSync(tmpDir, { recursive: true, force: true });
}

async function main() {
  if (!GATEWAY_VERSION) {
    console.error('\u2717 gatewayVersion is not set in package.json');
    process.exit(1);
  }

  const start = Date.now();
  const elapsed = () => `${((Date.now() - start) / 1000).toFixed(1)}s`;

  console.log(`rep-gateway v${GATEWAY_VERSION}`);

  // Already installed — nothing to do.
  if (fs.existsSync(GATEWAY_BIN_PATH)) {
    console.log(`\u2713 Already installed (${elapsed()})`);
    return;
  }

  const target = detectPlatform();
  if (!target) {
    console.warn(`\u26a0 Unsupported platform: ${process.platform}/${process.arch}`);
    console.warn('  Download manually: https://github.com/RuachTech/rep/releases');
    return;
  }

  // Monorepo dev: use the locally-built binary instead of downloading.
  const local = findLocalBinary();
  if (local) {
    installBinary(local);
    console.log(`\u2713 Done in ${elapsed()} (local build)`);
    return;
  }

  try {
    await downloadBinary(target);
    console.log(`\u2713 Done in ${elapsed()}`);
  } catch (err) {
    console.error(`\u2717 ${err.message}`);
    console.warn('');
    console.warn('Download manually: https://github.com/RuachTech/rep/releases');
    console.warn('Or build locally:  cd gateway && make build');
    // Non-fatal: CLI still installs, gateway binary is just missing.
    process.exitCode = 0;
  }
}

main();
