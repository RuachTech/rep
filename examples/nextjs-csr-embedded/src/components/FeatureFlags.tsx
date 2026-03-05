'use client';

// Demonstrates hot-reload-aware feature flag consumption.
// When the Kubernetes ConfigMap is updated and a SIGHUP is sent to the gateway,
// active flags re-render automatically without a page refresh.
import { useRep } from '@rep-protocol/react';

const ALL_FLAGS = ['dark-mode', 'new-checkout', 'ai-assist', 'beta-pricing'] as const;

export function FeatureFlags() {
  // useRep() subscribes to hot reload: the component re-renders whenever
  // FEATURE_FLAGS changes (e.g., after a ConfigMap update + SIGHUP).
  const rawFlags = useRep('FEATURE_FLAGS', '');
  const active   = new Set(rawFlags.split(',').map(f => f.trim()).filter(Boolean));

  return (
    <section>
      <h2>Feature Flags</h2>
      <p style={{ fontSize: '0.875rem', color: '#6b7280' }}>
        Managed via <code>REP_PUBLIC_FEATURE_FLAGS</code> in the Kubernetes ConfigMap.
        Update the ConfigMap and send SIGHUP to the gateway to reload without restarting pods.
      </p>
      <ul style={{ listStyle: 'none', padding: 0 }}>
        {ALL_FLAGS.map(flag => (
          <li key={flag} style={{ padding: '0.4rem 0', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <span style={{ fontSize: '1.2rem' }}>{active.has(flag) ? '✅' : '⬜'}</span>
            <code>{flag}</code>
          </li>
        ))}
      </ul>
      <p style={{ fontSize: '0.875rem', color: '#6b7280' }}>
        Raw value: <code>{rawFlags || '(empty)'}</code>
      </p>
    </section>
  );
}
