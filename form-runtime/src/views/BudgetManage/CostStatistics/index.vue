<template>
  <div class="costStatistics">
    <div class="query">
      <div>
        <span>公司名称：</span>
        <el-select v-model="value" clearable placeholder="请选择">
          <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
      </div>
      <div>
        <span>所属部门/费用预算类型：</span>
        <el-select v-model="value" clearable placeholder="请选择">
          <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
      </div>
      <div>
        <span>统计时间：</span>
        <el-radio-group v-model="timeType" size="mini" style="margin-right: 10px;">
          <el-radio-button label="year">年度</el-radio-button>
          <el-radio-button label="month">月份</el-radio-button>
        </el-radio-group>
        <el-date-picker
          v-model="yearVal"
          type="year"
          v-if="timeType=='year'"
          placeholder="选择年度">
        </el-date-picker>
        <el-date-picker
          v-model="monthVal"
          type="month"
          v-else
          placeholder="选择月份">
        </el-date-picker>
      </div>
      <div>
        <span>按归口明细汇总：</span>
        <el-select v-model="value" clearable placeholder="请选择">
          <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
      </div>
      <div><el-button icon="el-icon-search" type="primary">查询</el-button></div>
    </div>
    <el-table
      :data="tableData"
      style="width: 100%;margin-bottom: 20px;"
      row-key="id"
      :summary-method="getSummaries"
      show-summary
      default-expand-all
      :tree-props="{children: 'children', hasChildren: 'hasChildren'}">
      <el-table-column
        prop="type"
        label="所属部门/费用预算类型">
      </el-table-column>
      <el-table-column
        prop="budgetAmount"
        label="预算总额（万元）"
        align="center"
        width="240">
      </el-table-column>
      <el-table-column
        prop="budgetUsed"
        label="已使用预算（万元）"
        align="center"
        width="240">
      </el-table-column>
      <el-table-column
        prop="budgetUnused"
        label="剩余预算（万元）"
        align="center"
        width="240">
      </el-table-column>
      <el-table-column            
        prop="relatedProjects"
        label="关联项目"
        width="240">
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
import { getSummaries } from '@/utils/index'
export default {
  name: "costStatistics",
  components: {

  },
  data() {
    return {
      tableData: [
        {
          id: 1,
          type: '2016-05-02',
          budgetAmount: 100,
          budgetUsed: 60,
          budgetUnused: 40,
          relatedProjects: '上海市普陀区金沙江路 1518 弄'
        }, {
          id: 2,
          type: '2016-05-04',
          budgetAmount: 100,
          budgetUsed: 60,
          budgetUnused: 40,
          relatedProjects: '上海市普陀区金沙江路 1517 弄'
        }, {
          id: 3,
          type: '2016-05-01',
          budgetAmount: 100,
          budgetUsed: 60,
          budgetUnused: 40,
          relatedProjects: '深圳总部大楼建设',
          children: [{
              id: 31,
              type: '2016-05-01',
              budgetAmount: 100,
              budgetUsed: 60,
              budgetUnused: 40,
              relatedProjects: '深圳总部大楼建设',
              children: [{
                  id: 38,
                  type: '2016-05-01',
                  budgetAmount: 100,
                  budgetUsed: 60,
                  budgetUnused: 40,
                  relatedProjects: '深圳总部大楼建设'
                }, {
                  id: 39,
                  type: '2016-05-01',
                  budgetAmount: 100,
                  budgetUsed: 60,
                  budgetUnused: 40,
                  relatedProjects: '深圳总部大楼建设'
              }]
            }, {
              id: 32,
              type: '2016-05-01',
              budgetAmount: 100,
              budgetUsed: 60,
              budgetUnused: 40,
              relatedProjects: '深圳总部大楼建设'
          }]
        }, {
          id: 4,
          type: '2016-05-03',
          budgetAmount: 100,
          budgetUsed: 60,
          budgetUnused: 40,
          relatedProjects: '上海市普陀区金沙江路 1516 弄'
        },],
      options: [{
        value: '选项1',
        label: '黄金糕'
      }, {
        value: '选项2',
        label: '双皮奶'
      }, {
        value: '选项3',
        label: '蚵仔煎'
      }, {
        value: '选项4',
        label: '龙须面'
      }, {
        value: '选项5',
        label: '北京烤鸭'
      }],
      value: '',
      timeType:'year',
      monthVal: '',
      yearVal: ''
    }
  },
  watch: {

  },
  computed: {

  },
  methods: {
    getSummaries(param){
      return getSummaries(param)
    }
  },
  created() {

  },
  mounted() {

  },
  updated() {

  },
  destroyed() {

  }
}
</script>

<style lang="scss" scoped>
.costStatistics{
  overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;
  padding: 25px;
  .query{
    display: grid;
    margin: 10px 0;
    grid-template-columns: 2fr 2fr 1fr;
    grid-template-rows: 30px 30px;
    row-gap: 10px;
    // grid-template-areas: "one two three four five";
    &>div:nth-child(1){
      grid-column-start: 1;
      grid-column-end: 2;
      grid-row-start: 1;
      grid-row-end: 2;
    }
    &>div:nth-child(2){
      grid-column-start: 2;
      grid-column-end: 3;
      grid-row-start: 1;
      grid-row-end: 2;
    }
    &>div:nth-child(3){
      grid-column-start: 1;
      grid-column-end: 2;
      grid-row-start: 2;
      grid-row-end: 3;
    }
    &>div:nth-child(4){
      grid-column-start: 2;
      grid-column-end: 3;
      grid-row-start: 2;
      grid-row-end: 3;
    }
    &>div:nth-child(5){
      grid-column-start: 3;
      grid-column-end: 4;
      grid-row-start: 1;
      grid-row-end: 3;
      align-content: center;
    }
  }
  @media screen and (max-width:1200px) {
    .query{
      grid-template-columns: 2fr 2fr 1fr;
      grid-template-rows: 30px 30px;
      &>div:nth-child(1){
        grid-column-start: 1;
        grid-column-end: 2;
        grid-row-start: 1;
        grid-row-end: 2;
      }
      &>div:nth-child(2){
        grid-column-start: 2;
        grid-column-end: 3;
        grid-row-start: 1;
        grid-row-end: 2;
      }
      &>div:nth-child(3){
        grid-column-start: 1;
        grid-column-end: 2;
        grid-row-start: 2;
        grid-row-end: 3;
      }
      &>div:nth-child(4){
        grid-column-start: 2;
        grid-column-end: 3;
        grid-row-start: 2;
        grid-row-end: 3;
      }
      &>div:nth-child(5){
        grid-column-start: 3;
        grid-column-end: 4;
        grid-row-start: 1;
        grid-row-end: 3;
      }
    }
  }
}
</style>