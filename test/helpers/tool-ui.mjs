// 工具前端（127.0.0.1:19000）的高层页面操作：全部通过真实点击完成，
// 只有「读取事实」类接口（路径列表、运行就绪、历史候选）用页面内 fetch 读取，
// 与前端自身调用完全同源同参，不伪造任何写请求（任务书纪律 3）。
import { clickByText, currentView, delay, evalPage, fillByPlaceholder, waitFor } from './page-actions.mjs'

export const TOOL_ORIGIN = 'http://127.0.0.1:19000'

// gotoPlans 打开测试计划列表页。
export async function gotoPlans (browser) {
  await browser.goto(`${TOOL_ORIGIN}/plans`)
  await waitFor(browser, 'return document.body.innerText.includes("测试计划")', { label: '计划列表页', timeoutMs: 30000 })
}

// createPlan 在页面上真实新建一个计划并进入它的路径页，返回计划 ID。
// 流程从「新建计划」按钮点起，账号与流程模板都在页面上真实选择。
export async function createPlan (browser, { name, account, flowName }) {
  await browser.goto(`${TOOL_ORIGIN}/plans/new`)
  await waitFor(browser, 'return !!document.querySelector("input")', { label: '新建计划页', timeoutMs: 30000 })
  await fillByPlaceholder(browser, '例如：', name, { scope: 'document.body' })
  await fillByPlaceholder(browser, '请输入账号', account, { scope: 'document.body' })
  await clickByText(browser, '验证账号', { scope: 'document.body', tag: '.n-button' })
  await waitFor(browser, 'return document.body.innerText.includes("账号已验证")', { label: '账号验证通过', timeoutMs: 90000 })
  await fillByPlaceholder(browser, '搜索流程模板名称', flowName, { scope: 'document.body' })
  await waitFor(browser, `return $$('.candidate-row').some((el) => visible(el) && textOf(el).includes(${JSON.stringify(flowName)}))`, { label: `候选中出现「${flowName}」`, timeoutMs: 60000 })
  await clickByText(browser, flowName, { scope: 'document.body', tag: '.candidate-row', exact: false })
  await waitFor(browser, `return $$('.candidate-row').some((el) => visible(el) && textOf(el).includes('已选择'))`, { label: '流程已选中', timeoutMs: 15000 })
  await clickByText(browser, '创建并选择路径', { scope: 'document.body', tag: '.n-button' })
  await waitFor(browser, 'return /\\/plans\\/\\d+\\/paths/.test(location.pathname)', { label: '进入路径页', timeoutMs: 60000 })
  return await planIDFromURL(browser)
}

// planIDFromURL 从当前地址解析计划 ID。
export async function planIDFromURL (browser) {
  return await browser.eval('(location.pathname.match(/\\/plans\\/(\\d+)/) || [])[1] || ""')
}

// openPlanPaths 打开指定计划的路径页。
export async function openPlanPaths (browser, planId) {
  await browser.goto(`${TOOL_ORIGIN}/plans/${planId}/paths`)
  await waitFor(browser, 'return document.body.innerText.includes("路径准备与运行选择")', { label: '路径页就绪', timeoutMs: 60000 })
}

// readState 读取计划与路径的只读事实（路径列表、勾选、配置与数据状态）。
export async function readState (browser, planId) {
  return await evalPage(browser, `
    const response = await fetch('/api/plans/${planId}/execution-paths', { method: 'GET' });
    const envelope = await response.json();
    const items = (envelope.data && envelope.data.items) || envelope.items || [];
    return {
      planId: '${planId}',
      paths: items.map((item) => ({
        id: item.id, sequenceNo: item.sequenceNo, name: item.name,
        configurationStatus: item.configurationStatus, dataStatus: item.dataStatus,
        configurationDetail: item.configurationDetail, dataDetail: item.dataDetail,
      })),
    };
  `)
}

