<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-27 16:41:51
-->
<!-- 费用预算类型选择 -->

<template>
  <!-- <selectExpenseBudget v-model="projectId">
    <template v-slot="scope">
      <el-input readonly v-model="scope.viewName" style="width:100%"></el-input>
    </template>
  </selectExpenseBudget> -->
  <div>
    <!-- <el-cascader v-model="infoForm.expenseBudgetList[0]['allChildId']" clearable -->
    <el-cascader v-model="currentInfoObj" clearable
      @change="(value) => handleChange(value)" :props="props"  :options="departmentList"
      :disabled="!departmentList.length" style="width:100%;" filterable>
      <template slot-scope="{ node, data }">
        <span>{{ data.name }}</span>
      </template>
    </el-cascader>
  </div>
</template>

<script>
// import selectExpenseBudget from './selectExpenseBudget';
import Api from '@/api';
import {
  localstorageGet, localstorageSet
} from '@/utils/auth';

/* eslint-disable */
export default {
  name: 'ExpenseBudgetType',
  components: {},
  props: {
    value:{
      type:[Array],
      default(){
        return [];
      }
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      currentInfoObj: this.value,
      companyList:[],
      props: {
        value: 'id',
        label: 'name',
        children: 'childrenList',
      },
      departmentList: [],
      // companyNumber:'',
      // infoForm:{
      //   expenseBudgetList: [
      //     {
      //       companyNumber:'',
      //       allChildId: [],
      //       money: 0,
      //       remark:''
      //     }
      //   ],
      // },
      // viewData: {
      //   viewName: ''
      // },
    };
  },
  watch: {
    value(val) {
      console.log('val123',val)
      this.currentInfoObj = val
      // if (val.length){
      //   console.log(111)

      //   this.getCompanyNumber().then(r=>{
      //     if(r.isSuccess){
      //       this.companyNumberList = r.data?.dataList || []
      //       let currentCompanyId = localstorageGet('companyId')
      //       let find = this.companyNumberList.find(item=>item.companyId == currentCompanyId)
      //       if(find){
      //         this.companyNumber = find.number
      //       }
      //     }
      //     // this.getCompanyList();
      //     console.log('编辑与查看')
      //     this.getEchDetailData().then(() => {
      //       this.getDepartmentList();
      //       console.log('--------this.infoForm.expenseBudgetList-----------',this.infoForm.expenseBudgetList)
      //     });

      //     // if (this.operaType == 'add') {
      //     //   this.getPermisionForAdd();
      //     // }
      //     // 编辑和查看时才需用到
      //     // if (this.operaType == 'check' || this.operaType == 'edit' || this.operaType == 'examine') {
      //     //   this.getEchDetailData().then(() => {
      //     //     this.getDepartmentList();
      //     //     this.getProjectList();
      //     //     if (this.operaType == 'edit') {
      //     //       this.getPermisionForEdit();
      //     //     }
      //     //   });
      //     // }
      //   })
      // }
      // this.currentId = val
      // this.dataModel = val
    },
    currentInfoObj(val){
      console.log('=======1',val)
      this.$emit('input', val)
    },
  },
  computed: {

  },
  created() {
    // this.getCompanyNumber().then(r=>{
    //   if(r.isSuccess){
    //     this.companyNumberList = r.data?.dataList || []
    //     let currentCompanyId = localstorageGet('companyId')
    //     let find = this.companyNumberList.find(item=>item.companyId == currentCompanyId)
    //     if(find){
    //       this.companyNumber = find.number
    //     }
    //   }
      this.getCompanyList();
    // })
  },
  mounted() { },
  methods: {
    handleChange(value, index) {
      console.log('handleChange')
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
        // // 测试用
        // this.$emit('input', allChildId)
        // this.currentInfoObj = allChildId;

        // this.viewData.viewName = '1111';
      })
    },
    async selectCompany() {
      console.log('selectCompany')
      // this.infoForm.expenseBudgetList = [
      //   {
      //     companyNumber:this.companyNumber,
      //     allChildId: [],
      //     money: 0
      //   }
      // ];
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
              // this.infoForm.companyId = res.data.find(x => x.flag == 'mainDutyCompany').id;
              this.selectCompany();
            // }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取表单详情数据
    // async getEchDetailData() {
    //   console.log('getEchDetailData')
    //   return new Promise((resolve, reject) => {
    //     this.$axios.post(
    //       Api.budgetManage.getEchDetailData,
    //       {
    //         data: {
    //           id: '3585e86dfb034c8b95e849e1c9df850f' // 暂时写死
    //           // id: this.id
    //         }
    //       },
    //       async res => {
    //         if (res.isSuccess) {
    //           // this.originFileList = [];
    //           this.companyNumber = res.data?.companyNumber || this.companyNumber
    //           let expenseBudgetList = res?.data?.expenseBudgetList || []
    //           expenseBudgetList.forEach(x => {
    //             x.allChildId = x.allChildId.split(',');
    //           });
    //           expenseBudgetList.forEach(el=>{
    //             el.companyNumber = this.companyNumber
    //           })

    //           this.$set(res.data,'expenseBudgetList',expenseBudgetList)
    //           // console.log('expenseBudgetList',expenseBudgetList)
    //           console.log('表单详情数据-res.data',res.data)
    //           this.infoForm = res.data;
    //           resolve();
    //           // console.log('this.infoForm', this.infoForm);
    //         } else {
    //           this.$message.error(res.message);
    //         }
    //       }
    //     );
    //   });
    // },
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
            // console.log('res.data',res.data)
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
            // console.log('newData',newData)
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
        this.getBudgetTypeOfGroup('8fe922a5da21445a8a26aba74d0af5e1').then(res => { // 暂时写死了公司id
          // console.log('res',res)
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

  },
};
</script>
<style lang="scss" scoped></style>
