import { getOrCreateKeys } from './keys.js';

/**
 * Next.js App Router route handler for /rep/session-key.
 *
 * Usage:
 *   // app/api/rep/session-key/route.ts
 *   export { GET } from '@rep-protocol/next/session-key';
 *
 * Security:
 *   - Returns 404 in production. The gateway serves this endpoint in prod
 *     with rate limiting, single-use tokens, and CORS origin checking.
 *     This dev-only handler has none of those hardening measures.
 *   - Uses standard Response (not NextResponse) to avoid coupling to
 *     next/server internals.
 */
export function GET(): Response {
  if (process.env.NODE_ENV === 'production') {
    return new Response(
      JSON.stringify({
        error: 'Session key endpoint is disabled in production. Use the REP gateway.',
      }),
      {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      },
    );
  }

  const keys = getOrCreateKeys();

  return new Response(
    JSON.stringify({
      key: keys.encryptionKey.toString('base64'),
      expires_at: new Date(Date.now() + 30_000).toISOString(),
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
        'Access-Control-Allow-Origin': '*',
      },
    },
  );
}
