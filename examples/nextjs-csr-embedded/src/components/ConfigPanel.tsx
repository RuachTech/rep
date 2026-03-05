'use client';

import { useRep, useRepSecure } from '@rep-protocol/react';
import { rep } from '@rep-protocol/sdk';

export function ConfigPanel() {
  const apiUrl  = useRep('API_URL',  'http://localhost:3001');
  const envName = useRep('ENV_NAME', 'development');
  const meta    = rep.meta();

  const { value: analyticsKey, loading } = useRepSecure('ANALYTICS_KEY');

  return (
    <section>
      <h2>Runtime Config</h2>
      <table style={{ borderCollapse: 'collapse', width: '100%' }}>
        <tbody>
          <Row label="ENV_NAME"     value={envName} />
          <Row label="API_URL"      value={apiUrl} />
          <Row
            label="ANALYTICS_KEY"
            value={loading ? 'decrypting…' : (analyticsKey ? `${analyticsKey.slice(0, 4)}…` : 'not set')}
          />
        </tbody>
      </table>

      {meta && (
        <>
          <h2>Payload Metadata</h2>
          <table style={{ borderCollapse: 'collapse', width: '100%' }}>
            <tbody>
              <Row label="REP version"  value={meta.version} />
              <Row label="Injected at"  value={meta.injectedAt} />
            </tbody>
          </table>
        </>
      )}
    </section>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <tr style={{ borderBottom: '1px solid #e5e7eb' }}>
      <td style={{ padding: '0.5rem', fontWeight: 600, width: 180 }}>{label}</td>
      <td style={{ padding: '0.5rem' }}><code>{value}</code></td>
    </tr>
  );
}
