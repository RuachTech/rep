'use client';

// Verifies the REP payload has not been modified in transit.
//
// The gateway embeds a SHA-256 SRI hash as `data-rep-integrity` on the
// <script id="__rep__"> tag. The SDK recomputes this hash in the browser
// (Web Crypto API) and compares. A mismatch means the payload JSON was
// altered after the gateway injected it — e.g., by a CDN serving a cached
// tampered response or a misconfigured reverse proxy mutating the body.
//
// This is a client-side transit-integrity check. It does not prove the
// payload came from your gateway (no source authentication); it proves the
// content you received was not modified after injection.
import { useEffect, useState } from 'react';
import { rep } from '@rep-protocol/sdk';

type Status = 'checking' | 'ok' | 'failed' | 'unavailable';

const styles: Record<Status, React.CSSProperties> = {
  checking:    { background: '#f3f4f6', color: '#374151', border: '1px solid #e5e7eb' },
  ok:          { background: '#dcfce7', color: '#166534', border: '1px solid #bbf7d0' },
  failed:      { background: '#fee2e2', color: '#991b1b', border: '1px solid #fecaca' },
  unavailable: { background: '#f3f4f6', color: '#374151', border: '1px solid #e5e7eb' },
};

const messages: Record<Status, string> = {
  checking:    '⏳ Verifying payload integrity…',
  ok:          '✅ Integrity verified — payload matches the gateway-injected hash',
  failed:      '🚨 Integrity check FAILED — payload may have been modified in transit. Do not trust these values.',
  unavailable: '⚪ No REP payload detected (running without the gateway?)',
};

export function IntegrityBanner() {
  const [status, setStatus] = useState<Status>('checking');

  useEffect(() => {
    // rep.verify() reads the _tampered flag which is set by an async
    // microtask (_verifySRI) that fires during SDK module init.
    // Awaiting a resolved promise lets that microtask settle first.
    Promise.resolve().then(() => {
      const meta = rep.meta();
      if (!meta) {
        setStatus('unavailable');
      } else if (rep.verify()) {
        setStatus('ok');
      } else {
        setStatus('failed');
        console.error('[REP] Integrity check failed. Payload may have been tampered with.');
      }
    });
  }, []);

  return (
    <div
      role={status === 'failed' ? 'alert' : 'status'}
      style={{
        padding: '0.6rem 1rem',
        borderRadius: 6,
        marginBottom: '1.5rem',
        fontSize: '0.875rem',
        ...styles[status],
      }}
    >
      {messages[status]}
    </div>
  );
}
