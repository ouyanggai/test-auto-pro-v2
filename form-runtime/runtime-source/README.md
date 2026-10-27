# @rsh/flow-components

润世华流程审批组件 -- 独立 Web 应用，通过 iframe 嵌入 Vue 3 新平台，提供流程审批、查看、编辑、发起等功能。

## 技术栈

| 技术 | 版本 |
|------|------|
| Vue | 2.6.11 |
| Element UI | 2.15.8 |
| Vuex | 3.4.0 |
| Vue Router | 3.2.0 (hash 模式) |
| Axios | 0.20.0 |
| Vue CLI | 4.5.0 |

## 环境配置

| 环境 | API 地址 | 流程应用地址 | OnlyOffice |
|------|----------|-------------|------------|
| 开发 (development) | `http://192.168.1.220:38081/api` | `http://192.168.1.212:9528/flow/` | `http://192.168.1.218:8085` |
| 测试 (VUE_APP_FLAG=dev) | `http://192.168.1.218:8077/api` | - | `http://192.168.1.218:8085` |
| 预发 (VUE_APP_FLAG=test) | `http://192.168.1.220:8081/api` | `http://192.168.1.220:8081/flow/` | `http://192.168.1.220:8085` |
| 生产 | `https://iserver.runshihua.com/api` | `https://iserver.runshihua.com/flow/` | `https://ioffice.runshihua.com` |

配置文件: `src/config/env.js`。容器构建时通过以下参数注入 `VUE_APP_*`，iframe 宿主仍可通过 `RSH_FLOW_CONFIG` 消息或 URL 参数覆盖 `baseUrl`。流水线的 `BASE_URL` 会作为 API 访问地址写入前端；组件还会在浏览器发送 XHR/fetch 前，把 FormMaking 等表单定义中的旧绝对 `/api` 地址替换为该地址。

| 构建参数 | 前端变量 | 说明 |
|----------|----------|------|
| `BASE_URL` | `VUE_APP_BASE_URL` | API 根地址 |
| `VIEW_FILE_URL` | `VUE_APP_VIEW_FILE_URL` | 文件预览服务地址 |
| `ONLY_OFFICE_URL` | `VUE_APP_ONLY_OFFICE_URL` | OnlyOffice 地址 |
| `WS_URL` | `VUE_APP_WS_URL` | WebSocket 地址 |

## 快速开始

```bash
# 安装依赖
npm install

# 启动开发服务器 (端口 9528)
npm run serve

# 在 test 分支构建测试包（先从新 Git 同步平台 test）
npm run build:test

# 在 master 分支构建生产包（先同步平台 master，输出到 dist/）
npm run build:prod
```

### Docker 构建

Dockerfile 使用两个 stage：第一段构建 Vue 静态资源，第二段使用 OpenResty 提供 `/flow/`。

```bash
git switch master
npm run sync:prod
docker build \
  --build-arg FLOW_ENV_NAME=production \
  --build-arg BASE_URL=https://api.example.com/api \
  --build-arg VIEW_FILE_URL=https://reader.example.com \
  --build-arg ONLY_OFFICE_URL=https://office.example.com \
  --build-arg WS_URL=wss://api.example.com/sk/ \
  -t rsh-flow-components:latest .
```

流水线参数模板见 `Jenkinsfile.example`，变量清单见 `deploy/env/flow.env.example`。

`BASE_URL` 可填写同源路径 `/api`，也可填写完整的 `https://gateway.example.com/api`。修改流水线参数后需要重新构建镜像。

融合分页通过 `/flow/detail/:id?taskContext=...` 打开流程时，URL 会包含流程上下文。`nginx/rsh-flow-components.conf` 配置了 `client_header_buffer_size 16k` 和 `large_client_header_buffers 4 64k`，将单条 HTTP 请求行上限提高到 64KB，避免超过默认 8KB 时返回 414。修改此配置后需重新构建并发布流程组件镜像。若外层 Ingress 仍返回 414，还需将其请求头缓冲限制同步到不低于此值。

