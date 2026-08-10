<!--  -->
<template>
  <div class="outer">
    <div class="top">
      <span>项目名称：</span>&nbsp;
      <el-select v-model="query.projectId" placeholder="请选择" style="margin-right:35px;" @change="projectChange"
        clearable>
        <el-option v-for="item in projectOptions" :key="item.id" :label="item.name" :value="item.id">
        </el-option>
      </el-select>
      <span>创建时间：</span>&nbsp;
      <el-date-picker v-model="query.annual" type="year" placeholder="选择年份" value-format="yyyy"
        style="margin-right:35px;" @change="searchByQuery" clearable>
      </el-date-picker>
      <!-- <el-button type="primary" @click="add">新建项目预算</el-button> -->
    </div>
    <div class="content">
      <h4>单位：万元</h4>
      <div class="table-content">
        <el-table :data="tableData" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }"
          stripe>
          <el-table-column prop="projectName" label="项目名称" width="200px"> </el-table-column>
          <el-table-column prop="total" label="预算总额" width="120px">
            <template slot-scope="scope">
              <el-tooltip class="item" effect="dark" content="查看预算详情" placement="right">
                <el-link type="primary" :underline="false" @click="toProjectBudgetDetail(scope.$index)">{{
                scope.row.total
                }}
                </el-link>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column prop="useMoney" label="已使用预算">
          </el-table-column>
          <el-table-column prop="leftBudget" label="剩余预算"> </el-table-column>
          <!-- <el-table-column prop="yearBudget" label="当年预算"> </el-table-column> -->
          <el-table-column prop="total" label="当年预算"> </el-table-column>
          <!-- <el-table-column prop="yearUse" label="当年已使用"> </el-table-column>
          <el-table-column prop="yearLeft" label="当年剩余预算" width="120px"> </el-table-column> -->
          <el-table-column prop="useMoney" label="当年已使用"> </el-table-column>
          <el-table-column prop="leftBudget" label="当年剩余预算" width="120px"> </el-table-column>
          <el-table-column prop="createDate" label="创建时间" width="180px"> </el-table-column>
          <el-table-column prop="statusCn" label="状态"> </el-table-column>
          <el-table-column fixed="right" label="操作" width="150">
            <template slot-scope="scope">
              <div v-if="scope.row.status == 1">
                <el-button @click="costDetail(scope.$index)" type="text" size="small">使用情况</el-button>
                <!-- <el-button v-if="scope.row.examineStatus == 1" @click="add(scope.$index, 'append')" type="text"
                  size="small">追加预算</el-button> -->
                <el-button @click="add(scope.$index, 'detail')" type="text" size="small">审批详情</el-button>
                <!-- <el-button @click="add(scope.$index, 'edit')" type="text" size="small"
                  v-if="scope.row.examineStatus == 2">
                  再次发起审批
                </el-button> -->
              </div>
              <div v-else-if="scope.row.status == 0">
                <!-- <el-button @click="add(scope.$index, 'edit')" type="text" size="small"
                  v-if="scope.row.examineStatus == 0">编辑草稿</el-button> -->
              </div>
              <!-- <el-button @click="showDetail(scope.$index)" type="text" size="small">使用归口明细</el-button> -->
            </template>
          </el-table-column>
        </el-table>
        <el-pagination :page-sizes="[10, 20, 50, 100]" background :total="pagination.total"
          :current-page="pagination.currentPage" layout="total, sizes, prev, pager, next" @size-change="handlePageSize"
          @current-change="pageChange" style="text-align:center;margin-top:15px;"></el-pagination>
      </div>
    </div>
    <!-- <detailedDialog :detailVisible.sync="detailVisible"></detailedDialog> -->
  </div>
