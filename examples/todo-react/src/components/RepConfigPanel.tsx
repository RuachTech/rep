import { meta } from '@rep-protocol/sdk';
import { useRep, useRepSecure } from '@rep-protocol/react';

/**
 * RepConfigPanel — visualises everything the REP gateway injected.
 *
 * Toggle it with the "Show Config" button in App.tsx. In production you'd
 * remove this panel; it exists here purely to demonstrate the SDK API.
 */
export function RepConfigPanel() {
  // PUBLIC tier — useRep() is synchronous. Values are available immediately,
  // before the first render completes. No loading state, no Suspense needed.
  const appTitle = useRep('APP_TITLE', '—');
  const envName  = useRep('ENV_NAME',  '—');
  const apiUrl   = useRep('API_URL',   '—');
  const maxTodos = useRep('MAX_TODOS', '—');

  // SENSITIVE tier — useRepSecure() fetches a single-use session key from
  // /rep/session-key, decrypts the AES-256-GCM blob, and caches the result.
  const { value: analyticsKey, loading, error } = useRepSecure('ANALYTICS_KEY');

  const repMeta = meta();

  return (
    <div style={{
      marginBottom: '1.5rem', padding: '1rem 1.25rem',
      background: '#f0f9ff', borderRadius: '0.5rem',
      border: '1px solid #bae6fd', fontSize: '0.875rem',
    }}>
      <h2 style={{
        margin: '0 0 1rem', fontSize: '0.7rem', fontWeight: 700,
        color: '#0369a1', textTransform: 'uppercase', letterSpacing: '0.08em',
      }}>
        Runtime Config — injected by REP gateway at container start
      </h2>

      {/* ── PUBLIC tier ──────────────────────────────────────────── */}
      <Section label="PUBLIC tier" sublabel="plaintext in page source · safe to view via View Source">
        <Row name="REP_PUBLIC_APP_TITLE"  value={appTitle} />
        <Row name="REP_PUBLIC_ENV_NAME"   value={envName} />
        <Row name="REP_PUBLIC_API_URL"    value={apiUrl} />
        <Row name="REP_PUBLIC_MAX_TODOS"  value={maxTodos} />
      </Section>

      {/* ── SENSITIVE tier ───────────────────────────────────────── */}
      <Section
        label="SENSITIVE tier"
        sublabel="AES-256-GCM encrypted in page source · decrypted via /rep/session-key"
      >
        <Row
          name="REP_SENSITIVE_ANALYTICS_KEY"
          value={
            loading
              ? <em style={{ color: '#6b7280' }}>fetching session key…</em>
              : error
                ? <span style={{ color: '#dc2626' }}>unavailable — {error.message}</span>
                : <code style={{
                    background: '#dbeafe', padding: '0.1rem 0.35rem',
                    borderRadius: '0.2rem', fontSize: '0.8rem',
                  }}>{analyticsKey}</code>
          }
        />
      </Section>

      {/* ── Payload metadata ─────────────────────────────────────── */}
      {repMeta ? (
        <Section label="Payload metadata" sublabel="from _meta field in the injected script tag">
          <Row name="version"         value={repMeta.version} />
          <Row name="injected_at"     value={repMeta.injectedAt.toISOString()} />
          <Row name="integrity"       value={repMeta.integrityValid ? '✓ valid' : '✗ tampered'} />
          <Row name="public vars"     value={String(repMeta.publicCount)} />
          <Row name="sensitive blob"  value={repMeta.sensitiveAvailable ? 'present' : 'none'} />
          <Row name="hot reload"      value={repMeta.hotReloadAvailable ? 'enabled' : 'disabled'} />
        </Section>
      ) : (
        <p style={{ marginTop: '0.5rem', color: '#dc2626', fontSize: '0.8rem' }}>
          No REP payload found. Is the gateway running? See README.md for setup instructions.
        </p>
      )}
    </div>
  );
}

// ─── Small layout helpers ──────────────────────────────────────────────────────

function Section({ label, sublabel, children }: {
  label: string;
  sublabel: string;
  children: React.ReactNode;
}) {
  return (
    <div style={{ marginBottom: '1rem' }}>
      <p style={{ margin: '0 0 0.35rem', fontWeight: 600, fontSize: '0.7rem', color: '#0369a1', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
        {label}
        <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, color: '#7dd3fc', marginLeft: '0.5rem' }}>
          — {sublabel}
        </span>
      </p>
      <table style={{ borderCollapse: 'collapse', width: '100%' }}>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

function Row({ name, value }: { name: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={{ padding: '0.2rem 0.75rem 0.2rem 0', whiteSpace: 'nowrap', verticalAlign: 'top', width: 1 }}>
        <code style={{ fontSize: '0.775rem', color: '#0c4a6e', background: '#e0f2fe', padding: '0.05rem 0.3rem', borderRadius: '0.2rem' }}>
          {name}
        </code>
      </td>
      <td style={{ padding: '0.2rem 0', color: '#111827' }}>
        {value}
      </td>
    </tr>
  );
}
