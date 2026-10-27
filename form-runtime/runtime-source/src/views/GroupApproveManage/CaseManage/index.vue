<!--
 * @Description: 案件管理列表（我的案件、案件台账共用）
-->
<template>
  <div class="case-manage-page">
    <el-card class="case-manage-card" shadow="never">
      <el-form
        ref="searchForm"
        :model="searchForm"
        class="search-form"
        label-width="90px"
        inline
        @submit.native.prevent
      >
        <el-form-item label="编号：" prop="caseCode">
          <el-input
            v-model.trim="searchForm.caseCode"
            data-testid="8f3c2d7e-4b5a-4c9e-b8f1-3a7c9d2e6f4b"
            clearable
            placeholder="请输入"
            @keyup.enter.native="search"
          />
        </el-form-item>
        <el-form-item label="类型：" prop="caseType">
          <el-select v-model="searchForm.caseType" data-testid="1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d" clearable placeholder="请输入">
            <el-option
              v-for="item in caseTypeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="原告：" prop="plaintiffNames">
          <el-input
            v-model.trim="searchForm.plaintiffNames"
            data-testid="7c8d9e0f-1a2b-3c4d-5e6f-7a8b9c0d1e2f"
            clearable
            placeholder="请输入"
            @keyup.enter.native="search"
          />
        </el-form-item>
        <el-form-item label="被告：" prop="defendantNames">
          <el-input
            v-model.trim="searchForm.defendantNames"
            data-testid="2f3a4b5c-6d7e-8f9a-0b1c-2d3e4f5a6b7c"
            clearable
            placeholder="请输入"
            @keyup.enter.native="search"
          />
        </el-form-item>
        <el-form-item label="经办部门：" prop="departmentName">
          <el-input
            v-model.trim="searchForm.departmentName"
            data-testid="9a0b1c2d-3e4f-5a6b-7c8d-9e0f1a2b3c4d"
            clearable
            placeholder="请输入"
            @keyup.enter.native="search"
          />
        </el-form-item>
        <el-form-item label="发起时间：" prop="applyTimeRange" class="date-form-item">
          <el-date-picker
            v-model="searchForm.applyTimeRange"
            data-testid="4d5e6f7a-8b9c-0d1e-2f3a-4b5c6d7e8f9a"
            type="daterange"
            value-format="yyyy-MM-dd"
            range-separator="~"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            clearable
          />
        </el-form-item>
        <div class="search-actions">
          <el-button type="primary" data-testid="5a6b7c8d-9e0f-1a2b-3c4d-5e6f7a8b9c0d" @click="search">查询</el-button>
          <el-button
            v-permission="exportPermission"
            type="primary"
            data-testid="0e1f2a3b-4c5d-6e7f-8a9b-0c1d2e3f4a5b"
            @click="exportList"
          >导出</el-button>
        </div>
      </el-form>

      <dy-table
        ref="caseTable"
        data-testid="6b7c8d9e-0f1a-2b3c-4d5e-6f7a8b9c0d1e"
        :fetchData="fetchData"
        :keys="tableColumns"
        :list="tableData"
        :isPagination="true"
        :pagination="pagination"
        :height="tableHeight"
      />
    </el-card>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';

const CASE_TYPE_OPTIONS = [
  { label: '应诉案件', value: 'RESPOND' },
  { label: '拟起诉案件', value: 'PROSECUTION' },
  { label: '非诉讼案件', value: 'NON_LITIGATION' }
];

const CASE_STATUS_OPTIONS = [
  { label: '草稿', value: 'DRAFT' },
  { label: '审批中', value: 'IN_APPROVAL' },
  { label: '待填报', value: 'PENDING_FILL' },
  { label: '待跟进', value: 'PENDING_FOLLOW' },
  { label: '已完结', value: 'CLOSED' },
  { label: '驳回', value: 'REJECTED' },
  { label: '撤销', value: 'WITHDRAWN' },
  { label: '处理中', value: 'IN_HANDLING' }
];

function createSearchForm() {
  return {
    caseCode: '',
    caseType: '',
    plaintiffNames: '',
    defendantNames: '',
    departmentName: '',
    applyTimeRange: []
  };
}

