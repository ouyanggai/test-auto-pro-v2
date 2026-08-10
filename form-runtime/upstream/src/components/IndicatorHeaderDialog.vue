<!--
 * @Descripttion:集团人员和部门选择弹窗-用于表单里面打开的弹窗。（挪到src/component文件夹下，支持共用）（这里暂时不用）
 * @Author: zhengzetao
 * @Date: 2022-3-30
-->
<template>
  <el-dialog :visible="visible"
  ref="selectCompanyDialog"
    :title="fielSelectType == 'company' ? '选择人员' : fielSelectType == 'department' ? '选择部门' : fielSelectType == 'selectCompany' ? '选择公司' :'选择岗位'" width="50%"
    top="100px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog"
    @close='handleClose'>
    <el-tabs v-model='activeName' @tab-click="tabClick" type="card" class="dialog-container" id="testTab">
      <el-input placeholder="请输入" v-model="filterText" clearable>
      </el-input>
      <el-tab-pane name="first" label="公司架构">
        <el-tree :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10" :filter-node-method="filterNode" ref="companyTree">
          <span slot-scope="{node,data}">
            <!-- 选择人员 -->
            <template v-if="fielSelectType == 'company'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 5" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
            </template>
            <!-- 只选择公司 -->
            <template v-if="fielSelectType == 'selectCompany'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 1" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
            </template>

            <template v-if="fielSelectType == 'department'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 2 || (data.type == 1&& node.level != 1)" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
            </template>

            <template v-if="fielSelectType == 'duty'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 4" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <!-- <span style="color:#ccc;margin-left: 10px;">{{data.roleName}}</span> -->
            </template>

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
    fielSelectType: {
      type: String,
      default: ''
    },
    companyId:{
      type: String,
      default: ''
    },
    selectUserCompanyId:{ //选择用户的公司
      type: String,
      default: ''
    },
    departmentId:{ //选择部门的id
      type: String,
      default: ''
    },
    //是否需要关联只显示选中人员公司
    isRelative:{
      type:Boolean,
      default:true
    },
    // 关联选择部门的部门id
    relDepartmentId:{
      type:String,
      default:''
    },
  },
  data() {
    return {
      filterText: '',
      selectRadio: null,
      treeData: [],
      activeName: 'first',
      chooseHeaderRadio: '',
      defaultProps: {
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      }
    };
  },
  computed: {},
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.$refs.companyTree.filter(val);
      }
      // if (this.$refs.projectTree) {
      //   this.$refs.projectTree.filter(val);
      // }
      // if (this.$refs.associateTree) {
      //   this.$refs.associateTree.filter(val);
      // }
    }
  },
  created() { },
  mounted() {
    this.init();
  },
  methods: {
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    radioChange(node) {
      var func = (node,type) => {
        let selectType = 1
        if(type)selectType = type
        if (node.parent.data.type == selectType) {
          return node.parent.data;
        } else {
          return func(node.parent);
        }
      };
      this.selectRadio = func(node);
      if(this.fielSelectType == 'company'){
        this.departObj = func(node,2)
        console.log('this.departObj',this.departObj)
      }
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    tabClick() {

    },
    init() {
      this.getCompanyTree();
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
            flag: this.fielSelectType == 'duty' ? 4 : 3,
            id: this.selectUserCompanyId||localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData = res.data
            console.log(this.companyId,'this.companyId+++++****************************',this.selectUserCompanyId,this.departmentId)
            if(this.companyId && this.isRelative){
              if(res.data[0].id == this.companyId){
                const arr = []
                 res.data[0].childrenList.map(item=>{
                  if(item.type ==2){
                    arr.push(item)
                  }
                })
                this.treeData[0].childrenList = arr

              }else{
                const data = res.data[0].childrenList.filter(item=>{
                  return item.id == this.companyId
                })
                console.log(data,'+++',this.$parent.$refs.generateForm.getValues())
                this.treeData[0].childrenList=data
                if(this.fielSelectType == 'duty'&&!this.selectUserCompanyId){ // 人员增补表岗位\人员需求
                  const departmentId = this.departmentId||this.$parent.$refs.generateForm.getValue('department_id')
                  console.log(departmentId,'999',this.$parent.$refs.generateForm.getValues())
                  const nodeFamily = this.getNodeFamilyTree(this.treeData[0],departmentId)
                  console.log(nodeFamily,'666666')
                  this.treeData[0]=nodeFamily
                }
              }
            }
            if(this.selectUserCompanyId){ // 关联选中用户的公司部门
              console.log(res.data,'res.data+++',this.treeData)
              const data = res.data[0].childrenList.filter(item=>{
                  return item.id == this.selectUserCompanyId
                })
                console.log(data,'+++')
                this.treeData[0].childrenList=data
                console.log(this.treeData,'666666')
                if(this.fielSelectType == 'duty'){
                  const departmentId = this.$parent.$refs.generateForm.getValue('company_id')
                  const nodeFamily = this.getNodeFamilyTree(this.treeData[0],departmentId)
                  console.log(nodeFamily,'666666',this)
                  this.treeData[0]=nodeFamily
                }

            }
            if(this.fielSelectType == 'department'){
              const arr = this.filterPerson(this.treeData,'5')
              this.treeData = arr
              console.log(this.treeData,'department')
              console.log(this.$refs,'department')
              if(this.relDepartmentId){
                const nodeFamily = this.findNodeAndSubset(this.treeData[0],this.relDepartmentId)
                console.log(nodeFamily,'tree++++++++++++++')
                this.treeData = [nodeFamily]
                // this.$forceUpdate()
              }else{
              }
            }
            if(this.fielSelectType == 'selectCompany'){
              const arr = this.filterPerson(this.treeData,'2')
              this.treeData = arr
            }
            console.log(this.treeData,'this.treeData')
            if(this.relDepartmentId&&this.fielSelectType == 'duty'){
              const departmentId = this.departmentId
              const nodeFamily = this.getNodeFamilyTree(this.treeData[0],departmentId)
              console.log(nodeFamily,'666666')
              this.treeData[0]=nodeFamily
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    //获取节点的父子集
    getNodeFamilyTree(tree, targetId) {
      // 用于存储结果的树结构
      let result = null;

      // 深度优先搜索函数
      function dfs(node, parentId) {
          // 如果找到目标节点，则构建结果树
          console.log(targetId,'9999******',node.name,node.id);
          if (node.id === targetId) {
              // 复制当前节点作为结果树的根
              result = { ...node, childrenList: [] };
            console.log(result,'9999',node);
              // 添加直接子节点到结果树中
              (node.childrenList || []).forEach(childNode => {
                if(childNode.type != 2){
                  result.childrenList.push({ ...childNode, childrenList: [] }); // 不递归添加更深层次的子节点
                }

              });

              return true; // 结束搜索
          }

          // 如果不是目标节点，则继续搜索其子节点
          for (let child of (node.childrenList || [])) {
              if (dfs(child, node.id)) {
                  return true; // 在子树中找到目标节点
              }
          }

          return false; // 未在当前子树中找到目标节点
      }

      // 从根节点开始搜索
      dfs(tree, null);

      return result; // 返回结果树或null（如果未找到目标节点）
  },
    confirmTaskHeader() {
      const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
      const choseObj2 = this.getChidlren(this.chooseHeaderRadio, this.treeData2);
      const choseObj3 = this.getChidlren(this.chooseHeaderRadio, this.treeData3);
      // console.log('choseObj', choseObj);
      // console.log('choseObj2', choseObj2);
      if (choseObj || (choseObj2 || choseObj3)) {
        this.$emit('selectHeader', choseObj || (choseObj2 || choseObj3), this.selectRadio,this.departObj);
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
    },
    //过滤掉人
    filterPerson(treeData, excludeType) {
        // 辅助函数，用于递归检查并过滤节点
        function filterNode(node) {
            // 如果当前节点的type是要排除的，返回null
            if (node.type === excludeType) {
                return null;
            }

            // 递归处理子节点
            if (node.childrenList) {
                node.childrenList = node.childrenList.filter(filterNode);
                // 如果子节点数组为空，则删除该属性以避免不必要的嵌套
                if (node.childrenList.length === 0) {
                    delete node.childrenList;
                }
            }

            // 返回当前节点（如果它没有被排除）
            return node;
        }

        // 调用辅助函数并返回过滤后的树
        return treeData.filter(filterNode);
      },
    findNodeAndSubset(tree, nodeId) {
        // Helper function to recursively find the node and return its children
        function recursiveFind(node) {
          if (node.id === nodeId) {
            return node; // Return the children of the found node
          }
          
          // Iterate through the children and recursively search
          for (let child of node.childrenList || []) {
            const result = recursiveFind(child);
            if (result) {
              return result; // Return the result if found
            }
          }
          
          return null; // Return null if not found
        }
      
        return recursiveFind(tree); // Start the search from the root of the tree
      }
  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  .dialog-container {
    // height: 600px;
    height: 48vh;
    overflow-y: hidden;
    display: flex;
    flex-direction: column;
  }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}
::v-deep .el-tab-pane{
  height: calc(100% - 30px);
  overflow-y: auto;
}
</style>
