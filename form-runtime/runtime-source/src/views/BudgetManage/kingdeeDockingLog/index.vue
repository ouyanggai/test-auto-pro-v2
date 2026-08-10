<template>
  <div class="inboundManage" ref="container">
    <!-- 筛选表单 -->
    <el-form :inline="true" class="search-form">
      <el-form-item label="推送编号">
        <el-input v-model="searchForm.number" placeholder="请输入推送编号" maxlength="50" clearable></el-input>
      </el-form-item>
      <!-- <el-form-item label="银行账号">
        <el-input v-model="searchForm.failDescribe" placeholder="请输入银行账号" maxlength="50" clearable></el-input>
      </el-form-item> -->
      <el-form-item label="步骤">
        <el-select v-model="searchForm.step" clearable placeholder="请选择步骤">
          <el-option
            v-for="item in stepOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="searchForm.isSuccess" clearable placeholder="请选择是否成功">
          <el-option
            v-for="item in statusOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="onSearch">查询</el-button>
      </el-form-item>
      <el-form-item>
        <el-button @click="onReset">重置</el-button>
      </el-form-item>
    </el-form>

    <!-- dy-table 表格 -->
    <dy-table
      ref="dyTable"
      :showSerial="true"
      :fetchData="fetchData"
      :keys="colKey"
      :actions="actionKey"
      :list="tableData"
      :isPagination="true"
      :height="tableHeight"
      :pagination="pagination"
    />
    <!-- 入库记录 -->
    <kingdeeDockingLogDialog
      :visible.sync="dialogVisible"
      :rowData="logData"
    />
  </div>
</template>
<script>
import DyTable from '@/components/DyTable';
import kingdeeDockingLogDialog from './components/kingdeeDockingLogDialog.vue';
import Api from '@/api';
export default {
  name: 'InboundManage',
  components: { DyTable, kingdeeDockingLogDialog },
  data() {
    return {
      dialogVisible: false,
      logData: {},
      stepOptions: [
        { value: 'save', label: '保存' },
        { value: 'submit', label: '提交' },
        { value: 'audit', label: '审核' }
      ],
      statusOptions: [
        { value: '1', label: '成功' },
        { value: '0', label: '失败' }
      ],
      searchForm: {
        number: '',
        failDescribe: '',
        step: '',
        isSuccess: ''
      },
      tableData: [],
      colKey: {
        dataId: {
          label: '业务ID',
          minWidth: '150'
        },
        number: {
          label: '推送编号',
          minWidth: '100'
        },
        processName: {
          label: '流程名称',
          minWidth: '150'
        },
        type: {
          label: '单据类型',
          minWidth: '80',
          handle: (scope, h) => {
            if (scope.row.type === 'AP_PAYBILL') {
              return h('span', '付款单');
            } else if (scope.row.type === 'AR_RECEIVEBILL') {
              return h('span', '收款单');
            } else if (scope.row.type === 'AP_Payable') {
              return h('span', '应付单');
            } else if (scope.row.type === 'AR_receivable') {
              return h('span', '应收单');
            } else {
              return h('span', '');
            }
          }
        },
        step: {
          label: '步骤',
          minWidth: '80',
          handle: (scope, h) => {
            if (scope.row.step === 'save') {
              return h('el-tag', { props: { type: 'info' }}, '保存');
            } else if (scope.row.step === 'submit') {
              return h('el-tag', { props: { type: '' }}, '提交');
            } else if (scope.row.step === 'audit') {
              return h('el-tag', { props: { type: 'warning' }}, '审核');
            } else {
              return h('span', '');
            }
          }
        },
        isSuccess: {
          label: '状态',
          minWidth: '80',
          handle: (scope, h) => {
            if (scope.row.isSuccess === '1') {
              return h('el-tag', { props: { type: 'success' }}, '成功');
            } else if (scope.row.isSuccess === '0') {
              return h('el-tag', { props: { type: 'danger' }}, '失败');
            } else {
              return h('span', '');
            }
          }
        }
      },
      actionKey: [
        {
          label: '日志',
          width: '100',
          actionFixed: 'right',
          action: row => {
            this.handleInfo(row);
          }
        }
      ],
      pagination: {
        total: 400,
        pages: 1,
        size: 10
      },
      tableHeight: '400px'
    };
  },
  methods: {
    onReset() {
      this.searchForm = {
        number: '',
        failDescribe: '',
        step: '',
        isSuccess: ''
      };
      this.onSearch();
    },
    handleInfo(item) {
      this.dialogVisible = true;
      this.logData = JSON.parse(item.failDescribe);
    },
    onSearch() {
      // 查询逻辑，调用fetchData
      this.pagination.pages = 1;
      this.fetchData();
    },
    fetchData() {
      this.$axios.post(Api.kingdee.pushRecordsList,
        {
          data: {
            failDescribe: this.searchForm.failDescribe, // in_warehouse("入库单"),out_warehouse("出库单"),again_in_warehouse("归还单"),destroy("报废单"),check_record("盘点"),
            step: this.searchForm.step,
            type: '1',
            isSuccess: this.searchForm.isSuccess,
            number: this.searchForm.number
          },
          pagination: true,
          current: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          this.tableData = [];
          this.pagination.total = 0;
          if (res.isSuccess) {
            console.log(res, res);
            if (res.data && res.data.dataList && res.data.dataList.length) {
              this.tableData = res.data.dataList;
              this.pagination.total = res.data.total;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    setTableHeight() {
      this.$nextTick(() => {
        const container = this.$refs.container;
        if (container) {
          const style = window.getComputedStyle(container);
          const paddingTop = parseFloat(style.paddingTop) || 0;
          const paddingBottom = parseFloat(style.paddingBottom) || 0;
          const form = container.querySelector('.search-form');
          const formHeight = form ? form.offsetHeight : 0;
          const contentHeight = container.clientHeight - paddingTop - paddingBottom;
          const total = contentHeight - formHeight - 52 - 40; // 52为table组件中分页高度 40为dytable-view-container上下padding高度
          this.tableHeight = (total > 360 ? total : 360) + 'px';
        }
      });
    }
  },
  mounted() {
    this.fetchData();
    this.setTableHeight();
    window.addEventListener('resize', this.setTableHeight);
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.setTableHeight);
  }

};

</script>
<style lang="less" scoped>
.inboundManage {
  height: 100%;
  background-color: #ffffff;
}
.search-form {
  margin: 0px;
  padding: 20px 20px 0;
}
::v-deep .el-form-item--mini.el-form-item{
  margin-bottom: 10px !important;
}

::v-deep .el-table__fixed-right::before{
  background-color: #ffffff !important;
}
</style>
