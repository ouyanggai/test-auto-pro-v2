<!--
 * @description:  节点的人员选择
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    :visible="visible"
    title="选择人员"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    width="900px"
    @close='handleClose(true)'
    append-to-body
    top="100px"
    custom-class="select-person-dialog"
  >
    <div v-if="isProject" :key="1">
      <el-checkbox-group v-model="checkboxList">
        <el-checkbox v-for="data in treeData" :label="data.id" :key="data.id">
          <span>{{ data.name }}</span>
          <span style="color:#ccc;margin-left: 10px;">{{ data.dutyName }}</span>
        </el-checkbox>
      </el-checkbox-group>
    </div>
    <div v-else class="transe-div" :key="2">
      <div class="left-select-person">
        <div style="padding: 10px 10px 5px 10px;background: #f3f3f3;">
          <el-input placeholder="请输入人员名称" v-model="filterText" clearable></el-input>
          <div style="padding:10px 0;display: flex;align-items: center;" v-if="nextNodeName">
            <!-- 节点名称：<el-link type="primary" :underline="false" @click="showFlowNodeDetail" style="cursor: default;">{{nextNodeName}}</el-link> -->
            节点名称：<span style="color: #409EFF;">{{nextNodeName}}</span>
          </div>
        </div>
        <el-tree
          :filter-node-method="filterNode"
          node-key="id"
          ref="personSelectTree"
          :data="treeData"
          :props="defaultProps"
          :default-expand-all="false"
          :default-expanded-keys="defaultFirstLevelId"
          show-checkbox
          :indent="10"
          auto-expand-parent
        >
          <span slot-scope="{node,data}">
            <span>{{ data.name }}</span>
            <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
          </span>
        </el-tree>
      </div>
      <div class="trans-bt">
        <el-button style="margin-bottom: 10px;" type="primary" icon="el-icon-d-arrow-right" @click="choosePerson"></el-button>
        <el-button type="primary" icon="el-icon-d-arrow-left" @click="removePerson"></el-button>
      </div>
      <div class="right-select-person">
        <div style="padding: 10px 10px 5px 10px;background: #f3f3f3;">
          <div style="padding:10px 0;display: flex;align-items: center;">
            已选人员
          </div>
        </div>
        <div style="padding: 10px;" class="choosed-checked">
          <!-- <el-checkbox :indeterminate="isIndeterminate" v-model="checkAll" @change="handleCheckAllChange">全选</el-checkbox> -->
          <el-checkbox-group v-model="checkedPerson" @change="handleCheckedChange">
            <el-checkbox v-for="person in personList" :label="person.id" :key="person.id">{{person.name}}</el-checkbox>
          </el-checkbox-group>
          </div>
      </div>
    </div>
    <span slot="footer">
      <el-button @click="handleClose(true)">取 消</el-button>
      <el-button
        type="primary"
        @click="submit"
      >确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone,arrayToTree2 } from '@/utils';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    isProject: {
      type: Boolean,
      default: false
    },
    examinerId: {
      type: [Number, String],
      default: ''
    },
    nextNodeName:{
      type:String,
      default:''
    },
    nextNodeProxyId:{
      type:String,
      default:''
    },
    nodeAuditType:{
      type:String,
      default:''
    },
    countersignNum:{
      type:Number,
      default: 1
    },
    nodeAuditScopeList:{
      type:Array,
      default: function(){
        return [];
      }
    }
  },
  data() {
    return {
      filterText: '',
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
      checkboxList: [],
      loading: true,
      personList:[],
      isIndeterminate:false,
      checkAll:false,
      checkedPerson:[],
    };
  },
  computed: {},
  inject: {
    jusgeCustomChoose: {
      value: 'jusgeCustomChoose', default: null
    }
  },
  watch: {
    filterText(val) {
      if (this.$refs.personSelectTree) {
        this.$refs.personSelectTree.filter(val);
      }
    }
  },
  created() {
    console.log(2)
    // this.chooseHeaderRadio = this.examinerId;
    if (this.isProject) { // 项目空间，获取项目得列表
      this.getProjectTree();
    } else { // 公司空间
      this.getCompanyTree();
    }
  },
  mounted() {
  },
  methods: {
    removePerson(){
      if(this.checkedPerson.length){
        this.checkedPerson.forEach(item=>{
          let index = this.personList.findIndex(el=>el.id == item)
          if(index > -1)this.personList.splice(index,1)
        })
        this.checkedPerson = []
        this.isIndeterminate = false
        this.checkAll = false
      }
    },
    handleCheckedChange(val){
      if(val.length >= this.personList.length){
        this.isIndeterminate = false
        this.checkAll = true
      }else{
        this.checkAll = false
        if(!val.length){
          this.isIndeterminate = false
        }else{
          this.isIndeterminate = true
        }
      }
    },
    handleCheckAllChange(val){
      if(val){
        this.checkedPerson = this.personList.map(item=>{
          return item.id
        })
      }else{
        this.checkedPerson = []
      }
    },
    choosePerson(){
      console.log('this.nextNodeProxyId',this.nextNodeProxyId)
      const checkNode = this.$refs.personSelectTree.getCheckedNodes();
      let personList = checkNode.filter(x => x.type == 5);
      this.personList = []
      personList.forEach(item=>{
        let id = item.id
        if(this.nextNodeProxyId)item.nodeProxyId = this.nextNodeProxyId
        if(this.personList.findIndex(el=>el.id == id)==-1){
          this.personList.push(item)
        }
      })
    },
    showFlowNodeDetail(){
      this.$emit('showFlowNodeDetail')
    },
    filterNode(value, data) {
      if (!value) return true;
      return (data.name.indexOf(value) !== -1 && data.type == '5');
    },
    // handleCheckChange(data, checked, indeterminate) {
    //   console.log(data, checked, indeterminate);
    // },
    // getCheckedNodes() {
    //   console.log(this.$refs.personSelectTree.getCheckedNodes());
    // },

    handleClose(flag) {
      console.log('审批人员自选')
      if (flag) {
        this.$emit('contractPayClose');
      }
      this.$emit('update:visible', false);
    },

    submit() {
      if (this.isProject) {
        var checkboxList = this.checkboxList.map(item => {
          var data = this.treeData.find(it => it.id == item);
          if (data) {
            return data;
          }
        });

        if (this.nodeAuditType == 'countersign') {
          if (!(checkboxList.length > this.countersignNum || checkboxList.length == this.countersignNum)) {
            this.$message.error('选择的人员需大于等于会签人数' + this.countersignNum);
            return;
          }
        }
        this.$emit('getSelectPerson', {
          checkboxPersonGroup: checkboxList
        })
      } else {
        // const checkNode = this.$refs.personSelectTree.getCheckedNodes();
        // const personList = checkNode.filter(x => x.type == 5);
        // this.$emit('getSelectPerson', {
        //   checkboxPersonGroup: personList
        // });
        const personList = this.personList.filter(x => x.type == 5);
        if(!personList.length)return this.$message.error('请选择审核人员')

        if (this.nodeAuditType == 'countersign') {
          if (!(personList.length > this.countersignNum || personList.length == this.countersignNum)) {
            this.$message.error('选择的人员需大于等于会签人数' + this.countersignNum);
            return;
          }
        }
        this.$emit('getSelectPerson', {
          checkboxPersonGroup: personList
        });
      }
      // if (this.jusgeCustomChoose && typeof (this.jusgeCustomChoose) == 'function') {this.jusgeCustomChoose();}
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
    // 找到父节点和祖先节点
    getCheckTag(list, id) {
      for (let i in list) {
        if (list[i].id === id) {
          return [list[i]]
        }
        if (list[i].childrenList != null) {
          let node = this.getCheckTag(list[i].childrenList, id)
          if (node !== undefined) {
            return node.concat(list[i])
          }
        }
      }
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
          this.loading = false;
          if (res.isSuccess) {
            this.treeData = res.data;
            let currentDepartmentId = localstorageGet('userDepartmentId')
            this.defaultFirstLevelId = [currentDepartmentId];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取项目得架构
    getProjectTree() {
      this.$axios.post(
        Api.departmentFrameworkPage.getProjectMainDeptChildrenUserDuty,
        {
          data: {
            id: localstorageGet('projectDepartmentId') // 项目部id
          }
        },
        res => {
          this.loading = false;
          if (res.isSuccess) {
            var array = [];
            var data = res.data || [];
            data.forEach(item => {
              const arr = this.transArr(item);
              array = array.concat(arr);
            });
            this.treeData = array;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    transArr(node) {
      const queue = [node];
      const data = []; // 返回的数组结构
      while (queue.length !== 0) { // 当队列为空时就跳出循环
        const item = queue.shift(); // 取出队列中第一个元素
        data.push({
          id: item.id,
          // parentId: item.parentId,
          name: item.name,
          dutyName: item.dutyName
        });
        const children = item.childrenList; // 取出该节点的孩子
        if (children) {
          for (let i = 0; i < children.length; i++) {
            queue.push(children[i]); // 将子节点加入到队列尾部
          }
        }
      }
      return data;
    }
  }
};
</script>

<style scoped lang="scss">
// ::v-deep .el-dialog__body {
//   max-height: 600px;
// }
::v-deep {
  .select-person-dialog .el-dialog__body {
    height: 540px;
    padding: 0 5px !important;
  }

  .transe-div {
    height: calc(100% - 50px);
    overflow: hidden;
    margin: 15px 0;
    padding: 10px;
    display: flex;
    justify-content: center;

    .left-select-person {
      width: 400px;
      border: 1px solid rgb(235, 238, 245);
      border-radius: 5px;
      height: 100%;
      overflow: hidden;

      .el-tree {
        height: calc(100% - 80px);
        overflow: auto;
      }
    }

    .trans-bt {
      width: 100px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;

      .el-button {
        margin-left: 0;
      }
    }

    .right-select-person {
      width: 400px;
      border: 1px solid rgb(235, 238, 245);
      border-radius: 5px;
      overflow: auto;
      height: calc(100vh - 448px);
      // height: calc(100% - 60px);

      .el-checkbox {
        margin-right: 0;
        width: 100%;
        margin-bottom: 3px;
      }
      .el-checkbox.is-checked{
        background: aliceblue;
      }
    }
  }
  .choosed-checked .el-checkbox__input{
    display: none;
  }
}
</style>
