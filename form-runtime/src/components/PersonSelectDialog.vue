<!--
 * @description:  节点的人员选择
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    :visible="visible"
    :title="title"
    :close-on-click-modal="false"
    width="50%"
    top="100px"
    append-to-body
    center
    @close="handleClose"
  >
    <el-input placeholder="请输入" v-model="filterText" clearable></el-input>
    <el-tree
      ref="personSelectTree"
      node-key="id"
      :data="treeData"
      :props="defaultProps"
      :default-expand-all="false"
      :default-expanded-keys="defaultFirstLevelId"
      show-checkbox
      :indent="10"
      auto-expand-parent
      :filter-node-method="filterNode"
      :default-checked-keys="defaultCheckedKeys"
      @check="check"
    >
      <span slot-scope="{data}">
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
export default {
  name: '',
  components: {},
  computed: {
    ...mapGetters([
      'customerCode'
    ])
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    examinerId: {
      type: [Number, String],
      default: ''
    },
    //每次选择的数量，如果有数量，只能选择人员，不能选择公司，超过数量则置灰
    selectLimit:{
      type:String,
      default:''
    },
    defaultCheckedKeys:{
      type:Array,
      default:()=>{
        return []
      }
    },
    flag:{
      type:String,
      default:3
    },
    fielSelectType:{
      type:String,
      default:''
    }
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
          } else {
            return false;
          }
        }
      },
      defaultFirstLevelId: [],
      checkboxList: [],
      company: '',
      selectList: []
    };
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
    if (this.$route.query?.relative) {
      this.company = localstorageGet('formDetailCompany');
    }
    if(this.fielSelectType == 'company'){
      this.getCompanyList();
    }else{
      this.getCompanyTree();
    }
    
  },
  mounted() {
    if(this.fielSelectType == 'company'){
      this.title = '选择公司';
    }else if(this.fielSelectType == 'department'){
      this.title = '选择部门';
    }else if(this.fielSelectType == 'users'){
      this.title = '选择人员';
    }else{
      this.title = '选择群组';
    }
  },
  methods: {
    getCompanyList(){
      this.$axios.post(
        "/web/user/api/company/getParentCompanyList",
        {
          data: {
            id: this.$store.state.user.companyId,
          },
        },
        (res) => {
          if (res.isSuccess) {
            this.treeData = res.data||[];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    check(data, node) {
      this.selectList = node.checkedKeys;
    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1
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
      if(this.flag == 3){
        var personList = checkNode.filter(x => x.type == 5);
        // if(relativeList.length)personList = personList.concat(relativeList)
        this.$emit('select', personList);
      }else{
        this.$emit('select', checkNode);
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
      if(this.fielSelectType =='userGroup'){
        this.$axios.post(
          '/web/user/api/userGroup/list',
          {
            data: {
              ownerId: this.$store.state.user.userId
            }
          },
          res => {
            if (res.isSuccess) {
              const list = []
              res.data&&res.data.map(item=>{
                const data =item
                data.name = item.groupName
                data.parentId = null
                // if(item.groupUsers&&item.groupUsers.length>0){
                //   data.childrenList = item.groupUsers.map(k=>{
                //     k.parentId = item.id
                //     return k
                //   })
                // }
                list.push(data)
              })
              this.$set(this, 'treeData', JSON.parse(JSON.stringify(list)));
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }else{
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
              if(this.fielSelectType == 'company'){
                data.forEach(item=>{
                  if(item.childrenList && item.childrenList.length){
                    item.childrenList.forEach(el=>{
                      if(el.childrenList)delete el.childrenList
                    })
                  }
                })
              }
              this.$set(this, 'treeData', JSON.parse(JSON.stringify(data)));
              this.defaultFirstLevelId = [data[0].id];

              // console.log('treeData', this.treeData);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }
      
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
