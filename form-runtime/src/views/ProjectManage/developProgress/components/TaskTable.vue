<!--
 * @Descripttion: 开发进度查看任务-表格
 * @Author: zhengzetao
 * @Date: 2021-11-22
-->
<template>
  <div class="container">
    <dy-table :fetchData="fetchData" :keys="myTaskColKey" :actions="myTaskActions" :list="myTaskList"
      :pagination="pagination" :isPagination="true"></dy-table>
    <TaskDetailDialog :visible.sync="taskDetailDialogVisible" v-if="taskDetailDialogVisible" :detailId="detailId" />
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import TaskDetailDialog from './TaskDetailDialog';

export default {
  name: '',
  components: { DyTable, TaskDetailDialog },
  props: {
    planId: {
      type: String,
      default: ''
    },
    taskType: {
      type: String,
      default: ''
    },
    from: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      myTaskActions: [
        {
          label: '查看',
          action: (row) => {
            this.detailId = row.id;
            this.arrangeType = 'audit';
            this.taskDetailDialogVisible = true;
            // this.goDepartmentFramework(row);
          }
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
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
      },
      taskDetailDialogVisible: false,
      arrangeType: '',
      detailId: ''
    };
  },
  computed: {},
  watch: {
  },
  created() { },
  mounted() {
  },
  methods: {
    fetchData() {
      const param = {
        data: {
          planExamineStatus: this.taskType,
          id: this.planId
        },
        pagination: true,
        current: this.pagination.pages ? this.pagination.pages : 1,
        size: this.pagination.size ? this.pagination.size : 10
      };
      let apiUrl = Api.developProgress.getPlanByBusinessIdAndFinishStatus;
      if (this.$attrs) {
        if (this.from == 'targetView') { // 来自目标责任书的引用
          apiUrl = this.$attrs.fetchUrl;
          param.data = {
            kpi: {
              id: this.planId
            },
            planExamineStatus: this.taskType
          };
        }
      }
      // apiUrl='/web/plan/api/targetTypePlan/getKpiRelationPlan'
      // console.log('apiUrl',apiUrl)
      this.$axios.post(
        apiUrl,
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
    }
  }
};
</script>

<style scoped lang="scss">
// 任务完成背景色
::v-deep .el-table .style-common {
  color: #fff;
  padding: 2px 6px;
  border-radius: 16px;
}
</style>
