<template>
  <div class="flow-detail-page">
    <div v-if="loading" class="loading-overlay">
      <div class="loading-container">
        <div class="loading-spinner">
          <div class="spinner-ring"></div>
          <div class="spinner-ring"></div>
          <div class="spinner-ring"></div>
        </div>
        <p class="loading-text">正在加载流程数据...</p>
      </div>
    </div>
    <div v-if="error" class="error-text">{{ error }}</div>
  </div>
</template>

<script>
export default {
  name: 'FlowDetail',
  data() {
    return {
      loading: true,
      error: '',
      dialogStyleObserver: null,
      dialogStyleRafId: 0
    };
  },
  computed: {
    strictEditAuditWaySet() {
      // 费用类无表单在审核态必须按更新语义提交，避免主表/子表 id 丢失触发后端重建
      return new Set(['expense_budget', 'travel_expense', 'request_funds', 'expense_loan', 'expense_repayment']);
    }
  },
  mounted() {
    this.startDialogStyleObserver();
    this.patchNestedFlowDetailDialogOpen();
    this.$nextTick(() => {
      this.openFlowDialog();
    });
  },
  beforeDestroy() {
    this.stopDialogStyleObserver();
  },
  methods: {
    isFixedNoFormFlow(formExist) {
      if (formExist === null || formExist === undefined) return false;
      if (typeof formExist === 'boolean') return formExist;
      if (typeof formExist === 'number') return formExist !== 0;
      if (typeof formExist === 'string') {
        const normalized = formExist.trim().toLowerCase();
        if (['', '0', 'false', 'no', 'off', '否', 'null', 'undefined'].includes(normalized)) {
          return false;
        }
      }
      return !!formExist;
    },
    shouldUseStrictEditForAudit(taskContext) {
      if (!taskContext || typeof taskContext !== 'object') return false;
      const auditWay = taskContext.auditWay || '';
      if (!this.strictEditAuditWaySet.has(auditWay)) return false;
      return this.isFixedNoFormFlow(taskContext.formExist);
    },
    normalizeTrackingValue(value, fallback) {
      const target = value === undefined || value === null ? fallback : value;
      if (target === undefined || target === null) {
        return false;
      }
      if (typeof target === 'boolean') {
        return target;
      }
      if (typeof target === 'number') {
        return target !== 0;
      }
      if (typeof target === 'string') {
        const normalized = target.trim().toLowerCase();
        if (['1', 'true', 'yes', 'y', 'on', '是'].includes(normalized)) {
          return true;
        }
        if (['0', 'false', 'no', 'n', 'off', '否', ''].includes(normalized)) {
          return false;
        }
      }
      return !!target;
    },
    getQueryParam(name) {
      const hash = window.location.hash || '';
      if (hash.includes('?')) {
        const params = new URLSearchParams(hash.split('?')[1]);
        const val = params.get(name);
        if (val) return val;
      }
      const urlParams = new URLSearchParams(window.location.search);
      return urlParams.get(name) || '';
    },
    mergeFlowRowData(baseRow, overrideRow) {
      const merged = { ...(baseRow || {}) };
      if (!overrideRow || typeof overrideRow !== 'object') {
        return merged;
      }

      Object.keys(overrideRow).forEach((key) => {
        const value = overrideRow[key];
        if (Array.isArray(value)) {
          if (value.length > 0) {
            merged[key] = value;
          }
          return;
        }
        if (value !== undefined && value !== null && value !== '') {
          merged[key] = value;
        }
      });

      return merged;
    },
    getExpenseCommonFlowAuditWay(row) {
      const relation = Array.isArray(row?.flowInstanceBizRelevanceList)
        ? row.flowInstanceBizRelevanceList.find(item => ['expense_budget', 'travel_expense'].includes(item.otherBiz))
        : null;
      return row?.auditWay || relation?.otherBiz || '';
    },
    isExpenseCommonFlowCompat(row) {
      return ['expense_budget', 'travel_expense'].includes(this.getExpenseCommonFlowAuditWay(row));
    },
    isExpenseCommonFlowEditMode(mode, taskContext) {
      return mode === 'edit' && this.isExpenseCommonFlowCompat(taskContext);
    },
    getTaskContext(flowInstanceId) {
      const rawTaskContext = this.$route.query.taskContext || this.getQueryParam('taskContext');
      if (rawTaskContext) {
        try {
          const parsed = JSON.parse(rawTaskContext);
          const flowName = parsed.flowName || parsed.formName || parsed.flowInstanceName || parsed.name || '';
          return {
            ...parsed,
            id: parsed.id || flowInstanceId,
            flowInstanceId: parsed.flowInstanceId || flowInstanceId,
            flowName,
            flowStatus: parsed.flowStatus || parsed.status || '',
          };
        } catch (error) {
          console.error('parse taskContext failed', error);
        }
      }

      const fieldNames = [
        'id',
        'flowInstanceName',
        'name',
        'title',
        'formName',
        'flowName',
        'status',
        'flowStatus',
        'auditWay',
        'initiatorId',
        'createrId',
        'jobTaskId',
        'batchNo',
        'formExist',
        'formProxyId',
        'flowProxyId',
        'flowNodeProxyId',
        'currentNodeProxyId',
        'flowNextNodeAuditType',
        'nextNodeProxyId',
        'nextNodeName',
        'nextNodeType',
        'nextAuditNodeList',
        'auditPassLogicFlag',
        'branchExecuteType',
        'lastCountersignFlag',
        'currentPendingNodeName',
        'tracking',
        'trackingFlag',
      ];
      const row = { id: flowInstanceId, flowInstanceId };
      fieldNames.forEach((field) => {
        const value = this.$route.query[field] || this.getQueryParam(field);
        if (value) {
          row[field] = value;
        }
      });
      return Object.keys(row).length > 2 ? row : null;
    },
    applyEditEntryContext(dialog, taskContext) {
      if (!dialog?.instance || !taskContext) {
        return;
      }

      const flowName = taskContext.flowName || taskContext.formName || taskContext.flowInstanceName || taskContext.name || '';
      dialog.instance.flowName = flowName;
      dialog.instance.selectFlowName = flowName;
      dialog.instance.flowStatus = taskContext.flowStatus || taskContext.status || '';
      dialog.instance.btnVisible = true;
    },
    applyExpenseCommonFlowEditContext(dialog) {
      if (!dialog?.instance || dialog.instance.__expenseCommonFlowEditPatched) {
        return;
      }

      const root = dialog.instance;
      const apply = (row) => {
        if (!this.isExpenseCommonFlowCompat(row)) {
          return;
        }
        root.isExamine = false;
        root.isReInitiate = true;
        if (Array.isArray(row?.flowInstanceBizRelevanceList) && row.flowInstanceBizRelevanceList.length) {
          root.flowInstanceBizRelevanceList = row.flowInstanceBizRelevanceList.map(item => ({ ...item }));
        }
      };

      const originalClickReInitiate = root.clickReInitiate;
      if (typeof originalClickReInitiate === 'function') {
        root.__expenseCommonFlowEditPatched = true;
        root.clickReInitiate = function(row, type) {
          const result = originalClickReInitiate.call(this, row, type);
          apply(row);
          return result;
        };
      }
    },
    applyIframeAuditContext(dialog) {
      if (!dialog?.instance) {
        return;
      }

      const apply = () => {
        const root = dialog.instance;
        if (!root || root._isDestroyed) {
          return;
        }

        root.operaType = 'edit';
        root.actionType = 'examine';
        root.isExamine = true;
        root.btnVisible = true;

        const applyToChildren = (component) => {
          if (!component || component._isDestroyed) {
            return;
          }
          const data = component.$data || {};
          if (Object.prototype.hasOwnProperty.call(data, 'operaType')) {
            component.operaType = 'edit';
          }
          if (Object.prototype.hasOwnProperty.call(data, 'actionType')) {
            component.actionType = 'examine';
          }
          if (Object.prototype.hasOwnProperty.call(data, 'isExamine')) {
            component.isExamine = true;
          }
          if (Object.prototype.hasOwnProperty.call(data, 'btnVisible')) {
            component.btnVisible = true;
          }
          (component.$children || []).forEach(applyToChildren);
        };

        applyToChildren(root);
      };

      const originalPreviewHandle = dialog.instance.previewHandle;
      if (typeof originalPreviewHandle === 'function' && !dialog.instance.__iframeAuditPatched) {
        dialog.instance.__iframeAuditPatched = true;
        dialog.instance.previewHandle = function(row) {
          const result = originalPreviewHandle.call(this, row);
          apply();
          return result;
        };
      }

      this.$nextTick(() => {
        apply();
        [0, 80, 240, 600, 1200].forEach((delay) => {
          setTimeout(apply, delay);
        });
      });
    },
    applyTrackingContext(dialog, taskContext) {
      if (!dialog?.instance) {
        return;
      }

      const tracking = this.normalizeTrackingValue(taskContext?.tracking, taskContext?.trackingFlag);
      const assignTracking = (component) => {
        if (!component || typeof component !== 'object') {
          return;
        }
        if (component.clickRow && typeof component.clickRow === 'object') {
          this.$set(component.clickRow, 'tracking', tracking);
        }
        if (component.$data && Object.prototype.hasOwnProperty.call(component.$data, 'outTracking')) {
          component.outTracking = tracking;
        }
      };
      const apply = () => {
        assignTracking(dialog.instance);
        (dialog.instance.$children || []).forEach(assignTracking);
      };

      this.$nextTick(() => {
        apply();
        setTimeout(apply, 0);
        setTimeout(apply, 80);
        setTimeout(apply, 240);
      });
    },
    startDialogStyleObserver() {
      if (this.dialogStyleObserver || typeof MutationObserver === 'undefined') {
        return;
      }
      this.syncPortalDialogStyles();
      this.dialogStyleObserver = new MutationObserver(() => {
        this.scheduleDialogStyleSync();
      });
      this.dialogStyleObserver.observe(document.body, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ['class', 'style']
      });
    },
    stopDialogStyleObserver() {
      if (this.dialogStyleRafId) {
        window.cancelAnimationFrame(this.dialogStyleRafId);
        this.dialogStyleRafId = 0;
      }
      if (this.dialogStyleObserver) {
        this.dialogStyleObserver.disconnect();
        this.dialogStyleObserver = null;
      }
      document.querySelectorAll('.flow-detail-hide-header, .flow-detail-countersign-dialog').forEach((wrapper) => {
        wrapper.classList.remove('flow-detail-hide-header', 'flow-detail-countersign-dialog');
      });
      document.querySelectorAll('.flow-detail-back-btn').forEach((button) => {
        button.remove();
      });
    },
    scheduleDialogStyleSync() {
      if (this.dialogStyleRafId) {
        window.cancelAnimationFrame(this.dialogStyleRafId);
      }
      this.dialogStyleRafId = window.requestAnimationFrame(() => {
        this.dialogStyleRafId = 0;
        this.syncPortalDialogStyles();
      });
    },
    syncPortalDialogStyles() {
      const mainFlowDialogWrappers = this.getMainFlowDialogWrappers();
      document.querySelectorAll('.el-dialog__wrapper').forEach((wrapper) => {
        this.applyBrowser360DialogClarityFix(wrapper);
        if (!this.shouldHideDialogHeader(wrapper)) {
          wrapper.classList.remove('flow-detail-hide-header', 'flow-detail-countersign-dialog');
          this.removeCounterSignBackButton(wrapper);
          return;
        }

        if (this.shouldKeepHeaderForNestedFlowDetail(wrapper, mainFlowDialogWrappers)) {
          wrapper.classList.remove('flow-detail-hide-header', 'flow-detail-countersign-dialog');
          this.removeCounterSignBackButton(wrapper);
          return;
        }

        wrapper.classList.add('flow-detail-hide-header');
        if (this.isCounterSignDialog(wrapper)) {
          wrapper.classList.add('flow-detail-countersign-dialog');
          this.ensureCounterSignBackButton(wrapper);
        } else {
          wrapper.classList.remove('flow-detail-countersign-dialog');
          this.removeCounterSignBackButton(wrapper);
        }
      });
    },
    getMainFlowDialogWrappers() {
      return Array.from(document.querySelectorAll('.el-dialog__wrapper'))
        .filter(wrapper => this.isMainFlowDialog(wrapper) && !this.isCounterSignDialog(wrapper));
    },
    shouldKeepHeaderForNestedFlowDetail(wrapper, mainFlowDialogWrappers) {
      if (wrapper.classList.contains('flow-detail-keep-header')) {
        return true;
      }
      if (!this.isMainFlowDialog(wrapper) || this.isCounterSignDialog(wrapper)) {
        return false;
      }
      return mainFlowDialogWrappers.indexOf(wrapper) > 0;
    },
    isBrowser360() {
      const ua = navigator.userAgent || '';
      return /360SE|360EE|QihooBrowser|QHBrowser|QIHU 360|360 Aphone Browser/i.test(ua);
    },
    applyBrowser360DialogClarityFix(wrapper) {
      if (!this.isBrowser360() || !this.isMainFlowDialog(wrapper)) {
        return;
      }

      const dialog = wrapper.querySelector('.el-dialog');
      if (!dialog || !this.hasFractionalZoom(dialog.style.zoom)) {
        return;
      }

      dialog.style.zoom = '1';
      dialog.classList.add('flow-detail-browser360-clear');
    },
    hasFractionalZoom(zoomValue) {
      if (!zoomValue) {
        return false;
      }
      const zoomNumber = Number(String(zoomValue).replace('%', ''));
      if (!Number.isFinite(zoomNumber)) {
        return false;
      }
      const normalizedZoom = String(zoomValue).includes('%') ? zoomNumber / 100 : zoomNumber;
      return normalizedZoom > 0 && normalizedZoom !== 1 && normalizedZoom % 1 !== 0;
    },
    shouldHideDialogHeader(wrapper) {
      return this.isMainFlowDialog(wrapper) || this.isCounterSignDialog(wrapper);
    },
    isMainFlowDialog(wrapper) {
      return Boolean(wrapper.querySelector('.el-dialog__body .examine-content'));
    },
    isCounterSignDialog(wrapper) {
      return Boolean(wrapper.querySelector('.el-dialog__body .flow-container .btn-container'));
    },
    ensureCounterSignBackButton(wrapper) {
      const header = wrapper.querySelector('.el-dialog__header');
      const closeButton = wrapper.querySelector('.el-dialog__headerbtn');
      if (!header || !closeButton || header.querySelector('.flow-detail-back-btn')) {
        return;
      }
      const backButton = document.createElement('button');
      backButton.type = 'button';
      backButton.className = 'flow-detail-back-btn';
      backButton.innerHTML = '<i class="el-icon-arrow-left"></i><span>返回流程</span>';
      backButton.addEventListener('click', () => {
        closeButton.click();
      });
      header.insertBefore(backButton, header.firstChild);
    },
    removeCounterSignBackButton(wrapper) {
      const backButton = wrapper.querySelector('.flow-detail-back-btn');
      if (backButton) {
        backButton.remove();
      }
    },
    patchNestedFlowDetailDialogOpen(retryCount = 0) {
      const flowManager = this.$fm2 || window.$vue?.$fm2;
      if (!flowManager || typeof flowManager.show !== 'function') {
        if (retryCount < 5) {
          setTimeout(() => this.patchNestedFlowDetailDialogOpen(retryCount + 1), 100);
        }
        return;
      }
      if (flowManager.__flowDetailKeepHeaderPatched) {
        return;
      }

      const originalShow = flowManager.show;
      const markNestedFlowDetail = this.markNestedFlowDetailDialog.bind(this);
      flowManager.show = async function(component, propData) {
        const instance = await originalShow.call(this, component, propData);
        if (component === 'flowDetail') {
          markNestedFlowDetail(instance);
        }
        return instance;
      };
      flowManager.__flowDetailKeepHeaderPatched = true;
    },
    markNestedFlowDetailDialog(instance) {
      const mark = () => {
        if (!instance?.$el) {
          return;
        }
        instance.$el.querySelectorAll('.el-dialog__wrapper').forEach((wrapper) => {
          wrapper.classList.add('flow-detail-keep-header');
        });
      };

      this.$nextTick(mark);
      [0, 80, 240, 600].forEach((delay) => {
        setTimeout(mark, delay);
      });
    },
    openFlowDialog() {
      const flowInstanceId = this.$route.params.id || this.$route.query.id || this.getQueryParam('id');
      const mode = this.$route.query.mode || this.getQueryParam('mode') || 'view';
      const taskContext = this.getTaskContext(flowInstanceId);
      const isExpenseCommonFlowEdit = this.isExpenseCommonFlowEditMode(mode, taskContext);

      if (!flowInstanceId) {
        this.error = '缺少流程实例ID';
        this.loading = false;
        return;
      }

      let isExamine = false;
      let operaType = '';
      let taskStatus = '';

      // 直接操作类 mode：不打开弹窗，直接调用 API
      const directModes = ['rollback', 'tracking', 'retrieve'];
      if (directModes.includes(mode) || mode === 'transfer') {
        this.loading = false;
        this.handleDirectAction(flowInstanceId, mode);
        return;
      }

      switch (mode) {
        case 'audit':
          isExamine = true;
          operaType = this.shouldUseStrictEditForAudit(taskContext) ? 'edit' : 'examine';
          taskStatus = 'pending';
          break;
        case 'edit':
          operaType = 'reEdit';
          taskStatus = 'edit';
          break;
        case 'view':
        default:
          isExamine = false;
          operaType = 'check';
          break;
        case 'flow':
          // 查看流程模式 - 直接打开流程图
          operaType = 'flow';
          taskStatus = 'flow';
          break;
      }

      const data = {
        id: flowInstanceId,
        flowInstanceId: flowInstanceId,
        isExamine,
        isReInitiate: mode === 'edit',
        operaType,
        actionType: isExamine ? 'examine' : mode === 'edit' ? 'edit' : 'preview',
        taskStatus,
        tracking: this.normalizeTrackingValue(taskContext?.tracking, taskContext?.trackingFlag),
        flowInstanceBizRelevanceList: Array.isArray(taskContext?.flowInstanceBizRelevanceList)
          ? taskContext.flowInstanceBizRelevanceList
          : [],
      };

      // iframe 入口需要把待发行透传进去，这样内部编辑链路才和源平台待发页保持一致
      if (taskContext && !isExpenseCommonFlowEdit) {
        data.row = taskContext;
      }

      const dialog = this.$flowDetail({
        flowInstanceId,
        data,
        callback: (payload = {}) => {
          if (window.parent !== window) {
            window.parent.postMessage({
              type: 'RSH_FLOW_EVENT',
              eventName: 'flow-action-done',
              data: {
                flowInstanceId,
                mode,
                refresh: payload.refresh === true || payload.success === true,
                success: payload.success === true,
              }
            }, '*');
          }
        }
      });

      if (mode === 'edit') {
        this.applyEditEntryContext(dialog, taskContext);
      }
      if (isExpenseCommonFlowEdit) {
        this.applyExpenseCommonFlowEditContext(dialog);
      }
      if (mode === 'audit') {
        this.applyIframeAuditContext(dialog);
      }
      this.applyTrackingContext(dialog, taskContext);

      // 等弹窗 DOM 渲染后再关闭 loading
      setTimeout(() => {
        this.loading = false;
      }, 800);
    }
  }
};
</script>

