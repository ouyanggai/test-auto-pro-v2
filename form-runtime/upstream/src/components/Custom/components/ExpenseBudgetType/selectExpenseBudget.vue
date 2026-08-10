<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-15 15:07:55
-->
<template>
  <div>
    <slot slot="reference" :viewName='viewData.viewName'></slot>
    <el-cascader v-model="infoForm.expenseBudgetList[0]['allChildId']" clearable
      @change="(value) => handleChange(value)" :props="props"  :options="departmentList"
      :disabled="!departmentList.length" style="width:100%;" filterable>
      <template slot-scope="{ node, data }">
        <span>{{ data.name }}</span>
        <span v-if="data.projectName" style="color:rgb(145,145,145);font-size:12px;margin-left:5px;">项目:{{ data.projectName }}</span>
      </template>
    </el-cascader>
  </div>
  <!-- <el-popover
    placement="bottom"
    :disabled="disabled"
    v-model="visibletest">
    <el-input clearable size="mini" placeholder="输入项目名称搜索" suffix-icon="el-icon-search" v-model.trim="filterText"
    style="margin-bottom:5px;display:flow-root;"></el-input>
    <div style="display:inline-block;width:350px;vertical-align: top;border-right:1px solid #c4d1dd;
    max-height:500px;overflow: scroll;">
      <el-tree
      ref="myTree"
      node-key="id"
      :data="treedata"
      :highlight-current="true"
      :expand-on-click-node="false"
      :props="defaultProps"
      default-expand-all
      @node-click="handleNodeClick">
      </el-tree>
    </div>
    <div style="display:inline-block;width:350px;border-left:1px solid #c4d1dd;transform: translate(-1px,0);
    max-height:500px;overflow: scroll;
    ">
      <ul>
        <template v-for="i in selectTreeNode.projectList">
        <li class="clearfix liHover"  :key="i.id" @click="handleLiClick(i)" v-if="i.showItem"
        style="padding:2.5px;cursor:pointer;line-height:26px" :style="{backgroundColor:selectItem == i.id ? '#edf6ff' : 'white'}">
           <span style="float:left;padding-left:4px" :title="i.name" class="rightProjectName">{{ i.name}}</span>
          <span style="float:right;padding-right:4px">
            {{i.relationType == 'public_project' ? '集团公共' : '公司私有'}}
          </span>
        </li>
      </template>
      </ul>
    </div>
    <slot slot="reference" :viewName='viewData.viewName'></slot>
  </el-popover> -->
</template>

<script>
import Api from '@/api';
import {
  localstorageGet, localstorageSet
} from '@/utils/auth';

