<template>
  <div class="outer">
    <div class="top">
      <!-- <span>部门：</span>&nbsp; -->
      <!-- <el-select v-model="query.depart" placeholder="请选择" style="margin-right:5%" @change="queryData" clearable>
        <el-option v-for="(item, index) in departmentList" :key="index" :label="item.name" :value="item.id">
        </el-option>
      </el-select> -->
      <span>统计时间：</span>&nbsp;
      <el-radio v-model="query.dateType" label="1">年度</el-radio>
      <!-- <el-radio v-model="query.dateType" label="2">月份</el-radio> -->
      <!-- <el-date-picker v-model="query.date" :type="picker['dateType']" :value-format="picker['valueFormat']"
        style="width: 160px;margin-right:5%" :picker-options="expireTimeOption" @change="queryMonthData">
      </el-date-picker> -->
      <el-date-picker v-model="query.date" :type="picker['dateType']" :value-format="picker['valueFormat']"
        style="width: 160px;margin-right:5%" :picker-options="expireTimeOption" @change="changeDate">
      </el-date-picker>
      <el-button type="primary" @click="outPut">导出</el-button>
    </div>
    <div class="content">
      <h4>单位：元</h4>
      <div class="table-content" v-if="tableData.length">
        <el-table :data="tableData" row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }"
          :show-summary="true" :summary-method="summary" id="budgetTableId"
          ref="multipleTable" :max-height="tableMax">
          <el-table-column v-for="(val, index) in col" :key="index + 'fix'" :prop="val['prop']" :label="val['label'] == '部门' ? '部门' : query.date+val['label']"
            :width="val['width']" fixed='left'>
            <template slot-scope="scope">
              <span :class="[{'red':scope.row[val['prop']]<0}]">
                {{ scope.row[val['prop']] | toFix2(val['fix']) | numberWithCommas }}
              </span>
            </template>
          </el-table-column>
          <el-table-column v-for="(v, i) in monthKey" :label="v.label" :key="i + 'month'" :prop="v.prop" width="110">
            <template slot-scope="scope" >
              <span :class="[{'red':scope.row[v['prop']]<0}]">
                {{scope.row[v['prop']] | toFix2(true) | numberWithCommas}}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template slot-scope="scope">
              <div>
                <el-button v-if="scope.row.hasChildren" @click="openItemUseDetail(scope.row)" type="text" size="small">归口使用明细</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <el-empty description="暂无数据" v-else></el-empty>
    </div>
    <!-- 费用明细列表 -->
    <!-- <detailedDialog :detailVisible.sync="detailVisible" v-if="detailVisible" :detailRow="detailRow"
    :isShowBudget="false" :isShowDepartment="false"></detailedDialog> -->
  </div>
</template>

