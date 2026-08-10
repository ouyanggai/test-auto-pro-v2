<!--
 * @Descripttion: 组织架构的人员列表选择弹窗
 * @Author: zhengzetao
 * @Date: 2024-12-04
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog
      :visible="visible"
      title="选择人员（可多选）"
      :close-on-click-modal="false"
      width="50%"
      top="100px"
      append-to-body
      center
      @close="handleClose"
    >
      <el-input placeholder="请输入人员名称" v-model="filterText" clearable></el-input>
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
        <span slot-scope="{node,data}">
          <span>{{ data.name }}</span>
          <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
        </span>
      </el-tree>
      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import { deepClone } from '@/utils';
import DyTable from '@/components/DyTable';
import { localstorageGet } from '@/utils/auth';

export default {
  name: '',
  components: {DyTable},
  model: {
    prop: 'myValue', // value
    // event: 'changeMyValue' // input
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    myValue: { // value
      type: [String], // [Array, String, Number]
      default() {
        return '';
      }
    },
    //每次选择的数量，如果有数量，只能选择人员，不能选择公司，超过数量则置灰
    selectLimit:{
      type:String,
      default:''
    },
    // defaultCheckedKeys:{
    //   type:Array,
    //   default:()=>{
    //     return []
    //   }
    // },
    flag:{
      type:String,
      default:'3'
    }
  },
  data() {
    const _this = this;
    return {
      // 人员选择
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
      selectList: [],
      defaultCheckedKeys: [],

      // 通用数据
      viewName:'',
    };
  },
  computed: {
  },
  watch: {
    filterText(val) {
      if (this.$refs.personSelectTree) {
        this.$refs.personSelectTree.filter(val);
      }
    },
    visible(val){
      if (val) {
        this.$nextTick(x=>{
          let getMyNewVal = JSON.parse(this.myValue);
          if (getMyNewVal.flowList) {
            console.log('getMyNewVal.flowList',getMyNewVal.flowList)
            let selectTableIdList = getMyNewVal.flowList.map(x=>x.id);
            this.defaultCheckedKeys = selectTableIdList;
          }
        })
      }
    },
    async myValue(newVal, oldVal) {
      console.log('w====atch',newVal)
    },
  },
  created() { 
    this.getCompanyTree();
  },
  mounted() {
  },
  methods: {
    getCompanyTree() { // 获取公司部门架构数据
      console.log('getCompanyTree')
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: this.flag,
            id: localstorageGet('companyId') // 公司id
            // id: this.company || localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            // this.treeData = res.data;
            let data = res.data
            //flag 2过滤掉子公司部门和集团部门 flag3显示人员
            if(this.flag == 2){
              data.forEach(item=>{
                if(item.childrenList && item.childrenList.length){
                  item.childrenList.forEach(el=>{
                    if(el.childrenList)delete el.childrenList
                  })
                }
              })
              let childrenList = deepClone(data[0].childrenList)
              data[0].childrenList = []
              let topCompany = deepClone(data[0])
              // topCompany = delete topCompany.childrenList
              data[0].childrenList.push(topCompany)
              childrenList.forEach(item=>{
                if(item.type == 1){
                  data[0].childrenList.push(item)
                }
              })
              data[0].name = '全选'
            }
            this.$set(this, 'treeData', deepClone(data));
            this.defaultFirstLevelId = [data[0].id];
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
      return (data.name.indexOf(value) !== -1 && data.type == '5');
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    confirm() {
      const checkNode = this.$refs.personSelectTree.getCheckedNodes();
      if(this.flag == 3){
        var personList = checkNode.filter(x => x.type == 5);
        // 对人员进行去重处理
        const uniquePersonList = [];
        const personIds = new Set();
        personList.forEach(person => {
          if (!personIds.has(person.id)) {
            personIds.add(person.id);
            uniquePersonList.push(person);
          }
        });
        // this.$emit('select', personList);
        this.$emit('selectFlow', JSON.stringify({
          flowList: uniquePersonList,
        }));
      }else{
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

  }
};

</script>
<style lang='scss' scoped>
::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
