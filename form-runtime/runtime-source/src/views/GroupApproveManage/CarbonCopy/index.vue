<template>
  <div class="carbon-copy-page">
    <el-form :inline="true" :model="filters" class="search-form" @submit.native.prevent="search">
      <el-form-item label="流程名称">
        <el-input v-model.trim="filters.name" clearable placeholder="输入流程名称" @keyup.enter.native="search" />
      </el-form-item>
      <el-form-item label="流程状态">
        <el-select v-model="filters.status" clearable placeholder="全部状态">
          <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="最近抄送时间">
        <el-date-picker
          v-model="dateRange"
          type="datetimerange"
          value-format="yyyy-MM-dd HH:mm:ss"
          format="yyyy-MM-dd HH:mm:ss"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          :default-time="['00:00:00', '23:59:59']"
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" icon="el-icon-search" @click="search">搜索</el-button>
        <el-button icon="el-icon-refresh" @click="reset">重置</el-button>
      </el-form-item>
    </el-form>

    <dy-table
      :fetch-data="fetchRows"
      :actions="actions"
      :keys="columns"
      :list="rows"
      :pagination="pagination"
      :height="height"
      :loading="loading"
      :is-pagination="true"
      :resizable="true"
      show-check-box
    />

    <CheckFlowNodeDetail
      v-if="flowDetailVisible"
      :dialog-visible.sync="flowDetailVisible"
      :flow-instance-id="flowInstanceId"
      :flow-id="flowProxyId"
      :initiator-id="flowInitiatorId"
      :initiator-name="flowInitiatorName"
      :carbon-copy-flow="true"
    />

    <el-dialog title="抄送流程详情" :visible.sync="detailVisible" fullscreen append-to-body>
      <div v-loading="detailLoading" class="detail-content">
        <div v-if="detail" class="form-data">
          <div v-if="formTemplate" class="form-preview">
            <fm-generate-form
              ref="generateForm"
              :data="formTemplate.jsonData"
              :value="detail.formData || {}"
              :print-read="true"
              :custom-fields="customFields"
            />
          </div>
          <el-empty v-else :description="formTemplateError || '该抄送没有可查看的表单内容'" />
        </div>
      </div>
      <span slot="footer">
        <el-button @click="detailVisible = false">关闭</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
import customJson from '@/components/Custom/customJson';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';
import { approveManageFlowStatus } from '@/utils';

// 投资平台编码；axios 会将请求参数中的该值同时写入 URL 和 JSON Body。
const INVESTMENT_PLATFORM_CODE = '200001';

