<!--
  **
  * @description:查看工作指标
  * @Author: 刘福泽
  * @Date: 2022-12-03 17:22:59
  **
-->
<template>
  <div class="target-book-container">
    <el-button type="primary" style="position:absolute;z-index:99;" icon="el-icon-back" @click="goback" v-if="frompage != 'cost'">返 回</el-button>
    <h3>{{ title }}</h3>
    <el-card class="box-card mt-10" v-if="type != 'month'">
      <el-form :model="form" ref="form" :inline="true" :disabled="true">
        <el-form-item label="姓名">
          <el-input v-model="form.name"></el-input>
        </el-form-item>
        <el-form-item label="部门">
          <el-input v-model="form.departmentName"></el-input>
        </el-form-item>
        <el-form-item label="主考核人" prop="examiner.name" v-if="type != 'month'">
          <el-input v-model="form.examiner.name" placeholder="请选择主考核人"></el-input>
        </el-form-item>
        <el-form-item label="考核周期" prop="assessmentCycle">
          <el-select v-model="form.assessmentCycle" placeholder="请选择">
            <el-option label="半年" value="half_year">
            </el-option>
            <el-option label="年终" value="year">
            </el-option>
            <el-option label="月度" value="month">
            </el-option>
            <el-option label="半年及年终" value="year_and_half_year">
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <el-table :data="form.keyPerformanceIndicatorsList" ref="table" border align="center" style="width: 100%"
        :row-class-name="tableRowClassName">
        <el-table-column label="序号" type="index" width="50">
        </el-table-column>
        <template v-if="type == 'workTarget'">
          <!-- 工作指标开始 -->
          <el-table-column label="目标项目(一级)" width="150">
            <template slot-scope="scope">
              <template>
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ indicatorsTypeName(scope.row.indicatorsType.id) }}
                </div>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="目标项目(二级)">
            <template slot-scope="scope">
              <template>
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ scope.row.targetItemTwo }}
                </div>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="具体目标项目内容">
            <template slot-scope="scope">
              <template>
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ scope.row.content }}
                </div>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="权重" width="80" align="center">
            <template slot-scope="scope">
              {{ scope.row.weight }}
            </template>
          </el-table-column>
          <el-table-column label="目标完成标准(考核标准)">
            <template slot-scope="scope">
              <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                {{ scope.row.assessmentMethod }}
                </div>
            </template>
          </el-table-column>
          <el-table-column label="目标完成时间节点" width="120" align="center">
            <template slot-scope="scope">
              <template>
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ scope.row.assessmentTime }}
                </div>
              </template>
            </template>
          </el-table-column>
          <!-- 工作指标结束 -->
        </template>
        <template v-else-if="type == 'manageTarget'">
          <!--管理目标开始-->
          <el-table-column label="目标项目" width="150">
            <template slot-scope="scope">
              <template>
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ indicatorsTypeName(scope.row.indicatorsType.id) }}
                </div>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="具体目标项目内容">
            <template slot-scope="scope">
              <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                {{ scope.row.content }}
                </div>
            </template>
          </el-table-column>
          <el-table-column label="权重" width="80" align="center">
            <template slot-scope="scope">
              {{ scope.row.weight }}
            </template>
          </el-table-column>
          <el-table-column label="目标完成标准(考核标准)">
            <template slot-scope="scope">
              <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                {{ scope.row.assessmentMethod }}
                </div>
            </template>
          </el-table-column>
          <el-table-column label="目标完成时间节点" width="120" align="center">
            <template slot-scope="scope">
              <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                {{ scope.row.assessmentTime }}
                </div>
            </template>
          </el-table-column>
          <!-- 管理目标结束 -->
        </template>
        <!-- <el-table-column label="完成情况" width="150">
          <template slot-scope="scope" v-if="scope.row.assessmentTime">
            <div v-for="(val, index) in statusKey" :key="index" style="margin-bottom:10px;">
              <el-tag :type="val['tag']">
                <span style="padding-right: 10px;">{{ val['name'] }}({{ scope.row[val['key']] }})</span>
              </el-tag>
            </div>
          </template>
        </el-table-column> -->
        <el-table-column label="完成情况描述" width="150">
          <template slot-scope="scope" >
            <div style="max-height: calc(23* 5px);overflow-y: scroll;">
              {{ scope.row.assessmentRemarks }}
                </div>
          </template>
        </el-table-column>
        <el-table-column
            label="得分"
            width="100"
            prop="score"
          >
        </el-table-column>
        <el-table-column label="指标分解" width="300" v-if="form.manageType == 'work_target'">
            <template slot-scope="scope">
              <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                <div v-html="scope.row.resolveContent">
                </div>
                </div>
            </template>
          </el-table-column>
        <el-table-column fixed="right" label="操作" width="120">
          <template slot-scope="scope">
            <el-button type="text" size="small" @click="viewDetail(scope.row)">
              查看完成详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="grade-container flex-box flex-align-center">
        <div class="flex-1" style="text-align: center;"></div>
        <div class="weight">
          <span>总权重</span>
          <span>{{ totalWeight-0 }}</span>
        </div>
        <span v-if="totalGrade > 0">
          <div class="grade">总分 {{ totalGrade-0 }}</div>
        </span>
      </div>
    </el-card>
    <div v-else>
      <monthlyPerf :bizId="id" :actionType="actionType"></monthlyPerf>
    </div>
    <!-- <ViewDialog :visible.sync="viewVisible" v-dialogDraw :finishedNum="finishedNum" :pendingReviewNum="pendingReviewNum"
      :notSubmitNum="notSubmitNum" :kpiId="kpiId"></ViewDialog> -->
    <!-- 查看详情 -->
    <el-dialog :visible="viewVisible" title="" :close-on-click-modal="false" width="80%" @close='handleClose'
      class="examiner-dialog" append-to-body>
      <el-tabs v-model="outerActive">
        <el-tab-pane name="work" label="工作任务">
          <div>
            <el-tabs type="border-card" v-model="activeName">
              <el-tab-pane :name="item.name" v-for="(item, index) in tabList" :key="index">
                <span slot="label"> {{ item.label }}<i :style="{ 'color': item.color }" style="font-style: normal;">({{
                  item.number }})</i></span>
                <TaskTable :planId="kpiId" :taskType="item.name" :fetchUrl="fetchUrl" v-if="(activeName == item.name)" v-bind="$attrs" from='targetView'/>
              </el-tab-pane>
            </el-tabs>
          </div>
        </el-tab-pane>
        <el-tab-pane name="perf" label="月度绩效">
          <dy-table :keys="colKey" :list="tableData" :fetchData="()=>{}" style="padding:0px;" ref="usingTable" :isPagination="false">
        </dy-table>
        </el-tab-pane>
      </el-tabs>
    </el-dialog>
    <!-- <CheckTaskDdialog v-dialogDraw v-if="viewVisible" :visible.sync="viewVisible" :title="checkTaskDialogTitle"
      :planId="kpiId" :noBeginNum="notSubmitNum" :checkingNum="pendingReviewNum" :finishedNum="finishedNum"
      :fetchUrl="fetchUrl" from="targetView" /> -->
  </div>
