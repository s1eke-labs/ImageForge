import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    target: 'safari15',
    cssTarget: 'safari15',
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8020',
    },
  },
})
