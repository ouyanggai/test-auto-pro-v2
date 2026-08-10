<!--
 * @Descripttion: 一级归口树结构
 * @Author: zhengzetao
 * @Date: 2022-06-07
-->
<template>
  <div class="main-left-tree">
    <el-tree class="framework-tree" :props="defaultProps" node-key="id" :data="treeData" @node-expand="nodeExpand"
      @node-click="handleNodeClick" :default-expanded-keys="defaultFirstLevelId" ref="budgetTypeTree">
      <span class="custom-tree-node" slot-scope="{ node, data }">
        <!-- <span class="span-tree-icons">
          <el-button v-if="node.level == 2" type="text" @click.stop="openDetailTml">导入明细模板</el-button>
        </span> -->
        <span>{{ node.label }}</span>
        <span class="span-tree-icons" v-if="data.budgetTypeTree">
          <i class="el-icon-circle-plus-outline icon-margin" @click="addBudget(data)" title="新增"></i>
        </span>
      </span>
    </el-tree>
    <!-- <el-tree class="framework-tree" :data="treeData" :props="defaultProps" @node-click="handleNodeClick"
      :highlight-current="true" :expand-on-click-node="false" :indent="20" node-key="id" empty-text="暂无数据"
      :default-expanded-keys="defaultExpandedNode" :default-expand-all="false" ref="bigTree">
      <span class="custom-tree-node" slot-scope="{ node, data }">
        <span v-html="highNode(node.label)"></span>
        <span class="span-tree-icons">
          <el-button v-if="node.level == 2" type="text" @click.stop="openDetailTml">导入明细模板</el-button>
        </span>
      </span>
    </el-tree> -->

  </div>
</template>

<script>
import Api from '@/api';

