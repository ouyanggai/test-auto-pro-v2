<!--
 * @Descripttion: 超时跳过
 * @Author:
 * @Date: 2025-10-23 17:50:00
-->
<template>
  <div class="container">
    <dy-table
      :fetchData="fetchData"
      :actions="actions"
      :keys="colKey"
      :list="tableData"
      :isPagination="true"
      :pagination="pagination"
      :height="height"
      ref="dytable">
    </dy-table>
    <!-- 查看弹窗(对固定页面的查看) -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :searchFlowType="searchFlowType" :isInitiator="isInitiator" :actionType="actionType" :flowType="flowType"
      :flowProxyId="flowProxyId" :formId="formId" :tracking="tracking" @success="fetchData" :initiatorId="initiatorId" :isTimeoutPage="true"/>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId" :formId="formId"
      :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :selectFlowType="selectFlowType" :batchNo="batchNo" :actionType="actionType" :operaType="operaType"
      :visible.sync="examineDialogVisible" :businessId="businessId" :companyId="companyId" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :initiatorId="initiatorId" :tracking="tracking" :isInitiator="isInitiator" :isTimeoutPage="true" @success="fetchData"/>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { approveManageFlowStatus } from '@/utils';
import {deepClone} from '@/utils'
import { localstorageGet } from '@/utils/auth';
import ExamineDialog from '../components/ExamineDialog';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';

const EnterpriseExamineDialog = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue');

export default {
  name: 'TimedOut',
  components: { DyTable, ExamineDialog, EnterpriseExamineDialog, CheckFlowNodeDetail },
  props: {
    flowName: {
      type: String,
      default: ''
    },
    startDate: {
      type: String,
      default: ''
    },
    endDate: {
      type: String,
      default: ''
    },
    initiator: {
      type: String,
      default: ''
    },
    status: {
      type: String,
      default: ''
    },
    sFlowTypeList: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    return {
      tableData: [],
      height: null,
      btnVisible: false,
      colKey: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '430',
          handle: (scope, createElement) => {
            const flowName = scope.row.name || scope.row.formName;
            return createElement('span', flowName);
          }
        },
        initiatorName: {
          label: '发起人',
          minWidth: 80
        },
        initiatorDate: {
          label: '发起时间',
          showTooltip: true,
          minWidth: '160'
        },
        currentAuditUserInfo: {
          label: '当前处理人',
          minWidth: '100',
          handle: (scope, createElement) => {
            let currentAuditUserInfo = scope.row.currentAuditUserInfo || {};
            let strArr = [];
            for (let key in currentAuditUserInfo) {
              let userList = currentAuditUserInfo[key]?.userList || [];
              userList.forEach(el => {
                strArr.push(el.name);
              });
            }
            let str = strArr.join(',');
            return createElement('el-tooltip', { props: { content: str } }, [<span>{str}</span>]);
          }
        },
        flowStatus: {
          label: '当前状态',
          minWidth: 80,
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.flowStatus));
          }
        }
      },
      actions: [
        {
          label: '查看',
          width: '120',
          actionFixed: 'right',
          action: row => {
            this.previewHandle(row);
          }
        },
        {
          label: '查看流程',
          action: row => {
            this.handleCheckFlow(row);
          }
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      // 查看相关数据
      ExpensesClaimFormVisible: false,
      examineDialogVisible: false,
      checkViewFlowDetailVisible: false,
      operaType: '',
      actionType: '',
      flowId: '',
      isExamine: false,
      flowType: '',
      initiatorId: '',
      checkFlowId: '',
      flowInstanceBizRelevanceList: [],
      formId: '',
      flowInstanceId: '',
      flowProxyId: '',
      searchFlowType: '',
      isInitiator: false,
      tracking: '',
      batchNo: '',
      jobTaskId: '',
      flowNodeProxyId: '',
      selectFlowType: '',
      businessId: '',
      companyId: '',
      isReInitiate: false,
      formExist: ''
    };
  },
  watch: {
    sFlowTypeList: {
      handler: function () {
        this.pagination = {
          total: 0,
          pages: 1,
          size: 15
        };
        this.fetchData();
      },
      deep: true
    }
  },
  async mounted() {
    let clientHeight = document.querySelector('.content-container').clientHeight;
    this.height = clientHeight - 170;
  },
  methods: {
    queryData(query) {
      this.query = {
        ...query
      };
      this.pagination.pages = 1;
      this.fetchData();
    },
    fetchData() {
      let data = {
        name: this.query?.name || '',
        queryStartDate: this.query?.queryStartDate || '',
        queryEndDate: this.query?.queryEndDate || '',
        initiator: localstorageGet('userName'),
        // initiator: this.query?.initiator || '',
        status: this.query?.flowStatus || null,
        ...this.query,
        groupFlag: true
      };

      this.$axios.post(
        '/web/flowInstanceApi/timeoutSkipList',
        {
          data,
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            this.pagination.total = res.total;
            this.tableData = res?.data || [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    previewHandle(row) {
      // 根据Finished组件实现查看功能
      this.selectFlowType = row.auditWay
      this.formExist = row.formExist

      // 判断是否是发起人，方便他写附言
      let createrId = row.initiatorId
      this.isInitiator = false
      if(createrId == localstorageGet('userId')){
        this.isInitiator = true
      }
      this.tracking = row.tracking

      if (!this.formExist) { // formMaking表单
        this.operaType = 'check';
        this.actionType = 'preview'
        this.isExamine = false;
        this.isReInitiate = false;
        this.flowId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowInstanceId = row.id;
        this.batchNo = row.batchNo;
        this.jobTaskId = row.jobTaskId;
        this.selectFlowType = row.auditWay;
        this.examineDialogVisible = true;
        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);

        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
      } else { // 固定页面
        this.operaType = 'check';
        this.actionType = 'preview' //非费用预算流程
        if(row.auditWay == 'annual_perf'||row.auditWay == 'year_kpi_work_target'){
          let find = row.flowInstanceBizRelevanceList.find(item=>item.otherBiz == 'manageTarget')
          if(find){ //管理指标
            this.flowType = 'manageTarget'
          }else{ //工作指标
            this.flowType = 'workTarget'
          }
        }
        this.isExamine = false;
        if(row.flowInstanceBizRelevanceList.length == 1){
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        }else{
          let find = row.flowInstanceBizRelevanceList.find(item=>item.otherBiz == row.auditWay)
          this.flowId = find.otherBizId
        }
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.batchNo = row.batchNo;
        this.searchFlowType = row.auditWay;
        this.initiatorId = row.initiatorId;
        this.flowProxyId = row.flowProxyId;
        this.ExpensesClaimFormVisible = true;
      }
    },
    handleCheckFlow(row) {
      // 根据Finished组件实现查看流程功能
      this.checkFlowId = row.flowProxyId;
      this.flowInstanceId = row.id;
      // this.initiatorId = row.initiatorId;
      this.initiatorId = row.createrId;
      this.checkViewFlowDetailVisible = true;
    }
  }
};
</script>

<style scoped lang="scss"></style>
