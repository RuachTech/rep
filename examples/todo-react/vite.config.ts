import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    // Emit a single index.html so the REP gateway can inject its <script> tag.
    rollupOptions: {
      input: 'index.html',
    },
  },
});
