# 直接 URL 访问方式

流程应用支持通过 URL 参数直接访问，适用于 iframe 嵌入或浏览器直接打开。

## 基本格式

```
http://{host}/flow/#/flow/detail/{flowInstanceId}?sid={SID}&platformCode=200001&customerCode={customerCode}&mode={mode}
```

## 认证参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `sid` | 用户会话ID | `6d5acb0e-1bfc-44b5-b31e-347001fa5e61` |
| `platformCode` | 平台编码（固定值） | `200001` |
| `customerCode` | 客户编码（区分所属集团） | `d4036cc6581141a8b13cf39387c8d6f2` |

**参数读取优先级**：URL search params → hash params → localStorage

**customerCode 映射**：
| customerCode | 所属公司 |
|--------------|----------|
| `174d5b3f85834d4c964278299f5311ad` | 广东润世华控股集团 |
| `d4036cc6581141a8b13cf39387c8d6f2` | 润世华新能源控股集团 |

## 页面路由

| 路由 | 说明 | 额外参数 |
|------|------|----------|
| `/flow/approve` | 流程审批（5个Tab） | `tab=pending\|done\|waiting_send\|sent\|timeout_skip` |
| `/flow/detail/:id` | 流程详情/审批/查看 | `mode=audit\|view\|edit\|flow` |
| `/flow/initiate` | 发起流程 | `flowTemplateId`, `companyId`, `title` |
| `/flow/cost` | 流程统计 | - |

## mode 参数说明

| mode | 说明 | 使用场景 |
|------|------|----------|
| `audit` | 审核模式 | 待办列表点击"审核" |
| `view` | 查看模式 | 已办/超时跳转点击"查看"，已发点击"详情" |
| `edit` | 编辑模式 | 待发列表点击"编辑"/"重新发起" |
| `flow` | 流程图模式 | 任意列表点击"查看流程" |

## 示例

### 审核待办流程（iframe 嵌入）
```
http://192.168.1.212:9528/flow/#/flow/detail/FLOW_ID?sid=abc123&platformCode=200001&customerCode=d4036cc6581141a8b13cf39387c8d6f2&mode=audit&auditWay=or
```

### 查看流程详情
```
http://192.168.1.212:9528/flow/#/flow/detail/FLOW_ID?sid=abc123&platformCode=200001&customerCode=d4036cc6581141a8b13cf39387c8d6f2&mode=view
```

### 查看流程图
```
http://192.168.1.212:9528/flow/#/flow/detail/FLOW_ID?sid=abc123&platformCode=200001&customerCode=d4036cc6581141a8b13cf39387c8d6f2&mode=flow
```

## 环境地址

| 环境 | 流程应用地址 | API 地址 |
|------|-------------|----------|
| 开发 | `http://192.168.1.212:9528/flow/` | `http://192.168.1.220:38081/api` |
| 测试 | `http://192.168.1.220:8081/flow/` | `http://192.168.1.220:8081/api` |
| 生产 | `https://iserver.runshihua.com/flow/` | `https://iserver.runshihua.com/api` |
