<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2025-03-11 17:59:58
-->
<!--
 * @Descripttion: 流程审批首页
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div class="content-container">
      <!-- <div class="left-side">
        <div>
          <span class="el-icon-s-flag">
            流程类型：
          </span>
        </div>
        <el-checkbox style="margin-bottom: 8px;" :indeterminate="isIndeterminate" @change="handleCheckAllChange"
          v-model="checkAll">全选</el-checkbox>
        <el-checkbox-group v-model="sFlowTypeList">
          <el-checkbox v-for="item in flowTypeList" :label="item.value" :key="item.value" style="margin-bottom: 5px;">
            {{ item.label }}
          </el-checkbox>
        </el-checkbox-group>
      </div> -->
      <div class="right-aside">
        <div class="right-aside-inner">
          <el-button type="primary" icon="el-icon-circle-plus-outline" @click="addApprove"
            style="position: absolute;right:40px;z-index: 100;">发起流程
          </el-button>
          <el-tabs v-model="activeName" @tab-click="handleClick" size="medium">
            <div class="search">
              <el-input style="width:120px;margin-right:5px" v-model.trim="flowInstanceName" @keyup.enter.native="getTaskList"
                placeholder="标题搜索">
                <i slot="suffix" style="cursor:pointer" @click="getTaskList" class="el-input__icon el-icon-search"></i>
              </el-input>
              <el-input style="width:120px;margin-right:5px" v-model.trim="flowName" @keyup.enter.native="getTaskList"
                placeholder="查找流程名称">
                <i slot="suffix" style="cursor:pointer" @click="getTaskList" class="el-input__icon el-icon-search"></i>
              </el-input>
              <el-input style="width:120px;margin-right:5px" v-model.trim="initiator" @keyup.enter.native="getTaskList"
                placeholder="查找发起人"  v-if="activeName != 'dueout' && activeName != 'submitted' && activeName != 'timedout'">
                <i slot="suffix" style="cursor:pointer" @click="getTaskList" class="el-input__icon el-icon-search"></i>
              </el-input>
              <!-- activeName == 'finished' ||  -->
              <el-select v-model="flowStatus" placeholder="流程状态" style="width:120px;margin-right:5px" clearable
                @change="getTaskList" v-if="activeName == 'finished' || activeName == 'timedout'">
                <el-option v-for="item in optionsFinished" :key="item.value" :label="item.label" :value="item.value">
                </el-option>
              </el-select>
              <el-select v-model="flowStatus" placeholder="流程状态" style="width:120px;margin-right:5px" clearable
                @change="getTaskList" v-if="activeName == 'submitted'">
                <el-option v-for="item in options" :key="item.value" :label="item.label" :value="item.value">
                </el-option>
              </el-select>
              <el-date-picker v-model="dateVal" @change="handleDateChange" type="daterange" range-separator="至"
                start-placeholder="开始日期" end-placeholder="结束日期" size="mini" value-format="yyyy-MM-dd" format="yyyy-MM-dd"
                style="width:200px;">
              </el-date-picker>
              <el-button type="primary" icon="el-icon-search" @click="getTaskList" style="margin-left:5px;border: none;">搜索</el-button>
              <el-button type="primary" :disabled="batchButtonDisable" icon="el-icon-circle-check" @click="batchAddApprove" style="border: none;" v-if="activeName == 'backlog'">批量审批</el-button>
              <el-button type="primary" icon="el-icon-circle-check" @click="transpond" style="border: none;" v-if="activeName !== 'dueout' && activeName !== 'timedout'">转 发</el-button>
            </div>
            <el-tab-pane label="待办" name="backlog">
              <Backlog v-if="activeName == 'backlog'" ref='backlog' :sFlowTypeList="sFlowTypeList" :initiator="initiator"
                :startDate="startDate" :endDate="endDate" :flowName="flowName" :status="flowStatus" @selectDataEvent="selectDataEvent"/>
            </el-tab-pane>
            <el-tab-pane label="已办" name="finished">
              <Finished v-if="activeName == 'finished'" ref='finished' :sFlowTypeList="sFlowTypeList"
                :initiator="initiator" :startDate="startDate" :endDate="endDate" :flowName="flowName"
                :status="flowStatus" />
            </el-tab-pane>
            <el-tab-pane label="待发" name="dueout">
              <DueOut v-if="activeName == 'dueout'" ref='dueout' :sFlowTypeList="sFlowTypeList" :initiator="initiator"
                :startDate="startDate" :endDate="endDate" :flowName="flowName" :status="flowStatus" />
            </el-tab-pane>
            <el-tab-pane label="已发" name="submitted">
              <Submitted v-if="activeName == 'submitted'" ref='submitted' :sFlowTypeList="sFlowTypeList"
                :initiator="initiator" :startDate="startDate" :endDate="endDate" :flowName="flowName"
                :status="flowStatus" />
            </el-tab-pane>
            <el-tab-pane label="超时跳过" name="timedout">
              <TimedOut v-if="activeName == 'timedout'" ref='timedout' :sFlowTypeList="sFlowTypeList"
                :initiator="initiator" :startDate="startDate" :endDate="endDate" :flowName="flowName"
                :flowInstanceName="flowInstanceName" :status="flowStatus" />
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>

      <!--发起流程弹窗  -->
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="sFlowTypeList" v-if="approveDialogVisible"
      @success="success" :flowJson.sync="flowJson" :flowType.sync="flowType" />

      <!-- 查看弹窗(对固定页面的查看) -->
      <!-- <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId" :showFlowLog="showFlowLog"
      :searchFlowType="searchFlowType" :isInitiator="true" /> -->

      <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
      <!-- <EnterpriseExamineDialog :btnVisible="btnVisible" :isExamine="isExamine" :flowId="flowId" :formId="formId"
          :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
          :visible.sync="examineDialogVisible" :isInitiator="true" /> -->
  </div>
