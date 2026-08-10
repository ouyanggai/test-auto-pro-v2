<!--
 * @Descripttion:部门架构树结构
 * @Author: zhengzetao
 * @Date: 2021-06-07
-->
<template>
  <div class="main-left-tree">
    <!-- <el-tree class="framework-tree" :data="treeData" :props="defaultProps" @node-click="handleNodeClick"
      :highlight-current="true" :expand-on-click-node="false" :indent="20" node-key="id" empty-text="暂无数据"
      :default-checked-keys="defaultCheckedKeys" default-expand-all :current-node-key="currentNodeKey" ref="refTree">
      <span class="custom-tree-node" slot-scope="{ node, data }">
        <span>{{ node.label }}</span>
      </span>
    </el-tree> -->
    <ul>
      <li class="group-li" :class="{'select':currentNodeKey == item.id}" v-for="item in treeData" :key="item.id" @click="liClick(item)">{{ item.name }}</li>
    </ul>

  </div>
</template>

<script>
import Api from '@/api';
// import CreateOrganize from '@/views/MyRelatedParties/components/CreateOrganize';
import { localstorageGet } from '@/utils/auth';
export default {
  name: '',
  // components: { CreateOrganize },
  props: {
    isDepartmentFramework: {
      type: Boolean,
      default: false
    },
    isRelatedParties: {
      type: Boolean,
      default: false
    },
    topCompanyId: {
      type: String
    },
    treeData: {
      type: Array,
      default: () => {
        return [];
      }
    },
    companyFlowTemplateType:{
      type:Array,
      default:()=>{
        return []
      }
    }
  },
  data() {
    return {
      noTopCompany: false,
      organizeDialogVisible: false,
      isTop: false,
      isVisible: false,
      companyForm: {},
      modifyDialogTitle: '',
      treeDataRow: null,
      treeDataType: null, // edit,parentAdd,childrenAdd，departmentAdd
      // 树结构
      nodeTreeDialogVisible: false,
      nodeTreeForm: {
        name: ''
      },
      nodeTreeRules: {
        name: [
          { required: true, max: 64, message: '请输入名称', trigger: 'blur' }
        ]
      },
      rules: {
        name: { required: true, max: 64, message: '请输入公司名称', trigger: 'blur' },
        creditCode: { required: true, message: '请输入社会信用代码', trigger: 'blur' },
        companyMainType: { required: true, message: '请输入企业主体类别', trigger: 'blur' }
      },

      defaultProps: {
        // 放开
        children: 'childrenList',
        // children: 'children',
        label(data) {
          return data.name;
        }
      },
      defaultCheckedKeys: [],
      currentNodeKey: ''
    };
  },
  computed: {},
  watch: {
    treeData(n, o) {
      this.initClick()
    },
    companyFlowTemplateType(val,oldVal){
      this.initClick()
    }
  },
  created() {
    // 默认选中
    /* var mainCompanyId = localstorageGet('companyId');
    if (mainCompanyId) {
      this.mainCompanyId = this.currentNodeKey = mainCompanyId;
      this.defaultCheckedKeys = [mainCompanyId]
    } */
  },
  mounted() {},
  methods: {
    initClick(){
      if(this.companyFlowTemplateType.length && this.treeData.length){
        this.$nextTick(()=>{
          this.liClick(this.treeData[0])
        })
      }
    },
    liClick(row){
      this.currentNodeKey = row.id
      this.treeDataRow = row;
      this.$emit('clickFrameworkTree', row);
    },
    handleNodeClick(data, data2, data3) {
      // return
      this.treeDataRow = data;
      this.$emit('clickFrameworkTree', data);
    }
  }
};
</script>
<style lang="scss" scoped>
.main-left-tree {
  height: 100%;
  width: 300px;
  background: #fff;
  float: left;
  overflow: auto;

  .main-left-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 0 10px;

    .title {
      padding: 12px;
    }
  }

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

    & ::v-deep .el-tree-node__content {
      height: 40px;
      position: relative;
    }

    .custom-tree-node {
      position: relative;
      width: 100%;

      .span-tree-icons {
        position: absolute;
        right: 20px;
        top: -4px;
      }
    }
  }

  .tree-btn-popover ::v-depp .el-button {
    width: 100%;
    text-align: left;
  }

  .group-li{
    width: 100%;
    height: 40px;
    line-height: 40px;
    padding-left: 24px;
    cursor: pointer;
  }
  .group-li:hover{
    background: #F5F7FA;
  }
  .group-li.select{
    background-color: #f0f7ff;
    color: #1890ff;
    position: relative;
  }
  .group-li.select::after {
    position: absolute;
    content: "";
    width: 3px;
    height: 40px;
    background: #1890ff;
    top: 0px;
    right: 0px;
  }

}
.el-checkbox{
  width: 100%;
}
</style>