</template>
<script>
import {
  localstorageGet,
} from '@/utils/auth';
import Api from '@/api';
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
export default {
  name: '',
  data() {
    return {
      tableData: [],
      query: {
        companyId: localstorageGet('companyId'),
        type: '3',
        annual: '',
        projectId: '',
        budgetTime: '',
      },
      options: [],
      value: '',
      projectOptions: [],
      pagination: {
        total: 0,
        size: 10,
        currentPage: 1,
        pageCount: 0,
        pageSizes: [10, 20, 50, 100]
      }
    };
  },
  computed: {},
  watch: {},
  created() {
    // this.getProjectVosByCompanyId(this.query.companyId)
    this.getAllCompany()
    this.searchByQuery()
  },
  mounted() {
  },
  methods: {
    projectChange(projectId) {
      if (projectId) {
        let index = this.projectOptions.findIndex(el => el.id == projectId)
        if (index > -1) {
          this.query.companyId = this.projectOptions[index].companyId
        } else {
          this.query.companyId = ''
        }
      }
      this.getProjectBudget()
    },
    searchByQuery() {
      this.getProjectBudget()
    },
    //获取主岗和副岗公司
    getAllCompany() {
      this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {}).then(res => {
        if (res.isSuccess) {
          let companyList = res.data || []
          companyList.forEach(item => {
            this.getProjectVosByCompanyId(item.id)
          })
        }
      })
    },
    getProjectVosByCompanyId(companyId) {
      this.$axios.post(
        Api.annualBudget.getProjectVosByCompanyId,
        {
          data: {
            companyId
          }
        },
        res => {
          if (res.isSuccess) {
            res.data.forEach(el => {
              el.companyId = companyId
              this.projectOptions.push(el)
            })
          }
        }
      );
    },
    getProjectBudget() {
      this.query.endTime = ''
      if (this.query.annual) {
        this.query.endTime = `${this.query.annual}-08-01 12:00:00`
      }
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: this.query,
          pagination: true,
          pages: this.pagination.currentPage,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            let data = res.data || {}
            let list = this.list = data.dataList || []
            this.processData(list)
            this.pagination.total = data.total
          } else {
            this.$message.error(res.message)
          }
        }
      );
    },
    processData(list) {
      this.tableData = list.map(item => {
        let total = (item.total || 0)
        let useMoney = item.useMoney || 0
        let leftBudget = total - useMoney
        let companyBudgetVos = item.companyBudgetVos
        let yearBudget = 0
        let yearUse = 0
        //原始当年追加
        if (companyBudgetVos.length) {
          let budgetDetailsVos = companyBudgetVos[0].budgetDetailsVos
          budgetDetailsVos.forEach(item => {
            let useMoney = item.useMoney || 0
            let money = item.money
            yearUse += useMoney
            yearBudget += money
          })
        }
        //追加当年的追加
        let appendCostBudgetVos = item.appendCostBudgetVos
        appendCostBudgetVos.forEach(el => {
          let appendCompanyBudgetVos = []
          if (el.companyBudgetVos[0] && el.companyBudgetVos[0].appendBudgetDetailsVos) appendCompanyBudgetVos = el.companyBudgetVos[0].appendBudgetDetailsVos
          appendCompanyBudgetVos.forEach(it => {
            let useMoney = it.useMoney || 0
            yearBudget += (it.money - 0)
            yearUse += (useMoney - 0)
          })
        })
        // let examineStatus = this.calculateStatus(item, 'examineStatus')
        let status = this.calculateStatus(item, 'status')
        let examineStatus = this.calculateExamineStatus(item)
        if (examineStatus == 0) {
          yearBudget = 0
          yearUse = 0
        }
        return {
          projectName: item.projectName,
          total: (total - 0).toFixed(6),
          useMoney: useMoney.toFixed(6),
          leftBudget: (leftBudget - 0).toFixed(6),
          yearBudget: (yearBudget - 0).toFixed(6),
          yearUse: (yearUse - 0).toFixed(6),
          status,
          yearLeft: (yearBudget - yearUse).toFixed(6),
          createDate: item.createDate,
          examineStatus,
          statusCn: this.examineStatusSn(status, examineStatus)
        }
      })
    },
    calculateExamineStatus(item) {
      let examineStatus = 1
      if (item.examineStatus != 1) {
        examineStatus = item.examineStatus
      } else {
        let appendCostBudgetVos = item.appendCostBudgetVos
        if (appendCostBudgetVos.length) {
          appendCostBudgetVos.forEach(el => {
            if (el.examineStatus != 1) {
              examineStatus = el.examineStatus
            }
          })
        }
      }
      return examineStatus
    },
    examineStatusSn(status, examineStatus) {
      if (status == 0) {
        return '草稿'
      } else {
        const CN = { '0': '审核中', '1': '审核通过', '2': '审核驳回' }
        return CN[examineStatus]
      }
    },
    calculateStatus(item, key) { //examineStatus status 需要遍历里层状态
      let status = 1
      if (item[key] != 1) {
        status = item[key]
      } else {
        let appendCostBudgetVos = item.appendCostBudgetVos
        if (appendCostBudgetVos.length) {
          appendCostBudgetVos.forEach(el => {
            if (el[key] != 1) {
              status = el[key]
            }
          })
        }
      }
      return status
    },
    handlePageSize(pageSize) {
      this.pagination.currentPage = 1
      this.pagination.size = pageSize
      this.searchByQuery()
    },
    pageChange(page) {
      this.pagination.currentPage = page
      this.searchByQuery()
    },
    costDetail(index) {
      let paramStr = {}
      if (this.list[index]) {
        paramStr = JSON.stringify(this.list[index])
      }
      this.$router.push({
        path: '/groupBudgetManage/ProjectBudget/BudgetDetails',
        name: 'GroupBudgetDetails',
        params: { str: paramStr },
      });
    },
    toProjectBudgetDetail(index) {
      let query = {
        companyId: this.list[index].companyId,
        budgetTime: this.list[index].createDate,
        projectId: this.list[index].projectId
      }
      this.$router.push({
        path: '/groupBudgetManage/ProjectBudget/ProjectBuegetDetails',
        query
      });
    },
    add(index, type) {
      if (type) {
        let paramStr = {}
        if (this.list[index]) {
          paramStr = JSON.stringify(this.list[index])
        }
        this.$router.push({
          path: '/groupBudgetManage/ProjectBudget/NewBudget',
          name: 'GroupNewBudget',
          params: { str: paramStr },
          query: { type }
        });
      } else {
        this.$router.push({
          path: '/groupBudgetManage/ProjectBudget/NewBudget',
          name: 'GroupNewBudget',
        });
      }
    },
  }
};

</script>
<style lang='scss' scoped>
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
</style>
