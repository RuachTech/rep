// Server component — renders the shell.
// REP SDK access happens in the EnvDisplay client component below.
import { EnvDisplay } from '@/components/EnvDisplay';

export default function HomePage() {
  return (
    <main style={{ fontFamily: 'system-ui', maxWidth: 640, margin: '4rem auto', padding: '0 1rem' }}>
      <h1>REP — Next.js Proxy Mode</h1>
      <p>
        The REP gateway is running in <strong>proxy mode</strong> in front of this
        Next.js server. It intercepts HTML responses and injects environment variables
        before the browser receives them. No rebuilds required when config changes.
      </p>
      {/* Client component reads REP vars from the injected payload */}
      <EnvDisplay />
    </main>
  );
}
