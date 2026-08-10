<template>
  <div class="content-container" ref="container">
    <!-- 筛选表单 -->
    <el-form :inline="true" class="filter-form">
      <el-form-item label="公司">
        <el-select v-model="filters.company" placeholder="请选择" clearable style="width: 240px;">
          <el-option v-for="item in companyOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="年度">
        <el-date-picker
          v-model="filters.year"
          type="year"
          placeholder="选择年"
          value-format="yyyy"
          style="width: 120px;">
        </el-date-picker>
      </el-form-item>
      <el-form-item label="季度" v-if="filters.year">
        <el-select v-model="filters.quarter" placeholder="请选择" style="width: 120px;">
          <el-option
            v-for="item in quarterOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="状态">
        <el-select v-model="filters.status" clearable placeholder="请选择">
          <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="onSearch">查询</el-button>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="handleAdd">发起公示</el-button>
      </el-form-item>
    </el-form>

    <!-- dy-table 表格 -->
    <dy-table
      ref="dyTable"
      :fetchData="fetchData"
      :keys="colKey"
      :actions="actionKey"
      :list="tableData"
      :isPagination="true"
      :height="tableHeight"
      :pagination="pagination"
    />
    <announcement-dialog :visible.sync="dialogVisible" :type="type" :id="currentData.id" :companyOptions="companyOptions"/>
  </div>
</template>

<script>
import Api from '@/api';
import dayjs from 'dayjs';
import DyTable from '@/components/DyTable';
import { localstorageGet } from '@/utils/auth';
import AnnouncementDialog from './components/announcementDialog.vue';

export default {
  name: 'Announcement',
  components: { DyTable, AnnouncementDialog },
  data() {
    return {
      type: 'add', // add edit
      currentData: {},
      dialogVisible: false,
      filters: {
        company: '',
        year: '',
        quarter: '',
        status: ''
      },
      companyOptions: [],
      quarterOptions: [
        { label: '一季度', value: '1' },
        { label: '二季度', value: '2' },
        { label: '三季度', value: '3' },
        { label: '四季度', value: '4' }
      ],
      statusOptions: [
        { label: '未公示', value: 'not_started' },
        { label: '公示中', value: 'in_progress' },
        { label: '公示结束', value: 'end' }
      ],
      tableData: [],
      colKey: {
        companyName: {
          label: '公司',
          minWidth: '180',
          showTooltip: true
        },
        year: {
          label: '年度',
          minWidth: '80',
          handle: (scope, h) => {
            const targetTime = scope.row.targetTime.toString();
            return h('span', `${targetTime.slice(0, targetTime.length - 1)}年`);
          }
        },
        quarter: {
          label: '季度',
          minWidth: '80',
          handle: (scope, h) => {
            const targetTime = scope.row.targetTime.toString();
            let quarter = '';
            switch (targetTime.slice(targetTime.length - 1, targetTime.length)) {
              case '1':
                quarter = '第一季度';
                break;
              case '2':
                quarter = '第二季度';
                break;
              case '3':
                quarter = '第三季度';
                break;
              case '4':
                quarter = '第四季度';
                break;
            };
            return h('span', quarter);
          }
        },
        planUserGroupName: {
          label: '考核组',
          minWidth: '100',
          showTooltip: true
        },
        updateDate: {
          label: '公示时间',
          minWidth: '120',
          handle: (scope, h) => {
            return h('span', `${dayjs(scope.row.startTime).format('YYYY-MM-DD')} 至 ${dayjs(scope.row.endTime).format('YYYY-MM-DD')}`);
          }
        },
        action: {
          label: '状态',
          minWidth: '100',
          handle: (scope, h) => {
            if (scope.row.noticeStatus === 'not_started') {
              return h('el-tag', { props: { type: 'info' }}, '未公示');
            } else if (scope.row.noticeStatus === 'in_progress') {
              return h('el-tag', { props: { type: 'primary' }}, '公示中');
            } else if (scope.row.noticeStatus === 'end') {
              return h('el-tag', { props: { type: 'success' }}, '公示结束');
            }
          },
          ifFixed: 'right'
        }
      },
      actionKey: [
        {
          label: '详情',
          width: '100',
          actionFixed: 'right',
          action: row => {
            this.handleView(row);
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
    // 获取公司列表
    getCompanyList() {
      this.$axios.post(Api.recruit.getCompanyList,
        { data: { id: localstorageGet('companyId') }},
        res => {
          this.companyOptions = [];
          this.filters.company = '';
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              var cList = [];
              res.data.forEach(item => {
                if (item.parentId) {
                  cList.push(item);
                } else {
                  cList.unshift(item);
                }
              });
              this.companyOptions = cList;
            } else {
              this.companyOptions = [
                {
                  id: localstorageGet('companyId'),
                  name: localstorageGet('companyName')
                }
              ];
            }
          } else {
            this.$message.error(`获取公司列表失败：${res.message}`);
          }
        }
      );
    },
    onSearch() {
      // 查询逻辑，调用fetchData
      this.pagination.pages = 1;
      this.fetchData();
    },
    fetchData() {
      let targetTime = '';
      if (this.filters.year) {
        if (this.filters.quarter) {
          targetTime = `${this.filters.year}${this.filters.quarter}`;
        } else {
          targetTime = this.filters.year;
        }
      }
      this.$axios.post(Api.newPerformance.workPlanPromulgateList,
        {
          data: {
            companyId: this.filters.company,
            noticeStatus: this.filters.status ? this.filters.status : null,
            targetTime
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
            if (res.data && res.data.length) {
              this.tableData = res.data;
              this.pagination.total = res.total;
            }
          } else {
            this.$message.error(`获取分组设置失败：${res.message}`);
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
          const form = container.querySelector('.filter-form');
          const formHeight = form ? form.offsetHeight : 0;
          const contentHeight = container.clientHeight - paddingTop - paddingBottom;
          const total = contentHeight - formHeight - 52 - 40; // 52为table组件中分页高度 40为dytable-view-container上下padding高度
          this.tableHeight = (total > 360 ? total : 360) + 'px';
        }
      });
    },
    handleAdd() {
      // 新增
      this.type = 'add';
      this.currentData = {};
      this.dialogVisible = true;
    },
    handleView(row) {
      // 编辑
      this.type = 'edit';
      this.currentData = row;
      this.dialogVisible = true;
      console.log('当前行数据:', row);
    }
  },
  mounted() {
    this.getCompanyList();
    this.fetchData();
    this.setTableHeight();
    window.addEventListener('resize', this.setTableHeight);
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.setTableHeight);
  }
};
</script>

<style lang='scss' scoped>
.content-container {
  background: #fff;
  padding: 10px;
  height: 100%;
}
.filter-form {
  margin-bottom: 0px;
  margin-left: 20px;
}
::v-deep .el-form-item--mini.el-form-item{
  margin-bottom: 10px !important;
}
</style>
