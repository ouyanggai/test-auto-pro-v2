<!--
  执行宿主（发起）：挂载真实 FlowDialog 并提供执行桥接所需的非界面操作入口。
  纪律（docs/s/01-执行引擎设计.md 4.3/4.4）：动作执行只调用目标组件的真实业务方法
  （sumbitFlow / formMakingFormBusiness / 分支并行选人的继续方法），不做任何按钮或 DOM 操作；
  表单模板与字段权限在有 flowId 时由目标接口（经 Go 转发）自行装载，工具不复制目标逻辑。
-->
<template>
  <flow-dialog ref="flowDialog" :visible="visible" :append-to-body="false" />
</template>

<script>
import FlowDialog from '@runtime/views/GroupApproveManage/Submitted/components/FlowDialog.vue'

export default {
  name: 'ExecutionHostInitiate',
  components: { FlowDialog },
  data () {
    return {
      visible: false
    }
  },
  methods: {
    // applyExecutionLoad 由 App 装载链路调用：配置真实 FlowDialog 到第二步（表单填写）状态。
    // payload.flowId 存在时 OtherSteps2 会按 watch(flowId) 自行经 Go 转发拉取模板与字段权限，
    // 完成后以 fmDone 置位 fmComplete；无 flowId（离线模板注入）时由 App 走注入分支。
    async applyExecutionLoad ({ payload, template, permissions }) {
      const flowDialog = this.$refs.flowDialog
      flowDialog.formExist = String(payload.formExist ?? '')
      flowDialog.selectFlowType = String(payload.selectFlowType ?? '')
      flowDialog.stepsActive = 2
      this.visible = true
      const flowId = String(payload.flowId ?? '')
      if (flowId) {
        flowDialog.flowId = flowId
        flowDialog.formId = String(payload.formId ?? '')
        await this.waitForFormReady(60000)
        return { mode: 'selfloaded' }
      }
      // 离线注入：模板来自 Go 装载载荷（已按权限预处理），交由 App 通用注入逻辑写入 OtherSteps2。
      flowDialog.flowId = ''
      flowDialog.formId = String(payload.formId ?? '')
      await this.$nextTick()
      const steps2 = this.getOtherSteps2()
      if (steps2) {
        steps2.enableData = steps2.enableData || []
        steps2.disabledData = steps2.disabledData || []
        steps2.jsonData = template && template.list ? template : { list: [], config: {} }
      }
      return { mode: 'injected' }
    },
    // waitForFormReady 等待 OtherSteps2 真实模板装载完成（fmDone 事件置位 fmComplete）。
    waitForFormReady (timeoutMs) {
      return new Promise((resolve, reject) => {
        const startedAt = Date.now()
        const poll = () => {
          const flowDialog = this.$refs.flowDialog
          if (flowDialog && flowDialog.fmComplete === true) return resolve()
          if (Date.now() - startedAt > timeoutMs) return reject(new Error('发起表单模板装载超时'))
          setTimeout(poll, 200)
        }
        poll()
      })
    },
    getOtherSteps2 () {
      const flowDialog = this.$refs.flowDialog
      return flowDialog && flowDialog.$refs.OtherSteps2 ? flowDialog.$refs.OtherSteps2 : null
    },
    getGenerateForm () {
      const steps2 = this.getOtherSteps2()
      return steps2 && steps2.$refs.generateForm ? steps2.$refs.generateForm : null
    },
    // ===== 以下是把执行动作转成真实组件方法调用的薄适配层 =====
    // 动作语义与参数差异完全由目标方法自己保留；这里只做等价于“按了哪个入口”的调用。
    actionSaveDraft () {
      this.$refs.flowDialog.sumbitFlow('draft')
    },
    actionSubmit () {
      this.$refs.flowDialog.sumbitFlow('submit')
    },
    // putSelectionClosedLoop 装载选择闭环数据（T010B）：Go 已配置的分支入口与节点人员，
    // 原样写入目标组件在弹窗确认时会写入的字段，再由桥接调用弹窗确认按钮对应的继续方法。
    putSelectionClosedLoop ({ branchEntryNodeProxyIds = [], nextAuditors = [] } = {}) {
      const flowDialog = this.$refs.flowDialog
      for (const auditor of nextAuditors) {
        const exists = flowDialog.nextAuditorList.some(item =>
          String(item.nodeProxyId ?? '') === String(auditor.nodeProxyId ?? '') && String(item.bizId ?? '') === String(auditor.bizId ?? ''))
        if (!exists) flowDialog.nextAuditorList.push({ ...auditor })
      }
      for (const branchNodeId of branchEntryNodeProxyIds) {
        if (flowDialog.chooseBranchList.indexOf(branchNodeId) === -1) flowDialog.chooseBranchList.push(branchNodeId)
      }
    },
    // actionContinueBranchChoice 等价于手动分支弹窗的“提交”按钮：handleSaveBranchChooseNode。
    // chooseBranchNode 需要指向本次选中的分支（数据形态与 manualChooseNodes 条目一致）。
    actionContinueBranchChoice ({ branchEntryNodeProxyIds = [], nextAuditors = [] } = {}) {
      const flowDialog = this.$refs.flowDialog
      this.putSelectionClosedLoop({ branchEntryNodeProxyIds, nextAuditors })
      const entryId = branchEntryNodeProxyIds[branchEntryNodeProxyIds.length - 1] ?? ''
      if (entryId && (!flowDialog.chooseBranchNode || !flowDialog.chooseBranchNode.nextNodeTemplateId)) {
        flowDialog.chooseBranchNode = { nextNodeTemplateId: entryId, nodeType: 'empty' }
      }
      flowDialog.handleSaveBranchChooseNode()
    },
    // actionContinueParallelChoice 等价于并行自选弹窗的“提交”按钮：handleSaveParallelChooseNode。
    actionContinueParallelChoice ({ branchEntryNodeProxyIds = [], nextAuditors = [] } = {}) {
      const flowDialog = this.$refs.flowDialog
      this.putSelectionClosedLoop({ branchEntryNodeProxyIds, nextAuditors })
      flowDialog.handleSaveParallelChooseNode()
    },
    // actionContinueAuditorChoice 等价于审批人自选完成回调：getSelectPerson。
    actionContinueAuditorChoice ({ nextAuditors = [] } = {}) {
      const flowDialog = this.$refs.flowDialog
      this.putSelectionClosedLoop({ nextAuditors })
      const nodeProxyId = nextAuditors[0]?.nodeProxyId ?? ''
      flowDialog.getSelectPerson({
        checkboxPersonGroup: nextAuditors.map(item => ({
          id: item.bizId,
          name: item.name,
          nodeProxyId: item.nodeProxyId || nodeProxyId
        }))
      })
    }
  }
}
</script>
