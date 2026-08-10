<template>
  <div style="width: 100%; height:100%;background-color:white" v-if="false">
    <split-panel
      direction="vertical"
      :splitterSize="2"
      :initialRatio="0.18"
      :minRatio="0.18"
      :maxRatio="0.3"
      @ratio-change="handleRatioChange"
    >
      <template #left>
        <div class="left-panel">
          <div class="item">test润世华软件公司</div>
          <div class="item">test润世华软件公司test润世华软件公司test润世华软件公司</div>
          <div class="item active" title="test润世华软件公司">test润世华软件公司</div>
          <div class="item" v-for="i in 10" :key="i">test润世华软件公司</div>
        </div>
      </template>
      <template #right>
        <div class="right-content">
          <div class="search-box">
            <el-input placeholder="请输入关键字" v-model="searchValue" clearable style="width: 200px;margin-right: 10px;"></el-input>
            <el-button type="primary" icon="el-icon-search" @click="handleSearch">搜索</el-button>
          </div>
          <div class="list">
            <dy-table maxTableHeight="600" :keys="colKey" :fetchData="fetchData" :list="tableData" :actions="actions1"
              style="padding:0px;" ref="usingTable" :isPagination="true" :pagination="pagination">
            </dy-table>
          </div>
        </div>
      </template>
    </split-panel>
  </div>
</template>

<script>
import SplitPanel from './SplitPanel.vue';
import DyTable from '@/components/DyTable';
export default {
  data() {
    return {
      actions1: [],
      actions12: [
        {
          label: '详情',
          width: '80',
          actionFixed:'right',
          action: row => {
            console.log('row',row)
            this.checkFormFlow(row,this.invoiceDialogType)
          }
        },
      ],
      tableData: [{userName:'test润世华软件公司',userName2:'test润世华软件公司2'}],
      colKey: {
        userName: {
          label: '姓名',
          // ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row.userName);
          }
        },
        userName2: {
          label: '姓名2',
          // ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row.userName2);
          }
        },
        userName22: {
          label: '操作',
          width: '100',
          handle: (scope, createElement) => {
            return <span>
              <el-button type='text' onClick={_ => { }}>查看</el-button>
              <el-button type="text" size="small">编辑</el-button>
            </span>;
          }
        },
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      searchValue: '',
    }
  },
  components: {
    SplitPanel, DyTable
  },
  methods: {
    fetchData() {
      const param = {
        data: {
          // ...this.searchForm
          // manageType: 'work_and_manager_target',
          // kpiScope: typeEnum[kpiType],
          // companyName: this.searchForm.companyName,
          // depName: this.searchForm.depName
        },
        pagination: true,
        size: this.pagination.size,
        current: this.pagination.pages
      };
      return;
      this.$axios.post('/web/plan/api/workPlanGroup/list', param,
        res => {
          if (res.isSuccess) {
            this.tableData = res.data || [];
            this.pagination.total = res.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleSearch() {

    },
    handleRatioChange(ratio) {
      console.log('当前比例:', ratio);
    }
  }
};
</script>

<style lang="scss" scoped>
.left-panel {
  width: 100%;
  height: 100%;
  .item {
    padding-left: 10px;
    line-height: 37px;
    height: 37px;
    white-space: nowrap;
    overflow: hidden;
    cursor: pointer;
    &:hover {
      background-color: #e6f7ff;
      // color: #1890ff;
    }
  }
  .active {
    background-color: #e6f7ff;
    color: #1890ff;
    border-right: 3px solid #1890ff;
  }
}
.right-content {
  width: 100%;
  height: 100%;
  padding: 10px;
  // box-sizing: border-box;
}
</style>