<script>
import Api from '@/api';
// import detailedDialog from './components/detailedDialog.vue';
import { deepClone } from '@/utils/index';
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import numFunc from '@/utils/number'  //重写toFixed
import math from '@/utils/math.js'
Number.prototype.toFixed = numFunc
const numberWithCommas = (x)=>{
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
const monthKey = (() => {
  let keys = [
    { label: '1月预算', prop: 'januaryBudgetTotal' }, { label: '1月已使用', prop: 'januaryCostTotal' }, { label: '1月剩余预算', prop: 'januaryCostLeft',calc:true,minuend:'januaryBudgetTotal',subtrahend:'januaryCostTotal' },
    { label: '2月预算', prop: 'februaryBudgetTotal' }, { label: '2月已使用', prop: 'februaryCostTotal' }, { label: '2月剩余预算', prop: 'februaryCostLeft',calc:true,minuend:'februaryBudgetTotal',subtrahend:'februaryCostTotal' },
    { label: '3月预算', prop: 'marchBudgetTotal' }, { label: '3月已使用', prop: 'marchCostTotal' }, { label: '3月剩余预算', prop: 'marchCostLeft',calc:true,minuend:'marchBudgetTotal',subtrahend:'marchCostTotal' },
    { label: '4月预算', prop: 'aprilBudgetTotal' }, { label: '4月已使用', prop: 'aprilCostTotal' }, { label: '4月剩余预算', prop: 'aprilCostLeft',calc:true,minuend:'aprilBudgetTotal',subtrahend:'aprilCostTotal' },
    { label: '5月预算', prop: 'mayBudgetTotal' }, { label: '5月已使用', prop: 'mayCostTotal' }, { label: '5月剩余预算', prop: 'mayCostLeft',calc:true,minuend:'mayBudgetTotal',subtrahend:'mayCostTotal' },
    { label: '6月预算', prop: 'juneBudgetTotal' }, { label: '6月已使用', prop: 'juneCostTotal' }, { label: '6月剩余预算', prop: 'juneCostLeft' ,calc:true,minuend:'juneBudgetTotal',subtrahend:'juneCostTotal'},
    { label: '7月预算', prop: 'julyBudgetTotal' }, { label: '7月已使用', prop: 'julyCostTotal' }, { label: '7月剩余预算', prop: 'julyCostLeft' ,calc:true,minuend:'julyBudgetTotal',subtrahend:'julyCostTotal'},
    { label: '8月预算', prop: 'augustBudgetTotal' }, { label: '8月已使用', prop: 'augustCostTotal' }, { label: '8月剩余预算', prop: 'augustCostLeft' ,calc:true,minuend:'augustBudgetTotal',subtrahend:'augustCostTotal'},
    { label: '9月预算', prop: 'septemberBudgetTotal' }, { label: '9月已使用', prop: 'septemberCostTotal' }, { label: '9月剩余预算', prop: 'septemberCostLeft',calc:true,minuend:'septemberBudgetTotal',subtrahend:'septemberCostTotal' },
    { label: '10月预算', prop: 'octoberBudgetTotal' }, { label: '10月已使用', prop: 'octoberCostTotal' }, { label: '10月剩余预算', prop: 'octoberCostLeft',calc:true,minuend:'octoberBudgetTotal',subtrahend:'octoberCostTotal' },
    { label: '11月预算', prop: 'novemberBudgetTotal' }, { label: '11月已使用', prop: 'novemberCostTotal' }, { label: '11月剩余预算', prop: 'novemberCostLeft' ,calc:true,minuend:'novemberBudgetTotal',subtrahend:'novemberCostTotal'},
    { label: '12月预算', prop: 'decemberBudgetTotal' }, { label: '12月已使用', prop: 'decemberCostTotal' }, { label: '12月剩余预算', prop: 'decemberCostLeft',calc:true,minuend:'decemberBudgetTotal',subtrahend:'decemberCostTotal' },
  ]
  // for(let i=1;i<=12;i++){
  //   [{label:'月预算',prop:'monthBudget'},{label:'月已使用',prop:'monthUse'},{label:'月剩余预算',prop:'monthLeft'}].forEach(item=>{
  //     keys.push({
  //       label:`${i}${item.label}`,
  //       prop:`${item.prop}${i}`,
  //       month:('0'+i).substr(-2,2),
  //       num:i,
  //     })
  //   })
  // }
  return keys
})()
export default {
  name: 'BudgetCostDetail',
  // components: { detailedDialog },
  data() {
    return {
      detailVisible: false,
      query: {
        date: new Date().getFullYear(),
        dateType: '1',
        depart: localstorageGet('userDepartmentId'),
      },
      companyId: localstorageGet('companyId'),
      tableData: [],
      departOptions: [],
      col: [
        { prop: 'departName', label: '部门', width: '250px' },
        { prop: 'money', label: `年预算`, width: '150px', fix: true },
        { prop: 'costTotal', label: '年已使用', width: '150px', fix: true },
        { prop: 'leftMoney', label: '年剩余', width: '150px', fix: true },
        // { prop: 'monthControl', label: '当月计划控制', width: '150px', fix: true },
        // { prop: 'nextMonth', label: '下月预算', width: '100px', fix: true },
        // { prop: 'actualMonth', label: '当月实际', width: '150px', fix: true },
        // { prop: 'monthBudget', label: '当月预算', width: '150px', fix: true },
        // { prop: 'budgetDiff', label: '当月实际比预算结余', width: '150px', fix: true },
        // { prop: 'planDiff', label: '当月实际比计划结余', width: '150px', fix: true },
        // { prop: 'date', label: '年度/月份' }
      ],
      monthKey,
      detailRow: {},
      expireTimeOption: {},
      departmentList:[],
      companyIds:[], // 拥有得公司权限
      departmentIds:[]
    }
  },
  computed: {
    picker() {
      if (this.query.dateType == '1') {
        this.query.date = String(this.query.date).substr(0, 4);
        return {
          dateType: 'year',
          valueFormat: 'yyyy'
        };
      } else {
        // let curruntMonth = new Date().getMonth() + 1
        // curruntMonth = curruntMonth >= 10 ? curruntMonth : '0' + curruntMonth
        // this.query.date = `${this.query.date}-${curruntMonth}`;
        return {
          dateType: 'month',
          valueFormat: 'yyyy-MM'
        };
      }
    },
    tableMax() {
      let winHeight = window.innerHeight
      let tHeight = winHeight - 50 - 50 - 90 - 80
      return tHeight
    },
  },
  filters: {
    toFix2(val, isFix) {
      if (!isFix) {
        return val
      }
      if (val != '' && (val - 0) == (val - 0)) {
        let v = val - 0
        return v.toFixed(2)
      }
      if ((val - 0) !== (val - 0)) {
        return val
      }
      if (val == '') {
        return '0.00'
      }
    },
    numberWithCommas(x) {
      return numberWithCommas(x)
    }
  },
  async created() {
    // this.getCompanyBudget()
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
          current:1,
          size:99
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
              if(item.type=='2'){
                this.companyIds.push(item.companyId)
                this.departmentIds.push(item.departmentId)
              }
              // else{
              //   this.departmentIds.push(item.departmentId)
              // }
            })
            if(this.companyIds.length>0){
              this.getCompanyBudget()
            }
            
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    //修改日期，查询当前公司当年的预算
    changeDate(){
      // changeDate
      this.getCompanyBudget()
    },
    getCompanyBudget() {
      let data = {
        companyIds:this.companyIds,
        stringList:[1,6],
        budgetTime:`${this.query.date}-01-01 00:00:00`

      }//deepClone(this.query)
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data,//: this.query,
          pagination: true,
          grouping:false,
          detailed:false,
          projectDetailed:false,
          monthlyDetailed:false,
          costExistence:false
        },
        res => {
          if (res.isSuccess) {
            const data = res.data?.dataList || [];
            let ids = []
            if(data.length){
              data.forEach(el=>{
                ids.push(el.id)
              })
              this.getDepartmentList().then(() => {
                this.getlistByBudgetId(ids.join(','))
              })
            }else{
              this.tableData = []
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    _numberWithCommas(x){
      return numberWithCommas(x)
    },
    getlistByBudgetId(ids) {
      let idArr = ids.split(',')
      let data = {
        data: {},
        grouping: true,
        ids: idArr,
        projectId: ''
      }
      this.$axios.post(Api.budgetManage.listByBudgetId, data, res => {
        if (res.isSuccess) {
          let data = res.data
          let departmentBudget = [], 
          projectBudget = [{departName:'项目',id:'project'}]
          for (let key in data) {
            let detailData = data[key]
            detailData.forEach(el => {
              let type = el.type
              let currentTarget = type == 1||6 ? departmentBudget : projectBudget
              let index = currentTarget.findIndex(item => item.id == el.departmentId)
              //没有找到部门，新建部门，再把归口算到部门里面去
              let expenseLedgerVo = el.budgetTypeVo?.expenseLedgerVo || {}
              if (!Object.keys(expenseLedgerVo).length) {
                monthKey.forEach(item => {
                  let mkey = item.prop
                  expenseLedgerVo[mkey] = 0
                })
              }
              let money = math.multiply(el.money, 10000),  //这个单位是万元，需要转换成元
                costTotal = el.budgetTypeVo?.expenseLedgerVo?.costTotal || 0,
                leftMoney = money - costTotal
              let sort = el.sort
              if (index == -1) {
                let departName = el.budgetTypeVo.departmentName || this.departmentList.find(item=>item.id == key)?.name || ''
                departName = departName == '公司领导' ? '公司固定费用':departName

                let departmentObj = {
                  id: el.departmentId,
                  departName,//: el.budgetTypeVo.departmentName || this.departmentList.find(item=>item.id == key)?.name || '',
                  money,
                  costTotal,
                  type,
                  leftMoney,
                  sort
                }
                let childrenObj = {
                  id:el.id,
                  budgetTypeId:el.budgetTypeId,
                  departName:el.budgetTypeVo.name,
                  departmentId:el.departmentId,
                  companyId:el.budgetTypeVo.companyId,
                  money,
                  costTotal,
                  type,
                  leftMoney,
                  sort,
                  hasChildren:true
                }
                this.monthKey.forEach(item => {
                  let mkey = item.prop
                  departmentObj[mkey] = expenseLedgerVo[mkey] || 0
                  childrenObj[mkey] = expenseLedgerVo[mkey] || 0
                  if(item.calc){
                    let minuendKey = item.minuend,subtrahendKey = item.subtrahend
                    let monthLeft = (expenseLedgerVo[minuendKey] || 0) - (expenseLedgerVo[subtrahendKey] || 0)
                    childrenObj[mkey] = monthLeft
                    departmentObj[mkey] = monthLeft
                  }
                })
                departmentObj.children = [childrenObj]
                currentTarget.push(departmentObj)
              }else{
                currentTarget[index].money = math.add(currentTarget[index].money,money)
                currentTarget[index].costTotal = math.add(currentTarget[index].costTotal,costTotal)
                currentTarget[index].leftMoney = math.add(currentTarget[index].leftMoney,leftMoney)
                let childrenObj = {
                  id:el.id,
                  budgetTypeId:el.budgetTypeId,
                  departName:el.budgetTypeVo.name,
                  departmentId:el.departmentId,
                  companyId:el.budgetTypeVo.companyId,
                  money,
                  costTotal,
                  type,
                  sort,
                  leftMoney,
                  hasChildren:true
                }
                this.monthKey.forEach(item => {
                  let mkey = item.prop
                  childrenObj[mkey] = expenseLedgerVo[mkey] || 0
                  if(item.calc){
                    let minuendKey = item.minuend,
                        subtrahendKey = item.subtrahend
                    let monthLeft = (expenseLedgerVo[minuendKey] || 0) - (expenseLedgerVo[subtrahendKey] || 0)
                    childrenObj[mkey] = monthLeft
                    currentTarget[index][mkey] = math.add(currentTarget[index][mkey],childrenObj[mkey])
                  }else{
                    currentTarget[index][mkey] = math.add(currentTarget[index][mkey],childrenObj[mkey])
                  }
                })
                currentTarget[index].children.push(childrenObj)
              }
            })
          }
          this.tableData = []
          console.log(departmentBudget,'departmentBudget+++++++++',this.departmentIds)
          console.log(projectBudget,'projectBudget+++++++++')
          departmentBudget.sort((a,b)=>a.sort - b.sort)
          departmentBudget = departmentBudget.filter(item=>this.departmentIds.includes(item.id))
          console.log(departmentBudget,'departmentBudget+++++++++')
          projectBudget.sort((a,b)=>a.sort - b.sort)
          projectBudget = projectBudget.filter(item=>this.departmentIds.includes(item.id)||item.id=='project')
          console.log(projectBudget,'projectBudget+++++++++')
          this.tableData = this.tableData.concat(departmentBudget)
          console.log(this.tableData,'this.tableData+++++++++')
          this.orginData = deepClone(this.tableData)
          console.log(this.orginData,'this.orginData+++++++++',)
        }
      })
    },
    // 费用类型列表(归口)
    getDepartmentList() {
      return new Promise((resolve, reject) => {
        let data = {
          data: {
            annually: this.query.date,//new Date().getFullYear(),
            companyIds: this.companyIds
          }
        }
        this.$axios.post(Api.budgetManage.getBudgetList, data, res => {
          let list = res?.data?.dataList || []
          let departmentList = []

          list.forEach(item => {
            let departmentId = item.departmentId
            if (departmentList.findIndex(el => el.id == departmentId) == -1) {
              departmentList.push({
                id: departmentId,
                name: item.departmentName == '公司领导' ? '公司固定费用' : item.departmentName,
                type: item.type,
              })
            }
          })
          this.departmentList = departmentList
          resolve()
        })
      })
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        let v = item || 0;
        // val += v - 0;
        val = math.add(val, Number(v));
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
        if (column.property === "money"
          || column.property === "useMoney"
        ) {
          let values = data.map(item => item[column.property]);
          let total = this.sumNums(values)
          if (total != total) total = ''
          if (total) {
            sums[index] = "￥" + this._numberWithCommas(this.sumNums(values).toFixed(2))
          }
        } else {
          sums[index] = "";
        }
      });
      return sums;
    },

    // ---------排序方法结束------------
    outPut() {
      // return
      let tdStr = '';
      let keyArr = [];
      let col = this.col.concat(this.monthKey)
      col.forEach(item => {
        tdStr += `<td style="font-weight:700;font-size:15px;background:rgb(245,247,250);height:35px;line-height:35px;">${item.label}</td>`;
        keyArr.push(item.prop);
      });
      let head = `<tr>${tdStr}</tr>`;
      let trStr = '';
      this.tableData.forEach(item => {
        let tdStr = '';
        col.forEach(el => {
          let labelVal = item[el.prop] == undefined ? '' : item[el.prop];
          if (item.useMoney !== undefined) {
            tdStr += `<td style="text-align:center;font-weight:700;font-size:14px;">${labelVal + '\t'}</td>`;
          } else {
            tdStr += `<td style="font-weight:700;font-size:14px;">${labelVal + '\t'}</td>`;
          }
        });
        trStr += `<tr>${tdStr}</tr>`;
        let children = item.children || []

        children.forEach(it => {
          let strs = ''
          col.forEach(el => {
            let labelVal = it[el.prop] == undefined ? '' : it[el.prop];
            if (el.prop == 'departName') {
              strs += `<td style="text-align:right;">${labelVal + '\t'}</td>`;
            } else {
              strs += `<td style="text-align:center;">${labelVal + '\t'}</td>`;
            }

          });
          trStr += `<tr>${strs}</tr>`;
        })
      });
      let str = head + trStr;

      // Worksheet名
      let worksheet = 'Sheet1';
      let uri = 'data:application/vnd.ms-excel;base64,';

      // 下载的表格模板数据
      let template = `<html xmlns:o="urn:schemas-microsoft-com:office:office"
                  xmlns:x="urn:schemas-microsoft-com:office:excel"
                  xmlns="http://www.w3.org/TR/REC-html40">
                  <head><!--[if gte mso 9]><xml><x:ExcelWorkbook><x:ExcelWorksheets><x:ExcelWorksheet>
                    <x:Name>${worksheet}</x:Name>
                    <x:WorksheetOptions><x:DisplayGridlines/></x:WorksheetOptions></x:ExcelWorksheet>
                    </x:ExcelWorksheets></x:ExcelWorkbook></xml><![endif]-->
                    </head><body><table>${str}</table></body></html>`;
      // 下载模板
      let a = document.createElement('a');
      let companyName = localstorageGet('companyName')
      a.download = `${companyName}-年度预算-部门使用汇总.xls`;
      a.href = uri + base64(template);
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);

      function base64(s) { return window.btoa(unescape(encodeURIComponent(s))); }
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
    margin: 40px 0 0 40px;
  }

  .content {
    padding: 25px;
    height: calc(100% - 90px);
  }
}

::v-deep .el-table__body-wrapper {
  z-index: 2;
}

::v-deep {
  // .el-table__fixed-right {
  //   box-shadow: none !important;
  // }

  // .el-table__fixed {
  //   box-shadow: none !important;
  // }
  .el-table__row--level-0{
    font-weight: 600;
    color:#000;
  }
  .el-table td.el-table__cell{
    color:#000;
  }
  .red{
    color:#F56C6C;
  }
  .el-table th.el-table__cell{
    color: #000;
  }
  .el-table--mini .el-table__cell{
    color: #000;
  }
}

</style>
