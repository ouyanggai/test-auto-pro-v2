<!--
 * @description:  人员、部门、岗位、公司、角色选择穿梭组件
 * @Author: zhengzetao
 * @Date: 2024-12-22
-->
<template>
  <div class="transe-div">
    <div class="left-select-person">
      <div style="padding: 10px 10px 5px 10px;background: #f3f3f3;">
        <el-input v-if="activeName == 'role'" :placeholder="'请输入角色名称'" v-model="roleSearchName" @input="searchRoleName" clearable></el-input>
        <el-input v-else :placeholder="'请输入'+ rangeMapList[activeName]['name'] +'名称'" v-model="filterText" clearable></el-input>
      </div>
      <!-- :default-expanded-keys="defaultFirstLevelId" -->
      
      <!-- 角色 -->
      <div v-if="activeName == 'role'" style="padding: 5px;">
        <div style="height:230px;overflow:auto;padding: 5px;">
          <div v-for="(item,index) in roleList" :key="item.id" @click="selectRole(item,index)" :class="{'roleHight': activeRoleItem.id == item.id}"
           style="padding: 10px 6px;cursor: pointer;">
            {{item.name}}
          </div>
        </div>
        <div style="padding: 10px 10px 5px 10px;background: #f3f3f3;">
          <div style="padding:10px 0;display: flex;align-items: center;">
            显示人员姓名
          </div>
        </div>
        <div style="height:106px;overflow:auto;padding-top: 4px;">
          <el-tag v-for="user in activeRoleItem.roleUserList" :key="user.id">{{ user.userVo.name }}</el-tag>
        </div>
      </div>
      <!-- 人员、公司、部门、岗位 -->
      <el-tree
        v-else
        :filter-node-method="filterNode"
        node-key="id"
        ref="personSelectTree"
        :data="treeData"
        :props="defaultProps"
        :check-strictly="activeName == 'department'"
        :default-expand-all="true"
        :default-checked-keys="defaultCheckedKeys"
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
          {{ '已选' + rangeMapList[activeName]['name'] }}
        </div>
      </div>
      <div style="padding: 10px;" class="choosed-checked">
        <el-checkbox-group v-model="checkedPerson">
          <el-checkbox v-for="person in personList" :label="person.id" :key="person.id">{{person.name}}</el-checkbox>
        </el-checkbox-group>
        </div>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone,getObjById } from '@/utils';
import mixin from '@/views/flowLibrary/components/mixin.js';

