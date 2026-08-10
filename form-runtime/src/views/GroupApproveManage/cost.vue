<template>
  <div class="content-container">
    <div style="display: flex;height: 100%;">
      <div class="left-side">
        <el-tabs tab-position="left" @tab-click="tabClick" v-model="tab">
          <el-tab-pane v-for="(item, index) in flowName" :key="index" :label="item.label"
            :name="item.value"></el-tab-pane>
        </el-tabs>
      </div>
      <div class="right-aside">
        <div class="right-aside-inner">
          <div class="search" style="padding: 3px 5px;">
            <el-select v-model="search.flowName" placeholder="选择表单" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" v-if="tab == 'all'">
              <el-option v-for="(item, index) in flowNameoptions" :key="index" :label="item" :value="item">
              </el-option>
            </el-select>
            <el-select v-model="search.company" placeholder="选择公司" style="width:180px;margin-right:3px"
              @change="fetchData" v-if="tab != 'annual_perf' && tab != 'monthly_perf'">
              <el-option v-for="(item, index) in companyOptions" :key="index" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
            <el-select v-model="search.depart" placeholder="选择部门" style="width:100px;margin-right:3px" clearable
              @change="getTaskList('depart')">
              <el-option v-for="(item, index) in departOptions" :key="index" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
            <el-select v-model="search.handler" placeholder="经办人" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" filterable
              v-if="tab != 'contract_review' && tab != 'expense_budget' && tab != 'travel_expense' && tab != 'annual_perf' && tab != 'monthly_perf' && tab != 'contract_pay_request' && tab != 'invoice_apply'">
              <el-option v-for="(item, index) in handlerOptions" :key="index" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
            <el-select v-model="search.initiator" placeholder="发起人" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" v-if="tab == 'annual_perf' || tab == 'monthly_perf'">
              <el-option v-for="(item, index) in initiatorOptions" :key="index" :label="item" :value="item">
              </el-option>
            </el-select>
            <el-select v-model="search.project" placeholder="选择项目" style="width:100px;margin-right:3px" clearable
              @change="getTaskList"
              v-if="tab != 'expense_budget' && tab != 'annual_perf' && tab != 'monthly_perf' && tab != 'travel_expense'"
              popper-class="my-select" :popper-append-to-body="false">
              <el-option v-for="(item, index) in projectOptions" :key="index" :label="item" :value="item">
              </el-option>
            </el-select>
            <el-select v-model="search.contract_name" placeholder="选择合同" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" popper-class="my-select" :popper-append-to-body="false"
              v-if="tab != 'invoice_apply' && tab != 'expense_budget' && tab != 'request_funds' && tab != 'annual_perf' && tab != 'monthly_perf' && tab != 'refund_bid' && tab != 'travel_expense'">
              <el-option v-for="(item, index) in contrctOptions" :key="index" :label="item" :value="item">
              </el-option>
            </el-select>
            <el-select v-model="search.flowStatus" placeholder="选择状态" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" v-if="tab != 'annual_perf' && tab != 'monthly_perf'">
              <el-option v-for="(item, index) in statusOptions" :key="item.value" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
            <el-select v-model="search.flowStatus" placeholder="选择状态" style="width:100px;margin-right:3px" clearable
              @change="getTaskList" v-if="tab == 'annual_perf' || tab == 'monthly_perf'">
              <el-option v-for="(item, index) in kpiStatusOptions" :key="item.value" :label="item.label"
                :value="item.value">
              </el-option>
            </el-select>
            <template v-if="tab != 'annual_perf' && tab != 'monthly_perf'">
              <div style="margin-right:3px">
                <RangeInput ref="RangeInput" placeholderMin="最低金额" placeholderMax="最高金额" v-model="search.range"
                  @input="getTaskList"></RangeInput>
              </div>
              <el-date-picker v-model="search.dateVal" @change="handleDateChange" type="daterange" range-separator="至"
                start-placeholder="开始日期" end-placeholder="结束日期" size="mini" value-format="yyyy-MM-dd" format="yyyy-MM-dd"
                style="width:220px;">
              </el-date-picker>
            </template>
          </div>
          <div v-if="tab != 'annual_perf' && tab != 'monthly_perf'">
            <dy-table :fetchData="() => { }" :actions="actions" :keys="colKey" :list='tableData' :isPagination="false"
              :pagination="pagination" :height="height" :showSummary="true" :summaryMethod="getSummaries"></dy-table>
            <div style="display: flex;flex-direction: row-reverse;align-items: center;">
              <el-pagination v-if="pagination.total" background layout="total, sizes, prev, pager, next"
                style="text-align: right;display: inline-block;" :page-size="pagination.size" @current-change="pageChange"
                @size-change="sizeChange" :total="pagination.total">
              </el-pagination>
              <!-- <span style="margin-right: 20px;" v-if="tab == 'all'">总计：{{ (expTotal + totalCount) }}</span> -->
              <span style="margin-right: 20px;" v-if="tab == 'all'">总计：{{ total }}</span>
            </div>
          </div>
          <div v-else>
            <dy-table :fetchData="getPerformanceList" :actions="actions" :keys="colKey" :list='tableData'
              :isPagination="false" :pagination="pagination" :height="height" ref='table'></dy-table>
            <div style="display: flex;flex-direction: row-reverse;align-items: center;">
              <el-pagination v-if="pagination.total" background layout="total, sizes, prev, pager, next"
                style="text-align: right;display: inline-block;" :page-size="pagination.size" @current-change="pageChange"
                @size-change="sizeChange" :total="pagination.total" :current-page="pagination.pages">
              </el-pagination>
            </div>
          </div>
        </div>
      </div>
    </div>
    <!-- 查看弹窗(对固定页面的查看) -->
    <ExamineDialog :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine" :operaType="operaType"
      :searchFlowType="searchFlowType" :flowId="flowId" :flowInstanceId="flowInstanceId" :actionType="actionType"
      :flowType="flowType" v-if="ExpensesClaimFormVisible" />

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :flowId="flowId"
      :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
      :visible.sync="examineDialogVisible" />
  </div>
