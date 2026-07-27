import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', {
  state: () => ({
    productName: '流程自动化测试平台',
  }),
})