<style>
/* 全局样式：在 iframe 嵌入模式下隐藏 el-dialog 的关闭按钮，由宿主应用控制关闭 */
.flow-detail-hide-header .el-dialog__headerbtn {
  display: none !important;
}
/* 隐藏 el-dialog 的遮罩层背景，因为宿主已经有遮罩 */
.v-modal {
  background: transparent !important;
}
/* el-dialog 头部也隐藏，让内容更沉浸 */
.flow-detail-hide-header .el-dialog__header {
  display: none !important;
}

.flow-detail-countersign-dialog .el-dialog__header {
  display: flex !important;
  align-items: center;
}

.flow-detail-countersign-dialog .el-dialog__headerbtn {
  display: block !important;
}

.flow-detail-back-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  border: none;
  background: transparent;
  color: #409eff;
  cursor: pointer;
  font-size: 14px;
}
</style>

<style scoped>
.flow-detail-page {
  height: 100vh;
  width: 100vw;
  position: relative;
  background: #f5f7fa;
}

.loading-overlay {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
}

.loading-spinner {
  position: relative;
  width: 60px;
  height: 60px;
}

.spinner-ring {
  position: absolute;
  inset: 0;
  border: 3px solid transparent;
  border-radius: 50%;
}

.spinner-ring:nth-child(1) {
  border-top-color: #409eff;
  animation: spin 1s linear infinite;
}

.spinner-ring:nth-child(2) {
  inset: 6px;
  border-right-color: #67c23a;
  animation: spin 1.5s linear infinite reverse;
}

.spinner-ring:nth-child(3) {
  inset: 12px;
  border-bottom-color: #e6a23c;
  animation: spin 2s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.loading-text {
  color: #909399;
  font-size: 14px;
  margin: 0;
}

.error-text {
  color: #f56c6c;
  font-size: 14px;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}
</style>
