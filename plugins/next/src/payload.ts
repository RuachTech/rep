import {
  computeIntegrity,
  encryptSensitive,
  computeSRI,
  goEscapeJSON,
  type Keys,
} from './crypto.js';
import type { ClassifiedVars } from './env.js';

export interface PayloadResult {
  json: string;
  scriptTag: string;
  sri: string;
}

export interface PayloadOptions {
  version: string;
  hotReload?: boolean;
}

/**
 * Build the REP payload JSON and script tag.
 * Matches gateway/pkg/payload/payload.go:Build + ScriptTag exactly.
 *
 * JSON field order must match Go struct serialization:
 *   { "public": {...}, "sensitive": "...", "_meta": {...} }
 *
 * All string values get Go HTML escaping (< > & → \u003c \u003e \u0026).
 */
export function buildPayload(
  classified: ClassifiedVars,
  keys: Keys,
  options: PayloadOptions,
): PayloadResult {
  const publicMap = classified.public;
  const sensitiveMap = classified.sensitive;
  const hasSensitive = Object.keys(sensitiveMap).length > 0;

  // Step 1: Compute integrity over public vars only (empty sensitive blob).
  // This matches Go's payload.go — integrity is computed BEFORE encryption,
  // and the integrity token is used as AAD for AES-GCM.
  const integrity = computeIntegrity(publicMap, '', keys.hmacSecret);

  // Step 2: Encrypt sensitive vars if present.
  let sensitiveBlob = '';
  if (hasSensitive) {
    sensitiveBlob = encryptSensitive(sensitiveMap, keys.encryptionKey, integrity);
  }

  // Step 3: Build the payload JSON manually to match Go's field order.
  // Go's json.Marshal produces fields in struct declaration order:
  //   public, sensitive (omitempty), _meta
  const injectedAt = new Date().toISOString();

  // Build _meta object
  const metaParts: string[] = [
    `"version":${goEscapeJSON(JSON.stringify(options.version))}`,
    `"injected_at":${goEscapeJSON(JSON.stringify(injectedAt))}`,
    `"integrity":${goEscapeJSON(JSON.stringify(integrity))}`,
  ];

  if (hasSensitive) {
    metaParts.push(`"key_endpoint":"/rep/session-key"`);
  }

  if (options.hotReload) {
    metaParts.push(`"hot_reload":"/rep/changes"`);
  }

  metaParts.push(`"ttl":0`);

  // Build public object with sorted keys + Go escaping
  const publicSorted = Object.keys(publicMap).sort();
  const publicParts = publicSorted.map(
    k => `${goEscapeJSON(JSON.stringify(k))}:${goEscapeJSON(JSON.stringify(publicMap[k]))}`,
  );
  const publicJSON = `{${publicParts.join(',')}}`;

  // Assemble top-level payload
  const parts: string[] = [`"public":${publicJSON}`];

  if (hasSensitive) {
    parts.push(`"sensitive":${goEscapeJSON(JSON.stringify(sensitiveBlob))}`);
  }

  parts.push(`"_meta":{${metaParts.join(',')}}`);

  const json = `{${parts.join(',')}}`;
  const jsonBytes = Buffer.from(json, 'utf-8');
  const sri = computeSRI(jsonBytes);

  const scriptTag =
    `<script id="__rep__" type="application/json" data-rep-version="${options.version}" data-rep-integrity="${sri}">${json}</script>`;

  return { json, scriptTag, sri };
}
