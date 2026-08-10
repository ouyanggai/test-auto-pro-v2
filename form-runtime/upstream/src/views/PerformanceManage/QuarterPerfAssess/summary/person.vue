<template>
  <div class="performance-dashboard">
     <!-- 临时调试切换日期：<el-date-picker type="year" value-format="yyyy" v-model="formData.year" :clearable="false" style="width:100px;" class="font-large" @change="init">
    </el-date-picker>
    <el-select ref="quarterPicker" style="width:80px;" placeholder=" " v-model="formData.quarter" class="font-large" @change="init">
      <el-option v-for="item in [{ label: '一', value: 1 }, { label: '二', value: 2 }, { label: '三', value: 3 }, { label: '四', value: 4 }]"
      :key="item.value" :label="item.label" :value="item.value"></el-option>
    </el-select> -->
    <!-- 顶部信息区 -->
    <div class="header-section">
      <div class="header-info">
        <div class="user-profile">
          <div class="current-date" style="margin-right:30px;">{{ currentDate }}</div>
          <span class="user-name">{{ userName }}</span>
          <span class="user-title">{{ userTitle }}</span>
          <!-- <span class="quarter-text" style="margin-left:22px">当前季度：</span>
          <span class="quarter-value">{{ currentQuarter }}</span> -->
        </div>
        <div class="quarter-indicator">
          <span class="quarter-text">当前季度：</span>
          <span class="quarter-value">{{ currentQuarter }}</span>
        </div>
      </div>
      <div class="action-buttons">
        <el-button 
          v-for="(action, index) in actions" 
          :key="index"
          :type="action.type"
          plain
          size="medium"
          class="action-btn"
          @click="handleActionClick(action)"
        >
          {{ action.label }}
        </el-button>
      </div>
    </div>
    <!-- 中间区域：任务列表 + 绩效趋势 -->
    <div class="middle-content">
      <div class="task-list-container">
        <el-card class="section-card">
          <div slot="header" class="section-header">
            <!-- <i class="el-icon-tasks header-icon"></i> -->
            <span class="header-title">季度工作计划</span>
            <span style="margin-left:auto" >
              <el-date-picker v-model="year1" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="currentPlan"/>
              <el-select ref="quarterPicker" style="width:90px;" placeholder=" " v-model="quarter1" class="font-large" @change="currentPlan">
                <el-option v-for="item in [{ label: '一季度', value: 1 }, { label: '二季度', value: 2 }, { label: '三季度', value: 3 }, { label: '四季度', value: 4 }]"
                :key="item.value" :label="item.label" :value="item.value"></el-option>
              </el-select>
            </span>
          </div>
          <div class="task-list-container-scroll" style="min-height:400px;">
          <div class="task-list">
            <el-empty v-if="!currentPlans || currentPlans.length === 0"></el-empty>
            <el-card 
              v-for="(task, index) in currentPlans"
              :key="index"
              class="task-item high-priority"><!-- :class="{ 'high-priority': task.priority === 'high' }" -->
              <!-- <div class="task-header">
                <span class="task-title">{{ task.title }}</span>
                <span class="task-progress">{{ task.progress }}</span>
              </div> -->
              <div class="task-desc">{{ task.phasedAchieve }}  <span style="color:#668eff;font-weight:700">（{{ task.weight*100}} %）</span></div>
              <!-- <el-progress :percentage="task.weight*100"></el-progress> -->
              <div class="task-actions" style="">
                <el-button type="text" size="mini" @click="viewTask(task)">查看</el-button>
                <el-button type="text" size="mini" @click="handleTask(task)">下发任务</el-button>
              </div>
            </el-card>
          </div>
          </div>
        </el-card>
      </div>
      <!-- 绩效趋势图表 -->
      <div class="performance-chart-container">
        <el-card class="section-card">
          <div slot="header" class="section-header">
            <!-- <i class="el-icon-line-chart header-icon"></i> -->
            <span class="header-title">绩效趋势分析</span>
            <span style="margin-left:auto" >
              <el-date-picker v-model="year2" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="getChartData"/>
            </span>
          </div>
          <div class="chart-wrapper" style="height:400px;">
            <div id="performanceChart" class="chart-area"></div>
          </div>
        </el-card>
      </div>
    </div>
    <!-- 年度工作计划 -->
    <div class="annual-plan-container" v-if="false">
      <el-card class="section-card">
        <div slot="header" class="section-header">
          <!-- <i class="el-icon-calendar header-icon"></i> -->
          <span class="header-title">年度工作计划</span>
        </div>
        <div class="annual-plans">
          <el-row :gutter="16">
            <el-col 
              v-for="(plan, index) in annualPlans" 
              :key="index" 
              :span="3"
              class="plan-col"
            >
              <div class="plan-item" :style="{ backgroundColor: plan.color }">
                <div class="plan-number">{{ index + 1 }}</div>
                <div class="plan-content">{{ plan.content }}</div>
              </div>
            </el-col>
          </el-row>
        </div>
      </el-card>
    </div>

    <!-- 底部表格区：季度计划 + 绩效考核 -->
    <div class="bottom-tables">
      <!-- 季度工作计划 -->
      <div class="quarter-plan-table">
        <el-card class="section-card" style="min-height:320px;">
          <div slot="header" class="section-header">
            <!-- <i class="el-icon-document header-icon"></i> -->
            <span class="header-title">季度工作计划</span>
            <span style="margin-left:auto" >
              <el-date-picker v-model="year3" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="getQuarterPlans"/>
            </span>
          </div>
          <el-table 
            :data="quarterPlans" 
            border max-height="400"
            class="content-table"
            v-loading="loading.quarterPlans"
          >
            <el-table-column prop="name" label="计划时间" mwd="180">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewQuarterPlan(scope.row)">{{ showPlanName(scope.row) }}</el-button>
              </template>
            </el-table-column>
            <el-table-column prop="depName" label="部门" mwd="120"></el-table-column>
            <el-table-column prop="dutyName" label="岗位" mwd="120"></el-table-column>
            <!-- <el-table-column prop="quarter" label="季度" mwd="80"></el-table-column> -->
            <el-table-column prop="publishStatus" label="公示状态" mwd="120">
              <template slot-scope="scope">
                <el-tag :type="publicityColor[scope.row.noticeStatus]">{{ publicity[scope.row.noticeStatus] || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="auditStatus" label="审核状态" mwd="120">
              <template slot-scope="scope">
                <el-tag :type="flowOptions[scope.row.kpiGroupStatus].tag">{{ flowOptions[scope.row.kpiGroupStatus].statusName }}</el-tag>
              </template>
            </el-table-column>
            <!-- <el-table-column label="操作" mwd="120">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewQuarterPlan(scope.row)">查看详情</el-button>
              </template>
            </el-table-column> -->
          </el-table>
        </el-card>
      </div>
      <!-- 绩效考核 -->
      <div class="performance-review-table">
        <el-card class="section-card" style="min-height:320px;">
          <div slot="header" class="section-header">
            <!-- <i class="el-icon-star-off header-icon"></i> -->
            <span class="header-title">绩效考核结果</span>
            <span style="margin-left:auto" >
              <el-date-picker v-model="year4" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="getPerformance"/>
            </span>
          </div>
          <el-table 
            :data="performanceReviews" 
            border max-height="400"
            class="content-table"
            v-loading="loading.performanceReviews"
          >
            <el-table-column prop="name" label="考核时间" mwd="180">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewPerformanceReview(scope.row)">{{ showPlanName(scope.row) }}</el-button>
              </template>
            </el-table-column>
            <el-table-column prop="depName" label="部门" mwd="120"></el-table-column>
            <el-table-column prop="dutyName" label="岗位" mwd="120"></el-table-column>
            <!-- <el-table-column prop="quarter" label="季度" mwd="80"></el-table-column> -->
            <el-table-column prop="type" label="绩效类型" mwd="120">
              <template slot-scope="scope">
                {{ scope.row.kpi2Type == 'personal_kpi' ? '个人绩效' : '组织绩效' }}
              </template>
            </el-table-column>
            <!-- <el-table-column prop="ratingLevel" label="组织绩效评级" mwd="140">
              <template slot-scope="scope">
                {{ scope.row.ratingLevel == 'none' ? '未评级' : scope.row.ratingLevel }}
              </template>
            </el-table-column> -->
            <el-table-column prop="totalScore" label="分数" mwd="80"></el-table-column>
            <el-table-column prop="ratingLevel" label="评级" mwd="80">
              <template slot-scope="scope">
                {{ scope.row.ratingLevel == 'none' ? '未评级' : levelEnums[scope.row.ratingLevel] }}
              </template>
            </el-table-column>
            <!-- <el-table-column label="操作" mwd="120">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewPerformanceReview(scope.row)">查看详情</el-button>
              </template>
            </el-table-column> -->
          </el-table>
        </el-card>
      </div>
    </div>
    <chooseFlowDialog :visible.sync="visible" @confirm="confirm" :flowList="flowList"></chooseFlowDialog>
    <FlowDialog :visible.sync="approveDialogVisible" :sFlowTypeList="[]" v-if="approveDialogVisible" @success="fetchData"
      :flowJson.sync="flowJson" :flowType.sync="flowType" :closeAll="true" />
    <el-dialog center v-if="previewVisible" :visible.sync="previewVisible" fullscreen append-to-body>
      <div class="dialog-container" style="height: calc(100vh - 90px);">
        <personDetail :currentPlanRow="currentPlanRow" ref="detailRef"></personDetail>
      </div>
      <div slot="footer" class="dialog-footer" style="text-align: center;">
        <el-button @click="previewVisible=false">关 闭</el-button>
      </div>
    </el-dialog>
    <!-- 任务下发弹窗 -->
    <AddTaskArrangeDialog v-if="AddArrangeDialogVisible" arrangeType="add" :visible.sync="AddArrangeDialogVisible"
      :dataFromAssigned="rowData" @addIndicatorEvent="addIndicatorEvent" :fromAssign="3" />
  </div>
</template>

<script>
import * as echarts from 'echarts'
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import chooseFlowDialog from '../../TargetBook/components/chooseFlowDialog.vue';
import FlowDialog from '@/views/GroupApproveManage/Submitted/components/FlowDialog.vue'
import personDetail from './personDetail.vue'
import AddTaskArrangeDialog from '@/views/TaskManage/TaskArrange/components/AddTaskArrangeDialog';
var publicity = { not_started: '未公示', in_progress: '公示中', end: '公示结束' };
var publicityColor = { not_started: 'danger', in_progress: 'success', end: '' };
export default {
  name: 'PerformanceDashboard',
  components: {chooseFlowDialog, FlowDialog, personDetail, AddTaskArrangeDialog},
  data() {
    return {
      levelEnums: { 'level_a': 'A级', 'level_b': 'B级', 'level_c': 'C级', 'level_d': 'D级', 'level_e': 'E级' },
      year1: '',
      quarter1: '',
      year2: '',
      year3: '',
      year4: '',
      rowData: {},
      AddArrangeDialogVisible: false,
      formData: { quarter: '', year: '' },
      currentPlanRow: {},
      previewVisible: false,
      currentPlans: [],
      publicity,
      publicityColor,
      flowOptions: {
        not_submitted: {
          tag: 'warning',
          statusName: '草稿'
        },
        under_review: {
          tag: 'primary',
          statusName: '审核中'
        },
        rejected: {
          tag: 'danger',
          statusName: '驳回'
        },
        pass: {
          tag: 'success',
          statusName: '已通过'
        },
        finish: {
          tag: 'success',
          statusName: '已通过'
        }
      },
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' },
      flowType: '',
      flowList: [],
      visible: false,
      flowJson: {},
      approveDialogVisible: false,
      // 当前日期
      currentDate: '',
      // 用户信息
      userName: '-',
      userTitle: '- / -',
      // 当前季度
      currentQuarter: '第*季度',
      // 操作按钮
      actions: [
        { label: '发起工作计划', type: 'success', auditWay: "kpi2_work_plan" },
        { label: '发起绩效考核', type: 'primary', auditWay: "kpi2_appraise" },
        { label: '绩效管控与精进', type: 'warning', auditWay: "" },
        { label: '发起绩效复盘', type: 'info', auditWay: "" }
      ],
      // 任务列表
      taskList: [
        {
          title: '进度管理',
          description: '完成目标计划的开发与部署工作，确保按时交付',
          progress: '20%',
          priority: 'normal'
        },
        {
          title: '新功能上线',
          description: '一次上线成功率达到80%以上，降低回滚率',
          progress: '30%',
          priority: 'high'
        },
        {
          title: '数据安全',
          description: '确保数据安全事故0次，完成安全审计',
          progress: '20%',
          priority: 'normal'
        },
        {
          title: '系统稳定',
          description: '月度无故障运行时间达到99.9%以上',
          progress: '30%',
          priority: 'high'
        }
      ],
      // 年度计划
      annualPlans: [
        { content: '实现净利润1000万元', color: '#66cc99' },
        { content: '实现合同额3000万元', color: '#668eff' },
        { content: '实现合同回款2400万元', color: '#6666ff' },
        { content: '实现产值3000万元', color: '#ff6666' },
        { content: '完成人力资源全生命周期管理的开发上线', color: '#ffcc66' },
        { content: '实现财务业务一体化的功能开发和数据对接', color: '#99ccff' },
        { content: '实现人才梯队的建设', color: '#9999ff' }
      ],
      personChartData: [],
      // 季度计划数据
      quarterPlans: [],
      // 绩效考核数据
      performanceReviews: [],
      // 加载状态
      loading: {
        quarterPlans: false,
        performanceReviews: false
      },
      // 图表实例
      chartInstance: null
    }
  },
  created() {
    this.getUserInfo();
    const month = new Date().getMonth() + 1;
    const quarter = Math.ceil(month / 3);
    this.formData.quarter = quarter;
    this.formData.year = new Date().getFullYear() + '';
    this.quarter1 = quarter;
    this.year1 = this.formData.year;
    this.year2 = this.formData.year;
    this.year3 = this.formData.year;
    this.year4 = this.formData.year;
    Object.defineProperty(this.formData, 'targetTime', { get() { return this.year + '' + this.quarter; } });
    this.currentQuarter = `第${this.quarterEnums[quarter]}季度`;
    this.init();
  },
  mounted() {
    window.abb = this;
    // 初始化日期
    this.initCurrentDate()
    // 初始化图表
    // this.initPerformanceChart()
    // 监听窗口大小变化，重绘图表
    window.addEventListener('resize', this.handleWindowResize)
    document.addEventListener('visibilitychange', this.handleVisibilityChange);
  },
  beforeDestroy() {
    // 移除事件监听
    window.removeEventListener('resize', this.handleWindowResize)
    // 销毁图表实例
    if (this.chartInstance) {
      this.chartInstance.dispose()
    }
    document.removeEventListener('visibilitychange', this.handleVisibilityChange);
  },
  watch: {
    '$store.state.app.sidebar.opened'(newV, oldV) {
      setTimeout(_ => { this.handleWindowResize(); }, 300);
    }
  },
  methods: {
    handleVisibilityChange() {
      // 检查页面是否从隐藏状态变为可见状态
      if (!document.hidden) {
        console.log('浏览器标签页已切换回来，准备刷新数据。');
        this.init();
      }
    },
    addTask(row, processName) {
      console.log('row', row);
      this.rowData = {
        form: {
          name: '', // `${this.userName}${this.formData.year}年${this.formData.quarter}季度计划/${row.phasedAchieve}`,
          isAssociated: false,
          project: {
            id: '',
            name: ''
          },
          user: {
            name: '',
            id: ''
          },
          endTime: '',
          remark: '', // 任务要求
          planType: 'target', // 任务要求
          planBusinessList: [
            {
              businessId: row.id,
              planBusinessType: 'work_plan'
            }
          ]
        }
        // planBusiness: {
        //   businessId: row.id,
        //   planBusinessType: 'kpi2_appraise', // 'prophase_procedures',
        //   businessType: '季度计划', // row.type,
        //   processName: row.phasedAchieve
        // },
        // kpiGroup:{
        //   name:'',
        //   resolveContent:''
        // },
      };
      this.AddArrangeDialogVisible = true;
    },
    addIndicatorEvent() {
      this.AddArrangeDialogVisible = false;
      // this.initData()
    },
    init () {
      this.getQuarterPlans();
      this.getPerformance();
      this.getChartData();
      this.currentPlan();
    },
    getUserInfo() {
      var curCompanyId = localstorageGet('companyId');
      console.log(curCompanyId, 'curCompanyId');
      this.$axios.post('/web/user/api/user/getUserInfoById', // 用户信息
        { data: { id: this.$store.state.user.userId, flag: "company", } },
        res => {
          if (res.isSuccess) {
            var deptDutyVo = {};
            var { data = {} } = res;
            var { deptDutyVos = [] } = data;
            deptDutyVo = deptDutyVos?.find(i => {
              // return i.companyVo.id == curCompanyId;
              return i.userDutyVo.dutyType == '1';
            });
            var { deptVo = {}, dutyVo = {}, companyVo = {}} = deptDutyVo;
            this.userName = data?.name || '';
            this.userTitle = deptVo?.departmentName + ' / ' + dutyVo?.dutyName;
            // this.formData.userId = data?.id || '';
            // this.formData.companyName = companyVo?.name || '';
            // this.formData.companyId = companyVo?.id || '';
            // this.formData.departmentName = deptVo?.departmentName || '';
            // this.formData.departmentId = deptVo?.id || '';
            // this.formData.dutyName = dutyVo?.dutyName || '';
            // this.formData.dutyId = dutyVo?.id || '';
          }
        });
    },
    showPlanName({targetTime}) {
      targetTime += '';
      console.log(targetTime, 'targetTime');
      const year = targetTime.substr(0, 4);
      const month = targetTime.substr(4, 2);
      return year + '年' + this.quarterEnums[month] + '季度';
    },
    currentPlan() {
      const param = {
        data: {
          kpiScope: 'personal',
          // targetTime: this.formData.targetTime
          targetTime: this.year1 ? this.year1 + '' + this.quarter1 : this.formData.targetTime
        },
        pagination: false,
      };
      this.$axios.post('/web/plan/api/workPlanGroup/list', param,
        res => {
          if (res.isSuccess && res.data) {
            this.currentPlanById(res.data[0]?.id)
          } else {
            this.currentPlans = [];
          }
        }
      );
    },
    currentPlanById(id) {
      this.$axios.post('/web/plan/api/workPlanGroup/findById', { data: { id }}, res => {
        if (res.isSuccess) {
          this.currentPlans = res.data.workPlans || [];
        }
      });
    },
    getQuarterPlans() {
      console.log(this.formData.targetTime, 'targetTime')
      const param = {
        data: {
          kpiScope: 'personal',
          targetTime: this.year3 ? this.year3 : this.formData.year + ''
        },
        pagination: false,
      };
      this.$axios.post('/web/plan/api/workPlanGroup/list', param,
        res => {
          if (res.isSuccess) {
            this.quarterPlans = res.data || [];
          }
        }
      );
    },
    getPerformance() {
      const param = {
        data: {
          kpiScope: 'personal',
          targetTime: this.year4 ? this.year4 : this.formData.year + ''
        },
        pagination: false,
      };
      this.$axios.post('/web/plan/api/kpi2Group/list', param,
        res => {
          if (res.isSuccess) {
            this.performanceReviews = res.data || [];
          } else {
            this.performanceReviews = [];
          }
        }
      );
    },
    getChartData() {
      var targetTime = this.year2 ? this.year2 + '' : this.formData.year + '';
      const param = {
        data: {
          kpiScope: 'personal',
          targetTime
        },
        pagination: false,
      };
      this.$axios.post('/web/plan/api/kpi2Group/list', param,
        res => {
          if (res.isSuccess) {
            this.processEchartData(res.data || [], targetTime);
          } else {
            this.processEchartData([], targetTime);
          }
        }
      );
    },
    processEchartData(data, targetTime) {
      console.log(data, 'processEchartData data');
      var arr = [];
      for (var i = 1; i <= 4; i++) {
        const quarter = data.find(item => String(item.targetTime) == targetTime + '' + i);
        // if (!quarter && quarter.ratingLevel === 'none') return;
        console.log(quarter, targetTime + i);
        if (quarter) {
          arr.push(quarter.totalScore || 0);
        } else {
          arr.push(0);
        }
      }
      console.log(arr, 'arrr')
      this.personChartData = arr;
      this.initPerformanceChart();
    },
    fetchData() {
      this.init();
    },
    getFlowByTemplate(auditWay) {
      const param = {
        data: {
          useScope: 'invest',
          // auditWay,
          auditWayList: [auditWay]
        },
        showMe: true,
        ignoreFormTemplateBizRelevanceData: true,
        ignoreTemplateData: true,
        platformCode: '999999',
        pagination: true,
        pages: 1,
        size: 99
      };
      this.$axios.post(Api.schedule.getFlowTemplateList, param, (res) => {
        if (res.isSuccess) {
          if (!res.data || res.data.length == 0) {
            this.$message.error('暂无流程权限，请联系管理员');
            return;
          }
          if (res.data?.length > 1) {
            this.flowList = res.data;
            this.visible = true;
          } else {
            this.getFlowFindById(res.data[0].id);
          }
        }
      });
    },
    getFlowFindById(id) {
      this.$axios.post(Api.schedule.flowTemplateFindById, { data: { id }}, (res) => {
        if (res.isSuccess) {
          this.flowJson = res.data;
          this.approveDialogVisible = true;
        }
      });
    },
    confirm(val) {
      if (val) {
        const find = this.flowList.find(item => item.id == val);
        if (find) {
          this.visible = false
          // this.toFlowPage(find);
          this.getFlowFindById(find.id);
        }
      } else {
        this.$message.error('请选择流程')
      }
    },
    /**
     * 初始化当前日期
     */
    initCurrentDate() {
      const options = { 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric', 
        weekday: 'long' 
      }
      this.currentDate = new Date().toLocaleDateString('zh-CN', options)
    },
    
    /**
     * 初始化绩效趋势图表
     */
    initPerformanceChart() {
      const chartDom = document.getElementById('performanceChart')
      this.chartInstance = echarts.init(chartDom)
      
      const option = {
        tooltip: {
          trigger: 'axis',
          backgroundColor: 'rgba(255, 255, 255, 0.9)',
          borderColor: '#ddd',
          borderWidth: 1,
          textStyle: { color: '#333' },
          formatter: '{b}<br/>{a0}: {c0}<br/>'
          // formatter: '{b}<br/>{a0}: {c0}<br/>{a1}: {c1}'
        },
        legend: {
          data: ['个人绩效', '组织绩效'],
          top: 0
        },
        grid: {
          left: '1%',
          right: '3%',
          bottom: '1%',
          containLabel: true
        },
        xAxis: {
          type: 'category',
          boundaryGap: false,
          data: ['第一季度', '第二季度', '第三季度', '第四季度'],
          axisTick: {
            show: true,
            alignWithLabel: true
          },
          axisLine: {
            lineStyle: { color: '#ddd' }
          }
        },
        yAxis: {
          type: 'value',
          // max: 5,
          // type: 'category',
          data: ['D', 'C', 'B', 'A'],
          axisTick: {
            show: true,
            alignWithLabel: true
          },
          axisLine: {
            lineStyle: { color: '#ddd' },
            alignWithLabel: true
          },
          splitLine: {
            show: true,
            // interval: 0,
            lineStyle: { color: '#f5f5f5' }
          }
        },
        series: [
          {
            name: '个人绩效',
            type: 'line',
            data: this.personChartData, //  [90, 97, 89, 0],
            // data: ['B', 'D', 'A', 'C'],
            symbol: 'circle',
            symbolSize: 8,
            label: { 
              show: true, 
              position: 'top',
              fontSize: 12
            },
            lineStyle: { 
              color: '#668eff',
              width: 2
            },
            itemStyle: {
              color: '#668eff'
            },
            areaStyle: {
              color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: 'rgba(102, 142, 255, 0.3)' },
                { offset: 1, color: 'rgba(102, 142, 255, 0)' }
              ])
            }
          },
          // {
          //   name: '组织绩效',
          //   type: 'line',
          //   // data: [94, 97, 95, 0],
          //   data: ['C', 'A', 'B', 'C'],
          //   symbol: 'circle',
          //   symbolSize: 8,
          //   label: { 
          //     show: true, 
          //     position: 'top',
          //     fontSize: 12
          //   },
          //   lineStyle: { 
          //     color: '#66cc99',
          //     width: 2
          //   },
          //   itemStyle: {
          //     color: '#66cc99'
          //   },
          //   areaStyle: {
          //     color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          //       { offset: 0, color: 'rgba(102, 204, 153, 0.3)' },
          //       { offset: 1, color: 'rgba(102, 204, 153, 0)' }
          //     ])
          //   }
          // }
        ]
      }

      this.chartInstance.setOption(option)
    },
    /**
     * 处理窗口大小变化
     */
    handleWindowResize() {
      if (this.chartInstance) {
        this.chartInstance.resize()
      }
    },
    /**
     * 根据状态获取标签类型
     */
    getStatusType(status) {
      switch(status) {
        case '审核中':
        case '公示中':
          return 'warning'
        case '通过':
        case '已结束':
          return 'success'
        case '未通过':
          return 'danger'
        default:
          return 'info'
      }
    },
    /**
     * 根据评级获取标签类型
     */
    getRatingType(rating) {
      const ratingMap = {
        'A': 'success',
        'B': 'primary',
        'C': 'warning',
        'D': 'danger'
      }
      return ratingMap[rating] || 'info'
    },
    /**
     * 处理操作按钮点击
     */
    handleActionClick(action) {
      if (!action.auditWay) {
        this.$message.warning('建设中...');
        return;
      }
      var instance = this.$initFlow({
        params: {
          auditWay: action.auditWay // 流程模板类型
        },
        onSuccess: () => { // 流程发起成功回调
          this.fetchData();
        }
      });
      console.log(instance, 'instance');
      // this.flowType = action.auditWay;
      // this.getFlowByTemplate(action.auditWay);
    },
    /**
     * 查看任务详情
     */
    viewTask(task) {
      // this.$message.info(`查看任务：${task.title}`)
      // console.log(this.$router, 'this.$router')
      sessionStorage.setItem('currentPlanRow', JSON.stringify(task));
      // var url = this.$router.resolve({ name: 'myQuarterlySummaryDetail', params: { currentPlanRow: task }});
      // window.open(url.href, '_blank');
      this.$router.push({ name: 'myQuarterlySummaryDetail', params: { currentPlanRow: task } });
      // this.currentPlanRow = task;
      // this.previewVisible = true;
    },
    /**
     * 处理任务
     */
    handleTask(task) {
      this.addTask(task);
      // this.$message.success(`处理任务：${task.title}`)
    },
    /**
     * 查看季度计划详情
     */
    viewQuarterPlan(row) {
      var operaType = row.kpiGroupStatus === 'not_submitted' ? 'reEdit' : undefined;
      this.$flowDetail({
        data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }], operaType },
        callback: () => {
          setTimeout(() => {
            this.init();
          }, 700);
        }
      });
      // this.$fm.show("flowDetail", {
      //   data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }], operaType },
      //   callback: () => {
      //     this.init();
      //   }
      // });
      // this.$fm.show("flowDetail", { data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }], operaType: 'edit', actionType: 'examine', isExamine: true }});
    },
    /**
     * 查看绩效考核详情
     */
    viewPerformanceReview(row) {
      this.$flowDetail({ data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }] }});
      // this.$fm.show("flowDetail", { data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }] }});
    }
  }
}
</script>

