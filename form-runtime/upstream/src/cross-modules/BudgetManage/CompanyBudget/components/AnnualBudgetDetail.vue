<template>
  <!-- 年度预算详情 -->
  <div class="outer">
    <div class="content">
      <h3 style="margin-bottom:10px;">{{ annual }}年 预算详情</h3>
      <h4 style="padding-left:15px;">单位：万元</h4>
      <div class="table-content">
        <el-table :data="tableData" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }"
          :show-summary="true" :summary-method="summary" style="max-width: 1080px;" :max-height="tableMax">
          <el-table-column prop="departName" label="部门" width="220px"> </el-table-column>
          <el-table-column prop="money" label="预算总额">
            <template slot-scope="scope">
              {{ scope.row.money | toFix6 }}
            </template>
          </el-table-column>
          <el-table-column prop="useMoney" label="已使用预算">
            <template slot-scope="scope">
              {{ scope.row.useMoney | toFix6 }}
            </template>
          </el-table-column>
          <el-table-column prop="leftMoney" label="剩余预算">
            <template slot-scope="scope">
              {{ scope.row.leftMoney | toFix6 }}
            </template>
          </el-table-column>
          <el-table-column prop="projectName" label="关联项目"> </el-table-column>
        </el-table>
      </div>

      <div style="margin-top:20px;max-width: 1080px;">
        <h4>查看预算审批详情</h4>
        <el-link type="primary" :underline="false" @click="toCompanyDetail" style="margin-top:15px;">公司年度预算详情<i
            class="el-icon-arrow-right"></i>
        </el-link>
        <div style="margin-top:15px;overflow: hidden;">
          <div style="overflow:auto;max-height: 80px;">
            <el-row v-for="index of projectRow" :key="index">
              <el-col :span="8" v-for="i in 3" :key="i">
                <el-link type="primary" :underline="false" @click="toBugetDetail(projectList[(index - 1) * 3 + (i - 1)])"
                  :key="'' + index + i" style="margin-right:15px;" v-if="projectList[(index - 1) * 3 + (i - 1)]">
                  {{ projectList[(index - 1) * 3 + (i - 1)].projectName }} 项目预算详情<i class="el-icon-arrow-right"></i></el-link>
              </el-col>
            </el-row>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import { deepClone } from '@/utils/index'
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
export default {
  name: 'AnnualBudgetDetail',
  data() {
    return {
      annual: '',
      projectList: [],
      tableData: [],
      projectRow: 0,
    };
  },
  filters: {
    toFix6(val) {
      if (val != '' && (val - 0) == (val - 0)) {
        let v = val - 0
        return v.toFixed(6)
      }
      if (val == '') {
        return '0.000000'
      }
    }
  },
  async created() {
    let query = {
      companyId: this.$route.query.companyId,
      budgetTime: this.$route.query.budgetTime,
      type: 1
    }
    let that = this
    this.getCompanyBudget(query).then(async function (res) {
      const data = res.data || {};
      const list = that.list = data.dataList || [];
      that.params = list[0]//JSON.parse(params)
      let budgetTime = that.params.budgetTime
      that.annual = budgetTime.substr(0, 4)
      query.type = 3
      query.endTime = query.budgetTime
      delete query.budgetTime
      await that.getDepartByCompanyId(that.params.companyId)
      await that.getProjectBudget(query)
      that.processData(deepClone(that.params), deepClone(that.projectList))
      that.processProjectType(that.projectList) //处理项目的追加公司预算归口，放到上面
    })
  },
  computed: {
    tableMax() {
      let winHeight = window.innerHeight
      let tHeight = winHeight - 50 - 50 - 90 - 180
      return tHeight
    }
  },
  methods: {
    getCompanyBudget(query) {

      return this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        }
      );
    },
    async getProjectBudget(query) {
      await this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query,
        },
        res => {
          if (res.isSuccess) {
            let data = res.data || {}
            this.projectList = data.dataList || []
            // console.log('this.projectList', this.projectList)
            this.projectRow = Math.ceil(this.projectList.length / 3)
          } else {
            // this.$message.error(res.message)
          }
        }
      );
    },
    processProjectType(projectList) {
      projectList.forEach(item => {
        let appendCostBudgetVos = item.appendCostBudgetVos
        if (item.examineStatus == 1 || appendCostBudgetVos.length ) {
          let companyBudgetVos = item.companyBudgetVos[0]
          if (companyBudgetVos) {
            let budgetDetailsVos = companyBudgetVos.budgetDetailsVos
            budgetDetailsVos.forEach(el => {
              let departmentId = el.departmentId
              let money = (el.money - 0) || 0
              let useMoney = (el.useMoney - 0) || 0
              let leftMoney = money - useMoney
              let idx = this.tableData.findIndex(it => it.departmentId == departmentId)
              if (idx > -1) {
                this.tableData[idx].money = ((this.tableData[idx].money - 0) + (money - 0))
                this.tableData[idx].useMoney = ((this.tableData[idx].useMoney - 0) + (useMoney - 0))
                this.tableData[idx].leftMoney += (leftMoney - 0)
                this.tableData[idx].children.push(
                  {
                    budgetDetailsId: el.budgetDetailsId || '',
                    budgetTypeId: el.budgetTypeId,
                    departName: el.budgetTypeVo.name,//this.getDepartNameById(departmentId),//"约德尔2022公司1-2",
                    id: el.id + this.getRandom(1000, 9999),
                    leftMoney,
                    money,
                    projectName: '',
                    type: "depart",
                    useMoney,
                    projectName: item.projectName
                  }
                )
              } else {
                let obj = {
                  children: [
                    {
                      budgetDetailsId: el.budgetDetailsId || '',
                      budgetTypeId: el.budgetTypeId,
                      departName: el.budgetTypeVo.name,//this.getDepartNameById(departmentId),//"约德尔2022公司1-2",
                      id: el.id + this.getRandom(1000, 9999),
                      leftMoney,
                      money,
                      projectName: '',
                      type: "depart",
                      useMoney,
                      projectName: item.projectName
                    }
                  ],
                  departName: this.getDepartNameById(departmentId),
                  departmentId,
                  id: el.id + this.getRandom(1000, 9999),
                  leftMoney,
                  money,
                  type: "depart",
                  useMoney,
                }
                let idx = this.tableData.findIndex(el => el.type == 'depart')
                this.tableData.splice(idx, 0, obj)
              }
            })
          }
          //查看追加的归口
          let appendCostBudgetVos = item.appendCostBudgetVos
          appendCostBudgetVos.forEach(el => { //遍历每一次追加
            if (el.examineStatus == 1) {
              let companyBudgetVos = el.companyBudgetVos[0].appendBudgetDetailsVos
              companyBudgetVos.forEach(it => {
                let departmentId = it.departmentId
                let money = (el.money || 0)
                let useMoney = (el.useMoney || 0)
                let leftMoney = (money - useMoney)
                let index = this.tableData.findIndex(el => el.departmentId == departmentId)
                if (index > -1) {
                  this.tableData[index].money += money
                  this.tableData[index].useMoney += useMoney
                  this.tableData[index].leftMoney += leftMoney
                  let budgetTypeId = it.budgetTypeId
                  let idx = this.tableData[index].children.findIndex(el => el.budgetTypeId == budgetTypeId)
                  if (idx > -1) {
                    this.tableData[index].children[idx].money += money
                    this.tableData[index].children[idx].useMoney += useMoney
                    this.tableData[index].children[idx].leftMoney += leftMoney
                  } else {
                    this.tableData[index].children.push({
                      budgetDetailsId: it.budgetDetailsId || '',
                      budgetTypeId: it.budgetTypeId,
                      departName: it.budgetTypeVo.name,//this.getDepartNameById(departmentId),//"约德尔2022公司1-2",
                      id: it.id + this.getRandom(1000, 9999),
                      leftMoney,
                      money,
                      projectName: item.projectName,
                      type: "depart",
                      useMoney,
                    })
                  }
                } else {
                  let obj = {
                    children: [
                      {
                        budgetDetailsId: it.budgetDetailsId || '',
                        budgetTypeId: it.budgetTypeId,
                        departName: it.budgetTypeVo.name,//this.getDepartNameById(departmentId),//"约德尔2022公司1-2",
                        id: it.id + this.getRandom(1000, 9999),
                        leftMoney,
                        money,
                        projectName: item.projectName,
                        type: "depart",
                        useMoney,
                      }
                    ],
                    departName: this.getDepartNameById(departmentId),
                    departmentId,
                    id: it.id + this.getRandom(1000, 9999),
                    leftMoney,
                    money,
                    type: "depart",
                    useMoney,
                  }
                  let idx = this.tableData.findIndex(el => el.type == 'depart')
                  this.tableData.splice(idx, 0, obj)
                }
              })
            }
          })
        }
      })
    },
    getRandom(min, max) {
      return Math.floor(Math.random() * (max - min + 1)) + min;
    },
    processData(data, projectData) {
      let tableData = [
        {
          id: 1,
          departName: '部门'
        }
      ]
      //1 遍历计算每个部门得每个归口金额信息

      //1.1： 初始预算 部门-------------------------------
      let budgetDetailsVos = data.budgetDetailsVos || []
      // console.log('budgetDetailsVos', budgetDetailsVos)
      let appendCostBudgetVos = data.appendCostBudgetVos || []
      if (data.examineStatus == 1 || appendCostBudgetVos.length || data.status == 2) {
        this.createObj(budgetDetailsVos, tableData, 'depart')
      }

      //1.2 计算追加的预算 ------------------------------

      appendCostBudgetVos.forEach(it => {
        //每一个it就是一次追加
        if (it.examineStatus == 1) {
          let appendBudgetDetailsVos = it.appendBudgetDetailsVos
          this.createObj(appendBudgetDetailsVos, tableData, 'depart', '', '', 'append')
        }
      })
      this.tableData = tableData
      // console.log('this.tableData', this.tableData)
      // return
      //1.3 计算金额调剂的 ------------------------------ 取最新的一个
      let budgetAdjustVo = data.budgetAdjustVo || []
      let len = budgetAdjustVo.length
      for (let i = len - 1; i >= 0; i--) {
        if (budgetAdjustVo[i].examineStatus != 1) {
          budgetAdjustVo.splice(i, 1)
        }
      }
      if (budgetAdjustVo.length > 0) {
        budgetAdjustVo.sort((a, b) => {
          // new Date(a.datetime).getTime()
          return new Date(b['createDate']).getTime() - new Date(a['createDate']).getTime();
        });
        let newBu = [budgetAdjustVo[0]]
        newBu.forEach(it => {
          if (it.examineStatus == 1) {
            let adjustMoneyVos = it.adjustMoneyVos
            adjustMoneyVos.forEach(item => {
              let id = item.budgetDetailsId
              let checkObj = this.checkByBudgetDetailsId(id, tableData)
              if (checkObj.has) {
                let increaseOrReduce = (item.increaseOrReduce - 0)
                tableData[checkObj.x].children[checkObj.y].money += increaseOrReduce
                tableData[checkObj.x].children[checkObj.y].leftMoney =
                  tableData[checkObj.x].children[checkObj.y].money - tableData[checkObj.x].children[checkObj.y].useMoney

                tableData[checkObj.x].money += increaseOrReduce
                tableData[checkObj.x].leftMoney = tableData[checkObj.x].money - tableData[checkObj.x].useMoney
              } else {
                let id = item.id
                let departmentId = item.budgetDetailsVo.departmentId
                let index = tableData.findIndex(it => it.type == 'depart' && it.departmentId == departmentId)
                let money = (item.budgetDetailsVo.money - 0) + (item.surplus - 0)
                let useMoney = item.budgetDetailsVo.useMoney || 0
                item.budgetDetailsVo.money = money
                if (index > -1) {
                  tableData[index].money += (item.surplus - 0)
                  tableData[index].useMoney = useMoney
                  tableData[index].leftMoney = money - useMoney
                  let children = this.createChildrenObj(item.budgetDetailsVo, 'depart')
                  tableData[index].children.push(children)
                } else {
                  let obj = {
                    id,
                    departmentId,
                    departName: this.getDepartNameById(departmentId),
                    money,
                    useMoney,
                    leftMoney: money - useMoney,
                    type: 'depart',
                    children: [
                      this.createChildrenObj(item.budgetDetailsVo, 'depart')
                    ]
                  }
                  tableData.push(obj)
                }
              }
            })
          }
        })
      }

      //2 计算项目预算列表
      tableData.push({
        id: 2,
        departName: '项目'
      })
      projectData.forEach(it => {
        //2.1 计算项目
        let appendCostBudgetVos = it.appendCostBudgetVos || []
        if (it.examineStatus == 1 || appendCostBudgetVos.length) {
          let budgetDetailsVos = it.budgetDetailsVos
          this.createObj(budgetDetailsVos, tableData, 'project', it.projectId, it.projectName)
          //2.2 计算追加的
          appendCostBudgetVos.forEach(item => {
            //每一个it就是一次追加
            let appendBudgetDetailsVos = item.appendBudgetDetailsVos
            this.createObj(appendBudgetDetailsVos, tableData, 'project', it.projectId, it.projectName)
          })
        }

      })

      this.tableData = tableData
    },
    checkByBudgetDetailsId(id, tableData) {
      let has = false, x, y
      for (let i = 0; tableData[i]; i++) {
        if (tableData[i].children && tableData[i].children.length && tableData[i].type == 'depart') {
          for (let j = 0; tableData[i].children[j]; j++) {
            let adjustMoneyVoId = tableData[i].children[j].budgetDetailsId
            if (adjustMoneyVoId == id) {
              has = true
              x = i
              y = j
              break
            }
          }
        }
      }
      return {
        has, x, y
      }
    },
    createObj(list, tableData, key, projectId, projectName, appendType) {
      list.forEach(item => {
        let id = item.id + this.getRandom(1000, 9999)
        let departmentId = item.departmentId
        // let projectName = item.projectName || ''
        let departName
        if (key == 'project') {
          departmentId = projectId
          departName = projectName
        }
        let index = tableData.findIndex(it => it.type == key && it.departmentId == departmentId)
        let money = item.money || 0
        let useMoney = item.useMoney || 0
        // if (examineStatus == 0) {
        //   money = 0
        //   useMoney = 0
        // }
        if (index > -1) {
          //已经有部门,计算总额
          tableData[index].money += (money - 0)
          tableData[index].useMoney += (useMoney - 0)
          tableData[index].leftMoney += (money - useMoney)
          //判断是否为同一归口
          let budgetTypeId = item.budgetTypeId
          let idx = tableData[index].children.findIndex(el => el.type == key && el.budgetTypeId == budgetTypeId)
          if (idx > -1) {
            tableData[index].children[idx].money += (money - 0)
            tableData[index].children[idx].useMoney += (useMoney - 0)
            tableData[index].children[idx].leftMoney += (money - useMoney)
          } else {
            let childrenObj
            if (key == 'project') {
              childrenObj = this.createChildrenObj(item, key, projectName)
            } else {
              childrenObj = this.createChildrenObj(item, key, '', appendType)
            }
            tableData[index].children.push(childrenObj)
          }
          // let childrenObj = this.createChildrenObj(item)
          // tableData[index].children.push(childrenObj)
        } else {
          //没有部门
          let obj = {
            id,
            departmentId,
            // departName: this.getDepartNameById(departmentId),
            money,
            useMoney,
            leftMoney: money - useMoney,
            type: key,
            // children: [
            //   this.createChildrenObj(item, key)
            // ]
          }
          if (key == 'depart') {
            obj.departName = this.getDepartNameById(departmentId)
            obj.children = [
              this.createChildrenObj(item, key, '', appendType)
            ]
          } else {
            obj.departName = departName
            obj.children = [
              this.createChildrenObj(item, key, projectName)
            ]
          }
          tableData.push(obj)
        }
      });
    },

    createChildrenObj(item, key, proName, appendType) {
      let money = item.money || 0
      let useMoney = item.useMoney || 0
      let projectName = item.projectName || proName || ''
      let budgetDetailsId = item.budgetDetailsId || item.id
      if (appendType == 'append') budgetDetailsId = item.id
      let obj = {
        id: item.budgetTypeVo.id + this.getRandom(1000, 9999),
        budgetTypeId: item.budgetTypeId,
        budgetDetailsId,
        // departmentId,
        departName: item.budgetTypeVo.name,//this.getDepartNameById(departmentId),
        money,
        useMoney,
        leftMoney: money - useMoney,
        type: key,
        projectName,
      }
      if (item.adjustMoneyVo && item.adjustMoneyVo.length) { //初始预算或者追加
        obj.adjustMoneyVoId = item.adjustMoneyVo[0].id
      }
      return obj
    },
    async getDepartByCompanyId(id) { // 获取公司部门架构数据
      await this.$axios.post(
        Api.annualBudget.getDepartByCompanyId,
        {
          data: {
            id//: localstorageGet('companyId')
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
      let index = this.departOptions.findIndex(item => item.id == id)
      if (index > -1) {
        return this.departOptions[index].departmentName
      } else {
        return ''
      }
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        let v = item || 0;
        val += v - 0;
      });
      return val;
    },
    summary(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (index === 0) {
          // 只找第一列放合计
          sums[index] = "合计：";
          return;
        }
        if (column.property === "money" || column.property === "useMoney" || column.property === "leftMoney") {
          let values = data.map(item => item[column.property]);
          sums[index] = "￥" + (this.sumNums(values) - 0).toFixed(6);
        } else {
          sums[index] = "";
        }
      });
      return sums;
    },
    toCompanyDetail() {
      // console.log('ds')
      // console.log('this.params', this.params)
      // return
      let paramStr = JSON.stringify(this.params);
      let type = 'detail'
      this.$router.push({
        path: '/groupBudgetManage/companyBudget/addAnnualBudget',
        name: 'GroupAddAnnualBudget',
        params: { str: paramStr },
        query: { type }
      });
    },
    toBugetDetail(val) {
      let paramStr = JSON.stringify(val), type = 'detail'
      this.$router.push({
        path: '/groupBudgetManage/ProjectBudget/NewBudget',
        name: 'GroupNewBudget',
        params: { str: paramStr },
        query: { type }
      });
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
    margin: 40px 0 0 40px;
  }

  .content {
    padding: 25px;
  }
}

// ::v-deep .cell {
//   font-size: 14px !important;
// }
</style>
