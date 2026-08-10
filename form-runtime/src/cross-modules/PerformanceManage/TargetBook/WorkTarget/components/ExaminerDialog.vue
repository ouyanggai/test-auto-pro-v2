<!--
 * @description:
 * @Author: Calvin
 * @Date: 2022-03-23 14:48:49
 * @FilePath: \src\views\PerformanceManage\TargetBook\WorkTarget\components\ExaminerDialog.vue
-->
<template>
  <el-dialog :visible="visible" title="选择人员" :close-on-click-modal="false" width="50%" top="100px" @close='handleClose'
    class="examiner-dialog" append-to-body>
    <el-input placeholder="请输入人员名称" v-model="filterText" clearable>
    </el-input>
    <el-tree :data="treeData" :props="defaultProps" :default-expand-all="false"
      :default-expanded-keys="defaultFirstLevelId" :indent="10" :filter-node-method="filterNode" ref="companyTree"
      node-key="id">
      <span slot-scope="{node,data}">
        <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 5"><span></span></el-radio>
        <span>{{data.name}}</span>
        <span style="color:#ccc;margin-left: 10px;">{{data.roleName}}</span>
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
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.$refs.companyTree.filter(val);
      }
      if (this.$refs.projectTree) {
        this.$refs.projectTree.filter(val);
      }
      if (this.$refs.associateTree) {
        this.$refs.associateTree.filter(val);
      }
    }
  },
  data() {
    return {
      treeData: [],
      chooseHeaderRadio: '',
      defaultProps: {
        // children: 'children',
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      defaultFirstLevelId: [],
      filterText: ''
    };
  },
  computed: {},
  created() { },
  mounted() {
    this.chooseHeaderRadio = this.examinerId;
    this.getCompanyTree();
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },

    submit() {
      const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
      if (choseObj) {
        this.$emit('select', choseObj);
      }
      this.handleClose();
    },
    getChidlren(id, data) {
      var hasFound = false; // 表示是否有找到id值
      var result = null;
      function fn(data) {
        if (Array.isArray(data) && !hasFound) { // 判断是否是数组并且没有的情况下，
          data.forEach(item => {
            if (item.id === id) { // 数据循环每个子项，并且判断子项下边是否有id值
              result = item; // 返回的结果等于每一项
              hasFound = true; // 并且找到id值
            } else if (item.childrenList) {
              fn(item.childrenList); // 递归调用下边的子项
            }
          });
        }
      }
      fn(data);
      if (result) {
        result.mapProMainDeptId = data[0].id;
      }
      return result;
    },
    getCompanyTree() { // 获取公司部门架构数据
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: 3,
            id: localstorageGet('companyId') // 公司id
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
.examiner-dialog {
  & ::v-deep.el-radio {
    margin-right: 0px;
  }

  ::v-deep.el-tree {
    height: 48vh;
    overflow-y: auto;
  }
}

::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
