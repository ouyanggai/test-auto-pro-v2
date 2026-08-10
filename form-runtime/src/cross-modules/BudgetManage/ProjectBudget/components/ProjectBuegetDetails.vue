<template>
  <!-- 年度预算详情 -->
  <div class="outer">
    <div class="content">
      <h3 style="margin-bottom:10px;">费用预算计划</h3>
      <!-- <h4 style="padding-left:15px;">单位：万元</h4> -->
      <el-table :data="projectData" row-key="id" :show-summary="true" style="width: 950px;">
        <el-table-column prop="departmentName" label="部门" width="180px"> </el-table-column>
        <el-table-column prop="name" label="费用预算类型(一级）"> </el-table-column>
        <el-table-column prop="money" label="预算总额（万）">
          <template slot-scope="scope">
            {{ (scope.row.money - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="useMoney" label="已使用预算（万）">
          <template slot-scope="scope">
            {{ (scope.row.useMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="leftMoney" label="剩余预算（万）">
          <template slot-scope="scope">
            {{ (scope.row.leftMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div class="content">
      <h3 style="margin-bottom:10px;">追加当年公司预算</h3>
      <!-- <h4 style="padding-left:15px;">单位：万元</h4> -->
      <el-table :data="companyData" row-key="id" :show-summary="true" style="width: 950px;">
        <el-table-column prop="departmentName" label="部门" width="180px"> </el-table-column>
        <el-table-column prop="name" label="费用预算类型(一级）"> </el-table-column>
        <el-table-column prop="money" label="预算总额（万）">
          <template slot-scope="scope">
            {{ (scope.row.money - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="useMoney" label="已使用预算（万）">
          <template slot-scope="scope">
            {{ (scope.row.useMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="leftMoney" label="剩余预算（万）">
          <template slot-scope="scope">
            {{ (scope.row.leftMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top:10px;">
        <h4>查看预算审批详情</h4>
        <el-link type="primary" :underline="false" @click="toBudgetDetail">项目预算详情<i class="el-icon-arrow-right"></i>
        </el-link>
      </div>
    </div>
  </div>
</template>
<script>
import Api from '@/api';
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
export default {
  name: 'ProjectBuegetDetails',
  data() {
    return {
      projectData: [],
      companyData: []
    }
  },
  async created() {
    let query = {
      companyId: this.$route.query.companyId,
      endTime: this.$route.query.budgetTime,
      projectId: this.$route.query.projectId,
      type: 3
    }
    let that = this
    this.getProjectBudget(query).then(async function (res) {
      let data = res.data || {}
      let list = that.list = data.dataList || []
      that.params = list[0]
      await that.getDepartByCompanyId(that.params.companyId)
      //初始项目数据
      let budgetDetailsVos = that.params.budgetDetailsVos || []
      let companyBudgetVos = that.params.companyBudgetVos || []
      let appendCostBudgetVos = that.params.appendCostBudgetVos || []
      if (that.params.examineStatus == 1 || appendCostBudgetVos.length) {
        that.processData(budgetDetailsVos, that.projectData, false)
      }
      //初始追加的公司预算数据
      if (that.params.examineStatus == 1) {
        that.processData(companyBudgetVos, that.companyData, false, 'company')
      }

      //追加的初始项目数据

      appendCostBudgetVos.forEach(it => {
        if (it.examineStatus == 1) {
          let appendBudgetDetailsVos = it.appendBudgetDetailsVos
          appendBudgetDetailsVos.forEach(item => {
            let budgetTypeId = item.budgetTypeId
            let index = that.projectData.findIndex(el => el.budgetTypeId == budgetTypeId)
            let departmentId = item.departmentId
            let name = item.budgetTypeVo.name
            let money = item.money || 0
            let useMoney = item.useMoney || 0
            let leftMoney = money - useMoney
            if (index > -1) {
              that.projectData[index].money += (money - 0)
              that.projectData[index].useMoney += (useMoney - 0)
              that.projectData[index].leftMoney += (leftMoney - 0)
            } else {
              let obj = {
                departmentName: that.getDepartNameById(departmentId),
                departmentId,
                name,
                money,
                useMoney,
                leftMoney,
                budgetTypeId
              }
              that.projectData.push(obj)
            }
          })

          //当年度公司追加公司预算 的追加
          let companyBudgetVos = it.companyBudgetVos
          let appendCompanyBudgetVos = companyBudgetVos[0].appendBudgetDetailsVos || []

          that.processData(appendCompanyBudgetVos, that.companyData, true)
        }

      })
    })
  },
  methods: {
    getProjectBudget(query) {
      return this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query,
        }
      );
    },
    processData(list, data, isAppend, type) {
      list.forEach(item => {
        let money = item.money || 0
        let useMoney = item.useMoney || 0
        let departmentId = item.departmentId
        let leftMoney = money - useMoney
        if (type == 'company') {
          if (item.budgetDetailsVos.length) {
            let budgetDetailsVos = item.budgetDetailsVos
            budgetDetailsVos.forEach(el => {
              money = el.money || 0
              useMoney = el.useMoney || 0
              departmentId = el.departmentId
              leftMoney = money - useMoney
              let budgetTypeId = el.budgetTypeId
              let obj = {
                departmentName: this.getDepartNameById(departmentId),
                departmentId,
                name: el.budgetTypeVo.name,
                money,
                useMoney,
                leftMoney,
                budgetTypeId
              }
              data.push(obj)
            })
          }
        }
        if (isAppend) {
          let index = data.findIndex(it => it.departmentId == departmentId)
          if (index > -1) {
            //查看是否有相同归口，有相同归口才相加，不相同则push
            let budgetTypeId = item.budgetTypeId
            let idx = data.findIndex(el => el.budgetTypeId == budgetTypeId)
            if (idx > -1) {
              data[index].money += (money - 0)
              data[index].useMoney += (useMoney - 0)
              data[index].leftMoney += (leftMoney - 0)
            } else {
              let obj = {
                departmentName: this.getDepartNameById(departmentId),
                departmentId,
                name: item.budgetTypeVo.name,
                money,
                useMoney,
                leftMoney,
                budgetTypeId
              }
              data.push(obj)
            }
          } else {
            // getDepartNameById
            let budgetTypeId = item.budgetTypeId
            let obj = {
              departmentName: this.getDepartNameById(departmentId),
              departmentId,
              name: item.budgetTypeVo.name,
              money,
              useMoney,
              leftMoney,
              budgetTypeId
            }
            data.push(obj)
          }
        } else {
          if (type != 'company') {
            let obj = {
              departmentName: this.getDepartNameById(departmentId),
              departmentId,
              // name: ,
              money,
              useMoney,
              leftMoney,
            }
            obj['budgetTypeId'] = item.budgetTypeId
            obj['name'] = item.budgetTypeVo.name
            data.push(obj)
          }
        }
      });
    },
    async getDepartByCompanyId(id) { // 获取公司部门架构数据
      await this.$axios.post(
        Api.annualBudget.getDepartByCompanyId,
        {
          data: {
            id//: this.query.companyId // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            let data = res.data
            if (data && data.departmentVos) {
              data.departmentVos.forEach(item => {
                if (item.departmentName == '公司领导') item.departmentName = '公司固定费用'
              })
              this.departOptions = data.departmentVos
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getDepartNameById(id) {
      if (id) {
        let obj = this.departOptions.find(item => item.id == id)
        return obj.departmentName
      } else {
        return ''
      }
    },
    toBudgetDetail() {
      let paramStr = JSON.stringify(this.params), type = 'detail'
      this.$router.push({
        path: '/groupBudgetManage/ProjectBudget/NewBudget',
        name: 'GroupNewBudget',
        params: { str: paramStr },
        query: { type }
      });
    }
  }
}
</script>
<style lang="scss" scoped>
.outer {
  background: white;
  display: flow-root;
  height: 100%;

  .top {
    margin: 40px 0 0 40px;
  }

  .content {
    padding: 25px;
  }
}
</style>
