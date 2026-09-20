// 页面交互通用层：只依赖可见文本、角色与位置做定位，不写死任何业务 ID。
// 所有点击都在页面里派发真实鼠标事件（pointerdown/mousedown/mouseup/click），
// 与执行服务 native 动作的做法一致，保证触发的是前端真实交互逻辑。
import { delay } from './browser-driver.mjs'

// PAGE_BOOTSTRAP 定义了页面里反复复用的查询工具函数，注入到每个页面上下文。
// 使用方式：eval(`(() => { ${PAGE_BOOTSTRAP}; return ... })()`)
export const PAGE_BOOTSTRAP = `
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const $ = (sel, root = document) => root.querySelector(sel);
const visible = (el) => {
  if (!el) return false;
  const rect = el.getBoundingClientRect();
  if (rect.width < 1 || rect.height < 1) return false;
  const style = getComputedStyle(el);
  if (style.visibility === 'hidden' || style.display === 'none' || style.opacity === '0') return false;
  return true;
};
const textOf = (el) => (el ? (el.innerText || el.textContent || '').replace(/\\s+/g, ' ').trim() : '');
const fire = (el) => {
  if (!el) return false;
  const rect = el.getBoundingClientRect();
  const x = rect.left + rect.width / 2;
  const y = rect.top + rect.height / 2;
  const base = { bubbles: true, cancelable: true, composed: true, clientX: x, clientY: y, view: window, button: 0 };
  el.dispatchEvent(new PointerEvent('pointerdown', { ...base, pointerId: 1, isPrimary: true }));
  el.dispatchEvent(new MouseEvent('mousedown', base));
  el.dispatchEvent(new PointerEvent('pointerup', { ...base, pointerId: 1, isPrimary: true }));
  el.dispatchEvent(new MouseEvent('mouseup', base));
  el.dispatchEvent(new MouseEvent('click', base));
  return true;
};
const setInput = (el, value) => {
  const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
  setter.call(el, value);
  el.dispatchEvent(new Event('input', { bubbles: true }));
  el.dispatchEvent(new Event('change', { bubbles: true }));
  return true;
};
// 按可见文本找元素：优先完全相等，其次包含；skipOverlay 用于跳过遮罩层里的同名文字。
const byText = (tag, text, { exact = true, root = document, nth = 0 } = {}) => {
  const hits = $$(tag, root).filter((el) => {
    if (!visible(el)) return false;
    const t = textOf(el);
    return exact ? t === text : t.includes(text);
  });
  return hits[nth] || null;
};
const byTextAll = (tag, text, { root = document } = {}) => $$(tag, root).filter((el) => visible(el) && textOf(el).includes(text));
// dialogRoots 只认真正叠加在页面之上的浮层；n-card 之类内联容器不算，避免把整页当成弹窗。
const dialogRoots = () => $$('.n-modal, .n-drawer, .n-popover, .n-dropdown-menu').filter(visible);
const topDialog = () => {
  const roots = dialogRoots();
  return roots.length ? roots[roots.length - 1] : document.body;
};
`

// evalPage 在页面上下文里执行一段脚本（自动带上通用工具函数），返回可序列化结果。
// 包装成 async IIFE，脚本体内可以直接用 await（页面内 fetch 读取事实时要用）。
export async function evalPage (browser, body) {
  return await browser.eval(`(async () => { ${PAGE_BOOTSTRAP}\n${body}\n})()`)
}

// waitFor 轮询等待页面脚本返回真值；超时抛出带上下文的错误，便于定位。
export async function waitFor (browser, body, { timeoutMs = 30000, intervalMs = 400, label = '条件' } = {}) {
  const deadline = Date.now() + timeoutMs
  let last = null
  while (Date.now() < deadline) {
    try {
      last = await evalPage(browser, body)
      if (last) return last
    } catch (error) {
      last = `脚本异常：${error.message}`
    }
    await delay(intervalMs)
  }
  const snapshot = await currentView(browser).catch(() => '(无法读取页面)')
  throw new Error(`等待「${label}」超时（${timeoutMs}ms），最后一次结果=${JSON.stringify(last)}\n当前页面：${snapshot.slice(0, 1500)}`)
}

// currentView 返回当前最上层对话框（没有则为整页）的可见文本，用于错误诊断与断言。
export async function currentView (browser) {
  return await evalPage(browser, 'return textOf(topDialog()).slice(0, 4000)')
}

// clickByText 在指定容器范围内按文本点击第一个可见元素，返回是否点到。
// tag 支持逗号分隔的多个选择器，候选按选择器顺序合并后取第 nth 个。
export async function clickByText (browser, text, { tag = 'button, a, span, div, li, label', exact = true, nth = 0, scope = 'topDialog()' } = {}) {
  const result = await evalPage(browser, `
    const root = ${scope};
    const hits = $$(${JSON.stringify(tag)}, root).filter((el) => {
      if (!visible(el)) return false;
      const t = textOf(el);
      return ${exact} ? t === ${JSON.stringify(text)} : t.includes(${JSON.stringify(text)});
    });
    const el = hits[${nth}];
    if (!el) return { ok: false, count: hits.length };
    fire(el);
    return { ok: true, text: textOf(el), count: hits.length };
  `)
  if (!result?.ok) {
    throw new Error(`点不到「${text}」（候选 ${result?.count ?? 0} 个）\n当前页面：${(await currentView(browser)).slice(0, 1200)}`)
  }
  await delay(250)
  return result
}

// clickIfPresent 尝试点击，不存在时返回 false（用于「有就点、没有就继续」的可选交互）。
export async function clickIfPresent (browser, text, options = {}) {
  try {
    await clickByText(browser, text, options)
    return true
  } catch {
    return false
  }
}

// hasText 判断当前最上层视图里是否出现某段文字。
export async function hasText (browser, text, { scope = 'topDialog()' } = {}) {
  return await evalPage(browser, `return textOf(${scope}).includes(${JSON.stringify(text)})`)
}

// fillByPlaceholder 按输入框 placeholder 填值（naive-ui 输入框的稳定锚点）。
export async function fillByPlaceholder (browser, placeholderPart, value, { scope = 'topDialog()', nth = 0 } = {}) {
  const result = await evalPage(browser, `
    const root = ${scope};
    const hits = $$('input, textarea', root).filter((el) => visible(el) && (el.placeholder || '').includes(${JSON.stringify(placeholderPart)}));
    const el = hits[${nth}];
    if (!el) return { ok: false, count: hits.length, placeholders: $$('input, textarea', root).filter(visible).map((e) => e.placeholder) };
    setInput(el, ${JSON.stringify(value)});
    return { ok: true, placeholder: el.placeholder };
  `)
  if (!result?.ok) {
    throw new Error(`找不到 placeholder 含「${placeholderPart}」的输入框，现有：${JSON.stringify(result?.placeholders)}\n当前页面：${(await currentView(browser)).slice(0, 1200)}`)
  }
  await delay(200)
  return result
}

// listVisibleTexts 返回当前视图内所有可见交互元素文本，用来在没有定位锚点时做选择。
export async function listVisibleTexts (browser, { scope = 'topDialog()', limit = 120 } = {}) {
  return await evalPage(browser, `
    const root = ${scope};
    return $$('button, a, .n-button, .n-base-selection-label, .n-checkbox, .n-radio, td, .n-tab', root)
      .filter(visible).map(textOf).filter((t) => t).slice(0, ${limit});
  `)
}

export { delay }
