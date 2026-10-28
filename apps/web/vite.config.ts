import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

const ApiUrl = process.env.DHOW_DEV_API_URL ?? 'http://127.0.0.1:8080';
const Polling = process.env.DHOW_DEV_POLLING === 'true';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./Src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: { '/api': ApiUrl },
    watch: { usePolling: Polling, interval: 300 },
  },
  preview: {
    port: 4173,
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
  build: {
    sourcemap: false,
    assetsInlineLimit: 0,
    modulePreload: { polyfill: false },
  },
});