export default {
  name: '',
  model: {
    prop: 'myValue', // value
    event: 'changeMyValue' // input
  },
  components: {},
  props: {
    myValue: { // value
      type: [String], // [Array, String, Number]
      default() {
        return '';
      }
    },
    disabled: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return false;
      }
    }
  },
  data () {
    return {
      props: {
        value: 'id',
        label: 'name',
        children: 'childrenList',
      },
      departmentList: [],
      companyNumber:'',
      infoForm:{
        expenseBudgetList: [
          {
            companyNumber:'',
            allChildId: [],
            money: 0,
            remark:''
          }
        ],
      },
      viewData: {
        viewName: ''
      },
      // filterText: '',
      // projectArr: [],
      // oldVlue: this.myValue,
      // treedata: [],
      // selectTreeNode: {},
      // selectItem: '',
      // visibletest: false,
      // defaultProps: {
      //   children: 'childrenList',
      //   label: 'name'
      // }
    };
  },
  computed: {
    // compMyValue() {
    //   return this.myValue;
    // }
  },
  watch: {
    // filterText(val) {
    //   this.handleFilter(val);
    // },
    // compMyValue(newVal, oldVal) {
    //   var fidNum = 0;
    //   const fn = (sr) => {
    //     sr.forEach(el => {
    //       var fid = el.projectList.find(i => i.id == newVal);
    //       if (fid) {
    //         this.$emit('selectChange', fid);
    //         this.viewData.viewName = fid.name;
    //         this.selectTreeNode = el;
    //         this.selectItem = fid.id;
    //         fidNum++;
    //       };
    //       ((el.childrenList) && (el.childrenList.length > 0)) && fn(el.childrenList);
    //     });
    //   };
    //   fn(this.treedata);
    //   if (!fidNum) {
    //     const first = this.treedata[0];
    //     this.selectTreeNode = first;
    //     this.$refs.myTree.setCurrentKey(first.id);
    //     this.viewData.viewName = '';
    //     this.selectItem = '';
    //   }
    // }
  },
  created() {
    // this.handleInitTree();
    this.getCompanyNumber().then(r=>{
      if(r.isSuccess){
        this.companyNumberList = r.data?.dataList || []
        let currentCompanyId = localstorageGet('companyId')
        let find = this.companyNumberList.find(item=>item.companyId == currentCompanyId)
        if(find){
          this.companyNumber = find.number
        }
      }
      this.getCompanyList();
      // if (this.operaType == 'add') {
      //   this.getPermisionForAdd();
      // }
      // // 编辑和查看时才需用到
      // if (this.operaType == 'check' || this.operaType == 'edit' || this.operaType == 'examine') {
      //   this.getEchDetailData().then(() => {
      //     this.getDepartmentList();
      //     this.getProjectList();
      //     if (this.operaType == 'edit') {
      //       this.getPermisionForEdit();
      //     }
      //   });
      // }
    })
  },
  mounted() {

  },
  methods: {
    async selectCompany() {
      this.infoForm.expenseBudgetList = [
        {
          companyNumber:this.companyNumber,
          allChildId: [],
          money: 0
        }
      ];
      // this.infoForm.projectId = '';
      this.departmentList = [];
      this.getDepartmentList();
    },
    getCompanyNumber(){
      return this.$axios.post(Api.budgetManage.getCompanyNumberList,{})
    },
    // 获取公司列表
    getCompanyList() {
      console.log('获取公司列表')
      this.$axios.post(
        Api.budgetManage.getParentCompanyList,
        {
          data: {
            id: this.$store.state.user.companyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.companyList = res.data;
            console.log('this.companyList',this.companyList)
            // if (this.operaType == 'add') { // 新增的时候公司默认选择主岗公司
              this.infoForm.companyId = res.data.find(x => x.flag == 'mainDutyCompany').id;
              this.selectCompany();
            // }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取表单详情数据
    async getEchDetailData() {
      console.log('getEchDetailData')
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.budgetManage.getEchDetailData,
          {
            data: {
              id: this.id
            }
          },
          async res => {
            if (res.isSuccess) {
              // this.originFileList = [];
              this.companyNumber = res.data?.companyNumber || this.companyNumber
              let expenseBudgetList = res?.data?.expenseBudgetList || []
              expenseBudgetList.forEach(x => {
                x.allChildId = x.allChildId.split(',');
              });
              console.log('expenseBudgetList',expenseBudgetList)
              // for (let i = 0; i < res.data.expenseDetailList.length; i++) {
              //   const x = res.data.expenseDetailList[i];
              //   const fileArr = await this.getAttachmentList(x.id);
              //   if (fileArr) {
              //     this.$set(x, 'uploadFileList', fileArr);
              //   } else {
              //     this.$set(x, 'uploadFileList', []);
              //   }
              //   this.originFileList.push(JSON.parse(JSON.stringify(fileArr)));
              // }
              expenseBudgetList.forEach(el=>{
                el.companyNumber = this.companyNumber
              })
              // res.data.expenseInAccountInfoList.sort((a,b)=>a.sort-b.sort)
              console.log('res.data',res.data)
              this.infoForm = res.data;
              resolve();
              // console.log('this.infoForm', this.infoForm);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 递归操作树
    getTreeData(data) {
      for (let i = 0; i < data.length; i++) {
        if (data[i].childrenList.length < 1) {
          data[i].childrenList = undefined;
        } else {
          this.getTreeData(data[i].childrenList);
        }
      }
      return data;
    },
    transformName(type){
      return {
        1:'(公司归口)',
        2:'(月度归口)',
        3:'(项目归口)',
      }[type]
    },
    // 获取费用预算归口
    getBudgetData(y) {
      console.log('getBudgetData')
      const param = {
        type:1,
        stringList: [
          '1','2'
        ],
        annually: new Date().getFullYear(),
        departmentId: y.id,
        //projectId: this.infoForm.projectId
      }
      if(y.isProject){
        param.type = 3
        param.projectId = y.id
        delete param.departmentId
      }
      this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: param
        },
        res => {
          if (res.isSuccess) {
            console.log('res.data',res.data)
            // return
            res.data.dataList.forEach(item=>{
              item.name = item.name + this.transformName(item.type)
              // item.projectName = ''
              // if(item.type == 3 && item.projectId){
              //   let projectId = item.projectId
              //   let project = this.projectList.find(el=>el.id == projectId)
              //   if(project)item.projectName = project.shortName || project.name
              // }
              // item.isProject = y?.isProject || false
              // if(item.isProject)item.projectId = y.id
            })
            const newData = this.getTreeData(res.data.dataList);
            console.log('newData',newData)
            this.$set(y,'childrenList',newData)
            // y.childrenList = newData;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取公司下的部门列表
    async getDepartmentList(node, resolve) {
      console.log('getDepartmentList')
      return new Promise((resolve, reject) => {
        this.getBudgetTypeOfGroup('8fe922a5da21445a8a26aba74d0af5e1').then(res => {
          console.log('res',res)
          this.departmentList = res
          this.departmentList.forEach(y => {
            this.getBudgetData(y);
          });
          resolve();
        })
      })
    },
    getBudgetTypeOfGroup(companyId) {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            const data = res.data || [];
            const find = data.find(item => item.companyVo.id == companyId);
            if (find) {
              this.centralizedApiVos = find.centralizedApiVos[0];
              this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
              let departmentList = this.generateDepartOption();
              // console.log('departmentList',departmentList)
              resolve(departmentList)
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
      })
    },
    generateDepartOption() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo } = item;
        departOptions.push({
          id:sysDepartmentVo.id,
          name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item=>{
        departOptions.push({
          id:item.projectVo.id,
          name:item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject:true
        })
      })
      return departOptions
    },
    handleChange(value, index) {
      // console.log('this.departmentList',this.departmentList)
      // console.log('scope.row.allChildId',this.infoForm.expenseBudgetList)
      this.departmentList.forEach(item=>{
        const childrenList = item?.childrenList || []
        childrenList.forEach(it=>{
          this.$set(it,'disabled',false)
        })
      })
      console.log('this.infoForm.expenseBudgetList',this.infoForm.expenseBudgetList)
      this.infoForm.expenseBudgetList.forEach(el=>{
        let allChildId = el?.allChildId || []
        let departmentId = allChildId[0]
        let childId = allChildId[1]
        let find = this.departmentList.find(item=>item.id == departmentId)
        // console.log('find',find)
        console.log('allChildId',allChildId)
        if(find && find.childrenList){
          let childFind = find.childrenList.find(item=>item.id == childId)
          console.log('childFind',childFind)
          if(childFind)this.$set(childFind,'disabled',true)
        }

        // this.viewData.viewName = '1111';
      })
    },
    // handleLiClick(val) {
    //   this.selectItem = val.id;
    //   this.$emit('changeMyValue', val.id);
    //   this.$emit('selectChange', val);
    //   this.viewData.viewName = val.name;
    //   this.visibletest = false;
    // },
    // handleNodeClick(val) {
    //   this.selectTreeNode = val;
    // },
    // handleInitTree() {
    //   this.$axios.post('/web/user/api/company/getCompanyTree', { },
    //     (res) => {
    //       if (res.isSuccess) {
    //         this.getAllProject(res.data || []);
    //       }
    //     }
    //   );
    // },
    // getAllProject(treeData) {
    //   var selectTreeId = '';
    //   function GetAllpromises() {
    //     const promises = [];
    //     const fn = (sr) => {
    //       sr.forEach(el => {
    //         promises.push(this.$axios.post('/web/project/api/getProjectVosOfCompanyAndGroup', { data: { companyId: el.id }},
    //           (res) => {
    //             if (res.isSuccess) {
    //               el.projectList = res.data || [];
    //               this.projectArr.push(el.projectList);
    //               var fid = el.projectList.find(i => i.id == this.myValue);
    //               if (fid) {
    //                 this.$emit('selectChange', fid);
    //                 this.viewData.viewName = fid.name;
    //                 this.selectTreeNode = el;
    //                 this.selectItem = fid.id;
    //                 selectTreeId = el.id;
    //               };
    //             }
    //           }
    //         ));
    //         ((el.childrenList) && (el.childrenList.length > 0)) && fn(el.childrenList);
    //       });
    //     };
    //     fn(treeData);
    //     return promises;
    //   }
    //   const getAllpromises = GetAllpromises.bind(this);
    //   Promise.all(getAllpromises()).then(res => {
    //     this.treedata = treeData;
    //     this.handleFilter('');
    //     this.$nextTick(() => {
    //       if (selectTreeId) {
    //         this.$refs.myTree.setCurrentKey(selectTreeId);
    //       } else {
    //         const first = treeData[0];
    //         this.selectTreeNode = first;
    //         this.$refs.myTree.setCurrentKey(first.id);
    //       }
    //     });
    //   }).catch(err => { console.log(err); });
    // },
    // handleFilter(val) {
    //   this.projectArr.forEach(j => {
    //     j.forEach(i => {
    //       if (i.name && i.name.includes(val)) {
    //         i.showItem = true;
    //       } else {
    //         i.showItem = false;
    //       }
    //     });
    //   });
    // }
  },
};

</script>
<style lang='scss' scoped>
.liHover:hover{
  background-color: #edf6ff !important;
}
.rightProjectName{
  text-overflow: ellipsis;
  white-space: nowrap;
  overflow: hidden;
  width: 78%;
}
</style>
