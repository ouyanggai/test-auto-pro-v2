<!--
 * @Descripttion: 预算卡片
 * @Author: liufuze
-->
<template>
  <div>
    <el-card class="box-card" style="margin-top:30px">
      <div class="box card-header-title">
        <h3>
          {{ config['cardTitle'] }}
        </h3>
        <el-button style="margin-left:20px" type="primary" icon="el-icon-plus" @click="addDepartPlan"
          v-if="config['addButton']">新增</el-button>
        <div class="flex-right">
          <el-button type="primary" icon="el-icon-search" @click="showTemplate"
            v-if="config['selectTempButton']">选择模板</el-button>
          <el-button style="margin-left:20px" type="primary" plain icon="el-icon-view" @click="preview"
            v-if="config['previewButton']">预览</el-button>
        </div>
      </div>

      <el-form :model="{ datas }" :rules="subRule" ref="subForm">
        <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;" shadow="hover">
          <template slot="header">
            <div class="card-title" @click.stop.prevent="() => { }">
              <el-form-item :prop="'datas.' + index + '.departId'" :rules="subRule.departId">
                <span style="color: rgb(245, 108, 108);">*</span> <el-select v-model="val.departId" placeholder="请选择"
                  @change="departChange" style="margin-right:10px;">
                  <el-option v-for="item in config.departOptions" :key="item.id" :label="item.name" :value="item.id"
                    :disabled="item.hasSelect">
                  </el-option>
                  <div class="line" v-if="config.departOptions.length"></div>
                  <el-option label="+自定义" value="customIpunt"></el-option>
                </el-select>
              </el-form-item>
              <el-form-item :prop="'datas.' + index + '.customDepart'" :rules="subRule.customDepart"
                v-if="val.departId == 'customIpunt'">
                <el-input v-model="val.customDepart"></el-input>
              </el-form-item>
            </div>
          </template>
          <div class="el-icon-close" @click="deleteDepartPlan(index)" v-show="datas.length > 1">
          </div>
          <div class="el-icon-top" @click="upMove(index)" title="上移" v-show="index > 0">
          </div>
          <el-collapse v-model="val['activeNames']">
            <el-collapse-item name="1">
              <el-form :model="{ val: val['budget'] }" :rules="tableRule" ref="tableForm">
                <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border default-expand-all
                  row-key="id" :tree-props="{ children: 'children', hasChildren: 'hasChildren' }">
                  <el-table-column label="编号" width="65px">
                    <template slot-scope="scope" v-if="!scope.row.isChildren" align="center">
                      {{ scope.row.index }}
                    </template>
                  </el-table-column>
                  <el-table-column prop="budgetType" width="180px">
                    <template slot="header">
                      <span style="color: rgb(245, 108, 108);">*</span><span>费用预算类型</span>
                    </template>
                    <template slot-scope="scope">
                      <el-form-item :prop="'val.' + getPropPosi(scope.row.id, val['budget']) + '.budgetType'"
                        :rules="tableRule.budgetType" class="children-row">
                        <div v-if="config.tableInfo.budgetType && config.tableInfo.budgetType.isSelect">
                          <el-select v-model="scope.row.budgetType" placeholder="请选择" @change="departChange"
                            style="width:120px;" v-if="scope.row.budgetType != 'newBudget'"
                            :disabled="config.tableInfo.budgetType && config.tableInfo.budgetType.disabled">
                            <el-option v-for="item in config.departOptions" :key="item.id" :label="item.name"
                              :value="item.id" :disabled="item.hasSelect">
                            </el-option>
                            <div class="line" v-if="config.departOptions.length"></div>
                            <el-option label="+新增" value="newBudget" style="color: rgb(24,144,255);"></el-option>
                          </el-select>
                          <el-input v-model="scope.row.customBudgetType"
                            v-show="scope.row.budgetType == 'newBudget'"></el-input>
                        </div>
                        <el-input v-model="scope.row.budgetType"
                          :disabled="config.tableInfo.budgetType && config.tableInfo.budgetType.disabled"
                          v-else></el-input>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="使用年初预算（万）"
                    v-if="config.tableInfo.useBudget && config.tableInfo.useBudget.isShow">
                    <template slot-scope="scope">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column label=""
                    v-if="config.tableInfo.appendToBudget && config.tableInfo.appendToBudget.isShow">
                    <template slot="header">
                      <span style="color: rgb(245, 108, 108);">*</span><span>追加至当年公司预算(万)</span>
                    </template>
                    <template slot-scope="scope">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column label="本项目费用预算(万)"
                    v-if="config.tableInfo.projectBudgetMoney && config.tableInfo.projectBudgetMoney.isShow">
                    <template slot="header">
                      <span style="color: rgb(245, 108, 108);">*</span><span>本项目费用预算(万)</span>
                    </template>
                    <template slot-scope="scope">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column prop="relateProjId" label="是否关联项目" :width="config.tableInfo.relateProj['width']"
                    v-if="config.tableInfo.relateProj && config.tableInfo.relateProj.isShow">
                    <template slot-scope="scope">
                      <div>
                        <span v-if="config.tableInfo.relateProj && config.tableInfo.relateProj.isShowRadio"
                          style="margin-right:10px;">
                          <el-radio v-model="scope.row.isRelateProj" :label="true"
                            @change="v => radioChange(v, index, scope.$index)">是
                          </el-radio>
                          <el-radio v-model="scope.row.isRelateProj" :label="false"
                            @change="v => radioChange(v, index, scope.$index)">
                            否
                          </el-radio>
                        </span>
                        <el-select v-model="scope.row.relateProjId" placeholder="请选择" :disabled="!scope.row.isRelateProj">
                          <el-option v-for="item in config.projectOptions" :key="item.id" :label="item.name"
                            :value="item.id">
                          </el-option>
                        </el-select>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column v-if="config.tableInfo.budgetMoney && config.tableInfo.budgetMoney.isShow"
                    :width="config.tableInfo.budgetMoney.width" prop="budgetMoney">
                    <template slot="header">
                      <span style="color: rgb(245, 108, 108);">*</span><span>预算金额(万)</span>
                    </template>
                    <template slot-scope="scope">
                      <el-form-item
                        :prop="'val.' + getPropPosi(scope.row.id, val['budget']) + '.' + config.tableInfo.budgetMoney.prop"
                        :rules="tableRule.budgetMoney">
                        <el-input v-model="scope.row['budgetMoney']" :controls="false" @focus="selectText($event)"
                          @input="handleInput($event, scope.row, 'budgetMoney')"
                          @blur="handleBlur($event, scope.row, 'budgetMoney')"></el-input>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column v-if="config.tableInfo.appendMoney && config.tableInfo.appendMoney.isShow"
                    prop="appendMoney" label="追加预算金额(万)" :width="config.tableInfo.appendMoney.width">
                    <template slot="header">
                      <span style="color: rgb(245, 108, 108);">*</span><span>追加预算金额(万)</span>
                    </template>
                    <template slot-scope="scope">
                      <el-form-item
                        :prop="'val.' + getPropPosi(scope.row.id, val['budget']) + '.' + config.tableInfo.appendMoney.prop"
                        :rules="tableRule.budgetMoney">
                        <el-input v-model="scope.row[config.tableInfo.appendMoney.prop]" :controls="false"
                          @focus="selectText($event)" @input="handleInput($event, scope.row, 'appendMoney')"
                          @blur="handleBlur($event, scope.row, 'appendMoney')"></el-input>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" :width="config.tableInfo.operation.width"
                    v-if="config.tableInfo.operation && config.tableInfo.operation.isShow">
                    <template slot-scope="scope">
                      <div style="display: flex;">
                        <el-button @click="planDelete(scope.row)" type="text" size="small"><i
                            class="el-icon-delete-solid delete-icon"></i></el-button>
                        <el-button class="el-icon-circle-plus-outline delete-icon" type="text" size="small"
                          @click="addChildrenPlan(scope.row)" v-if="!scope.row.isChildren" title="增加子归口"></el-button>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="本月增减原因" v-if="config.tableInfo.cause && config.tableInfo.cause.isShow"
                    width="110px">
                    <template slot-scope="scope">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column label="下月预算说明" v-if="config.tableInfo.explain && config.tableInfo.explain.isShow">
                    <template slot-scope="scope" width="110px">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column label="备注" v-if="config.tableInfo.remark && config.tableInfo.remark.isShow"> <template
                      slot-scope="scope" width="110px">
                      <el-input class="m-input"></el-input>
                    </template>
                  </el-table-column>
                </el-table>
              </el-form>
              <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(index)"
                v-if="config.tableInfo.addLineButton && config.tableInfo.addLineButton.isShow"></i>
            </el-collapse-item>
          </el-collapse>
        </el-card>
      </el-form>
    </el-card>
    <budgetTemplate :tempVisible.sync="tempVisible"></budgetTemplate>
    <budgeInfotPreview :previewVisible.sync="previewVisible" :datas="datas"></budgeInfotPreview>
  </div>
