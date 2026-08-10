<!--
 * @description:目标责任书/工作指标
 * @Author: Calvin
 * @Date: 2022-03-07 17:59:44
 * @FilePath: \src\views\PerformanceManage\TargetBook\WorkTarget\index.vue
 1 create 新建； 2 edit 编辑 开放全部编辑权限； 3 examine 审核 按照流程配置开放编辑； 4 preview 查看
-->
<template>
  <div class="target-book-container" ref="TargetBookWorkTarget">
    <el-button icon="el-icon-back" @click="goback" v-if="!isInFlow">返 回</el-button>
    <!-- height: calc(100% - 60px); -->
    <el-card class="box-card mt-10" style="overflow:auto;" shadow="never">
      <h3>
        <div style="display:flex;align-items: center;justify-content: center;font-size:18px;">
          <span>目标责任书(工作指标)</span>
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
        <el-form-item label="考核周期" style="border-right: none;width:209px;"  v-if="assessmentCycle||searchFlowType=='year_kpi_work_target'">
          <!-- <el-select v-model="assessmentCycle" placeholder="请选择" :disabled="isDisabled('assessmentCycle')" style="width: 110px;">
            <el-option label="半年" value="half_year"></el-option>
            <el-option label="年终" value="year"></el-option>
          </el-select> -->
          <span>{{ ({ year: '年终', half_year: '半年' })[assessmentCycle] }}</span>
        </el-form-item>
        <el-form-item label="考核周期" prop="assessmentCycle" style="border-right: none;width:209px;" v-else>
          <el-select v-model="form.assessmentCycle" placeholder="请选择" :disabled="isDisabled('assessmentCycle')" style="width: 110px;">
            <el-option label="半年及年终" value="year_and_half_year"></el-option>
            <el-option label="年终" value="year"></el-option>
          </el-select>
        </el-form-item>
        </div>
        <div class="btn-box" v-if="(actionType == 'create' || actionType == 'edit')&&!assessmentCycle">
          <el-button icon="el-icon-plus"  @click="handleInsert">插入行</el-button>
          <el-button type="danger" icon="el-icon-delete"  :disabled="deleteDisable"
            @click="handleDel">删除行</el-button>
          <!-- <el-link :href="downloadExcelUrl" target="_blank" style="margin:0 10px" :underline="false">
            <el-button type="success" plain icon="el-icon-download" round>模板下载</el-button>
          </el-link> -->
          <el-button type="primary" plain icon="el-icon-download"  @click="downLoadTemplate">模板下载</el-button>
          <el-button type="primary" icon="el-icon-document-add" @click="handleImport"
            >导入数据</el-button>
          <!-- <el-button type="primary" round @click="submit('form')">提交</el-button> -->
        </div>
        <el-table :data="form.keyPerformanceIndicatorsList" :key="forReload" highlight-current-row ref="table" border align="center" :max-height="tableMaxHeight"
          style="width: 100%;" :row-class-name="tableRowClassName" @row-click="handleRowClick" :summary-method="getSummaries" :showSummary="true" class="main-table">
          <el-table-column label="序号" type="index" width="40" align="center" >
          </el-table-column>
          <el-table-column label="目标项目(一级)" width="100">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.indicatorsType.id }}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.indicatorsType.id'"
                  :rules='rules.indicatorsType' >
                  <el-select style="width: 100%;" v-model="scope.row.indicatorsType.id" filterable placeholder="请选择" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')"><!-- isDisabled('target') -->
                    <el-option v-for="item in targetList" :key="item.id" :label="item.name" :value="item.id"
                      >
                    </el-option>
                  </el-select>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="目标项目(二级)" width="140">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.targetItemTwo}}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.targetItemTwo'"
                  :rules='rules.targetItemTwo'>

                  <el-input type="textarea" size="medium" :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.targetItemTwo" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')"> <!-- isDisabled('targetItemTwo') -->
                  </el-input>
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
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.content'"
                  :rules='rules.content'  style="position: relative;">
                  <el-input type="textarea" size="medium" :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.content" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')"><!-- isDisabled('content') -->
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '具体目标项目内容')" ></i>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="权重" width="60" align="center">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.weight }}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.weight'" :rules='rules.weight'>
                  <el-input type="number" size="mini" v-model="scope.row.weight" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')" class="weight-class" ><!-- isDisabled('weight') -->
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
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentMethod'"
                  :rules='rules.assessmentMethod' style="position: relative;">
                  <el-input type="textarea" size="medium" :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.assessmentMethod" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')"><!-- isDisabled('assessmentMethod') -->
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '目标完成标准(考核标准)')" ></i>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="目标完成时间节点">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.assessmentTime}}
              </template> -->
              <template>
                <el-form-item class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentTime'"
                  :rules='rules.assessmentTime' style="position: relative;">
                  <el-input size="medium" :autosize="{minRows: 1,maxRows:5 }" type="textarea"
                    v-model="scope.row.assessmentTime" :disabled="isTranspondFlow || (actionType !='create' && actionType !='edit' && actionType != 'examine')"></el-input><!-- isDisabled('assessmentTime') -->
                    <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '目标完成时间节点')" ></i>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="完成情况描述" v-if="false"><!--v-if="!isDisabled('assessmentRemarks')" -->
            <template slot-scope="scope">
                <el-form-item class="elFormExpanded" v-if="isDisabled('assessmentRemarks')" key="assessmentRemarks">
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.assessmentRemarks"
                    :disabled="isDisabled('assessmentRemarks')"><!-- isDisabled('assessmentRemarks') -->
                  </el-input>
                </el-form-item>
                <el-form-item
                class="elFormExpanded"
                v-else
                :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentRemarks'"
                :rules="rules.assessmentRemarks"
                >
                  <el-input
                    type="textarea"
                    size="medium"
                    :autosize="{minRows: 1,maxRows:5 }"
                    v-model="scope.row.assessmentRemarks"
                    :disabled="isDisabled('assessmentRemarks')"><!-- isDisabled('assessmentRemarks') -->
                  </el-input>
                </el-form-item>
            </template>
          </el-table-column>
          <el-table-column v-if="false"
            label="得分"
            width="60">
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
          <el-table-column label="指标分解" style="position: relative;">
            <template slot-scope="scope" >
              <div v-html="scope.row.resolveContent" style="max-height: calc(23* 5px);overflow-y: scroll;"></div>
              <i class="el-icon-zoom-in" title="放大" @click.stop="showBigInput(scope.row, '指标分解')" ></i>
            </template>
          </el-table-column>
          <!--  v-if="!isDisabled('halfYearAssessmentRemarks')||(assessmentCycle=='year'&&assessmentCycleType=='year_and_half_year')||(searchFlowType=='year_kpi_work_target'&&actionType=='preview')" -->
          <el-table-column label="半年完成情况描述" v-if="showYearHalfCol">
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
          <!-- v-if="!isDisabled('yearAssessmentRemarks')||(searchFlowType=='year_kpi_work_target'&&assessmentCycle=='year'&&actionType=='preview')" -->
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
                  >  <!-- :disabled="isDisabled('yearAssessmentRemarks')||yearKpiScoreGroup.length>0" -->
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
          <el-table-column label="操作" width="90" align="left">
            <template slot-scope="scope">
              <!-- <el-button type="text" size="small" @click="viewDetail(scope.row)"> -->
              <el-button  type="text" size="small" @click="viewDetail(scope.row)">
                查看完成详情
              </el-button>
              <!-- <el-button v-if="actionType != 'examine' && actionType != 'preview' && !isExamine" type="text" size="small" @click="openResolveDialog(scope.row)" style="margin-left: 0;">
                指标分解
              </el-button> -->
              <el-button v-if="actionType =='create' || actionType =='edit' || actionType == 'examine'" type="text" size="small" @click="openResolveDialog(scope.row)" style="margin-left: 0;">
                指标分解
              </el-button>
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
        <div class="grade">
          <span v-if="totalGrade > 0">
            总分 {{ totalGrade - 0 }}
          </span></div>
      </div> -->
    </el-card>
    <!-- 指标分解 -->
    <IndexResolveDialog :visible.sync="IndexResolveVisible" v-if="IndexResolveVisible" @resolveContent="resolveContent" :kpiSplitItems="kpiSplitItems" :isHalfOrYearType="isHalfOrYearType"/>

    <ExaminerDialog :visible.sync="examinerVisible" :examinerId="form.examiner.id" v-if="examinerVisible"
      @select="handleExaminerSelect" />
    <UploadDialog :visible.sync="importDialogVisible" @success="handleImportSuccess" assessmentCycle="year"/>

    <!-- <div class="footer-bt" v-if="isReInitiate">
      <div class="footer-inner" >
        <el-button type="primary" @click="reSubmit('save')">保存草稿</el-button>
        <el-button type="primary" @click="reSubmit('submit')">提交</el-button>
      </div>
    </div> -->
    <div class="footer-bt" v-if="!isReInitiate">
      <div class="footer-inner" v-if="actionType =='create' || actionType =='edit'">
        <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
        <el-button @click="goback" plain v-if="!isInFlow">取 消</el-button>
        <el-button @click="cancel" plain v-if="isInFlow">取 消</el-button>
        <el-button type="primary" plain v-if="!isInFlow" @click="save">保 存</el-button>
        <el-button type="primary" plain @click="submit('not_submitted')" v-if="isInFlow">保 存</el-button>
        <el-button type="primary" @click="submit('under_review')" v-if="isInFlow">提 交</el-button>
      </div>
    </div>
    <el-dialog :visible="viewVisible" title="" :close-on-click-modal="false" width="80%" @close='handleClose'
      class="examiner-dialog" append-to-body>
        <div>
          <el-tabs type="border-card" v-model="activeName">
            <el-tab-pane :name="item.name" v-for="(item, index) in tabList" :key="index">
              <span slot="label"> {{ item.label }}<i :style="{ 'color': item.color }" style="font-style: normal;">({{
                item.number }})</i></span>
              <TaskTable :planId="kpiId" :taskType="item.name" :fetchUrl="fetchUrl" v-if="(activeName == item.name)" v-bind="$attrs" from='targetView'/>
            </el-tab-pane>
          </el-tabs>
        </div>
    </el-dialog>
