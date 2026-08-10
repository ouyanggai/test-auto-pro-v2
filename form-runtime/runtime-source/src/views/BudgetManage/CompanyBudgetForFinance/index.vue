<!--  -->
<template>
  <div class="outer">
    <div class="top">
      <!-- <span>公司名称：</span>&nbsp; -->
      <!-- <el-select v-model="query.companyId" placeholder="请选择" style="margin-right:35px;" @change="companyChange">
        <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id">
        </el-option>
      </el-select> -->
      <span>预算年度：</span>&nbsp;
    <!-- <el-select v-model="query.annual" placeholder="请选择" style="margin-right:10%" @change="dateChange" clearable>
        <el-option v-for="item in dateOption" :key="item.dictLabel" :label="item.dictValue" :value="item.dictValue">
        </el-option>
                  </el-select> -->
      <el-date-picker v-model="query.annual" type="year" value-format="yyyy" style="width: 160px;margin-right:35px;"
        @change="dateChange">
      </el-date-picker>
      <!-- <el-button type="primary" @click="add">新建年度预算</el-button> -->
    </div>
    <div class="content">
      <h4>单位：元</h4>
      <div class="table-content">
        <el-table :data="tableData" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }"
          stripe>
          <el-table-column prop="companyName" label="公司名称" width="280px">
            <template slot-scope="scope">
              {{ getCompanyAliasName(scope.row.companyId,scope.row.companyName) }}
            </template>
          </el-table-column>
          <el-table-column prop="budgetYear" label="预算年度"> </el-table-column>
          <el-table-column prop="totalBudget" label="预算总额">
            <template slot-scope="scope">
              {{ scope.row.totalBudget }}
              <!-- <el-tooltip class="item" effect="dark" content="查看预算详情" placement="right"> -->
              <!-- <el-link type="primary" :underline="false" @click="toAnnualBudgetDetail(scope.$index)">{{
                scope.row.totalBudget
                }}
                            </el-link> -->
                <!-- <el-link type="primary" :underline="false" @click="toAnnualBudgetDetail(scope.$index)">{{
                  scope.row.totalBudget
                }}
                </el-link> -->
              <!-- </el-tooltip> -->
            </template>
          </el-table-column>
          <el-table-column prop="usedBudget" label="已使用预算">
          </el-table-column>
          <el-table-column prop="leftBudget" label="剩余预算">
          </el-table-column>
          <!-- <el-table-column prop="statusName" label="状态" sortable="custom"> -->
          <el-table-column prop="statusName" label="状态" width="130px">
          </el-table-column>
          <el-table-column label="操作" width="180">
            <template slot-scope="scope">
              <div v-if="scope.row.status >= 1">
                <el-button @click="toCostDetail(scope.$index)" type="text" size="small">部门使用汇总
                </el-button>
                <!-- <el-button @click="handleView(scope.row)" type="text" size="small">审批详情</el-button> -->
              </div>
              <!-- <div v-else-if="scope.row.status == 0">
                <el-button  @click="handleView(scope.row)" type="text" size="small"
                            v-if="scope.row.examineStatus == 0">草稿详情</el-button>
                <el-button type="text" size="small" @click="deleteBudget(scope.row)">删除</el-button>
              </div> -->
            </template>
          </el-table-column>
        </el-table>
        <el-pagination :page-sizes="[10, 20, 50, 100]" background :total="pagination.total"
          :current-page="pagination.currentPage" layout="total, sizes, prev, pager, next" @size-change="handlePageSize"
          @current-change="pageChange" style="text-align:center;margin-top:15px;"></el-pagination>
      </div>
    </div>

  </div>
</template>