</template>
<script>
import budgetTemplate from '../budgetTemplateDialog.vue'
import budgeInfotPreview from '../budgetInfoDialog.vue'
import { Calculate } from '@/utils/number';
import { deepClone } from '@/utils';
export default {
  name: 'CardTable',
  props: ['infoData', 'cardConfig'],
  components: { budgetTemplate, budgeInfotPreview },
  data() {
    return {
      subRule: {
        departId: [
          { required: true, message: '请选择部门', trigger: 'change' }
        ],
        customDepart: [
          { required: true, message: '请输入自定义部门', trigger: 'blur' }
        ]
      },
      tableRule: {
        budgetType: [
          { required: true, message: '请输入归口', trigger: 'blur' }
        ],
        budgetMoney: [
          { required: true, message: '请填入预算金额', trigger: 'blur' },
          {
            pattern: /^(?!(0[0-9]{0,}$))[0-9]{1,}[.]{0,}[0-9]{0,}$/,
            message: '预算金额要大于0',
            trigger: 'blur'
          }
        ]
      },
      datas: [],//deepClone(this.infoData),
      config: this.cardConfig,
      tempVisible: false,
      previewVisible: false
    }
  },
  watch: {
    infoData: {
      handler(val) {
        // this.datas = deepClone(val)
        this.genTable(deepClone(val))
      },
      deep: true
    }
  },
  created() {
    // this.datas = deepClone(this.infoData)
    this.genTable(deepClone(this.infoData))
  },
  computed: {
    getPropPosi() {
      return (id, target) => {
        let pos = ''
        target.forEach((item, index) => {
          if (item.id == id) {
            pos = index
            return pos
          } else {
            if (item.children && item.children.length) {
              item.children.forEach((el, idx) => {
                if (el.id == id) {
                  pos = `${index}.children.${idx}`
                  return pos
                }
              })
            }
          }
        })
        return pos
      }
    }
  },
  methods: {
    genTable(datas) {
      if (datas.length) {

      } else {
        var template = []
        template.push(deepClone(this.config.departBudgetTemp))
        var budgetTemplate = this.addPlan()
        budgetTemplate.index = 1
        template[0].budget.push(budgetTemplate)
        this.datas = template
      }
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        let v = item || 0;
        v -= 0
        val = Calculate.accAdd(val, v)
      });
      return val;
    },
    summary(param) {
      const { columns, data } = param;
      const sums = [];
      let appendTotal = 0
      columns.forEach((column, index) => {
        if (index === 0) {
          // 只找第一列放合计
          sums[index] = "合计:";
          return;
        }
        if (column.property === "budgetMoney") {
          let values = data.map(item => item[column.property]);
          sums[index] = "￥" + this.sumNums(values)
        } else if (column.property === "appendMoney") {
          let values = data.map(item => item[column.property]);
          appendTotal = this.sumNums(values)
          sums[index] = "￥" + appendTotal;
        } else {
          sums[index] = "";
        }
      });
      return sums;
    },
    //是否关联项目
    radioChange(val, i, j) {
      if (!val) { // 不关联
        this.datas[i].budget[j].relateProjId = '';
      }
    },
    //选择部门
    departChange() {
      this.config.departOptions.forEach(item => {
        item.hasSelect = false;
      });
      this.datas.forEach(item => {
        const departId = item.departId;
        const index = this.config.departOptions.findIndex(it => it.id == departId);
        if (index > -1) {
          this.config.departOptions[index].hasSelect = true;
        }
      });
    },

    //新增预算详情
    addPlan(index) {
      var budgetTemp = deepClone(this.config.budgetTemp)
      budgetTemp.id = new Date().getTime()
      budgetTemp.children = []
      if (index !== undefined) {
        budgetTemp.index = this.datas[index].budget.length + 1
        this.datas[index].budget.push(budgetTemp)
      }
      return budgetTemp
    },
    addChildrenPlan(o) {
      // console.log('o', o)
      var budgetTemp = deepClone(this.config.budgetTemp)
      if (o.children === undefined) o.children = []
      budgetTemp.isChildren = true
      budgetTemp.id = new Date().getTime()
      o.children.push(budgetTemp)
    },
    //新增部门预算
    addDepartPlan() {
      var departBudgetTemp = deepClone(this.config.departBudgetTemp)
      if (departBudgetTemp) {
        var budgetTemp = this.addPlan()
        budgetTemp.index = 1
        departBudgetTemp.budget.push(budgetTemp)
        this.datas.push(deepClone(departBudgetTemp))
        this.$nextTick(() => {
          var flowContent = document.querySelector('.flow-content')
          flowContent.style.scrollBehavior = 'smooth'
          var flowChild = flowContent.children[0]
          let chidlHeight = flowChild.clientHeight
          flowContent.scrollTop = chidlHeight
        })
      }
    },
    //上移部门预算
    upMove(index) {
      if (index >= 1) {
        let prevIndex = index - 1
        let prevData = deepClone(this.datas[prevIndex])
        let currentData = deepClone(this.datas[index])
        //交换
        this.datas.splice(prevIndex, 2, currentData, prevData)

        this.$nextTick(() => {
          this.$refs.subForm.clearValidate()
        })
      }
    },

    planDelete(o) {
      this.datas.forEach((item, i) => {
        var budget = item.budget
        budget.forEach((el, j) => {
          if (el.id == o.id) {
            this.datas[i].budget.splice(j, 1)
            this.genIndex(i)
            return
          } else {
            budget[j].children.forEach((it, k) => {
              if (it.id == o.id) {
                this.datas[i].budget[j].children.splice(k, 1)
                return
              }
            })
          }

        })
      })
    },
    //重新整理index
    genIndex(i) {
      this.datas[i].budget.forEach((item, index) => {
        item.index = index + 1
      })
    },
    deleteDepartPlan(index) {
      if (this.datas[index].departId) {
        //释放这个部门为可选
        var currentId = this.datas[index].departId
        var i = this.config.departOptions.findIndex(item => item.id == currentId)
        if (i > -1) {
          var departOptions = deepClone(this.config.departOptions[i])
          departOptions.hasSelect = false
          this.config.departOptions.splice(i, 1, departOptions)
        }
      }
      this.datas.splice(index, 1);
    },
    //预算预览
    preview() {
      this.previewVisible = true
    },
    //显示模板
    showTemplate() {
      this.tempVisible = true
    },
    //点击输入框选中
    selectText(e) {
      e.target.select()
    },

    //最多输入六位小数
    handleInput(e, o, k) {
      // 通过正则过滤小数点后两位
      var value = e.match(/^\d*(\.?\d{0,6})/g)[0] || null;
      o[k] = value
    },
    handleBlur(e, o, k) {
      var val = e.target.value
      var value = val - 0;
      o[k] = value
    },
    validData() {
      return new Promise((resolve, reject) => {
        this.$refs.subForm.validate((valid) => {
          if (valid) {
            var tableForms = this.$refs.tableForm
            var hasErr = false
            tableForms.forEach(item => {
              item.validate(res => {
                if (!res) hasErr = true
              })
            })
            if (!hasErr) {
              resolve(deepClone(this.datas));
            } else {
              reject(false);
            }

          } else {
            setTimeout(() => {
              reject(false);
            });
          }
        });
      });
    }

  }
}
</script>
<style lang="scss" scoped src="@/views/BudgetManage/CompanyBudget/components/style/style.scss"></style>
<style scoped lang="scss">
::v-deep .flow-content {
  scroll-behavior: smooth;
}

.line {
  width: 100%;
  height: 1px;
  background: rgb(235, 235, 235);
  margin: 3px 0;
}


::v-deep .el-table__body-wrapper,
::v-deep .el-table__body-wrapper * {
  overflow: visible !important;
}

::v-deep .el-form-item__error {
  z-index: 1;
}

::v-deep .table .el-input.el-input--mini {
  width: auto;
}

::v-deep table .el-input.el-input--mini.m-input {
  width: 100%;
}

::v-deep .expanded.el-table__row--level-0 td.el-table__cell {

  border-bottom: none !important;
}

::v-deep .el-table--border .el-table__cell:first-child .cell {
  text-align: center;
}

::v-deep .el-table__row--level-1 .children-row {
  padding-left: 20px;
}
</style>

