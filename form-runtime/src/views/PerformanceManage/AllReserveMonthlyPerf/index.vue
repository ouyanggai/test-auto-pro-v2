
<!-- 查看所有轮岗人员的月度绩效 -->

<template>
  <div class="target-book-container">
    <el-main>
      <el-card class="box-card">
        <div slot="header">
          <el-form :model="searchForm" style="margin-top: 20px" label-width="90px" inline>
            <el-form-item label="">
              <el-date-picker
                style="width:180px;"
                v-model="searchForm.targetTime"
                type="month"
                format="yyyy年M月"
                value-format="yyyyM"
                placeholder="请选择考核时间">
              </el-date-picker>
            </el-form-item>
            <el-form-item label="">
              <el-input  v-model="searchForm.companyName" placeholder="按轮岗公司查询" clearable></el-input>
            </el-form-item>
            <el-form-item label="">
              <el-input  v-model="searchForm.depName" placeholder="按轮岗部门查询" clearable></el-input>
            </el-form-item>
            <el-form-item label="">
              <el-input  v-model="searchForm.userName" placeholder="按轮岗人员查询" clearable></el-input>
            </el-form-item>
            <el-form-item label="">
              <el-select v-model="searchForm.kpiGroupStatus" filterable placeholder="状态" class="search-select">
                <el-option v-for="item in optionsList" :key="item.value" :label="item.label" :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
            <el-button
              type="primary"
              @click="search"
              style="margin-left: 10px"
            >查询</el-button>
          </el-form>
        </div>
        <dy-table maxTableHeight="600" :keys="colKeyReserve" :actions="actionsReserve" :fetchData="fetchReserveData" :list="reserveTableData"
          style="padding:0px;" ref="reserveTable" :isPagination="true" :pagination="reservePagination">
        </dy-table>
      </el-card>
    </el-main>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :isExamine="isExamine" :flowId="flowId" :formId="formId"
    :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :btnVisible="btnVisible"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" :companyId="companyId"
    :showRightSide="showRightSide" :parallelNodeChooseList="parallelNodeChooseList" :manualChooseNodes="manualChooseNodes"
    :flowName="flowName" :isReInitiate="isReInitiate" :flowNodeType="flowNodeType" :initiatorId="initiatorId" />
    <!-- @success="getContractInfoList" -->

    <!-- 查看流程 -->
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
    :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import mixin from '../TargetBook/components/mixin';
import { localstorageSet } from '@/utils/auth.js';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import Api from '@/api';
import math from '@/utils/math.js'

