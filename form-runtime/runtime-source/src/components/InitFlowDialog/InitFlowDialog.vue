<template>
  <div class="initFlowDialog-container">
    <!-- <el-dialog
    :title="title"
    :visible.sync="visible"
    :before-close="handleBeforeClose"
    @open="handleOpen"
    @close="handleClose"
    >
      <slot v-bind="dialogData"></slot>
      <div v-if="loading" class="loading-wrapper">
        <el-button type="text">加载中...</el-button>
      </div>
    </el-dialog> -->
    <chooseFlowDialog :visible.sync="chooseFlowDialogVisible" @confirm="confirm" :flowList="flowList"></chooseFlowDialog>
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="[]" v-if="approveDialogVisible" @success="handleSuccess"
      :flowJson.sync="flowJson" :flowType.sync="flowType" :closeAll="true" />
  </div>
</template>

<script>
import Api from '@/api';
const chooseFlowDialog = () => import('@/views/PerformanceManage/TargetBook/components/chooseFlowDialog.vue');
const FlowDialog = () => import('@/views/GroupApproveManage/Submitted/components/FlowDialog.vue');
// import chooseFlowDialog from '@/views/PerformanceManage/TargetBook/components/chooseFlowDialog.vue';
// import FlowDialog from '@/views/GroupApproveManage/Submitted/components/FlowDialog.vue';
export default {
  name: 'InitFlowDialog',
  components: { chooseFlowDialog, FlowDialog },
  props: {
    title: { type: String, default: '-' },
    params: { type: Object, default: () => ({}) },
    options: { type: Object, default: () => ({}) },
    visible: { type: Boolean, default: false },
    loading: { type: Boolean, default: false },
    callbacks: {
      type: Object,
      default: () => ({
        onSuccess: null,
        onOpen: null,
        onOpened: null,
        onClose: null,
        onClosed: null
      })
    }
  },
  provide() {
    return {
      initFlowDialogOptions: this.options
    };
  },
  data() {
    return {
      chooseConfirmed: false,
      successConfirmed: false,
      flowType: '',
      flowList: [],
      flowJson: {},
      approveDialogVisible: false,
      chooseFlowDialogVisible: false,
      dialogData: null
    };
  },
  mounted() {
    console.log(this.params, 'this.datas')
    var auditWay = this.params.auditWay;
    if (auditWay) {
      this.flowType = Array.isArray(auditWay) ? auditWay[0] : auditWay;
      this.getFlowByTemplate(auditWay);
    }
  },
  watch: {
    chooseFlowDialogVisible(val) {
      if (!val && !this.chooseConfirmed) {
        this.handleClose();
      }
    },
    approveDialogVisible(val) {
      if (!val && !this.successConfirmed) {
        this.handleClose();
      }
    }
  },
  methods: {
    handleSuccess() {
      this.successConfirmed = true;
      this.callbacks.onSuccess?.();
      this.handleClose();
    },
    getFlowByTemplate(auditWay) {
      const param = {
        data: {
          useScope: 'invest',
          // auditWay,
          auditWayList: Array.isArray(auditWay) ? auditWay : [auditWay]
        },
        showMe: true,
        ignoreFormTemplateBizRelevanceData: true,
        ignoreTemplateData: true,
        platformCode: '999999',
        pagination: true,
        pages: 1,
        size: 99
      };
      this.$axios.post(Api.schedule.getFlowTemplateList, param, (res) => {
        if (res.isSuccess) {
          if (!res.data || res.data.length == 0) {
            this.$message.error('暂无流程权限，请联系管理员');
            return;
          }
          if (res.data?.length > 1) {
            this.flowList = res.data;
            this.chooseFlowDialogVisible = true;
          } else {
            this.getFlowFindById(res.data[0].id);
          }
        } else {
          this.handleClose();
        }
      });
    },
    getFlowFindById(id) {
      this.$axios.post(Api.schedule.flowTemplateFindById, { data: { id }}, (res) => {
        if (res.isSuccess) {
          this.flowJson = res.data;
          this.approveDialogVisible = true;
        } else {
          this.handleClose();
        }
      });
    },
    confirm(val) {
      this.chooseConfirmed = true;
      if (val) {
        const find = this.flowList.find(item => item.id == val);
        if (find) {
          this.chooseFlowDialogVisible = false
          // this.toFlowPage(find);
          this.getFlowFindById(find.id);
        }
      } else {
        this.handleClose();
        this.$message.error('请选择流程');
      }
    },
    handleOpen() {
      this.dialogData = null;
      this.callbacks.onOpen?.();
      if (this.callbacks.onDataFetch) {
        this.loading = true;
        this.callbacks.onDataFetch((data) => {
          this.dialogData = data;
          this.loading = false;
          this.$nextTick(this.callbacks.onOpened);
        });
      }
    },
    handleBeforeClose(done) {
      if (this.callbacks.onBeforeClose) {
        const result = this.callbacks.onBeforeClose();
        if (result === false) return;
      }
      done();
    },
    handleClose() {
      this.callbacks.onClose?.();
      this.$destroy();
      this.$el.remove();
      // document.body.removeChild(this.$el)
      this.callbacks.onClosed?.();
    }
  }
};
</script>

<style scoped>
.loading-wrapper {
  text-align: center;
  padding: 20px;
}
</style>
