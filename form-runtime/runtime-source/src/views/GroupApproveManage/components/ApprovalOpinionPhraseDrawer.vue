<template>
  <span ref="root" class="approval-phrase" @click.stop>
    <el-button
      type="text"
      class="approval-phrase__trigger"
      :class="{ 'is-active': drawerVisible }"
      @click="toggleDrawer"
    >常用语</el-button>

    <transition name="approval-phrase-drawer">
      <div
        v-show="drawerVisible"
        class="approval-phrase__drawer"
      >
        <div class="approval-phrase__header">
          <div class="approval-phrase__title">常用语</div>
          <div class="approval-phrase__tip">双击填充到审批意见</div>
        </div>
        <div
          v-loading="listLoading"
          class="approval-phrase__body"
        >
          <div
            v-if="opinionList.length"
            class="approval-phrase__list"
          >
            <div
              v-for="(item, index) in opinionList"
              :key="item.id || index"
              class="approval-phrase__item"
              @dblclick="appendOpinion(item)"
            >
              <span class="approval-phrase__index">{{ index + 1 }}.</span>
              <span class="approval-phrase__content">{{ item.content }}</span>
              <el-button
                type="text"
                icon="el-icon-delete"
                class="approval-phrase__delete"
                :loading="deleteLoadingId === item.id"
                @click.stop="deleteOpinion(item)"
              ></el-button>
            </div>
          </div>
          <div
            v-else
            class="approval-phrase__empty"
          >
            <i class="el-icon-document approval-phrase__empty-icon"></i>
            <span>{{ listLoading ? '加载中...' : '暂无常用语' }}</span>
          </div>
        </div>
        <div class="approval-phrase__footer">
          <el-button
            type="text"
            icon="el-icon-edit"
            class="approval-phrase__new"
            @click="openAddDialog"
          >新建</el-button>
          <el-button
            size="mini"
            @click="closeDrawer"
          >关闭</el-button>
        </div>
      </div>
    </transition>

    <el-dialog
      title="新增常用语"
      :visible.sync="addDialogVisible"
      width="420px"
      :custom-class="dialogClassName"
      append-to-body
      @opened="focusAddInput"
      :close-on-click-modal="false"
      @closed="resetAddForm"
    >
      <el-form
        ref="addForm"
        :model="addForm"
        :rules="addRules"
      >
        <el-form-item prop="content">
          <el-input
            ref="addInput"
            v-model.trim="addForm.content"
            type="textarea"
            :rows="4"
            maxlength="100"
            show-word-limit
            placeholder="请输入常用语，100字以内"
          ></el-input>
        </el-form-item>
      </el-form>
      <span slot="footer">
        <el-button @click="addDialogVisible = false">取 消</el-button>
        <el-button
          type="primary"
          :loading="saveLoading"
          @click="submitAddOpinion"
        >提 交</el-button>
      </span>
    </el-dialog>
  </span>
</template>

<script>
import Api from '@/api';

export default {
  name: 'ApprovalOpinionPhraseDrawer',
  props: {
    value: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      dialogClassName: `approval-phrase-dialog-${this._uid}`,
      drawerVisible: false,
      loaded: false,
      listLoading: false,
      opinionList: [],
      addDialogVisible: false,
      saveLoading: false,
      deleteLoadingId: '',
      addForm: {
        content: ''
      },
      addRules: {
        content: [
          { required: true, message: '请输入常用语', trigger: 'blur' },
          { max: 100, message: '常用语不能超过100字', trigger: 'blur' }
        ]
      }
    };
  },
  beforeDestroy() {
    this.removeDocumentClose();
  },
  methods: {
    toggleDrawer() {
      if (this.drawerVisible) {
        this.closeDrawer();
        return;
      }
      this.openDrawer();
    },
    openDrawer() {
      this.drawerVisible = true;
      this.bindDocumentClose();
      if (!this.loaded) {
        this.fetchOpinionList();
      }
    },
    closeDrawer() {
      this.drawerVisible = false;
      this.removeDocumentClose();
    },
    bindDocumentClose() {
      this.removeDocumentClose();
      this.$nextTick(() => {
        document.addEventListener('click', this.handleDocumentClick);
      });
    },
    handleDocumentClick(event) {
      if (this.addDialogVisible) {
        return;
      }
      const target = event && event.target;
      if (!target) {
        this.closeDrawer();
        return;
      }
      if (this.isTargetInside(target, this.$refs.root)) {
        return;
      }
      if (this.hasClassInParents(target, this.dialogClassName)) {
        return;
      }
      if (this.hasClassInParents(target, 'el-dialog__wrapper')) {
        return;
      }
      if (this.hasClassInParents(target, 'el-message-box__wrapper')) {
        return;
      }
      this.closeDrawer();
    },
    isTargetInside(target, root) {
      return !!(root && target && root.contains(target));
    },
    hasClassInParents(target, className) {
      let current = target;
      while (current && current !== document.body) {
        if (current.classList && current.classList.contains(className)) {
          return true;
        }
        current = current.parentNode;
      }
      return false;
    },
    removeDocumentClose() {
      document.removeEventListener('click', this.handleDocumentClick);
    },
    async fetchOpinionList() {
      this.listLoading = true;
      try {
        const res = await this.$axios.post(
          Api.schedule.approvalOpinionList,
          {
            pagination: true,
            pages: 1,
            size: 100,
            data: {}
          },
          '',
          false,
          { noErrMsg: false, noLoading: true }
        );
        if (res.isSuccess) {
          this.opinionList = Array.isArray(res.data)
            ? res.data.filter(item => item && item.content)
            : [];
          this.loaded = true;
        }
      } finally {
        this.listLoading = false;
      }
    },
    appendOpinion(item) {
      const content = (item.content || '').trim();
      if (!content) {
        return;
      }
      const current = this.value || '';
      const next = current ? `${current}\n${content}` : content;
      this.$emit('input', next);
    },
    openAddDialog() {
      this.addDialogVisible = true;
    },
    focusAddInput() {
      this.$nextTick(() => {
        if (this.$refs.addInput) {
          this.$refs.addInput.focus();
        }
      });
    },
    resetAddForm() {
      this.addForm.content = '';
      if (this.$refs.addForm) {
        this.$refs.addForm.clearValidate();
      }
    },
    submitAddOpinion() {
      this.$refs.addForm.validate((valid) => {
        if (!valid) {
          return;
        }
        this.saveOpinion();
      });
    },
    async saveOpinion() {
      const content = (this.addForm.content || '').trim();
      if (!content) {
        this.$message.warning('请输入常用语');
        return;
      }
      if (content.length > 100) {
        this.$message.warning('常用语不能超过100字');
        return;
      }
      this.saveLoading = true;
      try {
        const res = await this.$axios.post(
          Api.schedule.approvalOpinionSave,
          {
            data: {
              content
            }
          },
          '',
          false,
          { noErrMsg: false, noLoading: true }
        );
        if (res.isSuccess) {
          this.$message.success('新增成功');
          this.addDialogVisible = false;
          this.drawerVisible = true;
          this.bindDocumentClose();
          this.loaded = false;
          await this.fetchOpinionList();
        }
      } finally {
        this.saveLoading = false;
      }
    },
    deleteOpinion(item) {
      if (!item || !item.id) {
        return;
      }
      return this.$confirm('确定删除该常用语？', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(async () => {
        this.deleteLoadingId = item.id;
        try {
          const res = await this.$axios.post(
            Api.schedule.approvalOpinionDelete,
            {
              data: {
                id: item.id
              }
            },
            '',
            false,
            { noErrMsg: false, noLoading: true }
          );
          if (res.isSuccess) {
            this.$message.success('删除成功');
            this.opinionList = this.opinionList.filter(opinion => opinion.id !== item.id);
          }
        } finally {
          this.deleteLoadingId = '';
        }
      }).catch(() => {});
    }
  }
};
</script>

