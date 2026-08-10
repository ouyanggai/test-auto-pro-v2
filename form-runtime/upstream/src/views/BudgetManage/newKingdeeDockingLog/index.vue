<template>
  <div class="inboundManage" ref="container">
    <div class="log-layout">
      <div class="company-tree-panel" :class="{ collapsed: companyTreeCollapsed }">
        <template v-if="!companyTreeCollapsed">
          <div class="tree-title">
            <span>公司列表</span>
            <el-button type="text" size="mini" @click="toggleCompanyTree">收起</el-button>
          </div>
          <el-tree
            v-if="companyTree.length"
            ref="companyTree"
            class="company-tree"
            :data="companyTree"
            node-key="id"
            default-expand-all
            highlight-current
            :props="companyTreeProps"
            @node-click="handleCompanyNodeClick"
          />
          <el-empty v-else description="暂无可查看公司" :image-size="80" />
        </template>
        <div v-else class="tree-collapsed-trigger" @click="toggleCompanyTree">
          <i class="el-icon-s-unfold"></i>
          <span>公司</span>
        </div>
      </div>

      <div class="log-content">
        <el-form :inline="true" class="search-form">
          <el-form-item label="流程名称">
            <el-input
              v-model="searchForm.keyword"
              clearable
              placeholder="请输入流程名称"
              style="width: 200px"
              @keyup.enter.native="onSearch"
            />
          </el-form-item>
          <el-form-item label="业务ID">
            <el-input
              v-model.trim="searchForm.businessDataId"
              clearable
              placeholder="请输入业务ID"
              style="width: 240px"
              @keyup.enter.native="onSearch"
            />
          </el-form-item>
          <el-form-item label="推送成功编号">
            <el-input
              v-model="searchForm.kingdeeBillNo"
              clearable
              placeholder="请输入推送成功编号"
              style="width: 200px"
              @keyup.enter.native="onSearch"
            />
          </el-form-item>
          <el-form-item label="推送状态">
            <el-select
              v-model="searchForm.pushStatus"
              clearable
              placeholder="请选择推送状态"
            >
              <el-option
                v-for="item in statusOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="推送时间">
            <el-date-picker
              v-model="searchForm.createTimeRange"
              type="daterange"
              clearable
              range-separator="至"
              start-placeholder="开始时间"
              end-placeholder="结束时间"
              value-format="yyyy-MM-dd"
              style="width: 220px"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="onSearch">查询</el-button>
          </el-form-item>
          <el-form-item>
            <el-button @click="onReset">重置</el-button>
          </el-form-item>
        </el-form>

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
      </div>
    </div>

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
  name: 'KingdeeDockingLog',
  components: { DyTable, kingdeeDockingLogDialog },
  data() {
    return {
      dialogVisible: false,
      logData: null,
      statusOptions: [
        { value: 'SUCCESS', label: '成功' },
        { value: 'PENDING', label: '处理中' },
        { value: 'FAILED', label: '失败' }
      ],
      searchForm: {
        keyword: '',
        businessDataId: '',
        kingdeeBillNo: '',
        pushStatus: '',
        createTimeRange: []
      },
      tableData: [],
      selectedCompanyId: '',
      selectedCompanyIds: [],
      companyTreeCollapsed: false,
      companyTree: [],
      companyNameMap: {},
      companyTreeProps: {
        children: 'children',
        label: 'name'
      },
      colKey: {
        businessDataId: {
          label: '业务ID',
          minWidth: '220',
          showTooltip: true
        },
        createDate: {
          label: '创建时间',
          minWidth: '150',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', this.formatDateTime(scope.row.createDate));
          }
        },
        companyName: {
          label: '公司',
          minWidth: '220',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', this.resolveRowCompanyName(scope.row));
          }
        },
        flowName: {
          label: '流程名称',
          minWidth: '320',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', scope.row.flowName || '-');
          }
        },
        documentType: {
          label: '单据类型',
          minWidth: '100',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', scope.row.documentType || '-');
          }
        },
        kingdeeBillNo: {
          label: '推送成功编号',
          minWidth: '140',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', scope.row.kingdeeBillNo || '-');
          }
        },
        pushTime: {
          label: '推送时间',
          minWidth: '150',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', scope.row.pushTime || '-');
          }
        },
        pushStatusValue: {
          label: '状态',
          minWidth: '80',
          handle: (scope, h) => this.renderStatusTag(scope.row, h)
        },
        retryCount: {
          label: '重试次数',
          minWidth: '80',
          handle: (scope, h) => {
            return h('span', `${scope.row.retryCount || 0}`);
          }
        },
        failReason: {
          label: '失败原因',
          minWidth: '220',
          showTooltip: true,
          handle: (scope, h) => {
            return h('span', scope.row.failReason || '-');
          }
        }
      },
      actionKey: [
        {
          width: '240',
          actionFixed: 'right',
          handle: (scope, h) => this.renderActionButtons(scope.row, h)
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      tableHeight: '400px'
    };
  },
  methods: {
    onReset() {
      this.searchForm = {
        keyword: '',
        businessDataId: '',
        kingdeeBillNo: '',
        pushStatus: '',
        createTimeRange: []
      };
      this.selectedCompanyId = '';
      this.selectedCompanyIds = [];
      this.pagination.pages = 1;
      this.$nextTick(() => {
        if (this.$refs.companyTree && this.companyTree.length) {
          this.$refs.companyTree.setCurrentKey('');
        }
      });
      this.fetchData();
    },
    onSearch() {
      this.pagination.pages = 1;
      this.fetchData();
    },
    fetchData() {
      const [createTimeStart, createTimeEnd] = this.searchForm.createTimeRange || [];
      this.$axios.post(
        Api.kingdee.pushRecordManagementList,
        {
          data: {
            keyword: this.searchForm.keyword,
            businessDataId: this.searchForm.businessDataId,
            kingdeeBillNo: this.searchForm.kingdeeBillNo,
            pushStatus: this.searchForm.pushStatus,
            companyId: this.selectedCompanyIds.length
              ? this.selectedCompanyIds.join(',')
              : this.selectedCompanyId,
            customerCode: this.$store.state.user.customerCode,
            createTimeStart: createTimeStart || '',
            createTimeEnd: createTimeEnd || ''
          },
          pagination: true,
          current: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          this.tableData = Array.isArray(res.data) ? res.data : [];
          this.pagination.total = Number(res.total) || 0;
          this.pagination.pages = Number(res.current) || this.pagination.pages;
          this.pagination.size = Number(res.size) || this.pagination.size;
        }
      );
    },
    initCompanyTree() {
      return this.fetchCurrentUserLogPermission().then(permission => {
        return this.fetchCompanyTree(permission).then(tree => {
          this.selectedCompanyId = '';
          this.selectedCompanyIds = [];
          this.companyTree = this.buildPermissionCompanyTree(tree, permission);
          this.companyNameMap = this.buildCompanyNameMap(tree);
          this.$nextTick(() => {
            if (this.$refs.companyTree && this.companyTree.length) {
              this.$refs.companyTree.setCurrentKey('');
            }
          });
        });
      });
    },
    fetchCompanyTree(permission) {
      if (!permission) {
        return Promise.resolve([]);
      }
      const companyTreePromise = this.$axios.post(
        '/web/user/api/company/children',
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            flag: 3,
            id: this.$store.state.user.companyId
          }
        },
        null,
        false,
        { noLoading: true }
      ).then(res => {
        return res && res.isSuccess ? this.filterCompanyNodes(res.data || []) : [];
      }).catch(() => []);

      return Promise.all([companyTreePromise, this.fetchProjectCompanyList()])
        .then(([tree, projectCompanies]) => this.appendProjectCompaniesToTree(tree, projectCompanies));
    },
    fetchCurrentUserLogPermission() {
      return this.$axios.post(
        Api.kingdee.pushRecordLogPermissionList,
        {
          pagination: false,
          data: {
            customerCode: this.$store.state.user.customerCode
          }
        },
        null,
        false,
        { noLoading: true }
      ).then(res => {
        const list = res && res.isSuccess && Array.isArray(res.data) ? res.data : [];
        return list.find(item => item.userId === this.$store.state.user.userId) || null;
      }).catch(() => null);
    },
    buildPermissionCompanyTree(tree, permission) {
      if (!permission) {
        return [];
      }
      const companyIds = permission.companyIds || [];
      if (permission.scopeType === 'ALL') {
        return tree;
      }
      if (!companyIds.length) {
        return [];
      }
      const companyIdSet = new Set(companyIds);
      const matchedTree = this.filterCompanyTreeByIds(tree, companyIdSet);
      if (matchedTree.length) {
        return matchedTree;
      }
      return companyIds.map(id => ({
        id,
        name: id
      }));
    },
    filterCompanyTreeByIds(nodes, companyIdSet) {
      const result = [];
      nodes.forEach(node => {
        if (companyIdSet.has(node.id)) {
          result.push({
            id: node.id,
            name: node.name,
            type: node.type,
            children: node.children || []
          });
          return;
        }
        const children = node.children && node.children.length
          ? this.filterCompanyTreeByIds(node.children, companyIdSet)
          : [];
        if (companyIdSet.has(node.id) || children.length) {
          result.push({
            id: node.id,
            name: node.name,
            children
          });
        }
      });
      return result;
    },
    collectCompanyIds(nodes) {
      const result = [];
      const walk = list => {
        (list || []).forEach(node => {
          if (node.id && result.indexOf(node.id) === -1) {
            result.push(node.id);
          }
          if (node.children && node.children.length) {
            walk(node.children);
          }
        });
      };
      walk(nodes);
      return result;
    },
    filterCompanyNodes(nodes) {
      return nodes
        .filter(node => node.type !== '5' && node.type !== '2' && node.type !== 'department')
        .map(node => {
          const next = {
            id: node.id,
            name: node.name || node.departmentName
          };
          const children = node.childrenList || node.children || [];
          if (children.length) {
            next.children = this.filterCompanyNodes(children);
          }
          return next;
        });
    },
    fetchProjectCompanyList() {
      return this.$axios.post(
        Api.budgetManage.projectCompany,
        {
          pagination: false,
          data: {
            companyId: '',
            name: ''
          }
        },
        null,
        false,
        { noLoading: true }
      ).then(res => {
        if (!res || !res.isSuccess) {
          return [];
        }
        return res.data && Array.isArray(res.data.dataList) ? res.data.dataList : [];
      }).catch(() => []);
    },
    appendProjectCompaniesToTree(tree, projectCompanies) {
      if (!projectCompanies || !projectCompanies.length) {
        return tree;
      }
      const projectCompanyMap = {};
      projectCompanies.forEach(item => {
        if (!item || !item.companyId || !item.id) {
          return;
        }
        if (!projectCompanyMap[item.companyId]) {
          projectCompanyMap[item.companyId] = [];
        }
        projectCompanyMap[item.companyId].push({
          id: item.id,
          name: item.name,
          type: 'projectCompany'
        });
      });
      const append = nodes => {
        return (nodes || []).map(node => {
          const children = append(node.children || []);
          const projectChildren = projectCompanyMap[node.id] || [];
          return {
            ...node,
            children: children.concat(projectChildren)
          };
        });
      };
      return append(tree);
    },
    buildCompanyNameMap(tree) {
      const result = {};
      const walk = nodes => {
        (nodes || []).forEach(node => {
          if (node.id) {
            result[node.id] = node.name || node.id;
          }
          if (node.children && node.children.length) {
            walk(node.children);
          }
        });
      };
      walk(tree);
      return result;
    },
    resolveRowCompanyName(row) {
      if (!row) {
        return '-';
      }
      if (row.companyId && this.companyNameMap[row.companyId]) {
        return this.companyNameMap[row.companyId];
      }
      return this.resolveCompanyNameFromBusinessData(row.businessDataJson) || row.companyId || '-';
    },
    resolveCompanyNameFromBusinessData(businessDataJson) {
      if (!businessDataJson) {
        return '';
      }
      try {
        const payload = typeof businessDataJson === 'string' ? JSON.parse(businessDataJson) : businessDataJson;
        const data = payload && payload.data ? payload.data : payload;
        return data.companyName ||
          data.expenseCompanyName ||
          data.payCompanyName ||
          data.collectCompanyName ||
          data.invoicingCompanyName ||
          '';
      } catch (e) {
        return '';
      }
    },
    formatDateTime(value) {
      if (!value) {
        return '-';
      }
      if (typeof value === 'string') {
        return value;
      }
      if (typeof value === 'number') {
        return this.formatDateObject(new Date(value));
      }
      if (Array.isArray(value)) {
        const [year, month, day, hour = 0, minute = 0, second = 0] = value;
        return this.formatDateObject(new Date(year, month - 1, day, hour, minute, second));
      }
      return this.formatDateObject(new Date(value));
    },
    formatDateObject(date) {
      if (!date || Number.isNaN(date.getTime())) {
        return '-';
      }
      const pad = number => `${number}`.padStart(2, '0');
      return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
        `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
    },
    handleCompanyNodeClick(node) {
      const selectedIds = node.id ? this.collectCompanyIds([node]) : [];
      const allVisibleCompanyIds = this.collectCompanyIds(this.companyTree);
      const isAllVisibleScope = selectedIds.length &&
        allVisibleCompanyIds.length &&
        allVisibleCompanyIds.every(id => selectedIds.indexOf(id) > -1);
      this.selectedCompanyId = isAllVisibleScope ? '' : (node.id || '');
      this.selectedCompanyIds = isAllVisibleScope ? [] : selectedIds;
      this.pagination.pages = 1;
      this.fetchData();
    },
    toggleCompanyTree() {
      this.companyTreeCollapsed = !this.companyTreeCollapsed;
      this.setTableHeight();
    },
    renderStatusTag(row, h) {
      const status = row.pushStatusValue || row.pushStatus;
      if (status === 'SUCCESS') {
        return h('el-tag', { props: { type: 'success' }}, '成功');
      }
      if (status === 'FAILED') {
        return h('el-tag', { props: { type: 'danger' }}, '失败');
      }
      if (status === 'PENDING') {
        return h('el-tag', { props: { type: 'info' }}, '处理中');
      }
      return h('span', status || '-');
    },
    renderActionButtons(row, h) {
      const buttons = [
        h(
          'el-button',
          {
            props: {
              type: 'text',
              size: 'small'
            },
            on: {
              click: e => {
                e.stopPropagation();
                this.handleFlowForm(row);
              }
            }
          },
          '表单详情'
        ),
        h(
          'el-button',
          {
            props: {
              type: 'text',
              size: 'small'
            },
            on: {
              click: e => {
                e.stopPropagation();
                this.handleInfo(row);
              }
            }
          },
          '日志详情'
        )
      ];

      if (['FAILED', 'PENDING'].includes(row.pushStatusValue) && Number(row.canRetry) === 1) {
        buttons.push(
          h(
            'el-button',
            {
              props: {
                type: 'text',
                size: 'small'
              },
              on: {
                click: e => {
                  e.stopPropagation();
                  this.handleRetry(row);
                }
              }
            },
            '重新推送'
          )
        );
      }

      return h('div', buttons);
    },
    handleFlowForm(row) {
      if (!row.flowInstanceId && !row.businessDataId) {
        this.$message.warning('该日志没有流程实例或业务数据信息，无法查看表单');
        return;
      }
      this.$flowDetail({
        data: {
          id: row.flowInstanceId || undefined,
          flowInstanceBizRelevanceList: row.businessDataId
            ? [{ otherBizId: row.businessDataId }]
            : undefined
        }
      });
    },
    handleInfo(row) {
      this.$axios.post(
        Api.kingdee.pushRecordManagementDetail,
        {
          data: {
            id: row.id
          }
        },
        res => {
          if (!res.isSuccess) return;
          this.logData = res.data || {
            record: row
          };
          this.dialogVisible = true;
        }
      );
    },
    handleRetry(row) {
      this.$confirm('确认重新推送该记录吗？', '提示', {
        type: 'warning',
        closeOnClickModal: false
      }).then(() => {
        this.$axios.post(
          Api.kingdee.pushRecordManagementRetry,
          {
            data: {
              id: row.id
            }
          },
          res => {
            if (!res.isSuccess) return;
            this.$message.success(res.message || '重新推送成功');
            this.fetchData();
          }
        );
      }).catch(() => {});
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
          const total = contentHeight - formHeight - 52 - 40;
          this.tableHeight = (total > 360 ? total : 360) + 'px';
        }
      });
    }
  },
  mounted() {
    this.initCompanyTree().finally(() => {
      this.fetchData();
    });
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

.log-layout {
  display: flex;
  height: 100%;
  min-height: 0;
}

.company-tree-panel {
  width: 260px;
  height: 100%;
  flex: 0 0 260px;
  border-right: 1px solid #ebeef5;
  background: #fbfcfe;
  overflow: auto;
  transition: width 0.2s ease, flex-basis 0.2s ease;

  &.collapsed {
    width: 40px;
    flex-basis: 40px;
    overflow: hidden;
  }
}

.tree-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  line-height: 48px;
  padding: 0 16px;
  color: #303133;
  font-weight: 600;
  border-bottom: 1px solid #ebeef5;
}

.tree-collapsed-trigger {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 14px;
  color: #409eff;
  cursor: pointer;
  user-select: none;

  i {
    margin-bottom: 8px;
    font-size: 18px;
  }

  span {
    width: 14px;
    line-height: 16px;
    font-size: 13px;
  }
}

.company-tree {
  padding: 10px 8px;
  background: transparent;
}

.log-content {
  flex: 1;
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

.search-form {
  margin: 0;
  padding: 20px 20px 0;
}

::v-deep .el-form-item--mini.el-form-item {
  margin-bottom: 10px !important;
}

::v-deep .el-table__fixed-right::before {
  background-color: #ffffff !important;
}
</style>
