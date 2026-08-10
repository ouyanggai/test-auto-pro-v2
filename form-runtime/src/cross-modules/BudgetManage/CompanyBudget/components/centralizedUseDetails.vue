<!--归口使用明细界面-->
<template>
  <div class="centralizedUseDetails">
    <el-row style="margin-bottom: 12px;">
        <el-col :xs="24" :sm="12" :md="8" :lg="5">
          <span>所属部门：</span>&nbsp;
          <el-select v-model="query.projectId" placeholder="请选择" style="margin-right:35px;" @change="projectChange" clearable>
            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name" :value="item.id">
            </el-option>
          </el-select>
        </el-col>
        <el-col :xs="24" :sm="12" :md="8" :lg="5">
          <span>费用预算：</span>&nbsp;
          <el-select v-model="query.projectId" placeholder="请选择" style="margin-right:35px;" @change="projectChange" clearable>
            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name" :value="item.id">
            </el-option>
          </el-select>
        </el-col>
        <el-col :xs="24" :sm="12" :md="8" :lg="5">
          <span>统计时间：</span>&nbsp;
          <el-date-picker v-model="query.annual" type="year" placeholder="选择年份" value-format="yyyy" style="margin-right:35px;"
            @change="searchByQuery" clearable>
          </el-date-picker>
        </el-col>
      </el-row>
      <el-table
          :data="tableData"
          style="width: 100%;margin-bottom: 20px;"
          row-key="id"
          show-summary>
          <el-table-column
            prop="type"
            label="所属部门/费用预算类型">
          </el-table-column>
          <el-table-column
            prop="budgetAmount"
            label="预算总额（万元）"
            align="center"
            sortable
            width="240">
          </el-table-column>
          <el-table-column
            prop="budgetUsed"
            label="已使用预算（万元）"
            align="center"
            sortable
            width="240">
          </el-table-column>
          <el-table-column
            prop="budgetUnused"
            label="剩余预算（万元）"
            align="center"
            sortable
            width="240">
          </el-table-column>
          <el-table-column            
            prop="relatedProjects"
            label="关联项目"
            sortable
            width="240">
          </el-table-column>
        </el-table>
  </div>
</template>

<script>
export default {
  name: "centralizedUseDetails",
  components: {

  },
  data() {
    return {
      isBudgetOrFinance: true, // 是预算管理中还是财务管理中的界面 true：预算管理 ； false 财务管理
      query: {
        // companyId: localstorageGet('companyId'),
        type: '3',
        annual: '',
        projectId: '',
        budgetTime: '',
      },
      projectOptions: [],
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
        }, {
          id: 4,
          type: '2016-05-03',
          budgetAmount: 100,
          budgetUsed: 60,
          budgetUnused: 40,
          relatedProjects: '上海市普陀区金沙江路 1516 弄'
        },],
    }
  },
  watch: {

  },
  computed: {

  },
  methods: {

  },
  created() {
    this.isBudgetOrFinance = this.$route.path.includes('groupBudgetManage');

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
.centralizedUseDetails{
  background-color: #fff;
  min-height: calc(100vh - 120px);
  padding: 25px;

}
</style>