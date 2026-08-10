<template>
  <el-dialog v-if="pralleNodeVisible" width="400px" title="并行节点自选" class="nodePerson" :visible="pralleNodeVisible"
    :before-close="handleCloseParallelChoose" :close-on-click-modal="false" append-to-body>
    <div v-for="chooseNode in pralleNodeList" :key="chooseNode.id">
      <span>{{ chooseNode.nodeName }} : </span>
      <span v-for="(audit, index) in parlleNodePerson[chooseNode.nodeId]" :key="index">
        {{ audit.name }}
      </span>
      <el-button type="text" size="mini" @click="chooseParallelNode(chooseNode.nodeId)">{{
          parlleNodePerson[chooseNode.nodeId] && parlleNodePerson[chooseNode.nodeId].length ? "重新选择" : "选择人员"
      }}</el-button>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button type="primary" @click="handleSaveParallelChooseNode">提 交</el-button>
    </span>
  </el-dialog>
</template>
<script>
export default {
  name: 'BranchPralleChoose',
  props: ['pralleNodeVisible', 'pralleNodeList', 'parlleNodePerson'],
  data() {
    return {
      chooseBranchNode: '',
    }
  },
  methods: {
    handleCloseParallelChoose() {
      this.$emit("update:pralleNodeVisible", false)
    },
    chooseParallelNode(nodeId) {
      this.$emit('parlleChoosePerson', nodeId)
    },
    handleSaveParallelChooseNode() {
      this.$emit('parlleSubmit')
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

