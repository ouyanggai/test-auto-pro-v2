<template>
  <div class="flowDetail-container">
    <!-- 无表单流程查看 -->
    <!-- :flowName="flowName" -->
    <ExamineDialog
      v-if="ExpensesClaimFormVisible"
      :visible.sync="ExpensesClaimFormVisible"
      :isInitiator="isInitiator"
      :isExamine="isExamine"
      :isReInitiate="isReInitiate"
      :operaType="operaType"
      :flowId="flowId"
      :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType"
      :showFlowLog="showFlowLog"
      :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes"
      :formId="formId"
      :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId"
      :noFormFlowInstanceId="noFormFlowInstanceId"
      :flowProxyId="flowProxyId"
      :initiatorId="initiatorId"
      :actionType="actionType"
      :flowType="flowType"
      :clickRow="clickRow"
      :nextNodeProxyId="nextNodeProxyId"
      :nextNodeName="nextNodeName"
      :jobTaskId="jobTaskId"
      :flowName="selectFlowName"
      :lastCountersignFlag="lastCountersignFlag"
      :tracking="tracking"
      :hideTrackingButton="hideTrackingButton"
      :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"
      @success="handleDialogSuccess"
    />
    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <!-- :flowName="flowName" -->
    <EnterpriseExamineDialog
      v-if="examineDialogVisible"
      :isExamine="isExamine"
      :flowId="flowId"
      :formId="formId"
      :flowNodeProxyId="flowNodeProxyId"
      :jobTaskId="jobTaskId"
      :flowInstanceId="flowInstanceId"
      :btnVisible="btnVisible"
      :visible.sync="examineDialogVisible"
      :isInitiator="isInitiator"
      :selectFlowType="selectFlowType"
      :businessId="businessId"
      :companyId="companyId"
      :showRightSide="showRightSide"
      :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes"
      :actionType="actionType" :operaType="operaType"
      :isReInitiate="isReInitiate"
      :flowNodeType="flowNodeType"
      :initiatorId="initiatorId"
      :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"
      :clickRow="clickRow"
      :nextNodeProxyId="nextNodeProxyId"
      :nextNodeName="nextNodeName"
      :currentPendingNodeName="currentPendingNodeName"
      :flowName="selectFlowName"
      :lastCountersignFlag="lastCountersignFlag"
      :flowProxyId="flowProxyId"
      :tracking="tracking"
      :hideTrackingButton="hideTrackingButton"
      @success="handleDialogSuccess"
    />

    <!-- 查看流程 -->
    <CheckFlowNodeDetail
      v-if="checkViewFlowDetailVisible"
      :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId"
      :flowId="flowId"
      :initiatorId="initiatorId"
    ></CheckFlowNodeDetail>
  </div>