// generatePaths 等待「新发起」计划进入路径页后自动解析出的路径出现。
// 解析由页面首次进入时自动触发（startAutomaticGeneration），这里只做轮询与失败判断，
// 不代页面发起写请求；路径列表为空且页面没有解析任务时才按「新增路径」补一次真实点击。
export async function generatePaths (browser, planId, { timeoutMs = 600000 } = {}) {
  const hasPaths = async () => (await readState(browser, planId)).paths.length
  if (await hasPaths() > 0) return await hasPaths()
  const deadline = Date.now() + timeoutMs
  let stable = 0
  while (Date.now() < deadline) {
    await delay(5000)
    const count = await hasPaths()
    if (count > 0 && count === stable) return count
    stable = count
    const view = await currentView(browser)
    if (view.includes('路径解析失败') || view.includes('新增路径') && !view.includes('正在后台解析')) {
      // 页面明确没有在解析且仍无路径：用页面上的「新增路径」按钮补一次真实触发
      const clicked = await evalPage(browser, 'const b = byText(".n-button", "新增路径", { root: document.body, exact: false }); if (!b) return false; fire(b); return true;')
      if (!clicked && Date.now() > deadline - timeoutMs / 2) {
        throw new Error(`路径解析未产出路径且页面上没有「新增路径」入口：${view.slice(0, 600)}`)
      }
    }
  }
  throw new Error('等待路径解析超时')
}

// keepFirstPaths 按序号保留前 N 条路径，其余在页面上逐条删除，使参与测试的路径数不超过上限。
// 删除走页面上的「删除路径」确认弹窗，与人工操作完全一致。
export async function keepFirstPaths (browser, planId, keep) {
  const state = await readState(browser, planId)
  const sorted = [...state.paths].sort((a, b) => a.sequenceNo - b.sequenceNo)
  const extra = sorted.slice(keep)
  if (!extra.length) return { removed: 0, kept: sorted.length }
  for (const path of extra) {
    await openPlanPaths(browser, planId)
    await clickPathConfigButton(browser, path.sequenceNo)
    await waitFor(browser, 'return location.pathname.includes("/configuration")', { label: '进入配置页', timeoutMs: 30000 })
    await clickByText(browser, '删除路径', { scope: 'document.body', tag: '.n-button', exact: false })
    await waitFor(browser, 'return document.body.innerText.includes("只删除当前工具中的路径记录")', { label: '删除确认弹窗', timeoutMs: 15000 })
    await clickByText(browser, '确认删除', { scope: '.n-modal', tag: '.n-button' })
    await waitFor(browser, 'return location.pathname.includes("/paths")', { label: '返回路径页', timeoutMs: 30000 })
    await delay(600)
  }
  return { removed: extra.length, kept: keep }
}

// clickPathConfigButton 在路径页按序号点击某条路径的「配置节点」。
export async function clickPathConfigButton (browser, sequenceNo) {
  const result = await evalPage(browser, `
    const rows = $$('.path-prepare__item').filter(visible);
    const row = rows.find((el) => textOf(el.querySelector('.path-prepare__sequence')) === '#${sequenceNo}');
    if (!row) return { ok: false, sequences: rows.map((el) => textOf(el.querySelector('.path-prepare__sequence'))) };
    const button = byText('.n-button', '配置节点', { root: row, exact: false });
    if (!button) return { ok: false, reason: '行内没有配置节点按钮' };
    fire(button);
    return { ok: true };
  `)
  if (!result?.ok) throw new Error(`点不到第 ${sequenceNo} 条路径的「配置节点」：${JSON.stringify(result)}`)
  await delay(300)
}