<style scoped lang="scss">
::v-deep .is-fullscreen {
  .el-dialog__body {
    padding: 0 10px 0 10px;
    max-height: unset;
    min-height: unset;
    overflow-y: auto;
  }
}
::v-deep .el-card__header{
  padding: 7px 10px 7px 10px;
}
::v-deep .el-card__body {
  padding: 14px;
}
.task-list-container-scroll {
  max-height: 400px; /* 设置最大高度 */
  overflow-y: auto;  /* 垂直滚动 */
}
/* 添加滚动条美化 */
.task-list-container-scroll::-webkit-scrollbar {
  width: 8px;
}

.task-list-container-scroll::-webkit-scrollbar-track {
  background: #f5f7fa;
  border-radius: 4px;
}

.task-list-container-scroll::-webkit-scrollbar-thumb {
  background: #c0c4cc;
  border-radius: 4px;
}

.task-list-container-scroll::-webkit-scrollbar-thumb:hover {
  background: #909399;
}
.performance-dashboard {
  // padding: 10px;
  background-color: #f5f7fa;
  min-height: 100vh;
}

/* 顶部信息区样式 */
.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  flex-wrap: wrap;

  background-color: white;
  padding: 10px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
  border-radius: 5px;
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.current-date {
  font-size: 14px;
  color: #666;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-name {
  font-size: 18px;
  font-weight: 600;
  color: #333;
}

