<!--
 * @Descripttion:费用预算类型设置
 * @Author: zhengzetao
 * @Date: 2021-06-07
-->
<template>
  <div class="bigItem-manage-setting-container">
    <!-- <div style="padding:10px;background: #fff;border-bottom: 1px solid #e4e7ed;">
      <el-input placeholder="请输入" suffix-icon="el-icon-search" v-model="searchInput" style="width:300px;">
      </el-input>
    </div> -->
    <div style="padding:10px;background: #fff;border-bottom: 1px solid #e4e7ed;">
      <!-- {{ new Date().getFullYear() }}年 -->
      <el-date-picker
        v-model="selectYear"
        type="year"
        value-format="yyyy"
        @change="changeYear"
        placeholder="选择年"
        :clearable="false"
        style="width:100px;margin-right:10px;"
      >
      </el-date-picker>
      <el-autocomplete
        v-model="searchName"
        :fetch-suggestions="querySearch"
        placeholder="请输入查询归口"
        :trigger-on-focus="false"
        @select="selectSearchItem"
        :popper-append-to-body="false"
      >
        <i
          class="el-icon-search el-input__icon"
          slot="suffix"
        >
        </i>
        <template slot-scope="{ item }">
          <div class="name">{{ item.name }}</div>
        </template>
      </el-autocomplete>
    </div>
    <!-- 左侧公司树和一级归口树 -->
    <BigItemTreeSet
      :treeData="treeData"
      :defaultFirstLevelId="defaultFirstLevelId"
      :searchInput="searchInput"
      @clickBigItemTree="clickBigItemTree"
      @openDetailTml="openDetailTml"
      @openAddBudgetDialog="openAddBudgetDialog"
      ref='budgetTypeTreeCompo'
    />
    <div class="main-right-panel">
      <div>
        <h4 style="padding:10px;">
          费用归口分级明细
        </h4>
        <!-- 右侧费用归口分级树 -->
        <GradingDetailTree
          v-if="gradingTreeData.length"
          ref="a"
          :treeData="gradingTreeData"
          @openAddBudgetDialog="openAddBudgetDialog"
          @clickChildTree="clickChildTree"
          :searchInput="searchInput"
        />
      </div>
    </div>

    <ImportDetailTmlDialog
      :visible.sync="detailTmlDialogVisible"
      v-if="detailTmlDialogVisible"
    />

    <!-- 添加归口弹窗 -->
    <AddBudgetDialog
      :visible.sync="nodeTreeDialogVisible"
      v-if="nodeTreeDialogVisible"
      :modifyDialogTitle="modifyDialogTitle"
      @updateBudgetChildList="updateBudgetChildList"
    />
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
// 引入公共表单组件
// import DyTable from '@/components/DyTable';
import BigItemTreeSet from './components/BigItemTreeSet';
import GradingDetailTree from './components/GradingDetailTree';
import ImportDetailTmlDialog from './components/ImportDetailTmlDialog';
import AddBudgetDialog from './components/addBugdetDialog.vue';