export default {
  name: 'MyCarbonCopy',
  components: { DyTable, CheckFlowNodeDetail },
  props: {
    initialFlowInstanceId: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      height: null,
      loading: false,
      detailLoading: false,
      detailVisible: false,
      flowDetailVisible: false,
      flowInstanceId: '',
      flowProxyId: '',
      flowInitiatorId: '',
      flowInitiatorName: '',
      rows: [],
      detail: null,
      formTemplate: null,
      formTemplateError: '',
      customFields: customJson,
      lastRequestedFlowInstanceId: '',
      filters: { name: '', status: '' },
      dateRange: [],
      pagination: { pages: 1, size: 10, total: 0 },
      statusOptions: [
        { label: '待发送', value: 'await_sent' },
        { label: '草稿', value: 'draft' },
        { label: '审批中', value: 'run' },
        { label: '已撤回', value: 'withdraw' },
        { label: '已终止', value: 'termination' },
        { label: '已放弃', value: 'abandon' },
        { label: '已驳回', value: 'rejected' },
        { label: '已结束', value: 'end' }
      ],
      columns: {
        name: {
          label: '标题',
          minWidth: '480',
          showTooltip: true,
          handle: (scope, createElement) => {
            const click = () => this.openDetail(scope.row);
            return createElement('el-link', { props: { type: 'primary', underline: false }, on: { click } }, scope.row.name || '未命名流程');
          }
        },
        status: {
          label: '状态',
          minWidth: '90',
          handle: (scope, createElement) => createElement('span', this.flowStatusText(scope.row.status))
        },
        initiatorName: {
          label: '发起人',
          minWidth: '120',
          showTooltip: true,
          handle: (scope, createElement) => createElement('span', scope.row.initiatorName || scope.row.initiatorUserName || scope.row.initiatorId || '-')
        },
        firstCopyTime: { label: '首次抄送时间', minWidth: '170', showTooltip: true },
        lastCopyTime: { label: '最近抄送时间', minWidth: '170', showTooltip: true },
        copyCount: { label: '抄送次数', width: '100' }
      },
      actions: [
        {
          label: '查看',
          width: '160',
          actionFixed: 'right',
          action: row => this.openDetail(row)
        },
        {
          label: '查看流程',
          actionFixed: 'right',
          action: row => this.openFlowDetail(row)
        }
      ]
    };
  },
  watch: {
    initialFlowInstanceId: {
      immediate: true,
      handler(id) {
        if (!id) return;
        this.openDetail({ flowInstanceId: id });
        this.$emit('initial-detail-opened', id);
      }
    }
  },
  mounted() {
    const container = document.querySelector('.content-container');
    this.height = container ? container.clientHeight - 170 : null;
  },
  methods: {
    flowStatusText(status) {
      const labels = {
        await_sent: '待发送',
        abandon: '已放弃'
      };
      return labels[status] || approveManageFlowStatus(status) || status || '-';
    },
    search() {
      if (this.dateRange && this.dateRange.length === 2 && this.dateRange[1] < this.dateRange[0]) {
        this.$message.warning('结束时间不能早于开始时间');
        return;
      }
      this.pagination.pages = 1;
      this.fetchRows();
    },
    reset() {
      this.filters = { name: '', status: '' };
      this.dateRange = [];
      this.search();
    },
    fetchRows() {
      this.loading = true;
      const data = {};
      if (this.filters.name) data.name = this.filters.name;
      if (this.filters.status) data.status = this.filters.status;
      if (this.dateRange && this.dateRange.length === 2) {
        data.startTime = this.dateRange[0];
        data.endTime = this.dateRange[1];
      }
      this.$axios.post(Api.flowCarbonCopy.list, {
        platformCode: INVESTMENT_PLATFORM_CODE,
        pagination: true,
        pages: this.pagination.pages,
        size: this.pagination.size,
        data
      }, res => {
        this.loading = false;
        if (!res || !res.isSuccess) {
          if (res) this.$message.error(res.message || '查询抄送列表失败');
          return;
        }
        this.rows = Array.isArray(res.data) ? res.data.map(row => Object.assign({}, row, {
          initiatorName: row.initiatorName || row.initiatorUserName || row.createrName || row.initiatorId || ''
        })) : [];
        this.pagination.total = Number(res.total || 0);
        this.pagination.pages = Number(res.current || this.pagination.pages);
        this.pagination.size = Number(res.size || this.pagination.size);
        this.resolveInitiatorNames();
      }).catch(() => {
        this.loading = false;
        this.$message.error('查询抄送列表失败，请检查网络后重试');
      });
    },
    resolveInitiatorNames() {
      const unresolvedIds = Array.from(new Set(this.rows
        .filter(row => row.initiatorId && row.initiatorName === row.initiatorId)
        .map(row => row.initiatorId)));
      unresolvedIds.forEach(id => {
        this.$axios.post(Api.user.getUserInfoById, { data: { id, flag: 'company' } }, res => {
          if (!res || !res.isSuccess || !res.data) return;
          const name = res.data.name || res.data.userName || res.data.realName;
          if (!name) return;
          this.rows = this.rows.map(row => row.initiatorId === id
            ? Object.assign({}, row, { initiatorName: name })
            : row);
        });
      });
    },
    openDetail(row) {
      const flowInstanceId = row && (row.flowInstanceId || row.processId);
      if (!flowInstanceId) {
        this.$message.warning('缺少流程实例编号');
        return;
      }
      this.detailVisible = true;
      this.detailLoading = true;
      this.detail = null;
      this.formTemplate = null;
      this.formTemplateError = '';
      this.lastRequestedFlowInstanceId = flowInstanceId;
      const data = { flowInstanceId };
      if (row.executionId) data.executionId = row.executionId;
      this.$axios.post(Api.flowCarbonCopy.detail, {
        platformCode: INVESTMENT_PLATFORM_CODE,
        data
      }, async res => {
        if (this.lastRequestedFlowInstanceId !== flowInstanceId) return;
        if (!res || !res.isSuccess) {
          this.detailLoading = false;
          this.$message.error((res && res.message) || '查询抄送详情失败');
          return;
        }
        this.detail = res.data || null;
        try {
          await this.loadFormTemplate(this.detail);
        } finally {
          if (this.lastRequestedFlowInstanceId === flowInstanceId) this.detailLoading = false;
        }
      }).catch(() => {
        if (this.lastRequestedFlowInstanceId !== flowInstanceId) return;
        this.detailLoading = false;
        this.$message.error('查询抄送详情失败，请检查网络后重试');
      });
    },
    loadFormTemplate(detail) {
      if (!detail || !detail.formProxyId) return Promise.resolve();
      return new Promise(resolve => {
        this.$axios.post(Api.qualityManage.getTaskFormDetail, {
          data: { id: detail.formProxyId }
        }, res => {
          const formProxy = res && res.isSuccess && res.data;
          const templateText = formProxy && formProxy.templateData;
          if (templateText) {
            try {
              const formJson = typeof templateText === 'string' ? JSON.parse(templateText) : templateText;
              const fields = Array.isArray(formProxy.fieldsTemplateList)
                ? formProxy.fieldsTemplateList
                  .map(item => (item.englishName || item.name || '').replaceAll('_$$_', '.'))
                  .filter(Boolean)
                : [];
              this.formTemplate = { name: formProxy.name || detail.name, jsonData: formJson, fields };
              this.$nextTick(() => {
                const form = this.$refs.generateForm;
                if (form && typeof form.refresh === 'function') form.refresh();
                if (form && fields.length && typeof form.disabled === 'function') form.disabled(fields, true);
                if (form && typeof form.setData === 'function') form.setData(detail.formData || {});
              });
            } catch (error) {
              this.formTemplateError = '流程表单模板格式无效，无法显示表单详情';
            }
          } else {
            this.formTemplateError = (res && res.message) || '获取流程表单模板失败';
          }
          resolve();
        }).catch(() => {
          this.formTemplateError = '获取流程表单模板失败，请检查网络后重试';
          resolve();
        });
      });
    },
    openFlowDetail(row) {
      const flowInstanceId = row && (row.flowInstanceId || row.processId);
      if (!flowInstanceId) {
        this.$message.warning('缺少流程实例编号');
        return;
      }
      if (!row.flowProxyId) {
        this.$message.warning('缺少流程代理编号，无法查看流程');
        return;
      }
      this.flowInstanceId = flowInstanceId;
      this.flowProxyId = row.flowProxyId || '';
      this.flowInitiatorId = row.initiatorId || '';
      this.flowInitiatorName = row.initiatorName || row.initiatorUserName || row.createrName || '';
      this.flowDetailVisible = true;
    }
  }
};
</script>

<style scoped lang="scss">
.carbon-copy-page {
  height: 100%;
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  padding: 12px 10px 0;
}

.detail-content {
  min-height: 180px;
}

.form-data {
  margin-top: 20px;
}

.form-preview {
  max-width: 1200px;
  margin: 0 auto;
}

</style>
