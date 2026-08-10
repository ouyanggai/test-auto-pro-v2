<template>
   <el-dialog center v-if="visible" :visible.sync="visible" fullscreen append-to-body :title="viewTitle[type]" @close='handleClose' custom-class="reset-padding-dialog">
      <div class="dialog-container" style="height: calc(100vh - 115px);">
        <div class="container" v-if="type === 'publicity'">
          <div class="section-header">
            <span style="margin-left:auto" >
              <el-date-picker v-model="year1" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="getPublicityList"/>
            </span>
          </div>
          <el-table
            :data="publicityList"
            :max-height="mHeight"
            class="content-table"
          >
            <el-table-column prop="planUserGroupName" label="分组名称" mwd="120">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewPerformanceReview(scope.row)">{{ scope.row.planUserGroupName }}</el-button>
              </template>
            </el-table-column>
            <el-table-column prop="targetTime" label="时间" mwd="180">
              <template slot-scope="scope">
                {{ showPlanName(scope.row) }}
              </template>
            </el-table-column>
            <el-table-column prop="publishStatus" label="公示状态" mwd="120">
              <template slot-scope="scope">
                <el-tag :type="publicityColor[scope.row.noticeStatus]">{{ publicity[scope.row.noticeStatus] || '-' }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination background layout="total, sizes, prev, pager, next" :total="page2.total" style="float:right;text-align:right;" :page-size="page2.size" :current-page="page2.current"
              @size-change="handleSizeChange2" @current-change="handleCurrentChange2"></el-pagination>
        </div>
        <div class="container" v-if="type === 'plan'">
          <div slot="header" class="section-header">
            <span style="margin-left:auto" >
              <el-date-picker v-model="year2" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
              value-format="yyyy" format="yyyy" @change="_=>{page1.current=1;getQuarterPlans()}"/>
              <el-select placeholder="季度" ref="quarterPicker" style="width:90px;margin-right:10px" v-model="quarter2" class="font-large" @change="_=>{page1.current=1;getQuarterPlans()}" clearable>
                <el-option v-for="item in [{ label: '一季度', value: 1 }, { label: '二季度', value: 2 }, { label: '三季度', value: 3 }, { label: '四季度', value: 4 }]"
                :key="item.value" :label="item.label" :value="item.value"></el-option>
              </el-select>
              <el-input style="width:120px;margin-right:10px" placeholder="公司" clearable v-model="quarterPlansSearch.companyName"></el-input>
              <el-input style="width:110px;margin-right:10px" placeholder="部门" clearable v-model="quarterPlansSearch.depName"></el-input>
              <el-input style="width:110px;margin-right:10px" placeholder="姓名" clearable v-model="quarterPlansSearch.userName"></el-input>
              <el-button type="primary" size="mini" @click="_=>{page1.current=1;getQuarterPlans()}">查询</el-button>
            </span>
          </div>
          <el-table
            :data="quarterPlans"
            :max-height="mHeight"
            class="content-table"
          >
            <el-table-column prop="name" label="计划时间" min-width="100">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewQuarterPlan(scope.row)">{{ showPlanName(scope.row) }}</el-button>
              </template>
            </el-table-column>
            <el-table-column prop="userName" label="姓名" mwd="120"></el-table-column>
            <el-table-column prop="depName" label="部门" mwd="120"></el-table-column>
            <el-table-column prop="dutyName" label="岗位" mwd="120"></el-table-column>
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
          </el-table>
          <el-pagination background layout="total, sizes, prev, pager, next" :total="page1.total" style="float:right;" :page-size="page1.size" :current-page="page1.current"
            @size-change="handleSizeChange" @current-change="handleCurrentChange"></el-pagination>
        </div>
        <div class="container" v-if="type === 'performance'">
          <div slot="header" class="section-header">
          <span style="margin-left:auto" >
            <el-date-picker v-model="year3" type="year" placeholder="选择年" style="width:99px;margin-right:10px" :clearable="false"
            value-format="yyyy" format="yyyy" @change="_=>{page3.current=1;getPerformance()}"/>
            <el-select placeholder="季度" ref="quarterPicker" style="width:90px;margin-right:10px" v-model="quarter3" class="font-large" @change="_=>{page3.current=1;getPerformance()}" clearable>
              <el-option v-for="item in [{ label: '一季度', value: 1 }, { label: '二季度', value: 2 }, { label: '三季度', value: 3 }, { label: '四季度', value: 4 }]"
              :key="item.value" :label="item.label" :value="item.value"></el-option>
            </el-select>
            <el-input style="width:120px;margin-right:10px" placeholder="公司" clearable v-model="performanceSearch.companyName"></el-input>
            <el-input style="width:110px;margin-right:10px" placeholder="部门" clearable v-model="performanceSearch.depName"></el-input>
            <el-input style="width:110px;margin-right:10px" placeholder="姓名" clearable v-model="performanceSearch.userName"></el-input>
            <el-button type="primary" size="mini" @click="_=>{page3.current=1;getPerformance()}">查询</el-button>
            <!-- <el-button type="primary" size="mini" @click="_=>{}">导出</el-button> -->
          </span>
        </div>
        <el-table 
          :data="performanceReviews"
          :max-height="mHeight"
          class="content-table">
          <el-table-column prop="name" label="考核时间" mwd="180">
            <template slot-scope="scope">
              <el-button type="text" size="small" @click="viewPerformanceFlow(scope.row)">{{ showPlanName(scope.row) }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="userName" label="姓名" mwd="120"></el-table-column>
          <el-table-column prop="companyName" label="公司" mwd="120"></el-table-column>
          <el-table-column prop="depName" label="部门" mwd="120"></el-table-column>
          <el-table-column prop="dutyName" label="岗位" mwd="120"></el-table-column>
          <el-table-column prop="type" label="绩效类型" mwd="120">
            <template slot-scope="scope">
              {{ scope.row.kpi2Type == 'personal_kpi' ? '个人绩效' : '组织绩效' }}
            </template>
          </el-table-column>
          <el-table-column prop="totalScore" label="分数" mwd="80"></el-table-column>
          <el-table-column prop="totalKpi" label="岗效比">
            <template slot-scope="scope">
              {{ scope.row.totalKpi === null ? '-' : math.multiply(100, scope.row.totalKpi || 0) + '%' }}
            </template>
          </el-table-column>
          <el-table-column prop="ratingLevel" label="评级" mwd="80">
            <template slot-scope="scope">
              {{ scope.row.ratingLevel == 'none' ? '未评级' : levelEnums[scope.row.ratingLevel] }}
            </template>
          </el-table-column>
        </el-table>
        <el-pagination background layout="total, sizes, prev, pager, next" :total="page3.total" style="float:right;" :page-size="page3.size" :current-page="page3.current"
          @size-change="handleSizeChange3" @current-change="handleCurrentChange3"></el-pagination>
        </div>
      </div>
      <div slot="footer" class="dialog-footer" style="text-align: center;">
        <el-button @click="handleClose" type="primary">关 闭</el-button>
      </div>
    </el-dialog>
</template>

<script>
import math from '@/utils/math.js';
import DyTable from '@/components/DyTable';
// import TaskTable from './TaskTable';
import Api from '@/api';
var publicity = { not_started: '未公示', in_progress: '公示中', end: '公示结束' };
var publicityColor = { not_started: 'danger', in_progress: 'success', end: '' };
var ratingEnum = { not_submitted: '未提交', under_review: '审核中', pass: '已通过' };
var ratingColor = { not_submitted: 'danger', under_review: 'success', pass: '' };
export default {
  name: 'ViewDialog',
  components: { DyTable },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    type: {
      type: String,
      default: 'publicity'
    },
    finishedNum: {
      type: Number,
      default: 0
    },
    pendingReviewNum: {
      type: Number,
      default: 0
    },
    notSubmitNum: {
      type: Number,
      default: 0
    }
    // taskType: {
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      mHeight: '500',
      math,
      levelEnums: { 'level_a': 'A级', 'level_b': 'B级', 'level_c': 'C级', 'level_d': 'D级', 'level_e': 'E级' },
      performanceSearch: {
        companyName: '',
        depName: '',
        userName: '',
      },
      quarterPlansSearch: {
        companyName: '',
        depName: '',
        userName: '',
      },
      viewTitle: { plan: '工作计划', publicity: '计划公示', performance: '绩效考核结果'},
      year: new Date().getFullYear() + '',
      publicity,
      publicityColor,
      ratingEnum,
      ratingColor,
      year1: '',
      year2: '',
      year3: '',
      quarter2: '',
      quarter3: '',
      year4: '',
      quarter4: '',
      page1: {
        total: 0,
        current: 1,
        pages: 0,
        size: 50
      },
      page2: {
        total: 0,
        current: 1,
        pages: 0,
        size: 50
      },
      page3: {
        total: 0,
        current: 1,
        pages: 0,
        size: 50
      },
      publicityList: [],
      quarterPlans: [],
      performanceReviews: [],
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
      title: '计划公示',


      activeName: 'done',
      tabList: [
        {
          name: 'done',
          label: '已完成',
          number: 0,
          color: '#2FC25B',
          key: 'finishedNum'
        },
        {
          name: 'pending',
          label: '审核中',
          number: 0,
          color: '#223273',
          key: 'pendingReviewNum'
        },
        {
          name: 'waiting_send',
          label: '进行中',
          number: 0,
          color: '#ccc',
          key: 'notSubmitNum'
        }
      ],
      pagination: {
        total: 0,
        size: 10,
        currentPage: 1
      },
      myTaskList: [],
      myTaskColKey: {
        name: {
          label: '任务名称',
          showTooltip: true,
          handle: function (scope, createElement) {
            return createElement('span', scope.row.name);
          }
        },
        business: {
          label: '关联业务',
          showTooltip: true,
          handle: function (scope, createElement) {
            if (scope.row.planBusinessList) {
              const businessList = [
                {
                  name: '手续文件',
                  tag: 'prophase_procedures'
                },
                {
                  name: '管理进度',
                  tag: 'software_progress_plan'
                },
                {
                  name: '季度工作计划',
                  tag: 'work_plan'
                }
              ];
              let businessPlanName = '';
              businessList.map(item => {
                if (item.tag == scope.row.planBusinessList[0].planBusinessType) {
                  businessPlanName = item.name + '/' + scope.row.planBusinessList[0].assembledName;
                }
              });
              return createElement('span', businessPlanName);
            } else {
              return createElement('span', '无');
            }
          }
        },
        endTime: '截止时间',
        creater: {
          label: '审核人',
          handle: function (scope, createElement) {
            if (scope.row.creator) {
              return createElement('span', scope.row.creator.name);
            } else {
              return createElement('');
            }
          }
        },
        user: {
          label: '接收人',
          handle: function (scope, createElement) {
            if (scope.row.creator) {
              return createElement('span', scope.row.user.name);
            } else {
              return createElement('');
            }
          }
        },
        taskStatus: {
          label: '审核状态',
          handle: function (scope, createElement) {
            const type = scope.row.task.taskStatus;
            if (type == 'waiting_send') {
              return createElement('span', { class: 'bg-running style-common' }, '待提交');
            } else if (type == 'pending') {
              return createElement('span', { class: 'bg-willchecked style-common' }, '已提交');
            } else if (type == 'done') {
              return createElement('span', { class: 'bg-finished style-common' }, '已通过');
            }
          }
        },
        finishStatus: {
          label: '完成状态',
          handle: function (scope, createElement) {
            const type = scope.row.finishStatus;
            if (type == 'finish') {
              return createElement('span', '完成');
            } else if (type == 'not_finish') {
              return createElement('span', '未完成');
            } else if (type == 'early_finish') {
              return createElement('span', '提前完成');
            } else if (type == 'overtime_finish') {
              return createElement('span', '超时完成');
            } else if (type == 'overtime_not_finish') {
              return createElement('span', '超时未完成');
            }
          }
        }
      }
    };
  },
  computed: {},
  watch: {},
  updated() { },
  created() {
    this.mHeight = window.innerHeight - 175;
    const month = new Date().getMonth() + 1;
    const quarter = Math.ceil(month / 3);
    this.quarter = quarter;
    this.year = new Date().getFullYear() + '';
    this.year1 = this.year;
    this.year2 = this.year;
    this.year3 = this.year;
    this.year4 = this.year;
    this.quarter4 = quarter;
    this.fetchData();
  },
  methods: {
    viewQuarterPlan(row) {
      this.$fm.show("flowDetail", { data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }] }});
    },
    viewPerformanceFlow(row) {
      this.$fm.show("flowDetail", { data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }] }});
    },
    getQuarterPlans() {
      // console.log(this.formData.targetTime, 'targetTime')
      const param = {
        data: {
          kpiScope: 'my_company',
          companyName: this.quarterPlansSearch.companyName || '',
          depName: this.quarterPlansSearch.depName || '',
          userName: this.quarterPlansSearch.userName || '',
          targetTime: this.year2 + '' + this.quarter2,
          // targetTime: this.year2 ? this.year2 : this.formData.year + ''
        },
        pagination: true,
        current: this.page1.current,
        size: this.page1.size
      };
      this.$axios.post('/web/plan/api/workPlanGroup/list', param,
        res => {
          if (res.isSuccess) {
            this.quarterPlans = res.data || [];
            this.page1.total = res.total || 0;
            this.page1.pages = res.pages || 0;
          }
        }
      );
    },
    getPerformance() {
      const param = {
        data: {
          kpiScope: 'my_company',
          targetTime: this.year3 + '' + this.quarter3,
          userName: this.performanceSearch.userName || '',
          companyName: this.performanceSearch.companyName || '',
          depName: this.performanceSearch.depName || ''
        },
        pagination: true,
        current: this.page3.current,
        size: this.page3.size
      };
      this.$axios.post('/web/plan/api/kpi2Group/list', param,
        res => {
          if (res.isSuccess) {
            this.performanceReviews = res.data || [];
          } else {
            this.performanceReviews = [];
          }
          this.page3.total = res.total || 0;
          this.page3.pages = res.pages || 0;
        }
      );
    },
    getPublicityList() {
      this.$axios.post(Api.newPerformance.workPlanPromulgateList,
        {
          data: {
            companyId: '',
            noticeStatus: null,
            targetTime: this.year1 ? this.year1 : this.year, // new Date().getFullYear() + ''
          },
          pagination: true,
          current: this.page2.current,
          size: this.page2.size
        },
        res => {
          // this.tableData = [];
          // this.pagination.total = 0;
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              this.publicityList = res.data;
              // this.pagination.total = res.total;
            } else {
              this.publicityList = []
            }
            this.page2.total = res.total || 0;
            this.page2.pages = res.pages || 0;
          }
        }
      );
    },
    handleSizeChange(val) {
      this.page1.size = val;
      this.getQuarterPlans();
    },
    handleCurrentChange(val) {
      this.page1.current = val;
      this.getQuarterPlans();
    },
    handleSizeChange2(val) {
      this.page2.size = val;
      this.getPublicityList();
    },
    handleCurrentChange2(val) {
      this.page2.current = val;
      this.getPublicityList();
    },
    handleSizeChange3(val) {
      this.page3.size = val;
      this.getPerformance();
    },
    handleCurrentChange3(val) {
      this.page3.current = val;
      this.getPerformance();
    },
    showPlanName({targetTime}) {
      targetTime += '';
      console.log(targetTime, 'targetTime');
      const year = targetTime.substr(0, 4);
      const month = targetTime.substr(4, 2);
      return year + '年' + this.quarterEnums[month] + '季度';
    },
    viewPerformanceReview(row) {
      this.$parent.viewPerformanceReview(row);
    },
    handleClick() {
      this.fetchData();
    },
    handleOpen() {
      this.tabList.forEach(item => {
        const k = item.key;
        item.number = this[k];
      });
      this.pagination.currentPage = 1;
      this.fetchData();
    },
    fetchData() {
      if (this.type == 'publicity') {
        this.getPublicityList();
      } else if (this.type == 'performance') {
        this.getPerformance();
      } else if (this.type == 'plan') {
        this.getQuarterPlans();
      }
      return;
      const action = Api.taskManage.taskArrange.getTargetCompleteInfo;
      const id = this.kpiId;
      const param = {
        data: {
          kpi: {
            id
          },
          planExamineStatus: this.activeName
        },
        pagination: true,
        current: this.pagination.currentPage ? this.pagination.currentPage : 1,
        size: this.pagination.size ? this.pagination.size : 10
      };
      this.$axios.post(
        action,
        param,
        res => {
          if (res.isSuccess) {
            this.myTaskList = res.data ? res.data : [];
            this.pagination.total = res.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    fontColor(type) {
      return 'red';
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    handlePageSize(size) {
      this.pagination.pages = 1;
      this.pagination.size = size;
      this.fetchData();
    },
    pageChange(page) {
      this.pagination.currentPage = page;
      this.fetchData();
    }
  }
};
</script>

<style scoped lang="scss"></style>
