<!--
  执行宿主（审批）：复刻目标待办审批页的组件契约——FormMaking 表单在宿主层（$refs.generateForm、
  editData 数据字段在 $parent.$parent 层），EnterpriseExamineOpinion 挂在内层包装下。
  审批动作（同意/不同意/暂存/重新提交）全部调用 opinion 的真实业务方法；
  意见写入 approveForm.approveMessage 后由目标链路自己构造协议载荷。
-->
<template>
  <div class="exec-host-approve">
    <fm-generate-form ref="generateForm" :data="jsonData" :value="editData" />
    <audit-panel-wrapper>
      <enterprise-examine-opinion
        ref="opinion"
        :visible="true"
        @post-examine="businessDone = true"
        :is-re-initiate="isReInitiate"
        :initiator-id="initiatorId"
        :job-task-id="jobTaskId"
        :flow-node-type="flowNodeType"
        :next-node-proxy-id="nextNodeProxyId"
        :flow-instance-id="flowInstanceId"
        :flow-name="flowName"
        :flow-proxy-id="flowProxyId"
        :select-flow-type="selectFlowType"
        :flow-node-proxy-id="flowNodeProxyId"
        :flow-instance-biz-relevance-list="flowInstanceBizRelevanceList"
      />
    </audit-panel-wrapper>
  </div>
</template>

<script>
import EnterpriseExamineOpinion from '@runtime/views/GroupApproveManage/components/EnterpriseExamineOpinion.vue'
import flowTypeMixin from '@runtime/views/GroupApproveManage/components/flowTypeMixin'

// auditPanelWrapper 是内层包装：使 opinion 的 $parent 指向它、$parent.$parent 指向宿主，
// 从而命中目标源码里 `this.$parent.$parent.editData` 与 `$parent.$parent.$refs.generateForm` 的既有契约。
const auditPanelWrapper = {
  name: 'ExecApprovePanelWrapper',
  render (h) { return h('div', this.$slots.default) }
}

