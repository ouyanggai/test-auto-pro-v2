<!--
 * @Descripttion:集团公司或部门多选与单选弹窗
 * @Author: zhengzetao
 * @Date: 2024-12-18
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible"
      :title="fieldType+'公司或部门'" width="50%"
      top="100px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog"
      @close='handleClose'>
      <el-input placeholder="请输入公司或部门名称" v-model="filterText" clearable></el-input>
      <div class="dialog-container">
        <el-tree :data="treeData" :props="defaultProps" show-checkbox :check-strictly="true" :default-expand-all="true" node-key="id"
        :indent="10" :filter-node-method="filterNode" @check-change="handleCheckChange" ref="companyTree">
        </el-tree>
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirmTaskHeader">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import { localstorageGet } from '@/utils/auth';
import { getObjById } from '@/utils';
import { parseJsonArray } from '@/utils/parse-value';

export default {
  name: '',
  components: {},
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
    fieldSelectType: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      typeList:{
        'mulSelect':{
          name:'多选'
        },
        'singleSelect':{
          name:'单选'
        },
      },
      filterText:'',
      viewName:'',
      treeData: [],
      chooseHeaderRadio: '',
      defaultProps: {
        children: 'childrenList',
        label(data) {
          // console.log('data.name',data.name)
          return data.name;
        }
      }
    };
  },
  computed: {
    fieldType(){
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
      var str = '';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        if (result) {
          str = this.typeList[item]['name'];
          // str = item;
          break;
        }
      }
      // console.log('str',str)
      return str ? str : '选择'
    }
  },
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.$refs.companyTree.filter(val);
      }
    },
    visible(val){
      if (val) {
        this.$nextTick(x=>{
          const getMyNewVal = parseJsonArray(this.myValue);
          // if (getMyNewVal.flowList) {
          if (getMyNewVal) {
            // console.log('getMyNewVal.flowList',getMyNewVal.flowList)
            console.log('this.$refs.companyTree',this.$refs.companyTree)
            console.log('getMyNewVal',getMyNewVal)
            this.$refs.companyTree.setCheckedNodes(getMyNewVal);
            // let selectTableIdList = getMyNewVal.flowList.map(x=>x.id);
            // this.defaultCheckedKeys = selectTableIdList;
          }
        })
      }
    },
  },
  created() { 
    this.init();
  },
  mounted() {
    // console.log('this.myValue',this.myValue)
  },
  methods: {
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    async init() {
      console.log('init')
      if (!this.treeData.length) {
        this.treeData = await this.getCompanyTree();
      }
    },
    getCompanyTree() { // 获取公司部门架构数据
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.taskManage.taskArrange.getCompanyDepartTree,
          {
            data: {
              // flag: this.typeList[this.fieldType]['flag'],
              flag: 2, // 返回公司和部门
              id: localstorageGet('companyId') // 公司id
            }
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
              this.$message.error(res.message);
              
            }
          }
        );
      })
    },
    // 树节点选择前判断
    handleCheckChange(data, checked, indeterminate) {
      // console.log('handleCheckChange',data)
      if (!this.canCheckNode(data)) {
        // 确保DOM已经更新
        this.$nextTick(() => {
          this.$refs.companyTree.setChecked(data.id, false);
        });
      }
    },
    canCheckNode(node) {
      let checkedNodes = this.$refs.companyTree.getCheckedNodes();
      let flag = true;
      if (this.fieldType == '单选' && checkedNodes.length >1){
        flag = false;
        this.$message.error('只能单选')
      }
      return flag;
    },
    confirmTaskHeader() {
      console.log('confirmTaskHeader')
      // console.log('1',this.$refs.companyTree.getCheckedNodes())
      let checkedNodes = this.$refs.companyTree.getCheckedNodes();
      var selectList = checkedNodes.map(x => ({
        id:x.id,
        name:x.name
      }));
      this.$emit('selectHeader', JSON.stringify(selectList));
      // this.viewName = this.selectRadio.name
      this.handleClose();
    },
    getTreeCurrentNode(name, data) { // 后端要求改为名称查询，不用id。表单内保存的是名称
      var hasFound = false; // 表示是否有找到id值
      var result = null;
      function fn(data) {
        if (Array.isArray(data) && !hasFound) { // 判断是否是数组并且没有的情况下，
          data.forEach(item => {
            if (item.name === name) { // 数据循环每个子项，并且判断子项下边是否有id值
              result = item; // 返回的结果等于每一项
              hasFound = true; // 并且找到id值
            } else if (item.childrenList) {
              fn(item.childrenList); // 递归调用下边的子项
            }
          });
        }
      }
      fn(data);
      return result;
    }
  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  .dialog-container {
    // height: 600px;
    height: 48vh;
    overflow-y: auto;
  }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}
</style>
