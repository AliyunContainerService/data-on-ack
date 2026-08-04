import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 9528,
    proxy: {
      '/user': 'http://localhost:8080',
      '/researcher': 'http://localhost:8080',
      '/group': 'http://localhost:8080',
      '/user_group': 'http://localhost:8080',
      '/dataset': 'http://localhost:8080',
      '/k8s': 'http://localhost:8080',
      '/dashboard': 'http://localhost:8080',
      '/grafana': 'http://localhost:8080',
      '/login': 'http://localhost:8080',
      '/logout': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
