<template>
  <div class="table-container">
    <el-form
    ref="form"
    :rules="rules"
    :model="form">
      <el-table
        :data="form.tableData"
        border
        style="width: 100%"
        :header-cell-style="{ background: '#f5f7fa' }"
        :span-method="arraySpanMethod"
      >
        <!-- 合并表头 -->
        <el-table-column label="调整归口（出）" align="center">
          <el-table-column prop="outSeqNo" label="序号"  align="center">
            <template slot-scope="scope">
              <el-form-item :prop="'tableData.' + scope.$index + '.outSeqNo'" :rules="rules.outSeqNo">
              <el-select :disabled="false" v-model="scope.row.outSeqNo" @change="numberChange(scope, 'old')" placeholder=" ">
                <el-option v-for="n in numberOptions" :key="n.number" :label="n.number" :value="n.companyId"></el-option>
              </el-select>
            </el-form-item>
            </template>
          </el-table-column>
          <el-table-column prop="outYear" label="归口所在年份"  align="center">
            <template slot-scope="scope" >
              <el-date-picker  :disabled="false" v-model="scope.row.outYear" type="year" value-format="yyyy" :clearable="false"  @change="numberChange(scope, 'old')"></el-date-picker>
            </template>
          </el-table-column>
          <el-table-column prop="outBudgetTypeId" label="归口"  align="center">
            <template slot-scope="scope">
            <el-cascader
              placeholder=" "  :disabled="false"
              @change="attributeChange(scope, 'old')"
              :props="{label: 'name', value: 'id', children: 'childrenList'}"
              v-model="scope.row.outBudgetTypeId"
              :options="scope.row.oldFeeOptions"></el-cascader>
            </template>
          </el-table-column>
          <el-table-column prop="outMonth" label="调整月份"  align="center">
            <template slot-scope="scope">
              <el-select  :disabled="false" v-model="scope.row.outMonth" @change="monthChange(scope, 'old')" placeholder=" ">
                <el-option v-for="m in months" :key="m" :label="m" :value="m"></el-option>
              </el-select>
            </template>
          </el-table-column>
          <el-table-column prop="outCostMoney" label="已使用费用"  align="center"></el-table-column>
          <el-table-column prop="outAdjustMoney" label="调出已使用费用"  align="center">
            <template slot-scope="scope">
              <el-form-item :prop="'tableData.' + scope.$index + '.outAdjustMoney'" :rules="rules.outAdjustMoney">
              <el-input  :disabled="false" v-model="scope.row.outAdjustMoney" type="number"></el-input>
            </el-form-item>
            </template>
          </el-table-column>
        </el-table-column>
        <el-table-column label="目标归口（进）" align="center">
          <el-table-column prop="inSeqNo" label="序号" align="center">
            <template slot-scope="scope">
              <el-select  :disabled="false" v-model="scope.row.inSeqNo" @change="numberChange(scope, 'new')" placeholder=" ">
                <el-option v-for="n in numberOptions" :key="n.number" :label="n.number" :value="n.companyId"></el-option>
              </el-select>
            </template>
          </el-table-column>
          <el-table-column prop="inYear" label="归口所在年份" align="center">
            <template slot-scope="scope" >
              <el-date-picker   :disabled="false" v-model="scope.row.inYear" type="year" value-format="yyyy" :clearable="false"  @change="numberChange(scope, 'new')"></el-date-picker>
            </template>
          </el-table-column>
          <el-table-column prop="inBudgetTypeId" label="归口" align="center">
            <template slot-scope="scope">
            <el-cascader
              placeholder=" "  :disabled="false"
              @change="attributeChange(scope, 'new')"
              :props="{label: 'name', value: 'id', children: 'childrenList'}"
              v-model="scope.row.inBudgetTypeId"
              :options="scope.row.newFeeOptions"></el-cascader>
            </template>
          </el-table-column>
          <el-table-column prop="inMonth" label="调整月份" align="center">
            <template slot-scope="scope">
              <el-select  :disabled="false" v-model="scope.row.inMonth" @change="monthChange(scope, 'new')" placeholder=" ">
                <el-option v-for="m in months" :key="m" :label="m" :value="m"></el-option>
              </el-select>
            </template>
          </el-table-column>
          <el-table-column prop="inCostMoney" label="已使用费用" align="center"></el-table-column>
          <el-table-column prop="inAdjustMoney" label="调整后费用" align="center">
            <template slot-scope="scope">
              <el-input :disabled="false" v-model="scope.row.inAdjustMoney" type="number"></el-input>
            </template>
          </el-table-column>
        </el-table-column>
        <el-table-column prop="remark" label="备注" align="center" width="120px">
          <template slot-scope="scope">
            <el-input  :disabled="false" v-model="scope.row.remark" type="textarea"></el-input>
          </template>
        </el-table-column>
        <el-table-column label="操作" width='80px'>
          <template #default="scope">
            <el-button
              v-if="!hasSubmit"
              type="danger"
              size="mini"
              @click="deleteRow(scope.$index)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-form>
    <el-button style="margin-top:7px;float:left" type="primary" @click="addRow" v-if="!hasSubmit">+添加行</el-button>
  </div>
