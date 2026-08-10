<!--
 * @Author: junshao
 * @Date: 2023-03-17 09:52:30
 * @LastEditors: junshao
 * @LastEditTime: 2023-03-17 11:41:38
 * @Description: file content
-->
<template>
  <div>
    <el-dialog
      :visible="flowRoleSelectVisible"
      title="添加角色"
      :close-on-click-modal="false"
      :destroy-on-close="false"
      width="520px"
      top="100px"
      append-to-body
      center
      @close="handleClose"
    >
      <div class="scroll-bar">
        <el-radio-group v-model="radioRole">
          <p v-for="(item, index) in roleList" :key="index" @change="handleCheckedRole(item)">
            <el-radio :label="item">{{ item.name }}</el-radio>
          </p>
        </el-radio-group>
      </div>
      <!-- <el-checkbox :indeterminate="isIndeterminate" v-model="checkAll" @change="handleCheckAllChange">全选</el-checkbox>
        <div style="margin: 15px 0;"></div>
        <el-checkbox-group v-model="checkedRole" @change="handleCheckedRoleChange">
          <el-checkbox v-for="role in roleList" :label="role.id" :key="role.id">{{ role.name }}</el-checkbox>
        </el-checkbox-group> -->
      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="submit">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    flowRoleSelectVisible: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      roleList: [],
      radioRole: {}
      // checkAll: false,
      // isIndeterminate: true,
      // checkedRole: []
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    this.getRoleList();
  },
  methods: {
    // 多选
    /* handleCheckAllChange(val) {
      this.checkedRole = val ? this.roleList : [];
      this.isIndeterminate = false;
    },
    handleCheckedRoleChange(value) {
      const checkedCount = value.length;
      this.checkAll = checkedCount === this.checkedRole.length;
      this.isIndeterminate = checkedCount >= 0 && checkedCount < this.roleList.length;
    }, */
    handleCheckedRole() {},
    handleClose() {
      this.$emit('update:flowRoleSelectVisible', false);
    },
    submit() {
      this.$emit('handleSelectRole', this.radioRole);
      this.handleClose();
    },
    // 获取角色列表
    getRoleList() {
      this.$axios.post(
        Api.roleManage.getRoleList,
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            scope: 'invest'
          }
        },
        res => {
          if (res.isSuccess) {
            this.roleList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }
  }
};

</script>
<style lang='scss' scoped>
.scroll-bar {
  min-height: 300px;
  max-height: 600px;
  overflow-y: auto;
}
.el-radio {
  margin-bottom: 8px;
}
</style>
