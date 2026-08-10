<!--
 * @Descripttion: 选择下一个审批节点(暂时无用)
 * @Author: zhengzetao
 * @Date: 2022-08-29
-->

<template>
  <el-dialog width="500px" :title="`选择-${nextNodeName}-审批节点`" class="nodePerson" :visible="visible"
    :before-close="handleCancelCheckNode" append-to-body>
    <div class="checkbox-con scroll-bar">
      <el-checkbox-group v-model="checkboxPersonGroup">
        <p v-for="(item, index) in personList" :key="index">
          <el-checkbox :label="item.id">{{ item.name }}</el-checkbox>
        </p>
      </el-checkbox-group>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="handleCancelCheckNode">取 消</el-button>
      <el-button type="primary" @click="handleGetNode">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';

export default {
  name: '',
  components: {},
  data() {
    return {
      checkboxPersonGroup: [],
      nextAuditorList: [],
      personList: [] // 流程人员列表
    };
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    nextNodeName: {
      type: String,
      default: ''
    },
    nextNodeProxyId: {
      type: String,
      default: ''
    }
    // flowInstanceId: {
    //   type: String,
    //   default: ''
    // },
    // isExamine: {
    //   type: Boolean,
    //   default: true
    // },
    // jobTaskId: {
    //   type: String,
    //   default: ''
    // },
    // flowNodeType: {
    //   type: String,
    //   default: ''
    // },

  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    this.getNodeList();
  },
  methods: {
    // 获取节点人员列表
    getNodeList() {
      const nextNodeProxyId = this.nextNodeProxyId;
      this.$axios.post(
        Api.approveManage.getFlowNodeProxyConfigUserList,
        {
          data: {
            id: nextNodeProxyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.personList = res.data.flowNodeAuditConfig.userVoList;
          }
        }
      );
    },
    // 选择节点人员
    handleCheckedPreson(val) {
    },
    // 取消选择节点人员
    handleCancelCheckNode() {
      this.$emit('update:visible', false);
      // this.nodeChooseVisible = false;
      this.checkboxPersonGroup = [];
    },
    // 获取审批节点人员
    handleGetNode() {
      if (!this.checkboxPersonGroup.length) {
        this.$message.error('请选择下一节点审批人员');
        return;
      }
      this.$emit('update:visible', false);
      this.$emit('getSelectPerson', {
        checkboxPersonGroup: this.checkboxPersonGroup
      });
      // this.nodeChooseVisible = false;
      // this.handleBeforeSubmit('pass');
    }
  }
};

</script>
<style lang='scss' scoped>

</style>
