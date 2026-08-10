<template>
  <el-dialog :visible="visible" title="选择指标" :close-on-click-modal="false"
              width="1000px" @close='handleClose' @opened="fetchData"
              append-to-body>
      <el-radio-group v-model="typeRadio">
        <el-radio-button label="work_target" >工作指标</el-radio-button>
        <el-radio-button label="manager_target" >管理指标</el-radio-button>
      </el-radio-group>
      <div style="margin-top: 15px;">
        <!-- <el-radio-group v-model="flowRadio">
          <el-radio v-for="item in flowList" :label="item.id" style="margin: 5px 8px; ">{{ item.name }}</el-radio>
        </el-radio-group> -->
        <dy-table :showRadio="true" :keys="colKey" :fetchData="()=>{}" :list="tableData" style="padding:0px;" ref="usingTable" :isPagination="false">
        </dy-table>
      </div>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="confirm">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
export default {
  name:'',
  components: {DyTable},
  props: ['visible','currentRelativId','relateRow'],
  data() {
    return {
      typeRadio:'work_target', //manager_target
      year:new Date().getFullYear(),
      flowRadio:'',
      flowList:[],
      tableData:[],
      colKey:{
        indicatorsTypeName: {
          label:'目标项目',
          showTooltip:true
        },
        content:{
          label:'具体目标项目内容',
          showTooltip:true
        },
        weight:'权重',
        assessmentMethod:{
          label:'目标完成标准（考核标准）',
          showTooltip:true
        },
        assessmentTime:{
          label:'目标完成时间节点',
          showTooltip:true
        },
      }
    };
  },
  created() {},
  mounted() {},
  watch: {
    typeRadio(){
      this.tableData = []
      this.fetchData()
    },
    currentRelativId(val){
        this.$nextTick(()=>{
          this.$refs.usingTable.currentRowId = val
        })
    }
  },
  computed: {},
  methods: {
    fetchData() {
      var currentMonth = this.$parent.dateForm.currentMonth;
      var targetTime = new Date(currentMonth).getFullYear();
      const param = {
        data: {
          manageType: this.typeRadio,
          targetTime // :this.year,
          // kpiGroupStatus:this.status
        }
      };
      if(this.status)param.data.kpiGroupStatus = this.status
      this.$axios.post(
        Api.performance.getKpiGroupList,
        param,
        res => {
          if (res.isSuccess) {
            let list = res.data || []
            list.forEach(item=>{
              if(item.kpiGroupStatus == 'pass')this.getDetail(item.id)
              // if(item.kpiGroupStatus == ''){

              // }
            })
            // this.tableData = res.data ? res.data : [];
            this.$refs.usingTable.currentRowId = this.relateRow?.yearKpi?.id || ''
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getDetail(id){
      return this.$axios.post(
        Api.performance.getWorkTargetDetail, // 放开
        {
          data: {
            id
          }
        },
        res => {
          var data = res.data;
          let keyPerformanceIndicatorsList = data.keyPerformanceIndicatorsList
          // console.log('keyPerformanceIndicatorsList',keyPerformanceIndicatorsList)
          keyPerformanceIndicatorsList.forEach(item=>{
            item.indicatorsTypeName = item.indicatorsType.name
          })
          this.tableData = keyPerformanceIndicatorsList
        }
      );
    },
    confirm(){
      let flowRadio = this.$refs.usingTable.currentRowId
      if(!flowRadio){
        return this.$message.error('请选择关联指标')
      }
      let find = this.tableData.find(item=>item.id == flowRadio)
      if(!find){
        return this.$message.error('请选择关联指标')
      }
      this.$emit('comfirmRelate',find)
    },
    handleClose(){
      this.tableData = []
      this.$refs.usingTable.currentRowId = ''
      this.$emit('update:visible',false)
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
