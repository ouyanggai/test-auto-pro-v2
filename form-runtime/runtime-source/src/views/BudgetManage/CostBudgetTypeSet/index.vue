<template>
  <div class="container">
    <div class="search">
      <el-date-picker v-model="annual" type="year" value-format="yyyy" style="width: 120px;margin-right:8px" @change="dateChange">
      </el-date-picker>
      <el-select v-model="company" placeholder="公司" style="width:240px;margin-right:8px">
        <el-option v-for="item in options" :key="item" :label="item" :value="item">
        </el-option>
      </el-select>
      <el-input style="width:220px;margin-right:8px" v-model.trim="searchName" placeholder="输入关键字查询">
        <i slot="suffix" style="cursor:pointer" @click="search" class="el-input__icon el-icon-search"></i>
      </el-input>
      <!-- <el-button type="primary" style="margin-right: 8px;">查询</el-button> -->
      <!-- <el-link type="primary">导入费用预算类型</el-link> -->
    </div>
    <div class="content">
      <el-tree
        v-if="treeData.length"
        :data="treeData"
        :props="defaultProps"
        default-expand-all
        node-key="id"
        :filter-node-method="filterNode"
        ref="tree">
        <span class="custom-tree-node" slot-scope="{ node, data }" >
          <span >{{ node.label }} </span>
        </span>
      </el-tree>
    </div>
  </div>
</template>

<script>
import {localstorageGet} from '@/utils/auth'
import Api from '@/api';
export default {
  name:'CostBudgetTypeSet',
  components: {},
  props: [],
  data() {
    return {
      title:'添加归口',
      visible:false,
      annual:new Date().getFullYear()+'',
      form:{
        type:''
      },
      formRules:{
        type:[
          {required:true,message:'请输入归口名称'}
        ]
      },
      options:[
        localstorageGet('companyName')
      ],
      company:localstorageGet('companyName'),
      companyId:localstorageGet('companyId'),
      searchName:'',
      defaultProps:{
        children: 'children',
        label: (data, node)=>{
          if(data.type == 'company'){
            if(data.departmentName == '公司领导'){
              return '公司固定费用'
            }else{
              return data.departmentName
            }
          }else{
            return data.name
          }
        }
      },
      treeData:[]
    };
  },
  created() {
    // this.getBudgetData()
    this.getDepartmentList()
  },
  mounted() {},
  watch: {
    searchName(val){
      this.$refs.tree.filter(val);
    }
  },
  computed: {},
  methods: {
    dateChange(){
      this.getDepartmentList()
    },
    getDepartmentList() {
      this.$axios.post(
        Api.budgetManage.getCompanyDeptVoByCompanyId,
        {
          data: {
            id: this.companyId
          }
        },
        res => {
          if (res.isSuccess) {
            let list = []
            if (res.data && res.data.departmentVos) {
              list = res.data.departmentVos.map(item => {
                return {
                  id: item.id,
                  name: item.departmentName,
                  children:[]
                }
              })
            }
            this.departOptions = list
            this.getBudgetData()
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getBudgetData(y) {
      const param = {
        // status: '1',
        stringList: [
          '1','2'
        ],
        annually: this.annual,
        departmentId:'',
        projectId: ''
      };
      this.$axios.post(
        Api.budgetManage.getBudgetList,
        {
          data: param
        },
        res => {
          if (res.isSuccess) {
            let data = res.data?.dataList || []
            data.forEach(item=>{
              let departmentId = item.departmentId
              let find = this.departOptions.find(el=>el.id == departmentId)
              if(find){
                find.children.push(item)
              }
            })
            this.treeData = this.departOptions
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getBudgetTypeOfGroup() {
      this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            let data = res.data || []
            data.forEach(item=>{
              let companyId = this.companyId
              item.centralizedApiVos[0].deptBudgetCentralizedVoList.forEach(el=>{
                el.sysDepartmentVo.companyId = companyId
              })
            })
            let find = data.find(item=>item.companyVo.id == this.companyId)
            if(find){
              this.centralizedApiVos = find.centralizedApiVos[0]
              this.generateTree()
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree(){
      let {deptBudgetCentralizedVoList} = this.centralizedApiVos
      let treeData = []
      deptBudgetCentralizedVoList.forEach(item=>{
        let {sysDepartmentVo,budgetCentralizedVoList} = item
        sysDepartmentVo.children = []
        // this.$set(sysDepartmentVo,'children',[])
        if(budgetCentralizedVoList && budgetCentralizedVoList.length){
          sysDepartmentVo.children = budgetCentralizedVoList
        }
        treeData.push(sysDepartmentVo)
      })
      this.treeData = treeData
    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    search(){

    }
  },
};
</script>
<style lang="scss" scoped>
  .container{
    height: 100%;
    padding: 14px;
    background: #fff;
    .content{
      margin-top: 20px;
      max-width: 600px;
    }
  }

  ::v-deep{
    .el-tree{
      height: 60vh;
    }
    .custom-tree-node{
      display: block;
      position: relative;
      min-width: 180px;
      .oparation-icon{
        position: absolute;
        right: 0;
        top:0;
        height: 100%;
      }
    }
  }
</style>
