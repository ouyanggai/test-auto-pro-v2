<!--
 * @Descripttion: 导入明细模板树结构
 * @Author: zhengzetao
 * @Date: 2022-06-07
-->
<template>
  <el-dialog :visible="visible" title="导入明细模板" width="50%" :close-on-click-modal="false"
    class="adjust-department-dialog" @close='handleClose'>
    <el-checkbox v-model="checkAll" :indeterminate="isIndeterminate" @change="selectAll">全选</el-checkbox>
    <el-tree :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10" node-key="id" show-checkbox
      @check="check" ref="detailTree">
    </el-tree>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="postTml">导 入</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    // treeData: {
    //   type: Array,
    //   default: function () {
    //     return [];
    //   }
    // },
    selectRow: {
      type: Object,
      default: function () {
        return {};
      }
    }
  },
  data() {
    return {
      checkAll: false,
      isIndeterminate: false,
      firstLevelList: [],
      defaultProps: {
        children: 'childrenList',
        // children: 'children',
        label(data) {
          return data.name;
        }
      },
      treeData: [{
        id: 1,
        name: '一级 1',
        childrenList: [{
          id: 4,
          name: '二级 1-1',
          childrenList: [{
            id: 9,
            name: '三级 1-1-1'
          }, {
            id: 10,
            name: '三级 1-1-2'
          }]
        }]
      }, {
        id: 2,
        name: '一级 2',
        childrenList: [{
          id: 5,
          name: '二级 2-1'
        }, {
          id: 6,
          name: '二级 2-2'
        }]
      }, {
        id: 3,
        name: '一级 3',
        childrenList: [{
          id: 7,
          name: '二级 3-1'
        }, {
          id: 8,
          name: '二级 3-2'
        }]
      }],
      treeNodeNumberList: []

    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    this.treeData.forEach(x => {
      this.firstLevelList.push(x.id);
    });
    this.getTreeNode(this.treeData);
  },
  methods: {
    check(a, b) {
      const checkedCount = this.$refs.detailTree.getCheckedKeys().length;
      this.checkAll = checkedCount === this.treeNodeNumberList.length;
      console.log(checkedCount, this.treeNodeNumberList.length);
      this.isIndeterminate = checkedCount > 0 && checkedCount < this.treeNodeNumberList.length;
    },
    selectAll(val) {
      console.log(val);
      this.$refs.detailTree.setCheckedKeys(val ? this.firstLevelList : []);
      this.isIndeterminate = false;
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    getTreeNode(list) {
      list.forEach(y => {
        this.treeNodeNumberList.push(y.id);
        if (y.childrenList && y.childrenList.length) {
          this.getTreeNode(y.childrenList);
        }
      });
    },
    postTml() {
      this.$axios.post(
        Api.frameworkInfo.adjustmentDepartment,
        {
          data: {
          }
        },
        res => {
          if (res.isSuccess) {
            this.$message.success('导入成功！');
            this.$emit('update:visible', false);
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
.adjust-department-dialog {
  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}
</style>
