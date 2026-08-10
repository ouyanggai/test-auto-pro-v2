<!--
 * @description:  节点的人员选择
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    :visible="visible"
    :title="flag == '7' ? '选择公司' : flag == '2' ? '选择部门' : '选择人员'"
    :close-on-click-modal="false"
    width="50%"
    top="100px"
    append-to-body
    center
    @close="handleClose"
  >
    <!-- <el-input v-if="flag == '3'" placeholder="请输入人员名称" v-model="filterText" clearable></el-input> -->
    <div style="padding: 10px 10px 5px 10px;background: #f3f3f3;">
      <el-input :placeholder="'请输入'+ nameType +'名称'" v-model="filterText" clearable></el-input>
    </div>

    <!-- :default-expanded-keys="defaultFirstLevelId" -->
    <el-tree
      :filter-node-method="filterNode"
      node-key="id"
      ref="personSelectTree"
      :data="treeData"
      :props="defaultProps"
      :check-strictly="flag == '2'"
      :default-expand-all="true"
      :default-checked-keys="startDefaultCheckedKeys"
      show-checkbox
      :indent="10"
      auto-expand-parent
      @check="check"
    >
      <span slot-scope="{node,data}">
        <span>{{ data.name }}</span>
        <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
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
import { mapGetters } from 'vuex';
import Bus from '@/bus';
const RELATED_PARTY_COMPANY = 'RELATED_PARTY_COMPANY'; // 相关方
import { deepClone } from '@/utils';
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
    },
    // 每次选择的数量，如果有数量，只能选择人员，不能选择公司，超过数量则置灰
    selectLimit: {
      type: String,
      default: ''
    },
    defaultCheckedKeys: {
      type: Array,
      default: () => {
        return [];
      }
    },
    startDefaultCheckedKeys:{
      type:Array,
      default:()=>{
        return []
      }
    },
    flag: {
      type: String,
      default: 3
    }
    // company:{
    //   type:String,
    //   default:''
    // }
    // relative:{
    //   type:Boolean,
    //   default:false
    // }
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
          if (_this.selectLimit != '') { // 参数不传，全部不禁用
            if (node.data.type == 5) { // 组织架构里面人
              if (_this.selectList.length >= _this.selectLimit) {
                // 如果所选的人等于传入的限制数量时，除了本身，其他都禁用
                if (_this.selectList.findIndex(item => item == node.data.id) > -1) {
                  return false;
                } else {
                  return true;
                }
              }
              return false;
            } else {
              return true;
            }
          } else if (_this.flag == '2') {
            return node.data.disabled;
          } else {
            return false;
          }
        }
      },
      defaultFirstLevelId: [],
      checkboxList: [],
      company: '',
      selectList: [],
      rangeMapList:{
        "7":{
          name:'公司',
          type:'1',
        },
        "2":{
          name:'部门',
          type:'2',
        },
        "3":{
          name:'人员',
          type:'5',
        },
        // "position":{
        //   name:'岗位',
        //   flag:'4',
        //   type:'4',
        // },
        // "role":{
        //   name:'角色',
        //   flag:'',
        // },
      },
    };
  },
  computed: {
    ...mapGetters([
      'customerCode'
    ]),
    dataType:function(){
      let obj = this.rangeMapList[this.flag];
      return obj?.type || '';
    },
    nameType:function(){
      let obj = this.rangeMapList[this.flag];
      return obj.name;
    },
  },
  watch: {
    filterText(val) {
      if (this.$refs.personSelectTree) {
        this.$refs.personSelectTree.filter(val);
      }
    }
  },
  created() {
    this.chooseHeaderRadio = this.examinerId;
    if (this.$route.query?.relative && !this.$route.query?.isGeneralFlow) {
      this.company = localstorageGet('formDetailCompany');
    }
    this.getCompanyTree();
  },
  mounted() {
    console.log('dataType',this.dataType)
  },
  methods: {
    check(data, node) {
      this.selectList = node.checkedKeys;
    },
    filterNode(value, data) {
      if (!value) return true;
      // return (data.name.indexOf(value) !== -1 && data.type == '5');
      return (data.name.indexOf(value) !== -1 && data.type == this.dataType);
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    submit() {
      const checkNode = this.$refs.personSelectTree.getCheckedNodes();
      // console.log('checkNode',checkNode)
      // let relativeList = []
      // if(this.relative){
      //   let relativeChecNode = this.$refs.relativePersonSelectTree.getCheckedNodes()
      //   relativeList = relativeChecNode.filter(x => x.type == 5);
      // }
      if(this.flag == 3){ // 人员
        var personList = checkNode.filter(x => x.type == 5);
        // if(relativeList.length)personList = personList.concat(relativeList)
        this.$emit('select', personList);
      } else if (this.flag == 2) { // 部门
        let checkNodes = []
        checkNode.forEach(item=>{
          if(item.type == '2'){
            let allAncestorsList = this.getCheckTag(this.treeData, item.id);
            let firstCompanyObj = allAncestorsList.find(x=>x.type == '1');
            console.log('firstCompanyObj',firstCompanyObj)
            // item.firstCompanyObj = firstCompanyObj; // 这里改成直接赋值给部门name
            item.name = firstCompanyObj.name + ' / ' + item.name
            checkNodes.push(item);
          }
        })
        this.$emit('select', checkNodes);
      } else{ // 公司
        let checkNodes = []
        checkNode.forEach(item=>{
          if(!item.childrenList || !item.childrenList.length){
            checkNodes.push(item)
          }
        })
        this.$emit('select', checkNodes);
      }
      this.handleClose();
    },
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
            flag: this.flag,
            id: this.company || localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            // this.treeData = res.data;
            let data = res.data
            //flag 7公司 flag3显示人员 flag2选择部门
            if(this.flag == 7){ // 公司
              // data.forEach(item=>{
              //   if(item.childrenList && item.childrenList.length){
              //     item.childrenList.forEach(el=>{
              //       if(el.childrenList)delete el.childrenList
              //     })
              //   }
              // })
              let childrenList = deepClone(data[0].childrenList)
              data[0].childrenList = []
              console.log('childrenList',childrenList)
              // let topCompany = deepClone(data)
              let topCompany = deepClone(data[0])
              console.log('topCompany',topCompany)
              data[0].childrenList.push(topCompany)
              childrenList.forEach(item=>{
                // if(item.type == 1){
                  data[0].childrenList.push(item)
                // }
              })
              data[0].name = '全选'
            } else if (this.flag == 2) { // 部门：给一些选项置灰
              this.addPropertyToTree(data, 'disabled');
            }
            console.log('data',data)
            this.$set(this, 'treeData', deepClone(data));
            // this.defaultFirstLevelId = [data[0].id];


            // this.echoTreeData();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 回显已选择的数据(公司、部门、人员、岗位)
    // echoTreeData(){
    //   setTimeout(x=>{
    //     // console.log('this.startDefaultCheckedKeys',this.startDefaultCheckedKeys)
    //     if (this.startDefaultCheckedKeys.length) {
    //       this.choosePerson();
    //     }
    //   },100)
    // },
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
