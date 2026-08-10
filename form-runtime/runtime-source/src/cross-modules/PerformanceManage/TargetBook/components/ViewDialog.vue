<template>
  <el-dialog :title="title" :visible="visible" :close-on-click-modal="false" @close='handleClose' @open="handleOpen"
    width="80%">
    <el-tabs type="border-card" v-model="activeName" @tab-click="handleClick">
      <el-tab-pane :name="item.name" v-for="(item, index) in tabList" :key="index">
        <span slot="label"> {{ item.label }}<i :style="{ 'color': item.color }" style="font-style: normal;">({{
          item.number
        }})</i></span>
        <div class="container">
          <dy-table :keys="myTaskColKey" :list="myTaskList" :fetchData="() => { }"></dy-table>
          <el-pagination background :total="pagination.total" :page-size="pagination.size"
            layout="total, prev, pager, next" @size-change="handlePageSize" @current-change="pageChange"
            style="text-align:right; ">
          </el-pagination>
        </div>
      </el-tab-pane>
    </el-tabs>
    <div slot="footer" class="dialog-footer">
      <!-- <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="submitForm('addTypeForm')"
      >确 定</el-button> -->
    </div>
  </el-dialog>
</template>

<script>
import DyTable from '@/components/DyTable';
// import TaskTable from './TaskTable';
import Api from '@/api';
export default {
  name: 'ViewDialog',
  components: { DyTable },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    kpiId: {
      type: String,
      default: ''
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
      activeName: 'done',
      title: '查看完成情况详情',
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
  methods: {
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
