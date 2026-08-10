<template>
  <div class="target-book-container">
    <div style="padding: 1px 10px 0 10px;">
      <el-form :model="searchForm" style="margin-top:10px;overflow:auto" label-width="90px" inline>
            <!-- <el-form-item label="">
              <el-date-picker
                style="width:180px;"
                v-model="searchForm.targetTime"
                type="month"
                format="yyyy年M月"
                value-format="yyyyM"
                placeholder="请选择考核时间">
              </el-date-picker>
            </el-form-item> -->
            <el-form-item label="">
              <el-input  v-model="searchForm.companyName" placeholder="按公司查询" clearable  v-if="!isPersonal"></el-input>
            </el-form-item>
            <el-form-item label="">
              <el-input  v-model="searchForm.depName" placeholder="按部门查询" clearable v-if="!isPersonal"></el-input>
            </el-form-item>
            <!-- <el-form-item label="">
              <el-input  v-model="searchForm.userName" placeholder="按姓名查询" clearable></el-input>
            </el-form-item>
            <el-form-item label="">
              <el-select v-model="searchForm.kpiGroupStatus" filterable placeholder="状态" class="search-select">
                <el-option v-for="item in optionsList" :key="item.value" :label="item.label" :value="item.value">
                </el-option>
              </el-select>
            </el-form-item> -->
            <el-button  v-if="!isPersonal"
              type="primary"
              @click="searchClick"
              style="margin-left: 10px"
            >查询</el-button>
            <!-- <el-button
              type="primary"
              @click="exportListData"
              style="margin-left: 10px"
            >导出</el-button> -->
            <el-button type="primary" style="float:right;margin-right:10px;margin-top:5px" v-if="isPersonal" @click="getFlowByTemplate()">发起流程</el-button>
          </el-form>
          <div style="margin-top: 10px;">
            <dy-table maxTableHeight="600" :keys="colKey" :fetchData="fetchData" :list="tableData"
              style="padding:0px;" ref="usingTable" :isPagination="true" :pagination="pagination">
            </dy-table>
          </div>
    </div>
    <chooseFlowDialog :visible.sync="visible" @confirm="confirm" :flowList="flowList"></chooseFlowDialog>
    <!-- 无表单：再次发起流程/查看 -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="isReInitiate" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
      :initiatorId="initiatorId" @success="fetchData" :actionType="actionType" :flowType="flowType" :flowName="flowName"/>

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

    <!-- 公司流程发起的弹窗 -->
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="[]" v-if="approveDialogVisible" @success="fetchData"
      :flowJson.sync="flowJson" :flowType.sync="flowType" :closeAll="true" />
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import mixin from '../../TargetBook/components/mixin';
import chooseFlowDialog from '../../TargetBook/components/chooseFlowDialog.vue';
import { localstorageSet } from '@/utils/auth.js';
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
import FlowDialog from '@/views/GroupApproveManage/Submitted/components/FlowDialog.vue'
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import Api from '@/api';
import math from '@/utils/math.js'