export default {
  name: '',
  components: {},
  props: {
    companyId: { // 当前用户所在公司id 在personnel时，根据只展示该公司人员，专用于新绩效考核中分组设置中绑定人员时
      type: String,
      default: ''
    },
    activeName:{
      type:String,
      default:''
    },
    defaultCheckedKeys:{
      type:Array,
      default: function(){
        return [];
      }
    }
  },
  data() {
    const _this = this;
    return {
      filterText: '',
      treeData: [],
      chooseHeaderRadio: '',
      defaultProps: {
        children: 'childrenList',
        label(data) {
          return data.name;
        },
        disabled(data, node) {
          if (_this.activeName == 'department') {
            return node.data.disabled;
          } else {
            return false;
          }
        }
      },
      defaultFirstLevelId: [],
      checkboxList: [],
      loading: true,
      personList:[],
      // isIndeterminate:false,
      // checkAll:false,
      checkedPerson:[],
      rangeMapList:{
        "company":{
          name:'公司',
          flag:'7',
          type:'1',
        },
        "department":{
          name:'部门',
          flag:'2',
          type:'2',
        },
        "personnel":{
          name:'人员',
          flag:'3',
          type:'5',
        },
        "position":{
          name:'岗位',
          flag:'4',
          type:'4',
        },
        "role":{
          name:'角色',
          flag:'',
        },
      },
      // 选择角色
      roleList:[],
      originalRoleList:[],
      activeRoleItem:{},
      roleSearchName:'',
      
    };
  },
  mixins:[mixin],
  computed: {
    flagType:function(){
      let obj = this.rangeMapList[this.activeName];
      return obj?.flag || '';
    },
    nameType:function(){
      let obj = this.rangeMapList[this.activeName];
      return obj.name;
    },
    dataType:function(){
      let obj = this.rangeMapList[this.activeName];
      return obj?.type || '';
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
    console.log(this.activeName, 'this.activeName')
    if (this.activeName == 'role') { // 角色
      this.getRoleList();
    } else {
      this.getCompanyTree();
    }
  },
  mounted() {
    console.log('defaultCheckedKeys',this.defaultCheckedKeys)
    this.echoRoleData();
  },
  methods: {
    // 回显已选择角色的数据
    echoRoleData(){
      if (this.activeName == 'role') { // 角色
        setTimeout(x=>{
          // console.log('this.defaultCheckedKeys',this.defaultCheckedKeys)
          if (this.defaultCheckedKeys.length) {
            // console.log('roleList',this.roleList)
            let fileterRole = this.roleList.filter(x=> this.defaultCheckedKeys.includes(x.id));
            this.personList = fileterRole;
          }
        },100)
      }
    },
    // 回显已选择的数据(公司、部门、人员、岗位)
    echoTreeData(){
      setTimeout(x=>{
        // console.log('this.defaultCheckedKeys',this.defaultCheckedKeys)
        if (this.defaultCheckedKeys.length) {
          this.choosePerson();
        }
      },100)
    },
    searchRoleName(){
      if (this.roleSearchName) {
        let newArr = this.roleList.filter(y=>y.name.indexOf(this.roleSearchName) > -1);
        this.roleList = deepClone(newArr)
      } else {
        this.roleList = deepClone(this.originalRoleList);
      }
    },
    // 点击角色列表
    selectRole(item,index){
      this.activeRoleItem = item;
    },
    removePerson(){
      if(this.checkedPerson.length){
        this.checkedPerson.forEach(item=>{
          let index = this.personList.findIndex(el=>el.id == item)
          if(index > -1)this.personList.splice(index,1)
        })
        this.checkedPerson = []
        this.$emit('getSelectPerson',this.personList)
        // this.isIndeterminate = false
        // this.checkAll = false
      }
    },
    // 穿梭框向右穿梭
    choosePerson(){
      console.log('choosePerson')
      
      let personList = [];
      const obj = {};
      if (this.activeName == 'personnel') {
        const checkNode = this.$refs.personSelectTree.getCheckedNodes();
        personList = checkNode.filter(x => x.type == this.dataType);
        // let personList = checkNode.filter(x => x.type == this.dataType);
        // console.log(2,personList)
        this.personList = []
        personList.forEach(item=>{
          let id = item.id
          if(this.personList.findIndex(el=>el.id == id)==-1){
            this.personList.push(item)
          }
        })
        // console.log(4,this.personList)
      } else if(this.activeName == 'department') {
        console.log('选择部门')
        const checkNode = this.$refs.personSelectTree.getCheckedNodes();
        let checkNodes = []
        checkNode.forEach(item=>{
          // if(item.type == '2'){
          if(item.type == this.flagType){
            let allAncestorsList = this.getCheckTag(this.treeData, item.id);
            console.log('allAncestorsList',allAncestorsList)
            let firstCompanyObj = allAncestorsList.find(x=>x.type == '1');
            console.log('firstCompanyObj',firstCompanyObj)
            item.firstCompanyObj = firstCompanyObj;
            checkNodes.push(item);
          }
        })
        console.log('checkNode',checkNode)
        this.personList = checkNode;
      } else if (this.activeName == 'position'){
        console.log('选择岗位')
        const checkNode = this.$refs.personSelectTree.getCheckedNodes();
        console.log('checkNode',checkNode)

        personList = checkNode.filter(x => x.type == this.flagType);
        personList.forEach(async item=>{
          console.log('item',item)
          let result = await this.getUserListByDutyId(item.id);
          this.$set(item,'userList',result)
          let currentTarget = getObjById(this.treeData,item.parentId)
          console.log('currentTarget',currentTarget)
          this.$set(item,'departName', currentTarget.name)
        })
        this.personList = personList;
      } else if (this.activeName == 'company'){
        // console.log('选择公司')
        const checkNode = this.$refs.personSelectTree.getCheckedNodes();
        this.personList = checkNode;
        console.log('选择公司',this.personList)
      } else if (this.activeName == 'role'){
        this.personList.push(this.activeRoleItem)
        
        const newArr = this.personList.reduce((item, next) => {
          obj[next.name] ? '' : obj[next.name] = true && item.push(next);
          return item;
        }, []);
        this.personList = newArr;
      }  
      
      this.$emit('getSelectPerson',this.personList)
    },
    showFlowNodeDetail(){
      this.$emit('showFlowNodeDetail')
    },
    filterNode(value, data) {
      if (!value) return true;
      return (data.name.indexOf(value) !== -1 && data.type == this.dataType);
    },
    handleClose() {
      // this.$emit('update:visible', false);
    },
    // 给树结构数据添加disabled
    addPropertyToTree(arr, propName) {
      arr.forEach(item => {
        if (item.type == '2') {
          item[propName] = false;
        } else {
          item[propName] = true;
        }
        
        if (item.childrenList && item.childrenList.length) {
          this.addPropertyToTree(item.childrenList, propName);
        }
      });
    },
    getCompanyTree() { // 获取公司部门架构数据
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: this.flagType,
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          this.loading = false;
          if (res.isSuccess) {
            let data = res.data
            console.log(data,'data')
            //flag 7公司 flag3显示人员 flag2选择部门
            if(this.activeName == 'company'){
              let childrenList = deepClone(data[0].childrenList)
              data[0].childrenList = []
              // console.log('childrenList',childrenList)
              // let topCompany = deepClone(data)
              let topCompany = deepClone(data[0])
              // console.log('topCompany',topCompany)
              data[0].childrenList.push(topCompany)
              childrenList.forEach(item=>{
                data[0].childrenList.push(item)
              })
              data[0].name = '全选'
            } else if (this.activeName == 'department') { // 部门：给一些选项置灰
              this.addPropertyToTree(data, 'disabled');
            } else if (this.activeName == 'personnel' && this.companyId && res.data && res.data.length) { // 根据公司id设置范围
              data = this.findCompanyData(data, this.companyId);
            }
            this.$set(this, 'treeData', deepClone(data));
            // this.treeData = res.data;

            this.echoTreeData();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    findCompanyData(data, targetId) {
      const results = [];

      // 处理当前层级的每个节点
      const traverse = (node) => {
        // 1. 匹配目标ID：无论type如何都直接添加当前节点
        if (node.id === targetId) {
          results.push(node);
        }
        // 2. 递归终止条件：type>1 或 无子节点时停止递归
        if (Number(node.type) <= 1 && node.childrenList?.length > 0) {
          node.childrenList.forEach(child => {
            // 3. 仅当子节点type<=1时才继续递归
            if (Number(child.type) <= 1) traverse(child);
            // 4. 即使type>1，若匹配目标ID也添加（独立判断）
            else if (child.id === targetId) results.push(child);
          });
        }
      };

      // 初始化遍历（兼容数组/对象输入）
      Array.isArray(data) ? data.forEach(traverse) : traverse(data);
      console.log(results, 'results');
      return results;
    }
  }
};
</script>

<style scoped lang="scss">
// ::v-deep .el-dialog__body {
//   max-height: 600px;
// }
.roleHight {
  background: #8CC5FF;
  // color: #fff;
}
.transe-div {
  // height: calc(100% - 50px);
  height: calc(100vh - 460px);
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
    height: calc(100vh - 480px);
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
::v-deep {
  .select-person-dialog .el-dialog__body {
    height: 540px;
  }
}
</style>