export default {
  name: 'CaseManageList',
  components: { DyTable },
  props: {
    listType: {
      type: String,
      required: true,
      validator: value => ['myCase', 'caseLedger'].includes(value)
    }
  },
  data() {
    return {
      searchForm: createSearchForm(),
      tableData: [],
      pagination: {
        total: 0,
        pages: 1,
        current: 1,
        size: 10
      },
      tableHeight: 360,
      caseTypeOptions: CASE_TYPE_OPTIONS,
      caseStatusOptions: CASE_STATUS_OPTIONS,
      colKey: {
        caseCode: {
          label: '编号',
          minWidth: '170',
          showTooltip: true
        },
        projectName: {
          label: '项目名称',
          minWidth: '180',
          showTooltip: true
        },
        caseType: {
          label: '类型',
          minWidth: '110',
          handle: (scope, createElement) => createElement(
            'span',
            this.getOptionLabel(this.caseTypeOptions, scope.row.caseType)
          )
        },
        plaintiffNames: {
          label: '原告',
          minWidth: '150',
          showTooltip: true
        },
        defendantNames: {
          label: '被告',
          minWidth: '150',
          showTooltip: true
        },
        departmentName: {
          label: '经办部门',
          minWidth: '140',
          showTooltip: true
        },
        applicantName: {
          label: '创建人',
          minWidth: '120',
          showTooltip: true
        },
        handlerName: {
          label: '经办人',
          minWidth: '120',
          showTooltip: true
        },
        applyTime: {
          label: '发起时间',
          minWidth: '170',
          showTooltip: true
        },
        caseStatus: {
          label: '状态',
          minWidth: '100',
          handle: (scope, createElement) => {
            const click = event => this.openCaseDetail(scope.row, event);
            return createElement(
              'el-button',
              {
                props: { type: 'text' },
                class: 'case-status-text',
                attrs: { 'data-testid': '3f4a5b6c-7d8e-9f0a-1b2c-3d4e5f6a7b8c' },
                on: { click }
              },
              this.getStatusLabel(scope.row)
            );
          }
        }
        // ,
        // operation: {
        //   label: '操作',
        //   minWidth: '90',
        //   handle: (scope, createElement) => {
        //     const action = this.getCaseAction(scope.row);
        //     if (!action) return createElement('span');
        //     const click = event => this.openCaseAction(scope.row, action.type, event);
        //     // 根据操作类型使用不同的固定 UUID
        //     const testidMap = {
        //       fill: '8c9d0e1f-2a3b-4c5d-6e7f-8a9b0c1d2e3f',
        //       follow: '1f2a3b4c-5d6e-7f8a-9b0c-1d2e3f4a5b6c'
        //     };
        //     return createElement(
        //       'el-button',
        //       {
        //         props: { type: 'text' },
        //         class: 'case-operation-text',
        //         attrs: { 'data-testid': testidMap[action.type] || '9a0b1c2d-3e4f-5a6b-7c8d-9e0f1a2b3c4d' },
        //         on: { click }
        //       },
        //       action.label
        //     );
        //   }
        // }
      }
    };
  },
  computed: {
    tableColumns() {
      return {
        ...this.colKey,
        applicantName: {
          ...this.colKey.applicantName,
          noShowColumn: this.listType !== 'myCase'
        },
        handlerName: {
          ...this.colKey.handlerName,
          noShowColumn: this.listType !== 'myCase'
        },
        operation: {
          ...this.colKey.operation,
          noShowColumn: this.listType !== 'myCase'
        }
      };
    },
    listApi() {
      return this.listType === 'myCase'
        ? Api.caseManage.myCaseList
        : Api.caseManage.caseLedgerList;
    },
    exportApi() {
      return this.listType === 'myCase'
        ? Api.caseManage.exportMyCase
        : Api.caseManage.exportCaseLedger;
    },
    exportPermission() {
      return this.listType === 'myCase'
        ? '/caseManage/myCase/export'
        : '/caseManage/checkList/export';
    },
    exportFileName() {
      return this.listType === 'myCase' ? '我的案件.xlsx' : '案件台账.xlsx';
    }
  },
  watch: {
    listType() {
      this.tableData = [];
      this.pagination.pages = 1;
      this.pagination.current = 1;
      this.fetchData();
      this.updateTableHeight();
    }
  },
  mounted() {
    this.updateTableHeight();
    window.addEventListener('resize', this.updateTableHeight);
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.updateTableHeight);
  },
  methods: {
    updateTableHeight() {
      this.$nextTick(() => {
        const table = this.$refs.caseTable;
        if (!table || !table.$el) return;
        const tableBody = table.$el.querySelector('.dytable-view-body');
        if (!tableBody) return;
        const tableTop = tableBody.getBoundingClientRect().top;
        this.tableHeight = Math.max(window.innerHeight - tableTop - 100, 240);
      });
    },
    getOptionLabel(options, value) {
      const option = options.find(item => item.value === value);
      return option ? option.label : (value || '');
    },
    getStatusLabel(row) {
      return row.caseStatusName || this.getOptionLabel(this.caseStatusOptions, row.caseStatus);
    },
    buildRequestData(pagination) {
      const data = {};
      Object.keys(this.searchForm).forEach(key => {
        if (key !== 'applyTimeRange' && this.searchForm[key] !== '') {
          data[key] = this.searchForm[key];
        }
      });

      const applyTimeRange = this.searchForm.applyTimeRange || [];
      return {
        platformCode: '200001',
        pagination,
        current: this.pagination.pages,
        size: this.pagination.size,
        applyTimeStart: applyTimeRange.length ? `${applyTimeRange[0]} 00:00:00` : '',
        applyTimeEnd: applyTimeRange.length ? `${applyTimeRange[1]} 23:59:59` : '',
        data
      };
    },
    fetchData() {
      const requestData = this.buildRequestData(true);
      this.$axios.post(this.listApi, requestData, res => {
        if (!res || !res.isSuccess) return;
        this.tableData = Array.isArray(res.data) ? res.data : [];
        this.pagination.total = Number(res.total) || 0;
        this.pagination.pages = Number(res.current) || 1;
        this.pagination.current = this.pagination.pages;
      }).catch(() => {});
    },
    search() {
      this.pagination.pages = 1;
      this.fetchData();
    },
    getCaseAction(row) {
      if (row.canFill === true) return { type: 'fill', label: '填写' };
      if (row.canFollow === true || row.canComplete === true) {
        return { type: 'follow', label: '跟进' };
      }
      return null;
    },
    getCaseFlowType(row) {
      if (row.flowType) return row.flowType;
      if (row.caseType === 'RESPOND') {
        return row.entryType === 'BUSINESS' ? 'lawsuit_biz' : 'lawsuit_legal';
      }
      return row.caseType === 'PROSECUTION' ? 'lawsuit_plan' : 'non_lawsuit';
    },
    getFlowInstanceByCaseId(row) {
      const flowType = this.getCaseFlowType(row);
      const data = {
        useScope: 'invest',
        initiator: 'all',
        auditWayList: [flowType],
        flowInstanceBizRelevanceList: [{
          otherBiz: flowType,
          otherBizId: row.id
        }]
      };
      return new Promise(resolve => {
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          { data, pagination: false },
          res => {
            if (!res || !res.isSuccess) {
              resolve(null);
              return;
            }
            const list = Array.isArray(res.data)
              ? res.data
              : (res.data && res.data.dataList) || [];
            resolve(list[0] || null);
          }
        ).catch(() => resolve(null));
      });
    },
    async openCaseDetail(row, event) {
      if (event && event.stopPropagation) event.stopPropagation();
      if (!row || !row.id) return;
      const flowData = await this.getFlowInstanceByCaseId(row);
      const flowInstanceId = flowData && (
        flowData.flowInstanceId ||
        flowData.id ||
        (flowData.flowInstanceBizRelevanceList || []).find(item => item.flowInstanceId)?.flowInstanceId
      );
      if (!flowInstanceId || !this.$flowDetail) {
        this.$message.warning('未找到该案件关联的流程');
        return;
      }
      this.$flowDetail({
        data: {
          id: flowInstanceId,
          flowInstanceBizRelevanceList: [{
            otherBiz: this.getCaseFlowType(row),
            otherBizId: row.id
          }]
        }
      });
    },
    openCaseAction(row, actionType, event) {
      if (event && event.stopPropagation) event.stopPropagation();
      if (!row || !row.flowInstanceId) {
        this.$message.warning('当前案件暂无可打开的流程表单');
        return;
      }
      this.$router.push({
        path: '/groupApproveManage/index',
        query: {
          tab: 'dueout',
          id: row.flowInstanceId,
          fromCase: '1',
          caseAction: actionType
        }
      });
    },
    exportList() {
      const requestData = this.buildRequestData(false);
      this.$axios.post(this.exportApi, requestData, (blob, response) => {
        if (!(blob instanceof Blob)) return;
        const fileName = this.getExportFileName(response) || this.exportFileName;
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.style.display = 'none';
        link.href = url;
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      }, '', { responseType: 'blob' }).catch(() => {});
    },
    getExportFileName(response) {
      if (!response || !response.headers) return '';
      const disposition = response.headers['content-disposition'] || '';
      const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
      if (utf8Match) {
        try {
          return decodeURIComponent(utf8Match[1]);
        } catch (error) {
          return utf8Match[1];
        }
      }
      const normalMatch = disposition.match(/filename="?([^";]+)"?/i);
      return normalMatch ? normalMatch[1] : '';
    }
  }
};
</script>