<!--
    <CheckTaskDdialog v-dialogDraw  :visible.sync="viewVisible" :title="checkTaskDialogTitle"
      :planId="kpiId" :noBeginNum="notSubmitNum" :checkingNum="pendingReviewNum" :finishedNum="finishedNum"
      :fetchUrl="fetchUrl" from="targetView" /> -->
      <el-dialog v-if="bigInputVisible" :visible="bigInputVisible" :title="currentBigTitle" :close-on-click-modal="false" width="80%"
      @close="bigInputVisible = false" append-to-body style="height:100%;">
      <div v-if="currentBigTitle == '指标分解'">
        <div v-html="currentEditText" style="max-height:500px;overflow-y:auto;"></div>
      </div>
      <div style="position:relative;" v-else>
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
import Api from '@/api';
import IndexResolveDialog from './components/IndexResolveDialog';
import ExaminerDialog from './components/ExaminerDialog';
import UploadDialog from '../components/UploadDialog';
import Sortable from 'sortablejs';
import { localstorageGet } from '@/utils/auth.js';
import TaskTable from '@/views/ProjectManage/developProgress/components/TaskTable.vue';
import moment from 'moment';
import math from '@/utils/math.js';
import { deepClone } from '@/utils';
export default {
  name: 'TargetBookWorkTarget',
  components: { UploadDialog, ExaminerDialog, IndexResolveDialog, TaskTable },
  props: {
    bizId: {
      type: String,
      default: ''
    },
    actionType: { // actionType：examine审核，create新增，edit编辑,preview详情
      type: String,
      default: 'create'
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    isReInitiate: {
      type: Boolean,
      default: false
    },
    flowProxyId: { // 流程id
      type: String,
      default: ''
    },
    flowInstanceId:{
      type:String,
      default:''
    },
    searchFlowType:{
      type:String,
      default:''
    },
    selectFlowType:{
      type:String,
      default:''
    },
    isTranspondFlow: {
      type: Boolean,
      default: false
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
  inject:
  {
    prevStepHandle: { value: 'prevStepHandle', default: null },
    sumbitFlow: { value: 'sumbitFlow', default: null },
    submitFlowFinal: { value: 'submitFlowFinal', default: null },
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
      tableIndex: '',
      IndexResolveVisible: false,
      forReload: new Date().getTime(),
      editId: '',
      isGoback: false,
      deleteDisable: false,
      currentRowIndex: null,
      editType: 1,
      importDialogVisible: false,
      examinerVisible: false,
      downloadExcelUrl: 'https://fserver.runshihua.com/files//200001/5ece72d13dc7407bb66b9e2e290ae381.xlsx', // 下载模板的地址
      targetList: [],
      company: localstorageGet('companyName'),
      form: {
        manageType: 'work_target', // 指标类型
        name: '',
        targetTime: moment().format('YYYY'),
        departmentName: '',
        examiner: {
          id: '', // 审核人id
          name: ''
        },
        assessmentCycle: '', // 考核周期
        keyPerformanceIndicatorsList: [
          {
            indicatorsType: { // 管理指标对应的是：目标项目；工作指标对应的是目标项目（一级）
              id: ''
            },
            targetItemTwo: '',
            content: '',
            weight: '',
            assessmentMethod: '',
            assessmentTime: '',
            assessmentRemarks: '',
            score: '',
            resolveContent: '',
            kpiSplitItems: [
              // {
              //   targetTime:0,//分解项的目标时间，整型，季度1-4，月度1-12
              //   content:"",//考核标准
              //   kpiSplitType:"",//  quarterly为季度，month为月度，季度kpiSplitItemWeights不能为null
              //   kpiSplitItemWeights:[{//月度占比
              //     targetTime:0,//月底占比目标时间，1季度1-3，2季度4-6，3季度7-9，4季度10-12
              //     weight:0//占比
              //   }]
              // }
            ]
          }
        ]
      },

      // 表单验证规则
      rules: {
        'examiner.name': [
          {
            required: true,
            trigger: 'change',
            message: '请输入主考核人'
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
          message: '请选择考核周期'
        }],
        indicatorsType: [{
          required: true,
          trigger: 'change',
          message: '请选择目标项目(一级)'
        }],
        targetItemTwo: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入目标项目(二级)'
        }],
        content: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入具体目标项目内容'
        }],
        weight: [{
          required: true,
          trigger: 'blur',
          message: '请输入权重'
        }, {
          pattern: /^[+-]?(0|([1-9]\d*))(\.\d+)?$/g,
          message: '请输入数字',
          trigger: 'blur'
        }],
        assessmentMethod: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入目标完成标准(考核标准)'
        }],
        assessmentTime: [{
          required: true,
          trigger: 'blur',
          message: '请输入目标完成时间节点'
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
      permisionList: [],
      isInFlow: true,
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
      kpiId: '',
      fetchUrl: '',
      viewVisible: false,
      checkTaskDialogTitle: '查看完成情况详情',
      notSubmitNum: 0,
      finishedNum: 0,
      pendingReviewNum: 0,
      detailtableData: [],
      activeName: 'waiting_send',
      isExamine:false,
      scaleVal:localstorageGet('annual_scaleVal') || 10,
      yearKpiScoreGroup:[],
      currentHalfOrYear: '' // 通过流程模板名称后缀信息来判断是半年还是年终
    };
  },
  computed: {
    isHalfOrYearType() {
      return this.selectFlowType == 'year_kpi_work_target' || this.searchFlowType == 'year_kpi_work_target';
    },
    totalGrade() {
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.score - 0));
      }, 0);
      return result.toFixed(2);
      // return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    totalWeight() {
      const result = this.form.keyPerformanceIndicatorsList.reduce((prev, cur) => {
        return Number(prev + (cur.weight - 0));
      }, 0);
      return result.toFixed(2);
      // return ((result * 1000000000).toFixed(0)) / 1000000000;
    },
    isDisabled() {
      return (prop) => {
        if (!this.permisionList.length) {
          return true;
        } else {
          if (this.permisionList.indexOf(prop) > -1) {
            return false;
          } else {
            return true;
          }
        }
      };
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
    searchFlowType(val){
      this.getWorkTargetDetail(val)
    }
  },
  mounted() {
    // var doc = document.querySelector('.el-dialog__headerbtn i');
    // doc.style.fontSize = '28px';
    // doc.classList.add('el-icon-error');
  },
  created() {
    (this.searchFlowType == 'year_kpi_work_target' || this.selectFlowType == 'year_kpi_work_target') && this.flowTemplateFindById();
    console.log(this.searchFlowType,'this.searchFlowType+++++++++',this.actionType)
    if (this.$route?.query?.editType) {
      this.editType = this.$route.query.editType;
      this.isInFlow = false;
    }
    if (this.$route?.query?.isExamine) {
      this.isExamine = true;
    }
    if (this.actionType !== 'create') this.editType = 2;
    if (this.editType == 2) { // editType：新增1，编辑2（业务页面给的状态）
      this.editId = this.$route.query.id || this.bizId;
      this.getWorkTargetDetail(this.searchFlowType);

    }else{
      this.$nextTick(() => {
        this.calcateInputHeight()
      })
      this.getPersonInfo();
    }
    this.getIndicatorsTypeList();
    // console.log('isInFlow',this.isInFlow) // true
    // console.log('action',this.actionType) // examine
    // console.log('editType',this.editType) // 2
    if (this.isInFlow) { // 是否是流程
      if (this.actionType == 'examine') {
        this.getInputPermision().then(res => {
          res.push('detailButton');
          this.permisionList = res;
        });
      } else if (this.actionType == 'create' || this.actionType == 'edit') {
        this.getPermisionForCreate().then(res => {
          this.permisionList = res;

        });
      } else if (this.actionType == 'preview') {
        this.permisionList = [];
      }
    } else {
      this.permisionList = ['name', 'company', 'departmentName',
        'examinerName', 'assessmentCycle', 'target',
        'targetItemTwo', 'content', 'weight',
        'assessmentMethod', 'assessmentTime'];
      // 'assessmentRemarks']
    }

    // this.$bus.$off('annual_perf_validate')
    // this.$bus.$on('annual_perf_validate', val => {
    //   console.log('annual_perf_validate')

    //   if (this.validateForm('form')) {
    //     this.$bus.$emit('validateEnd',true);
    //   }
    // });
    this.$bus.$off('annual_perf_before_handle');
    this.$bus.$on('annual_perf_before_handle', (val,that) => {
      this.examneObj = that //把审核组件实例传过来,方便后面把loading取消
      if (this.status == 'not_submitted') {
        this.postData();
      } else {
        this.status = val;
        // this.submitData('form').then(res=>{
        this.submitData(that?.isTemporarySave || this.validateForm('form')).then(res => {
          const name = `${this.form.targetTime}年度目标责任书（工作指标）-${this.workTargetData?.userName || localstorageGet('userName')}`;
          const obj = {
            status: 'success',
            name
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        }).catch(err => {
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          const obj = {
            status: 'fail'
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        });
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
          const name = `${this.form.targetTime}年度目标责任书（工作指标）-${localstorageGet('userName')}-${enums[this.assessmentCycle]}`;
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
      this.$bus.$emit('targetType', 1);
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
      if (title == '指标分解') {
        this.currentEditText = row.resolveContent;
      } else if (title == '目标完成时间节点') {
        this.currentEditText = row.assessmentTime;
        this.showBigInput.showType = 'assessmentTime';
      } else if (title == '半年完成情况描述') {
        this.currentEditText = row.halfYearAssessmentRemarks;
        this.showBigInput.showType = 'halfYearAssessmentRemarks';
      } else if (title == '年终完成情况描述') {
        this.currentEditText = row.yearAssessmentRemarks;
        this.showBigInput.showType = 'yearAssessmentRemarks';
      } else if (title == '具体目标项目内容') {
        this.currentEditText = row.content;
        this.showBigInput.showType = 'content';
      } else if (title == '目标完成标准(考核标准)') {
        this.currentEditText = row.assessmentMethod;
        this.showBigInput.showType = 'assessmentMethod';
      }
      this.bigInputVisible = true;
      this.currentBigRow = row;
      this.currentBigTitle = title;
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
    calculateWeightScore(index) {
      const pretendScore = this.form.keyPerformanceIndicatorsList[index].pretendScore || 0;
      const score = this.form.keyPerformanceIndicatorsList[index].score || 0;
      let weight = this.form.keyPerformanceIndicatorsList[index].weight || 0;
      //weight = weight * 100;
      let weightScore = 0;
      if (this.form.keyPerformanceIndicatorsList[index].maxScore > 0) {
        weightScore = (math.add(pretendScore * 0.3, score * 0.7) * weight) / this.form.keyPerformanceIndicatorsList[index].maxScore;
        weightScore = weightScore.toFixed(2);
        return weightScore;
      } else {
        return 0;
      }
    },
    // 指标分解内容
    resolveContent(value, type) {
      let newStr = '';
      if (type == 'month') {
        newStr = value.reduce((str, curvalue, index) => {
          return str + '<p>' + Number(index + 1) + '、<span class="time-tag-style">' + curvalue.targetTime + '月</span> ' + curvalue.content + '</p>';
        }, '');
      } else {
        newStr = value.reduce((str, curvalue, index) => {
          const ratioStr = curvalue.kpiSplitItemWeights.reduce((str2, curvalue2) => str2 + curvalue2.targetTime + '月占比' + curvalue2.weight + '%,', '').replace(/,([^,]*)$/, '$1');
          return str + '<p>' + Number(index + 1) + '、<span class="time-tag-style">第' + curvalue.targetTime + '季度</span> ' + curvalue.content + '(' + ratioStr + ')</p>';
        }, '');
      }
      this.form.keyPerformanceIndicatorsList[this.tableIndex].kpiSplitItems = value;
      this.$set(this.form.keyPerformanceIndicatorsList[this.tableIndex], 'resolveContent', newStr);

    },
    openResolveDialog(row) {
      this.IndexResolveVisible = true;
      this.tableIndex = row.row_index;
      this.kpiSplitItems = row.kpiSplitItems;
    },
    downLoadTemplate() {
      const data = {
        code: 'year_work_kpi_template'
      };
      this.$axios.post(Api.performance.downloadTemplate, { data }).then(res => {
        if (res.isSuccess) {
          const fileUrl = res.data.fileVo.fileUrl;
          const aEle = document.createElement('a');
          aEle.href = fileUrl;
          aEle.target = '_blank';
          aEle.click();
        }
      });
    },
    viewDetail(row) {
      this.tabList[0].number = row.notSubmit;
      this.tabList[1].number = row.pendingReview;
      this.tabList[2].number = row.finished;
      // this.notSubmitNum=row.notSubmit
      // this.finishedNum=row.finished
      // this.pendingReviewNum=row.pendingReview
      this.kpiId = row.id;
      this.fetchUrl = Api.taskManage.taskArrange.getTargetCompleteInfo;
      this.viewVisible = true;
      // let monthKpiList = row.monthKpiList
      // monthKpiList.forEach(item=>{
      //   item.weightScore = this.calculateWeightScore(item)
      //   item.createDate = item.createDate.substr(0,10)
      //   item.title = item.indicatorsType.name
      // })
      // this.detailtableData = row.monthKpiList
    },
    handleClose() {
      this.viewVisible = false;
    },
    getPermisionForCreate() {
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      return new Promise((resolve, reject) => {
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
            resolve(enableList);
          }
        );
      });
    },
    cancel() {
      if (this.$route?.params?.from) {
        this.$router.go(-1);
      } else {
        this.prevStepHandle();
      }
    },
    // 获取输入的权限
    getInputPermision() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.findApprovePermission,
          {
            data: {},
            nodeProxyId: this.flowNodeProxyId
          },
          (res) => {
            let enableList = [];
            if (res.data && res.data.flowNodeFieldPowerTemplateList) {
              const tmpList = res.data.flowNodeFieldPowerTemplateList || [];
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName;
              });
            }
            resolve(enableList);
          }
        );
      });
    },
    handleImportSuccess(data) {
      data.kpiList.forEach(item => {
        if (!item.indicatorsType) {
          item.indicatorsType = {
            id: ''
          };
        }
        item.weight = math.multiply(item.weight, 100)
      });


      this.form.keyPerformanceIndicatorsList = data.kpiList;
      this.$nextTick(() => {
        this.$refs.form.clearValidate()
        setTimeout(()=>{
          this.calcateInputHeight()
        })
      })
    },
    getCompanyListOfOnDuty(id) {
      const data = {
        id,
        flag: 'company'
      };
      return this.$axios.post(Api.taskManage.myTask.findUserDetail, { data });
    },
    getDepartmentList(relationId) {
      const data = {
        relationId,
        type: 'company'
      };
      return this.$axios.post(Api.frameworkInfo.getUperDepartmentList, { data });
    },
    getWorkTargetDetail(type) {
      this.$axios.post(
      type=='year_kpi_work_target'?Api.performance.findKpiByYearScoreId:Api.performance.getWorkTargetDetail, // 放开
        {
          data: {
            id: this.editId
          }
        },
        res => {

          const data = this.workTargetData = res.data;
          this.createrId = data.createrId;
          this.company = res.data.companyName||localstorageGet('companyName')
          if(type=='year_kpi_work_target'){
            var li = data.yearKpiScoreGroupList || []
            this.yearKpiScoreGroup = li.map(item=>{
              return item.assessmentCycle
            })
          }
          this.form.examiner = data.examiner;
          this.form.departmentName = data.depName;
          this.form.targetTime = data.targetTime + '';
          this.form.assessmentCycle = data.assessmentCycle;
          data.keyPerformanceIndicatorsList.sort((a, b) => {
            return a.sort - b.sort;
          });
          const permisionList = []
          this.form.keyPerformanceIndicatorsList = data.keyPerformanceIndicatorsList.map(item => {
            // item.grade = item.score
            item.weight = math.multiply(item.weight, 100); // item.weight *= 100
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
          // console.log('=======this.form.keyPerformanceIndicatorsList=========',this.form.keyPerformanceIndicatorsList)
          this.resolveDetailContent();
          this.$nextTick(()=>{
            setTimeout(()=>{
              this.calcateInputHeight()
              // this.permisionList = this.permisionList.concat(permisionList)
            },500)
          })
          this.getCompanyListOfOnDuty(data.createrId).then(res => {
            if (res.isSuccess) {
              const data = res.data;
              this.form.name = data.name;
              const companyId = data.companyId;
              const departmentId = data.userDutyVos[0]?.departmentId || '';
              if (companyId && departmentId) {
                this.getDepartmentList(companyId).then(res => {
                  if (res.isSuccess) {
                    const find = res.data.find(item => item.id == departmentId);
                    if (find) {
                      // this.form.departmentName = find.departmentName;
                    }
                  }
                });
              }
            }
          });
        }
      );
    },
    // 指标分解内容
    resolveDetailContent() {
      this.form.keyPerformanceIndicatorsList.forEach((x, xIndex) => {
        if (x.kpiSplitItems) {
          const type = x.kpiSplitItems[0].kpiSplitType;
          const value = x.kpiSplitItems;
          let newStr = '';
          if (type == 'month') {
            newStr = value.reduce((str, curvalue, index) => {
              return str + '<p>' + Number(index + 1) + '、<span class="time-tag-style">' + curvalue.targetTime + '月</span> ' + curvalue.content + '</p>';
            }, '');
          } else {
            newStr = value.reduce((str, curvalue, index) => {
              const ratioStr = curvalue.kpiSplitItemWeights.reduce((str2, curvalue2) => str2 + curvalue2.targetTime + '月占比' + curvalue2.weight + '%,', '').replace(/,([^,]*)$/, '$1');
              return str + '<p>' + Number(index + 1) + '、<span class="time-tag-style">第' + curvalue.targetTime + '季度</span> ' + curvalue.content + '(' + ratioStr + ')</p>';
            }, '');
          }
          this.$set(this.form.keyPerformanceIndicatorsList[xIndex], 'resolveContent', newStr);
        }
      });
      // console.log('keyPerformanceIndicatorsList',this.form.keyPerformanceIndicatorsList)
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
          const data = res.data;
          this.createrId = data.id;
          this.form.name = data.name;
          const userDutyVosList = data.userDutyVos.filter(item => {
            return item.dutyType == 1;
          });
          const {
            departmentId
          } = userDutyVosList[0];
          this.findDepartmentName(departmentId);
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
            this.form.departmentName = res.data.departmentName;
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
          manageType: 'work_target'
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
      this.$router.push({
        path: '/manpowerResource/performanceManage/targetBook'
      });
    },
    // 对部分表单字段进行校验
    validateField(form, index) {
      let result = true;
      for (const item of this.$refs[form].fields) {
        if (item.prop.split('.')[1] == index) {
          this.$refs[form].validateField(item.prop, (error) => {
            if (error != '') {
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
      this.currentRowIndex = row.row_index;
      if (this.editType == 2) {
        if (row.id) {
          this.deleteDisable = !row.canDelete;
        } else {
          this.deleteDisable = false;
        }
      }
    },
    clearValidate() {
      this.$refs.form.clearValidate();
    },
    getSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        // console.log('column',column)
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
    // 插入行
    handleInsert() {
      const itemData = {
        indicatorsType: { // 管理指标对应的是：目标项目；工作指标对应的是目标项目（一级）
          id: ''
        },
        targetItemTwo: '',
        content: '',
        weight: '',
        assessmentMethod: '',
        assessmentTime: '',
        assessmentRemarks: '',
        score: ''
      };
      if (!this.currentRowIndex) {
        this.form.keyPerformanceIndicatorsList.push(itemData);
      } else {
        this.form.keyPerformanceIndicatorsList.splice(this.currentRowIndex + 1, 0, itemData);
      }
      this.$nextTick(() => {
        this.clearValidate();
        this.calcateInputHeight()
      })
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
    // 删除行
    handleDel() {
      if (this.currentRowIndex === null) {
        this.$message.error('请选中一条数据');
        return;
      }
      this.$confirm('确认删除行！', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
        closeOnClickModal: false
      }).then(() => {
        // 只有一条数据的时候，不允许删除
        if (this.form.keyPerformanceIndicatorsList.length <= 1) {
          this.$message.error('至少保留一条数据');
          return;
        } else {
          if (this.currentRowIndex === null) {
            this.$message.error('请选中一条数据');
            return;
          }
          this.form.keyPerformanceIndicatorsList.splice(this.currentRowIndex, 1);
          if (this.form.keyPerformanceIndicatorsList.length <= 1) {
            this.currentRowIndex = null;
          } else {
            if (this.currentRowIndex != 0) {
              this.currentRowIndex = this.currentRowIndex - 1;
            }
            this.$refs.table.setCurrentRow(this.form.keyPerformanceIndicatorsList[this.currentRowIndex]);
          }
        }
        this.$nextTick(() => {
          this.clearValidate();
        });
      }).catch(() => {
      });
    },
    // 导入数据
    handleImport() {
      this.importDialogVisible = true;
    },
    submit(status) {
      if (status == 0) {
        this.status = 'not_submitted';
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
        // this.submitData(this.validateForm('form')).then(() => {
        //   this.prevStepHandle();
        // });
      } else {
        this.status = status;
        if (status == 'under_review') {
          // console.log('this.isExamine',this.isExamine)
          if (this.isExamine) {
            // 发起目标责任书考核，重新提交流程和业务
            this.postData();
          } else {
            // console.log('into_sumbitFlow')
            // 将表单校验单独剥离出来，为了先校验表单，然后再执行业务（表单节点等）判断，避免先弹窗自选节点，然后才高亮报错表单
            if (this.validateForm('form')) {
              this.sumbitFlow('submit');
            }
          }
        } else {
          this.sumbitFlow('draft');
        }
      }
      // 提交之前的校验,之后自动调用 postData方法提交业务和绑定流程
    },
    save() {
      // console.log('save')
      this.status = 'not_submitted';

      this.submitData(this.validateForm('form')).then(() => {
        this.$message.success('操作成功');
        this.$router.go(-1);
      }).catch(err => {
        console.log('err', err);
      });
    },
    // 提交流程,绑定业务
    postData() {
      this.submitData(this.validateForm('form')).then(res => {
        // const year = moment().format('YYYY')
        // const name = `${year}年度目标责任书（工作指标）-${localstorageGet('userName')}`;
        var name = `${this.form.targetTime}年度目标责任书（工作指标）-${localstorageGet('userName')}`;
        if (this.selectFlowType == 'year_kpi_work_target') {
          var enums = { year: '年终', half_year: '半年' };
          name = `${this.form.targetTime}年度目标责任书（工作指标）-${localstorageGet('userName')}-${enums[this.assessmentCycle]}`;
        }
        this.submitFlowFinal(true, res.id, '', '', name);
        // this.submitFlowFinal(true, res.id);
      }).catch();
    },
    // 校验表单
    validateForm(form) {
      // console.log('validateForm')
      let result = true;
      for (const item of this.$refs[form].fields) {
        this.$refs[form].validateField(item.prop, (error) => {
          if (error != '') {
            console.log('error', item.prop);
            result = false;
          }
        });
      }
      // 校验报错页面定位
      setTimeout(() => {
        const element = document.getElementsByClassName('is-error')[0];
        if(element){
          element.scrollIntoView({
            behavior: 'smooth',
            block: 'center'
          });
        }
      }, 200);
      return result;
    },
    // 提交业务
    submitData(result) {
      // console.log('submitData')
      return new Promise((resolve, reject) => {
        // let result = true;
        // console.log('1')
        // return;
        // for (const item of this.$refs[form].fields) {
        //   this.$refs[form].validateField(item.prop, (error) => {
        //     if (error != '') {
        //       result = false;
        //     }
        //   });
        // }
        if (result) {
          // 总权重不能大于1
          if (this.totalWeight != 100) {
            this.$parent.$parent.$parent.submitLoading = false
            if(this.examneObj)this.examneObj.submitLoading = false
            this.$message.error('总权重必须为100%');
            reject();
            return;
          }
          let postUrl = Api.performance.postKpiGroup;
          var hasResolve = false; // 有指标分解项尚未确认，点击指标分解，进行确认
          for (let i = 0; i < this.form.keyPerformanceIndicatorsList.length; i++) {
            const item = this.form.keyPerformanceIndicatorsList[i];
            item.sort = i;
            if (item.kpiSplitItems && item.kpiSplitItems.length > 0 && !item.resolveContent) {
              this.$message.error(item.targetItemTwo + '有指标分解项尚未确认，请点击指标分解，进行确认');
              hasResolve = true;
              break;
            }
          }
          if (hasResolve) return;
          const params = {
            manageType: 'work_target', // 指标类型
            targetTime: this.form.targetTime,
            examiner: {
              id: this.form.examiner.id // 审核人id
            },
            assessmentCycle: this.form.assessmentCycle,
            keyPerformanceIndicatorsList: deepClone(this.form.keyPerformanceIndicatorsList)
          };
          if (this.editType == 2) {
            params.id = this.editId;
            // params.keyPerformanceIndicatorsList.forEach(item=>{
            //   item.score = item.grade
            // })
            postUrl = Api.performance.updateKpiGroup;
          }
          params.kpiGroupStatus = this.status;
          // console.log(1, params);
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
              kpiScoringType:'work_scoring',
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

                // this.$message.success(this.editType == 2 ? '编辑成功' : '创建成功');
              } else {
                reject();
                this.$parent.$parent.$parent.submitLoading = false
                if(this.examneObj)this.examneObj.submitLoading = false
                this.$message.error(res.message);
              }
              // this.goback();
            }
          );
        } else {
          reject();
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          this.$message.error('请完善目标责任书信息');
        }
      });
    }
  }
};
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

