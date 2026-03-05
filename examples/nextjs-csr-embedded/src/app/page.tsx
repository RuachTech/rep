// Static export: all pages are pre-rendered to HTML at build time.
// REP config is injected by the gateway at request time (after build).
import { IntegrityBanner } from '@/components/IntegrityBanner';
import { ConfigPanel } from '@/components/ConfigPanel';
import { FeatureFlags } from '@/components/FeatureFlags';

export default function HomePage() {
  return (
    <main style={{ fontFamily: 'system-ui', maxWidth: 640, margin: '4rem auto', padding: '0 1rem' }}>
      {/* Verifies the payload was not modified in transit (SRI hash check) */}
      <IntegrityBanner />
      <h1>REP — Next.js CSR + Embedded Mode</h1>
      <p>
        This Next.js app is exported as a static site (<code>output: &apos;export&apos;</code>)
        and served by the REP gateway in embedded mode. The gateway injects environment
        variables at request time — the static files are identical across all environments.
      </p>
      <p>
        In Kubernetes, a <strong>ConfigMap</strong> holds all <code>REP_PUBLIC_*</code> vars.
        Update the ConfigMap to change feature flags without rebuilding or redeploying the app.
      </p>
      <ConfigPanel />
      <FeatureFlags />
    </main>
  );
}
