import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Standalone output bundles the Next.js server into .next/standalone.
  // Required for Dockerfile.single (single container: gateway + Next.js).
  // Also compatible with the two-service docker-compose.yml setup.
  output: 'standalone',

  // REP proxy mode: the gateway sits in front of the Next.js server.
  // No special Next.js config is needed beyond the above — REP works by
  // intercepting HTML responses and injecting the __rep__ payload.
  // Client components read config via rep.get() / useRep().
};

export default nextConfig;
