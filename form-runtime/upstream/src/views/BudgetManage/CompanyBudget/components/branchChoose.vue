<template>
  <el-dialog v-if="branchChooseVisible" width="500px" title="选择流程分支" class="nodePerson" :visible="branchChooseVisible"
    :before-close="handleCloseBranchChoose" :close-on-click-modal="false" append-to-body>
    <el-radio-group v-model="chooseBranchNode" @change="handleChangeChooseBranch">
      <el-radio v-for="(branch, index) in manualChooseNodes" :key="index" :label="branch" class="radio-choose-item">
        <span>{{ branch.branchName }}-{{ branch.nodeName }} : </span>
        <span v-if="branch.auditType == 'run_node_choose'">
          <span v-for="(audit, idx) in chooseBranchNodeList" :key="idx"
            v-if="audit.nextNodeTemplateId == branch.nextNodeTemplateId">
            {{ audit.name }}
          </span>
          <el-button type="primary" :disabled="
            branch.nextNodeTemplateId != chooseBranchNode.nextNodeTemplateId
          " @click="handleChooseBranchNode(branch.nextNodeTemplateId)">选择审批人</el-button>
        </span>
        <span v-if="branch.auditType == 'initiator'">由发起人审批</span>
        <span v-if="branch.auditType == 'assign'">由流程配置人员审批</span>
        <span v-if="branch.nodeType == 'empty'">空节点</span>
      </el-radio>
    </el-radio-group>
    <span slot="footer" class="dialog-footer">
      <el-button type="primary" @click="handleSaveBranchChooseNode">提 交</el-button>
    </span>
  </el-dialog>
</template>
<script>
export default {
  name: 'BranchChoose',
  props: ['branchChooseVisible', 'manualChooseNodes', 'chooseBranchNodeList'],
  data() {
    return {
      chooseBranchNode: '',

    }
  },
  methods: {
    handleCloseBranchChoose() {
      this.chooseBranchNode = {};
      this.$emit('clearCheckboxPersonGroup')
      this.$emit("update:branchChooseVisible", false)
    },
    handleChooseBranchNode(nextNodeTemplateId) {
      this.$emit('showSelectPerson', nextNodeTemplateId)
    },
    handleSaveBranchChooseNode() {
      this.$emit('saveBranchChooseNode', this.chooseBranchNode)
    },
    handleChangeChooseBranch(branch) {
      // if (branch.auditType != 'run_node_choose') {
      this.$emit('clearCheckboxPersonGroup')
      // this.chooseBranchNodeList = [];
      // }
    },
  }
}
</script>
<style lang="scss">
.radio-choose-item {
  display: inline-block;
  width: 460px;
  margin: 5px 0;
  white-space: pre-wrap;
  line-height: 20px;
}
</style>