// configureAllPaths 在路径页做「一键配置」：勾选全部路径、选择动作模式、选历史实例数据、开始配置。
// 全部步骤都是页面真实点击；配置是后台任务，这里轮询页面上的进度文案直到结束。
export async function configureAllPaths (browser, planId, { actionMode = 'random', historyKeyword = '', timeoutMs = 900000 } = {}) {
  await openPlanPaths(browser, planId)
  await clickByText(browser, '全选', { scope: 'document.body', tag: '.n-button', exact: true })
  await delay(400)
  const modeLabel = actionMode === 'approve_only' ? '每一步都同意（快速跑通）' : '每节点随机一个动作'
  await clickByText(browser, modeLabel, { scope: 'document.body', tag: '.n-radio-button' })
  await delay(300)
  await clickByText(browser, '一键配置', { scope: 'document.body', tag: '.n-button', exact: true })
  await waitFor(browser, 'return textOf(document.body).includes("选择基础表单数据")', { label: '基础表单数据弹窗', timeoutMs: 30000 })
  const chosen = await chooseHistoryCandidate(browser, historyKeyword)
  // 「开始配置」按钮在弹窗卡片底部，作用域限定到基础表单数据弹窗这个具体卡片
  await clickByText(browser, '开始配置', { scope: '$(".base-form-data-picker")', tag: '.n-button' })
  // 配置任务开始后，页面会出现进度区；等到进度区出现且不再显示"正在处理"为止
  await waitFor(browser, 'return textOf(document.body).includes("已处理")', { label: '一键配置进度出现', timeoutMs: 60000 })
  const deadline = Date.now() + timeoutMs
  let lastText = ''
  while (Date.now() < deadline) {
    await delay(5000)
    lastText = await evalPage(browser, `
      const job = $$('.path-prepare__job').find(visible);
      return job ? textOf(job) : '';
    `)
    if (!lastText) break
    if (!lastText.includes('正在处理') && !lastText.includes('等待开始处理')) break
  }
  return { historyCandidate: chosen, progress: lastText }
}

// chooseHistoryCandidate 在「选择基础表单数据」弹窗里按关键词挑选一条可读取的历史实例。
// 关键词为空时取第一条可用的候选，保证「历史数据注入」这一步一定真实发生。
async function chooseHistoryCandidate (browser, keyword) {
  if (keyword) {
    await fillByPlaceholder(browser, '搜索单据名称', keyword, { scope: '.n-modal' })
    await delay(1500)
  }
  await waitFor(browser, 'return $$(".base-form-data-picker__candidate").some((el) => visible(el) && !el.disabled)', { label: '出现可选历史数据', timeoutMs: 60000 })
  const picked = await evalPage(browser, `
    const rows = $$('.base-form-data-picker__candidate').filter((el) => visible(el) && !el.disabled);
    const row = rows[0];
    fire(row);
    return { count: rows.length, title: textOf(row).slice(0, 200) };
  `)
  await delay(400)
  return picked
}

// openRunDialog 在路径页点击页头右上角的「运行」按钮，并等运行前检查完成。
// 定位用 data-testid（页面为该按钮显式提供的稳定锚点），避免误点到路径行里的「运行」复选框。
export async function openRunDialog (browser, planId) {
  await openPlanPaths(browser, planId)
  const clicked = await evalPage(browser, `
    const button = $('[data-testid="plan-detail-run-button"]');
    if (!button) return false;
    fire(button);
    return true;
  `)
  if (!clicked) throw new Error('页面上找不到「运行」按钮（data-testid=plan-detail-run-button）')
  // 等“检查中”结束：以页面显式提供的 data-testid 摘要块为准，不用正文文案猜测。
  // 检查完成后才会出现 run-preflight-summary；仍在核对时只有 checking 提示。
  await waitFor(browser, 'return !!$(\'[data-testid="run-preflight-summary"]\')', { label: '运行前检查完成', timeoutMs: 180000 })
}

