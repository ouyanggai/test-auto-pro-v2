<!--
 * @Descripttion:任务负责人弹窗
 * @Author: zhengzetao
 * @Date: 2021-6-18
-->
<template>
  <el-dialog :visible="visible" title="选择人员" width="50%" top="100px" :close-on-click-modal="false"
    class="adjust-department-dialog" @close='handleClose'>
    <el-tabs v-model='activeName' @tab-click="tabClick" type="card" class="dialog-container" id="testTab">
      <el-input placeholder="请输入人员名称" v-model="filterText" clearable>
      </el-input>
      <el-tab-pane name="companyTree" label="公司架构">
        <!-- v-if="!associateId" -->
        <!-- v-elTabPermission="'groupTaskManage/taskArrange/add/taskHeader/companyFW'" -->
        <el-tree :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10"
          :filter-node-method="filterNode" ref="companyTree">
          <!-- node-key="id" 后端返回的id有重复，先注释掉看看能不能不要也能用-->
          <span slot-scope="{node,data}">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 5"><span></span></el-radio>
            <span>{{data.name}}</span>
            <span style="color:#ccc;margin-left: 10px;">{{data.roleName}}</span>
          </span>
        </el-tree>
      </el-tab-pane>
      <el-tab-pane name="projectTree" label="项目部架构" v-if="!!associateId">
        <!-- v-elTabPermission="'groupTaskManage/taskArrange/add/taskHeader/projectFW'" -->
        <!-- <el-input
          placeholder="请输入内容"
          prefix-icon="el-icon-search"
          v-model="searchVal"
        >
        </el-input> -->
        <el-tree :data="treeData2" :props="defaultProps" :default-expand-all="true" :indent="10" node-key="id"
          :filter-node-method="filterNode" ref="projectTree">
          <span slot-scope="{node,data}">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.flag == 1"><span></span></el-radio>
            <span>{{data.name}}</span>
            <!-- <span style="color:#ccc;margin-left: 10px;">{{data.jobs}}</span> -->
          </span>
        </el-tree>
      </el-tab-pane>
      <el-tab-pane name="associateTree" label="项目相关方" v-if="!!associateId">
        <!-- v-elTabPermission="'groupTaskManage/taskArrange/add/taskHeader/projectRP'" -->
        <el-tree :data="treeData3" :props="defaultProps" :default-expand-all="true" :indent="10" node-key="id"
          :filter-node-method="filterNode" ref="associateTree">
          <span slot-scope="{node,data}">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.flag == 1"><span></span></el-radio>
            <span>{{data.name}}</span>
            <!-- <span style="color:#ccc;margin-left: 10px;">{{data.jobs}}</span> -->
          </span>
        </el-tree>
      </el-tab-pane>
    </el-tabs>

    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="confirmTaskHeader">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import { localstorageGet } from '@/utils/auth';

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
    associateId: {
      type: [Number, String],
      default: ''
    },
    taskHeaderId: {
      type: [Number, String],
      default: ''
    },
    projectRelationType: {
      type: String,
      default: ''
    }
  },
  // watch: {
  //   visible(val) {
  //     if (val) {
  //       this.chooseHeaderRadio = '';
  //     }
  //   }
  // },
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
      treeData2: [],
      treeData3: [],
      // searchVal: '',
      activeName: 'companyTree',
      chooseHeaderRadio: '',
      defaultProps: {
        // children: 'children',
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      defaultProps2: {
        // children: 'children',
        children: 'childrenList',
        label(data) {
          return data.mainDeptOrgName;
        }
      },
      filterText: ''
    };
  },
  computed: {},
  created() { },
  mounted() {
    // console.log(!!this.associateId);
    // this.activeName = !this.associateId ? 'companyTree' : this.hasPermission('groupTaskManage/taskArrange/add/taskHeader/projectFW') ? 'projectTree' : 'associateTree';
    this.activeName = !this.associateId ? 'companyTree' : 'projectTree';
    this.chooseHeaderRadio = this.taskHeaderId;
    // console.log('this.chooseHeaderRadio', this.chooseHeaderRadio);

    this.init();
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    tabClick() {

    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    init() {
      if (!this.associateId) { // 没有项目id
        this.getCompanyTree();
      } else {
        this.getCompanyTree();
        this.getProjectFrameworkTree();
        this.getProjectRelatedPartyTree();
      }
    },
    hasPermission(href) { // 除了html中可以用v-permission控制节点隐藏，代码中需要用到的权限判断用这个方法
      const btnPermissionList = store.getters.btnPermissionList;
      const flag = btnPermissionList.some(item => {
        return item.href == href;
      });
      return flag; // 有权限返回true
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
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getProjectFrameworkTree() { // 获取项目部架构数据
      console.log('this.projectRelationType', this.projectRelationType);
      this.$axios.post(
        Api.taskManage.taskArrange.getMainDeptTree,
        {
          data: {
            projectId: this.associateId,
            relationType: this.projectRelationType
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData2 = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getProjectRelatedPartyTree() { // 获取项目相关方数据
      this.$axios.post(
        Api.taskManage.taskArrange.getRelatedPartyTree,
        {
          data: {
            projectId: this.associateId,
            relatedPartyTreeTypeEnum: 'THREE_LEVEL'
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData3 = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    confirmTaskHeader() {
      const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
      const choseObj2 = this.getChidlren(this.chooseHeaderRadio, this.treeData2);
      const choseObj3 = this.getChidlren(this.chooseHeaderRadio, this.treeData3);
      // console.log('choseObj', choseObj);
      // console.log('choseObj2', choseObj2);
      // console.log(choseObj3);
      if (choseObj || (choseObj2 || choseObj3)) {
        this.$emit('selectHeader', choseObj || (choseObj2 || choseObj3));
        // this.$emit('getHeaderDutyList');
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
    }
  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  // .dialog-container {
  //   // height: 600px;
  //   height: 48vh;
  //   overflow-y: auto;
  // }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }

  ::v-deep.el-tree {
    height: 48vh;
    overflow-y: auto;
  }
}
</style>
