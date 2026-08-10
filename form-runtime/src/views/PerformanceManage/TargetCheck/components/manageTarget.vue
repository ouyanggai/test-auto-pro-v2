<template>
  <div class="manageTarget">
    <div style="margin: 5px 0px 10px;">
      <span class="title">公司：</span>
      <el-select v-model="query.company" placeholder="请选择公司">
        <el-option
          v-for="item in companyOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value">
        </el-option>
      </el-select>
      <span class="title">部门：</span>
      <el-select v-model="query.department" placeholder="请选择部门">
        <el-option
          v-for="item in departmentOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value">
        </el-option>
      </el-select>
      <span class="title">选择状态：</span>
      <el-select v-model="query.status" placeholder="请选择状态">
        <el-option
          v-for="item in statusOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value">
        </el-option>
      </el-select>
      <span class="title">选择年度：</span>
      <el-date-picker
        style="width: 180px;"
        v-model="query.year"
        type="year"
        placeholder="选择年都">
      </el-date-picker>
    </div>
    <el-scrollbar
      style="height: calc(100vh - 300px);">
      <el-table
        :data="tableData"
        min-height="200">
        <el-table-column
          prop="name"
          label="姓名"
          width="180">
        </el-table-column>
        <el-table-column
          prop="year"
          width="180"
          label="考核年限">
        </el-table-column>
        <el-table-column
          prop="company"
          label="公司">
        </el-table-column>
        <el-table-column
          prop="department"
          label="部门"
          width="180">
        </el-table-column>
        <el-table-column
          prop="initiator"
          label="发起人"
          width="180">
        </el-table-column>
        <el-table-column
          prop="createTime"
          label="发起时间"
          width="180">
        </el-table-column>
        <el-table-column
          label="状态"
          width="180">
          <templete slot-scope="scope">
            <el-tag v-if="scope.row.kpiGroupStatus=='not_submitted'" size="small" type="warning">草稿</el-tag>
            <el-tag v-else-if="scope.row.kpiGroupStatus=='under_review'" size="small" type="primary">审核中</el-tag>
            <el-tag v-else-if="scope.row.kpiGroupStatus=='rejected'" size="small" type="danger">已驳回</el-tag>
            <el-tag v-else-if="scope.row.kpiGroupStatus=='pass'||scope.row.kpiGroupStatus=='finish'" size="small" type="success">已通过</el-tag>
          </templete>
        </el-table-column>
        <el-table-column
          label="操作"
          width="180">
          <template slot-scope="scope">
            <el-button type="text" @click="lookFlow(scope.row)">查看流程</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-scrollbar>
    <el-pagination
      style="margin-top: 10px;"
      @size-change="handleSizeChange"
      @current-change="handleCurrentChange"
      background
      :current-page="currentPage"
      :page-sizes="[10, 20, 50, 100]"
      :page-size="pageSizes"
      layout="total, sizes, prev, pager, next, jumper"
      :total="total">
    </el-pagination>
  </div>
</template>

<script>
export default {
  name: 'ManageTarget',
  props: {

  },
  data() {
    return {
      query: {
        company: '',
        department: '',
        status: '',
        year: ''
      },
      companyOptions: [
        {
          value: '选项1',
          label: '黄金糕'
        }, {
          value: '选项2',
          label: '双皮奶'
        }
      ],
      departmentOptions: [
        {
          value: '选项1',
          label: '黄金糕'
        }, {
          value: '选项2',
          label: '双皮奶'
        }
      ],
      statusOptions: [
        {
          value: '选项1',
          label: '黄金糕'
        }, {
          value: '选项2',
          label: '双皮奶'
        }
      ],
      tableData: [{
        name: '王小虎1',
        year: '2022',
        company: '上海市普陀区金沙江路 1518 弄',
        department: '开发部',
        initiator: '张三',
        createTime: '2022-12-12 12:00:00',
        kpiGroupStatus: 'not_submitted'
      }, {
        name: '王小虎',
        year: '2022',
        company: '上海市普陀区金沙江路 1519 弄',
        department: '开发部',
        initiator: '张三',
        createTime: '2022-12-12 12:00:00',
        kpiGroupStatus: 'under_review'
      }, {
        name: '王小虎',
        year: '2022',
        company: '上海市普陀区金沙江路 1518 弄',
        department: '开发部',
        initiator: '张三',
        createTime: '2022-12-12 12:00:00',
        kpiGroupStatus: 'rejected'
      }, {
        name: '王小虎',
        year: '2022',
        company: '上海市普陀区金沙江路 1517 弄',
        department: '开发部',
        initiator: '张三',
        createTime: '2022-12-12 12:00:00',
        kpiGroupStatus: 'pass'
      }, {
        name: '王小虎',
        year: '2022',
        company: '上海市普陀区金沙江路 1519 弄',
        department: '开发部',
        initiator: '张三',
        createTime: '2022-12-12 12:00:00',
        kpiGroupStatus: 'pass'
      }],
      currentPage: 1,
      pageSizes: 10,
      total: 400
    };
  },
  watch: {

  },
  computed: {

  },
  methods: {
    lookFlow(item) {
      console.log(item);
    },
    handleSizeChange(val) {
      this.pageSizes = val;
      console.log(`每页 ${val} 条`);
    },
    handleCurrentChange(val) {
      this.currentPage = val;
      console.log(`当前页: ${val}`);
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
};
</script>

<style lang="scss" scoped>
.manageTarget{
  .title{
    display: inline-block;
    margin-left: 20px;
  }
  ::v-deep .el-table__header-wrapper{
    position: fixed;
    z-index: 99;
  }
  ::v-deep .el-table__body-wrapper{
    margin-top: 36px;
  }
}
</style>
