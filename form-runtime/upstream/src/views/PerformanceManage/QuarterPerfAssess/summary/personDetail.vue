<template>
    <div class="task-management">
        <el-button type="primary" size="mini" icon="el-icon-back" @click="_=>$router.push({name:'myQuarterlySummaryIndex'})" class="back-button">返回</el-button>
        <!-- 目标计划详情 -->
        <div class="target-header">
            <div class="target-header__item target-header__item--green">
                <div class="target-header__title">年度目标及完成进度</div>
                <div class="target-header__desc">{{ detail.ultimateAchieve }}</div>
                <i class="el-icon-zoom-in" title="放大查看" style="position:absolute;right:2px;top:2px;cursor:pointer;font-size:large;"
                 @click.stop="showBigInput({data: detail, value: detail.ultimateAchieve, key: 'ultimateAchieve', auth: ''})" ></i>
                <!-- <div class="target-header__desc">进度管理，完成目标计划的开发上线，上线率达到90%及以上</div> -->
            </div>
            <div class="target-header__item target-header__item--blue">
                <div class="target-header__title">目标</div>
                <div class="target-header__desc">{{ detail.phasedAchieve }}</div>
                <i class="el-icon-zoom-in" title="放大查看" style="position:absolute;right:2px;top:2px;cursor:pointer;font-size:large;"
                 @click.stop="showBigInput({data: detail, value: detail.phasedAchieve, key: 'phasedAchieve', auth: ''})" ></i>
                <!-- <div class="target-header__desc">上线率达到90%及以上</div> -->
            </div>
            <div class="target-header__item target-header__item--darkblue">
                <div class="target-header__title">目标界定、定义等</div>
                <div class="target-header__desc">{{ detail.appraisalMethod }}</div>
                <i class="el-icon-zoom-in" title="放大查看" style="position:absolute;right:2px;top:2px;cursor:pointer;font-size:large;"
                 @click.stop="showBigInput({data: detail, value: detail.appraisalMethod, key: 'appraisalMethod', auth: ''})" ></i>
                <!-- <div class="target-header__desc">实际上线功能数/计划上线总数*100%</div> -->
            </div>
            <div class="target-header__item target-header__item--gray">
                <div class="target-header__title">目标值</div>
                <div class="target-header__desc">
                  挑战值：{{ showTargetItem('high_difficulty') }}<br>
                  达标值：{{ showTargetItem('intermediate_difficulty')}}<br>
                  底限值：{{ showTargetItem('easy')}}</div>
            </div>
            <div class="target-header__item target-header__item--yellow">
                <div class="target-header__title">权重</div>
                <div class="target-header__desc">{{ detail.weight * 100 }}%</div>
            </div>
        </div>

        <!-- 任务列表（做什么 + 成果） -->
        <div class="task-section">
            <div class="task-section__header">
                <span>做什么</span>
                <span>达成什么结果</span>
                <span>完成日期</span>
                <!-- <span>成果</span> -->
            </div>
            <div class="task-section__item" v-for="(item, index) in detail.items" :key="index">
                <div class="task-section__item-col">{{ item.doWhat }}</div>
                <div class="task-section__item-col">{{ item.result }}</div>
                <div class="task-section__item-col">{{ item.endTime }}</div>
                <!-- <div class="task-section__item-col">
                    <template v-if="item.file">
                        <a href="javascript:;" class="file-link">{{ item.file }} 查看</a>
                        <el-button type="primary" size="mini" style="margin-left: 8px;">
                            提交成果文件
                        </el-button>
                    </template>
                    <template v-else>
                        <el-button type="primary" size="mini">提交成果文件</el-button>
                    </template>
                </div> -->
            </div>
        </div>

        <!-- 任务信息统计（我的任务 + 我下发的任务） -->
        <div class="task-stats">
            <div class="task-stats__block" style="margin-left:0;margin-right:5px">
                <div class="task-stats__title">任务完成情况</div>
                <div class="task-stats__items">
                    <div class="task-stats__item task-stats__item--green">
                        <div class="task-stats__item-number">{{ LedgerData.myPlanTotal }}</div>
                        <div class="task-stats__item-label">总任务</div>
                    </div>
                    <div class="task-stats__item task-stats__item--yellow">
                        <div class="task-stats__item-number">{{ LedgerData.myNotSubmitted }}</div>
                        <div class="task-stats__item-label">未提交</div>
                    </div>
                    <div class="task-stats__item task-stats__item--blue">
                        <div class="task-stats__item-number">{{ LedgerData.myDone }}</div>
                        <div class="task-stats__item-label">已完成</div>
                    </div>
                    <div class="task-stats__item task-stats__item--orange">
                      <div class="task-stats__item-number">{{ LedgerData.myUnreviewed }}</div>
                      <div class="task-stats__item-label">未审核</div>
                    </div>
                    <div class="task-stats__item task-stats__item--red">
                        <div class="task-stats__item-number">{{ LedgerData.myPlanTimeout }}</div>
                        <div class="task-stats__item-label">超时</div>
                    </div>
                </div>
            </div>
            <div class="task-stats__block" style="margin-right:0;margin-left:5px">
                <div class="task-stats__title">下发任务完成情况</div>
                <div class="task-stats__items">
                    <div class="task-stats__item task-stats__item--green">
                        <div class="task-stats__item-number">{{ LedgerData.userPlanTotal }}</div>
                        <div class="task-stats__item-label">总任务</div>
                    </div>
                    <div class="task-stats__item task-stats__item--yellow">
                        <div class="task-stats__item-number">{{ LedgerData.userNotSubmitted }}</div>
                        <div class="task-stats__item-label">未提交</div>
                    </div>
                    <div class="task-stats__item task-stats__item--blue">
                        <div class="task-stats__item-number">{{ LedgerData.userDone }}</div>
                        <div class="task-stats__item-label">已完成</div>
                    </div>
                    <div class="task-stats__item task-stats__item--orange">
                      <div class="task-stats__item-number">{{ LedgerData.userUnreviewed }}</div>
                      <div class="task-stats__item-label">未审核</div>
                    </div>
                    <div class="task-stats__item task-stats__item--red">
                        <div class="task-stats__item-number">{{ LedgerData.userPlanTimeout }}</div>
                        <div class="task-stats__item-label">超时</div>
                    </div>
                </div>
            </div>
        </div>
        <div style="width:100%;display:grid;grid-template-columns:1fr 1fr;grid-gap:10px;">
            <div style="min-width:0">
                <!-- 我的任务表格 -->
                <div class="task-table" style="min-height: 300px;">
                    <div class="task-table__header">我的任务</div>
                    <!-- <el-table :data="myTasks" border style="width: 100%" :fit="true">
                        <el-table-column prop="name" label="任务名称"></el-table-column>
                        <el-table-column prop="issueTime" label="下发时间"></el-table-column>
                        <el-table-column prop="deadline" label="截止时间"></el-table-column>
                        <el-table-column prop="auditor" label="审核人"></el-table-column>
                        <el-table-column label="状态">
                            <template #default="scope">
                                <span :style="{ color: scope.row.status === '超时未完成' ? 'red' : '' }">
                                    {{ scope.row.status }}
                                </span>
                            </template>
                        </el-table-column>
                        <el-table-column prop="auditStatus" label="审核状态"></el-table-column>
                    </el-table> -->
                    <el-table :data="myTasks" border style="width: 100%" :fit="true" max-height="400">
                        <el-table-column prop="name" label="任务名称"></el-table-column>
                        <el-table-column prop="createDate" label="下发时间"></el-table-column>
                        <el-table-column prop="endTime" label="截止时间"></el-table-column>
                        <el-table-column prop="user.name" label="接收人"></el-table-column>
                        <el-table-column prop="examiner.name" label="审核人"></el-table-column>
                        <el-table-column label="审核状态">
                          <template #default="scope">
                            <span>
                              {{ taskStatusNames[scope.row.task.taskStatus] }}
                            </span>
                          </template>
                        </el-table-column>
                        <el-table-column prop="auditStatus" label="完成状态">
                          <template #default="scope">
                            <span :style="{ color: scope.row.finishStatus === 'overtime_not_finish' ? 'red' : '' }">
                              {{ finishStatusNames[scope.row.finishStatus] }}
                            </span>
                          </template>
                        </el-table-column>
                    </el-table>
                </div>
            </div>
            <div style="min-width:0">
                <!-- 我下发的任务表格 -->
                <div class="task-table" style="min-height: 300px;">
                    <div class="task-table__header">
                        <span>我下发的任务</span>
                        <el-button type="primary" size="mini" style="float: right;" @click="addTaskInner">
                            下发任务
                        </el-button>
                    </div>
                    <el-table :data="issuedTasks" border style="width: 100%" :fit="true" max-height="400">
                        <el-table-column prop="name" label="任务名称"></el-table-column>
                        <el-table-column prop="createDate" label="下发时间"></el-table-column>
                        <el-table-column prop="endTime" label="截止时间"></el-table-column>
                        <el-table-column prop="user.name" label="接收人"></el-table-column>
                        <el-table-column prop="examiner.name" label="审核人"></el-table-column>
                        <el-table-column label="审核状态">
                          <template #default="scope">
                            <span>
                              {{ taskStatusNames[scope.row.task.taskStatus] }}
                            </span>
                          </template>
                        </el-table-column>
                        <el-table-column prop="auditStatus" label="完成状态">
                          <template #default="scope">
                            <span :style="{ color: scope.row.finishStatus === 'overtime_not_finish' ? 'red' : '' }">
                              {{ finishStatusNames[scope.row.finishStatus] }}
                            </span>
                          </template>
                        </el-table-column>
                        <!-- <el-table-column label="操作">
                            <template #default="scope">
                                <el-button size="mini" type="text">{{ scope.row.operation }}</el-button>
                            </template>
                        </el-table-column> -->
                    </el-table>
                </div>
            </div>
        </div>
      <!-- 任务下发弹窗 -->
    <AddTaskArrangeDialog v-if="AddArrangeDialogVisible" arrangeType="add" :visible.sync="AddArrangeDialogVisible"
      :dataFromAssigned="rowData" @addIndicatorEvent="addIndicatorEvent" :fromAssign="3" />
      <el-dialog v-if="bigInputVisible" :visible="bigInputVisible" :title="''" :close-on-click-modal="true" width="80%"
      @close="bigInputVisible=false" append-to-body style="height:100%;">
      <div>
        <el-input :disabled="true" class="bigInputClass" type="textarea" :autosize="{minRows:10}" style="width:100%;"
         v-model="bigInputData.value" show-word-limit maxlength="5000"></el-input>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="bigInputVisible=false" type="primary">关闭</el-button>
      </div>
    </el-dialog>
    </div>
