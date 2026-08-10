<!--
 * @Descripttion: 报销单组件
 * @Author: liufuze
-->
<template>
  <el-form :model="{ tableData }" :rules="rules" ref="form">
    <div class="add-button-div">
      <el-button v-if="showAddBt" type="primary" @click="addOneRow" icon="el-icon-circle-plus-outline">新增</el-button>
      <el-button v-if="showDetail" type="text" style="margin-right:30px"><i class="el-icon-view"
          style=" font-size: 24px;color: #333333;"></i></el-button>
    </div>
    <el-table :data="tableData" header-row-class-name="table-header" :summary-method="getExpendDetailSummaries"
      show-summary style="margin-top: 10px;" v-if="tableData.length">
      <el-table-column v-for="el in eltableConfig" :prop="el.prop" :label="el.label" :width="el.width">
        <template slot="header">
          <span style="color: rgb(245, 108, 108);" v-if="el.isRequire">*</span><span>{{ el.label }}</span>
        </template>
        <template slot-scope="scope" v-if="el.slot">
          <el-form-item :prop="'tableData.' + scope.$index + '.' + el.prop" :rules="rules[el.prop]">
            <el-date-picker :type="el.slot.type" v-model="scope.row[el.prop]" v-if="el.slot.nodeType == 'date-picker'">
            </el-date-picker>
            <el-input v-model="scope.row[el.prop]" v-if="el.slot.nodeType == 'input'">
            </el-input>
            <el-input-number v-model="scope.row[el.prop]" :min="0.00" :precision="2" :step="0.1" :controls="false"
              v-if="el.slot.nodeType == 'moneyInput'"></el-input-number>
            <el-autocomplete v-model="scope.row[el.prop]"
              :fetch-suggestions="(queryString, cb) => querySearch(queryString, cb, el.slot.source)" placeholder="请输入内容"
              :trigger-on-focus="false" v-if="el.slot.nodeType == 'input-select'"
              @select="item => handleSelect(item, scope.$index, el.prop)">
              <template slot-scope="{ item }">
                <div class="name">{{ item[el.slot.label] }}</div>
              </template>
            </el-autocomplete>
            <el-select v-model="scope.row[el.prop]" v-if="el.slot.nodeType == 'select'">
              <el-option v-for="item in el.slot.source" :label="item.label" :value="item.value"></el-option>
            </el-select>
            <el-cascader v-model="scope.row[el.prop]" v-if="el.slot.nodeType == 'cascader'" :options="el.slot.source"
              :props="{ expandTrigger: 'hover' }"></el-cascader>
          </el-form-item>
          <i v-if="el.slot.nodeType == 'icon'" :class="el.slot.icon"
            style="text-align: center;cursor: pointer;font-size: 18px;color: #f56c6c;"
            @click="removeRow(scope.$index)"></i>
        </template>
      </el-table-column>
    </el-table>
  </el-form>
</template>
<script>
import { deepClone } from '@/utils';
export default {
  name: 'Eltables',
  props: {
    eltableConfig: {
      type: Array,
      default() {
        return [];
      }
    },
    showAddBt: {
      type: Boolean,
      default: true
    },
    showDetail: {
      type: Boolean,
      default: false
    },
    summaryArr: { // 金额统计对应的表格字段
      type: Array,
      default() {
        return [];
      }
    }

  }, // ['eltableConfig'],
  data() {
    return {
      person: '',
      rules: {},
      templates: {}, // 添加一行的模板
      tableData: [
      ]
    };
  },
  created() {
    this.genRulesAndDatas();
  },
  methods: {
    querySearch(queryString, cb, source) {
      var results = queryString ? this.findByQuery(queryString, source) : source;
      cb(results);
    },
    findByQuery(queryString, source) {
      // TODO 如果只有一个结果，则把对应结果id放入data
      var findArr = [];
      source.forEach(item => {
        if (item.name.indexOf(queryString) > -1) {
          findArr.push(item);
        }
      });
      return findArr;
    },
    handleSelect(items, index, key) {
      this.tableData[index][key] = items.name;
    },
    genRulesAndDatas() {
      // 根据配置文件生成数据模板和校验规则
      var rule = {}; var templates = {};
      this.eltableConfig.forEach(item => {
        if (item.prop) templates[item.prop] = item.default || '';
        if (item.isRequire && item.slot && item.slot.nodeType) {
          const tempArr = [];
          let message, trigger;
          const inputArr = ['input', 'number', 'text'];
          if (inputArr.indexOf(item.slot.nodeType) > -1) {
            message = `请输入${item.label}`;
            trigger = 'blur';
          } else {
            message = `请选择${item.label}`;
            trigger = 'change';
          }
          const tempObj = {
            required: true,
            message,
            trigger
          };
          tempArr.push(tempObj);
          rule[item.prop] = tempArr;
        }
      });
      this.templates = deepClone(templates);
      this.addOneRow();
      this.rules = deepClone(rule);
      console.log('rules', this.rules);
    },
    addOneRow() {
      this.tableData.push(deepClone(this.templates));
    },
    removeRow(index) {
      // this.$refs.form.validate(res => {

      // })
    },
    getExpendDetailSummaries(param) { // 费用明细表格合计
      // if (this.summaryArr.length == 0) return;
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (this.summaryArr.includes(column.property)) {
          const values = data.map(item => Number(item[column.property]));
          if (!values.every(value => isNaN(value))) {
            sums[index] = values.reduce((prev, curr) => {
              const value = Number(curr);
              if (!isNaN(value)) {
                return prev + curr;
              } else {
                return prev;
              }
            }, 0);
            sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
          } else {
            sums[index] = '';
          }
        }
      });
      return sums;
    }
  }
};
</script>
<style scoped>
::v-deep .el-table,
::v-deep .el-table__body-wrapper,
::v-deep .el-table__body-wrapper * {
  overflow: visible !important;
}

::v-deep .el-form-item__error {
  z-index: 1;
}

::v-deep .el-form-item--mini.el-form-item {
  margin: 2px 0 !important;
}

.add-button-div {
  display: flex;
  flex-direction: row-reverse;
}
</style>
