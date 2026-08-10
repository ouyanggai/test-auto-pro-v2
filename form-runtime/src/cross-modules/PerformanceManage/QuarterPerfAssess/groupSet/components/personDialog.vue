<template>
  <el-dialog
    width="900px"
    title="人员选择"
    :visible.sync="innerVisible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    append-to-body
  >
    <TransferPerson :activeName="'personnel'" :companyId="companyId" :defaultCheckedKeys="selectedUserIds" @getSelectPerson="getSelectPerson"/>
    <div slot="footer" class="dialog-footer">
      <el-button @click="handleCancel">取 消</el-button>
      <el-button type="primary" @click="handleConfirm">确 定</el-button>
    </div>
  </el-dialog>
</template>

<script>
import { localstorageGet } from '@/utils/auth';
import TransferPerson from '@/views/BacklogManage/components/NoFormMulBranch/components/Workflow/components/TransferPerson.vue';

export default {
  name: 'PersonDialong',
  components: {
    TransferPerson
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    selectedUsers: {
      type: Array,
      default: () => []
    },
    // 是否当前用户所在公司, true: 是, false: 否, 默认为true
    isCurrentCompany: {
      type: Boolean,
      default: true
    }

  },
  data() {
    return {
      companyId: '',
      innerVisible: this.visible,
      selectedList: [],
      selectedUserIds: []
    };
  },
  watch: {
    visible(newVal) {
      if (this.isCurrentCompany) {
        this.companyId = localstorageGet('companyId');
      } else {
        // 不是当前用户所在公司, 则获取所有公司
        this.companyId = '';
      }
      this.innerVisible = newVal;
      this.selectedUserIds = this.selectedUsers.map(item => item.id);
      console.log(this.selectedUsers, 'this.selectedUsers');
      console.log(this.selectedUserIds, 'this.selectedUserIds');
    }
  },
  methods: {
    handleCancel() {
      this.$emit('update:visible', false);
    },
    handleConfirm() {
      this.$emit('confirm', this.selectedList);
      this.$emit('update:visible', false);
    },
    getSelectPerson(checkData) {
      this.selectedList = checkData;
      console.log(checkData, 'checkData');
    }
  }
};
</script>

<style lang='scss' scoped>
</style>