</template>
<script>
import ExamineDialog from "@/views/GroupApproveManage/components/ExamineDialog.vue";
import EnterpriseExamineDialog from "@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue";
import CheckFlowNodeDetail from "@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue";
import { deepClone } from '@/utils';
import Api from "@/api";
import { localstorageGet } from "@/utils/auth";
export default {
  name: "",
  components: {
    ExamineDialog,
    EnterpriseExamineDialog,
    CheckFlowNodeDetail,
  },
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    dialogType: {
      type: String,
      default: ''
    },
    propData: {
      type: Object,
      default: (_) => ({ chooseData: false }),
    },
  },
  data() {
    return {
      //   selectFlowType: '',
      ExpensesClaimFormVisible: false,
      operaType: "",
      //   flowId:'',
      //   flowInstanceId:'',
      //   flowNodeType:'',
      showFlowLog: "",
      //   parallelNodeChooseList:'',
      //   manualChooseNodes:'',
      //   formId:'',
      searchFlowType: "",
      //   flowNodeProxyId:'',
      noFormFlowInstanceId: "",
      flowProxyId: "",
      //   initiatorId:'',
      actionType: "",
      //   flowType:'',
      //   flowName:'',
      //   isExamine:false,
      //   isReInitiate:false,


      currentRowFlowData: {},
      checkViewFlowDetailVisible: false,
      isReInitiate: false,
      btnVisible: false,
      flowTemplateVisible: false,
      approveDialogVisible: false,
      flowJson: {},
      selectFlowType: "",
      flowType: "", // 默认为合同盖章评审
      flowTempList: [],
      examineDialogVisible: false,
      initiatorId: "", // 发起人id
      flowName: "",
      flowNodeType: "",
      flowId: "", // 绑定的业务id
      flowInstanceId: "", // 流程实例id
      formId: "",
      flowNodeProxyId: "",
      jobTaskId: "",
      businessId: "",
      companyId: "",
      isExamine: false,
      showRightSide: true,
      parallelNodeChooseList: [],
      manualChooseNodes: [],
      flowInstanceBizRelevanceList:[],
      clickRow: {},
      nextNodeProxyId: '',
      nextNodeName: '',
      currentPendingNodeName: '',
      lastCountersignFlag: false,
      tracking: false,
      hideTrackingButton: false,
      dialogChanged: false,
    };
  },
  computed: {
    isInitiator() {
      return this.initiatorId == localstorageGet("userId");
    },
  },
  watch: {
    checkViewFlowDetailVisible(val) {
      if (!val) {
        if (this.propData.callback) {
          this.propData.callback({ refresh: false, success: false });
        }
        this.dialogChanged = false;
        this.handleClose();
      }
    },
    examineDialogVisible(val) {
      console.log('examineDialogVisible', val);
      if (!val){
        console.log('发送组件事件',this.propData.data)
        if (this.propData.data.row) {
          this.$bus.$emit('messageCenterSuccess')
        }
        if (this.propData.callback) {
          this.propData.callback({
            refresh: this.dialogChanged === true,
            success: this.dialogChanged === true,
          })
        }
        this.dialogChanged = false;
        this.handleClose();
      }

      // val || this.handleClose();
    },
    ExpensesClaimFormVisible(val) {
      console.log('ExpensesClaimFormVisible', val);
      if (!val){
        console.log('发送组件事件',this.propData.data)
        if (this.propData.data.row) {
          this.$bus.$emit('messageCenterSuccess')
        }
        if (this.propData.callback) {
          this.propData.callback({
            refresh: this.dialogChanged === true,
            success: this.dialogChanged === true,
          })
        }
        this.dialogChanged = false;
        this.handleClose();
      }
      // val || this.handleClose();
    }
  },
  created() {},
  mounted() {
    console.log(this.propData, "propData");
    // setTimeout(() => {
    //   this.$emit("confirmed", 2343);
    // }, 5000);
    this.handleView();
  },
  methods: {
    normalizeTrackingValue(value, fallback) {
      const target = value === undefined || value === null ? fallback : value;
      if (target === undefined || target === null) {
        return false;
      }
      if (typeof target === 'boolean') {
        return target;
      }
      if (typeof target === 'number') {
        return target !== 0;
      }
      if (typeof target === 'string') {
        const normalized = target.trim().toLowerCase();
        if (['1', 'true', 'yes', 'y', 'on', '是', '跟踪', '已跟踪', '已设为跟踪'].includes(normalized)) {
          return true;
        }
        if (['0', 'false', 'no', 'n', 'off', '否', '', '不跟踪', '未跟踪', '取消跟踪'].includes(normalized)) {
          return false;
        }
        return false;
      }
      return false;
    },
    shouldHideTrackingButton(row) {
      const propData = this.propData?.data || {};
      const isEnd = row?.flowStatus === 'end' || row?.status === 'end' || propData.flowStatus === 'end' || propData.status === 'end';
      if (isEnd) {
        return true;
      }
      const explicit = row?.hideTrackingButton !== undefined ? row.hideTrackingButton : propData.hideTrackingButton;
      if (typeof explicit === 'boolean') {
        return explicit;
      }
      return false;
    },
    syncTrackingState(row) {
      const tracking = this.normalizeTrackingValue(
        row?.tracking,
        row?.trackingFlag !== undefined ? row.trackingFlag : this.propData?.data?.tracking
      );
      const hideTrackingButton = this.shouldHideTrackingButton(row);
      this.tracking = tracking;
      this.hideTrackingButton = hideTrackingButton;
      return {
        ...(row || {}),
        tracking,
        trackingFlag: tracking,
        hideTrackingButton,
      };
    },
    handleDialogSuccess() {
      this.dialogChanged = true;
    },
    // 查看流程
    async handleCheckFlow(row = this.currentRowFlowData) {
      this.currentRowFlowData = row;
      // 查看流程
      this.selectFlowType = row.auditWay;
      this.flowId = row.flowProxyId;
      this.flowInstanceId = row.flowInstanceId || row.id;
      this.initiatorId = row.createrId || row.initiatorId;
      this.checkViewFlowDetailVisible = true;
    },
    handleView() {
      // 查询当前绑定的流程，调用查看弹窗
      this.getInstanceId().then((data) => {
        if (data) {
          console.log('消息中心-handleView',data)
          let newData = this.propData.data?.row || data;
          if (!newData.flowInstanceId) { newData.flowInstanceId = newData.id; }
          if (this.propData.data.operaType === 'reEdit') {
            this.clickReInitiate(newData);
          } else if (this.propData.data.operaType === 'flow') {
            this.handleCheckFlow(newData);
          } else {
            this.previewHandle(newData);
          }
        } else {
          this.$message.error("流程已删除");
          this.handleClose();
        }
      });
    },
    previewHandle(row) {
      row = this.syncTrackingState(row);
      this.clickRow = row;
      if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) { // 转发流程
        const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
        this.formExist = formExistType?.otherBizId || '';
        const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
        row.auditWay = auditWayType?.otherBizId || '';
      } else { // 不是转发流程
        this.formExist = row.formExist;
      }
      console.log('this.formExist',this.formExist)
      this.parallelNodeChooseList = [];
      this.manualChooseNodes = []
      this.initiatorId = row.createrId;
      this.tracking = row.tracking
      if (!this.formExist) { // formMaking表单
        console.log('进入有表单')
        this.operaType = "check";
        this.actionType = "preview";
        // this.currentRowFlowData = row;
        // this.isExamine = true;
        this.isExamine = this.propData.data?.isExamine || false;
        this.btnVisible = this.propData.data?.isExamine || false;
        // this.btnVisible = type;
        this.flowId = row.flowProxyId;
        this.flowInstanceId = row.flowInstanceId || row.id;
        this.batchNo = row.batchNo;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.flowNodeProxyId || row.currentNodeProxyId;
        this.flowProxyId = row.flowProxyId;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.nextNodeProxyId = row.nextNodeProxyId;
        this.nextNodeName = row.nextNodeName;
        this.currentPendingNodeName = row.currentPendingNodeName; // 当前审批节点
        this.jobTaskId = row.jobTaskId;
        this.selectFlowType = row.auditWay;
        this.selectFlowName = row.flowName || row.formName; // 不要用flowInstanceName，可能会出现叠加

        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
          this.businessId = find?.otherBizId || '';
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.businessId = find?.otherBizId || '';
        }
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        this.examineDialogVisible = true;

        // this.selectFlowType = row.auditWay;
        // this.flowId = row.flowProxyId;
        // this.flowInstanceId = row.id;
        // this.formId = row.formProxyId;
        // this.flowNodeProxyId = row.currentNodeProxyId;
        // this.jobTaskId = row.jobTaskId;
        // this.isExamine = false;
        // this.isReInitiate = false;
        // this.btnVisible = false;
        // this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        // const find = row.flowInstanceBizRelevanceList.find(
        //   (item) => item.otherBiz == row.auditWay
        // );
        // this.businessId = find?.otherBizId || '';
        // const company = row.flowInstanceBizRelevanceList.find(
        //   (item) => item.otherBiz == "company"
        // );
        // this.companyId = company?.otherBizId || "";
        // this.examineDialogVisible = true;
      } else {
        console.log('进入无表单')
        // this.currentRowFlowData = row;
        // this.selectFlowType = row.auditWay;
        // this.formExist = row.formExist;
        // this.operaType = "check";
        // this.actionType = "preview";
        this.operaType = this.propData.data?.operaType || "check";
        this.actionType = this.propData.data?.actionType || "preview";
        // this.isExamine = false;
        // this.isReInitiate = false;
        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        if (row.auditWay == 'annual_perf'||row.auditWay == 'year_kpi_work_target') {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
          if (find) { // 管理指标
            this.flowType = 'manageTarget';
          } else { // 工作指标
            this.flowType = 'workTarget';
          }
        }
        this.searchFlowType = row.auditWay;
        this.isExamine = this.propData.data?.isExamine || false;
        this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
            this.flowId = find?.otherBizId || '';
          } else {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
            this.flowId = find.otherBizId;
          }
        }
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.batchNo = row.batchNo;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.nextNodeProxyId = row.nextNodeProxyId;
        this.nextNodeName = row.nextNodeName;
        this.jobTaskId = row.jobTaskId;
        this.flowProxyId = row.flowProxyId;
        this.selectFlowName = row.flowName || row.formName;

        this.ExpensesClaimFormVisible = true;
        // this.btnVisible = this.propData.data?.btnVisible || false;

        // if (row.flowInstanceBizRelevanceList.length == 1) {
        //   this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        // } else {
        //   const find = row.flowInstanceBizRelevanceList.find(
        //     (item) => item.otherBiz == row.auditWay
        //   );
        //   this.flowId = find.otherBizId;
        // }
        // if (row.auditWay == 'annual_perf' || row.auditWay == 'year_kpi_work_target') {
        //   let find = row.flowInstanceBizRelevanceList.find(item=>item.otherBiz == 'manageTarget')
        //   if(find){ //管理指标
        //     this.flowType = 'manageTarget'
        //   }else{ //工作指标
        //     this.flowType = 'workTarget'
        //   }
        // }
        // this.flowInstanceId =
        //   row.flowInstanceBizRelevanceList[0].flowInstanceId;
        // this.searchFlowType = row.auditWay;
        // this.flowProxyId = row.flowProxyId;
      }
    },
    getInstanceId() {
    //   let otherBiz = type;
      const flowInstanceBizRelevanceList = this.propData.data.flowInstanceBizRelevanceList;
      const taskStatus = this.propData.data.taskStatus;
      const data = {
        id: this.propData.data.id,
        useScope: "invest",
        // taskStatus:'waiting_send',
        // statusList:["await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end","draft"],//: 'waiting_send',
        initiator: "all",
        // auditWayList: this.sFlowTypeList,
        flowInstanceBizRelevanceList,
      };

      let api;
      if (taskStatus == "edit") {
        data.taskStatus = "waiting_send";
        api = Api.approveManage.getTaskList;
      } else {
        api = Api.schedule.getFlowInstanceList;
      }
      return new Promise((resolve, reject) => {
        this.$axios
          .post(api, { data, size: 1, pagination: true, pages: 1 })
          .then((res) => {
            if (res.isSuccess) {
              let data = res?.data || [];
              if (data.length) {
                resolve(data[0]);
              } else {
                resolve();
              }
            }
          });
      });
    },
    handleClose() {
      this.$emit("update:visible", false);
      if (this.dialogType === 'appendToBody') {
        this.$destroy();
        this.$el.remove();
      }
    },
    clickReInitiate(row, type) {
      row = this.syncTrackingState(row);
      console.log('clickReInitiate',row)
      if (row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) { // 转发流程
        const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
        this.formExist = formExistType?.otherBizId || '';
        const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
        row.auditWay = auditWayType?.otherBizId || '';
      } else { // 不是转发流程
        this.formExist = row.formExist;
      }

      this.flowStatus = row.flowStatus
      this.selectFlowType = row.auditWay;
      // 这里的接口和已提的接口不一样字段也不一样---参考审批功能
      this.parallelNodeChooseList = [];
      this.manualChooseNodes = [];
      if (row.nextNodeType == 'parallel') {
        // 下一节点为并行,取出其中需要自选的节点
        row.nextAuditNodeList.map(x => {
          if (x.flowNodeAuditConfig?.auditType == 'run_node_choose') {
            // 取出其中的审批人自选节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          } else if (x.flowNodeAuditConfig?.auditType == 'department_supervisor' || x.flowNodeAuditConfig?.auditType == 'branched_passage_manager') {
            // 取出其中的主管审批和副总审批的类型节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          }
        });
      }
      if (row.auditPassLogicFlag && row.branchExecuteType == 'custom_choose' && row.nextAuditNodeList.length > 1) {
        // 下一节点为手动分支
        row.nextAuditNodeList.map((x, index) => {
          this.manualChooseNodes.push({
            nextNodeTemplateId: x.id,
            nodeName: x.nodeName,
            nodeType: x.type, // 为处理空节点
            branchName: '分支' + (index + 1),
            auditType: x.flowNodeAuditConfig.auditType
          });
        });
      }
      this.flowName = row.flowName;
      // this.formExist = row.formExist;
      this.nextNodeName = row.nextNodeName;

      // if (await this.isForm(row.formProxyId)) {
      if (!this.formExist) { // formMaking表单
        // formMaking表单
        this.operaType = 'reEdit';
        this.actionType = 'edit';
        this.isExamine = false;
        this.isReInitiate = true;
        this.initiatorId = row.initiatorId;
        this.flowId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.flowNodeProxyId || row.currentNodeProxyId;
        this.flowProxyId = row.flowProxyId;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.flowInstanceId = row.flowInstanceId || row.id;;
        this.jobTaskId = row.jobTaskId;
        this.examineDialogVisible = true;

        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
      } else {
        // 固定页面
        this.operaType = 'reEdit';
        if (row.auditWay == 'expense_budget') this.operaType = 'edit';
        this.actionType = 'edit';
        if (row.auditWay == 'annual_perf' || row.auditWay == 'year_kpi_work_target') {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
          if (find) { // 管理指标
            this.flowType = 'manageTarget';
          } else { // 工作指标
            this.flowType = 'workTarget';
          }
        }
        this.isExamine = false;
        this.initiatorId = row.initiatorId;
        this.flowNodeProxyId = row.flowNodeProxyId || row.currentNodeProxyId;
        this.flowNodeType = row.flowNextNodeAuditType || 'run_node_choose';
        // this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId;
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.flowId = find.otherBizId;
        }
        this.flowProxyId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.noFormFlowInstanceId = row.flowInstanceId || row.id;;
        this.searchFlowType = row.auditWay;
        this.ExpensesClaimFormVisible = true;
      }
    },
  },
};
</script>

  <style scoped lang="scss">
::v-deep {
  .dytable-view-container {
    padding: 0;
  }
}
::v-deep .el-dialog {
  // margin-top: 5vh !important;
}
</style>
