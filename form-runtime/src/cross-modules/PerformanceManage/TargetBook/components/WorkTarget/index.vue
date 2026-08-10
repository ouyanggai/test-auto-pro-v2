<!--
 * @description:工作指标
 * @Author: Calvin
 * @Date: 2022-03-07 15:53:59
 * @FilePath: \src\views\PerformanceManage\TargetBook\components\WorkTarget\index.vue
-->
<!-- 分业务和流程，业务这边发起流程，直接跳转至GroupApproveManage文件夹下的components/FlowDialog文件 -->
<template>
  <div>
    <div>
      <el-button @click="addTarget" type="primary" icon="el-icon-circle-plus-outline" v-if="!kpiScope">发起流程</el-button>
      <span class="title" v-if="!kpiScope">选择日期：</span>
      <el-date-picker
        style="width:180px;"
        v-model="year"
        type="year"
        format="yyyy"
        value-format="yyyy"
        @change="fetchData"
        placeholder="请选择考核年限">
      </el-date-picker>
      <template v-if="kpiScope">
        <el-input clearable placeholder="按公司查询" v-model="companyName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-input clearable placeholder="按部门查询" v-model="depName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-input clearable placeholder="按姓名查询" v-model="userName" style="width:180px;margin-left:10px;" @change="fetchData"></el-input>
        <el-select v-model="status" placeholder="请选择状态" @change="fetchData" style="margin-left:10px;">
          <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
        <el-button type="primary" @click="fetchData" style="margin-left:10px;">查询</el-button>
      </template>
      <!-- <span class="title">选择状态：</span>
      <el-select v-model="status" placeholder="请选择状态" @change="fetchData">
        <el-option
          v-for="item in options"
          :key="item.value"
          :label="item.label"
          :value="item.value">
        </el-option>
      </el-select> -->
    </div>
    <el-table
      :data="tableData"
      max-height="600"
      style="width: 100%">
      <el-table-column
        v-if="kpiScope"
        prop="companyName"
        label="公司">
      </el-table-column>
      <el-table-column
        v-if="kpiScope"
        prop="depName"
        label="部门">
      </el-table-column>
      <el-table-column
        v-if="kpiScope"
        prop="userName"
        label="姓名">
      </el-table-column>
      <el-table-column
        prop="title"
        label="名称">
      </el-table-column>
      <el-table-column
        prop="targetTimeType"
        label="考核年限">
      </el-table-column>
      <el-table-column
        prop="createDate"
        label="发起时间">
      </el-table-column>
      <el-table-column
        prop="kpiGroupStatus"
        label="状态">
        <template slot-scope="scope">
          <el-tag v-if="scope.row.kpiGroupStatus=='not_submitted'" size="small" type="warning">草稿</el-tag>
          <el-tag v-else-if="scope.row.kpiGroupStatus=='under_review'" size="small" type="primary">审核中</el-tag>
          <el-tag v-else-if="scope.row.kpiGroupStatus=='rejected'" size="small" type="danger">已驳回</el-tag>
          <el-tag v-else-if="scope.row.kpiGroupStatus=='pass'||scope.row.kpiGroupStatus=='finish'" size="small" type="success">已通过</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="kpiGroupStatus"
        label="年终/半年考核">
        <template slot-scope="scope">
          <!-- <el-button v-if="scope.row.yearKpiScoreGroupList&&scope.row.yearKpiScoreGroupList.length>0" @click="handleView(scope.row,'year_kpi_work_target')" type="text" size="small">查看详情</el-button> -->
          <span v-if="scope.row.yearKpiScoreGroupList&&scope.row.yearKpiScoreGroupList.length>0">
            <el-button :disabled="halfOrYearDisable(scope, 'half_year')" @click="handleView(scope.row,'year_kpi_work_target', 'half_year')" type="text" size="small">半年详情</el-button>
            <el-button :disabled="halfOrYearDisable(scope, 'year')" @click="handleView(scope.row,'year_kpi_work_target', 'year')" type="text" size="small">年终详情</el-button>
          </span>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column
      label="操作" width="240">
      <span slot-scope="scope" v-if="kpiScope">
        <el-button @click="handleView(scope.row,'annual_perf')" type="text" size="small">查看详情</el-button>
      </span>
      <span slot-scope="scope" v-else>
        <el-button v-if="scope.row.kpiGroupStatus!='not_submitted'" @click="handleView(scope.row,'annual_perf')" type="text" size="small">查看详情</el-button>
        <!-- <el-button v-if="scope.row.kpiGroupStatus=='not_submitted'||scope.row.kpiGroupStatus=='rejected'" @click="toDuePage(scope.row)" type="text" size="small">发起流程</el-button> -->
        <!-- <el-button v-if="scope.row.kpiGroupStatus=='not_submitted'||scope.row.kpiGroupStatus=='rejected'" @click="handleEdit(scope.row)" type="text" size="small">编辑</el-button> -->
        <el-button v-if="scope.row.kpiGroupStatus=='not_submitted'||scope.row.kpiGroupStatus=='rejected'" @click="handleEdit(scope.row)" type="text" size="small">编辑</el-button>
        <!-- <el-button  @click="handlExame(scope.row)" v-if="scope.row.kpiGroupStatus=='pass' || scope.row.kpiGroupStatus=='finish'" type="text" size="small">发起考核</el-button> -->
        <el-button  @click="handlExame(scope.row,'half_year')" v-if="(scope.row.kpiGroupStatus=='pass' || scope.row.kpiGroupStatus=='finish')&&isShow(scope.row)" :disabled="isDisable(scope.row,'half_year')" type="text" size="small">发起半年考核</el-button>
        <el-button  @click="handlExame(scope.row,'year')" v-if="(scope.row.kpiGroupStatus=='pass' || scope.row.kpiGroupStatus=='finish')" :disabled="isDisable(scope.row,'year')" type="text" size="small">发起年终考核</el-button>
        <el-button v-if="scope.row.kpiGroupStatus=='not_submitted'" @click="handleDel(scope.row,'annual_perf')" type="text" size="small">删除</el-button>
        <!-- <el-button  @click="handleDel(scope.row)" type="text" size="small">删除</el-button> -->
      </span>
    </el-table-column>
    </el-table>
    <el-pagination
      v-if="pagination.total"
      background
      ref="paginationRef"
      layout="total, sizes, prev, pager, next"
      style="text-align: right;margin-top: 15px;"
      :page-size="pagination.size"
      @current-change="pageChange"
      @size-change="sizeChange"
      :total="pagination.total">
    </el-pagination>
    <chooseFlowDialog :visible.sync="visible" @confirm="confirm" :flowList="flowList"></chooseFlowDialog>
    <!-- 再次发起流程/查看 -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="isReInitiate" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
      :initiatorId="initiatorId" @success="fetchData" :actionType="actionType" :flowType="flowType" :flowName="flowName"/>
    <!-- 公司流程发起的弹窗 -->
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="[]" v-if="approveDialogVisible" @success="fetchData"
      :flowJson.sync="flowJson" :flowType.sync="performFlowType" :closeAll="true" />

  </div>
