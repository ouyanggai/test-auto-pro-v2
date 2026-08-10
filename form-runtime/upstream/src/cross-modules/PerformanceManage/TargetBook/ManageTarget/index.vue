<!--
 * @description:目标责任书/管理指标
 * @Author: Calvin
 * @Date: 2022-03-07 17:59:44
 * @FilePath: \src\views\PerformanceManage\TargetBook\ManageTarget\index.vue
-->
<template>
  <div class="target-book-container">
    <el-button icon="el-icon-back" @click="goback" v-if="!isInFlow">返 回</el-button>
    <!-- height: calc(100% - 60px); -->
    <el-card class="box-card mt-10" style="overflow:auto;padding-bottom: 50px;" shadow="never">
      <h3 >
      <div style="display:flex;align-items: center;justify-content: center;font-size:18px;">
        <span>目标责任书(管理指标)</span>
      </div>
      <div class="zoom">
        <!-- <el-button icon="el-icon-minus" plain circle type="primary" size="mini" @click="zoom(-1,'boxCard','scaleVal')" :disabled="scaleVal< 5"></el-button> -->
        <div class="zoom-button" @click="zoom(-1)" :class="{ 'disabled': scaleVal <= 10 }">-</div>
        <span>{{ scaleVal * 10 }}%</span>
        <div class="zoom-button" @click="zoom(1)" :class="{ 'disabled': scaleVal >= 12 }">+</div>
      </div>
    </h3>
      <el-form :model="form" :rules="rules" ref="form" :inline="true" class="main-form">
        <div class='top-form'>
          <el-form-item label="姓名">
              <el-input v-model="form.name" :disabled="isDisabled('name')" style="width:86px;"></el-input>
            </el-form-item>
            <el-form-item label="所属单位" style="flex: 1;" >
              <el-input v-model="company" readonly :disabled="isDisabled('company')" style="width:300px"></el-input>
            </el-form-item>
            <el-form-item label="日期" prop="targetTime">
              <el-date-picker
                style="width:120px;"
                v-model="form.targetTime"
                type="year"
                format="yyyy"
                value-format="yyyy"
                :disabled="isDisabled('name')"
                placeholder="考核年限">
              </el-date-picker>
            </el-form-item>
            <el-form-item label="部门" >
              <el-input v-model="form.departmentName" readonly :disabled="isDisabled('departmentName')" style="width:120px;"></el-input>
            </el-form-item>
            <el-form-item label="主考核人" prop="examiner.name" >
              <el-input v-model="form.examiner.name" placeholder="选择主考核人" @focus="openExaminerDialog" readonly :disabled="isDisabled('examinerName')" style="width:100px;"></el-input>
            </el-form-item>
            <el-form-item label="考核周期" style="border-right: none;width:209px;" v-if="assessmentCycle||searchFlowType=='year_kpi_work_target'">
              <!-- <el-select v-model="assessmentCycle" placeholder="请选择" :disabled="isDisabled('assessmentCycle')" style="width: 110px;">
                <el-option label="半年" value="half_year"></el-option>
                <el-option label="年终" value="year"></el-option>
              </el-select>  prop="assessmentCycle"-->
              <span>{{ ({ year: '年终', half_year: '半年' })[assessmentCycle] }}</span>
            </el-form-item>
            <el-form-item label="考核周期" prop="assessmentCycle" style="border-right: none;width:209px;" v-else>
              <el-select v-model="form.assessmentCycle" placeholder="请选择" :disabled="isDisabled('assessmentCycle')" style="width: 110px;">
                <el-option label="半年及年终" value="year_and_half_year"></el-option>
                <el-option label="年终" value="year"></el-option>
              </el-select>
            </el-form-item>
        </div>
        <div class="btn-box" v-if="actionType == 'create' || actionType == 'edit'">
          <el-button icon="el-icon-plus" @click="handleInsert">插入行</el-button>
          <el-button type="danger" icon="el-icon-delete" :disabled="deleteDisable"
            @click="handleDel">删除行</el-button>
          <!-- <el-link :href="downloadExcelUrl" :underline="false" style="margin:0 10px"> -->
          <el-button type="primary" plain icon="el-icon-download" @click="downLoadTemplate">模板下载</el-button>
          <el-button type="primary" icon="el-icon-document-add" @click="handleImport">导入数据</el-button>
          <!-- </el-link> -->
          <!-- <el-button type="primary" round @click="submit('form')">提交</el-button> -->
        </div>
        <el-table :data="form.keyPerformanceIndicatorsList" :key="forReload" highlight-current-row ref="table" border :max-height="tableMaxHeight"
          align="center" style="width: 100%" :row-class-name="tableRowClassName" @row-click="handleRowClick" :summary-method="getSummaries" :showSummary="true" class="main-table">
          <el-table-column label="序号" type="index" width="40" align="center" >
          </el-table-column>
          <el-table-column label="目标项目"  width="99">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.indicatorsType.id }}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.indicatorsType.id'"
                  :rules='rules.indicatorsType'>
                  <el-select style="width: 100%;" v-model="scope.row.indicatorsType.id" placeholder="请选择" :disabled="isDisabled('target')" filterable>
                    <el-option v-for="item in targetList" :key="item.id" :label="item.name" :value="item.id"
                      :disabled="item.enableType == 'disable' ? true : false">
                    </el-option>
                  </el-select>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="具体目标项目内容">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.content}}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.content'" :rules='rules.content'>
                  <el-input type="textarea" size="medium" :autosize="{ minRows: 1,maxRows: 5 }"
                    v-model="scope.row.content" :disabled="isDisabled('content')">
                  </el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="权重" width="70" align="center">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.weight }}
              </template> -->
              <template>
                <el-form-item :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.weight'" :rules='rules.weight'>
                  <el-input size="mini" v-model.trim="scope.row.weight" :disabled="isDisabled('weight')" style="text-align:center;" class="weight-class" >
                    <template slot="append">%</template>
                  </el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="目标完成标准(考核标准)">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.assessmentMethod}}
              </template> -->
              <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentMethod'"
                :rules='rules.assessmentMethod' style="position: relative;">

                <el-input type="textarea" size="medium" :autosize="{ minRows: 1,maxRows: 5 }"
                  v-model="scope.row.assessmentMethod" :disabled="isDisabled('assessmentMethod')">
                </el-input>
                <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '目标完成标准(考核标准)')" ></i>
              </el-form-item>
            </template>
          </el-table-column>
          <el-table-column label="目标完成时间节点">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.assessmentTime}}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentTime'"
                  :rules='rules.assessmentTime'>

                  <el-input type="textarea" size="medium" :autosize="{ minRows: 1,maxRows: 5 }"
                    v-model="scope.row.assessmentTime" :disabled="isDisabled('assessmentTime')">
                  </el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <!--  v-if="!isDisabled('assessmentRemarks')" -->
          <el-table-column label="完成情况描述" v-if="false">
            <template slot-scope="scope">
                <el-form-item class="elFormExpanded" v-if="isDisabled('assessmentRemarks')" key="assessmentRemarks">
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{ minRows: 1,maxRows: 5 }"
                    v-model="scope.row.assessmentRemarks"
                    :disabled="isDisabled('assessmentRemarks')"
                    :key=2
                  >
                  </el-input>
                </el-form-item>
                <el-form-item
                class="elFormExpanded"
                v-else
                :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentRemarks'"
                :rules="rules.assessmentRemarks"
                :key=1
                >
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{ minRows: 1,maxRows: 5 }"
                    v-model="scope.row.assessmentRemarks"
                    :disabled="isDisabled('assessmentRemarks')"
                  >
                  </el-input>
                </el-form-item>
            </template>
          </el-table-column>
          <el-table-column v-if="false"
            label="得分"
            width="100">
            <template slot-scope="scope">
                <el-form-item
                  v-if="isDisabled('grade')"
                  key="grade"
                >
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('grade')"
                    v-model.trim="scope.row.score"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
                <el-form-item
                  v-else
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.score'"
                  :rules="rules.score"
                >
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('grade')"
                    v-model.trim="scope.row.score"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
            </template>
          </el-table-column>
          <!--  v-if="!isDisabled('halfYearAssessmentRemarks')||(assessmentCycle=='year'&&assessmentCycleType=='year_and_half_year')||(searchFlowType=='year_kpi_work_target'&&actionType=='preview')" -->
          <el-table-column label="半年完成情况描述"  v-if="showYearHalfCol">
            <template slot-scope="scope">
                <el-form-item class="elFormExpanded" v-if="isDisabled('halfYearAssessmentRemarks')" key="halfYearAssessmentRemarks" style="position: relative;">
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.halfYearAssessmentRemarks"
                    :disabled="isDisabled('halfYearAssessmentRemarks')"
                  >
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '半年完成情况描述')" ></i>
                </el-form-item>
                <el-form-item
                class="elFormExpanded"
                v-else
                :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.halfYearAssessmentRemarks'"
                :rules="rules.halfYearAssessmentRemarks"
                 style="position: relative;"
                >
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.halfYearAssessmentRemarks"
                    :disabled="isDisabled('halfYearAssessmentRemarks')"
                  > <!-- :disabled="isDisabled('halfYearAssessmentRemarks')||yearKpiScoreGroup.length>0" -->
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '半年完成情况描述')" ></i>
                </el-form-item>
            </template>
          </el-table-column>
          <!-- v-if="!isDisabled('halfYearGrade')||(assessmentCycle=='year'&&assessmentCycleType=='year_and_half_year')||(searchFlowType=='year_kpi_work_target'&&actionType=='preview')" -->
          <el-table-column
            label="半年得分"
            width="70"
            v-if="showYearHalfCol"
          >
            <template slot-scope="scope">
                <el-form-item
                  v-if="isDisabled('halfYearGrade')"
                  key="halfYearGrade"
                >
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('halfYearGrade')"
                    v-model.trim="scope.row.halfYearGrade"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
                <el-form-item
                  v-else
                >
                <!--                   :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.halfYearGrade'"
                  :rules="rules.halfYearGrade" -->
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('halfYearGrade')"
                    v-model.trim="scope.row.halfYearGrade"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
            </template>
          </el-table-column>
          <!--  v-if="!isDisabled('yearAssessmentRemarks')||(searchFlowType=='year_kpi_work_target'&&assessmentCycle=='year'&&actionType=='preview')" -->
          <el-table-column label="年终完成情况描述" v-if="showYearEndCol">
            <template slot-scope="scope">
                <el-form-item class="elFormExpanded" v-if="isDisabled('yearAssessmentRemarks')" key="yearAssessmentRemarks" style="position: relative;">
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.yearAssessmentRemarks"
                    :disabled="isDisabled('yearAssessmentRemarks')"
                  >
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '年终完成情况描述')" ></i>
                </el-form-item>
                <el-form-item
                class="elFormExpanded"
                v-else
                :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.yearAssessmentRemarks'"
                :rules="rules.yearAssessmentRemarks"
                 style="position: relative;"
                >
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.yearAssessmentRemarks"
                    :disabled="isDisabled('yearAssessmentRemarks')"
                  >
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '年终完成情况描述')" ></i>
                </el-form-item>
            </template>
          </el-table-column>
          <!-- v-if="!isDisabled('yearGrade')||(searchFlowType=='year_kpi_work_target'&&actionType=='preview'&&assessmentCycle=='year')" -->
          <el-table-column
            label="年终得分"
            width="70"
            v-if="showYearEndCol"
          >
            <template slot-scope="scope">
                <el-form-item
                  v-if="isDisabled('yearGrade')"
                  key="yearGrade"
                >
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('yearGrade')"
                    v-model.trim="scope.row.yearGrade"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
                <el-form-item
                  v-else
                >
                <!--                   :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.yearGrade'"
                  :rules="rules.yearGrade" -->
                  <el-input
                    type="number"
                    size="mini"
                    :disabled="isDisabled('yearGrade')"
                    v-model.trim="scope.row.yearGrade"
                    @keypress.native="e => keypress(e, scope.$index)"
                  ></el-input>
                </el-form-item>
            </template>
          </el-table-column>
        </el-table>
      </el-form>
      <!-- <div class="grade-container flex-box flex-align-center">
        <div class="flex-1" style="text-align: center;"></div>
        <div class="weight">
          <span>总权重</span>
          <span>{{ totalWeight-0 }}</span>

        </div>
        <div class="grade" >
          <span v-if="totalGrade > 0">
            总分 {{ totalGrade - 0 }}
          </span>
        </div>
      </div> -->
    </el-card>
    <ExaminerDialog :visible.sync="examinerVisible" :examinerId="form.examiner.id" v-if="examinerVisible"
      @select="handleExaminerSelect" />
    <UploadDialog :visible.sync="importDialogVisible" @success="handleImportSuccess" assessmentCycle="year" />

    <!-- <div class="footer-bt" >
      <div class="footer-inner">
        <el-button type="primary" @click="reSubmit('save')">保存草稿</el-button>
        <el-button type="primary" @click="reSubmit('submit')">提交</el-button>
      </div>
    </div> -->
    <div class="footer-bt" v-if="!isReInitiate">
      <div class="footer-inner" v-if="actionType == 'create' || actionType == 'edit'">
        <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
        <el-button @click="goback" plain v-if="!isInFlow">取 消</el-button>
        <el-button @click="cancel" plain v-if="isInFlow">取 消</el-button>
        <el-button type="primary" plain v-if="!isInFlow" @click="save">保 存</el-button>
        <el-button type="primary" plain @click="submit('not_submitted')" v-if="isInFlow">保 存</el-button>
        <el-button type="primary" @click="submit('under_review')" v-if="isInFlow">提 交</el-button>
      </div>
    </div>
    <el-dialog v-if="bigInputVisible" :visible="bigInputVisible" :title="currentBigTitle" :close-on-click-modal="false" width="80%"
      @close="bigInputVisible = false" append-to-body style="height:100%;">
      <div style="position:relative;">
        <el-input :disabled="isDisabled(showBigInput.showType)" class="bigInputClass" type="textarea" :autosize="{minRows:10}" style="width: 100%;" v-model="currentEditText"></el-input>
        <span style="position: absolute;z-index: 1;right: 10px;bottom:0px;color:#999;font-size: 12px;font-weight: 100;">{{ currentEditText.length }}</span>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="bigInputVisible = false">取 消</el-button>
        <el-button @click="confirmBigInput" type="primary">确 定</el-button>
      </div>
    </el-dialog>
  </div>
