<!-- 关联任务 -->
<template>
  <el-dialog title="关联任务" :visible="visible" :close-on-click-modal="false" @close='handleClose' @open="handleOpen"
    width="80%" append-to-body>
    <div style="height:70vh">
      <el-tabs v-model="activeName" @tab-click="onTabClick">
        <el-tab-pane name="self" label="我的任务"></el-tab-pane>
        <el-tab-pane :name="item.id" v-for="(item, index) in tabList" :key="item.id">
          <span slot="label"> {{ item.name }}的任务</span>
        </el-tab-pane>
      </el-tabs>
      <dy-table :fetchData="getTaskList" :keys="projectDepartColKey" :list="myTaskList" :pagination="pagination"
        :isPagination="true" :showCheckBox="true" ref="table" @handleSelect="handleSelect"
        @handleSelectAll="handleSelectAll" :height="'45vh'"></dy-table>
      <el-tag v-for="val in selectList" :closable="!val.sys" @close="handleTagClose(val.id)" style="margin-right: 2px;">{{ val.name
        }}</el-tag>
      <div slot="footer" class="dialog-footer" style="text-align: center;margin-top: 15px;">
        <el-button @click="handleClose">取 消</el-button>
        <el-button
          type="primary"
          @click="confirm"
        >确 定</el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script>
