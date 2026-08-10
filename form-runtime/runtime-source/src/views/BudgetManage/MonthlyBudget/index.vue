<!--  -->
<template>
  <div class="outer">
    <div class="top">
      <!-- <span v-if="companyList.length > 1">
        <span>公司：</span>&nbsp;
        <el-select v-model="query.companyId" placeholder="请选择" style="margin-right:35px;" @change="companyChange">
          <el-option v-for="item in companyList" :key="item.id" :label="item.name" :value="item.id">
          </el-option>
        </el-select>
      </span> -->
      <span>部门：</span>&nbsp;
      <el-select v-model="query.projectId" placeholder="请选择" style="margin-right:35px;" @change="searchByQuery" clearable>
        <el-option v-for="item in departOptions" :key="item.id" :label="item.name" :value="item.id">
        </el-option>
      </el-select>
      <span>月份：</span>&nbsp;
      <el-date-picker v-model="query.month" type="month" placeholder="选择月份" value-format="yyyy-MM"
        style="margin-right:35px;" @change="searchByQuery" clearable>
      </el-date-picker>
    <!-- <el-select v-model="datas.budgetMonth" placeholder="请选择" style="margin-right:10%">
        <el-option maxTableHeight="73vh" v-for="item in monthOptions" :key="item.value" :label="item.label"
          :value="item.value">
        </el-option>
          </el-select> -->
      <!-- <el-button type="primary" @click="add">新建月度预算</el-button> -->
    </div>
    <div class="content">
      <h4>单位：元</h4>
      <div class="table-content">
        <el-table :data="tableData" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }" lazy :load="load">
          <el-table-column prop="departmentName" label="部门" :show-overflow-tooltip="true" min-width="200"> </el-table-column>
          <el-table-column prop="total" label="月度预算金额(元)">
            <template slot-scope="scope">
              {{ scope.row.total | toFix2 | numberWithCommas }}
            </template>
          </el-table-column>
          <el-table-column prop="useMoney" label="已使用预算(元)">
            <template slot-scope="scope">
              {{ scope.row.useMoney | toFix2 | numberWithCommas }}
            </template>
          </el-table-column>
          <el-table-column prop="leftBudget" label="剩余预算(元)">
            <template slot-scope="scope">
              <span :class="[{'red':scope.row.leftBudget<0}]">
                {{ scope.row.leftBudget | toFix2 | numberWithCommas }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="month" label="月份"> </el-table-column>
          <el-table-column prop="statusCn" label="状态"> </el-table-column>
          <el-table-column label="操作" width="150">
            <template slot-scope="scope">
              <div v-show="scope.row.parent == 1">
                <div v-if="scope.row.status == 1">
                  <!-- <el-button @click="add(scope.row.id, 'detail')" type="text" size="small">审批详情</el-button> -->
                  <el-button @click="handleView(scope.row)" type="text" size="small">审批详情</el-button>
                  <!-- <el-button @click="add(scope.row.id, 'edit')" type="text" size="small"
                    v-if="scope.row.examineStatus == 2">再次发起审批
                  </el-button> -->
                </div>
                <div v-if="scope.row.status == 0">
                  <el-button @click="deleteBudget(scope.row)" type="text" size="small">删除</el-button>
                </div>
              <!-- <el-button @click="add(scope.row.id, 'edit')" type="text" size="small">再次发起审批
                    </el-button> -->
              </div>
              <div v-show="scope.row.children == 1">
                <el-button @click="openItemUseDetail(scope.row)" type="text" size="small">使用归口明细</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination :page-sizes="[10, 20, 50, 100]" background :total="pagination.total"
          :current-page="pagination.currentPage" layout="total, sizes, prev, pager, next" @size-change="handlePageSize"
          @current-change="pageChange" style="text-align:center;margin-top:15px;"></el-pagination>
      </div>
    </div>
    <!-- 费用明细列表 -->
    <detailedDialog :detailVisible.sync="detailVisible" v-if="detailVisible" :isShowBudget="false" :detailRow="detailRow" :isShowDepartment="false"></detailedDialog>
    <!-- 再次发起流程/查看 -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="isReInitiate" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
      :initiatorId="initiatorId" :actionType="actionType" :flowType="flowType" :flowName="flowName"/>
  </div>
</template>

<script>

import detailedDialog from '../CompanyBudget/components/detailedDialog.vue';
import Api from '@/api';
import {
  localstorageGet
} from '@/utils/auth';
import numFunc from '@/utils/number'  //重写toFixed
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
Number.prototype.toFixed = numFunc
export default {
  name: 'MonthlyBudget',
  components: { detailedDialog ,ExamineDialog},
  data() {
    return {
      detailVisible: false,
      datas: {
        depart: '',
        depratId: '',
        budgetMonth: ''
      },
      query: {
        companyId: localstorageGet('companyId'),
        month: '', // new Date().getFullYear(),
        projectId: '',
        budgetTime: '',
        // type: 2 // 月度预算
        stringList:[2,5]
      },
      companyList: [],
      tableData: [
      ],
      departOptions: [],
      pagination: {
        total: 0,
        size: 10,
        currentPage: 1,
        pageCount: 0,
        pageSizes: [10, 20, 50, 100]
      },
      detailRow: {},
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
  filters: {
    toFix2(val) {
      if (val != '' && (val - 0) == (val - 0)) {
        let v = val - 0
        return v.toFixed(2)
      }
      if (val == '') {
        return '0.00'
      }
    },
    numberWithCommas(x) {
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
    }
  },
  activated() {
    // this.getCompanyBudget()
    // this.getAllCompany()
    this.getBudgetTypeOfGroup(this.query.companyId);
    this.searchByQuery();
  },
  mounted() { },
  methods: {
    handleView(row) {
      //查询当前绑定的流程，调用查看弹窗
      this.getInstanceId(row.id).then(data=>{
        if(data){
          this.previewHandle(data)
        }else{
          // this.$message.error('流程已删除')
          //获取预算详情，跳转详情页
          this.handleViewPage(row)
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
      this.flowProxyId = row.flowProxyId
      this.$nextTick(() => {
        this.ExpensesClaimFormVisible = true;
      })
    },
    handleViewPage(row) {
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
                path: '/groupBudgetManage/monthlyBudget/addMonthlyBudget',
                name: 'GroupAddMonthlyBudget',
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
    },
    load(tree, treeNode, resolve){
      let {month,projectId} = tree
      //projectId 部门id和项目id都用这个
      this.getCompanyBudgetDetail(month,projectId).then(children=>{
        resolve(children)
      })
    },
    getCompanyBudgetDetail(month,projectId){
      return new Promise((resolve,reject)=>{
        this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: {
            budgetTime:`${month}-01 00:00:00`,
            companyId:this.query.companyId,
            projectId,
            stringList:[2, 5]
          },
          pagination: true, //后台说这个接口如果要分页就要给false，奇葩接口
          pages: this.pagination.currentPage,
          size: this.pagination.size,
          grouping:false,
          detailed:true,
        },
        res => {
          if (res.isSuccess) {
            const data = res.data || {};
            const list = this.list = data.dataList || [];
            let budgetData = list[0]
            let budgetDetailsVos = budgetData.budgetDetailsVos || []
            let children = []
            budgetDetailsVos.forEach(ele => {
              let childUseMoney = ele.useMoney || 0
              childUseMoney*=10000
              const departmentId = ele.departmentId;
              let childMoney = ele.money || 0
              childMoney*=10000
              const tmpObj = {
                id: ele.id,
                budgetTypeId: ele.budgetTypeId,
                departmentName: ele.budgetTypeVo?.name || '', // 归口名称
                departmentId,
                companyId: this.query.companyId,
                total: childMoney,
                useMoney: childUseMoney,
                leftBudget: childMoney - childUseMoney,
                children: 1
              };
              children.push(tmpObj);
            });
            resolve(children)
          }else{
            resolve([])
          }
        }
      );
      })
    },
    clickDeleteFlow(id) {
      this.$axios.post(
        Api.approveManage.taskFlowDelete,
        {
          data: {},
          ids: [id]
        });
    },
    deleteBudget(row){
      this.$confirm('确认删除预算？','提示').then(async res=>{
        let resp = await this.getInstanceId(row.id)
        if(resp){
          this.clickDeleteFlow(resp.id)
        }
        //删除预算
        this.$axios.post(
          Api.annualBudget.budgetDelete,
          { data: { id: row.id }},
          res=>{
            if(res.isSuccess){
              this.getCompanyBudget()
            }else{
              this.$message.error(res.message)
            }
          }
        );
      }).catch(()=>{})
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
        // taskStatus:'waiting_send',
        // statusList:["await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end","draft"],//: 'waiting_send',
        initiator: 'all',
        // auditWayList: this.sFlowTypeList,
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
    getBudgetTypeOfGroup(companyId) {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            const data = res.data || [];
            const find = data.find(item => item.companyVo.id == companyId);
            if (find) {
              this.centralizedApiVos = find.centralizedApiVos[0];
              this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
              this.generateDepartOption();
              resolve()
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
      })
    },
    generateDepartOption() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo } = item;
        departOptions.push({
          id:sysDepartmentVo.id,
          name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item=>{
        departOptions.push({
          id:item.projectVo.id,
          name:item.projectVo.name,
          hasSelect: false,
          isProject:true
        })
      })
      this.departOptions = departOptions
    },
    searchByQuery() {
      this.pagination.currentPage = 1;
      this.getCompanyBudget();
    },
    getCompanyBudget() {
      this.query.budgetTime = '';
      if (this.query.month) {
        this.query.budgetTime = `${this.query.month}-01 00:00:00`;
      }
      // this.query.companyId = ''
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: this.query,
          pagination: false, //后台说这个接口如果要分页就要给false，奇葩接口
          pages: this.pagination.currentPage,
          size: this.pagination.size,
          grouping:false,
          detailed:false,
          monthlyDetailed:false
        },
        res => {
          if (res.isSuccess) {
            const data = res.data || {};
            const list = this.list = data.dataList || [];
            this.processData(list);
            this.pagination.total = data.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    processData(list) {
      const monthList = list.map(item => {
        let total = item.total || 0;
        total = total * 10000
        let useMoney = item.useMoney || 0;
        useMoney *= 10000
        const obj = {
          id: item.id,
          departmentName: item.departmentName == '公司领导' ? '公司固定费用' : item.departmentName || item.projectName,
          total: total - 0,
          useMoney: (useMoney - 0),
          leftBudget: (total - useMoney),
          statusCn: this.examineStatusSn(item.status, item.examineStatus),
          status: item.status,
          month: item.budgetTime.substr(0, 7),
          parent: 1,
          examineStatus: item.examineStatus,
          projectId:item.projectId,
          hasChildren:true,
          children: []
        };
        return obj;
      });
      this.tableData = monthList;
    },
    examineStatusSn(status, examineStatus) {
      if (status == 0) {
        return '草稿';
      } else {
        const CN = { 0: '审核中', 1: '审核通过', 2: '审核驳回' };
        return CN[examineStatus];
      }
    },
    handlePageSize(pageSize) {
      this.pagination.currentPage = 1;
      this.pagination.size = pageSize;
      this.searchByQuery();
    },
    pageChange(page) {
      this.pagination.currentPage = page;
      this.getCompanyBudget();
    },
    add(id, type) {
      let paramStr = {}, obj
      if (this.list) obj = this.list.find(item => item.id == id);
      if (obj) {
        paramStr = JSON.stringify(obj);
      }
      this.$router.push({
        path: '/groupBudgetManage/monthlyBudget/addMonthlyBudget',
        name: 'GroupAddMonthlyBudget',
        params: { str: paramStr },
        query: { type }
      });
    },
    openItemUseDetail(row) {
      this.detailRow = row;
      this.detailVisible = true;
    }
  }
};
</script>
<style lang="scss" scoped>
.outer {
  overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;

  .top {
    padding: 40px 25px 0 25px;
  }
}


::v-deep .el-date-editor.el-input,
::v-deep .el-date-editor.el-input__inner {
  width: 180px !important;
}

::v-deep .el-table .cell {
  white-space: nowrap;
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
.red{
  color:#F56C6C;
}
</style>