export default {
  name: '',
  components: {
    BigItemTreeSet,
    GradingDetailTree,
    ImportDetailTmlDialog,
    AddBudgetDialog
  },
  data() {
    return {
      selectYear: '',
      searchName: '',
      searchInput: '',
      treeData: [],
      gradingTreeData: [],
      treeDataRow: {},
      treeChildDataRow: {},
      detailTmlDialogVisible: false,
      defaultFirstLevelId: [],
      modifyDialogTitle: '',
      nodeTreeDialogVisible: false,
      rowData: {},
      isTypeChild: false
    };
  },
  computed: {},
  watch: {
  },
  created() {
    this.selectYear = String(new Date().getFullYear());
  },
  mounted() {
    this.getDepartTree();
  },
  updated() {
  },
  methods: {
    querySearch(queryString, cb) {
      this.searchAutocompleteList().then((res) => {
        if (res.isSuccess) {
          cb(res.data.dataList);
        } else {
          this.$message.error(res.message);
        }
      });
    },
    // 根据名称模糊查询,结果显示在输入建议列表
    searchAutocompleteList() {
      return this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: {
            status: '1',
            annually: this.selectYear,
            parentId: 1,
            name: this.searchName,
            type: this.$store.state.user.groupDepartment != 'group' ? '3' : '1' // 传1代表公司  传3代表项目
          }
        }
      );
    },
    searchBigItemInDepart(item) {
      return this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: {
            parentId: 1,
            status: '1',
            annually: this.selectYear,
            departmentId: item.departmentId,
            type: this.$store.state.user.groupDepartment != 'group' ? '3' : '1' // 传1代表公司  传3代表项目
          }
        }
      );
    },
    // 点击查询项后根据部门id查出所有数据更新在部门下并高亮显示、展开父节点
    selectSearchItem(item) {
      item.budgetTypeTree = true;
      this.clickBigItemTree(item);

      this.searchBigItemInDepart(item).then((res) => {
        if (res.isSuccess) {
          res.data.dataList.forEach((x) => {
            x.budgetTypeTree = true;
          });
          const treeRef = this.$refs.budgetTypeTreeCompo.$refs.budgetTypeTree;
          treeRef.updateKeyChildren(item.departmentId, res.data.dataList);
          treeRef.setCurrentKey(item.id);

          const selected = treeRef.getCurrentNode();
          // 若当前组件有父节点 展开其所有祖宗节点
          if (treeRef.getNode(selected) && treeRef.getNode(selected).parent) {
            this.expandParents(treeRef.getNode(selected).parent);
          }
        } else {
          this.$message.error(res.message);
        }
      });
    },
    // 展开所有祖宗节点
    expandParents(node) {
      node.expanded = true;
      if (node.parent) {
        this.expandParents(node.parent);
      }
    },

    changeYear() {
      this.getDepartTree();
      this.gradingTreeData = [];
    },
    // 新增
    updateBudgetChildList(data) {
      this.$axios.post(
        Api.budgetManage.budgetTypeSave,
        {
          data: {
            name: data.name,
            parentId: this.rowData.id,
            departmentId: this.rowData.departmentId,
            type: '1',
            status: '1',
            annually: this.selectYear
          }
        },
        res => {
          if (res.isSuccess) {
            // this.getBudgetList(this.treeDataRow);
            if (this.isTypeChild) {
              this.rowData.childrenList.push(res.data);
            } else {
              this.gradingTreeData.push(res.data);
            }
            this.$message.success('新增成功！');
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    openAddBudgetDialog(data) {
      this.nodeTreeDialogVisible = true;
      this.modifyDialogTitle = data.title;

      // data.isTypeChild ? this.treeChildDataRow = data.rowData : this.treeDataRow = data.rowData;
      this.rowData = data.rowData;
      this.isTypeChild = data.isTypeChild;
      // this.rowData = data.isTypeChild ? this.treeChildDataRow : this.treeDataRow;
    },
    getDepartTree() {
      this.loading = true;
      this.$axios.post(
        Api.frameworkInfo.getCompanyFrameworkData, // 放开
        {
          data: {
            flag: '2', // flag为1,公司-部门-岗位  公司-部门--flag2
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          this.loading = false;
          if (res.isSuccess) {
            this.treeData = res.data;
            this.defaultFirstLevelId = [res.data[0].id];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    clickBigItemTree(data) {
      console.log(111, data);
      if (data.budgetTypeTree) { // 点击左侧费用类型树节点
        console.log(1, data);
        this.getBudgetList(data);
      } else {
        this.gradingTreeData = [];
      }
      this.treeDataRow = data;
    },
    clickChildTree(data) {
      this.treeChildDataRow = data;
    },
    // 获取右侧子树
    getBudgetList(data) {
      this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: {
            parentId: data.id,
            status: '1',
            annually: this.selectYear,
            departmentId: data.departmentId,
            type: this.$store.state.user.groupDepartment != 'group' ? '3' : '1' // 传1代表公司  传3代表项目
          }
        },
        res => {
          if (res.isSuccess) {
            this.gradingTreeData = res.data.dataList;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    openDetailTml() {
      this.detailTmlDialogVisible = true;
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

<style scoped lang="less">
.bigItem-manage-setting-container {
  height: 100%;
  padding: 14px;
  cursor: default;

  .main-right-panel {
    margin-left: 180px;
    border-left: 2px solid #e4e7ed;
    height: 100%;
    // height: calc(100% - 40px);
    overflow-y: auto;
    padding-left: 10px;
    background-color: #fff;

    // background-color: #f0f3f5;
    .main-right-personTable {
      // padding: 5px 20px;
      overflow: hidden;
      // margin-top: 10px;
    }
  }

  ::v-deep .el-dialog.is-fullscreen {
    width: 95%;
    height: 95%;
    margin: 20px auto;
  }
}
</style>

