// 冒烟：确认无头本机 Chrome 能被 CDP 驱动，并真实打开工具前端页面。
import { Browser } from './browser-driver.mjs'

const browser = await Browser.launch({ headless: true })
await browser.goto('http://127.0.0.1:19000/')
const title = await browser.eval('document.title')
const text = await browser.eval('document.body.innerText.slice(0,300)')
console.log('title =', title)
console.log('text =', JSON.stringify(text))
await browser.screenshot('logs/night-evidence-3/冒烟_工具首页.png')
browser.close()
