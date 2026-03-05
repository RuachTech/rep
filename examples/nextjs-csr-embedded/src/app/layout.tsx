import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'REP — Next.js CSR Embedded Example',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
