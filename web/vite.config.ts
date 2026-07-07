import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: process.env.VITE_API_BASE_URL ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    setupFiles: './src/test/setup.ts',
    coverage: {
      provider: 'v8',
      reporter: ['text-summary', 'html'],
      // Exclude generated/config/entry files so thresholds reflect meaningful code.
      exclude: [
        'src/**/*.{test,spec}.{ts,tsx}',
        'src/**/*.d.ts',
        'src/test/**',
        'src/lib/api/generated/**',
        'src/main.tsx',
        'src/i18n/**',
      ],
      // Start ~5% below the current numbers to block regression; raise over time.
      thresholds: {
        statements: 74,
        branches: 64,
        functions: 72,
        lines: 74,
      },
    },
  },
});
