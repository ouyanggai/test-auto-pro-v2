// F-042 本轮：报告中心名词解释术语表（唯一来源）。
// 用户不熟悉测试术语，所以每个指标/表头旁的 info 图标与右上角「指标口径说明」总入口
// 都从这里取解释，页面里不再各写一份文案，避免同一名词两种说法。
// 纪律：解释全部中文白话，不出现内部英文键、接口路径或数据库表名之外的内部标识；
// 只允许新增条目或修订文案，不允许为了让页面好看而删条目。

// ReportTerm 是一条名词解释：DTO 结构固定，保证图标弹层与总入口渲染一致。
export interface ReportTerm {
  /** id 是稳定标识，供 TermHint 与总入口按同一键取同一条解释。 */
  id: string
  /** 名称是页面上的展示名，与卡片/表头标签一致。 */
  name: string
  /** 一句话解释用大白话说明这个名词是什么。 */
  summary: string
  /** 怎么算写明分子/分母或统计方式；无比率含义的条目写统计方式。 */
  formula: string
  /** 数据来源说明这个数字从哪里来，让用户判断可不可信。 */
  source: string
  /** 容易误解的点写清最常见的读错方式。 */
  caveat: string
}

// reportTerms 是报告中心全部名词解释，按「运行事实 → 比率 → 耗时 → 统计 → 接口与重试 → 结果 → 数据来源」分组。
export const reportTerms: ReportTerm[] = [
  {
    id: 'run',
    name: '运行',
    summary: '一次启动测试计划后形成的执行记录，包含这次跑了的全部执行路径。',
    formula: '点击「运行」一次就生成一条运行记录，运行号按计划累加（显示为 #N）。',
    source: '本工具运行记录表。',
    caveat: '一次运行里可以同时跑多条路径；单条路径失败不会自动结束其他路径。',
  },
  {
    id: 'path-run',
    name: '路径运行',
    summary: '一次运行里某一条执行路径的实际执行过程，也是报告里所有统计的最小单位。',
    formula: '一条运行记录下的每条已选路径各对应一条路径运行。',
    source: '本工具运行记录表。',
    caveat: '同一条执行路径跑十次就是十条路径运行；看成功率时分子分母都按它数。',
  },
  {
    id: 'step',
    name: '步骤',
    summary: '一条路径运行按顺序执行的一个动作，比如提交、同意、加签。',
    formula: '一次路径运行的步骤数 = 这次实际执行过的不同步骤个数；一个步骤失败后重试仍算同一步骤。',
    source: '本工具运行记录表。',
    caveat: '步骤不含系统自动插入的导航步骤；「平均步骤数」是按次平均，不是按步平均。',
  },
  {
    id: 'instance-ref',
    name: '目标实例号',
    summary: '这次运行在目标平台上创建或操作的流程实例编号，用于回目标平台核对。',
    formula: '只读引用，按目标平台返回的原值展示，工具不改写。',
    source: '目标平台返回的实例标识，随运行记录落库。',
    caveat: '它只是编号，不代表结果；查明细请点「查看明细」进运行详情。',
  },
  {
    id: 'generated-at',
    name: '数据截至时间',
    summary: '这一屏统计是哪个时刻算出来的，用来判断数字有多新。',
    formula: '服务端每次读取报告时的时间戳，三个页面各自显示自己那一刻。',
    source: '服务端统计时刻。',
    caveat: '正在跑的计划，数字会比「数据截至时间」旧一点；点「刷新」才会重新统计。',
  },
  {
    id: 'success-rate',
    name: '路径运行成功率',
    summary: '跑完的路径运行里，有多少比例是成功的。',
    formula: '分子：结果为「成功」的路径运行数；分母：已结束的路径运行数（成功/失败/已停止/已取消/结果待确认）。',
    source: '本工具运行记录表的结果与状态字段。',
    caveat: '还在跑和排队中的不计入分母；「已取消」算已结束但不算成功，会拉低成功率。',
  },
  {
    id: 'first-pass-rate',
    name: '一次通过率（FPY）',
    summary: '不需要任何重试、一次就成功的路径运行占比，衡量「顺不顺」。',
    formula: '分子：首次尝试就成功、且最终也成功、期间没有发生重试的路径运行数；分母：已结束的路径运行数。',
    source: '本工具运行记录表的尝试序号与重放标记。',
    caveat: '它永远不高于成功率；两者差距大说明经常要靠重试才过得去。',
  },
  {
    id: 'path-coverage',
    name: '路径覆盖度',
    summary: '计划里保存的路径，有多少比例至少被跑过一次。',
    formula: '分子：至少有一次运行记录的路径数；分母：该计划当前保存的路径总数。',
    source: '本工具路径表与运行记录表。',
    caveat: '覆盖度低说明很多分支还没验证过，不代表已验证的路径没问题。',
  },
  {
    id: 'step-duration',
    name: '步骤耗时',
    summary: '执行一个步骤从开始到结束花了多久，包含工具自己的准备、等待和收尾。',
    formula: '按步骤尝试逐个记录，取墙钟毫秒数；分位卡展示 P50/P90/P95 与均值。',
    source: '本工具运行记录表的步骤尝试耗时字段。',
    caveat: '它不等于「目标平台慢」：里面含工具侧编排和目标等待，看目标快慢请用「目标接口耗时」。',
  },
  {
    id: 'interface-duration',
    name: '目标接口耗时',
    summary: '每一次调用目标平台接口，从发出请求到收到响应（或传输失败）花了多久。',
    formula: '逐条请求按传输层真实计时记录，毫秒；写请求单独再给一份合计。',
    source: '运行目录里的 network.log（传输层计时）。',
    caveat: '它只算「路上的时间」，不含排队、准备、核验；历史日志被清理的运行显示「暂无真实请求耗时」，不会拿估算值充数。',
  },
  {
    id: 'executor-duration',
    name: '执行器耗时',
    summary: '独立执行服务（真实浏览器）处理这个动作花了多久的墙钟时间。',
    formula: '执行服务从收到装载指令到动作收敛的真实计时，毫秒，只在真实执行过的动作上有值。',
    source: '独立执行服务上报、随动作结果落库。',
    caveat: '非浏览器执行的动作显示「无执行器环节」；这部分时间天然包含它对目标的等待，别和接口耗时的合计相减做判断。',
  },
  {
    id: 'orchestration-duration',
    name: '工具端编排耗时',
    summary: '除掉目标接口等待和执行器处理之后，工具自己花掉的时间。',
    formula: '派生值 = 步骤总耗时 − 目标接口传输耗时 − 执行器耗时；任一分量不可靠或相减为负时显示「未知」。',
    source: '由前三个真实分量派生，页面已标注口径。',
    caveat: '它是派生值不是直接测量值；显示「未知」时说明分量不全，不代表 0。',
  },
  {
    id: 'queue-wait',
    name: '账号排队等待',
    summary: '同一账号的目标操作必须排队，等待别人先用完的这段时间。',
    formula: '单独统计，不并入目标接口耗时，也不并入工具端编排耗时。',
    source: '本工具运行记录与执行阶段事实。',
    caveat: '别把排队时间算成「目标平台响应慢」。',
  },
  {
    id: 'percentile',
    name: '分位（P50/P90/P95）',
    summary: '把一批耗时从小到大排，P50 是一半请求比它快，P95 是 95% 的请求比它快。',
    formula: 'P50=中位数水平，P90/P95 看长尾有多慢；均值容易被个别极慢请求拉高。',
    source: '在真实耗时样本上用统计库计算。',
    caveat: '看长尾优先看 P95；样本太少时分位没有代表性，页面会显示样本数。',
  },
  {
    id: 'mean',
    name: '均值',
    summary: '一批耗时的平均值。',
    formula: '全部样本求和后除以样本个数。',
    source: '在真实耗时样本上计算。',
    caveat: '少数极慢请求会把均值抬高，和 P50 一起看更准。',
  },
  {
    id: 'sample-count',
    name: '样本数',
    summary: '这个统计实际基于多少条真实记录。',
    formula: '有多少条记录参与计算就显示多少。',
    source: '统计范围内的真实记录条数。',
    caveat: '样本数为 0 时显示「暂无足够样本」，不显示 0 毫秒。',
  },
  {
    id: 'sample-cap',
    name: '采样上限',
    summary: '为了不让一次读取太慢，接口耗时最多只统计最近的一部分运行。',
    formula: '超过上限时只统计最近的样本，页面会提示「仅统计最近的运行样本」；计数类指标仍是全量。',
    source: '报告服务的采样保护。',
    caveat: '出现这条提示时，分位数只代表最近那部分，不代表全部历史。',
  },
  {
    id: 'read-request',
    name: '读请求',
    summary: '只查询信息、不改变目标平台状态的接口调用。',
    formula: '按接口是否属于写端点白名单划分，白名单之外都算读。',
    source: '运行目录 network.log 的请求分类。',
    caveat: '读请求多通常是在轮询/查结构，属于正常现象。',
  },
  {
    id: 'write-request',
    name: '写请求',
    summary: '会真正改变目标平台状态的接口调用，比如提交、同意、加签。',
    formula: '命中写端点白名单的请求记为写请求，单独统计次数与耗时合计。',
    source: '运行目录 network.log 的请求分类。',
    caveat: '写请求次数直接对应真实副作用次数，是核对目标平台的重要依据。',
  },
  {
    id: 'interface-calls',
    name: '目标接口调用',
    summary: '这次运行一共向目标平台发出了多少次接口调用。',
    formula: '读请求次数 + 写请求次数，按这次运行的全部请求行统计。',
    source: '运行目录 network.log 的真实请求记录。',
    caveat: '它是「实际发生的事实」，不包含没发出去的请求（未发出的请求在运行详情里显示「未发出」）。',
  },
  {
    id: 'retry',
    name: '重试',
    summary: '同一步骤因为失败或中断又试了一次。',
    formula: '尝试序号大于 1 或标记为重放的，按次计入重试次数；涉及多少条路径运行单独计数。',
    source: '本工具运行记录表的尝试记录。',
    caveat: '写请求只有在确认没发出去时才允许重试；「结果待确认」的运行不会自动重试。',
  },
  {
    id: 'result-succeeded',
    name: '成功',
    summary: '这次路径运行按配置跑完了。',
    formula: '路径运行结果为成功，或已结束且没有记录失败结果。',
    source: '本工具运行记录表的结果字段。',
    caveat: '成功只代表这次执行跑通，不代表目标业务被审批通过。',
  },
  {
    id: 'result-failed',
    name: '失败',
    summary: '这次路径运行因为确定的原因没能完成。',
    formula: '结果为失败；报告同时给出失败分类（如门禁阻塞）。',
    source: '本工具运行记录表的结果与失败分类字段。',
    caveat: '失败的具体原因请进运行详情看步骤与错误说明。',
  },
  {
    id: 'result-stopped',
    name: '已停止',
    summary: '被用户主动停掉的运行。',
    formula: '路径运行状态为已停止。',
    source: '本工具运行记录表的状态字段。',
    caveat: '停止前已经发生的写操作不会回滚。',
  },
  {
    id: 'result-cancelled',
    name: '已取消',
    summary: '还没真正开始执行就被取消的运行。',
    formula: '路径运行状态为已取消。',
    source: '本工具运行记录表的状态字段。',
    caveat: '已取消计入已结束分母，会拉低成功率。',
  },
  {
    id: 'result-awaiting',
    name: '结果待确认',
    summary: '写请求已经发生，但工具没能确认它到底生效没有，只能如实挂起。',
    formula: '路径运行状态为结果待确认；这种情况下工具不会自动重发。',
    source: '本工具运行记录表的状态字段。',
    caveat: '这是诚实结论不是故障；要继续只能从计划重新发起一次运行。',
  },
  {
    id: 'failure-class',
    name: '失败分类 / 阻塞',
    summary: '把失败归成几类，方便看是哪类问题反复出现。',
    formula: '按失败分类统计条数；「阻塞」指前置条件不满足（如分支没选、没设审批人），工具没有发起写请求。',
    source: '本工具运行记录表的失败分类字段。',
    caveat: '阻塞不是目标平台故障，通常改配置就能解决；别和「结果待确认」混为一谈。',
  },
  {
    id: 'run-records',
    name: '运行记录表',
    summary: '本工具自己保存的运行事实，报告的全部数字都从这里来。',
    formula: '包含运行、路径运行、步骤、尝试等表的真实记录。',
    source: '本工具数据库（只读，报告不改写任何运行事实）。',
    caveat: '报告只读不写；删除运行记录会同步影响这里的历史统计。',
  },
  {
    id: 'network-log',
    name: 'network.log',
    summary: '每次运行向目标平台发请求时留下的原始传输日志，一行一条请求。',
    formula: '报告按它统计接口调用次数与接口耗时；内网项目按原样全量记录，不做脱敏。',
    source: '运行目录下的 network.log 文件。',
    caveat: '日志被清理或历史运行没有它时，接口耗时显示「暂无真实请求耗时」，不会用估算值冒充。',
  },
]