</template>
<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import DyTable from '@/components/DyTable';
import { approveManageFlowStatus, deepClone } from '@/utils';
import mixin from '@/views/ApproveManage/components/mixin'
import ExamineDialog from './components/ExamineDialog';
import EnterpriseExamineDialog from './components/EnterpriseExamineDialog';
import RangeInput from '@/components/RangeInput'
const searchOrigin = {
  flowName: '',
  company: localstorageGet('companyId'),
  depart: '',
  handler: '',
  project: '',
  contract_name: '',
  flowStatus: '',
  range: { min: '', max: '' },
  initiator: '',
  dateVal: []
}
const kpiStatusOptions = [{
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
export default {
  name: 'Cost',
  components: { DyTable, ExamineDialog, EnterpriseExamineDialog, RangeInput },
  props: [],
  mixins: [mixin],
  data() {
    let defaultTab = 'all'
    return {
      flowName: [
        { label: '全部费用表单', value: 'all' },
        { label: '差旅报销', value: 'travel_expense' },
        { label: '合同评审', value: 'contract_review' },
        { label: '合同付款申请', value: 'contract_pay_request' },
        { label: '开票申请', value: 'invoice_apply' },
        { label: '请款单', value: 'request_funds' },
        { label: '投标保证金退还请款', value: 'refund_bid' },
        { label: '费用报销', value: 'expense_budget' },
        { label: '年度目标责任书', value: 'annual_perf' },
        { label: '月度绩效', value: 'monthly_perf' },
      ],
      tab: defaultTab,
      auditWayList: [defaultTab],

      tableData: [],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      wholeData: [],
      originData: [],
      height: null,
      actions: [
        {
          width: 120,
          label: '查看',
          action: row => {
            this.previewHandle(row, false);
          }
        }
      ],
      isExamine: false,
      flowId: '',
      formId: '',
      flowNodeProxyId: '',
      flowInstanceId: '',
      jobTaskId: '',
      examineDialogVisible: false,
      operaType: '',
      actionType: '',
      flowType: '',
      searchFlowType: '',
      ExpensesClaimFormVisible: false,
      btnVisible: false,
      totalCount: 0,
      expTotal: 0,

      search: deepClone(searchOrigin),
      flowNameoptions: [],
      companyOptions: [],
      departOptions: [],
      contrctOptions: [],
      handlerOptions: [],
      initiatorOptions: [],
      statusOptions: [
        { label: '完结', value: 'end' },
        { label: '审批中', value: 'run' },
        { label: '撤回', value: 'withdraw' },
        { label: '驳回', value: 'rejected' },
        { label: '草稿', value: 'draft' }
      ],
      kpiStatusOptions,
      projectOptions: [],
      startDate: '',
      endDate: '',
      total: 0,


    };
  },
  async created() {
    // this.getFlowList()
    this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {}).then(res => {
      if (res.isSuccess) {
        let list = res.data
        this.companyOptions = list.map(item => {
          return {
            label: item.name,
            value: item.id
          }
        })
        this.getCompanyDepartTree()
      }
    })

  },
  mounted() {
    this.$nextTick(() => {
      let clientHeight = document.querySelector('.content-container').clientHeight
      this.height = clientHeight - 210
    })
  },
  computed: {
    colKey() {
      let colKey = {
        formName: {
          label: "表单类型",
          showTooltip:true,
          width:200
        },
        company:{
          label:'公司',
          showTooltip:true,
          width:150
        },
        depart: {
          label: '部门',
          showTooltip:true
        },
        initiator: {
          label: '发起人',
          showTooltip:true
        },
        project_name: {
          label:'项目名称',
          showTooltip:true
        },
        contract_name: {
          label:'合同名称',
          showTooltip:true
        },
        contract_price: {
          label:'金额',
          showTooltip:true
        },
        handler: '经办人',
        status: {
          label: '状态',
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.status));
          }
        },
        createDate: {
          label:'时间',
          showTooltip:true
        },
      }
      if (this.tab == 'annual_perf') {
        colKey = {
          targetTime:{
            label:'年度',
            width:100
          },
          company: {
            label:'公司',
            width:150,
            showTooltip:true
          },
          depart: {
            label: '部门'
          },
          initiator: {
            label: '姓名'
          },
          roleName: '岗位',
          totalScore: '得分',
          kpiGroupStatus: {
            label: '状态',
            handle: function (scope, createElement) {
              let kpiGroupStatus = scope.row.kpiGroupStatus
              let find = kpiStatusOptions.find(item => item.value == kpiGroupStatus)
              let statusName = find?.label || ''
              return createElement('span', {
                // attrs:{
                //   type:options[scope.row.kpiGroupStatus].tag
                // },
                domProps: {
                  innerHTML: statusName//kpiStatusOptions[scope.row.kpiGroupStatus].statusName
                }
              }
              );
            }
          }
        }
      } else if (this.tab == 'monthly_perf') {
        colKey = {
          title:{
            label:'考核季度日期',
            width:200,
            showTooltip:true
          },
          company: {
            label:'公司',
            width:150,
            showTooltip:true
          },
          depart: {
            label: '部门'
          },
          initiator: {
            label: '姓名'
          },
          totalScore: '最终得分',
          rewardPonitsValue: '奖励得分',
          punishPonitsValue: '扣罚得分',
          kpiGroupStatus: {
            label: '状态',
            handle: function (scope, createElement) {
              let kpiGroupStatus = scope.row.kpiGroupStatus
              let find = kpiStatusOptions.find(item => item.value == kpiGroupStatus)
              let statusName = find?.label || ''
              return createElement('span', {
                // attrs:{
                //   type:options[scope.row.kpiGroupStatus].tag
                // },
                domProps: {
                  innerHTML: statusName//kpiStatusOptions[scope.row.kpiGroupStatus].statusName
                }
              }
              );
            }
          },
          // rate:'岗效比例',
        }
      } else {
        if (this.tab != 'all') {
          delete colKey['formName']
          if (this.tab == 'expense_budget') {
            delete colKey['project_name']
            delete colKey['contract_name']
            delete colKey['handler']
          } else if (this.tab == 'travel_expense') {
            delete colKey['formName']
            delete colKey['project_name']
            delete colKey['contract_name']
            delete colKey['handler']
          } else if (this.tab == 'contract_pay_request') {
            delete colKey['handler']
          } else if (this.tab == 'refund_bid') {
            delete colKey['contract_name']
          } else if (this.tab == 'request_funds') {
            delete colKey['contract_name']
          } else if (this.tab == 'contract_review') {
            delete colKey['handler']
          } else if (this.tab == 'invoice_apply') {
            delete colKey['handler']
            delete colKey['contract_name']
          }
        }
      }
      return colKey
    }
  },
  methods: {
    getTaskListByPost(){

    },
    isForm() {
      if (this.selectFlowType == 'enterprise') {
        return true
      } else {
        if (['travel_expense', 'contract_review', 'contract_pay_request', 'request_funds', 'refund_bid', 'invoice_apply'].indexOf(this.selectFlowType) > -1) {
          return true
        } else {
          return false
        }
      }
    },
    getSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (index === 0) {
          sums[index] = '合计';
          return;
        }
        const values = data.map(item => Number(item[column.property]));
        if (
          column.property === "contract_price"
        ) {
          //官网是不为空的条件，我改成我想要的列，
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return prev + curr;
            } else {
              return prev;
            }
          }, 0);
          sums[index];
        }
      });
      return sums;

    },
    getCompanyDepartTree() { // 获取公司部门架构数据
      this.$axios.post(
        Api.frameworkManageInfo.getCompanyDepartTree,
        {
          data: {
            flag: 3,
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          this.frameTree = res.data
          this.findDepart = this.frameTree[0].childrenList.find(item => item.id == this.search.company)
          if (this.findDepart) {
            this.departOptions = this.findDepart.childrenList.map(item => {
              return {
                label: item.name,
                value: item.id
              }
            })
          }
          this.tabClick()
        }
      );
    },
    tabClick() {
      this.pagination.pages = 1
      this.search = deepClone(searchOrigin)
      if (this.$refs.RangeInput) this.$refs.RangeInput.clear()
      if (this.tab == 'all') {
        let auditWayList = []
        this.flowName.forEach(item => {
          if (item.value != 'monthly_perf' && item.value != 'annual_perf') {
            auditWayList.push(item.value)
          }
        })
        this.auditWayList = auditWayList
      } else {
        this.auditWayList = [this.tab]
      }
      this.fetchData()
    },
    //获取目标责任书 月度绩效
    getPerformanceList() {
      let manageType = this.tab == 'monthly_perf' ? 'work_and_manager_target' : 'work_target'
      const param = {
        data: {
          manageType,//: '',
          kpiScope: 'my_company'
        },
      };
      if (this.year) param.data.targetTime = this.year
      if (this.status) param.data.kpiGroupStatus = this.status
      this.$axios.post(
        Api.performance.getKpiGroupList,
        param,
        res => {
          if (res.isSuccess) {
            let arr = res.data ? res.data : [];
            arr.sort((a, b) => {
              let aTime = new Date(a.createDate).getTime()
              let bTime = new Date(b.createDate).getTime()
              return bTime - aTime
            })
            let departOptions = [], companyOptions = [], initiatorOptions = []
            arr.forEach(item => {
              let createrId = item.createrId
              let obj = this.findNode(this.frameTree, createrId)
              if (obj) {
                item.depart = obj.parent.name
                item.departId = obj.parent.id
                item.company = obj.grandparent.name
                item.companyId = obj.grandparent.id
                item.roleName = obj.node.roleName
                item.initiator = obj.node.name
              }
              // if (item.depart && departOptions.indexOf(item.depart) <= -1) departOptions.push(item.depart)
              if (item.initiator && initiatorOptions.indexOf(item.initiator) <= -1) initiatorOptions.push(item.initiator)
              // if (item.company && companyOptions.indexOf(item.company) <= -1) companyOptions.push(item.company)
            })
            // this.departOptions = departOptions
            // this.companyOptions = companyOptions
            this.initiatorOptions = initiatorOptions

            this.pagination.total = arr.length
            this.originData = deepClone(arr)
            this.wholeData = arr
            let tableData = this.generateTableData(this.wholeData)
            this.tableData = tableData
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取表单详情数据
    getEchDetailData(id, item) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.budgetManage.getEchDetailData,
          {
            data: {
              id
            }
          },
          res => {
            let data = res.data
            let { expenseBudgetList } = data
            let contract_price = 0
            expenseBudgetList.forEach(item => {
              contract_price += item.money || 0
            })
            item.contract_price = contract_price.toFixed(2)
            this.expTotal += (item.contract_price - 0)
            resolve()
          }
        )
      })
    },
    fetchData(taskStatus) {
      if (this.tab == 'annual_perf' || this.tab == 'monthly_perf') {
        this.getPerformanceList()
        return
      }
      this.expTotal = 0
      if (!taskStatus) taskStatus = ''
      let data = {
        data: {
          // initiator: "all",
          initiator: 'all',
          useScope: "invest",
          auditWayList: this.auditWayList,
          taskStatus,
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'company',
              otherBizId: this.search.company//localstorageGet('companyId')
            }
          ]
        },
        fcqCellList: [ //提取的字段列
          "project_name", //表单模板定义的列
          "contract_name",
          "depart",
          "contract_price",
          "company",
          "handler"
        ],
        pagination: true,
        pages: 1,
        size: 9999

      }
      this.$axios.post(Api.schedule.getFlowInstanceList, data, res => {
        if (res.isSuccess) {
          let data = res.data
          let arr = [], totalCount = 0, flowNameoptions = [], departOptions = [], handlerOptions = [],
            companyOptions = [], projectOptions = [], contrctOptions = []
          data.forEach(item => {

            // frameTree
            let formCellData = item.formCellData || ''
            if (formCellData) {
              for (let key in formCellData) {
                item[key] = formCellData[key]
              }
            }
            let flowInstanceBizRelevanceList = item.flowInstanceBizRelevanceList
            if (!flowInstanceBizRelevanceList) {
              arr.push(item)
            } else {
              let index = flowInstanceBizRelevanceList.findIndex(el => {
                return el.otherBiz == 'project'
              })
              if (index == -1) {
                arr.push(item)
              }
            }
            let createId = item.createrId
            let obj = this.findNode(this.frameTree, createId)
            if (obj) {
              item.depart = obj.parent.name
              item.departId = obj.parent.id
              item.company = obj.grandparent.name
              item.companyId = obj.grandparent.id
              item.roleName = obj.node.roleName
            }
            if (item.contract_price) {
              let price = 0
              if (!isNaN(item.contract_price)) price = Number(item.contract_price)
              totalCount += price
            }

            //生成下拉选项
            if (item.formName && flowNameoptions.indexOf(item.formName) <= -1) flowNameoptions.push(item.formName)
            // if (item.depart && departOptions.indexOf(item.depart) <= -1) departOptions.push(item.depart)
            // if (item.handler && handlerOptions.indexOf(item.handler) <= -1) handlerOptions.push(item.handler)
            // if (item.company && companyOptions.indexOf(item.company) <= -1) companyOptions.push(item.company)
            if (item.project_name && projectOptions.indexOf(item.project_name) <= -1) projectOptions.push(item.project_name)
            if (item.contract_name && contrctOptions.indexOf(item.contract_name) <= -1) contrctOptions.push(item.contract_name)
          })
          this.flowNameoptions = flowNameoptions
          // this.departOptions = departOptions
          // this.handlerOptions = handlerOptions
          // this.companyOptions = companyOptions
          this.projectOptions = projectOptions
          this.contrctOptions = contrctOptions

          this.totalCount = totalCount
          this.pagination.total = arr.length//res.total;
          this.originData = deepClone(arr)
          this.wholeData = arr
          let tableData = this.generateTableData(this.wholeData)
          if (this.tab == 'all' || this.tab == 'expense_budget') {
            this.fillData(this.wholeData)
          } else if (this.tab == 'annual_perf') {
            this.fillAnnal(this.wholeData)
          } else if (this.tab == 'monthly_perf') {
            this.fillMonth(this.wholeData)
          } else {
            this.tableData = tableData
            this.queryData()
          }
        }
      })
    },
    findNode(nodeArray, id, parent = null, grandparent = null) {
      for (let i = 0; i < nodeArray.length; i++) {
        if (nodeArray[i].id === id) {
          return { node: nodeArray[i], parent, grandparent };
        } else if (nodeArray[i].childrenList) {
          let result = this.findNode(nodeArray[i].childrenList, id, nodeArray[i], parent);
          if (result) return result;
        }
      }
      return null;
    },
    getAnnualData(id, item) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.performance.getWorkTargetDetail, // 放开
          {
            data: {
              id,
            }
          },
          res => {
            const data = res.data;
            if (data) {
              item.annual = data.targetTime
              let score = 0
              data.keyPerformanceIndicatorsList.forEach(item => {
                score += (item.score - 0)
              })
              item.score = score
            }
            resolve()
          }
        );
      })
    },
    getMonth(id, item) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.performance.getWorkTargetDetail, // 放开
          {
            data: {
              id//: this.editId,
            }
          },
          res => {
            // ((自评分*评分比例+领导评分*评分比例)*权重*100)/3 -->
            const data = res.data
            // item.annual = data.targetTime
            if (data?.title) item.title = data.title
            let score = 0
            if (data?.keyPerformanceIndicatorsList) data.keyPerformanceIndicatorsList.forEach(el => {
              score += ((el.pretendScore * 0.3 + el.score * 0.7) * el.weight * 100) / 3
              //score += (el.score-0)
            })
            item.score = Math.round(score)

            resolve()
          }
        );
      })
    },
    fillMonth(wholeData) {
      let promiseArr = []
      wholeData.forEach(item => {
        if (find) {
          let flowInstanceBizRelevanceList = item.flowInstanceBizRelevanceList
          let find = flowInstanceBizRelevanceList.find(el => el.otherBiz == 'monthly_perf')
          let id = find.otherBizId
          promiseArr.push(this.getMonth(id, item))
        }
      })
      if (promiseArr.length) {
        Promise.all(promiseArr).then(res => {
          this.originData = deepClone(wholeData)
          this.tableData = this.generateTableData(wholeData)
        }).catch(err => {
        })
      } else {
        this.originData = deepClone(wholeData)
      }
    },
    fillAnnal(wholeData) {
      let promiseArr = []
      wholeData.forEach(item => {
        if (find) {
          let flowInstanceBizRelevanceList = item.flowInstanceBizRelevanceList
          let find = flowInstanceBizRelevanceList.find(el => el.otherBiz == 'annual_perf')
          let id = find.otherBizId
          promiseArr.push(this.getAnnualData(id, item))
        }
      })
      if (promiseArr.length) {
        Promise.all(promiseArr).then(res => {
          this.originData = deepClone(wholeData)
          this.tableData = this.generateTableData(wholeData)
        })
      } else {
        this.originData = deepClone(wholeData)
      }
    },
    fillData(wholeData) {
      let promiseArr = []
      wholeData.forEach(item => {
        let flowInstanceBizRelevanceList = item.flowInstanceBizRelevanceList
        let find = flowInstanceBizRelevanceList.find(el => el.otherBiz == 'expense_budget')
        if (find) {
          let id = find.otherBizId
          promiseArr.push(this.getEchDetailData(id, item))
        }
      })
      if (promiseArr.length) {
        Promise.all(promiseArr).then(res => {
          this.originData = deepClone(wholeData)
          this.tableData = this.generateTableData(wholeData)
          this.queryData()
        })
      } else {
        this.originData = deepClone(wholeData)
        this.tableData = this.generateTableData(wholeData)
        this.queryData()
      }
    },
    previewHandle(row, type) {
      if (this.tab == 'annual_perf' || this.tab == 'monthly_perf') {
        let queryType = this.tab == 'annual_perf' ? 'workTarget' : 'month'
        let url = this.$router.resolve({
          path: '/manpowerResource/performanceManage/targetView',
          query: {
            type: queryType,
            id: row.id,
            frompage: 'cost'
          }
        })
        window.open(url.href, '_blank')
      } else {
        this.selectFlowType = row.auditWay
        if (this.isForm()) { // formMaking表单
          this.isExamine = false;
          this.flowId = row.flowProxyId;
          this.formId = row.formProxyId;
          this.flowNodeProxyId = row.flowNodeProxyId;
          this.flowInstanceId = row.id;
          this.jobTaskId = row.jobTaskId;
          this.examineDialogVisible = true;
        } else { // 固定页面
          this.operaType = 'check';
          this.actionType = 'preview' //非费用预算流程
          if (row.auditWay == 'annual_perf') {
            let find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget')
            if (find) { //管理指标
              this.flowType = 'manageTarget'
            } else { //工作指标
              this.flowType = 'workTarget'
            }
          }
          this.isExamine = false;
          if (row.flowInstanceBizRelevanceList.length == 1) {
            this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
          } else {
            let find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay)
            this.flowId = find.otherBizId
          }
          this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
          this.searchFlowType = row.auditWay;
          this.ExpensesClaimFormVisible = true;
        }
      }
    },
    getTaskList(type) {
      if (type == 'company') {
        this.search.handle = ''
        this.search.depart = ''
        let find = this.frameTree[0].childrenList.find(item => item.id == this.search.company)
        if (find) {
          this.findDepart = find//.childrenList
          this.departOptions = this.findDepart.childrenList.map(item => {
            return {
              label: item.name,
              value: item.id
            }
          })
          this.updatePerson()
        }
      } else if (type == 'depart') {
        this.updatePerson()
      }
      this.queryData()
    },
    updatePerson() {
      let childrenList = this.findDepart?.childrenList || []
      this.search.handle = ''
      let find = childrenList.find(item => item.id == this.search.depart)
      if (find) {
        let personList = find?.childrenList || []
        this.handlerOptions = personList.map(item => {
          return {
            label: item.name,
            value: item.id
          }
        })
      }
    },
    queryData() {
      var data = deepClone(this.originData)
      if (this.search.project) {
        data = data.filter(item => {
          if (item.project_name) {
            let project_name = item.project_name
            // let handler = item.handler
            return project_name.indexOf(this.search.project) > -1
          }
          // return item.initiator.indexOf(this.search.handler)>-1
        })
      }
      //流程名称
      if (this.search.flowName) {
        data = data.filter(item => {
          let formName = item.name || item.formName
          return formName.indexOf(this.search.flowName) > -1
        })
      }
      //公司
      if (this.search.company) {
        let find = this.companyOptions.find(item => item.value == this.search.company)
        let searchName = find.label
        data = data.filter(item => {
          let company = item.company || item.company_name
          return company.indexOf(searchName) > -1
        })
      }
      //部门
      if (this.search.depart) {
        let find = this.departOptions.find(item => item.value == this.search.depart)
        let searchName = find.label
        data = data.filter(item => {
          let depart = item.depart || item.depart_name
          return depart.indexOf(searchName) > -1
        })
      }
      //发起人
      if (this.search.initiator) {
        data = data.filter(item => {
          if (item.initiator) {
            let initiator = item.initiator
            return initiator.indexOf(this.search.initiator) > -1
          }
        })
      }

      //经办人
      if (this.search.handler) {
        data = data.filter(item => {
          let find = this.handlerOptions.find(item => item.value == this.search.handler)
          let searchName = find.label
          if (item.handler) {
            let handler = item.handler
            return handler.indexOf(searchName) > -1
          }
        })
      }
      //合同
      if (this.search.contract_name) {
        data = data.filter(item => {
          if (item.contract_name) {
            let contract_name = item.contract_name
            return contract_name.indexOf(this.search.contract_name) > -1
          }

        })
      }
      //金额范围
      if (this.search.range.min !== undefined && this.search.range.max !== undefined) {
        if (this.search.range.min !== '' && this.search.range.max !== '') {
          let min = this.search.range.min, max = this.search.range.max
          data = data.filter(item => {
            if (item.contract_price) {
              return (Number(item.contract_price) <= max && Number(item.contract_price) >= min)
            }
          })
        }
      }
      //流程状态
      if (this.search.flowStatus) {
        data = data.filter(item => {
          let flowStatus = item.flowStatus || item.status || item.kpiGroupStatus
          return this.search.flowStatus == flowStatus
        })
      }

      //发起时间
      if (this.startDate && this.endDate) {
        data = data.filter(item => {
          let createTime = item.initiatorDate || item.createDate
          let createTimeUnix = new Date(createTime).getTime()
          let startUnix = new Date(this.startDate).getTime()
          let endUnix = new Date(this.endDate).getTime()
          return (createTimeUnix <= endUnix && createTimeUnix >= startUnix)
        })
      }

      this.wholeData = deepClone(data)
      this.tableData = this.generateTableData(data)

      let inital = 0
      this.wholeData.forEach(item => {
        if (item.contract_price) {
          inital += Number(item.contract_price)
        }
      })
      this.total = inital
    },
    handleDateChange() {
      if (this.search.dateVal && this.search.dateVal.length) {
        this.startDate = this.search.dateVal[0] + ' 00:00:00';
        this.endDate = this.search.dateVal[1] + ' 23:59:59';
      } else {
        this.startDate = '';
        this.endDate = '';
      }
      this.getTaskList();
    },
  },
};
</script>
<style lang="scss" scoped>
.content-container {
  background: #fff;
  padding: 10px;
  height: 100%;

  .left-side {
    width: 180px;
    height: 100%;
    border-right: 1px solid #e2e2e2;
  }

  .right-aside {
    // flex-grow: 1;
    position: relative;
    width: 100%;

    .right-aside-inner {
      position: absolute;
      left: 0;
      padding: 0 20px;
      width: 100%;
    }
  }
}

::v-deep .el-tabs__header.is-left {
  width: 100%;
  padding-right: 0;
}

::v-deep .el-tabs--left .el-tabs__item.is-left {
  text-align: left;
}

::v-deep .el-table__header-wrapper .el-table__footer-wrapper {
  overflow: visible !important;
}

::v-deep .dytable-view-container .dytable-view-body {
  overflow: visible !important;
}

::v-deep .el-table {
  overflow: visible !important;
}

.search {
  display: flex;
  flex-wrap: wrap;

  >* {
    margin-bottom: 3px;
  }
}
</style>
<style>
.my-select .el-select-dropdown__item {
  width: 300px;
  display: inline-block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
