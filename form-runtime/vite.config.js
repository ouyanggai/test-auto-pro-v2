import { defineConfig } from 'vite'
import vue2 from '@vitejs/plugin-vue2'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue2()],
  resolve: {
    extensions: ['.mjs', '.js', '.mts', '.ts', '.jsx', '.tsx', '.json', '.vue'],
    alias: {
      '@': fileURLToPath(new URL('./upstream/src', import.meta.url)),
      '~@': fileURLToPath(new URL('./upstream/src', import.meta.url)),
      '@local': fileURLToPath(new URL('./src', import.meta.url)),
      '@vendor': fileURLToPath(new URL('./vendor/form-making', import.meta.url)),
    },
  },
  base: './',
  build: {
    outDir: process.env.FORM_RUNTIME_OUT_DIR || '../web/dist/form-runtime',
    emptyOutDir: false,
    commonjsOptions: {
      include: [/vendor\/form-making/, /node_modules/],
    },
  },
})