</template>

<script>
import AddTaskArrangeDialog from '@/views/TaskManage/TaskArrange/components/AddTaskArrangeDialog';
var taskStatusNames = { waiting_send: '待提交', pending: '待审核', done: '已通过', null: '', undefined: '' };
var finishStatusNames = { finish: '完成', not_finish: '未完成', early_finish: '提前完成', overtime_finish: '超时完成', overtime_not_finish: '超时未完成', withdraw: '已撤销' };
export default {
  name: 'TaskManagement',
  props: ['currentPlanRow'],
  components: {
    AddTaskArrangeDialog
  },
  data() {
    return {
      bigInputData: { data: { v: '' }, value: '', key: 'v', auth: '' },
      bigInputVisible: false,
      LedgerData: {
        myPlanTotal: 0, // 我的总任务
        userPlanTotal: 0, // 我指派的总任务
        myDone: 0, // 我已完成的任务
        userDone: 0, // 我指派完成的任务
        myNotSubmitted: 0, // 我未提交审核的任务
        userNotSubmitted: 0, // 我指派未审核的任务
        myUnreviewed: 0, // 我未审核的任务
        userUnreviewed: 0, // 我的任务未被审核
        myPlanTimeout: 0, // 我超时的任务
        userPlanTimeout: 0 // 我指派已超时的任务
      },
      finishStatusNames,
      taskStatusNames,
      rowData: {},
      AddArrangeDialogVisible: false,
      detail: {},
      // 任务列表数据
      taskList: [
        { 
          do: '制定新版绩效考核功能模块的开发、测试、上线计划，以及开发人员的工作分配', 
          result: '输出原型和计划表', 
          date: '7月31日', 
          file: '原型.pdf' 
        },
        { 
          do: '制定集团经营数据看板第一期功能的开发、测试、上线计划，以及开发人员的工作分配', 
          result: '输出原型和计划表', 
          date: '7月31日', 
          file: '计划.pdf' 
        },
        { 
          do: '每周进行周例会，严格把控开发进度', 
          result: '解决开发过程中出现的问题，根据实际情况进行调控', 
          date: '每周进行', 
          file: '' 
        }
      ],
      // 我的任务数据
      myTasks: [
        // { 
        //   name: '制定开发工作计划A', 
        //   issueTime: '2025-05-28', 
        //   deadline: '2025-06-29', 
        //   auditor: '李四', 
        //   status: '超时未完成', 
        //   auditStatus: '审核中' 
        // },
        // { 
        //   name: '制定开发工作计划B', 
        //   issueTime: '2025-07-28', 
        //   deadline: '2025-08-29', 
        //   auditor: '李四', 
        //   status: '未完成', 
        //   auditStatus: '待提交' 
        // },
        // { 
        //   name: '制定开发工作计划C', 
        //   issueTime: '2025-07-28', 
        //   deadline: '2025-08-29', 
        //   auditor: '李四', 
        //   status: '未完成', 
        //   auditStatus: '审核中' 
        // }
      ],
      // 我下发的任务数据
      issuedTasks: [
        // { 
        //   name: '开发工作A', 
        //   issueTime: '2025-05-28', 
        //   deadline: '2025-06-29', 
        //   receiver: '王五', 
        //   status: '超时未完成', 
        //   auditStatus: '审核中', 
        //   operation: '审核' 
        // },
        // { 
        //   name: '开发工作B', 
        //   issueTime: '2025-07-28', 
        //   deadline: '2025-08-29', 
        //   receiver: '王五', 
        //   status: '未完成', 
        //   auditStatus: '审核中', 
        //   operation: '-' 
        // },
        // { 
        //   name: '开发工作C', 
        //   issueTime: '2025-07-28', 
        //   deadline: '2025-08-29', 
        //   receiver: '王五', 
        //   status: '未完成', 
        //   auditStatus: '审核中',
        //   operation: '审核' 
        // }
      ]
    }
  },
  created() {
    this.detail = JSON.parse(sessionStorage.getItem('currentPlanRow'));
    this.getMyTasks();
    this.getPlanLedger();
    this.getSubmitTasks();
  },
  mounted() {
    window.abb = this;

  },
  beforeDestroy() {
    // sessionStorage.removeItem('currentPlanRow');
  },
  methods: {
    showBigInput(data) {
      this.bigInputData = data;
      this.bigInputVisible = true;
    },
    addIndicatorEvent() {
      this.AddArrangeDialogVisible = false;
      this.getSubmitTasks();
      this.getPlanLedger();
    },
    addTaskInner() {
      var row = this.detail;
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
    getPlanLedger() {
      const param = {
        data: {
          planBusiness: {
            planBusinessType: 'work_plan',
            businessId: this.detail.id
          }
        }
      };
      this.$axios.post('/web/plan/api/targetTypePlan/getPlanLedger', param,
        res => {
          if (res.isSuccess && res.data) {
            this.LedgerData = res.data;
          } else {
            // this.currentPlans = [];
          }
        }
      );
    },
    getMyTasks() {
      const param = {
        data: {
          planBusiness: {
            planBusinessType: 'work_plan',
            businessId: this.detail.id
          }
        },
        pagination: true,
        current: 1,
        size: 100
      };
      this.$axios.post('/web/plan/api/targetTypePlan/getPlanByUser', param,
        res => {
          if (res.isSuccess && res.data) {
            this.myTasks = res.data;
          } else {
            // this.currentPlans = [];
          }
        }
      );
    },
    getSubmitTasks() {
      const param = {
        data: {
          planBusiness: {
            planBusinessType: 'work_plan',
            businessId: this.detail.id
          }
        },
        pagination: true,
        current: 1,
        size: 100
      };
      this.$axios.post('/web/plan/api/targetTypePlan/getMyIssuePlan', param,
        res => {
          if (res.isSuccess && res.data) {
            this.issuedTasks = res.data;
          } else {
            // this.currentPlans = [];
          }
        }
      );
    },
    showTargetItem(type) {
      if (this.detail && this.detail.values) {
        return this.detail.values.find(item => item.workPlanTargetType == type)?.remark || '-';
      }
    },
  }
};
</script>
<style scoped>
/* 目标计划详情 */
.back-button {
  margin: 5px 10px;
  position: sticky;
  top: 0;
  z-index: 2;
}
.target-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 20px;
}
.target-header__item {
  flex: 1;
  margin: 0 10px;
  padding: 16px;
  border-radius: 8px;
  color: #fff;
  position: relative;
}
.target-header__item--green { background: #87E8DE; }
.target-header__item--blue { background: #66B1FF; }
.target-header__item--darkblue { background: #409EFF; }
.target-header__item--gray { background: #C0C4CC; }
.target-header__item--yellow { background: #FFD04B; }
.target-header__title {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 8px;
}
.target-header__desc {
  font-size: 14px;
  line-height: 1.4;
  max-height: 160px;
  overflow: auto;
}
.target-header__desc::-webkit-scrollbar {
  width: 5px;
}
.target-header__desc::-webkit-scrollbar-track {
  /* background: #f5f7fa; */
  /* border-radius: 4px; */
}

.target-header__desc::-webkit-scrollbar-thumb {
  background: #c0c4cc;
  border-radius: 5px;
}

.target-header__desc::-webkit-scrollbar-thumb:hover {
  background: #909399;
}

/* 任务列表 */
.task-section {
  background: #F8F9FA;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 20px;
}
.task-section__header {
  display: flex;
  justify-content: space-between;
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 12px;
  color: #333;
  span {
    flex: 1;
    margin: 0 8px;
  }
}
.task-section__item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  padding: 12px 0px;
  border-radius: 4px;
  margin-bottom: 8px;
}
.task-section__item-col {
  flex: 1;
  margin: 0 8px;
  font-size: 14px;
  color: #666;
}
.file-link {
  color: #409EFF;
  text-decoration: underline;
}

/* 任务信息统计 */
.task-stats {
  display: flex;
  justify-content: space-between;
  margin-bottom: 20px;
}
.task-stats__block {
  flex: 1;
  margin: 0 10px;
  background: #F8F9FA;
  padding: 16px;
  border-radius: 8px;
}
.task-stats__title {
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 12px;
  color: #333;
}
.task-stats__items {
  display: flex;
  justify-content: space-around;
}
.task-stats__item {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  color: #fff;
}
.task-stats__item--green { background: #87E8DE; }
.task-stats__item--yellow { background: #FFD04B; }
.task-stats__item--blue { background: #66B1FF; }
.task-stats__item--red { background: #FF6B6B; }
.task-stats__item--orange { background: #FF9F43; }
.task-stats__item-number {
  font-size: 20px;
  font-weight: bold;
}
.task-stats__item-label {
  font-size: 12px;
}

/* 任务表格 */
.task-table {
  background: #F8F9FA;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 20px;
}
.task-table__header {
  height: 24px;
  font-size: 16px;
  font-weight: bold;
  margin-bottom: 12px;
  color: #333;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.el-table {
  background: #fff;
}
</style>