</template>

<script>
import Api from '@/api';
import mixin from '../mixin';
import chooseFlowDialog from '../chooseFlowDialog.vue';
import { localstorageSet } from '@/utils/auth.js';
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
import FlowDialog from '@/views/GroupApproveManage/Submitted/components/FlowDialog.vue'
export default {
  name: '',
  components: { chooseFlowDialog,ExamineDialog,FlowDialog },
  props: {
    kpiScope: {
      type: [String, Number, undefined],
      default: undefined
    }
  },
  mixins: [mixin],
  data() {
    return {
      approveDialogVisible:false,
      flowJson:{},
      // flowType:'',
      performFlowType:'workTarget',

      ExpensesClaimFormVisible:false,
      operaType:'',
      flowId:'',
      flowInstanceId:'',
      flowNodeType:'',
      showFlowLog:'',
      parallelNodeChooseList:'',
      manualChooseNodes:'',
      formId:'',
      searchFlowType:'',
      flowNodeProxyId:'',
      noFormFlowInstanceId:'',
      flowProxyId:'',
      initiatorId:'',
      actionType:'',
      flowType:'',
      flowName:'',
      isReInitiate:false,
      wholeData:[],
      tableData: [],
      year: '', // new Date().getFullYear().toString(),
      companyName: '',
      depName: '',
      userName: '',
      status: '',
      options: [{
        value: '',
        label: '全部状态'
      }, {
        value: 'not_submitted',
        label: '草稿'
      }, {
        value: 'under_review',
        label: '审核中'
      }, {
        value: 'rejected',
        label: '驳回'
      }, {
        value: 'pass',
        label: '已通过'
      }
      // {value: 'finish',label: '已完成'}
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      visible: false,
      flowRadio: '',
      flowList: [],
      editVisible: false,
      bizId: '',
      isExamine: false,
      exameBizId: '',
      assessmentCycle:'',
      assessmentCycleType:''
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    this.fetchData();
  },
  // activated(){
  //   this.fetchData();
  // },
  updated() {
  },
  methods: {
    halfOrYearDisable({ row }, type) {
      if (row.yearKpiScoreGroupList && row.yearKpiScoreGroupList.length) {
        return !(row.yearKpiScoreGroupList.some(i => i.assessmentCycle == type));
      }
      return true;
    },
    isShow(row){
      if(row.assessmentCycle=='year_and_half_year'){
        return true
      }else{
        return false
      }
    },
    isDisable(row,type) {
      if(row.canSubmitScore){
        if(row.assessmentCycle=='year_and_half_year'){
        if(type=='half_year'){
          if(row.yearKpiScoreGroupList){
            return true
          }else{
            return false;
          }
        }else{
          if(row.yearKpiScoreGroupList&&row.yearKpiScoreGroupList.length==1){
            return false;
          }else{
            return true
          }
        }
      }else{
        if(type=='half_year'){
          return true
        }else{
          if(row.yearKpiScoreGroupList){
            return true;
          }else{
            return false
          }
        }
      }
      }else{
        return true
      }
    },
    buttonName(row) {
      let buttonName = '发起';
      if (row.kpiGroupStatus == 'rejected')buttonName = '重新发起';
      return buttonName;
    },
    handleClick(row) {
      console.log(row);
    },
    fetchData() {
      const param = {
        data: {
          companyName: this.companyName || undefined,
          depName: this.depName || undefined,
          userName: this.userName || undefined,
          targetTime: this.year || undefined,
          kpiGroupStatus: this.status || undefined,
          manageType: 'work_target',
          kpiScope: this.kpiScope
          // targetTime:this.year,
          // kpiGroupStatus:this.status
        }
      };
      if (this.year)param.data.targetTime = this.year;
      if (this.status)param.data.kpiGroupStatus = this.status;

      this.$axios.post(
        Api.performance.getKpiGroupList,
        param,
        res => {
          if (res.isSuccess) {
            const tableData = res.data ? res.data : [];
            tableData.forEach(item => {
              const assessmentCycle = item.assessmentCycle == 'year' ? '年终' : '半年及年终';
              item.targetTimeType = item.targetTime + assessmentCycle;
            });
            tableData.sort((a, b) => {
              const aTime = new Date(a.createDate).getTime();
              const bTime = new Date(b.createDate).getTime();
              return bTime - aTime;
            });
            this.wholeData = tableData
            this.$refs.paginationRef?.handleCurrentChange(1);
            this.tableData = this.generateTableData(tableData);
            console.log(this.tableData,'++++++',this.wholeData);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // addTarget() {
    //   // 获取目标责任书的流程id,多个的话取最新的,自己排序
    //   this.isExamine = false
    //   this.formType().then(res => {
    //     if (res.isSuccess) {
    //       const data = res.data;
    //       const arr = [];
    //       // let find = data.find(item=>item.auditWay == 'annual_perf')
    //       const find = data.filter(item => item.auditWay == 'annual_perf');
    //       if (find && find.length) {
    //         const ids = find.map(item => item.id);
    //         this.getFlow(ids).then(resp => {
    //           if (resp.isSuccess) {
    //             const list = resp.data;
    //             if (list.length) {
    //               // 获取
    //               if (list.length > 1) {
    //                 this.flowList = list;
    //                 this.visible = true;
    //               } else {
    //                 // const params = JSON.stringify(list[0]);
    //                 const params = list[0];
    //                 this.toFlowPage(params);
    //               }
    //             } else {
    //               this.$message.error('暂无目标责任书流程,请先添加');
    //             }
    //           } else {
    //             this.$message.error('暂无目标责任书流程,请先添加');
    //           }
    //         });
    //       } else {
    //         this.$message.error('暂无目标责任书流程,请先添加');
    //       }
    //     } else {
    //       this.$message.error('暂无目标责任书流程,请先添加');
    //     }
    //   });
    // },
    toFlowPage(params) {
      this.flowType = 'annual_perf'
      this.flowJson = params//this.currentRow
      this.approveDialogVisible = true
      return
      // this.$router.push({
      //   name: 'groupApproveManage',
      //   params: { from: 'annual_perf', flow: params },
      //   query: { flowType: 'workTarget' }
      // });
    },
    handleCloseDialog() {
      this.editVisible = false;
    },
    handleEdit(row) {
      //查询当前绑定的流程，给到重新发起，调用弹窗审核
      this.getInstanceId(row.id,'annual_perf','edit').then(data=>{
        if(data){
          this.clickReInitiate(data)
        }else{
          this.$message.error('流程已删除，请删除本条目标责任书后重新发起')
        }
      })
      // this.bizId = row.id
      // this.editVisible = true
      // this.$router.push({
      //   path: '/manpowerResource/performanceManage/workTarget',
      //   query: {
      //     editType: 2,
      //     id: row.id
      //   }
      // });
    },
    toDuePage(row) {
      // this.$router.push({
      //   path: '/performanceManage/workTarget',
      //   query: {
      //     editType: 2,
      //     id: row.id
      //   }
      // });
      const url = this.$router.resolve({
        path: '/groupApproveManage',
        query: { tab: 'dueout' }
      });
      const findObj = {
        bizId: row.id, type: 'annual_perf'
      };
      localstorageSet('findObj', findObj);
      window.open(url.href, '_blank');
    },
    handleView(row,type, btnType) {
      console.log('handleView-row',row)
      var findRow = row.yearKpiScoreGroupList?.find(item => item.assessmentCycle == btnType);
       //查询当前绑定的流程，给到重新发起，调用弹窗审核
       this.getInstanceId(type=='year_kpi_work_target'?findRow.id:row.id,type,'view').then(data=>{
        if(data){
          this.previewHandle(data)
        }else{
          this.$message.error('流程已删除，请删除本条目标责任书后重新发起')
        }
      })
      // this.$router.push({
      //   path: '/manpowerResource/performanceManage/targetView',
      //   query: {
      //     type: 'workTarget',
      //     id: row.id
      //   }
      // });
    },
    handleClose() {
      this.visible = false;
    },
    confirm(val) {
      this.flowRadio = val;
      const find = this.flowList.find(item => item.id == this.flowRadio);
      if (find) {
        console.log(this.isExamine,'this.isExamine+++')
        if (this.isExamine) {
          this.$router.push({
            name: 'groupApproveManage',
            params: { from: 'annual_perf', flow: JSON.stringify(find) },
            query: { flowType: 'workTarget', id: this.exameBizId, isExamine: true,assessmentCycle:this.assessmentCycle,assessmentCycleType:this.assessmentCycleType }
          });
        } else {
          // const params = JSON.stringify(find);
          this.toFlowPage(find);
        }
      }
    },
    // handleDel(row) {
    //   this.$confirm('确认要删除吗?', '提示', {
    //     closeOnClickModal: false,
    //     confirmButtonText: '确定',
    //     cancelButtonText: '取消',
    //     type: 'warning'
    //   }).then(() => {
    //     this.$axios.post(
    //       Api.performance.deleteWorkTarget,
    //       { data: { id: row.id }},
    //       res => {
    //         if (res.isSuccess) {
    //           this.$message.success('操作成功');
    //           this.fetchData();
    //         // this.tableData = res.data ? res.data : [];
    //         } else {
    //           this.$message.error('删除失败');
    //         }
    //       });
    //     // 删除list
    //   }).catch(() => { });
    // }

  }
};
</script>

<style scoped lang="scss">
.title{
  display: inline-block;
  margin-left: 30px;
}
.el-radio{
  padding: 3px 5px;
}
</style>
