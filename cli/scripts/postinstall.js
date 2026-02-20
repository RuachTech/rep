#!/usr/bin/env node
/**
 * Postinstall script for @rep-protocol/cli
 * Attempts to bundle the rep-gateway binary for the current platform.
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const GATEWAY_BIN_DIR = path.join(__dirname, '../bin/gateway');
const GATEWAY_BIN_NAME = process.platform === 'win32' ? 'rep-gateway.exe' : 'rep-gateway';
const GATEWAY_BIN_PATH = path.join(GATEWAY_BIN_DIR, GATEWAY_BIN_NAME);

function detectPlatform() {
  const platform = process.platform;
  const arch = process.arch;
  
  // Map Node.js platform/arch to gateway build targets
  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows',
  };
  
  const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64',
  };
  
  const mappedPlatform = platformMap[platform];
  const mappedArch = archMap[arch];
  
  if (!mappedPlatform || !mappedArch) {
    return null;
  }
  
  return { platform: mappedPlatform, arch: mappedArch };
}

function findLocalGatewayBinary() {
  // Try to find a locally built gateway binary (for development)
  const binName = process.platform === 'win32' ? 'rep-gateway.exe' : 'rep-gateway';
  const localPaths = [
    path.join(__dirname, '../../gateway/bin/', binName),
    path.join(__dirname, '../../gateway/bin/rep-gateway-' + process.platform + '-' + process.arch),
  ];
  
  for (const localPath of localPaths) {
    if (fs.existsSync(localPath)) {
      return localPath;
    }
  }
  
  return null;
}

function copyBinary(sourcePath) {
  try {
    // Ensure target directory exists
    if (!fs.existsSync(GATEWAY_BIN_DIR)) {
      fs.mkdirSync(GATEWAY_BIN_DIR, { recursive: true });
    }
    
    // Copy the binary
    fs.copyFileSync(sourcePath, GATEWAY_BIN_PATH);
    
    // Make it executable (Unix-like systems)
    if (process.platform !== 'win32') {
      fs.chmodSync(GATEWAY_BIN_PATH, 0o755);
    }
    
    console.log('✓ Gateway binary installed successfully');
    return true;
  } catch (err) {
    console.error('✗ Failed to copy gateway binary:', err.message);
    return false;
  }
}

function buildGatewayLocally() {
  const gatewayDir = path.join(__dirname, '../../gateway');
  
  if (!fs.existsSync(gatewayDir)) {
    return false;
  }
  
  console.log('Attempting to build gateway locally...');
  
  try {
    execSync('make build', {
      cwd: gatewayDir,
      stdio: 'inherit',
    });
    
    const builtBinary = path.join(gatewayDir, 'bin/rep-gateway');
    if (fs.existsSync(builtBinary)) {
      return copyBinary(builtBinary);
    }
  } catch (err) {
    console.error('Failed to build gateway:', err.message);
  }
  
  return false;
}

function main() {
  console.log('Installing REP gateway binary...');
  
  const target = detectPlatform();
  if (!target) {
    console.warn('⚠ Unsupported platform:', process.platform, process.arch);
    console.warn('  You will need to build the gateway manually or specify --gateway-bin');
    return;
  }
  
  console.log(`Platform: ${target.platform}-${target.arch}`);
  
  // Strategy 1: Look for a locally built binary (development mode)
  const localBinary = findLocalGatewayBinary();
  if (localBinary) {
    console.log('Found local gateway binary:', localBinary);
    if (copyBinary(localBinary)) {
      return;
    }
  }
  
  // Strategy 2: Try to build it locally if we're in the monorepo
  if (buildGatewayLocally()) {
    return;
  }
  
  // Strategy 3: In the future, download from GitHub releases
  // For now, just provide helpful instructions
  console.warn('⚠ Gateway binary not found');
  console.warn('');
  console.warn('To use the `rep dev` command, you need the rep-gateway binary.');
  console.warn('');
  console.warn('Option 1: Build it manually');
  console.warn('  cd gateway && make build');
  console.warn('  Then run: node cli/scripts/postinstall.js');
  console.warn('');
  console.warn('Option 2: Specify the binary path when running');
  console.warn('  rep dev --gateway-bin /path/to/rep-gateway');
  console.warn('');
}

main();