<style lang="scss" scoped>
.approval-phrase {
  position: relative;
  display: inline-flex;
  align-items: center;
  line-height: 1;
}

.approval-phrase__trigger {
  padding: 0;
  line-height: 20px;
  transition: color 0.18s ease;
}

.approval-phrase__trigger:hover,
.approval-phrase__trigger.is-active {
  color: #409eff;
}

.approval-phrase__drawer {
  position: absolute;
  bottom: 30px;
  right: 0;
  z-index: 2100;
  width: 300px;
  max-height: 280px;
  overflow: hidden;
  text-align: left;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  box-shadow: 0 14px 32px rgba(31, 45, 61, 0.16);
  transform-origin: right bottom;
}

.approval-phrase__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 18px 12px;
}

.approval-phrase__title {
  font-size: 16px;
  font-weight: 600;
  line-height: 20px;
  color: #303133;
}

.approval-phrase__tip {
  font-size: 12px;
  line-height: 18px;
  color: #909399;
  white-space: nowrap;
}

.approval-phrase__body {
  min-height: 88px;
  max-height: 180px;
  padding: 4px 10px 0;
  overflow-y: auto;
}

.approval-phrase__item {
  position: relative;
  display: flex;
  align-items: flex-start;
  min-height: 36px;
  margin-bottom: 8px;
  padding: 10px 42px 10px 14px;
  font-size: 14px;
  line-height: 20px;
  color: #303133;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.approval-phrase__item:hover {
  color: #409eff;
  background: #f5faff;
  border-color: #c6e2ff;
  box-shadow: 0 8px 18px rgba(64, 158, 255, 0.12);
}

.approval-phrase__item:active {
  background: #ecf5ff;
  transform: translateY(1px);
}

.approval-phrase__index {
  flex: none;
  min-width: 26px;
}

.approval-phrase__content {
  flex: 1;
  word-break: break-all;
  white-space: pre-wrap;
}

.approval-phrase__delete {
  position: absolute;
  top: 50%;
  right: 12px;
  padding: 0;
  color: #f56c6c;
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.18s ease, color 0.18s ease;
}

.approval-phrase__item:hover .approval-phrase__delete,
.approval-phrase__item:focus-within .approval-phrase__delete {
  opacity: 1;
}

.approval-phrase__delete:hover {
  color: #f78989;
}

.approval-phrase__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 112px;
  padding: 24px 18px;
  gap: 8px;
  font-size: 13px;
  color: #909399;
  text-align: center;
}

.approval-phrase__empty-icon {
  font-size: 26px;
  color: #c0c4cc;
}

.approval-phrase__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px 14px 18px;
  background: #fafafa;
  border-top: 1px solid #f0f2f5;
}

.approval-phrase__new {
  padding: 0;
}

.approval-phrase-drawer-enter-active,
.approval-phrase-drawer-leave-active {
  transition: opacity 0.22s cubic-bezier(0.22, 0.61, 0.36, 1), transform 0.22s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.approval-phrase-drawer-enter,
.approval-phrase-drawer-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}
</style>