</template>
<script>

</script>
<script>
import Api from '@/api';
// import ViewDialog from './ViewDialog.vue';
// import CheckTaskDdialog from '@/views/ProjectManage/developProgress/components/CheckTaskDdialog.vue';
import TaskTable from '@/views/ProjectManage/developProgress/components/TaskTable.vue';
import monthlyPerf from '../../MonthlyPerf/MonthlyPerf.vue'
import DyTable from '@/components/DyTable';
export default {
  name:'TargetView',
  components:{TaskTable,monthlyPerf,DyTable},
  computed: {
    totalGrade(){
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.score-0));
      }, 0);
      return result.toFixed(2)
      // return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    totalWeight() {
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.weight-0));
      }, 0);
      return result.toFixed(2)
      // return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    indicatorsTypeName(){
      return id=>{
        let name = ''
        if(this.targetList.length){
          let findRes = this.targetList.find(item=>item.id == id)
          if(findRes){
            name = findRes.name
          }
        }
        return name
      }
    },
    title(){
      let obj = {
        manageTarget:'目标责任书(管理指标)',
        workTarget:'目标责任书(工作指标)'
      }
      return obj[this.type] || ''
    }
  },
  data() {
    var that = this
    return {
      outerActive:'work',
      activeName: 'waiting_send',
      tableData:[],
      colKey:{
        title: '指标',
        targetItemTwo: {
          label: 'KPI',
          showTooltip: true
        },
        weight:'权重',
        content: {
          label: '指标具体描述',
          showTooltip: true
        },
        maxScore:'分值',
        assessmentRemarks:'完成情况描述',
        pretendScore:'自评分30%',
        score:'领导评分70%',
        weightScore:'权重得分',
          // handle: (scope, createElement)=> {
            // return this.calculateWeightScore(scope.$index)
            // <template slot-scope="scope">
            //     {{ calculateWeightScore(scope.$index) }}
            // </template>
            // // return createElement('el-tag',{
            // //     attrs:{
            // //       type:options[scope.row.kpiGroupStatus].tag
            // //     },
            // //     domProps:{
            // //       innerHTML:options[scope.row.kpiGroupStatus].statusName
            // //     }
            // //   }
            // // );
          // }
        createDate:'日期'

      },
      tabList: [
        {
          name: 'waiting_send',
          label: '进行中',
          number: this.notSubmitNum,
          color: '#ccc'
        },
        {
          name: 'pending',
          label: '审核中',
          number: this.pendingReviewNum,
          color: '#223273'
        },
        {
          name: 'done',
          label: '已完成',
          number: this.finishedNum,
          color: '#2FC25B'
        }
      ],
      editId: '',
      isGoback: false,
      deleteDisable: false,
      currentRowIndex: null,
      editType: 1,
      targetList: [],
      form: {
        manageType: "work_target",//指标类型
        name: "",
        departmentName: "",
        examiner: {
          id: "",//审核人id
          name: ''
        },
        assessmentCycle: '',//考核周期
        keyPerformanceIndicatorsList: [
          {
            indicatorsType: {//管理指标对应的是：目标项目；工作指标对应的是目标项目（一级）
              id: ''
            },
            targetItemTwo: '',
            content: '',
            weight: '',
            assessmentMethod: '',
            assessmentTime: '',
            assessmentRemarks: '',
            grade: '',
          },
        ],
      },
      statusKey:[
        {name:'进行中',tag:'warning',key:'notSubmit'},
        {name:'审核中',tag:'',key:'pendingReview'},
        {name:'已完成',tag:'success',key:'finished'},
      ],
      notSubmitNum:0,
      finishedNum:0,
      pendingReviewNum:0,
      kpiId:'',
      viewVisible:false,

      checkTaskDialogTitle:'查看完成情况详情',
      fetchUrl:'',
      actionType:'',
      frompage:''
    }
  },
  beforeRouteEnter(to,from,next){
    if(to.query && to.query.id && to.query.type){
      if(to.query?.type == "month" ){
        to.meta.activeMenu = '/manpowerResource/performanceManage/monthlyPerf'
        if(to.query?.editType == 2)to.meta.saasTitle  = '编辑月度绩效'
        else to.meta.saasTitle  = '查看月度绩效'
      }else{
        to.meta.saasTitle  = '查看目标责任书'
        to.meta.activeMenu = '/manpowerResource/performanceManage/targetBook'
      }
    }
    next()
  },
  created() {
    this.id = this.$route.query.id
    this.type = this.$route.query.type
    this.frompage = this.$route.query.frompage
    this.actionType = this.$route.query.actionType || 'preview'
    this.getWorkTargetDetail()
    // this.editType = this.$route.query.editType
    // if (this.editType == 2) {
    //   this.editId = this.$route.query.id

    // }
    this.getIndicatorsTypeList()
    this.getPersonInfo()
  },
  destroyed() {
    // 解决Vue $on能拿到数据但是无法更新data数据
    if (this.isGoback) {
      this.$bus.$emit('targetType', 1);
    }
  },
  methods: {
    calculateWeightScore(item){
      // console.log('index',index)
      let pretendScore =item.pretendScore || 0
      let score = item.score || 0
      let weight = item.weight || 0
      weight = weight*100
      let weightScore = 0
      weightScore = ((pretendScore*0.3 + score*0.7)*weight)/item.maxScore
      weightScore = weightScore.toFixed(2)
      return weightScore
    },
    handleImportSuccess(data) {
      data.forEach(item => {
        if (!item.indicatorsType) {
          item.indicatorsType = {
            id: ''
          }
        }
      });
      this.form.keyPerformanceIndicatorsList = data
    },
    getWorkTargetDetail() {

      this.$axios.post(
        Api.performance.getWorkTargetDetail, // 放开
        {
          data: {
            id: this.id,
          }
        },
        res => {
          const data = res.data;
          this.form.examiner = data.examiner;
          this.form.assessmentCycle = data.assessmentCycle
          data.keyPerformanceIndicatorsList.sort((a,b)=>{
            return a.sort - b.sort
          })
          this.form.keyPerformanceIndicatorsList = data.keyPerformanceIndicatorsList
          this.resolveDetailContent();
        }
      );
    },
    // 指标分解内容
    resolveDetailContent() {
      this.form.keyPerformanceIndicatorsList.forEach((x,xIndex)=>{
        if (x.kpiSplitItems) {
          let type = x.kpiSplitItems[0]['kpiSplitType']
          let value = x.kpiSplitItems
          let newStr = '';
          if (type == 'month') {
            newStr = value.reduce((str,curvalue,index)=> {
              return str + '<p>'+Number(index+1)+'、<span class="time-tag-style">'+ curvalue.targetTime +'月</span> '+ curvalue.content +'</p>'
            },'')
          } else {
            newStr = value.reduce((str,curvalue,index)=> {
              let ratioStr = curvalue['kpiSplitItemWeights'].reduce((str2,curvalue2)=>str2+curvalue2['targetTime']+'月占比' +curvalue2['weight']+'%,','').replace(/,([^,]*)$/, "$1");
              return str + '<p>'+Number(index+1)+'、<span class="time-tag-style">第'+ curvalue.targetTime +'季度</span> '+ curvalue.content +'('+ratioStr +')</p>'
            },'')
          }
          this.$set(this.form.keyPerformanceIndicatorsList[xIndex],'resolveContent',newStr)
        }
      })
    },
    getPersonInfo() {
      this.$axios.post(
        Api.frameworkInfo.findUserDetail, // 放开
        {
          data: {
            id: this.$store.state.user.userId,
            flag: 'company'
          }
        },
        res => {
          const data = res.data
          this.form.name = data.name
          const userDutyVosList = data.userDutyVos.filter(item => {
            return item.dutyType == 1

          })
          const {
            departmentId
          } = userDutyVosList[0]
          this.findDepartmentName(departmentId)
        }
      );
    },
    findDepartmentName(id) {
      this.$axios.post(
        Api.performance.findDepartmentId,
        {
          data: {
            id
          }
        },
        res => {
          if (res.isSuccess) {
            this.form.departmentName = res.data.departmentName
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getIndicatorsTypeList() {
      var params = {
        data: {
          // enableType: 'enable',
          manageType: "work_target"
        }
      };
      if(this.type == 'manageTarget')params.data.manageType ='manager_target'
      if (this.type == 'month') {
        delete params.data.manageType
      }
      this.$axios.post(
        Api.performance.indicatorsTypeList,
        params,
        res => {
          if (res.isSuccess) {
            this.targetList = res.data ? res.data : [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    goback() {
      this.isGoback = true;
      this.$router.go(-1)
      // this.$router.push({
      //   path: '/performanceManage/targetBook',
      // });
    },
    tableRowClassName({ row, rowIndex }) {
      row.row_index = rowIndex;
    },
    clearValidate() {
      this.$refs.form.clearValidate()
    },
    viewDetail(row){
      // this.notSubmitNum=
      this.tabList[0].number =  row.notSubmit
      this.tabList[1].number =  row.pendingReview
      this.tabList[2].number =  row.finished
      // this.finishedNum=row.finished
      // this.pendingReviewNum=row.pendingReview
      this.kpiId = row.id
      this.fetchUrl = Api.taskManage.taskArrange.getTargetCompleteInfo
      this.viewVisible=true
      let monthKpiList = row.monthKpiList
      monthKpiList?.forEach(item=>{
        item.weightScore = this.calculateWeightScore(item)
        item.createDate = item.createDate.substr(0,10)
        item.title = item.indicatorsType.name
      })
      this.tableData = row.monthKpiList || []
    },
    handleClose(){
      this.viewVisible = false
    }
  },
}
</script>

<style lang="scss" scoped>
h3 {
  text-align: center;
}
.target-book-container {
  min-width: 1200px;

  .btn-box {
    border: 1px solid #ebeef5;
    border-bottom: 0;
    padding: 5px 10px;
  }

  ::v-deep .el-form-item__error {
    padding-top: 0;
    margin-top: 3px;
  }

  .grade-container {
    border: 1px solid #ebeef5;
    line-height: 40px;

    .weight {
      span {
        display: inline-block;
        width: 100px;
      }
    }

    .grade {
      width: 150px;
      text-align: center;
      border-left: 1px solid #ebeef5;
    }
  }

  ::v-deep .el-table td.el-table__cell div .time-tag-style {
    color: #fff;
    border-radius: 16px;
    background: rgb(47, 194, 91);
    text-align: center;
    padding: 2px 6px;
  }
  ::v-deep .el-textarea.is-disabled .el-textarea__inner,
  ::v-deep .el-input.is-disabled .el-input__inner {
    color: #2c2c2c;
  }
}
</style>