</template>

<script>
import Api from '@/api';
import DueOut from './DueOut';
import Submitted from './Submitted';
import Backlog from './Backlog';
import Finished from './Finished';
import TimedOut from './TimedOut';

import FlowDialog from './Submitted/components/FlowDialog';
// import ExamineDialog from './components/ExamineDialog';
// import EnterpriseExamineDialog from './components/EnterpriseExamineDialog';

export default {
  name: '',
  components: {
    DueOut,
    Submitted,
    Backlog,
    Finished,
    TimedOut,
    FlowDialog
  },
  props: {},
  data() {
    var pageTypeList = this.$store.state.settings.budgetPagesName;
    var flowTypeList = [];
    for (const key in pageTypeList) {
      if (pageTypeList[key].isShow) {
        flowTypeList.push({
          value: key,
          label: pageTypeList[key].name
        });
      }
    }
    return {
      type: '',
      activeName: 'backlog',
      originList: Object.keys(pageTypeList),
      sFlowTypeList: Object.keys(pageTypeList),
      flowTypeList,
      checkAll: true,
      approveDialogVisible: false,
      initiator: '',
      startDate: '',
      endDate: '',
      flowName: '',
      dateVal: '',
      flowStatus: '',
      flowInstanceName:'',
      flowJson: null, // 从外部传入的流程,需要默认打开这个流程表单
      optionsFinished: [
        { label: '完结', value: 'end' },
        { label: '审批中', value: 'run' },
        { label: '撤回', value: 'withdraw' },
        { label: '驳回', value: 'rejected' },
      ],
      options: [
        { label: '完结', value: 'end' },
        { label: '审批中', value: 'run' },
        { label: '撤回', value: 'withdraw' },
        { label: '驳回', value: 'rejected' },
        { label: '草稿', value: 'draft' }
      ],
      flowType: '',
      batchButtonDisable:true
    };
  },
  watch: {},

  async created() {
    const params = this.$route.params;
    if (params && params.from) {
      if (params.from == 'annual_perf' || params.from == 'monthly_perf'|| params.from == 'year_kpi_work_target') {
        let flowJson = params.flow;
        flowJson = JSON.parse(flowJson);
        this.flowJson = flowJson;
        const query = this.$route.query;
        if (query?.flowType) this.flowType = query?.flowType;
        this.addApprove();
        // console.log('打开')
      } else if (params.from == 'contract_compliance_review') {


      }
    }
    if (this.$route?.query?.tab) {
      this.activeName = this.$route.query.tab;
    }
  },
  computed: {
    isIndeterminate() {
      if (this.sFlowTypeList.length == this.originList.length || this.sFlowTypeList.length == 0) {
        return false;
      } else {
        return true;
      }
    }
  },
  mounted() { },
  methods: {
    selectDataEvent(val){
      if(val.length)this.batchButtonDisable = false
      else this.batchButtonDisable = true
    },
    batchAddApprove(){
      // console.log('this.$refs.backlog',)
      this.$refs.backlog.batchAddApprove()
    },
    transpond(){
      this.$refs[this.activeName].transpond(this.activeName)
      // this.$refs.backlog.transpond(this.activeName)
    },
    handleClick() {
      this.initiator = '';
      this.flowName = '';
      this.dateVal = '';
      this.flowStatus = '';
      this.flowInstanceName = ''
    },
    handleDateChange() {
      if (this.dateVal && this.dateVal.length) {
        this.startDate = this.dateVal[0] + ' 00:00:00';
        this.endDate = this.dateVal[1] + ' 23:59:59';
      } else {
        this.startDate = '';
        this.endDate = '';
      }
      this.getTaskList();
    },
    getTaskList() {
      let query = {
        initiator:this.initiator,
        flowName:this.flowName,
        queryStartDate:this.startDate,
        queryEndDate:this.endDate,
        // name:this.flowInstanceName
        flowInstanceName:this.flowInstanceName

      }
      
      if(this.activeName == 'finished' || this.activeName == 'timedout'){
        if (this.flowStatus) {
          query.flowStatus = this.flowStatus
        }
      }
      if(this.activeName == 'submitted' || this.activeName == 'timedout'){
        query.status = this?.flowStatus || null
        query.name = this.flowInstanceName
        delete query.flowInstanceName
      }
      this.$nextTick(() => {
        this.$refs[this.activeName].pagination.pages = 1;
        this.$refs[this.activeName].queryData(query);
      });
    },
    changeSearchFlowType() {
      // this.$bus.$emit('init', this.searchFlowType);
    },
    handleCheckAllChange(isAll) {
      if (isAll) {
        this.sFlowTypeList = this.originList;
      } else {
        this.sFlowTypeList = [];
      }
    },
    addApprove() {
      this.approveDialogVisible = true;
    },
    success() {
      var temp = this.activeName;
      // 刷新列表
      this.activeName = '';
      this.$nextTick(() => {
        this.activeName = temp;
      });
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .groupApproveManage {
  overflow: hidden;
}
.content-container {
  background: #fff;
  padding: 10px;
  height: 100%;

  .left-side {
    width: 200px;
    height: 100%;
    border-right: 1px solid #e2e2e2;
    padding: 0 10px;
  }

}
.right-aside {
  height:100%;
.right-aside-inner {
  height:100%;
  left: 0;
  padding: 0 20px;
  width: 100%;
  .el-tabs{
    height:100%;
  }
}
}
::v-deep{
  .el-tabs__content{
      height:calc(100% - 55px) !important;
    }
  .dytable-view-container{
    padding:0;
  }
  .el-tab-pane{
    height:calc(100% - 40px);
  }
  .container{
    height:100%;
  }
  .el-table__fixed-right:before{
    display:none;
  }
  .el-table__header-wrapper th,.el-table__fixed-right th{
    font-weight:700;
    color:#333;
    font-size:14px;
  }
  .el-table__row,.el-table__cell{
    font-size:13px;
    .el-button--text{
      font-size:13px !important;
    }
  }
}
.el-icon-s-flag {
  font-size: 15px;
  margin-bottom: 15px;
}
</style>