import moment from 'moment';
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth.js';
import { deepClone } from '@/utils';
const pointTabObj =
{
  ordinary: {
    value: '普通', type: 'primary'
  },
  urgent: {
    value: '较高级', type: 'warning'
  },
  milestone: {
    value: '紧急', type: 'danger'
  },

}
export default {
  name: 'relateTaskDialog',
  components: { DyTable },
  props: ['visible', 'relateRow','currentMonth'],
  data() {
    return {
      activeName: 'self',
      tabList: [],
      myTaskList:[],
      pagination: {
        pages: 1,
        size: 10
      },
      planList: [],
      selectList:[],
      projectDepartColKey: {
        name: {
          label: '任务名称',
          showTooltip: true,
          // handle: function (scope, createElement) {
          //   return createElement('el-link',scope.row.name);
          // }
        },
        createDate: {
          label: '下发时间',
          width: '90px',
          showTooltip: true,
          handle: function (scope, createElement) {
            return createElement('span', moment(scope.row.createDate).format('YYYY-MM-DD'));
          }
        },
        pointTab: {
          label: '优先级',
          width: '90px',
          handle: function (scope, createElement) {
            let pointTab = scope.row.pointTab
            return createElement('el-tag',
              {
                props: {
                  type: pointTabObj[pointTab].type
                }
              },
              pointTabObj[pointTab].value);
          }
        },
        project: {
          label: '关联项目',
          showTooltip: true,
          handle: function (scope, createElement) {
            if (scope.row.project) {
              return createElement('span', scope.row.project.name);
            } else {
              return createElement('');
            }
          }
        },
        business: {
          label: '关联业务',
          toolTipContent: (scope) => {
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
              scope.row.planBusinessList.forEach(x => {
                const businessItem = businessList.find(y => y.tag == x.planBusinessType);
                businessPlanName += (`${businessItem.name} / ${x.assembledName}<br/>`);
              });
              return businessPlanName;
            } else {
              return '无';
            }
          }
        },
        endTime: {
          label: '截止时间',
          width: '90px',
          showTooltip: true,
          handle: function (scope, createElement) {
            return createElement('span', moment(scope.row.endTime).format('YYYY-MM-DD'));
          }
        },
        creater: {
          label: '发送人',
          width: '80px',
          handle: function (scope, createElement) {
            if (scope.row.creator) {
              return createElement('span', scope.row.creator.name);
            } else {
              return createElement('');
            }
          }
        },
        // haveWorkTarget: {
        //   label: '工作计划',
        //   handle: function (scope, createElement) {
        //     return createElement('span', scope.row.haveWorkPlanItem ? '有' : '无');
        //   }
        // },
        taskStatus: {
          label: '审核状态',
          width: '90px',
          handle: function (scope, createElement) {
            const type = scope.row.task.taskStatus;
            if (type == 'waiting_send') {
              return createElement('span', { class: 'bg-running style-common' }, '待提交');
            } else if (type == 'pending' || type == 'has_been_sent_withdraw') {
              return createElement('span', { class: 'bg-willchecked style-common' }, '已提交');
            } else if (type == 'done') {
              return createElement('span', { class: 'bg-finished style-common' }, '已通过');
            } else if (type == 'withdraw') {
              return createElement('span', { class: 'bg-withdraw style-common' }, '已撤销');
            }
          }
        },
        finishStatus: {
          label: '完成状态',
          width: '100px',
          handle: function (scope, createElement) {
            let status = '';
            if (scope.row.finishStatus == 'finish') {
              status = '完成';
            } else if (scope.row.finishStatus == 'not_finish') {
              status = '未完成';
            } else if (scope.row.finishStatus == 'early_finish') {
              status = '提前完成';
            } else if (scope.row.finishStatus == 'overtime_finish') {
              status = '超时完成';
            } else if (scope.row.finishStatus == 'overtime_not_finish') {
              status = '超时未完成';
            } else if (scope.row.finishStatus == 'withdraw') {
              status = '已撤销';
            }
            return createElement('span', status);
          }
        }
      },
    };
  },
  created() { },
  mounted() { },
  watch: {},
  computed: {},
  methods: {
    dataInit(){
      this.activeName = 'self'
      this.tabList=[]
      this.myTaskList=[]
      this.pagination={
        pages: 1,
        size: 10
      },
      this.planList= []
      this.selectList=[]
    },
    confirm(){
      this.$emit('selectRelativeTask',this.selectList)
      this.handleClose()
    },
    onTabClick() {
      // this.selectList = []
      this.pagination.pages = 1
      this.getTaskList()
    },
    handleTagClose(id) {
      // handleToggleRowSelection
      this.$confirm('确认取消？', '提示').then(() => {
        let index = this.selectList.findIndex(el => el.id == id)
        if (index > -1) {
          let find = this.myTaskList.find(item => item.id == this.selectList[index].id)
          if (find) this.$refs.table.$refs.multipleTable.toggleRowSelection(find,false)
          this.$nextTick(() => {
            this.selectList.splice(index, 1)
          })
        }
      }).catch(() => { })
    },
    handleSelect(row) {
      let index = this.selectList.findIndex(item => item.id == row.id)
      //已经存在列表，说明当前操作时取消选择
      if (index > -1) {
        this.selectList.splice(index, 1)
      } else {
        //列表中没有，说明当前操作时添加
        this.selectList.push(row)
      }
    },
    handleSelectAll(selection) {
      if (selection.length) {
        //全选选中
        // let index = selection.findIndex(item=>item.id == row.id)
        selection.forEach(row => {
          let index = this.selectList.findIndex(item => item.id == row.id)
          if (index <= -1) this.selectList.push(row)
        })
      } else {
        //全选取消，用当前页面的数据去selectList删除
        this.myTaskList.forEach(row => {
          let index = this.selectList.findIndex(item => item.id == row.id)
          if (index >= -1) this.selectList.splice(index, 1)
        })
      }
    },
    handleClose() {
      this.$emit('update:visible', false)
      this.dataInit()
    },
    handleOpen() {
      let defaultMyTaskList = this.relateRow?.plans || []
      // if(defaultMyTaskList.length){
      //   defaultMyTaskList[0].sys = true
      // } //TODO 测试用，需要删除
      // defaultMyTaskList.forEach(item=>{
      //   item.selectable = true
      //   if(item.sys)item.selectable = false
      // })
      // console.log('defaultMyTaskList',defaultMyTaskList)

      this.selectList = defaultMyTaskList
      let data = {
        id: localstorageGet('userId'),
      }
      this.$axios.post(Api.performance.getSubordinateUser, { data }, res => {
        if (res.isSuccess) {
          this.tabList = res?.data || []
          this.getTaskList()
        }
      })
    },
    getTaskList() {
      let param = {
        data:{},
        pagination: true,
        current: this.pagination.pages ? this.pagination.pages : 1,
        size: this.pagination.size ? this.pagination.size : 10
      }
      if (this.activeName != "self") {
        param.data = {
          user: {
            id: this.activeName
          }
        }
      }
      let queryStartTime,queryEndTime
      if(this.currentMonth){
        let tempArr = this.currentMonth.split('-')
        let selectMonth = tempArr[1],selectYear = tempArr[0]
        let startYear,startMonth ,startDay = '29'
        let endYear,endMonth,endDay = '28'
        endYear = selectYear
        endMonth = selectMonth
        if(selectMonth == 1){
          startYear =selectYear - 1
          startMonth = 12
        }else{
          startYear = selectYear
          startMonth = selectMonth -1
        }
        queryStartTime = startYear+'-'+('0'+startMonth).substr(-2,2)+'-'+ ('0'+startDay).substr(-2,2)
        queryEndTime = endYear+'-'+('0'+endMonth).substr(-2,2)+'-'+ ('0'+endDay).substr(-2,2)
      }
      if(queryStartTime)param.data.queryStartTime = queryStartTime
      if(queryEndTime)param.data.queryEndTime = queryEndTime
      this.$axios.post(Api.performance.getPlanByUser, param, res => {
        if (res.isSuccess) {
          this.myTaskList = res?.data || []
          this.pagination = {
            total: res.total,
            size: res.size,
            pages: res.current
          }
          this.$nextTick(() => {
            this.selectList.forEach(row => {
              let find = this.myTaskList.find(item => item.id == row.id)
              if(row.sys)find.selectable = false
              if (find) this.$refs.table.handleToggleRowSelection(find)
            })
          })
        }
      })
    }
  },
};
</script>
<style lang="scss" scoped>
::v-deep .el-table .style-common {
  color: #fff;
  padding: 2px 6px;
  border-radius: 16px;
}
</style>
