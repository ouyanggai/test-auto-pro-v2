<template>
  <div class="flow-initiate-page">
    <div v-if="loading" class="loading-text">正在打开流程发起...</div>
  </div>
</template>

<script>
export default {
  name: 'FlowInitiate',
  data() {
    return {
      loading: true
    };
  },
  mounted() {
    this.$nextTick(() => {
      const params = this.$route.query;
      this.$initFlow({
        title: params.title || '发起流程',
        params: {
          flowTemplateId: params.flowTemplateId,
          companyId: params.companyId,
          ...params
        },
        onSuccess: (res) => {
          // 通知父窗口流程发起成功
          if (window.parent !== window) {
            window.parent.postMessage({
              type: 'RSH_FLOW_EVENT',
              eventName: 'flow-submitted',
              data: res
            }, '*');
          }
        },
        onClosed: () => {
          this.loading = false;
          // 通知父窗口弹窗关闭
          if (window.parent !== window) {
            window.parent.postMessage({
              type: 'RSH_FLOW_EVENT',
              eventName: 'flow-dialog-closed'
            }, '*');
          }
        }
      });
    });
  }
};
</script>

<style scoped>
.flow-initiate-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
.loading-text {
  color: #909399;
  font-size: 14px;
}
</style>
