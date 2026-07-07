import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// In dev, proxy API + health calls to the Go server so the SPA can use
// same-origin relative URLs (/api/v1/...). In prod, nginx does the same.
const API_TARGET = process.env.API_TARGET || 'http://localhost:8080';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/api': { target: API_TARGET, changeOrigin: true },
      '/healthz': { target: API_TARGET, changeOrigin: true },
      '/readyz': { target: API_TARGET, changeOrigin: true },
    },
  },
});