// selectRunnablePaths 只勾选在运行前检查里判定为可运行的路径。
// 判定来源是检查接口自身的结论，不会为了凑数勾选被阻塞的路径。
export async function selectRunnablePaths (browser, planId) {
  const readiness = await evalPage(browser, `
    const response = await fetch('/api/plans/${planId}/run-readiness', { method: 'GET' });
    const envelope = await response.json();
    const data = envelope.data || {};
    const paths = data.paths || [];
    return {
      runnable: paths.filter((p) => !(p.blocks || []).length).map((p) => p.pathId),
      blocked: paths.filter((p) => (p.blocks || []).length).map((p) => ({ pathId: p.pathId, pathName: p.pathName, reason: (p.blocks[0] || {}).reason })),
    };
  `)
  await openPlanPaths(browser, planId)
  // 勾选状态在虚拟列表里，必须先滚到目标行让它真实渲染出来，再点复选框；
  // 只点一次是因为 rerender 会让元素句柄失效，每次操作都重新定位。
  const state = await readState(browser, planId)
  const sequenceToID = new Map(state.paths.map((p) => [String(p.sequenceNo), String(p.id)]))
  const wantedSequences = [...sequenceToID.entries()]
    .filter(([, id]) => readiness.runnable.map(String).includes(id))
    .map(([sequence]) => sequence)
  await setAllRunPathSelections(browser, false)
  const missing = []
  for (const sequence of wantedSequences) {
    const ok = await toggleRunPathSelection(browser, sequence, true)
    if (!ok) missing.push(sequence)
  }
  const checked = await selectedRunPathSequences(browser)
  const expected = [...wantedSequences].sort((a, b) => Number(a) - Number(b))
  return {
    runnableCount: wantedSequences.length,
    checkedCount: checked.length,
    checkedSequences: checked.sort((a, b) => Number(a) - Number(b)),
    missingSequences: missing,
    selectionComplete: checked.length === expected.length && expected.every((s, i) => s === checked[i]),
    blocked: readiness.blocked,
  }
}

// PATH_LIST_SELECTOR 指向路径勾选列表（n-virtual-list 的根元素）。
const PATH_LIST_SELECTOR = '.path-prepare__list'

// PATH_LIST_SCROLLER 取虚拟列表真正可滚动的那个容器。
// n-virtual-list 的根元素自己 overflow 是 visible（实测 scrollHeight === clientHeight），
// 滚动发生在它内部的 .n-virtual-list__container 上；直接设根元素的 scrollTop 不会生效，
// 这也是"按序号滚到目标行"一直失败的原因。
export const PATH_LIST_SCROLLER = `${PATH_LIST_SELECTOR} .n-virtual-list__container`

// setAllRunPathSelections 用页面上的「全选／取消全选」按钮把勾选态归零或全开。
export async function setAllRunPathSelections (browser, all) {
  const label = all ? '全选' : '取消全选'
  const result = await evalPage(browser, `
    const buttons = $$('.path-prepare__header-actions .n-button').filter(visible);
    const button = buttons.find((el) => textOf(el) === ${JSON.stringify(label)});
    if (!button) return { ok: false, candidates: buttons.map(textOf) };
    fire(button);
    return { ok: true };
  `)
  if (!result?.ok) throw new Error(`点不到「${label}」：${JSON.stringify(result?.candidates)}`)
  await delay(500)
}

// toggleRunPathSelection 把某序号的路径滚进可视区并点击它的「运行」复选框，返回是否确实点到。
// naive-ui 的勾选类名是 n-checkbox--checked，未勾选时点击 .n-checkbox 本体即可切换。
// 虚拟列表只渲染可视范围内若干行，必须先把目标行滚进渲染集；这里不硬编码行高
// （样式里是 84px，且会随主题变化），而是按"当前实际渲染出的相邻行间距"推算目标位置，
// 再分步下滚、每步重新查找，直到目标行渲染出来为止，找不到才如实报错。
export async function toggleRunPathSelection (browser, sequence, wanted) {
  const result = await evalPage(browser, `
    const root = $(${JSON.stringify(PATH_LIST_SELECTOR)});
    if (!root) return { ok: false, reason: '找不到路径列表容器' };
    // 虚拟列表把滚动放在内部 container 上；取不到时退回根元素，并用实测滚动范围判断是否真的能滚。
    const list = root.querySelector('.n-virtual-list__container') || root;
    const wantedSequence = '#${sequence}';
    const rendered = () => $$('.path-prepare__item').filter(visible);
    const findRow = () => rendered()
      .find((el) => textOf(el.querySelector('.path-prepare__sequence')) === wantedSequence);
    const settle = () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));

    // 用当前渲染出的前两行实测行距，避免写死设计稿数值。
    let rowHeight = 0;
    const probe = rendered();
    if (probe.length >= 2) {
      rowHeight = Math.abs(probe[1].getBoundingClientRect().top - probe[0].getBoundingClientRect().top);
    }
    if (!rowHeight) rowHeight = probe.length ? probe[0].getBoundingClientRect().height : 84;

    let row = findRow();
    if (!row) {
      list.scrollTop = Math.max(0, (Number('${sequence}') - 1) * rowHeight - rowHeight);
      await settle();
      row = findRow();
    }
    const step = Math.max(rowHeight, Math.floor(list.clientHeight * 0.8));
    for (let offset = 0; !row && offset <= list.scrollHeight + step; offset += step) {
      list.scrollTop = Math.min(list.scrollTop + step, list.scrollHeight);
      await settle();
      row = findRow();
    }
    if (!row) {
      return {
        ok: false,
        reason: '序号行未渲染',
        scrollerClass: list.className,
        scrollTop: list.scrollTop,
        scrollHeight: list.scrollHeight,
        clientHeight: list.clientHeight,
        rendered: rendered().map((el) => textOf(el.querySelector('.path-prepare__sequence'))),
      };
    }
    const checked = !!row.querySelector('.n-checkbox--checked');
    if (checked === ${wanted}) return { ok: true, already: true };
    const box = row.querySelector('.n-checkbox');
    if (!box) return { ok: false, reason: '行内没有复选框' };
    // 行已在渲染集里但可能仍被容器裁掉一半，点之前先确保它在视口内。
    row.scrollIntoView({ block: 'nearest' });
    await settle();
    fire(row.querySelector('.n-checkbox'));
    return { ok: true, already: false };
  `)
  if (!result?.ok) throw new Error(`勾选第 ${sequence} 条路径失败：${JSON.stringify(result)}`)
  await delay(200)
  return true
}

