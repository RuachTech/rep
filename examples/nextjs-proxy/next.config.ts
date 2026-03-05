import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // REP proxy mode: the gateway sits in front of the Next.js server.
  // No special Next.js config is needed — REP works by intercepting HTML
  // responses from the Next.js server and injecting the __rep__ payload.
  //
  // The only convention required is that client components read config
  // via rep.get() / useRep() instead of process.env.NEXT_PUBLIC_*.
};

export default nextConfig;
