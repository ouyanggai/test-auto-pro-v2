<!--
 * @description:业绩指标
 * @Author: Calvin
 * @Date: 2022-03-07 15:53:59
 * @FilePath: \src\views\PerformanceManage\TargetBook\components\ManageTarget\index.vue
-->
<template>
  <div>
    <div>
      <el-button @click="addTarget" type="primary" icon="el-icon-circle-plus-outline" v-if="!kpiScope">发起流程
      </el-button>
      <span class="title" style="margin-left:30px;" v-if="!kpiScope">选择日期：</span>
        <el-date-picker
          style="width:180px;"
          v-model="year"
          type="year"
          format="yyyy"
          value-format="yyyy"
          @change="fetchData"
          placeholder="请选择考核年限">
      </el-date-picker>
      <template v-if="kpiScope">
        <el-input clearable placeholder="按公司查询" v-model="companyName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-input clearable placeholder="按部门查询" v-model="depName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-input clearable placeholder="按姓名查询" v-model="userName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-select v-model="status" placeholder="请选择状态" @change="fetchData" style="margin-left:10px;">
          <el-option
            v-for="item in flowOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
        <el-button type="primary" @click="fetchData" style="margin-left:10px;">查询</el-button>
      </template>
    </div>
    <dy-table :keys="colKey" :fetchData="fetchData" :list="tableData" style="padding:0px;" ref="usingTable" :isPagination="false">
    </dy-table>
    <el-pagination
      v-if="pagination.total"
      background
      ref="paginationRef"
      layout="total, sizes, prev, pager, next"
      style="text-align: right;margin-top: 15px;"
      :page-size="pagination.size"
      @current-change="pageChange"
      @size-change="sizeChange"
      :total="pagination.total">
    </el-pagination>
    <chooseFlowDialog :visible.sync="visible" @confirm="confirm" :flowList="flowList"></chooseFlowDialog>
    <!-- 再次发起流程/查看 -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="isReInitiate" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
      :initiatorId="initiatorId" @success="fetchData" :actionType="actionType" :flowType="flowType" :flowName="flowName"/>
    <!-- 公司流程发起的弹窗 -->
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="[]" v-if="approveDialogVisible" @success="fetchData"
      :flowJson.sync="flowJson" :flowType.sync="performFlowType" :closeAll="true" />
  </div>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