var options = {
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
var publicity = { not_started: '未公示', in_progress: '公示中', end: '公示结束' };
var publicityColor = { not_started: 'danger', in_progress: 'success', end: '' };
export default {
  name: 'MonthlyPerf',
  components: { DyTable, chooseFlowDialog, ExamineDialog,FlowDialog,EnterpriseExamineDialog,CheckFlowNodeDetail },
  mixins: [mixin],
  props: [],
  data() {
    return {
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' },
      isPersonal: false,
      searchForm: {
        targetTime: '',
        kpiGroupStatus: undefined,
        depName: '',
        userName: '',
        companyName: ''
      },
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
      }],
      // 查看轮岗考核表流程数据
      currentRowFlowData:{},
      checkViewFlowDetailVisible: false,
      isReInitiate:false,
      btnVisible:false,
      flowTemplateVisible:false,
      approveDialogVisible:false,
      flowJson:{},
      selectFlowType:'',
      flowType:'kpi2_work_plan',
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
          width: '160px',
          ifFixed:true,
        },
        depName: {
          label: '轮岗部门',
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
        rewardPonitsValue: {
          label: '奖励分值',
          handle(scope, createElement) {
            return createElement('span', scope.row?.rewardPonitsValue || '-');
          }
        },
        punishPonitsValue: {
          label: '扣罚分值',
          handle(scope, createElement) {
            return createElement('span', scope.row?.punishPonitsValue || '-');
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
            // let newVal = scope.row.kpiDynamicItemList.find(x=>x.kpiDynamicType == 'change_salary');
            // return createElement('span', newVal?.desc || '无');
          }
        },
        comment: {
          label: '考核结果',
          width: '300px',
          showTooltip:true,
          handle(scope, createElement) {
            // let newVal = scope.row.kpiDynamicItemList.find(x=>x.kpiDynamicType == 'ask_for_leave');
            // var m = math.multiply(scope.row?.totalKpi || 0, 100);
            // var text = m.toFixed(2) + '%';
            // return createElement('span', newVal?.desc || `${text}岗效工资+基本工资+餐费20元*${scope.row.workDays || 0}`);
          }
        },
        kpiGroupStatus: {
          label: '状态',
          handle: function (scope, createElement) {
            return createElement('el-tag', {
              attrs: {
                type: options[scope.row.kpiGroupStatus].tag
              },
              domProps: {
                innerHTML: options[scope.row.kpiGroupStatus].statusName
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
            // this.previewHandle(row,'monthly_perf_reserveTalent');
          }
        }
      ],

      // 月度绩效考核数据
      tabActiveName:'my',
      approveDialogVisible:false,
      flowJson:{},
      flowType:'',

      ExpensesClaimFormVisible:false,
      operaType:'',
      flowId:'',
      flowInstanceId:'',
      flowNodeType:'',
      showFlowLog:'',
      parallelNodeChooseList:'',
      manualChooseNodes:'',
      formId:'',
      searchFlowType:'',
      flowNodeProxyId:'',
      noFormFlowInstanceId:'',
      flowProxyId:'',
      initiatorId:'',
      actionType:'',
      flowName:'',
      isExamine:false,
      isReInitiate:false,

      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      visible: false,
      flowList: [],
      tableData: [],
      year: '',
      status: '',
      colKey: {
        // title: {
        //   label: '名称',
        //   showTooltip:true,
        //   handle(scope, createElement) {
        //     return createElement('span', scope.row.title);
        //   }
        // },
        targetTime: {
          label: '考核时间',
          // ifFixed:true,
          handle: (scope, createElement) => {
            const targetTime = scope.row.targetTime + '';
            if (targetTime) {
              const year = targetTime.substr(0, 4);
              let month = targetTime.substr(4, 2);
              return createElement('span', year + '年' + this.quarterEnums[month] + '季度');
            } else {
              return '';
            }
          }
        },
        // createDate: {
        //   label: '发起日期',
        //   showTooltip:true,
        // },
        userName: {
          label: '姓名',
          // ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row.userName);
          }
        },
        companyName: {
          label: '公司',
          showTooltip: true,
          // width: '160px',
          // ifFixed:true,
        },
        depName: {
          label: '部门',
          showTooltip: true,
          // ifFixed:true,
        },
        dutyName: {
          label: '岗位',
          showTooltip: true,
          // ifFixed:true,
        },
        createDate: {
          label: '发起时间',
          showTooltip: true
        },
        a: {
          label: '绩效评级',
          showTooltip: true
        },
        b: {
          label: '分数',
          showTooltip: true
        },
        c: {
          label: '评级',
          showTooltip: true
        },
        // noticeStatus: {
        //   label: '公示状态',
        //   handle(scope, createElement) {
        //     // return createElement('span', publicity[scope.row.noticeStatus]);
        //     return createElement('el-tag', {
        //       attrs: {
        //         effect: "dark",
        //         type: publicityColor[scope.row.noticeStatus]
        //       },
        //       domProps: {
        //         innerHTML: publicity[scope.row.noticeStatus]
        //       }
        //     });
        //   }
        // },

        // rewardPonitsValue: {
        //   label: '奖励分值',
        //   handle(scope, createElement) {
        //     return createElement('span', scope.row?.rewardPonitsValue || '-');
        //   }
        // },
        // punishPonitsValue: {
        //   label: '扣罚分值',
        //   handle(scope, createElement) {
        //     return createElement('span', scope.row?.punishPonitsValue || '-');
        //   }
        // },
        // totalKpi: {
        //   label: '岗效比例',
        //   handle(scope, createElement) {
        //     var m = math.multiply(scope.row?.totalKpi || 0, 100);
        //     var text = parseInt(m) + '%'; // m.toFixed(2) + '%';
        //     return createElement('span', text);
        //   }
        // },
        // changeSalary: {
        //   label: '调薪情况',
        //   width: '200px',
        //   showTooltip:true,
        //   handle(scope, createElement) {
        //     let newVal = scope.row.kpiDynamicItemList.find(x=>x.kpiDynamicType == 'change_salary');
        //     return createElement('span', newVal?.desc || '无');
        //   }
        // },
        // comment: {
        //   label: '考核结果',
        //   width: '300px',
        //   showTooltip:true,
        //   handle(scope, createElement) {
        //     let newVal = scope.row.kpiDynamicItemList.find(x=>x.kpiDynamicType == 'ask_for_leave');
        //     var m = math.multiply(scope.row?.totalKpi || 0, 100);
        //     var text = m.toFixed(2) + '%';
        //     return createElement('span', newVal?.desc || `${text}岗效工资+基本工资+餐费20元*${scope.row.workDays || 0}`);
        //   }
        // },
        kpiGroupStatus: {
          label: '状态',
          handle: function (scope, createElement) {
            return createElement('el-tag', {
              attrs: {
                type: options[scope.row.kpiGroupStatus].tag
              },
              domProps: {
                innerHTML: options[scope.row.kpiGroupStatus].statusName
              }
            }
            );
          }
        },
        actions: (() => {
          var context = this;
          return {
            label: '操作',
            width: 80,
            handle(scope, createElement) {
              const row = scope.row;
              const buttons =
                {
                  // toFlow: {
                  //   buttonName: '发起流程',
                  //   // buttonAction: context.handleToFlow
                  //   buttonAction: context.handleEdit
                  // },
                  edit: {
                    buttonName: '编辑',
                    buttonAction: context.handleEdit
                  },
                  view: {
                    buttonName: '详情',
                    buttonAction: context.handleView
                  },
                  delete: {
                    buttonName: '删除',
                    buttonAction: context.handleDel
                  }
                };
              var ele = {};
              for (const key in buttons) {
                const createEle = createElement('el-button', {
                  attrs: {
                    type: 'text'
                  },
                  domProps: {
                    innerHTML: buttons[key].buttonName
                  },
                  on: {
                    click: () => { buttons[key].buttonAction(row,'kpi2_work_plan'); }
                  }
                });
                ele[key] = createEle;
              }
              return createElement('span', [ele.view]);
              // if (row.kpiGroupStatus == 'not_submitted') {
              //   return createElement('span', [ele.edit, ele.view, ele.delete]);
              // } else if (row.kpiGroupStatus == 'under_review') {
              //   return createElement('span', [ele.view]);
              // } else if (row.kpiGroupStatus == 'rejected') {
              //   return createElement('span', [ele.edit, ele.view]);
              // } else {
              //   return createElement('span', [ele.view,]);
              // }
            }
          };
        })(this)
      },
      options: [{
        value: '',
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
    // this.fetchData();
  },
  mounted() {
  },
  watch: {},
  computed: {},
  methods: {
    searchClick() {
      this.pagination.pages = 1;
      this.fetchData();
    },
    getFlowByTemplate() {
      const param = {
        data: {
          useScope: 'invest',
          auditWay: "kpi2_appraise"
        },
        showMe: true,
        ignoreFormTemplateBizRelevanceData: true,
        ignoreTemplateData: true,
        platformCode: '999999',
        pagination: true,
        pages: 1,
        size: 99
      };
      this.$axios.post(Api.schedule.getFlowTemplateList, param, (res) => {
        if (res.isSuccess) {
          if (!res.data || res.data.length == 0) {
            this.$message.error('暂无流程权限，请联系管理员');
            return;
          }
          if (res.data?.length > 1) {
            this.flowList = res.data;
            this.visible = true;
          } else {
            this.getFlowFindById(res.data[0].id);
          }
        }
      });
    },
    getFlowFindById(id) {
      this.$axios.post(Api.schedule.flowTemplateFindById, { data: { id }}, (res) => {
        if (res.isSuccess) {
          this.flowJson = res.data;
          this.approveDialogVisible = true;
        }
      });
    },
    // 查询轮岗月度绩效列表
    fetchReserveData() {
      const param = {
        data: {
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
            // this.tableData = this.generateTableData(tableData);
            this.reserveTableData = reserveTableData;
            this.reservePagination.total = res.total
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    exportListData() {
      this.fetchData();
      const param = {
        data: {
          manageType: 'work_and_manager_target',
          kpiScope: 'my_company',
          ...this.searchForm
        },
        pagination: false
      };
      this.$axios.post(
        '/web/plan/api/kpiGroup/export',
        param,
        res => {
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            link.download = '员工月度绩效.xlsx';
            link.click();
            document.body.removeChild(link);
            window.URL.revokeObjectURL(link.href);
          }
        }, '', { responseType: 'blob' }
      );
    },
    fetchData() {
      var typeEnum = { myQuarterlyScores: 'personal', companyQuarterlyScores: 'my_company' };
      var kpiType = this.$route.name;
      this.isPersonal = typeEnum[kpiType] === 'personal';
      const param = {
        data: {
          // ...this.searchForm
          // manageType: 'work_and_manager_target',
          kpiScope: typeEnum[kpiType],
          companyName: this.searchForm.companyName,
          depName: this.searchForm.depName
        },
        pagination: true,
        size: this.pagination.size,
        current: this.pagination.pages
      };
      this.$axios.post('/web/plan/api/kpi2Group/list', param,
        res => {
          if (res.isSuccess) {
            this.tableData = res.data || [];
            this.pagination.total = res.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    addTarget() {
      this.formType().then(res => {
        if (res.isSuccess) {
          const data = res.data;
          const arr = [];
          const find = data.filter(item => item.auditWay == 'monthly_perf');
          if (find && find.length) {
            const ids = find.map(item => item.id);
            this.getFlow(ids).then(resp => {
              if (resp.isSuccess) {
                const list = resp.data;
                if (list.length) {
                  // 获取
                  if (list.length > 1) {
                    this.flowList = list;
                    this.visible = true;
                  } else {
                    // const params = JSON.stringify(list[0]);
                    // console.log('params',params)
                    this.toFlowPage(list[0]);
                  }
                } else {
                  this.$message.error('暂无月度绩效流程,请先添加');
                }
              } else {
                this.$message.error('暂无月度绩效流程,请先添加');
              }
            });
          } else {
            this.$message.error('暂无月度绩效流程,请先添加');
          }
        } else {
          this.$message.error('暂无月度绩效流程,请先添加');
        }
      });
    },
    toFlowPage(params) {
      // this.$router.push({
      //   name: 'groupApproveManage',
      //   params: { from: 'monthly_perf', flow: params }
      // });
      this.flowType = 'monthly_perf'
      this.flowJson = params//this.currentRow
      this.approveDialogVisible = true
    },
    confirm(val) {
      if(val){
        const find = this.flowList.find(item => item.id == val);
        if (find) {
          this.visible = false
          // this.toFlowPage(find);
          this.getFlowFindById(find.id);
        }
      }else{
        this.$message.error('请选择流程')
      }
    },
    handleEdit(row) {
      //查询当前绑定的流程，给到重新发起，调用弹窗审核
      this.getInstanceId(row.id,'monthly_perf','edit').then(data=>{
        if(data){
          this.clickReInitiate(data)
        }else{
          this.$message.error('流程已删除，请删除本条月度绩效后重新发起月度流程')
        }
      })
      // this.$router.push({
      //   path: '/manpowerResource/performanceManage/targetView',
      //   query: {
      //     type: 'month',
      //     id: row.id,
      //     editType: '2',
      //     actionType: 'edit'
      //   }
      // });
    },
    // 去到流程那边
    handleToFlow(row) {
      const url = this.$router.resolve({
        path: '/groupApproveManage',
        query: { tab: 'dueout' }
      });
      const findObj = {
        bizId: row.id, type: 'monthly_perf'
      };
      localstorageSet('findObj', findObj);
      window.open(url.href, '_blank');
    },
    handleView(row, type) {
      this.$fm.show("flowDetail", {
        data: {
          // id: row.id, // 流程实例id
          flowInstanceBizRelevanceList: [{
            otherBiz: 'kpi2_appraise', // 业务类型
            otherBizId: row.id // 业务id
          }]
        }
      });
      return;
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
      // this.$router.push({
      //   path: '/manpowerResource/performanceManage/targetView',
      //   query: {
      //     type: 'month',
      //     id: row.id
      //   }
      // });
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
    // handleDel(row) {
    //   this.$confirm('确认要删除吗?', '提示', {
    //     closeOnClickModal: false,
    //     confirmButtonText: '确定',
    //     cancelButtonText: '取消',
    //     type: 'warning'
    //   }).then(() => {
    //     this.$axios.post(
    //       Api.performance.deleteWorkTarget,
    //       { data: { id: row.id }},
    //       res => {
    //         if (res.isSuccess) {
    //           this.$message.success('操作成功');
    //           this.fetchData();
    //         // this.tableData = res.data ? res.data : [];
    //         } else {
    //           this.$message.success('操作成功');
    //           this.fetchData();
    //         // this.$message.error('删除失败');
    //         }
    //       });
    //     // 删除list
    //   }).catch(() => { });
    // },
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