</template>

<script>
export default {
  name: 'costAllocationAdjustment',
  props: ['value', 'propData'],
  inject: ['flowDialog', 'enterpriseExamineDialog'],
  data() {
    return {
      hasSubmit: false,
      numberOptions: [],
      generateForm: null,
      years: ['2025', '2026', '2027', '2028', '2029', '2030'],
      months: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12],
      rules: {
        number: [
          { required: false, message: '姓名不能为空', trigger: 'blur' }
        ],
        outAdjustMoney: [
          { required: false, message: '年龄不能为空', trigger: 'blur' }
        ]
      },
      form: {
        tableData: [{
          outSeqNo: '',
          inSeqNo: '',
          outYear: '',
          inYear: '',
          outBudgetTypeId: [],
          outMonth: '',
          inMonth: '',
          outCostMoney: '',
          inCostMoney: '',
          outAdjustMoney: '',
          inBudgetTypeId: [],
          inAdjustMoney: '',
          remark: '',
          oldFeeOptions: [],
          newFeeOptions: [],
          cost: {}
        }]
      },
      options: []
    };
  },
  created() {
    this.getNumber();
    window.abb = this;
  },
  methods: {
    monthChange(scope, type) {
      // if (!scope.row.outMonth || !scope.row.inMonth) {
      //   return;
      // }
      if (type == 'old') {
        scope.row.outCostMoney = scope.row.cost[scope.row.outMonth] || 0;
      } else {
        scope.row.inCostMoney = scope.row.cost[scope.row.inMonth] || 0;
      }
    },
    deleteRow(index) {
      this.form.tableData.splice(index, 1);
    },
    attributeChange({row}, type) {
      console.log(row, 'a-scope');
      if (!row.outBudgetTypeId || !row.outBudgetTypeId.length) {
        // return;
      }
      const data = {
        budgetId: type == 'old' ? row.outBudgetTypeId?.[row.outBudgetTypeId.length - 1] : row.inBudgetTypeId?.[row.inBudgetTypeId.length - 1]
      };
      this.$axios.post('/web/measuring/api/expenseLedger/findByBudgetId', {data}, res => {
        if (res.isSuccess) {
          console.log(res.data, 'res.data');
          const {cost = {}} = res.data;
          row.cost = cost;
          if (type == 'old') {
            row.outCostMoney = cost[row.outMonth] || 0;
          } else {
            row.inCostMoney = cost[row.inMonth] || 0;
          }
        }
      });
    },
    numberChange(scope, type) {
      console.log(scope, 'n-scope');
      // var selectDate = this.getValue('cell_02')
      // var {value, group, rowIndex, refresh} = arguments[0];
      // if (!scope.row.outYear || !scope.row.outSeqNo) {
      //   return;
      // }
      var transformName = { 1: '(公司归口)', 2: '(月度归口)', 3: '(项目归口)', };
      const data = {
        data: {
          annually: new Date(type == 'old' ? scope.row.outYear : scope.row.inYear).getFullYear(),
          companyId: type == 'old' ? scope.row.outSeqNo : scope.row.inSeqNo,
          stringList: [1, 2],
          listString: [1, 3]
        }
      };
      this.$axios.post('/web/measuring/api/budgetType/list', data, res => {
        let list = res?.data?.dataList || []
        list.sort((a, b) => a.sort - b.sort)
        let departmentList = []
        list.forEach(item => {
          let departmentId = item.departmentId
          if (!departmentId || item.type == 3) departmentId = item.projectId
          let index = departmentList.findIndex(it => it.id == departmentId)
          let name = `${item.name}${transformName[item.type]}`
          let child = {
            id: item.id,
            name
          }
          if (index == -1) {
            departmentList.push({
              id: departmentId,
              name: item.departmentName,
              childrenList: [child]
            })
          } else {
            departmentList[index].childrenList.push(child)
          }
        });
        if (type == 'old') {
          scope.row.oldFeeOptions = departmentList;
          scope.row.outBudgetTypeId = '';
          scope.row.outCostMoney = '';
        } else {
          scope.row.newFeeOptions = departmentList;
          scope.row.inBudgetTypeId = '';
          scope.row.inCostMoney = '';
        };
        console.log(departmentList, 'departmentList--233');
      })
    },
    async getNumber() {
      var {data: {dataList = []}} = await this.$axios.post('/web/companyExpenseBaseData/list',{});
      this.numberOptions = dataList.filter(item => {
        return !!item.number;
      });
    },
    addRow() {
      this.form.tableData.push({
        inSeqNo: '',
        inMonth: '',
        inCostMoney: '',
        inYear: '',
        inBudgetTypeId: [],
        inAdjustMoney: '',
        oldFeeOptions: [],
        outSeqNo: '',
        outYear: '',
        outBudgetTypeId: [],
        outMonth: '',
        outCostMoney: '',
        outAdjustMoney: '',
        remark: '',
        newFeeOptions: [],
        cost: {}
      });
      // console.log(this.tableData, 'this.tableData')
    },
    arraySpanMethod({ rowIndex, columnIndex }) {
      // 合并备注列的单元格
      if (columnIndex === 12) { // 备注列索引
        return [1, 1]; // 合并列
      }
    },
    submitForm(arg) {
      return new Promise((resolve, reject) => {
        this.$refs.form.validate((valid) => {
          if (valid) {
            console.log(this.form.tableData, 'this.form.tableData');
            if (this.form.tableData.length < 1) {
              this.$message.error('请至少添加一行数据!');
              reject(new Error(''));
              return;
            }
            var array = this.form.tableData;
            /*eslint-disable*/
            label: for (let i = 0; i < array.length; i++) {
              const obj = array[i];
              var mes = '',outErr = false,inErr = false;
              var out = ['outSeqNo','outYear','outBudgetTypeId','outMonth','outAdjustMoney'];
              for (let j = 0; j < out.length; j++) {
                if (!obj[out[j]] || obj[out[j]]?.length < 1) {
                  mes = `第${i+1}行数据，归口调出或调进信息不能为空并完整填写!`;
                  outErr = true;
                  break;
                }
              }
              var inArr = ['inSeqNo','inYear','inBudgetTypeId','inMonth','inAdjustMoney'];
              for (let l = 0; l < inArr.length; l++) {
                if (!obj[inArr[l]] || obj[inArr[l]]?.length < 1) {
                  mes = `第${i+1}行数据，归口调出或调进信息不能为空并完整填写!`;
                  inErr = true;
                  break;
                }
              }
              console.log(outErr, inErr, 'outErr, inErr');
              if(outErr && inErr){
                this.$message.error(mes);
                reject(new Error(''));
                return;
              }
              if(outErr && inErr) {break label};
            }
            console.log('到达这里了')
            var dataList = this.form.tableData.map(item => {
              return {
                inSeqNo: item.inSeqNo,
                outSeqNo: item.outSeqNo,
                outYear: item.outYear,
                outDeptId: item.outBudgetTypeId?.join(','),
                outBudgetTypeId: item.outBudgetTypeId?.[item.outBudgetTypeId.length - 1],
                outMonth: item.outMonth,
                inMonth: item.inMonth,
                outCostMoney: item.outCostMoney,
                inCostMoney: item.inCostMoney,
                outAdjustMoney: item.outAdjustMoney || 0,
                inYear: item.inYear,
                inDeptId: item.inBudgetTypeId?.join(','),
                inBudgetTypeId: item.inBudgetTypeId?.[item.inBudgetTypeId.length - 1],
                inAdjustMoney: item.inAdjustMoney || 0,
                remark: item.remark
              };
            });
            this.$axios.post('/web/api/measuring/budget/adjust/upgraded/save', { dataList, batchCode: arg.param.batchCode }, res => {
              if (res.isSuccess) {
                var batchId = res.data?.[0]?.batchId;
                var bizSet = new Set();
                res.data?.forEach(item => {
                  bizSet.add(item.batchId);
                  bizSet.add(item.id);
                });
                this.generateForm.setData({ costAllocationAdjustment: { batchId, tableData: this.form.tableData }});
                arg.param.formDataMongoVo.data.costAllocationAdjustment = { batchId, tableData: this.form.tableData };
                arg.param.data.flowInstanceBizRelevanceList.push({
                  // otherBizId: batchId,
                  otherBizIdList: [...bizSet],
                  otherBiz: 'budget_adjustment'
                });
                console.log(arg, 'arg--gggg');
                console.log(this.generateForm.getValues(), 'this.generateForm.getValues()');
                console.log(this.value, 'this.value-vvvv');
                this.$message.success('提交成功!');
                resolve(res.data);
              } else {
                reject(new Error(res.message));
              }
            });
          } else {
            this.$message.error('表单验证失败!');
            return false;
          }
        });
      });
    },
    init() {
      var t = this.generateForm;
      t.postData = (arg) => {
        if (arg.init) {
          return this.submitForm(arg);
        }
      };
    },
    testTrigger() {
      this.generateForm.triggerEvent('beforeSubmitAndDraft', { init: true, param: { formDataMongoVo: { data: {}}, batchCode: 2333, data: { flowInstanceBizRelevanceList: [] }}});
    },
    getById(batchId) {
      this.$axios.post('/web/api/measuring/budget/adjust/upgraded/findByBatchId', { batchId }, res => {
        if (res.isSuccess) {
        }
      });
    }
  },
  mounted () {
    window.abb = this;
    var draft = document.querySelector('.save_draft_button');
    if (draft) { draft.style.display = 'none'; }
    var print = document.querySelector('.to_print_button');
    if (print) { print.style.display = 'none'; }
    if (this.flowDialog) {
      this.generateForm = this.flowDialog.$refs.generateForm || this.flowDialog.$refs.OtherSteps2.$refs.generateForm;
      if (this.generateForm) {
        this.init();
      }
      var val = this.generateForm.getValue('costAllocationAdjustment');
      if (val && val.batchId && val.tableData) {
        this.form.tableData = val.tableData;
        this.hasSubmit = true;
        this.getById(val.batchId);
      }
    }
    // this.generateForm.formData.config.eventScript.push({ func: `var f = () =>{
    //   console.log(2345678910);
    //   };
    //   f();`, key: "zxfff",name: "zxfff",type: "js"})
    // this.generateForm.formData.eventFunction['zxfff'] = function(e) { console.log(e, 9999999); };
    // this.generateForm.triggerEvent('beforeSubmitAndDraft', { type: 'change' });
    // var fuc = this.generateForm.formData.config.eventScript.find(i => i.name == 'beforeSubmitAndDraft');
    // console.log(fuc, 'fucc')
    // fuc.func = `
    // var f = () =>{
    //   console.log(2345678910);
    //   };
    //   f();
    //   `;
    // this.generateForm.refresh();
    window.setTimeout(() => {
      // this.generateForm.triggerEvent('beforeSubmitAndDraft', { type: 'change' });
      // console.log(this.generateForm.getValues(), 'this.generateForm.getValues()');
      // console.log(this.value, 'this.value-vvvv');
    }, 1000);

    // .remove掉 保存草稿和打印按钮
  }
};
</script>

<style scoped>
.el-table {
  border-color: rgb(165, 160, 160) !important;
  border-right: solid 1px;
  border-bottom: solid 1px;
}
::v-deep .el-table tbody td{
  border-color: rgb(165, 160, 160);
}
::v-deep .el-table thead th{
  border-color: rgb(165, 160, 160) !important;
}
.table-container {
  padding: 20px;
}

.el-table {
  font-size: 14px;
}

.el-table::before {
  height: 0; /* 去除默认底部边框 */
}

.el-table--border {
  border: 1px solid #ebeef5;
}

.el-table--border th {
  border-right: 1px solid #ebeef5;
}

.el-table td {
  padding: 12px 0;
}
</style>