## 目录结构

```
src/
  adapters/          # postMessage 通信适配器
  api/               # 接口请求
  assets/            # 静态资源、图标、样式
  components/        # 公共组件 (FormMaking 自定义组件、InitFlowDialog 等)
  config/            # 环境配置 (env.js)
  cross-modules/     # 跨模块依赖文件 (BudgetManage, PerformanceManage 等)
  directives/        # 自定义指令
  filters/           # 全局过滤器
  layout/            # 布局组件
  lib/               # 第三方库 (vue-form-making)
  router/            # 路由配置
  store/             # Vuex 状态管理
  utils/             # 工具函数 (axios 封装、加密等)
  views/             # 页面组件
    FlowDetail.vue     # 流程详情 (核心页面)
    FlowInitiate.vue   # 发起流程
    GroupApproveManage/ # 流程审批管理
examples/            # 集成文档
scripts/             # 同步和发布脚本
```

## iframe 集成方式

### URL 格式

```
http://{host}/flow/#/flow/detail/{flowInstanceId}?sid={SID}&platformCode=200001&customerCode={customerCode}&mode={mode}
```

### 认证参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `sid` | 用户会话 ID | `6d5acb0e-1bfc-44b5-b31e-347001fa5e61` |
| `platformCode` | 平台编码，固定值 | `200001` |
| `customerCode` | 客户编码，区分所属集团 | 见下方映射表 |

参数读取优先级: URL search params > hash params > localStorage

### 页面路由

| 路由 | 说明 | 额外参数 |
|------|------|----------|
| `/flow/approve` | 流程审批 (5 个 Tab) | `tab=pending\|done\|waiting_send\|sent\|timeout_skip` |
| `/flow/approve/:tab` | 流程审批指定 Tab | - |
| `/flow/detail/:id` | 流程详情 | `mode=audit\|view\|edit\|flow` |
| `/flow/initiate` | 发起流程 | `flowTemplateId`, `companyId`, `title` |
| `/flow/cost` | 流程统计 | - |

### mode 参数说明

| mode | 说明 | 使用场景 |
|------|------|----------|
| `audit` | 审核模式 | 待办列表点击"审核" |
| `view` | 查看模式（默认） | 已办/超时跳转点击"查看" |
| `edit` | 编辑模式 | 待发列表点击"编辑"/"重新发起" |
| `flow` | 流程图模式 | 点击"查看流程" |

## postMessage 协议

流程应用与宿主应用 (Vue 3) 通过 `window.postMessage` 进行双向通信。

**当前主要使用方式**: 宿主通过 URL 参数传递认证信息，postMessage 主要用于操作完成后的事件回调。

### Parent -> iframe

| type | 说明 | 用途 |
|------|------|------|
| `RSH_FLOW_AUTH` | 注入认证信息 (sid, userData) | 可选，URL 参数已包含时无需发送 |
| `RSH_FLOW_CONFIG` | 注入运行时配置 (baseUrl 等) | 可选，覆盖 env.js 默认值 |
| `RSH_FLOW_NAVIGATE` | 控制 iframe 内路由导航 | 跳转到指定页面 |

### iframe -> Parent

| type | 说明 | 触发时机 |
|------|------|----------|
| `RSH_FLOW_READY` | 应用加载完成 | iframe 初始化完毕 |
| `RSH_FLOW_EVENT` | 流程操作事件 | 审批/编辑完成后 callback |
| `RSH_FLOW_AUTH_EXPIRED` | 认证过期 | 收到 401 响应 |

`RSH_FLOW_EVENT` 的 `eventName` 值: `flow-action-done`, `flow-submitted`, `flow-approved`

详细协议文档见 `examples/postmessage-api.md`。

## 运行时配置

