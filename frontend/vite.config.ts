import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  base: '/kmanager/', // 生产构建的基础路径（统一运维平台纳管要求的应用前缀）
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    minify: 'terser',
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom', 'react-router-dom'],
          antd: ['antd', '@ant-design/icons'],
          charts: ['echarts', 'echarts-for-react'],
        },
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // 前缀与后端路由保持一致，后端已挂载在 /kmanager/api/v1，无需改写路径
      '/kmanager/api': {
        target: 'http://localhost:18080',
        changeOrigin: true,
      },
    },
  },
})