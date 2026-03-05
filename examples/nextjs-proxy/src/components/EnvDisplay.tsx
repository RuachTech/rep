'use client';

// REP SDK runs in the browser — must be in a 'use client' component.
// rep.get() is synchronous: no loading state, no Suspense needed.
import { useRep, useRepSecure } from '@rep-protocol/react';
import { rep } from '@rep-protocol/sdk';

export function EnvDisplay() {
  // Public vars — synchronous, available on first render
  const apiUrl   = useRep('API_URL',   'http://localhost:3001');
  const envName  = useRep('ENV_NAME',  'development');
  const features = useRep('FEATURE_FLAGS', '');

  // Sensitive var — async, decrypted via the session-key endpoint
  const { value: analyticsKey, loading: keyLoading } = useRepSecure('ANALYTICS_KEY');

  // Payload metadata
  const meta = rep.meta();

  return (
    <section>
      <h2>Runtime Config</h2>
      <table style={{ borderCollapse: 'collapse', width: '100%' }}>
        <tbody>
          <Row label="ENV_NAME"      value={envName} />
          <Row label="API_URL"       value={apiUrl} />
          <Row label="FEATURE_FLAGS" value={features || '(none)'} />
          <Row
            label="ANALYTICS_KEY"
            value={keyLoading ? 'decrypting…' : (analyticsKey ? `${analyticsKey.slice(0, 4)}…` : 'not set')}
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
              <Row label="Hot reload"   value={meta.hotReloadEndpoint ?? 'disabled'} />
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
