import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Static export — produces a fully client-rendered site in `out/`.
  // The REP gateway serves the output directory in embedded mode.
  // No Node.js server needed; the final container can be FROM scratch.
  output: 'export',

  // Required for static export: disable image optimisation (needs a server)
  images: { unoptimized: true },
};

export default nextConfig;
