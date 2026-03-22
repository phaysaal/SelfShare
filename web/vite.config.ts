import { defineConfig } from 'vite';
import solidPlugin from 'vite-plugin-solid';

export default defineConfig({
  plugins: [solidPlugin()],
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/setup': 'http://localhost:8080',
      '/s': 'http://localhost:8080',
    },
  },
  build: {
    target: 'esnext',
    outDir: '../server/web/dist',
    emptyOutDir: true,
  },
});
