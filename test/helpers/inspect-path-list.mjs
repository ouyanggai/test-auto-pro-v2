// 一次性诊断脚本：检查「运行路径选择」弹窗里路径列表的真实 DOM 结构。
// 背景：夜间脚本发现列表只渲染 6 行（#1..#6）、scrollHeight 等于可视高度，
// 因此怀疑这里不是可滚动的虚拟列表，而是带「展开」入口的折叠列表。
// 用法：node test/helpers/inspect-path-list.mjs
import { Browser } from './browser-driver.mjs'
import { gotoPlans, openRunDialog } from './tool-ui.mjs'
import { evalPage } from './page-actions.mjs'

const browser = await Browser.launch({ headless: true })
try {
  await gotoPlans(browser)
  // openRunDialog 内部会自己导航到路径页并等待就绪，不要再手工拼 URL（路由不认识裸 ID）。
  await openRunDialog(browser)
  await new Promise((resolve) => setTimeout(resolve, 1500))

  const out = await evalPage(browser, `
    const list = $('.path-prepare__list');
    if (!list) return { noList: true, body: document.body.innerText.slice(0, 400) };
    const items = $$('.path-prepare__item');
    const parent = list.parentElement;
    return {
      listTag: list.tagName,
      listClass: list.className,
      listClientH: list.clientHeight,
      listScrollH: list.scrollHeight,
      listOverflow: getComputedStyle(list).overflowY,
      itemCount: items.length,
      itemTexts: items.map((el) => textOf(el).slice(0, 50)),
      parentClass: parent ? parent.className : '-',
      parentClientH: parent ? parent.clientHeight : 0,
      parentScrollH: parent ? parent.scrollHeight : 0,
      headerText: $('.path-prepare__header') ? textOf($('.path-prepare__header')).slice(0, 200) : '-',
      panelText: $('.run-preflight') ? textOf($('.run-preflight')).slice(0, 500) : '-'
    };
  `)
  console.log(JSON.stringify(out, null, 1))
} finally {
  await browser.close()
}
