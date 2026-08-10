<!--
 * @Descripttion:集团人员、公司、部门、岗位选择弹窗
 * @Author: zhengzetao
 * @Date: 2024-7-8
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible"
      :title="'选择'+ typeList[fieldType]['name']" width="50%"
      top="100px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog"
      @close='handleClose'>
      <el-input :placeholder="'请输入'+typeList[fieldType]['name']+'名称'" v-model="filterText" clearable></el-input>
      <div class="dialog-container">
        <el-tree :data="treeData" :props="defaultProps" :default-expand-all="fieldType == 'userName'? true : false" :indent="10" node-key="id"
        :default-expanded-keys="defaultFirstLevelId" :filter-node-method="filterNode" ref="companyTree">
          <span slot-scope="{node,data}">
            <template>
              <el-radio v-model="chooseHeaderRadio" @input="()=>{radioChange(node,data)}" :label="data.id" v-if="(fieldType == 'companyName') || ((fieldType == 'depName' && data.type == 2)||(fieldType == 'depName' && isSelectCompany)) || (fieldType == 'dutyName' && data.type == 4) || (fieldType == 'userName' && data.type == 5)"><span></span></el-radio>
              <span>{{ data.name }}</span>
            </template>
          </span>
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
    // myValue: { // value
    //   type: [Object], // [Array, String, Number]
    //   default() {
    //     return {};
    //   }
    // },
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
      typeList: {
        'userName': {
          name:'人员',
          flag: 3 // flag为后端接口查询参数
        },
        'companyName': {
          name:'公司',
          flag: 7 // flag为后端接口查询参数
        },
        'depName': {
          name:'部门',
          flag: 2 // flag为后端接口查询参数
        },
        'dutyName': {
          name:'岗位',
          flag: 4 // flag为后端接口查询参数
        },

        // 下面这几个多余的是为了兼容生产已有的数据（后续不使用这些字段）
        // 'handledBy': {
        //   name:'人员',
        //   flag: 3 // flag为后端接口查询参数
        // },
        // 'handlingCompany': {
        //   name:'公司',
        //   flag: 7 // flag为后端接口查询参数
        // },
        // 'handlingDepartment': {
        //   name:'部门',
        //   flag: 2 // flag为后端接口查询参数
        // },
        // 'handlingDuty': {
        //   name:'岗位',
        //   flag: 4 // flag为后端接口查询参数
        // },
      },
      filterText:'',
      viewName:'',
      selectRadio: null,
      treeData: [],
      chooseHeaderRadio: '',
      defaultProps: {
        children: 'childrenList',
        label(data) {
          console.log('data.name',data.name)
          return data.name;
        }
      },
      isSelectCompany:false,
      defaultFirstLevelId: [],
    };
  },
  computed: {
    fieldType(){
      console.log('this.fieldSelectType',this.fieldSelectType)
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
      if (this.fieldSelectType == 'handledBy'){
        copyType = 'userName';
      } else if (this.fieldSelectType == 'handlingCompany'){
        copyType = 'companyName';
      } else if (this.fieldSelectType == 'handlingDepartment'){
        copyType = 'depName';
      } else if (this.fieldSelectType == 'handlingDuty'){
        copyType = 'dutyName';
      } else if (this.fieldSelectType == 'handlingCompanyAndDepartment'){
        copyType = 'depName';
        this.isSelectCompany = true;
      }
      console.log('copyType****',copyType)
      var str = '';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        // var result = new RegExp(item,'i').test(this.fieldSelectType);
        if (result) {
          str = item;
          break;
        }
      }
      // console.log('str',str)
      return str
    }
  },
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.$refs.companyTree.filter(val);
      }
    },
    async myValue(newVal, oldVal) {
      // console.log('w====atch',newVal)
      // console.log(typeof newVal)

      if (!this.treeData.length) {
        this.treeData = await this.getCompanyTree();
        console.log('this.treeData1',this.treeData)
        this.defaultFirstLevelId = [this.treeData[0].id];
      }

      // 添加JSON判断是为了兼容生产已有数据（生产只存了id）
      let getMyNewVal = newVal == '' ? newVal : this.isJSON(newVal) ? JSON.parse(newVal) : newVal;
      let JsonMyValue = this.isJSON(newVal) ? JSON.parse(newVal) : getMyNewVal;
      let obj = getObjById(this.treeData,JsonMyValue?.id || JsonMyValue)
      // console.log('init-obj',obj)
      this.viewName = obj?.name || JsonMyValue;
      this.chooseHeaderRadio = JsonMyValue?.id || JsonMyValue;
    },
  },
  created() {

  },
  mounted() {
    this.init();
    // console.log('this.myValue',this.myValue)
  },
  methods: {
//     // 是否JSON
    isJSON(str) {
      try {
        const result = JSON.parse(str);
        // JSON 格式必须是一个对象或数组
        return typeof result === 'object' && result !== null;
      } catch (e) {
        return false;
      }
    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
      // return (data.name.indexOf(value) !== -1 && data.type == this.dataType);
    },
    radioChange(node,data) {
      this.selectRadio = getObjById([data],data.id,'childrenList')
      console.log('this.selectRadio',this.selectRadio)
    },
    handleClose() {
      this.filterText = '';
      this.$emit('update:visible', false);
    },
    async init() {
      console.log('init')
      // console.log('JsonMyValue',this.isJSON(this.myValue))
      if (!this.treeData.length) {
        this.treeData = await this.getCompanyTree();
        this.defaultFirstLevelId = [this.treeData[0].id];
      }
      // 添加JSON判断是为了兼容生产已有数据（生产只存了id）
      let getMyNewVal = this.myValue == '' ? this.myValue : this.isJSON(this.myValue) ? JSON.parse(this.myValue) : this.myValue;
      let JsonMyValue = this.isJSON(this.myValue) ? JSON.parse(this.myValue) : getMyNewVal;
      let obj = getObjById(this.treeData,JsonMyValue?.id || JsonMyValue)
      // console.log('init-obj',obj)
      // console.log('init-JsonMyValue',JsonMyValue)
      //为了避免这个组件设为必填时，写入空数据导致触发校验，先做判断，再给值
      let viewName = obj?.name || JsonMyValue.name
      if(viewName !== undefined)this.viewName = viewName;
      this.chooseHeaderRadio = JsonMyValue?.id || JsonMyValue;
      // this.viewName = obj?.name || JsonMyValue;
      // this.chooseHeaderRadio = JsonMyValue?.id || JsonMyValue;
    },
    getCompanyTree() { // 获取公司部门架构数据
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.taskManage.taskArrange.getCompanyDepartTree,
          {
            data: {
              flag: this.typeList[this.fieldType]['flag'],
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
    confirmTaskHeader() {
      // const choseObj = this.getTreeCurrentNode(this.chooseHeaderRadio, this.treeData);
      // if (choseObj) {
      //   this.$emit('selectHeader', choseObj, this.selectRadio);
      // }
      if (this.selectRadio) {
        console.log(this.selectRadio,'++++++++++++')
        // treeData
        let arr = []
        let currentCompany ={}
        if(this.selectRadio.parentId&&this.selectRadio.type=='2'){
          arr = this.findParents(this.treeData, this.selectRadio.id)
           const list = arr.filter(i=>i.type==1)
           currentCompany = list[list.length-1]
        }
        console.log(arr,'arr',currentCompany)
        this.$emit('selectHeader', JSON.stringify({
          id:this.selectRadio.id,
          name:this.selectRadio.name,
          type:this.selectRadio.type,
          companyId:currentCompany.id||'',
          parentId:this.selectRadio.parentId,
        }));
      }
      this.viewName = this.selectRadio.name
      this.handleClose();
    },
    findParents(tree, targetId, parents = []) {
      for (let node of tree) {
        // 如果当前节点是目标节点，返回当前的父级列表
        if (node.id === targetId) {
          return parents;
        }

        // 如果当前节点有子节点，递归查找
        if (node.childrenList && node.childrenList.length > 0) {
          const result = this.findParents(node.childrenList, targetId, [...parents, node]);
          if (result) {
            return result;
          }
        }
      }
      // 如果没有找到目标节点，返回 null
      return null;
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
::v-deep {
  .el-dialog__body {
    padding: 4px 20px !important;
  }
}
</style>
