<!--  -->
<template>
  <div class="outer">
    <div class="top">
      <span>
        <el-radio v-model="radio" label="1">归口视角</el-radio>
        <el-radio v-model="radio" label="2">年度视角</el-radio>
      </span>
      <span style="margin-left:20px;">费用预算类型：</span>&nbsp;
      <el-select v-model="selectName" placeholder="请选择" clearable @change="changeName" v-if="radio == 1">
        <el-option v-for="(item, key) in originData" :key="key" :label="item.name" :value="item.name">
        </el-option>
      </el-select>
      <el-select v-model="selectName" placeholder="请选择" clearable @change="changeName" v-else>
        <el-option v-for="(item, key) in originDataYear" :key="key" :label="item.name" :value="item.name">
        </el-option>
      </el-select>
      <span style="margin-left:20px;">统计时间：</span>&nbsp;
      <el-date-picker v-model="annual" type="year" placeholder="选择年份" value-format="yyyy" style="margin-right:30px;"
        @change="searchByQuery" clearable :disabled="radio == 2">
      </el-date-picker>
    </div>
    <div class="content" style="padding:25px;">
      <el-table :data="tableData" :show-summary="true" style="margin-top:20px;" v-show="radio == 1">
        <el-table-column prop="name" label="费用预算类型（一级）" width="220px" :show-overflow-tooltip="true"> </el-table-column>
        <el-table-column prop="money" label="预算金额">
          <template slot-scope="scope">
            {{ (scope.row.money - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="useMoney" label="已使用预算">
          <template slot-scope="scope">
            {{ (scope.row.useMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="leftMoney" label="剩余预算">
          <template slot-scope="scope">
            {{ (scope.row.leftMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <!-- <el-table-column prop="yearBudget" label="当年预算"> </el-table-column>
        <el-table-column prop="yearUse" label="当年已使用"> </el-table-column>
        <el-table-column prop="yearLeft" label="当年剩余预算"> </el-table-column> -->
        <!-- <el-table-column prop="createDate" label="创建时间" width="180px"> </el-table-column> -->
        <el-table-column fixed="right" label="操作" width="180">
          <template slot-scope="scope">
            <el-button @click="openItemUseDetail(scope.row)" type="text" size="small">查看明细</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-table :data="yearTableData" :show-summary="true" style="margin-top:20px;" row-key="id"
        :tree-props="{ children: 'children', hasChildren: 'hasChildren' }" v-show="radio == 2">
        <el-table-column prop="name" label="费用预算类型（一级）" width="220px" :show-overflow-tooltip="true">
        </el-table-column>
        <el-table-column prop="money" label="当年预算">
          <template slot-scope="scope">
            {{ (scope.row.money - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="useMoney" label="当年已使用">
          <template slot-scope="scope">
            {{ (scope.row.useMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <el-table-column prop="leftMoney" label="当年剩余预算">
          <template slot-scope="scope">
            {{ (scope.row.leftMoney - 0).toFixed(6) }}
          </template>
        </el-table-column>
        <!-- <el-table-column prop="createDate" label="创建时间" width="180px"> </el-table-column> -->
        <el-table-column fixed="right" label="操作" width="180">
          <template slot-scope="scope">
            <el-button @click="openItemUseDetail(scope.row)" type="text" size="small" v-if="!scope.row.children">查看明细
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 费用明细列表 -->
    <detailedDialog :detailVisible.sync="detailVisible" v-if="detailVisible" :detailRow="detailRow"></detailedDialog>
  </div>
</template>

<script>
import Api from '@/api';
import { deepClone } from '@/utils/index';
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import detailedDialog from '../../CompanyBudget/components/detailedDialog.vue';
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
export default {
  name: '',
  data() {
    return {
      radio: '1',
      options: [],
      value: '',
      annual: '',
      projectId: '',
      selectName: '',
      tableData: [],
      yearTableData: [
      ],
      detailVisible: false,
      detailRow: {},
      originData: [],
      companyId: '',
      originDataYear: []
    };
  },
  components: { detailedDialog },
  watch: {
    radio(a) {
      this.selectName = ''
      if (a == 2) {
        let query = {
          type: 3,
          companyId: '',//this.companyId,
          projectId: this.projectId,
          endTime: ''
        };
        this.getProjectBudget(query);
      } else {
        this.tableData = deepClone(this.originData)
      }
    }
  },
  async created() {
    if (this.$route.meta && this.$route.meta.type && this.$route.meta.type == 'group') { //企业空间
      let paramsStr = this.$route.params.str;
      if (!paramsStr) {
        paramsStr = localstorageGet('bugetDetailsParam');
      }
      if (paramsStr) {
        localstorageSet('bugetDetailsParam', paramsStr);
        let params = this.params = JSON.parse(paramsStr);
        this.projectId = params.projectId;
        this.annual = params.createDate.substr(0, 4);
        this.companyId = params.companyId;
        await this.getDepartByCompanyId(this.companyId)
        if (params.examineStatus || params.appendCostBudgetVos.length) {
          this.processData(params);
        }
      }
      this.$once('hook:beforeDestroy', () => {
        localstorageRemove('bugetDetailsParam');
      });
    } else {
      //项目空间
      //获取当前项目projectid
      let projectId = this.projectId = this.$store.state.user.projectId
      let companyId = this.companyId = this.$store.state.user.companyId
      await this.getDepartByCompanyId(this.companyId)
      this.getProjectBudgetById(projectId, companyId)
    }
  },
  mounted() {
  },
  methods: {
    async getDepartByCompanyId(companyId) {
      await this.$axios.post(
        Api.annualBudget.getDepartByCompanyId,
        {
          data: {
            id: companyId
          }
        },
        res => {
          if (res.isSuccess) {
            const data = res.data;
            if (data && data.departmentVos) {
              data.departmentVos.forEach(item => {
                if (item.departmentName == '公司领导') item.departmentName = '公司固定费用'
              })
              this.originDepart = data.departmentVos;
            }
          }
        }
      );
    },
    getDepartNameById(id) {
      let index = this.originDepart.findIndex(el => el.id == id)
      if (index > -1) {
        return this.originDepart[index]['departmentName']
      } else {
        return ''
      }

    },
    getProjectBudgetById(projectId, companyId) {
      let query = {
        annual: "",
        budgetTime: "",
        companyId,
        endTime: "",
        projectId,
        type: "3"
      }
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query,
        },
        res => {
          if (res.isSuccess) {
            let data = res.data || {}
            if (data.dataList && data.dataList.length) {
              let params = this.params = data.dataList[0]
              this.projectId = params.projectId;
              this.annual = params.createDate.substr(0, 4);
              this.companyId = params.companyId;
              if (params.examineStatus || params.appendCostBudgetVos.length) {
                this.processData(params);
              }
            }
          } else {
            this.$message.error(res.message)
          }
        }
      );
    },
    openItemUseDetail(row) {
      this.detailRow = row;
      this.detailVisible = true;
    },
    changeName() {
      let name = this.selectName;
      if (name) {
        if (this.radio == 2) {
          let copyData = deepClone(this.originYearTableDate)
          copyData.forEach(el => {
            let children = el.children || []
            let obj = []
            for (let i = 0; children[i]; i++) {
              if (children[i].name == name) obj.push(children[i])
            }
            el.children = obj
          })
          this.yearTableData = copyData
        } else {
          let tb = this.originData.find(item => item.name == name);
          if (tb) {
            this.tableData = [tb];
          } else {
            this.tableData = [];
          }
        }
      } else {
        if (this.radio == 2) {
          this.yearTableData = deepClone(this.originYearTableDate);
        } else {
          this.tableData = deepClone(this.originData);
        }

      }
      // console.log('originData', this.originData)
    },
    getProjectBudget(query) {
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        },
        res => {
          if (res.isSuccess) {
            let data = res.data || {};
            let list = this.list = data.dataList || [];
            if (list.length) {
              if (this.radio == 1) {
                let params = list[0]
                if (params.examineStatus == 1 || params.appendCostBudgetVos.length) {
                  this.processData(params);
                }
              } else {
                this.processDataByYear(list);
              }
            } else {
              this.tableData = [];
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    processDataByYear(list) {
      // console.log('list', list)
      let yearTableData = [];
      list.forEach(it => {
        let obj = {
          id: it.id,
          name: it.createDate.substr(0, 4),
          children: []
        };
        if (it.examineStatus == 1 || it.appendCostBudgetVos.length) {
          let companyBudgetVos = it.companyBudgetVos[0]
          if (companyBudgetVos) {
            let budgetDetailsVos = companyBudgetVos.budgetDetailsVos;
            budgetDetailsVos.forEach(item => {
              let departmentId = item.departmentId;
              let money = item.money || 0;
              let useMoney = item.useMoney || 0;
              let leftMoney = money - useMoney;
              let departmentName = this.getDepartNameById(item.budgetTypeVo.departmentId)
              let childrenObj = {
                id: item.id + this.getRandom(1000, 9999),
                budgetTypeId: item.budgetTypeId,
                budgetDetailsId: item.id,
                name: `${departmentName} / ${item.budgetTypeVo.name}`,
                money,
                useMoney,
                leftMoney,
                departmentId,
                companyId: this.params.companyId
              };
              obj.children.push(childrenObj);
            });
          }
        }
        // yearTableData.push(obj)
        // 追加
        let appendCostBudgetVos = it.appendCostBudgetVos;
        appendCostBudgetVos.forEach(el => {
          let companyBudgetVos = el.companyBudgetVos[0];
          if (el.examineStatus == 1 && companyBudgetVos) {
            let appendBudgetDetailsVos = companyBudgetVos.appendBudgetDetailsVos
            appendBudgetDetailsVos.forEach(item => {
              let budgetTypeId = item.budgetTypeId;
              let money = item.money || 0;
              let useMoney = item.useMoney || 0;
              let leftMoney = money - useMoney;
              let index = obj.children.findIndex(it => it.budgetTypeId == budgetTypeId);// this.checkById(budgetDetailsId, yearTableData)
              if (index > -1) {
                obj.children[index].money += money;
                obj.children[index].useMoney += useMoney;
                obj.children[index].leftMoney += leftMoney;
              } else {
                let departmentId = item.departmentId;
                let departmentName = this.getDepartNameById(item.budgetTypeVo.departmentId)
                let childrenObj = {
                  id: item.budgetDetailsId + this.getRandom(1000, 9999),
                  budgetDetailsId: item.budgetDetailsId,
                  budgetTypeId: item.budgetTypeId,
                  name: `${departmentName} / ${item.budgetTypeVo.name}`,
                  money,
                  useMoney,
                  leftMoney,
                  departmentId,
                  companyId: this.params.companyId
                };
                obj.children.push(childrenObj);
              }
            });
          }
        });
        yearTableData.push(obj);
      });
      this.yearTableData = yearTableData;
      this.originYearTableDate = deepClone(this.yearTableData)
      this.originDataYear = []
      this.originYearTableDate.forEach(item => {
        let children = item.children
        children.forEach(el => {
          this.originDataYear.push(el)
        })
      })
    },
    processData(list) {
      let tableData = [];
      // 初始化预算
      if (list.examineStatus == 1 || list.appendCostBudgetVos.length) {
        let budgetDetailsVos = list.budgetDetailsVos || [];
        budgetDetailsVos.forEach(item => {
          let departmentId = item.departmentId;
          let money = item.money || 0;
          let useMoney = item.useMoney || 0;
          let leftMoney = money - useMoney;
          let departmentName = this.getDepartNameById(item.budgetTypeVo.departmentId)
          let obj = {
            name: `${departmentName} / ${item.budgetTypeVo.name}`,
            budgetDetailsId: item.id,
            budgetTypeId: item.budgetTypeId,
            departmentName: this.getDepartNameById(item.budgetTypeVo.departmentId),
            money,
            useMoney,
            leftMoney,
            yearBudget: '',
            yearUse: '',
            yearLeft: '',
            departmentId,
            companyId: this.params.companyId
          };
          tableData.push(obj);
        });
      }
      // 追加公司当年预算 找到相同归口，金额补充相加

      // 追加的预算
      let appendCostBudgetVos = list.appendCostBudgetVos || [];
      appendCostBudgetVos.forEach(it => {
        if (it.examineStatus == 1) {
          let appendBudgetDetailsVos = it.appendBudgetDetailsVos;
          appendBudgetDetailsVos.forEach(item => {
            let budgetTypeId = item.budgetTypeId;
            let money = item.money || 0;
            let useMoney = item.useMoney || 0;
            let leftMoney = money - useMoney;
            let index = tableData.findIndex(el => el.budgetTypeId == budgetTypeId);
            if (index > -1) {
              tableData[index].money += (money - 0);
              tableData[index].useMoney += (useMoney - 0);
              tableData[index].leftMoney += (leftMoney - 0);
            } else {
              let departmentName = this.getDepartNameById(item.budgetTypeVo.departmentId)
              let obj = {
                name: `${departmentName} / ${item.budgetTypeVo.name}`,
                budgetDetailsId: item.budgetDetailsId,
                budgetTypeId: item.budgetTypeId,
                departmentId: item.departmentId,
                companyId: this.params.companyId,
                money,
                useMoney,
                leftMoney,
                yearBudget: '',
                yearUse: '',
                yearLeft: ''
              };
              tableData.push(obj);
            }
          });
        }
      });
      this.originData = deepClone(tableData);
      this.tableData = tableData;
    },
    getRandom(min, max) {
      return Math.floor(Math.random() * (max - min + 1)) + min;
    },
    searchByQuery() {
      let query = {
        type: 3,
        companyId: this.companyId,
        projectId: this.projectId,
        endTime: `${this.annual}-08-01 12:30:00`
      };
      if (!this.annual) query.endTime = ''
      this.getProjectBudget(query);
    },
    add() {
      this.$router.push({
        path: '/groupBudgetManage/ProjectBudget/NewBudget'
      });
    },
  }
};

</script>
<style lang='scss' scoped>
// ::v-deep .el-table__body-wrapper.is-scrolling-none {
//   // height: 73vh;
//   height: calc(72vh);
// }
::v-deep .el-date-editor.el-input,
::v-deep .el-date-editor.el-input__inner {
  width: 180px !important;
}

.outer {
  background: white;
  display: flow-root;
  height: 100%;

  .top {
    margin: 40px 0 0 40px;
  }
}
</style>