</template>
<script>

</script>
<script>
import Api from '@/api';
// import ExaminerDialog from '../components/ExaminerDialog';
import ExaminerDialog from '../WorkTarget/components/ExaminerDialog.vue';
import UploadDialog from '../components/UploadDialog';
import Sortable from 'sortablejs';
import {localstorageGet} from '@/utils/auth.js'
import moment from 'moment';
import math from '@/utils/math.js';
import { deepClone } from '@/utils';
export default {
  components: { UploadDialog, ExaminerDialog },
  computed: {
    totalGrade(){
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.score-0));
      }, 0);
      return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    totalWeight() {
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.weight-0));
      }, 0);
      return result.toFixed(2)
      // return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    isDisabled(){
      return (prop)=>{
        if(!this.permisionList.length){
          return true
        }else{
          if(this.permisionList.indexOf(prop) > -1){
            return false
          }else{
            return true
          }
        }
      }
    },
    assessmentCycle() {
      // return this.$route?.query?.assessmentCycle||(this.yearKpiScoreGroup.length==0?'':this.yearKpiScoreGroup.includes('year')?'year':'half_year')
      return this.$route?.query?.assessmentCycle || this.currentHalfOrYear;

    },
    assessmentCycleType() {
      return this.$route?.query?.assessmentCycleType
    },
    showYearEndCol() {
      return ((this.searchFlowType == 'year_kpi_work_target' || this.selectFlowType == 'year_kpi_work_target') && this.assessmentCycle == 'year')
      || (this.assessmentCycle == 'year' && this.actionType == 'preview')
    },
    showYearHalfCol() {
      return ((this.searchFlowType == 'year_kpi_work_target' || this.selectFlowType == 'year_kpi_work_target') && this.assessmentCycle == 'half_year')
      || (this.assessmentCycle == 'half_year' && this.actionType == 'preview')
    },
  },
  watch: {
    forReload: {
      handler() {
        this.$nextTick(() => {
          this.rowDrop();
        });
      },
      immediate: true
    }
  },
  props:{
    bizId:{
      type:String,
      default:''
    },
    actionType:{
      type:String,
      default:'create'
    },
    flowNodeProxyId:{
      type:String,
      default:''
    },
    isReInitiate:{
      type:Boolean,
      default:false
    },
    flowProxyId:{ //流程id
      type:String,
      default:''
    },
    flowInstanceId:{
      type:String,
      default:''
    },
    selectFlowType:{
      type:String,
      default:''
    },
    searchFlowType:{
      type:String,
      default:''
    }
  },
  inject:   {
    prevStepHandle:{value:'prevStepHandle',default:null},
    sumbitFlow:{value:'sumbitFlow',default:null},
    submitFlowFinal:{value:'submitFlowFinal',default:null},
    batchCode:{value:'batchCode',default:''}
  },
  data() {
    return {
      createrId: '',
      tableMaxHeight: window.innerHeight * 0.77,
      currentBigRow: null,
      currentEditText: '',
      currentBigTitle: '',
      bigInputVisible: false,
      forReload: new Date().getTime(),
      company:localstorageGet('companyName'),
      editId: '',
      isGoback: false,
      deleteDisable: false,
      currentRowIndex: null,
      editType: 1,
      importDialogVisible: false,
      examinerVisible: false,
      targetList: [],
      downloadExcelUrl: 'https://file.runshihua.com/files//200001/66a384f04e8a4868a8205fcb23ab7448.xlsx', //下载模板
      form: {
        manageType: "manager_target",//指标类型
        name: "",
        targetTime: moment().format('YYYY'),
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
            content: '',
            weight: '',
            assessmentMethod: '',
            assessmentTime: '',
            assessmentRemarks: '',
            score: '',
          },
        ],
      },

      //表单验证规则
      rules: {
        "examiner.name": [
          {
            required: true,
            trigger: 'change',
            message: '请输入主考核人',
          }
        ],
        targetTime: [{
          required: true,
          trigger: 'change',
          message: '请选择考核年限'
        }],
        assessmentCycle: [{
          required: true,
          trigger: 'change',
          message: '请选择考核周期',
        }],
        indicatorsType: [{
          required: true,
          trigger: 'change',
          message: '请选择目标项目(一级)',
        }],
        content: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入具体目标项目内容',
        }],
        weight: [{
          required: true,
          trigger: 'blur',
          message: '请输入权重',
        }, {
          pattern: /^[+-]?(0|([1-9]\d*))(\.\d+)?$/g,
          message: '请输入数字',
          trigger: 'blur'
        }],
        assessmentMethod: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入目标完成标准(考核标准)',
        }],
        assessmentTime: [{
          required: true,
          trigger: 'blur',
          message: '请输入目标完成时间节点',
        }],
        assessmentRemarks: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入完成情况描述'
        }],
        score: [{
          required: true,
          trigger: 'blur',
          message: '请输入得分'
        }],
        halfYearAssessmentRemarks:[{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入半年完成情况描述'
        }],
        yearAssessmentRemarks:[{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入年终完成情况描述'
        }],
        halfYearGrade: [{
          required: true,
          trigger: 'blur',
          message: '请输入半年得分'
        }],
        yearGrade: [{
          required: true,
          trigger: 'blur',
          message: '请输入年终得分'
        }],
      },
      isInFlow:true,
      permisionList:[],
      scaleVal:localstorageGet('annual_scaleVal') || 10,
      yearKpiScoreGroup:[],
      currentHalfOrYear: '' // 通过流程模板名称后缀信息来判断是半年还是年终
    }
  },
  mounted() {
    // var doc = document.querySelector('.el-dialog__headerbtn i');
    // doc.style.fontSize = '28px';
    // doc.classList.add('el-icon-error');
  },
  created() {
    (this.searchFlowType == 'year_kpi_work_target' || this.selectFlowType == 'year_kpi_work_target') && this.flowTemplateFindById();
    console.log(this.editType,'this.editType',this.actionType);
    if(this.$route?.query?.editType){
      this.editType = this.$route.query.editType;
      this.isInFlow = false
    }
    if(this.actionType !== 'create')this.editType = 2
    console.log(this.editType,'this.editType')
    if (this.editType == 2 ) {
      this.editId = this.$route.query.id || this.bizId
      this.getWorkTargetDetail(this.searchFlowType);
    } else{
      this.$nextTick(() => {
        // this.calcateInputHeight()
        this.getPersonInfo()
      })
    }
    if(this.$route?.query?.isExamine){
      this.isExamine = true
    }
    this.getIndicatorsTypeList()
    if(this.isInFlow){
      if(this.actionType == 'examine'){
        this.getInputPermision().then(res=>{
          res.push('detailButton')
          this.permisionList = res
        })
      }else if(this.actionType == 'create' || this.actionType == 'edit'){
        this.getPermisionForCreate().then(res=>{
          this.permisionList = res
          console.log(this.permisionList,'this.permisionList+++')
        })
        // this.permisionList=['name','company','departmentName','examinerName','assessmentCycle','target','targetItemTwo','content','weight','assessmentMethod','assessmentTime','assessmentRemarks']
      }
      else if(this.actionType == 'preview'){
        this.permisionList = []
      }
    }else {
      this.permisionList=['name','company','departmentName','examinerName',
      'assessmentCycle','target','targetItemTwo',
      'content','weight','assessmentMethod',
      'assessmentTime',]
      // 'assessmentRemarks']
    }

    this.$bus.$off('annual_perf_before_handle')
    this.$bus.$on('annual_perf_before_handle', (val,that) => {
      this.examneObj = that //把审核组件实例传过来,方便后面把loading取消
      if(this.status == 'not_submitted'){
        this.postData()
      }else{
        this.status = val
        // this.submitData('form').then(res=>{
        this.submitData(that?.isTemporarySave || this.validateForm('form')).then(res=>{
          // const year = moment().format('YYYY')
          // const name = `${year}年度目标责任书（管理指标）-${localstorageGet('userName')}`;
          const name = `${this.form.targetTime}年度目标责任书（管理指标）-${this.workTargetData?.userName || localstorageGet('userName')}`;
          const obj = {
            status: 'success',
            name
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        }).catch(err=>{
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          let obj = {
            status: 'fail',
          }
          this.$bus.$emit('submitBeforeHandleOk', obj);
        })
      }
    });
    this.$bus.$off('year_kpi_work_target_before_handle');
    this.$bus.$on('year_kpi_work_target_before_handle', (val,that) => {
      this.examneObj = that
      this.submitData(that?.isTemporarySave || this.validateForm('form')).then(res => {
          // const year = moment().format('YYYY')
          // const name = `${year}年度目标责任书（工作指标）-${localstorageGet('userName')}`;
          const obj = {
            status: 'success',
            // name
          };
          var enums = { year: '年终', half_year: '半年' };
          const name = `${this.form.targetTime}年度目标责任书（管理指标）-${localstorageGet('userName')}-${enums[this.assessmentCycle]}`;
          if (val == 'draft' || this.status == 'not_submitted') {
            this.submitFlowFinal(true, res.id, '', '', name);
          } else {
            if (this.isReInitiate) obj.name = name;
            this.$bus.$emit('submitBeforeHandleOk', obj);
          }
        }).catch(err => {
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          const obj = {
            status: 'fail'
          };
          // this.$bus.$emit('submitBeforeHandleOk', obj);
        });
    });
  },
  destroyed() {
    // 解决Vue $on能拿到数据但是无法更新data数据
    if (this.isGoback) {
      this.$bus.$emit('targetType', 2);
    }
  },
  methods: {
    flowTemplateFindById() { // 通过流程模板名称后缀信息来判断是半年还是年终
      this.$axios.post(Api.schedule.getFlowInstanceTemplateNode, { data: { id: this.flowProxyId }}, (res) => {
        if (res.isSuccess && res.data) {
          if (res.data.flowName.includes('（年终）')) {
            this.currentHalfOrYear = 'year';
          } else if (res.data.flowName.includes('（半年）')) {
            this.currentHalfOrYear = 'half_year';
          }
        }
      });
    },
    confirmBigInput() {
      this.currentBigRow[this.showBigInput.showType] = this.currentEditText;
      this.bigInputVisible = false;
    },
    showBigInput(row, title) {
      if (title == '目标完成标准(考核标准)') {
        this.currentEditText = row.assessmentMethod;
        this.showBigInput.showType = 'assessmentMethod';
      } else if (title == '半年完成情况描述') {
        this.currentEditText = row.halfYearAssessmentRemarks;
        this.showBigInput.showType = 'halfYearAssessmentRemarks';
      } else if (title == '年终完成情况描述') {
        this.currentEditText = row.yearAssessmentRemarks;
        this.showBigInput.showType = 'yearAssessmentRemarks';
      }
      this.currentBigRow = row;
      this.currentBigTitle = title;
      this.bigInputVisible = true;
    },
    //获取指定的祖宗节点
    getParents(element, className) {
      //dom.getAttribute('class')==dom.className，两者等价
      var returnParentElement = null;
      function getpNode(element, className) {
        //创建父级节点的类数组
        let pClassList = element.parentNode.getAttribute('class');
        let pNode = element.parentNode;
        if (!pClassList) {
          //如果未找到类名数组，就是父类无类名，则再次递归
          getpNode(pNode, className);
        } else if (pClassList && pClassList.indexOf(className) < 0) {
          //如果父类的类名中没有预期类名，则再次递归
          getpNode(pNode, className);
        } else if (pClassList && pClassList.indexOf(className) > -1) {
          returnParentElement = pNode;
        }
      }
      getpNode(element, className);
      //console.log(returnParentElement);
      return returnParentElement;
    },
    keypress(event, index) {
      if (event.keyCode == 13) {
        //回车,光标下移
        let targetClass = 'el-table__cell'
        let pnode = this.getParents(event.target, targetClass)
        let classTag = pnode.className
        let tdClass = classTag.replace(targetClass, '').trim()
        let nodeList = document.querySelectorAll(`td.${tdClass} input`)
        let len = nodeList?.length
        if (len - 1 > index) {
          nodeList[index + 1].focus()
        }
      }
    },
    zoom(val) {
      let value = Number(this.scaleVal) + Number(val)
      value = value >= 12 ? 12 : value <= 10 ? 10 : value
      let mainForm = document.querySelector('.main-form')
      if (mainForm) {
        this.scaleVal = value
        mainForm.style.zoom = value / 10
        localstorageSet('annual_scaleVal', value)
        this.$nextTick(()=>{
          this.calculateKpiWidth()
        })
      }
    },
    calcateInputHeight() {
      let allInputObj = document.querySelectorAll('.main-table td.el-table__cell')
      allInputObj.forEach(item => {
        if (!item) return;
        let height = item.clientHeight
        let input = item.querySelector('input')
        if (input) {
          input.style.height = `${height - 7}px`
        }
        let textInput = item.querySelector('textarea')
        if (textInput) {
          textInput.style.height = `${height - 7}px`
        }
        return
      })
    },
    getSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (column.label == '权重') {
          let sub = data.reduce((prev, cur) => {
            return math.add(prev, cur.weight);
          }, 0);
          sub = Number(sub);
          sums[index] = <div class={'text-center'}>{sub}</div>;
          return;
        }
        if (column.label == '得分') {
          let sub = data.reduce((prev, cur) => {
            return math.add(prev, cur.score);
          }, 0);
          sub = Number(sub);
          sums[index] = <div class={'text-center'}>{sub}</div>;
          return;
        }
        if (column.label == '半年得分') {
          let sub = data.reduce((prev, cur) => {
            return math.add(prev, cur.halfYearGrade||0);
          }, 0);
          sub = Number(sub);
          sums[index] = <div class={'text-center'}>{sub}</div>;
          return;
        }
        if (column.label == '年终得分') {
          let sub = data.reduce((prev, cur) => {
            return math.add(prev, cur.yearGrade||0);
          }, 0);
          sub = Number(sub);
          sums[index] = <div class={'text-center'}>{sub}</div>;
          return;
        }

      });
      return sums;
    },
    downLoadTemplate(){
      let data = {
        code:'year_manage_kpi_template'
      }
      this.$axios.post(Api.performance.downloadTemplate,{data}).then(res=>{
        if(res.isSuccess){
          let fileUrl = res.data.fileVo.fileUrl
          let aEle = document.createElement('a')
          aEle.href = fileUrl
          aEle.target = '_blank'
          aEle.click()
        }
      })
    },
    getPermisionForCreate() {
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          // Api.schedule.flowTemplateFindById,
          url,
          {
            data: {
              id: this.flowProxyId // 流程id
            }
          },
          (res) => {
            let enableList = [];
            if (res.data && res.data.flowNodeTemplate && res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList) {
              const tmpList = res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList || [];
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName;
              });
            }
            // this.enableData = enableList;
            resolve(enableList)
          }
        );
      })
    },
    //获取输入的权限
    getInputPermision() {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.qualityManage.findApprovePermission,
          {
            data: {},
            nodeProxyId: this.flowNodeProxyId
          },
          (res) => {
            let enableList = []
            if (res.data && res.data.flowNodeFieldPowerTemplateList) {
              let tmpList = res.data.flowNodeFieldPowerTemplateList || []
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName
              })
            }
            resolve(enableList)
          }
        );
      })
    },
    handleImportSuccess(data) {
      data.kpiList.forEach(item => {
        if (!item.indicatorsType) {
          item.indicatorsType = {
            id: ''
          }
        }
        item.weight = math.multiply(item.weight, 100)
      });
      this.form.keyPerformanceIndicatorsList = data.kpiList
      this.$nextTick(() => {
        this.$refs.form.clearValidate()
        setTimeout(()=>{
          this.calcateInputHeight()
        })
      })
    },
    getCompanyListOfOnDuty(id) {
      let data = {
        id,
        flag: "company"
      }
      return this.$axios.post(Api.taskManage.myTask.findUserDetail, {data})
    },
    getDepartmentList(relationId){
      let data = {
        relationId,
        type: "company"
      }
      return this.$axios.post(Api.frameworkInfo.getUperDepartmentList, {data})
    },
    getWorkTargetDetail(type) {
      this.$axios.post(
      type=='year_kpi_work_target'?Api.performance.findKpiByYearScoreId:Api.performance.getWorkTargetDetail, // 放开
        {
          data: {
            id: this.editId,
          }
        },
        res => {
          console.log(res,'888888')
          const data = this.workTargetData = res.data||{};
          this.createrId = data.createrId;
          this.company = res.data.companyName||localstorageGet('companyName')
          if(type=='year_kpi_work_target'){
            this.yearKpiScoreGroup = data.yearKpiScoreGroupList&&data.yearKpiScoreGroupList.map(item=>{
              return item.assessmentCycle
            })
          }
          this.form.examiner = data.examiner || {
            id: "",//审核人id
            name: ''
          };
          this.form.departmentName = data.depName;
          this.form.targetTime = data.targetTime + '';
          this.form.assessmentCycle = data.assessmentCycle
          data.keyPerformanceIndicatorsList.sort((a,b)=>{
            return a.sort - b.sort
          })
          const permisionList = []
          this.form.keyPerformanceIndicatorsList = data.keyPerformanceIndicatorsList.map(item => {
            item.weight = math.multiply(item.weight, 100);
            if(item.kpiAssessmentExpands){
              item.kpiAssessmentExpands.map(k=>{
                if(k.assessmentCycle=='half_year'){
                  item.halfYearAssessmentRemarks = k.assessmentRemarks
                  item.halfYearGrade = k.score
                  if (this.actionType == 'examine') {
                    permisionList.push('halfYearAssessmentRemarks')
                    permisionList.push('halfYearGrade')
                  }

                }else{
                  item.yearAssessmentRemarks = k.assessmentRemarks
                  item.yearGrade = k.score
                  if (this.actionType == 'examine') {
                    permisionList.push('yearAssessmentRemarks')
                    permisionList.push('yearGrade')
                  }

                }
              })
            }
            return item;
          });
          this.$nextTick(()=>{
            setTimeout(()=>{
              this.calcateInputHeight()
            },400)
          })
          this.getCompanyListOfOnDuty(data.createrId).then(res=>{
            if(res.isSuccess){
              let data = res.data
              this.form.name = data.name
              let companyId = data.companyId
              let departmentId = data.userDutyVos[0]?.departmentId || ''
              if(companyId && departmentId){
                this.getDepartmentList(companyId).then(res=>{
                  if(res.isSuccess){
                    let find = res.data.find(item=>item.id == departmentId)
                    if(find){
                      // this.form.departmentName = find.departmentName
                    }
                  }
                })
              }
            }
          })
        }
      );
    },
    openExaminerDialog() {
      this.examinerVisible = true;
    },
    // 获取选中人员的人员信息
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
          this.createrId = data.id
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
      const params = {
        data: {
          enableType: 'enable',
          manageType: "manager_target"
        }
      };

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
    handleExaminerSelect(data) {
      this.form.examiner.name = data.name;
      this.form.examiner.id = data.id;
    },
    goback() {
      this.isGoback = true;
      this.$router.go(-1)
      // this.$router.push({
      //   path: '/performanceManage/targetBook',
      // });
    },
    //对部分表单字段进行校验
    validateField(form, index) {
      let result = true;
      for (let item of this.$refs[form].fields) {
        if (item.prop.split(".")[1] == index) {
          this.$refs[form].validateField(item.prop, (error) => {
            if (error != "") {
              result = false;
            }
          });
        }
        if (!result) break;
      }
      return result;
    },
    tableRowClassName({ row, rowIndex }) {
      row.row_index = rowIndex;
    },
    handleRowClick(row) {
      this.currentRowIndex = row.row_index
      if (this.editType == 2) {
        if (row.id) {
          this.deleteDisable = !row.canDelete
        } else {
          this.deleteDisable = false
        }
      }
    },
    clearValidate() {
      this.$refs.form.clearValidate()
    },
    // 拖拽行
    rowDrop() {
      // 要侦听拖拽响应的DOM对象
      const tbody = document.querySelector('.el-table__body tbody');
      const that = this;
      Sortable.create(tbody, {
        onEnd({ newIndex, oldIndex }) {
          const currentRow = that.form.keyPerformanceIndicatorsList.splice(oldIndex, 1)[0];
          that.form.keyPerformanceIndicatorsList.splice(newIndex, 0, currentRow);
          that.forReload = new Date().getTime();
        }
      });
    },
    //插入行
    handleInsert() {
      let itemData = {
        indicatorsType: {//管理指标对应的是：目标项目；工作指标对应的是目标项目（一级）
          id: ''
        },
        content: '',
        weight: '',
        assessmentMethod: '',
        assessmentTime: '',
        assessmentRemarks: '',
        score: '',
      };
      if (!this.currentRowIndex) {
        this.form.keyPerformanceIndicatorsList.push(itemData);
      } else {
        this.form.keyPerformanceIndicatorsList.splice(this.currentRowIndex + 1, 0, itemData)
      }
      this.$nextTick(() => {
        this.clearValidate()
        this.calcateInputHeight()
      })
    },
    // 删除行
    handleDel() {
      if (this.currentRowIndex === null) {
        this.$message.error("请选中一条数据")
        return
      }
      this.$confirm('确认删除行！', '提示', {
              closeOnClickModal: false,
              confirmButtonText: '确定',
              cancelButtonText: '取消',
              type: 'warning',
              closeOnClickModal: false
            }).then(() => {
              //只有一条数据的时候，不允许删除
              if (this.form.keyPerformanceIndicatorsList.length <= 1) {
                this.$message.error("至少保留一条数据")
                return
              } else {
                if (this.currentRowIndex === null) {
                  this.$message.error("请选中一条数据")
                  return
                }
                this.form.keyPerformanceIndicatorsList.splice(this.currentRowIndex, 1);
                if (this.form.keyPerformanceIndicatorsList.length <= 1) {
                  this.currentRowIndex = null
                } else {
                  if (this.currentRowIndex != 0) {
                    this.currentRowIndex = this.currentRowIndex - 1
                  }
                  this.$refs.table.setCurrentRow(this.form.keyPerformanceIndicatorsList[this.currentRowIndex]);
                }
              }
              this.$nextTick(() => {
                this.clearValidate()
              })
            })
    },
    //导入数据
    handleImport() {
      this.importDialogVisible = true;
    },
    cancel(){
      if(this.$route?.params?.from){
        this.$router.go(-1)
      }else{
        this.prevStepHandle()
      }
    },
    submit(status){
      if(status == 0){
        this.status = 'not_submitted'
        // this.submitData('form').then(()=>{
        return new Promise((resolve, reject) => {
          this.$refs.form.validate((valid) => {
            if (valid) {
              this.submitData(true).then(() => {
                this.prevStepHandle();
              });
              resolve();
            } else {
              this.$message.error('请完善目标责任书信息');
              reject();
              return false;
            }
          });
        });
        // this.submitData(this.validateForm('form')).then(()=>{
        //   this.$message.success('操作成功')
        //   this.prevStepHandle()
        // })
      }else{
        this.status = status
        if(status == 'under_review'){
          if(this.isExamine){
            //发起目标责任书考核，重新提交流程和业务
            this.postData()
          }else{
            // 将表单校验单独剥离出来，为了先校验表单，然后再执行业务（表单节点等）判断，避免先弹窗自选节点，然后才高亮报错表单
            if (this.validateForm('form') ) {
              this.sumbitFlow('submit')
            }
          }
        }else{
          this.sumbitFlow('draft')
        }
      }
       //提交之前的校验,之后自动调用 postData方法提交业务和绑定流程
    },
    save(){
      this.status = 'not_submitted'
      this.submitData(this.validateForm('form')).then(()=>{
        this.$message.success('操作成功')
        this.$router.go(-1)
      })
    },
    //提交流程,绑定业务
    postData(){
      this.submitData(this.validateForm('form')).then(res=>{
        var name = `${this.form.targetTime}年度目标责任书（管理指标）-${localstorageGet('userName')}`;
        if (this.selectFlowType == 'year_kpi_work_target') {
          var enums = { year: '年终', half_year: '半年' };
          name = `${this.form.targetTime}年度目标责任书（管理指标）-${localstorageGet('userName')}-${enums[this.assessmentCycle]}`;
        }
        this.submitFlowFinal(true, res.id, '', '', name);
        // this.submitFlowFinal(true,res.id)
      }).catch()
    },
    // 校验表单
    validateForm(form){
      // console.log('validateForm')
      let result = true;
      for (const item of this.$refs[form].fields) {
        this.$refs[form].validateField(item.prop, (error) => {
          if (error != '') {
            console.log(error,item.prop,'item.prop++++++')
            result = false;
          }
        });
      }
      // 校验报错页面定位
      setTimeout(()=>{
        let element = document.getElementsByClassName('is-error')[0]
        if(element){
          element.scrollIntoView({
            behavior:'smooth',
            block:'center'
          })
        }
      },200)
      return result;
    },
    //提交
    submitData(result) {
      return new Promise((resolve,reject)=>{
        // let result = true
        // for (let item of this.$refs[form].fields) {
        //   this.$refs[form].validateField(item.prop, (error) => {
        //     if (error != "") {
        //       result = false;
        //     }
        //   });
        // }
        if (result) {
          //总权重不能大于1
          if (this.totalWeight != 100) {
            this.$message.error("总权重必须为100%")
            this.$parent.$parent.$parent.submitLoading = false
            if(this.examneObj)this.examneObj.submitLoading = false
            reject()
            return
          }
          let postUrl = Api.performance.postKpiGroup;
          this.form.keyPerformanceIndicatorsList.forEach((item,index)=>{
            item.sort = index
          })
          let params = {
            manageType: "manager_target",//指标类型
            targetTime: this.form.targetTime,
            examiner: {
              id: this.form.examiner.id,//审核人id

            },
            assessmentCycle: this.form.assessmentCycle,
            keyPerformanceIndicatorsList: deepClone(this.form.keyPerformanceIndicatorsList)
          }
          if (this.editType == 2) {
            params.id = this.editId
            // params.keyPerformanceIndicatorsList.forEach(item=>{
            //   item.score = item.grade
            // })
            postUrl = Api.performance.updateKpiGroup;
          }
          params.kpiGroupStatus = this.status
          params.keyPerformanceIndicatorsList.forEach((item, index) => {
            item.weight = item.weight/100
          });
          let data ={}
          if(this.assessmentCycle||this.yearKpiScoreGroup.length>0){
            postUrl = Api.performance.saveYearScore
            console.log(this.assessmentCycle,this.yearKpiScoreGroup,'this.yearKpiScoreGroup++++++++')
            const assessmentCycle = this.assessmentCycle?this.assessmentCycle:this.yearKpiScoreGroup.length==1?this.yearKpiScoreGroup[0]:'year'
            console.log(assessmentCycle,'this.yearKpiScoreGroup++++++++')
             data={
              yearKpiGroupId:this.$route.query.id,
              assessmentCycle:assessmentCycle,
              kpiGroupStatus:'under_review',
              kpiScoringType:'manage_scoring',
              assessmentExpands:[],
              id:this.searchFlowType?this.editId:''
            }
            data.assessmentExpands = this.form.keyPerformanceIndicatorsList.map(item=>{
              return{
                groupId:this.$route.query.id,
                kpiId:item.id,
                assessmentCycle:this.assessmentCycle,
                score:this.assessmentCycle=='half_year'?item.halfYearGrade:item.yearGrade,
                assessmentRemarks:this.assessmentCycle=='half_year'?item.halfYearAssessmentRemarks:item.yearAssessmentRemarks
              }
            })
          }
          this.$axios.post(
            postUrl,
            {
              data: this.yearKpiScoreGroup.length>0?data:this.assessmentCycle?data:params,
              batchCode:this.batchCode
            },
            res => {
              if (res.isSuccess) {
                if(this.assessmentCycle){
                  resolve(res.data);
                }else{
                  if (this.editType == 2) {
                    resolve({ id: params.id });
                  } else {
                    resolve(res.data);
                  }
                }
              } else {
                reject()
                this.$message.error(res.message)
                this.$parent.$parent.$parent.submitLoading = false
                if(this.examneObj)this.examneObj.submitLoading = false
                this.$message.error(res.message);
              }
              // this.goback()
            }
          );
        } else {

          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          reject()
          this.$message.error('请完善目标责任书信息')
        }
      })
    }
  },
}
</script>

<style lang="scss" scoped>
::v-deep .bigInputClass .el-textarea__inner{
  max-height:56vh !important;
  font-size:16px;
}
.el-icon-zoom-in {
  cursor: pointer;
  color: rgb(64, 158, 255);
  font-weight: 600;
  font-size: larger;
  right: 2px;
  top: 2px;
  position: absolute;
}
 @import '../components/style.scss';
</style>

