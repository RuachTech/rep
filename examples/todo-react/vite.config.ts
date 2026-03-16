import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { repPlugin } from '@rep-protocol/vite';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // Injects REP <script> tag during `vite dev` so the SDK works
    // without running the Go gateway. No-op during `vite build`.
    repPlugin({ env: '.env.local' }),
  ],
  build: {
    outDir: 'dist',
    // Emit a single index.html so the REP gateway can inject its <script> tag.
    rollupOptions: {
      input: 'index.html',
    },
  },
});