import mixin from '../mixin';
import chooseFlowDialog from '../chooseFlowDialog.vue';
import { localstorageSet } from '@/utils/auth.js';
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
import FlowDialog from '@/views/GroupApproveManage/Submitted/components/FlowDialog.vue'
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
export default {
  name: '',
  components: {
    DyTable,
    chooseFlowDialog,
    ExamineDialog,
    FlowDialog
  },
  props: {
    kpiScope: {
      type: [String, Number, undefined],
      default: undefined
    }
  },
  mixins: [mixin],
  data() {
    const context = this;
    return {
      approveDialogVisible:false,
      flowJson:{},
      performFlowType:'manageTarget',

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
      flowType:'',
      flowName:'',
      isReInitiate:true,
      wholeData:[],
      tableData: [],
      // 人员信息
      colKey: {
        ...(_ => {
          return this.kpiScope ? {
            companyName: '公司',
            depName: '部门',
            userName: '姓名'
          } : {};
        })(),
        title: '名称',
        targetTimeType: '考核年限',
        createDate: '发起时间',
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
        kpiGroup: {
          label: '年终/半年考核',
          handle: (scope, createElement) => {
            if (scope.row.yearKpiScoreGroupList && scope.row.yearKpiScoreGroupList.length) {
              var halfBtn = createElement('el-button', { attrs: { type: 'text', disabled: this.halfOrYearDisable(scope, 'half_year')},
                domProps: { innerHTML: '半年详情' },
                on: {
                  click: () => {
                    context.handleView(scope.row,'year_kpi_work_target', 'half_year')
                  }
                }
              });
              var yearBtn = createElement('el-button', { attrs: { type: 'text', disabled: this.halfOrYearDisable(scope, 'year')},
                domProps: { innerHTML: '年终详情' },
                on: {
                  click: () => {
                    context.handleView(scope.row,'year_kpi_work_target', 'year')
                  }
                }
              });
              return createElement('span', [halfBtn, yearBtn])
            } else {
              return <span>-</span>;
            }

          }
        },
        actions: (() => {
          var context = this;
          return {
            label: '操作',
            width: 240,
            handle(scope, createElement) {
              const row = scope.row;
              const buttons =
                {
                  // toFlow:{
                  //   buttonName:'发起流程',
                  //   buttonAction:context.handleToFlow
                  // },
                  edit: {
                    buttonName: '编辑',
                    buttonAction: context.handleEdit,
                    type:''
                  },
                  view: {
                    buttonName: '查看详情',
                    buttonAction: context.handleView,
                    type:''
                  },
                  delete: {
                    buttonName: '删除',
                    buttonAction: context.handleDel,
                    type:''
                  },
                  exameHalfYear: {
                    buttonName: '发起半年考核',
                    buttonAction: context.handlYearExame,
                    type:'year_kpi_work_target',
                    assessmentCycle:'half_year'
                  },
                  exameYear: {
                    buttonName: '发起年终考核',
                    buttonAction: context.handlYearExame,
                    type:'year_kpi_work_target',
                    assessmentCycle:'year'
                  }
                  // exame: {
                  //   buttonName: '发起考核',
                  //   buttonAction: context.handlExame
                  // }
                };
              var ele = {};
              for (const key in buttons) {
                const createEle = createElement('el-button', {
                  attrs: {
                    type: 'text',
                    disabled:buttons[key]['assessmentCycle']?context.isDisable(row,buttons[key]['assessmentCycle']):false
                  },
                  domProps: {
                    innerHTML: buttons[key].buttonName
                  },
                  on: {
                    click: () => {
                      buttons[key].buttonAction(row,buttons[key]['type']||'annual_perf',buttons[key]['assessmentCycle']);
                    }
                  }
                });
                ele[key] = createEle;
              }
              if (context.kpiScope) {
                return createElement('span', [ele.view]);
              }
              if (row.kpiGroupStatus == 'not_submitted') {
                return createElement('span', [ele.edit, ele.view,ele.delete]);
              } else if (row.kpiGroupStatus == 'under_review') {
                return createElement('span', [ele.view]);
              } else if (row.kpiGroupStatus == 'rejected') {
                return createElement('span', [ele.edit, ele.view]);
              } else {
                if(context.isShow(row)){
                  return createElement('span', [ele.view, ele.exameHalfYear,ele.exameYear]);
                }else{
                  return createElement('span', [ele.view,ele.exameYear]);
                }

              }
            }
          };
        })(this)
      },
      flowList: [],
      visible: false,
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      companyName: '',
      depName: '',
      userName: '',
      status: '',
      flowOptions: [{
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
        label: '已通过'
      }],
      year: '', // new Date().getFullYear().toString(),
      exameBizId: '',
      isExamine: false,
      assessmentCycle:'',
      assessmentCycleType:''
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() { },
  updated() {
  },
  // activated(){
  //   this.fetchData();
  // },
  methods: {
    halfOrYearDisable({ row }, type) {
      if (row.yearKpiScoreGroupList && row.yearKpiScoreGroupList.length) {
        return !(row.yearKpiScoreGroupList.some(i => i.assessmentCycle == type));
      }
      return true;
    },
    isShow(row){
      if(row.assessmentCycle=='year_and_half_year'){
        return true
      }else{
        return false
      }
    },
    isDisable(row,type) {
      if(row.canSubmitScore){
        if(row.assessmentCycle=='year_and_half_year'){
        if(type=='half_year'){
          if(row.yearKpiScoreGroupList){
            return true
          }else{
            return false;
          }
        }else{
          if(row.yearKpiScoreGroupList&&row.yearKpiScoreGroupList.length==1){
            return false;
          }else{
            return true
          }
        }
      }else{
        if(type=='half_year'){
          return true
        }else{
          if(row.yearKpiScoreGroupList){
            return true;
          }else{
            return false
          }
        }
      }
      }else{
        return true
      }
    },
    handlYearExame(row,type,assessmentCycle){
      this.handlExame(row,assessmentCycle)
    },
    fetchData() {
      const param = {
        data: {
          companyName: this.companyName || undefined,
          depName: this.depName || undefined,
          userName: this.userName || undefined,
          targetTime: this.year || undefined,
          kpiGroupStatus: this.status || undefined,
          manageType: 'manager_target',
          kpiScope: this.kpiScope
        }
      };
      if (this.year)param.data.targetTime = this.year;
      if (this.status)param.data.kpiGroupStatus = this.status;

      this.$axios.post(
        Api.performance.getKpiGroupList,
        param,
        res => {
          if (res.isSuccess) {
            const tableData = res.data ? res.data : [];
            tableData.forEach(item => {
              const assessmentCycle = item.assessmentCycle == 'year' ? '年终' : '半年及年终';
              item.targetTimeType = item.targetTime + assessmentCycle;
            });
            tableData.sort((a, b) => {
              const aTime = new Date(a.createDate).getTime();
              const bTime = new Date(b.createDate).getTime();
              return bTime - aTime;
            });
            this.wholeData = tableData
            this.$refs.paginationRef?.handleCurrentChange(1);
            this.tableData = this.generateTableData(tableData);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    confirm(val) {
      const find = this.flowList.find(item => item.id == val);
      if (find) {
        if (this.isExamine) {
          this.$router.push({
            name: 'groupApproveManage',
            params: { from: 'annual_perf', flow: JSON.stringify(find) },
            query: { flowType: 'manageTarget', id: this.exameBizId, isExamine: true,assessmentCycle:this.assessmentCycle,assessmentCycleType:this.assessmentCycleType }
          });
        } else {
          this.toFlowPage(find);
        }
      }
    },
    toFlowPage(params) {
      this.flowType = 'annual_perf'
      this.flowJson = params//this.currentRow
      this.approveDialogVisible = true
      // this.$router.push({
      //   name: 'groupApproveManage',
      //   params: { from: 'annual_perf', flow: params },
      //   query: { flowType: 'manageTarget' }
      // });
    },
    handleEdit(row) {
      //查询当前绑定的流程，给到重新发起，调用弹窗审核
      this.getInstanceId(row.id,'annual_perf','edit').then(data=>{
        if(data){
          this.clickReInitiate(data)
        }else{
          this.$message.error('流程已删除，请删除本条目标责任书后重新发起')
        }
      })
      // this.$router.push({
      //   path: '/manpowerResource/performanceManage/manageTarget',
      //   query: {
      //     editType: 2,
      //     id: row.id
      //   }
      // });
    },
    handleToFlow(row) {
      const url = this.$router.resolve({
        path: '/groupApproveManage',
        query: { tab: 'dueout' }
      });
      const findObj = {
        bizId: row.id, type: 'annual_perf'
      };
      localstorageSet('findObj', findObj);
      window.open(url.href, '_blank');
    },
    handleView(row,type, btnType) {
       //查询当前绑定的流程，给到重新发起，调用弹窗审核
       var findRow = row.yearKpiScoreGroupList?.find(item => item.assessmentCycle == btnType);
       this.getInstanceId(type=='year_kpi_work_target'?findRow.id:row.id,type,'view').then(data=>{
        if(data){
          this.previewHandle(data)
        }else{
          this.$message.error('流程已删除，请删除本条目标责任书后重新发起')
        }
      })
    },
  }
};
</script>

<style scoped lang="scss">
.dytable-view-container{
  min-height: initial !important;
}
::v-deep .dytable-view-paging{
  display: none !important;
}
</style>
