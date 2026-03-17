import type { Metadata } from 'next';
import { RepScript } from '@rep-protocol/next';

export const metadata: Metadata = {
  title: 'REP — Next.js Proxy Example',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <head>
        {/* Dev mode: RepScript injects the REP payload so the SDK works
            without running the Go gateway. Returns null in production —
            the gateway handles injection in prod. */}
        <RepScript />
      </head>
      <body>{children}</body>
    </html>
  );
}
