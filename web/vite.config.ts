import path from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// The production build is embedded into the Go binary from internal/webui/dist.
// `npm run dev` serves the UI with hot reload and forwards /api to a running AMS.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    // The app, and the API documentation page served at /docs/.
    rollupOptions: {
      input: { app: path.resolve(__dirname, 'index.html'), docs: path.resolve(__dirname, 'docs/index.html'), clientGuide: path.resolve(__dirname, 'docs/client-integration.html') },
    },
    chunkSizeWarningLimit: 1000,
  },
  server: {
    proxy: { '/api': 'http://127.0.0.1:8099' },
  },
});