var statusOptions = {
  not_submitted: {
    tag: 'warning',
    statusName: '草稿'
  },
  under_review: {
    tag: 'primary',
    statusName: '审核中'
  },
  rejected: {
    tag: 'danger',
    statusName: '驳回'
  },
  pass: {
    tag: 'success',
    statusName: '已通过'
  },
  finish: {
    tag: 'success',
    statusName: '已通过'
  }
};
export default {
  name: 'MonthlyPerf',
  components: { DyTable,EnterpriseExamineDialog,CheckFlowNodeDetail },
  mixins: [mixin],
  props: [],
  data() {
    return {
      // 查看轮岗考核表流程数据
      currentRowFlowData:{},
      checkViewFlowDetailVisible: false,
      isReInitiate:false,
      btnVisible:false,
      flowTemplateVisible:false,
      approveDialogVisible:false,
      flowJson:{},
      selectFlowType:'',
      flowType:'', // 默认为合同盖章评审
      flowTempList:[],
      examineDialogVisible: false,
      initiatorId: '', // 发起人id
      flowName: '',
      flowNodeType: '',
      flowId: '', // 绑定的业务id
      flowInstanceId: '', // 流程实例id
      formId: '',
      flowNodeProxyId: '',
      jobTaskId: '',
      businessId: '',
      companyId: '',
      isExamine: false,
      showRightSide:true,
      parallelNodeChooseList: [],
      manualChooseNodes: [],

      // 轮岗月度绩效数据
      searchForm:{
        targetTime:'',
        kpiGroupStatus: undefined,
        depName: '',
        userName:'',
        companyName:''
      },
      reserveTableData:[],
      reservePagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      colKeyReserve: {
        // title: {
        //   label: '名称',
        //   showTooltip:true,
        // },
        targetTime: {
          label: '考核时间',
          ifFixed:true,
          handle(scope, createElement) {
            const targetTime = scope.row.targetTime + '';
            if (targetTime) {
              const year = targetTime.substr(0, 4);
              let month = targetTime.substr(4, 2);
              month = month < 10 ? '0' + month : month;
              return createElement('span', year + '-' + month);
            } else {
              return '';
            }
          }
        },
        // createDate: {
        //   label: '发起日期',
        //   showTooltip:true,
        // },
        companyName: {
          label: '轮岗公司',
          showTooltip: true,
          width: '160px',
          ifFixed:true,
        },
        depName: {
          label: '轮岗部门',
          showTooltip: true,
          ifFixed:true,
        },
        userName: {
          label: '轮岗人员',
          ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row.user.name);
          }
        },
        totalScore: {
          label: '最终得分',
          ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row?.totalScore||0);
          }
        },
        extraPointsValue: {
          label: '奖励分值',
          handle(scope, createElement) {
            return createElement('span', scope.row?.extraPointsValue || '-');
          }
        },
        deductPointsValue: {
          label: '扣罚分值',
          handle(scope, createElement) {
            return createElement('span', scope.row?.deductPointsValue || '-');
          }
        },
        totalKpi: {
          label: '岗效比例',
          handle(scope, createElement) {
            var m = math.multiply(scope.row?.totalKpi || 0, 100);
            var text = parseInt(m) + '%'; // m.toFixed(2) + '%';
            return createElement('span', text);
          }
        },
        changeSalary: {
          label: '调薪情况',
          width: '200px',
          showTooltip:true,
          handle(scope, createElement) {
            let newVal = scope.row.kpiDynamicItemList.find(x=>x.kpiDynamicType == 'change_salary');
            return createElement('span', newVal?.desc || '无');
          }
        },
        comment: {
          label: '考核结果',
          width: '200px',
          showTooltip:true,
        },
        kpiGroupStatus: {
          label: '状态',
          handle: function (scope, createElement) {
            return createElement('el-tag', {
              attrs: {
                type: statusOptions[scope.row.kpiGroupStatus].tag
              },
              domProps: {
                innerHTML: statusOptions[scope.row.kpiGroupStatus].statusName
              }
            }
            );
          }
        },
      },
      actionsReserve: [
        {
          label: '查看详情',
          width: '120px',
          action: (row) => {
            this.checkFlowDetail(row)
          }
        }
      ],
      optionsList: [{
        value: undefined,
        label: '全部状态'
      }, {
        value: 'not_submitted',
        label: '草稿'
      }, {
        value: 'under_review',
        label: '审核中'
      }, {
        value: 'rejected',
        label: '驳回'
      }, {
        value: 'pass',
        label: '已通过审核'
      }, {
        value: 'finish',
        label: '已完成'
      }]
    };
  },
  created() {},
  activated(){
    this.fetchReserveData();
  },
  mounted() {
  },
  watch: {},
  computed: {},
  methods: {
    search() {
      this.reservePagination.pages = 1;
      this.reservePagination.size = 10;
      this.fetchReserveData();
    },
    // 查询轮岗月度绩效列表
    fetchReserveData() {
      const param = {
        data: {
          kpiScope: "my_company_group",
          ...this.searchForm
        },
        pagination:true,
        size:this.reservePagination.size,
        current:this.reservePagination.pages
      };
      this.$axios.post(
        Api.performance.getReserveList,
        param,
        res => {
          if (res.isSuccess) {
            const reserveTableData = res.data ? res.data : [];
            this.reserveTableData = reserveTableData;
            this.reservePagination.total = res.total
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleView(row,type) {
      //查询当前绑定的流程，调用查看弹窗
      this.getInstanceId(row.id,type).then(data=>{
        if(data){
          this.previewHandle(data,type)
        }else{
          if (type == 'monthly_perf'){
            this.$message.error('流程已删除，请删除本条月度绩效后重新发起月度流程')
          }
        }
      })
    },
    // 获取表单流程数据（传多条业务id获取数据）
    getAllFormFlowDataByBizId(id) {
      console.log('getAllFormFlowDataByBizId')
      let data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'monthly_perf_reserveTalent',
            otherBizId: id
          },
        ],
        initiator:"all"
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        )
      })
    },
    // 查看流程
    async handleCheckFlow() {
      let row = this.currentRowFlowData;
      // 查看流程
      this.selectFlowType = row.auditWay;
      this.flowId = row.flowProxyId;
      this.flowInstanceId = row.id;
      this.initiatorId = row.createrId;
      this.checkViewFlowDetailVisible = true;
    },
    // 点击查看轮岗绩效详情
    async checkFlowDetail(row){
      let flowData = await this.getAllFormFlowDataByBizId(row.id);
      console.log('flowData',flowData)
      this.previewHandle(flowData[0],'monthly_perf_reserveTalent')
    },
    // 查看详情
    previewHandle(row,type){
      console.log('previewHandle',row)
      if (type == 'monthly_perf_reserveTalent') { // 轮岗月度绩效
        this.currentRowFlowData = row;
        
        this.selectFlowType = row.auditWay;
        this.flowId = row.flowProxyId;
        this.flowInstanceId = row.id;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.currentNodeProxyId;
        this.jobTaskId = row.jobTaskId;
        this.isExamine = false;
        this.isReInitiate = false;
        this.btnVisible = false;
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find.otherBizId;
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        this.examineDialogVisible = true;
      } else if(type == 'monthly_perf'){ // 月度绩效
        this.selectFlowType = row.auditWay;
        this.formExist = row.formExist;
        this.operaType = 'check';
        this.actionType = 'preview';
        this.isExamine = false;
        this.isReInitiate =false
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.flowId = find.otherBizId;
        }
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.searchFlowType = row.auditWay;
        this.flowProxyId = row.flowProxyId
        this.ExpensesClaimFormVisible = true;
      }
    },
  }
};
</script>
<style lang="scss" scoped>
.target-book-container {
  height: 100%;
  background: #fff;
}
.title{
  display: inline-block;
  margin-left: 30px;
}
</style>