// reportTermById 按 id 取一条名词解释；查不到返回 undefined，由调用方决定不渲染图标。
export function reportTermById(id: string): ReportTerm | undefined {
  return reportTerms.find((term) => term.id === id)
}

// reportTermsByIds 按 id 列表取多条名词解释，保持传入顺序；供「指标口径说明」总入口分组渲染。
export function reportTermsByIds(ids: string[]): ReportTerm[] {
  const byID = new Map(reportTerms.map((term) => [term.id, term]))
  return ids.map((id) => byID.get(id)).filter((term): term is ReportTerm => term !== undefined)
}

// reportTermGroups 是「指标口径说明」总入口的分组顺序与标题，
// 保证三个页面按同一套分组呈现，不让用户在两处看到不同归类。
export const reportTermGroups: Array<{ title: string, ids: string[] }> = [
  {
    title: '运行与结果',
    ids: ['run', 'path-run', 'step', 'instance-ref', 'generated-at', 'result-succeeded', 'result-failed', 'result-stopped', 'result-cancelled', 'result-awaiting', 'failure-class'],
  },
  {
    title: '专业指标',
    ids: ['success-rate', 'first-pass-rate', 'path-coverage'],
  },
  {
    title: '耗时',
    ids: ['step-duration', 'interface-duration', 'executor-duration', 'orchestration-duration', 'queue-wait'],
  },
  {
    title: '统计口径',
    ids: ['percentile', 'mean', 'sample-count', 'sample-cap'],
  },
  {
    title: '接口与重试',
    ids: ['read-request', 'write-request', 'interface-calls', 'retry'],
  },
  {
    title: '数据来源',
    ids: ['run-records', 'network-log'],
  },
]

// overviewTermIds / planTermIds / pathTermIds 是三个页面右上角总入口各自列出的名词：
// 每级只列本级看得到的名词，避免用户在计划详情里翻到只有首页才有的图表口径。
export const overviewTermIds: string[] = reportTermGroups.flatMap((group) => group.ids)

export const planTermIds: string[] = [
  'run', 'path-run', 'generated-at',
  'success-rate', 'first-pass-rate', 'path-coverage',
  'step-duration', 'interface-duration', 'percentile', 'mean', 'sample-count',
  'result-succeeded',
]

export const pathTermIds: string[] = [
  'path-run', 'step', 'instance-ref', 'generated-at',
  'success-rate', 'first-pass-rate',
  'step-duration', 'interface-duration', 'executor-duration', 'orchestration-duration', 'queue-wait',
  'percentile', 'mean', 'sample-count', 'sample-cap',
  'write-request', 'interface-calls', 'retry',
  'result-succeeded', 'result-failed', 'result-stopped', 'result-cancelled', 'result-awaiting', 'failure-class',
]