在 iframe 加载前，宿主应用可通过 `window.__RSH_FLOW_CONFIG__` 覆盖默认环境配置:

```javascript
window.__RSH_FLOW_CONFIG__ = {
  baseUrl: 'https://custom-api.example.com/api',
  viewFileUrl: 'https://custom-api.example.com',
  onlyOfficeUrl: 'https://custom-office.example.com'
};
```

也可以在运行时通过 postMessage 发送 `RSH_FLOW_CONFIG` 消息动态修改。

## 跨模块依赖

流程表单中部分组件引用了源项目其他模块的代码。通过 webpack alias 实现零代码修改的重定向:

```javascript
// vue.config.js
'@/views/BudgetManage'        -> 'src/cross-modules/BudgetManage'
'@/views/PerformanceManage'   -> 'src/cross-modules/PerformanceManage'
'@/views/BacklogManage'       -> 'src/cross-modules/BacklogManage'
'@/views/QuestionnaireManage' -> 'src/cross-modules/QuestionnaireManage'
'@/views/TaskManage'          -> 'src/cross-modules/TaskManage'
'@/views/flowLibrary'         -> 'src/cross-modules/flowLibrary'
```

部分深层依赖直接放在 `src/views/` 下（与源项目路径一致），通过 `@` alias 自然解析：
- `src/views/PerformanceManage/QuarterPerfAssess/` -- 季度绩效考核（fmCustomPage 依赖）
- `src/views/TaskManage/TaskArrange/components/` -- 任务安排弹窗
- `src/views/BacklogManage/components/NoFormMulBranch/` -- 无表单多分支流程
- `src/views/flowLibrary/components/` -- 流程库组件（TransferPerson 等）
- `src/views/QuestionnaireManage/components/fill.vue` -- 问卷填写

添加新的跨模块依赖时，需要:
1. 在 `vue.config.js` 的 `resolve.alias` 中添加对应映射
2. 将源文件同步到 `src/cross-modules/` 对应目录

## 发布流程

本项目发布到内部 Verdaccio 私有 npm 仓库 (`http://192.168.1.248:4873/`)。

```bash
# 1. 从对应环境分支同步最新代码
# 测试环境：rsh-flow-components 必须在 test 分支，源项目使用 test 分支
npm run sync:test

# 正式环境：rsh-flow-components 必须在 master 分支，源项目使用 master 分支
npm run sync:prod

# 2. 构建测试包或生产包
npm run build:test
npm run build:prod

# 3. 发布到 Verdaccio
npm publish --registry http://192.168.1.248:4873/

# 或使用一体化脚本
npm run publish:flow        # 同步 + 构建 + 发布 (patch 版本)
npm run publish:flow:minor  # minor 版本升级
npm run publish:flow:dry    # dry-run 模式，仅预览不实际发布
```

验证发布:
```bash
npm view @rsh/flow-components --registry http://192.168.1.248:4873/
```

构建通过 Git 自动同步，无需在本机另放一份平台源码：`master` 的 `npm run build` / `build:prod` 拉取平台 `master`，`test` 的 `npm run build:test` 拉取平台 `test`。源仓库地址由本项目新 Git `origin` 推导，仓库名为 `rsh-cloud-invest-power-system`。

每次构建冻结源提交，并将分支、SHA、FormMaking 目录和文件哈希写入 `sync-source-lock.json`。拉取失败、分支不匹配、同步内容被本地修改时停止构建；不会自动提交或推送代码。PaaS 在镜像构建前通过已有 Git 凭据同步，Docker 校验准备好的快照。

同步范围见 `sync-manifest.json`，完整操作和验收方法见 [Git 同步说明](docs/git-sync.md)。

## customerCode 映射表

| customerCode | 所属公司 |
|--------------|----------|
| `174d5b3f85834d4c964278299f5311ad` | 广东润世华控股集团 |
| `d4036cc6581141a8b13cf39387c8d6f2` | 润世华新能源控股集团 |
