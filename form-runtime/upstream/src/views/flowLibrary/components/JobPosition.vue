<!--
 * @description:  选择岗位
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    :visible="visible"
    title="选择岗位"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    width="50%"
    top="100px"
    append-to-body
    center
    @close="handleClose"
  >
    <el-tree
      ref="personSelectTree"
      node-key="id"
      :data="treeData"
      :props="defaultProps"
      :default-expand-all="false"
      :default-expanded-keys="defaultFirstLevelId"
      show-checkbox
      :indent="10"
      auto-expand-parent
    >
      <span slot-scope="{node,data}">
        <span>{{ data.name }}</span>
        <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
      </span>
    </el-tree>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="submit">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    examinerId: {
      type: [Number, String],
      default: ''
    }
  },
  data() {
    return {
      treeData: [],
      defaultProps: {
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      defaultFirstLevelId: [],
      checkboxList: [],
      company: ''
    };
  },
  computed: {},
  watch: {},
  created() {
    if (this.$route.query?.relative) {
      this.company = localstorageGet('formDetailCompany');
    }
    this.getJobTitleTree();
  },
  mounted() {
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },

    submit() {
      console.log('submit');
      const checkNode = this.$refs.personSelectTree.getCheckedNodes();
      const personList = checkNode.filter(x => x.type == 4);
      this.$emit('select', personList);
      this.handleClose();
    },
    getJobTitleTree() { // 获取岗位树
      this.$axios.post(
        Api.user.getJobTitleTree,
        {
          data: {
            flag: 4,
            id: this.company || localstorageGet('companyId'), // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData = res.data;
            this.defaultFirstLevelId = [res.data[0].id];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