.user-title {
  font-size: 14px;
  color: #666;
  background-color: #e8f4fd;
  padding: 2px 8px;
  border-radius: 4px;
}

.quarter-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.quarter-text {
  font-size: 14px;
  color: #666;
}

.quarter-value {
  font-size: 16px;
  font-weight: 600;
  color: #333;
  background-color: #fff8e6;
  padding: 2px 8px;
  border-radius: 4px;
}

.action-buttons {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.action-btn {
  transition: all 0.3s ease;
}

.action-btn:hover {
  transform: translateY(-2px);
}

/* 中间内容区样式 */
.middle-content {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.task-list-container {
  flex: 1;
  min-width: 300px;
}

.performance-chart-container {
  flex: 1;
  min-width: 500px;
}

/* 通用区块样式 */
.section-card {
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
  border-radius: 6px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.section-card:hover {
  box-shadow: 0 4px 16px 0 rgba(0, 0, 0, 0.08);
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.header-icon {
  color: #668eff;
}

/* 任务列表样式 */
.task-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 10px;
}

.task-item {
  // transition: all 0.3s ease;
  // cursor: pointer;
  position: relative;
}
::v-deep .task-item .el-card__body {
  padding: 10px;
}

.task-item:hover {
  /* transform: translateX(5px); */
}

.task-item.high-priority {
  border-left: 3px solid #1989fa//  #ff6666;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.task-title {
  font-weight: 600;
  color: #333;
}

.task-progress {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 12px;
  background-color: #f0f2f5;
  color: #666;
}

.task-desc {
  font-size: 14px;
  color: #666;
  // margin-bottom: 12px;
  line-height: 1.5;
}

.task-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* 图表样式 */
.chart-wrapper {
  padding: 10px 0;
}

.chart-area {
  width: 100%;
  height: 350px;
}

/* 年度计划样式 */
.annual-plan-container {
  margin-bottom: 10px;
}

.annual-plans {
  padding: 10px 0;
}

.plan-col {
  transition: all 0.3s ease;
}

.plan-col:hover {
  transform: translateY(-5px);
}

.plan-item {
  height: 100%;
  border-radius: 6px;
  padding: 15px 10px;
  color: #fff;
  text-align: center;
  transition: all 0.3s ease;
}

.plan-number {
  font-size: 24px;
  font-weight: bold;
  margin-bottom: 8px;
  opacity: 0.9;
}

.plan-content {
  font-size: 14px;
  line-height: 1.4;
  height: calc(100% - 32px);
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 底部表格区样式 */
.bottom-tables {
  display: flex;
  gap:10px;
  flex-wrap: wrap;
}

.quarter-plan-table,
.performance-review-table {
  flex: 1;
  min-width: 600px;
}

.content-table {
  margin-top: 10px;
  width: 100%;
}

/* 响应式调整 */
@media screen and (max-width: 1200px) {
  .middle-content {
    flex-direction: column;
  }
  
  .task-list-container,
  .performance-chart-container {
    width: 100%;
    min-width: auto;
  }
  
  .quarter-plan-table,
  .performance-review-table {
    width: 100%;
    min-width: auto;
  }
}

@media screen and (max-width: 768px) {
  .header-section {
    flex-direction: column;
    align-items: flex-start;
    gap: 15px;
  }
  
  .action-buttons {
    width: 100%;
    justify-content: flex-start;
  }
  
  .plan-col {
    flex: 0 0 50% !important;
    max-width: 50% !important;
    margin-bottom: 15px;
  }
}
</style>