export default {
  name: '',
  components: {
  },
  props: {
    treeData: {
      type: Array,
      default: () => {
        return [];
      }
    },
    defaultFirstLevelId: {
      type: Array,
      default: () => {
        return [];
      }
    },
    searchInput: {
      type: [String, Number],
      default: ''
    }
  },
  data() {
    return {
      testTree: [{
        name: '一级 1',
        type: 1,
        childrenList: [{
          name: '二级 1-1',
          type: 1,
          childrenList: [{
            name: '三级 1-1-1',
            type: 2,
            childrenList: []
          }]
        }]
      }, {
        name: '一级 2',
        type: 1,
        childrenList: [{
          name: '二级 2-1',
          type: 1,
          childrenList: [{
            name: '三级 2-1-1',
            type: 2,
            childrenList: []
          }]
        }, {
          name: '二级 2-2',
          type: 1,
          childrenList: [{
            name: '三级 2-2-1',
            type: 2,
            childrenList: []
          }]
        }]
      }],

      defaultExpandedNode: [],
      treeDataRow: null,
      treeDataType: null,
      // 树结构
      defaultProps: {
        // 放开
        children: 'childrenList',
        // children: 'children',
        label(data) {
          return data.name;
        }
      },
      fun: null,
      newInputContent: ''
    };
  },
  computed: {},
  watch: {
    treeData: {
      handler: function (val) {
        // console.log(val);
        this.changeName(val);
        setTimeout(x => {
          // console.log(111, this.$refs.budgetTypeTree.store.root.childNodes);
          this.setLeaf(this.$refs.budgetTypeTree.store.root.childNodes);
        }, 1000);
      }
    }
    // searchInput: function (val) {
    //   this.newInputContent = val;
    //   this.debounce(this.inputSearchContent, 1000);
    // }
  },
  created() {
  },
  mounted() {

  },
  methods: {
    addBudget(data) {
      this.$emit('openAddBudgetDialog', {
        title: '新建',
        isTypeChild: false,
        rowData: data
      });
    },
    // 最后面的一个层级设置可展开图标
    setLeaf(data) {
      // console.log('data', data);
      data.forEach(x => {
        if (x.childNodes.length) {
          this.setLeaf(x.childNodes);
        } else {
          if (x.data.type == 2) {
            this.$set(x, 'isLeaf', false);
          }
        }
      });
    },
    // 公司领导改成公司固定费用
    changeName(data) {
      data.forEach(x => {
        if ((x.flag != null && x.flag == 'false')) {
          x.name = '公司固定费用';
        }
        if (x.childrenList.length) {
          this.changeName(x.childrenList);
        }
      });
    },
    nodeExpand(data, node) {
      // data为当前节点的数据
      // node为当前节点
      // console.log(1213, node);
      // 把获取到的数据赋给当前节点数据的children
      if (data.type == 2) {
        this.getBudgetList(node);
      }
    },

    getBudgetList(node) {
      this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: {
            // departmentId: 'bd8b6508fa8f477f8b29cbb6a8935c77',
            departmentId: node.data.id,
            status: '1',
            annually: this.$parent.selectYear,
            parentId: 1,
            type: node.data.projectId ? '3' : '1' // 传1代表公司  传3代表项目
          }
        },
        res => {
          if (res.isSuccess) {
            res.data.dataList.forEach(x => {
              x.childrenList = [];
              x.budgetTypeTree = true;
            });
            const resData = res.data.dataList.length ? res.data.dataList : [];
            node.data.childrenList = resData;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    openDetailTml() {
      this.$emit('openDetailTml');
    },
    debounce: function (fn, wait) {
      // 防抖函数
      if (this.fun !== null) {
        clearTimeout(this.fun);
      }
      this.fun = setTimeout(fn, wait);
    },
    packUpAll() {
      for (var i = 0; i < this.$refs.bigTree.store._getAllNodes().length; i++) {
        this.$refs.bigTree.store._getAllNodes()[i].expanded = false;
      }
    },
    highNode(label) {
      const title = label;
      const rep = new RegExp(this.searchInput, 'g');
      const resString = `<span style="color:#145afe">${this.searchInput}</span>`;
      return title.replace(rep, resString);
    },
    inputSearchContent() {
      const val = this.newInputContent;
      if (val != '') {
        this.defaultExpandedNode = [];
        this.packUpAll();
        this.getTreeNode(this.treeData, val);
      }
    },
    getTreeNode(list, val) {
      list.forEach(y => {
        if (y.name.indexOf(val) != -1) {
          this.defaultExpandedNode.push(y.id);
          // this.defaultExpandedNode.push(y.name);
        }
        if (y.childrenList.length) {
          this.getTreeNode(y.childrenList, val);
        }
      });
    },
    handleNodeClick(data, data2, data3) {
      this.treeDataRow = data;
      this.$emit('clickBigItemTree', data);
    },
    // 重置表单
    resetForm(formName) {
      if (this.$refs[formName] !== undefined) {
        this.$refs[formName].resetFields();
      }
    }
  }
};
</script>
<style lang="scss" scoped>
.main-left-tree {
  height: 100%;
  // height: calc(100% - 40px);
  width: 50%;
  // width: 300px;
  background: #fff;
  float: left;
  overflow: auto;

  .span-tree-icons {
    margin-left: 30px;
  }

  .framework-tree {
    & ::v-deep .el-tree-node.is-current>.el-tree-node__content {
      background-color: #f0f7ff;
      color: #1890ff;

      &::after {
        position: absolute;
        content: "";
        width: 3px;
        height: 40px;
        background: #1890ff;
        top: 0px;
        right: 0px;
      }

    }

    ::v-deep .el-tree-node__content {
      height: 100%;
      position: relative;
    }

    .custom-tree-node {
      position: relative;
      width: 100%;
      height: 40px;
      line-height: 40px;

      &:hover {
        &::after {
          position: absolute;
          content: "";
          width: 3px;
          height: 40px;
          background: #1890ff;
          top: 0px;
          right: 0px;
        }
      }

      .span-tree-icons {
        position: absolute;
        right: 20px;
        top: 0px;
      }
    }
  }

}
</style>
