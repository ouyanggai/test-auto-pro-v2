<template>
  <el-dialog :visible="visible" title="月度绩效历史" :close-on-click-modal="false" width="100%" @close='handleClose' @open="()=>fetchData('first')" append-to-body :fullscreen="true">
    <div>
      <el-tabs v-model="activeName" type="card" @tab-click="handleClick">
        <el-tab-pane :label="val+'月'" :name="String(val)" v-for="val in endMonth"></el-tab-pane>
      </el-tabs>
    </div>
    <div>
      <!-- <monthlyPerf :bizId="id" :actionType="actionType"></monthlyPerf> -->
      <template v-if="queryId" >
        <monthlyPerf :bizId="queryId" actionType="preview" :isHistoryDialog="true"></monthlyPerf>
      </template>
      <el-empty description="暂无数据" v-else :key="2"></el-empty>
    </div>
    <!-- <span slot="footer">
      <el-button @click="handleClose">关 闭</el-button>
    </span> -->
  </el-dialog>
</template>
<script>
import Api from '@/api';
// import monthlyPerf from '@/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue'
// import moment from 'moment'
// let currentYear = moment().format('YYYY')
// let currentMonth = moment().format('M')
// let activeName = currentMonth; // String(currentMonth-1) // String(currentMonth != 1 ? currentMonth-1 : 1)
export default {
  name: 'monthHistoryDialog',
  components: {monthlyPerf:()=>import('@/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue')},
  props: ['visible', 'initiatorId'],
  data() {
    return {
      currentYear: '',
      // currentMonth: moment().format('M'),
      endMonth: 12, // currentMonth-1, // currentMonth != 1 ? currentMonth-1 : 1,
      activeName: '1', // this.$parent.dateForm.currentMonth.slice(5, 7),
      queryType:'month',
      queryId:'',
    };
  },
  created() { },
  mounted() { },
  watch: {},
  computed: {},
  methods: {
    handleClick(){
      if(this.setTime) clearTimeout(this.setTime)
      this.setTime = setTimeout(()=>{
        this.fetchData()
      },200)
    },
    fetchData(type) {
      if (type === 'first') {
        this.currentYear = this.$parent.dateForm.currentMonth.slice(0, 4);
        this.activeName = this.$parent.dateForm.currentMonth.slice(5, 7);
      }
      let targetTime = `${this.currentYear}${this.activeName}`
      const param = {
        data: {
          user: { id: this.initiatorId || this.$store.state.user.userId },
          manageType: 'work_and_manager_target',
          targetTime
        }
      };
      // console.log('param',param)
      this.$axios.post(
        Api.performance.getKpiGroupList,
        param,
        res => {
          if (res.isSuccess) {
            const tableData = res.data ? res.data : [];
            if(tableData.length){
              this.queryId = tableData[0].id
            }else{
              this.queryId = ''
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleClose(){
      this.$emit('update:visible',false)
    }
  },
};
</script>
<style lang="scss" scoped>
::v-deep .el-dialog.is-fullscreen .el-dialog__body{
  max-height: initial !important;
  min-height: initial !important;
}
::v-deep .el-dialog.is-fullscreen .el-dialog__footer{
  text-align: center;
}
</style>