// selectedRunPathSequences 读取页面上当前已勾选的路径序号，用来做勾选完整性断言。
export async function selectedRunPathSequences (browser) {
  return await evalPage(browser, `
    return $$('.path-prepare__item').filter(visible)
      .filter((row) => row.querySelector('.n-checkbox--checked'))
      .map((row) => textOf(row.querySelector('.path-prepare__sequence')).replace('#', ''));
  `)
}

// clickPreflightCard 在运行弹窗里按卡片标题点选运行方式/路径执行方式（卡片本身就是按钮）。
async function clickPreflightCard (browser, title) {
  const result = await evalPage(browser, `
    const cards = $$('.run-preflight__card').filter(visible);
    const card = cards.find((el) => textOf(el.querySelector('.run-preflight__card-title')) === ${JSON.stringify(title)});
    if (!card) return { ok: false, titles: cards.map((el) => textOf(el.querySelector('.run-preflight__card-title'))) };
    fire(card);
    return { ok: true };
  `)
  if (!result?.ok) throw new Error(`运行弹窗里点不到「${title}」，现有卡片：${JSON.stringify(result?.titles)}`)
  await delay(250)
}

// startRun 在运行弹窗里选择运行方式与路径执行方式并真实点击「开始运行」。
export async function startRun (browser, { mode = 'auto', dispatch = 'serial', concurrency = 3 } = {}) {
  await clickPreflightCard(browser, mode === 'auto' ? '自动运行' : '手动控制')
  await clickPreflightCard(browser, dispatch === 'parallel' ? '并行' : '串行')
  if (dispatch === 'parallel') {
    await evalPage(browser, `
      const input = $$('.run-preflight__concurrency-input').find(visible);
      if (!input) return false;
      setInput(input, '${concurrency}');
      return true;
    `)
    await delay(250)
  }
  const started = await evalPage(browser, `
    const buttons = $$('.n-button').filter(visible);
    const button = buttons.find((el) => textOf(el) === '开始运行');
    if (!button) return { ok: false, texts: buttons.map(textOf).slice(-10) };
    fire(button);
    return { ok: true };
  `)
  if (!started?.ok) throw new Error(`点不到「开始运行」按钮：${JSON.stringify(started)}`)
  await waitFor(browser, 'return /\\/runs\\/\\d+/.test(location.pathname) || textOf(document.body).includes("运行中")', { label: '运行已启动', timeoutMs: 90000 })
  return await browser.eval('(location.pathname.match(/\\/runs\\/(\\d+)/) || [])[1] || ""')
}