<style lang="scss" scoped>
.case-manage-page {
  height: 100%;
  padding: 20px;
  box-sizing: border-box;
  background: #f2f3f5;
  overflow: hidden;
}

.case-manage-card {
  height: 100%;

  ::v-deep .el-card__body {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 24px 20px 20px;
    box-sizing: border-box;
  }
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  padding-bottom: 4px;

  .el-form-item {
    width: 25%;
    min-width: 265px;
    margin-right: 0;
    padding-right: 20px;
    box-sizing: border-box;

    ::v-deep .el-form-item__content {
      width: calc(100% - 90px);
    }

    ::v-deep .el-input,
    ::v-deep .el-select {
      width: 100%;
    }
  }

  .date-form-item {
    ::v-deep .el-date-editor {
      width: 100%;
    }
  }
}

.search-actions {
  display: flex;
  align-items: center;
  width: 25%;
  min-width: 350px;
  margin: 0 0 22px;
  padding-left: 90px;
  box-sizing: border-box;
}

::v-deep .dytable-view-container {
  flex: 1;
  min-height: 0;
  padding: 0;
}

::v-deep .el-table__header-wrapper th {
  font-weight: 700;
  color: #333;
  background: #fafafa;
}

::v-deep .case-status-text,
::v-deep .case-operation-text {
  color: #1890ff;
}

@media screen and (max-width: 1440px) {
  .search-form .el-form-item,
  .search-actions {
    width: 33.3333%;
  }
}
</style>
