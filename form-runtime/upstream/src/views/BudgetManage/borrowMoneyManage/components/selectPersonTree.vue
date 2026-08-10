<template>
    <el-select
      :disabled="disabled"
      class="main-select-tree"
      ref="selectTree"
      v-model="selectValue"
      style="width: 255px;"
      clearable
      @clear="clearSelect">
      <el-option
        v-for="item in treeToList"
        :label="item.label"
        :value="item.value"
        :key="item.value"
        style="display: none;"
        ></el-option>
      <el-tree
        size="medium"
        class="main-select-el-tree"
        ref="selectelTree"
        :data="treedata"
        :props='treeProps'
        highlight-current
        node-key="id"
        @node-click="handleNodeClick"
        :expand-on-click-node="expandOnClickNode"
        default-expand-all/>
    </el-select>
</template>

<script>

export default {
  data() {
    return {
      selectValue: '',
      expandOnClickNode: true,
      options: [],
      treeProps: {
        children: 'childrenList',
        label: 'name'
      },
      treeToList: []
    };
  },
  props: ['treedata', 'value', 'disabled'],
  created() {
    console.log('created')
    this.treeToList = this.optionData(this.treedata);
  },
  watch: {
    treedata: {
      handler(newName, oldName) {
        console.log(this.treedata, 'watch-this.treedata');
        this.treeToList = this.optionData(this.treedata);
      }
    }
  },
  mounted() {
    this.$nextTick(() => {
      this.$refs.selectelTree.setCurrentKey(this.value ?? '');
      this.selectValue = this.value || '';
    });
  },
  methods: {
    testInput() {
      console.log('testInput');
    },
    optionData(array, result = []) {
      console.log('array',array)
      array.forEach(item => {
        result.push({ label: item.name, value: item.id });
        if (item.childrenList && item.childrenList.length !== 0) {
          this.optionData(item.childrenList, result);
        }
      });
      return JSON.parse(JSON.stringify(result));
    },
    handleNodeClick(node) {
      this.selectValue = node.id;
      this.$emit('input', this.selectValue);
      this.$refs.selectTree.blur();
    },
    clearSelect() {
      this.$refs.selectelTree.setCurrentKey(null);
      this.$emit('input', '');
    }
  }
};
</script>

<style>
.main-select-el-tree .el-tree-node .is-current>.el-tree-node__content {
  font-weight: bold;
  color: #409eff;
}

.main-select-el-tree .el-tree-node.is-current>.el-tree-node__content {
  font-weight: bold;
  color: #409eff;
}
</style>
