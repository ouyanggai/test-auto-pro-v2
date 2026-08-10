<!--
 * @Descripttion: 分级明细树结构
 * @Author: zhengzetao
 * @Date: 2022-06-07
-->
<template>
  <div class="main-left-tree">
    <el-tree
      class="framework-tree"
      :data="treeData"
      :props="defaultProps"
      @node-click="handleNodeClick"
      @node-expand="handleNodeExpand"
      :highlight-current="true"
      :expand-on-click-node="false"
      :indent="20"
      node-key="id"
      empty-text="暂无数据"
      :default-expanded-keys="defaultExpandedNode"
      :default-expand-all="false"
      ref="gradingTree"
    >
      <span
        class="custom-tree-node"
        slot-scope="{ node, data }"
      >
        <el-input
          v-if="data.showEditInput"
          v-model="inputName"
          v-focus
          @blur="blurInput"
          placeholder="请输入名称"
          style="width:50%;"
        >
        </el-input>
        <span
          v-if="!data.showEditInput"
          v-html="highNode(node.label)"
        ></span>
        <span class="span-tree-icons">
          <i
            class="el-icon-circle-plus-outline icon-margin"
            @click="addBudget(data)"
            title="新增"
          ></i>
          <i
            class="el-icon-edit icon-margin"
            @click="editNode(data, node)"
            title="修改"
          ></i>
          <i
            class="el-icon-delete icon-margin"
            @click="deleteNode(data, node)"
            title="删除"
          ></i>
          <!-- <i class="el-icon-remove-outline icon-margin" @click.stop="deleteNode(data)" title="禁用"></i> -->
        </span>
      </span>
    </el-tree>

  </div>
</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    treeData: {
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
      defaultExpandedNode: [],
      treeDataRow: null,
      // 树结构
      // nodeTreeDialogVisible: false,
      // nodeTreeForm: {
      //   name: ''
      // },
      // nodeTreeRules: {
      //   name: [
      //     { required: true, max: 64, message: '请输入名称', trigger: 'blur' }
      //   ]
      // },

      defaultProps: {
        // 放开
        children: 'childrenList',
        // children: 'children',
        label(data) {
          return data.name;
        }
      },
      fun: null,
      newInputContent: '',
      inputName: '',
      currentEditNodeData: null
      // showEditInput: false
    };
  },
  computed: {},
  watch: {
    searchInput: function (val) {
      this.newInputContent = val;
      this.debounce(this.inputSearchContent, 1000);
    }
  },
  created() { },
  mounted() {
    setTimeout(x => {
      // console.log(313, this.$refs.gradingTree.store.root);
      this.setLeaf(this.$refs.gradingTree.store.root.childNodes);
    }, 1000);
    this.allNodeAddType(this.treeData);
  },
  methods: {
    addBudget(data) {
      this.$emit('openAddBudgetDialog', {
        title: '新建',
        isTypeChild: true,
        rowData: data
      });
    },
    allNodeAddType(list) {
      list.forEach(y => {
        // y.showEditInput = false;
        this.$set(y, 'showEditInput', false);
        if (y.childrenList && y.childrenList.length) {
          this.allNodeAddType(y.childrenList);
        }
      });
    },
    setLeaf(data) {
      // console.log('data', data);
      data.forEach(x => {
        if (x.childNodes.length) {
          this.setLeaf(x.childNodes);
        } else {
          this.$set(x, 'isLeaf', false);
          // if (x.data.type == 2) {
          //   this.$set(x, 'isLeaf', false);
          // }
        }
      });
    },
    // 获取右侧子树
    getBudgetList(data) {
      this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: {
            parentId: data.id,
            status: '1',
            annually: this.$parent.selectYear,
            departmentId: data.departmentId,
            type: data.projectId ? '3' : '1' // 传1代表公司  传3代表项目
          }
        },
        res => {
          if (res.isSuccess) {
            if (res.data.dataList.length) {
              data.childrenList = res.data.dataList;
              setTimeout(x => {
                console.log(313, this.$refs.gradingTree.store.root);
                this.setLeaf(this.$refs.gradingTree.store.root.childNodes);
              }, 1000);
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    debounce: function (fn, wait) {
      // 防抖函数
      if (this.fun !== null) {
        clearTimeout(this.fun);
      }
      this.fun = setTimeout(fn, wait);
    },
    packUpAll() {
      for (var i = 0; i < this.$refs.gradingTree.store._getAllNodes().length; i++) {
        this.$refs.gradingTree.store._getAllNodes()[i].expanded = false;
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
    editNode(data, node) {
      this.currentEditNodeData = data;
      this.inputName = data.name;
      this.treeDataRow = data;
      this.$set(data, 'showEditInput', true);
    },
    blurInput() {
      this.showEditInput = false;
      this.$set(this.currentEditNodeData, 'showEditInput', false);
      if (!this.inputName) {
        this.$message.error('名称不能为空！');
        return;
      }

      this.$set(this.currentEditNodeData, 'name', this.inputName);
      this.updateBudgetChildList(this.treeDataRow);
    },
    updateBudgetChildList(data) {
      data.name = this.inputName;
      this.$axios.post(
        Api.budgetManage.budgetTypeUpdate,
        {
          data: data
        },
        res => {
          if (res.isSuccess) {
            this.$message.success('修改成功');
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },

    handleNodeExpand(data, data2, data3) {
      console.log('handleNodeExpand');
      if (!data.childrenList.length) {
        this.getBudgetList(data);
      }

      // this.treeDataRow = data;
      // this.$emit('clickChildTree', data);
    },
    handleNodeClick(data, data2, data3) {
      // this.treeDataRow = data;
      // this.$emit('clickChildTree', data);
    },
    // 重置表单
    resetForm(formName) {
      if (this.$refs[formName] !== undefined) {
        this.$refs[formName].resetFields();
      }
    },
    deleteBudgetChildList(data, node) {
      this.$axios.post(
        Api.budgetManage.budgetTypeDelete,
        {
          data: data
        },
        res => {
          if (res.isSuccess) {
            const parent = node.parent;
            const childrenList = parent.data.childrenList || parent.data;
            const index = childrenList.findIndex(d => d.id === data.id);
            childrenList.splice(index, 1);
            this.$message.success('删除成功');
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    deleteNode(data, node) {
      this.$confirm('确定删除?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          this.deleteBudgetChildList(data, node);
        })
        .catch(() => { });
    }
  }
};
</script>
<style lang="scss" scoped>
.main-left-tree {
  height: 100%;
  width: 100%;
  // width: 300px;
  background: #fff;
  float: left;
  overflow: auto;

  .span-tree-icons {
    margin-left: 30px;
  }

  .framework-tree {
    & ::v-deep .el-tree-node.is-current > .el-tree-node__content {
      background-color: #f0f7ff;
      color: #1890ff;

      .span-tree-icons {
        display: inline-block;
      }

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

    // & ::v-deep .el-tree-node__content {
    //   height: 40px;
    //   position: relative;
    // }
    ::v-deep .el-tree-node__content {
      height: 100%;
      position: relative;
    }

    .custom-tree-node {
      position: relative;
      width: 100%;
      height: 40px;
      line-height: 40px;

      .icon-margin {
        margin-left: 10px;
      }

      &:hover {
        .span-tree-icons {
          display: inline-block;
        }

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
        display: none;
      }
    }
  }
}
</style>