export default {
  name: 'ExecutionHostApprove',
  components: {
    'enterprise-examine-opinion': EnterpriseExamineOpinion,
    'audit-panel-wrapper': auditPanelWrapper
  },
  // 转发与审批同属待办页链路：混入目标 flowTypeMixin 复用真实 confirmTranspond
  // （dynamicParam 组装、transpond 主请求与附言保存），不复制目标业务代码。
  mixins: [flowTypeMixin],
  data () {
    return {
      jsonData: { list: [], config: {} },
      editData: {},
      isReInitiate: false,
      initiatorId: '',
      jobTaskId: '',
      flowNodeType: '',
      nextNodeProxyId: '',
      flowInstanceId: '',
      flowName: '',
      flowProxyId: '',
      selectFlowType: '',
      flowNodeProxyId: '',
      flowInstanceBizRelevanceList: [],
      // 业务完成信号：审批链路 closeCheck → postExamine 事件、转发链路 transpondVisible 关闭时置位。
      businessDone: false
    }
  },
  watch: {
    transpondVisible (val) {
      if (val === false) this.businessDone = true
    }
  },
  methods: {
    // applyExecutionLoad 装载审批现场：模板/权限/值由 Go 装载载荷提供（历史实例数据），
    // 实例、任务、节点与两套身份在 App 的执行身份对象里分存，这里只落组件事实。
    async applyExecutionLoad ({ payload, template, values }) {
      this.jsonData = template && template.list ? template : { list: [], config: {} }
      this.editData = values || {}
      this.isReInitiate = payload.mode === 'resubmit'
      this.initiatorId = String(payload.initiator?.userId ?? '')
      this.jobTaskId = String(payload.jobTaskId ?? '')
      this.flowInstanceId = String(payload.instanceId ?? '')
      this.flowProxyId = String(payload.flowProxyId ?? '')
      this.flowNodeProxyId = String(payload.nodeProxyId ?? '')
      this.flowNodeType = String(payload.flowNodeType ?? '')
      this.nextNodeProxyId = String(payload.nextNodeProxyId ?? '')
      this.selectFlowType = String(payload.selectFlowType ?? '')
      this.flowName = String(payload.toolFlowName ?? '')
      this.flowInstanceBizRelevanceList = Array.isArray(payload.flowInstanceBizRelevanceList)
        ? payload.flowInstanceBizRelevanceList
        : []
      await this.$nextTick()
      const form = this.getGenerateForm()
      if (!form || typeof form.setData !== 'function') throw new Error('审批表单宿主尚未完成装载')
      return { mode: 'injected' }
    },
    getGenerateForm () {
      return this.$refs.generateForm || null
    },
    isBusinessDone () {
      return this.businessDone
    },
    resetBusinessDone () {
      this.businessDone = false
    },
    getOpinion () {
      return this.$refs.opinion || null
    },
    // ===== 审批动作薄适配层：意见写入目标数据后调用真实业务入口 =====
    actionApprove () {
      const opinion = this.mustOpinion()
      opinion.formMakingFormBusiness(true, 'pass')
    },
    actionReject () {
      const opinion = this.mustOpinion()
      // 不同意分支：不做同意专属的表单校验（validateType=false 由目标方法按 type 区分）。
      opinion.formMakingFormBusiness(null, 'no_pass')
    },
    actionStorageFormData () {
      // 暂存只保存检查点，不推进任务；beforeSubmitAndDraft 以 temporary:true 由目标链路区分。
      this.mustOpinion().temporaryStorage()
    },
    actionResubmit () {
      const opinion = this.mustOpinion()
      opinion.isReInitiate = true
      opinion.reSubmitFormMakingFormBusiness(true)
    },
    // setAuditMessage 写入审批意见：目标协议里 auditRecord.executeDesc 的来源。
    setAuditMessage (message) {
      this.mustOpinion().approveForm.approveMessage = String(message ?? '')
    },
    // setTracking 写入审批后的关注布尔（目标协议顶层 tracking 的页面来源）。
    setTracking (value) {
      this.mustOpinion().tracking = Boolean(value)
    },
    putSelectionClosedLoop ({ branchEntryNodeProxyIds = [], nextAuditors = [] } = {}) {
      const opinion = this.mustOpinion()
      for (const auditor of nextAuditors) {
        const exists = opinion.nextAuditorList.some(item =>
          String(item.nodeProxyId ?? '') === String(auditor.nodeProxyId ?? '') && String(item.bizId ?? '') === String(auditor.bizId ?? ''))
        if (!exists) opinion.nextAuditorList.push({ ...auditor })
      }
      for (const branchNodeId of branchEntryNodeProxyIds) {
        if (opinion.chooseBranchList.indexOf(branchNodeId) === -1) opinion.chooseBranchList.push(branchNodeId)
      }
    },
    actionContinueBranchChoice (choice) {
      const opinion = this.mustOpinion()
      this.putSelectionClosedLoop(choice)
      const entryId = choice.branchEntryNodeProxyIds?.[choice.branchEntryNodeProxyIds.length - 1] ?? ''
      if (entryId && (!opinion.chooseBranchNode || !opinion.chooseBranchNode.nextNodeTemplateId)) {
        opinion.chooseBranchNode = { nextNodeTemplateId: entryId, nodeType: 'empty' }
      }
      opinion.handleSaveBranchChooseNode()
    },
    actionContinueParallelChoice (choice) {
      this.mustOpinion()
      this.putSelectionClosedLoop(choice)
      this.mustOpinion().handleSaveParallelChooseNode()
    },
    actionContinueAuditorChoice ({ nextAuditors = [] } = {}) {
      this.putSelectionClosedLoop({ nextAuditors })
      this.mustOpinion().getSelectPerson({
        checkboxPersonGroup: nextAuditors.map(item => ({
          id: item.bizId,
          name: item.name,
          nodeProxyId: item.nodeProxyId
        }))
      })
    },
    mustOpinion () {
      const opinion = this.getOpinion()
      if (!opinion) throw new Error('审批组件尚未完成装载')
      return opinion
    },
    // ===== 转发（复合动作）薄适配层 =====
    // applyForwardLoad 注入行事实（等价于列表页勾选一条流程后点“转发”）；由 App 的
    // load_forward 装载命令调用（评审 P1-4 修复：此前无任何调用方，转发必然缺接收人）。
    // 接收人、意见与附言/日志勾选来自 Go 装载载荷（等价于用户在转发弹窗中的输入），
    // 表单模板与实例当前数据由 Go 离线注入（formProxyData / currentFormData）。
    async applyForwardLoad ({ payload, template, values }) {
      this.formExist = String(payload.formExist ?? '')
      this.flowInstanceId = String(payload.instanceId ?? '')
      this.batchNo = String(payload.batchNo ?? '')
      this.selectFlowType = String(payload.selectFlowType ?? '')
      this.formId = String(payload.formProxyId ?? payload.formId ?? '')
      // selectFlowName 是 mixin confirmTranspond 组装转发流程名的来源；
      // 目标页面取实例名称，执行模式取工具流程名称（同一业务事实，禁止猜测）。
      this.selectFlowName = String(payload.toolFlowName ?? '')
      const bizRelevance = Array.isArray(payload.flowInstanceBizRelevanceList)
        ? payload.flowInstanceBizRelevanceList
        : []
      this.oldFlowInstanceBizRelevanceList = bizRelevance
      const find = bizRelevance.find(item => item.otherBiz === this.selectFlowType)
      this.businessId = find?.otherBizId || ''
      // currentFormData=实例当前完整表单数据；formProxyData=表单模板（FormMaking JSON）。
      this.currentFormData = values && Object.keys(values).length
        ? JSON.parse(JSON.stringify(values))
        : (payload.currentFormData ? JSON.parse(JSON.stringify(payload.currentFormData)) : {})
      this.formProxyData = template && template.list
        ? JSON.parse(JSON.stringify(template))
        : (payload.formProxyData ? JSON.parse(JSON.stringify(payload.formProxyData)) : {})
      this.transpondFlowInstance = {
        selectFlowType: this.selectFlowType,
        businessId: this.businessId,
        formExist: this.formExist
      }
      this.approveForm = {
        approveMessage: String(payload.auditDesc ?? ''),
        checkList: Array.isArray(payload.forwardCheckList) ? payload.forwardCheckList : [],
        personName: String(payload.receiverName ?? ''),
        personId: String(payload.receiverId ?? '')
      }
      await this.$nextTick()
      if (!this.approveForm.personId) throw new Error('转发接收人不能为空，拒绝装载')
      return { mode: 'forward' }
    },
    // fetchData 是 mixin 转发成功后的列表刷新回调；执行模式没有列表，按无操作落地。
    fetchData () {},
    // actionForward 调用真实确认链路：dynamicParam 组装、transpond 请求与附言保存都由 mixin 完成。
    actionForward () {
      if (!this.approveForm.personId) throw new Error('转发接收人缺失，拒绝发送')
      return this.confirmTranspond()
    }
  }
}
</script>