<script>
import Api from '@/api';
import {
  localstorageGet
} from '@/utils/auth';
import { deepClone } from '@/utils';
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
import math from '@/utils/math.js'
export default {
  // name: 'CompanyBudget',
  name: 'GroupCompanyBudgetIndex',
  data() {
    return {
      value: '',
      tableData: [],
      query: {
        companyIds: [],
        annual: '', // new Date().getFullYear(),
        budgetTime: '',
        type: 1
        // stringList:[1,6]
      },
      companyOption: [],
      pagination: {
        total: 0,
        size: 10,
        currentPage: 1,
        pageCount: 0,
        pageSizes: [10, 20, 50, 100]
      },
      aliasNameList:[],
      isGroupMember:localstorageGet('companyId') == localstorageGet('topCompanyId') ? true:false, //是否是集团人员登录
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
      isExamine:false,
      isReInitiate:false,
    };
  },
  computed: {},
  watch: {},
  // created() {
  //   this.init();
  // },
  // activated() {
  //   this.getDictCode('company_alias').then( res => {
  //     if (res.isSuccess) {
  //       let data = res.data || []
  //       this.aliasNameList = data
  //     } else {}
  //     this.init();
  //   })

  // },
  mounted() { 
    this.getUserAutehority()
  },
  methods: {
    getUserAutehority(){
      this.$axios.post(
        "/web/measuring/api/costJurisdiction/list",
        {
          data: {
            userId: this.$store.state.user.userId,
          },
          pagination:true,
          current:this.pagination.currentPage,
          size:this.pagination.pageSize
        },
        (res) => {
          if (res.isSuccess) {
            const tableData = res.data.dataList||[]
            if(tableData.length>0){
              this.getUserCompanyAuth(tableData[0].id)
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getUserCompanyAuth(id){
      this.$axios.post(
        "/web/measuring/api/costJurisdiction/findById",
        {
          data: {
            id
          },
        },
        (res) => {
          if (res.isSuccess) {
            res.data.userBusinessVoList.map(item=>{
              if(item.type=='1'){
                this.query.companyIds.push(item.companyId)
              }
            })
            if(this.query.companyIds.length>0){
              this.getDictCode('company_alias').then( res => {
                if (res.isSuccess) {
                  let data = res.data || []
                  this.aliasNameList = data
                } else {}
                this.init();
              })
            }
            
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    deleteBudget(row){
      this.$confirm('确认删除预算？','提示').then(res=>{
        //删除预算
        this.$axios.post(
          Api.annualBudget.budgetDelete,
          { data: { id: row.id }},
          res=>{
            if(res.isSuccess){
              this.init()
            }else{
              this.$message.error(res.message)
            }
          }
        );
      }).catch(()=>{})
    },
    getCompanyAliasName(companyId,companyName){
      let find = this.aliasNameList.find(item=>item.dictValue == companyId)
      console.log('find',find)
      if(find){
        return find.dictLabel
      }else{
        return companyName
      }
    },
    getDictCode(val){
      return this.$axios.post(Api.algorithm.getDicCodeTree, {
        data: {
          dictCode: val
        }
      });
    },
    init() {
      // this.getParentCompanyList();
      this.getCompanyBudget();
    },
    companyChange() {
      this.pagination.currentPage = 1
      this.getCompanyBudget();
    },
    dateChange() {
      this.pagination.currentPage = 1
      this.getCompanyBudget();
    },
    getParentCompanyList() { // 查询公司列表
      this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id: localstorageGet('companyId') // 当前用的公司id
          }
        },
        res => {
          let companyOption = res.data;
          this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {}).then(res => {
            if (res.isSuccess) {
              let dutyCompanyOption = res.data;
              let companyOptions = []
              dutyCompanyOption.forEach(item => {
                let dutyCompanyId = item.id
                let index = companyOption.findIndex(el => el.id == dutyCompanyId)
                if (index > -1) {
                  companyOptions.push(deepClone(companyOption[index]))
                }
              })
              this.companyOption = companyOptions
              //遍历获取公司预算
              companyOptions.forEach(item => {
                if (item.flag == 'mainDutyCompany') {
                  this.query.companyId = item.id
                  this.getCompanyBudget()
                }
              })
            } else {
              this.$message.error('人员暂无公司岗位')
            }
          })
        }
      );
    },
    getCompanyBudget() {
      this.query.budgetTime = '';
      if (this.query.annual) {
        this.query.budgetTime = `${this.query.annual}-01-01 00:00:00`;
      }
      let data = deepClone(this.query)
      if(this.isGroupMember){
        //data.stringList = [6]
        data.type = 6
        // delete data.type
      }else{
        data.type = 1
        // delete data.stringList
      }
      this.$axios.post(
        Api.annualBudget.expenseLedgerList,
        {
          data,//: this.query,
          pagination: false,
          pages: this.pagination.currentPage,
          size: this.pagination.size,
          grouping:false,
          detailed:false,
          projectDetailed:false,
          monthlyDetailed:false
        },
        res => {
          if (res.isSuccess) {
            const data = res.data || {};
            const list = this.list = data.dataList || [];
            this.tableData = list.map(item => {
              const total = item.total || 0;
              const useMoney = item.useMoney || 0;
              let examineStatus = item.examineStatus//this.calculateStatus(item, 'examineStatus')
              let status = item.status//this.calculateStatus(item, 'status')
              let muti = 10000
              let totalBudget = math.multiply(total,muti)
              let usedBudget = math.multiply(useMoney,muti)
              let leftBudget = math.subtract(totalBudget , usedBudget)
              totalBudget = this.numberCommas(totalBudget.toFixed(2))
              usedBudget = this.numberCommas(usedBudget.toFixed(2))
              leftBudget = this.numberCommas(leftBudget.toFixed(2))
              const obj = {
                id:item.id,
                companyId:item.companyId,
                companyName: item.companyName,
                budgetYear: item.budgetTime.substr(0, 4),
                totalBudget,//: (total - 0).toFixed(6),
                usedBudget,//: (useMoney - 0).toFixed(6),
                leftBudget,//: (total - useMoney).toFixed(6),
                status,
                statusName: this.examineStatusSn(status, examineStatus),
                examineStatus: examineStatus
              };
              if(item.type == 6 || item.type == 4){
                let departmentName = item.departmentName == '公司领导' ? '公司固定费用' : (item.departmentName || '')
                obj.companyName += ` / ${departmentName}`
              }
              return obj;
            });
            this.pagination.total = data.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    numberCommas(x){
      if(x){
        var res = x.toString().replace(/\d+/, function (n) {
          return n.replace(/(\d)(?=(\d{3})+$)/g, function ($1) {
            return $1 + ",";
          });
        });
        return res;
      }else{
        return x
      }
    },
    handlePageSize(pageSize) {
      this.pagination.size = pageSize;
      this.getCompanyBudget();
    },
    pageChange(page) {
      this.pagination.currentPage = page;
      this.getCompanyBudget();
    },
    examineStatusSn(status, examineStatus) {
      if (status == 0) {
        return '草稿';
      } else if(status == 1) {
        const CN = { 0: '审核中', 1: '审核通过', 2: '审核驳回' };
        return CN[examineStatus];
      }else if(status == 2){
        return '系统生成'
      }
    },
    toCostDetail(index) {
      let ids = []
      if(this.list[index]){
        let obj = this.list[index]
        ids.push(obj.id)
        let budgetProjectVoList = obj?.budgetProjectVoList || []
        budgetProjectVoList.forEach(el=>{
          ids.push(el.projectBudgetId)
        })
        let year = obj.budgetTime.substr(0,4)
        let companyId = obj.companyId
        this.$router.push({
          path: '/groupBudgetManage/companyBudget/budgetCostDetail',
          name: 'BudgetCostDetail',
          query: { ids: ids.join(','),year,companyId}
        });
        return
      }
    },
    appendBudget(index, type) {
      let paramStr = {};
      if (this.list[index]) {
        paramStr = JSON.stringify(this.list[index]);
      }
      this.$router.push({
        path: '/groupBudgetManage/companyBudget/addAnnualBudget',
        name: 'GroupAddAnnualBudget',
        params: { str: paramStr },
        query: { type }
      });
    },

    handleView(row) {
      //查询当前绑定的流程，调用查看弹窗
      this.getInstanceId(row.id).then(data=>{
        if(data){
          this.previewHandle(data)
        }else{
          // this.$message.error('流程已删除')
          //获取预算详情，跳转详情页
          this.$axios.post(
            Api.annualBudget.budgetList,
            {
              data:{
                id:row.id
              },
              pagination: false,
              pages: 1,
              size: 1,
              grouping:false,
              detailed:true,
            },
            res => {
              if (res.isSuccess) {
                let list = res.data?.dataList || []
                if(list.length){
                  let paramStr = JSON.stringify(list[0]);
                  this.$router.push({
                    path: '/groupBudgetManage/companyBudget/addAnnualBudget',
                    name: 'GroupAddAnnualBudget',
                    params: { str: paramStr },
                    query: { type:'detail' }
                  });
                }else{
                  this.$message.error('数据已被删除');
                }
              } else {
                this.$message.error(res.message);
              }
            }
          );
        }
      })
    },
    previewHandle(row){
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
      this.ExpensesClaimFormVisible = true;
    },
    // 获取流程实例id
    getInstanceId(id, type,taskStatus) {
      let otherBiz = type
      const flowInstanceBizRelevanceList = [{
        otherBiz,
        otherBizId: id,
      }];
      const data = {
        useScope: 'invest',
        initiator: 'all',
        flowInstanceBizRelevanceList
      };
      let api
      if(taskStatus == 'edit'){
        data.taskStatus = "waiting_send"
        api = Api.approveManage.getTaskList
      }else{
        api = Api.schedule.getFlowInstanceList
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, size: 1, pagination: true, pages: 1 }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data[0])
            } else {
              resolve()
            }
          }
        });
      });
    },
  }
};
</script>
<style lang="scss" scoped>
::v-deep .el-table__body-wrapper.is-scrolling-none {
  // height: 73vh;
  // height: calc(72vh);
  overflow-y: auto;
}

.outer {
  overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;

  .top {
    padding: 40px 25px 0 25px;
  }
}

.content {
  padding: 25px;
  height: calc(100% - 90px);

  .table-content {
    height: 100%;
    overflow: hidden;
  }

  .el-table {
    max-height: calc(100% - 52px);
    overflow: auto;
  }

  .el-table::before {
    height: 0;
  }
}
::v-deep .el-table--mini .el-table__cell{
    color: #000;
  }
</style>
