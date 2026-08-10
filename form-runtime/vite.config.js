import { defineConfig } from 'vite'
import vue2 from '@vitejs/plugin-vue2'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue2()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  base: './',
  build: {
    outDir: '../web/dist/form-runtime',
    emptyOutDir: false,
    commonjsOptions: {
      include: [/src\/lib\/vue-form-making\/dist/, /node_modules/],
    },
  },
})
