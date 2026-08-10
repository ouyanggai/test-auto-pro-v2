<template>
  <div class="target-book-container">
    <!-- <el-button icon="el-icon-back" @click="goback" v-if="!isInFlow">返 回</el-button> -->
    <el-card class="box-card mt-10" shadow="never">
      <h3>
        <div class="check-history" @click="checkHistory" v-if="!$attrs.isHistoryDialog">查看历史</div>
        <div style="display:flex;align-items: center;justify-content: center;font-size:18px;">
          <span v-if="this.actionType == 'examine' || this.actionType == 'preview'">{{ dateForm.currentMonth }}</span>
          <el-form v-show="this.actionType != 'examine' && this.actionType != 'preview'" :model="dateForm" :rules="dateRules" ref="dateForm" :inline="true"
            style="margin-bottom:0;margin-right:0;">
            <el-form-item prop="currentMonth">
              <el-date-picker v-model="dateForm.currentMonth" type="month" placeholder="考核月份" format="yyyy-M" ref="datepicker"
                value-format="yyyy-M" :disabled="this.actionType == 'examine' || this.actionType == 'preview'"
                style="width: 90px;" @change="getCreateData" :clearable="false"></el-date-picker>
            </el-form-item>
          </el-form>
          <span>月度绩效考核表</span>
        </div>
        <div class="zoom" v-if="!$attrs.isHistoryDialog">
          <!-- <el-button icon="el-icon-minus" plain circle type="primary" size="mini" @click="zoom(-1,'boxCard','scaleVal')" :disabled="scaleVal< 5"></el-button> -->
          <div class="zoom-button" @click="zoom(-1)" :class="{ 'disabled': scaleVal <= 10 }">-</div>
          <span>{{ scaleVal * 10 }}%</span>
          <div class="zoom-button" @click="zoom(1)" :class="{ 'disabled': scaleVal >= 12 }">+</div>
        </div>
      </h3>
      <el-form :model="form" :rules="rules" ref="form" :inline="true" class="main-form">
        <div class='top-form'>
          <div style="width:50%;display:flex;">
            <el-form-item label="姓名">
              <el-input v-model="form.name" readonly :disabled="isDisabled('name')" style="width:100px;"></el-input>
            </el-form-item>
            <el-form-item label="所属单位" style="flex:1;" class="danwei">
              <el-input v-model="company" readonly :disabled="isDisabled('company')"></el-input>
            </el-form-item>
          </div>
          <div style="width:50%;display:flex;">
            <el-form-item label="部门" style="flex:1;" class="danwei">
              <el-input v-model="form.departmentName" readonly :disabled="isDisabled('departmentName')"></el-input>
            </el-form-item>
            <el-form-item label="考核周期" prop="assessmentCycle" style="flex:1;border-right:none;">
              <el-input v-model="form.assessmentCycle" readonly :disabled="isDisabled('assessmentCycle')"
                style="width:100px;"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- <div class="tb-row flex-box flex-align-center">
          <div class="tb-col tb-title">考勤及补助</div>
        </div> -->
        <!-- <div class="btn-box"> -->
        <div class="file-div">
          <div class="title">月度工作总结</div>
          <span style="padding-left: 10px;">
            <el-form-item prop="fileList">
              <eleupload @change="upChange" :showOnly="actionType != 'create' && actionType != 'edit'" ref="eleupload"
                :size="200" :attachFile="form.attachFile" :showFullName="true"></eleupload>
            </el-form-item>
          </span>
        </div>
        <!-- </div> -->
        <div class="btn-box" v-if="actionType == 'create' || actionType == 'edit'" style="display: flex;flex-direction: row-reverse;justify-content: space-between;">
          <div>
            <el-button icon="el-icon-plus" @click="handleInsert" type="primary" plain>插入行</el-button>
            <el-dropdown trigger="click" size="medium" style="margin:0 10px">
              <el-button type="primary" icon="el-icon-download">模板下载</el-button>
              <el-dropdown-menu slot="dropdown">
                <el-dropdown-item>
                  <el-button type="text" @click="downLoadTemplate('month_kpi_template')">通用模板</el-button>
                </el-dropdown-item>
                <el-dropdown-item>
                  <el-button type="text" @click="downLoadTemplate('month_kpi_template2')">复杂模板(带合并单元格)</el-button>
                </el-dropdown-item>
              </el-dropdown-menu>
            </el-dropdown>
            <!-- <el-button type="primary" icon="el-icon-download" @click="downLoadTemplate('month_kpi_template')">模板下载</el-button> -->
            <el-button type="danger" icon="el-icon-delete" :disabled="deleteDisable" @click="handleDel">删除行</el-button>
          </div>
          <div>
            <el-button type="primary" @click="importRecentData">导入最近一次数据</el-button>
            <el-button type="primary" plain icon="el-icon-document-add" @click="handleImport">导入数据</el-button>
          </div>
        </div>
        <el-table :data="[{}]" border style="width:100%;margin-bottom: 10px;">
          <el-table-column label="本月重点工作计划">
            <template slot-scope="scope">
              <el-input type="textarea" size="medium" :autosize="{ minRows: 5, maxRows: 5 }" show-word-limit
                maxlength="500" v-model="form.currentMonthWork" :disabled="true" style="width: 100%;">
              </el-input>
            </template>
          </el-table-column>
          <el-table-column label="本月重点工作完成情况">
            <template slot-scope="scope">
              <el-input type="textarea" size="medium" :autosize="{ minRows: 5, maxRows: 5 }" show-word-limit
                maxlength="3000" v-model="form.currentMonthDetail" :disabled="isDisabled('targetItemTwo')"
                style="width: 100%;">
              </el-input>
            </template>
          </el-table-column>
        </el-table>
        <el-table :data="form.keyPerformanceIndicatorsList" :key="forReload" highlight-current-row ref="table" border
          align="center" style="width: 100%;" :row-class-name="tableRowClassName" @row-click="handleRowClick"
          :showSummary="true" :summary-method="getSummaries" class="main-table">
          <el-table-column label="序号" type="index" width="35" align="center" style="position: relative;" class-name="drag-handle">
            <template #default="scope">
              <span>{{ scope.$index + 1 }}</span>
              <div class="row-remove-plus" v-if="actionType == 'create' || actionType == 'edit'">
                <i class="el-icon-remove" title="删除行" style="margin-right:2.5px;color:#f56c6c" @click="handleDel(scope.$index)"></i>
                <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="handleInsert(scope.$index)"></i>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="指标" width="100">
            <template slot-scope="scope">
              <!-- <template v-if="actionType == 'preview' || isDisabled('target')"> -->
              <template v-if="actionType !== 'create' && !isReInitiate">
                <!-- {{ targetList.find(item=>item.id == scope.row.indicatorsType.id).name || '' }} -->
                <!-- {{ findName(scope.row.indicatorsType.id) }} -->
                {{ scope.row.indicatorsType.name }}
              </template>
              <template v-else>
                <el-form-item class="elFormExpanded"
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.indicatorsType.id'"
                  :rules='rules.indicatorsTypeId'>
                  <el-select style="width: 100%;" v-model="scope.row.indicatorsType.id" placeholder="请选择"
                    :disabled="isDisabled('target')">
                    <el-option v-for="item in targetList" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="关键职责" width=100>
            <template slot-scope="scope">
              <template v-if="actionType == 'preview' || isDisabled('targetItemOne')">
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ scope.row.targetItemOne }}
                </div>
              </template>
              <template v-else>
                <el-form-item :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.targetItemOne'"
                  :rules='rules.targetItemOne' width="100%">
                  <el-input type="textarea" size="medium" :autosize="{ minRows: 1, maxRows: 5 }"
                    v-model="scope.row.targetItemOne" :disabled="isDisabled('targetItemOne')" style="width: 100%;">
                  </el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="KPI"> <!-- :width="kpiWidth" -->
            <template slot-scope="scope">
              <template v-if="actionType == 'preview' || isDisabled('targetItemTwo')">
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  {{ scope.row.targetItemTwo }}
                </div>
                <i class="el-icon-zoom-in" title="放大查看填写" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="showBigInput(scope.row, {k: 'targetItemTwo', s: 'targetItemTwo', t: 'KPI' })" ></i>
              </template>
              <template v-else>
                <el-form-item class="elFormExpanded"
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.targetItemTwo'" :rules='rules.targetItemTwo'
                  width="100%">
                  <el-input type="textarea" size="medium" :autosize="{ minRows: 1, maxRows: 5 }"
                    v-model="scope.row.targetItemTwo" :disabled="isDisabled('targetItemTwo')" style="width: 100%;">
                  </el-input>
                  <i class="el-icon-zoom-in" title="放大查看填写" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="showBigInput(scope.row, {k: 'targetItemTwo', s: 'targetItemTwo', t: 'KPI' })" ></i>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="权重" width="70" align="center">
            <template slot-scope="scope">
              <!-- <template v-if="scope.row.action == 'view'">
                {{scope.row.weight }}
              </template> -->
              <template v-if="actionType == 'preview' || isDisabled('weight')">
                {{ scope.row.weight }}%
              </template>
              <template v-else>
                <el-form-item :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.weight'" :rules='rules.weight'>
                  <!-- <el-input style="width:100%;" :min=0 :max="100" size="mini" @input="val => maxVal(val, scope.$index, 100, 'weight')"
                    v-model.trim="scope.row.weight" :disabled="isDisabled('weight')"
                    v-focusSelect class="weight-class" @change="handleInputChange(scope.$index)">
                    <template slot="append">%</template>
                  </el-input> -->
                  <el-input size="mini" min="0" max="100" :step="1"
                    @input="val => maxVal(val, scope.$index, 100, 'weight')"
                    :disabled="isDisabled('weight')" v-model.trim="scope.row.weight"
                    v-focusSelect><template slot="append">%</template></el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="计划完成时间">
            <template slot-scope="scope">
              <template v-if="actionType == 'preview' || isDisabled('content')">
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">{{ scope.row.assessmentTime }}</div>
                  <i class="el-icon-zoom-in" title="放大查看填写" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="showBigInput(scope.row, {k: 'assessmentTime', s: 'content', t: '计划完成时间' })" ></i>
              </template>
              <template v-else>
                <el-form-item :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentTime'"
                  :rules='rules.assessmentTime' style="position: relative;">
                  <el-input type="textarea" v-model="scope.row.assessmentTime" style="width: 100%;"
                    :disabled="isDisabled('content')"></el-input>
                    <i class="el-icon-zoom-in" title="放大查看填写" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="showBigInput(scope.row, {k: 'assessmentTime', s: 'content', t: '计划完成时间' })" ></i>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="指标具体描述" min-width=150>
            <template slot-scope="scope">
              <template v-if="isDisabled('content')">               <!-- v-if="!(actionType == 'create')" -->
                <div v-if="scope.row.criteriaItems && scope.row.criteriaItems.length">
                  <template v-for="(item,index) in scope.row.criteriaItems">
                    <section :key="index" v-if="item.criteria || item.score">
                      <div style="display:inline-block;width:86%;">{{ item.criteria}}</div>
                      <div style="display:inline-block;width:14%;float:right;white-space:nowrap;" v-if="item.score != null">（{{item.score}}分）</div>
                    </section>
                  </template>
                </div>
                 <div v-else style="max-height: calc(23* 5px);overflow-y: scroll;cursor:pointer;" @dblclick="doubleClickShow(scope.row.content,'指标具体描述')">
                  <pre style="font:unset;white-space:pre-line;">{{ scope.row.content }}</pre>
                </div>
              </template>
              <template v-else>
                <div class="index-description" v-if="scope.row.criteriaItems && scope.row.criteriaItems.length">
                  <span v-for="(item,index) in scope.row.criteriaItems" :key="index">
                    <el-input :rows="1" :autosize="{ minRows: 1, maxRows: 2 }" type="textarea" size="mini" name="notSetHeight"
                    style="width:85%;resize:vertical" placeholder="描述" v-model="item.criteria" resize="vertical"></el-input>
                    <el-input :rows="1" :autosize="{ minRows: 1, maxRows: 1 }" type="textarea" size="mini"
                    style="width:15%" placeholder="分数" v-model="item.score" @input="_=>{item.score=item.score.replace(/[^0-9]/g, '')}"></el-input>
                  </span>
                </div>
                <el-form-item v-else class="elFormExpanded" :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.content'"
                  :rules='rules.content' style="width: 100%;">
                  <el-input type="textarea" size="medium" :autosize="{ minRows: 1, maxRows: 5 }"
                    v-model="scope.row.content" :disabled="isDisabled('content')" style="width: 100%;">
                  </el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column :label="'年度指标分解'" align="center" :key="'zbfj'" v-if="!isDisabled('annualTarget') || actionType == 'preview'">
            <el-table-column label="完成标准" width="100" :key="'zbfjwcbz'">
              <template slot-scope="scope">
                <div style="max-height: calc(23* 5px);overflow-y: scroll;">
                  <div v-for="val in scope.row.kpiSplitItems" :key="val.id">
                    <span class="time-tag-style" v-if="val.kpiSplitType == 'quarterly'">第{{ val.targetTime }} 季度</span>
                    <span class="time-tag-style" v-else>{{ val.targetTime }} 月</span>
                    {{ val.content }}
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="完成记录" width="70" :key="'zbfjwcjl'">
              <template slot-scope="scope">
                <el-button type="text" @click="kpiDetailContent(scope.row)"
                  v-if="scope.row.kpiSplitItems">详情</el-button>
              </template>
            </el-table-column>
          </el-table-column>
          <!-- <el-table-column label="本月任务完成情况" v-if="actionType != 'create'"> -->
          <!-- <el-table-column label="本月任务完成情况" v-if="false">
            <template slot-scope="scope">
              <el-button type="text" @click="viewDetail(scope.row)">查看</el-button>
            </template>
          </el-table-column> -->
          <el-table-column label="完成情况描述" min-width="150">
            <template slot-scope="scope">
              <template v-if="actionType == 'preview' || isDisabled('assessmentRemarks')">
                <div v-fillHeight style="display: flex;align-items: center;overflow-y: scroll;cursor:pointer;max-height:100px;" @dblclick="doubleClickShow(scope.row.assessmentRemarks, '完成情况描述')">
                  <pre style="font:unset;white-space:pre-line;align-self:start">{{scope.row.remark ? (scope.row.remark + '\n' + '完成情况描述：' + '\n') : ''}}{{ scope.row.assessmentRemarks }}</pre>
                  <i class="el-icon-zoom-in" title="放大" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="doubleClickShow(scope.row.assessmentRemarks, '完成情况描述')" ></i>
                </div>
              </template>
              <template v-else>
                <article v-if="scope.row.performanceType && scope.row.performanceType != 'other' && scope.row.kpiSplitItems" style="font-weight: normal;">
                  <div>{{ scope.row.remark || '' }}</div>
                  本月实际完成:<template v-if="scope.row.kpiSplitItems[0].kpiSplitType == 'month'">
                    <el-input  style="width:60px;margin-left:3px" name="notSetHeight"
                   v-model="scope.row.kpiSplitItems[0].actualAmount" :disabled="isDisabled('assessmentRemarks')"></el-input>万元
                  </template>
                  <template v-else-if="scope.row.kpiSplitItems[0].kpiSplitType == 'quarterly'">
                    <el-input style="width:60px;margin-left:3px" name="notSetHeight"
                    v-model="scope.row.kpiSplitItems[0].kpiSplitItemWeights[0].actualAmount" :disabled="isDisabled('assessmentRemarks')"></el-input>万元
                  </template>
                </article>
                <el-form-item class="elFormExpanded" v-fillHeightTextarea="that"
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.assessmentRemarks'"
                  :rules='rules.assessmentRemarks'>
                  <div style="display: flex;align-items: center;position: relative;">
                    <el-input name="notSetHeight" placeholder="完成情况及其他描述" type="textarea" size="medium" :autosize="{ minRows: 2, maxRows: 5 }" v-model="scope.row.assessmentRemarks" :disabled="isDisabled('assessmentRemarks')">
                    </el-input>
                    <span style="position: absolute;z-index: 1;right:2px;bottom:0;color:#999;font-size: 12px;font-weight: 100;">{{ scope.row.assessmentRemarks ? scope.row.assessmentRemarks.length : 0 }}</span>
                    <!-- 单击放大这个输入框 -->
                    <i class="el-icon-zoom-in" title="放大查看填写" style="position: absolute;right: 2px;top: 2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:larger;" @click.stop="showBigInput(scope.row, {k: 'assessmentRemarks', s: 'assessmentRemarks', t: '完成情况描述'})" ></i>
                  </div>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="自评分30%" width="60">
            <template slot-scope="scope">
              <template v-if="actionType == 'preview' || isDisabled('pretendScore')">
                {{ scope.row.pretendScore }}
              </template>
              <template v-else>
                <el-form-item :rules='rules.pretendScore'
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.pretendScore'">
                  <el-input size="mini" @keydown.native="e => onKeydown(e, scope.$index)" min="0" max="3"
                    @input="val => maxVal(val, scope.$index, scope.row.maxScore, 'pretendScore')" @keypress.native="e => keypress(e, scope.$index)"
                    :disabled="isDisabled('pretendScore')" v-model.trim="scope.row.pretendScore"
                    v-focusSelect></el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="领导评分  70%" width="75">
            <template slot-scope="scope">
              <template v-if="actionType == 'preview'  || isDisabled('score')">
                {{ scope.row.score }}
              </template>
              <template v-else>
                <el-form-item v-if="isDisabled('score')" :rules='rules.score'
                :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.score'">
                  <el-input size="mini" @keypress.native="e => keypress(e, scope.$index)" min="0" max="3"
                    @input="val => maxVal(val, scope.$index, scope.row.maxScore, 'score')" @keydown.native="e => onKeydown(e, scope.$index)"
                    :disabled="isDisabled('score')" v-model.trim="scope.row.score" v-focusSelect></el-input>
                </el-form-item>
                <el-form-item v-else :rules='rules.score'
                  :prop="'keyPerformanceIndicatorsList.' + scope.$index + '.score'">
                  <el-input size="mini" @keypress.native="e => keypress(e, scope.$index)" min="0" max="3"
                    @input="val => maxVal(val, scope.$index, scope.row.maxScore, 'score')" @keydown.native="e => onKeydown(e, scope.$index)"
                    :disabled="isDisabled('score')" v-model.trim="scope.row.score" v-focusSelect></el-input>
                </el-form-item>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="权重得分" width="55">
            <!-- 权重得分 ((自评分*评分比例+领导评分*评分比例)*权重*100)/3 -->
            <template slot-scope="scope">
              <template v-if="actionType == 'preview'">
                {{ calculateWeightScore(scope.$index) }}
              </template>
              <template v-else>
                {{ calculateWeightScore(scope.$index) }}
              </template>
            </template>
          </el-table-column>
          <!-- <el-table-column label="操作" width="230" v-if="actionType!='examine'"> -->
          <el-table-column label="操作" width="105"
            v-if="(actionType == 'create' || actionType == 'edit') && !isDisabled('rowaction')">
            <template slot-scope="scope">
              <!-- <el-button type="text" size="small" @click="viewDetail(scope.row)"> -->
              <!-- <template v-if="scope.row.yearKpi && scope.row.yearKpi.id">
              <el-button type="text" @click="openRelative(scope.$index)">
                已关联
              </el-button>
              <el-button type="text" @click="removeRelative(scope.$index)" v-if="actionType != 'preview' && actionType != 'examine'">
                取消关联
              </el-button>
            </template> -->
              <el-button type="text" size="small" @click="openRelative(scope.row, scope)" :disabled="scope.row.sys"
                v-if="actionType != 'preview' && actionType != 'examine' && !!dateForm.currentMonth">
                关联目标责任书<span v-if="scope.row.yearKpi">(1)</span>
              </el-button><br>
              <el-button type="text" @click="relativeTask(scope.row)"
                v-if="actionType == 'edit' || actionType == 'create'">
                关联任务<span v-if="scope.row.plans && scope.row.plans.length">({{ scope.row.plans.length }})</span>
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="grade-container no-inline" style="border-bottom:0;margin-bottom:10px;margin-top: 10px;">
          <div class="tb-row flex-box flex-align-center" v-if="(actionType == 'create' && $store.state.user.dutyLevel != 'ordinary') || dutyLevelType != 'ordinary'">
            <div class="tb-col tb-title">补分项</div>
            <div style="flex:1;align-self:stretch;" class="border-right border-left">
              <div style="padding:3px;line-height:normal;text-align:left;">
                月度重点工作未按照月分解计划完成，如在次月完成，可视工作成果给予月度绩效补分，补分分数不超过上月扣除权重分；连续两月未完成不予补分。
              </div>
            </div>
            <div style="flex:1;align-self:stretch;" class="border-right">
              <div style="padding:3px;">
                <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                  :disabled="isDisabled('extraPointsKey')" v-model="form.recoupPonitsKey" maxlength="500" show-word-limit
                  placeholder="事由：">
                </el-input>
              </div>
            </div>
            <div style="width: 134px;">
              <div style="padding:3px;">
                <el-input :autosize="{ minRows: 1, maxRows: 5 }" placeholder="补分值"
                  :disabled="isDisabled('extraPointsValue')" @input="val => checkPosive(val, 'recoupPonitsValue')"
                  v-model="form.recoupPonitsValue">
                </el-input>
              </div>
            </div>
          </div>
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">加分项</div>
            <div style="flex:1;align-self:stretch;" class="border-right border-left">
              <div style="padding:3px;line-height:normal;text-align:left;">
                出色地完成领导交办的本职工作以外的工作，可根据工作类型和完成效果加1-20分【工作难度大且完成情况满意度高，根据完成效果可加12-20分；工作有难度或完成情况满意度高，根据完成效果可加6-11分；工作难度一般，可根据完成效果加1-5分】；阶段内进步显著，综合业绩突出的，可视情况加1-20分。
              </div>
            </div>
            <div style="flex:1;align-self:stretch;" class="border-right">
              <div style="padding:3px;">
                <el-input type="textarea" :autosize="{ minRows: 3, maxRows: 5 }"
                  :disabled="isDisabled('extraPointsKey')" v-model="form.extraPointsKey" maxlength="500" show-word-limit
                  placeholder="事由：">
                </el-input>
              </div>
            </div>
            <div style="width: 134px;">
              <div style="padding:3px;">
                <el-input :autosize="{ minRows: 1, maxRows: 5 }" placeholder="加分值"
                  :disabled="isDisabled('extraPointsValue')" @input="val => checkPosive(val, 'extraPointsValue')"
                  v-model="form.extraPointsValue">
                </el-input>
              </div>
            </div>
          </div>
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">扣分项</div>
            <div style="flex:1;align-self:stretch;" class="border-right border-left">
              <div style="padding:3px;line-height:normal;text-align:left;">
                不能接受领导交办岗位职责外工作或完成效果很差，可扣1-10分；阶段内综合情况不佳，或存在其他影响工作开展的情况，视情况扣1-10分。
                <!-- <el-input
                type="textarea"
                :autosize="{ minRows: 2,maxRows: 5 }"
                :disabled="isDisabled('deductPointsCondition')"
                placeholder=""
                v-model="form.deductPointsCondition"
                maxlength="500"
                show-word-limit
                >
              </el-input> -->
              </div>
            </div>
            <div style="flex:1;align-self:stretch;" class="border-right">
              <div style="padding:3px;">
                <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="事由"
                  :disabled="isDisabled('deductPointsKey')" v-model="form.deductPointsKey"
                  @input="val => checkPosive(val, 'deductPointsKey')" maxlength="500" show-word-limit>
                </el-input>
              </div>
            </div>
            <div style="width: 134px;">
              <div style="padding:3px;">
                <el-input :autosize="{ minRows: 1, maxRows: 5 }" placeholder="扣分值"
                  :disabled="isDisabled('deductPointsValue')" @input="val => checkPosive(val, 'deductPointsValue')"
                  v-model="form.deductPointsValue">
                </el-input>
              </div>
            </div>
          </div>
          <!-- <div class="tb-row flex-box flex-align-center" v-if="false">
            <div class="tb-col tb-title">值得肯定的地方和不足与改进</div>
            <div style="width: 50%;align-self:stretch;" class="border-right border-left">
              <div style="padding:3px;">
                <el-form-item :rules='rules.selfEvaluation' prop="selfEvaluation">
                  <el-input type="textarea" :placeholder="isDisabled('selfEvaluation') ? '' : '个人评价：'"
                    :disabled="isDisabled('selfEvaluation')" maxlength="500" show-word-limit
                    v-model="form.selfEvaluation">
                  </el-input>
                </el-form-item>
              </div>
            </div>
            <div style="width: 50%;align-self:stretch;">
              <div style="padding:3px;">
                <el-form-item prop="leaderEvaluation">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                    :disabled="isDisabled('leaderEvaluation')"
                    :placeholder="isDisabled('leaderEvaluation') ? '' : '上级评价：'" maxlength="500" show-word-limit>
                  </el-input>
                </el-form-item>
              </div>
            </div>
            <div style="width: 250px;">
            </div>
          </div> -->
          <div class="tb-row flex-box" v-if="true">
            <div class="border-right"
              style="text-align:left;width:50%;box-sizing:initial;display: flex;flex-direction: column;">
              <div style="display: flex; width: 100%;flex: 1;">
                <div class="tb-row border-right" style="display: flex;align-items: center">
                  <span style="display:inline-block;width:134px;text-align:center;font-weight:bold;">值得肯定的地方</span>
                </div>
                <el-form-item prop="advantage" style="width:100%;padding:5px;border-bottom:1px solid #666">
                  <el-input  type="textarea"
                    :placeholder="isDisabled('advantage') ? '' : '值得肯定的地方'" :disabled="isDisabled('advantage')"
                    :autosize="{ minRows: 2, maxRows: 5 }" maxlength="500" show-word-limit
                    v-model="form.advantage"></el-input>
                </el-form-item>
              </div>
              <div style="display: flex;  width: 100%;flex: 1;">
                <div class="border-right" style="display: flex;align-items: center">
                  <span style="display:inline-block;width:134px;text-align:center;font-weight:bold;">不足与改进</span>
                </div>
                <el-form-item prop="disadvantage" style="width:100%;padding:5px">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" maxlength="500"
                    :placeholder="isDisabled('disadvantage') ? '' : '不足与改进'" :disabled="isDisabled('disadvantage')"
                    show-word-limit v-model="form.disadvantage"></el-input>
                </el-form-item>
              </div>
            </div>
            <div style="padding:5px;width:50%;">
              <el-input class="superiorEvaluationGradeClass" type="textarea" :placeholder="isDisabled('leaderEvaluation') ? '' : '上级评价：'"
                :autosize="{ minRows: 5, maxRows: 5 }" maxlength="500" show-word-limit
                :disabled="isDisabled('leaderEvaluation')" v-model="form.leaderEvaluation" resize="none"></el-input>
            </div>
            <!-- <div style="width: 250px;"> </div> -->
          </div>
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">次月重点工作计划</div>
            <div style="width: 100%;align-self:stretch;" class=" border-left">
              <div style="padding:3px;">
                <el-form-item prop="nextMonthWork">
                  <el-input type="textarea" :autosize="{ minRows: 5, maxRows: 5 }" :disabled="isDisabled('nextMonthWork')"
                    :placeholder="isDisabled('nextMonthWork') ? '' : '次月重点工作计划'" v-model="form.nextMonthWork"
                    maxlength="500" show-word-limit>
                    <!-- show-word-limit -->
                  </el-input>
                </el-form-item>
              </div>
            </div>
          </div>
        </div>
        <h4 style="text-align: center;">以下为绩效考核组填写信息</h4>
        <div class="grade-container no-inline" style="border-bottom:0;margin-bottom:10px;margin-top: 10px;">
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">奖励</div>
            <div style="width: 100%;align-self:stretch;flex:1;" class="border-right border-left">
              <div style="padding:3px;">
                <el-form-item prop="rewardPonitsKey">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                    :disabled="isDisabled('rewardPonitsKey')" maxlength="500" show-word-limit
                    v-model="form.rewardPonitsKey">
                  </el-input>
                </el-form-item>
              </div>
            </div>
            <div style="width: 134px;align-self:center;">
              <div style="padding:3px;">
                <el-form-item prop="rewardPonitsValue">
                  <el-input placeholder="分值" :disabled="isDisabled('rewardPonitsValue')"
                    @input="val => checkPosive(val, 'rewardPonitsValue')" v-model="form.rewardPonitsValue">
                  </el-input>
                </el-form-item>
              </div>
            </div>
          </div>
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">扣罚</div>
            <div style="width: 100%;align-self:stretch;flex:1;" class="border-right border-left">
              <div style="padding:3px;">
                <el-form-item prop="punishPonitsKey">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                    :disabled="isDisabled('punishPonitsKey')" maxlength="500" show-word-limit
                    v-model="form.punishPonitsKey">

                  </el-input>
                </el-form-item>
              </div>
            </div>
            <div style="width: 134px;align-self:center;">
              <div style="padding:3px;">
                <el-form-item prop="punishPonitsValue">
                  <el-input placeholder="分值" :disabled="isDisabled('punishPonitsValue')"
                    @input="val => checkPosive(val, 'punishPonitsValue')" v-model="form.punishPonitsValue">
                  </el-input>
                </el-form-item>
              </div>
            </div>
          </div>
          <template v-if="actionType == 'examine'">
            <div class="tb-row flex-box flex-align-center">
              <div class="tb-col tb-title">岗位转正、任用及调薪情况</div>
              <div style="width: 100%;align-self:stretch;" class=" border-left">
                <div style="padding:3px;">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                    :disabled="isDisabled('changeSalary')"
                    :placeholder="isDisabled('changeSalary') ? '' : '岗位转正、任用及调薪情况'" v-model="form.changeSalary"
                    maxlength="500" show-word-limit>
                    <!-- show-word-limit -->
                  </el-input>
                </div>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="tb-row flex-box flex-align-center">
              <div class="tb-col tb-title">岗位转正、任用及调薪情况</div>
              <div style="width: 100%;align-self:stretch;" class=" border-left">
                <div style="padding:3px;">
                  <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }"
                    :disabled="isDisabled('changeSalary')"
                    :placeholder="isDisabled('changeSalary') ? '' : '岗位转正、任用及调薪情况'" v-model="form.changeSalary"
                    maxlength="500" show-word-limit>
                    <!-- show-word-limit -->
                  </el-input>
                </div>
              </div>
            </div>
          </template>

          <!-- <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">出勤信息</div>
            <div style="width: 100%;align-self:stretch;" class=" border-left">
              <div style="padding:3px;">
                <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 5 }" :disabled="isDisabled('askForLeave')"
                  :placeholder="isDisabled('askForLeave') ? '' : '出勤信息'" v-model="form.askForLeave" maxlength="500"
                  show-word-limit>
                </el-input>
              </div>
            </div>
          </div> -->
          <div class="tb-row flex-box flex-align-center">
            <div class="tb-col tb-title">考勤及补助</div>
            <div style="width: 100%;align-self:stretch;" class="border-left">
              <div style="padding:3px;display: flex;align-items:baseline;">
                <!-- <span style="width: 100px;">{{ jobPerf }}</span><span>岗效工资+基本工资+餐费</span>
                <el-input style="width: 50px;height: 20px;text-align: center;"
                  :placeholder="isDisabled('subsidy') ? '' : '输入餐费'" :disabled="isDisabled('subsidy')"
                  v-model="form.subsidy">
                </el-input>
                <span>元 * </span>
                <el-input style="width: 50px;height: 20px;text-align: center;"
                  :placeholder="isDisabled('workDays') ? '' : '天'" :disabled="isDisabled('workDays')"
                  v-model="form.workDays">
                </el-input> -->
                <div style="margin-left: 5px;flex:1;">
                  <el-form-item prop="askForLeave">
                    <el-input
                      type="textarea" v-model="form.askForLeave" maxlength="500" show-word-limit :disabled="isDisabled('askForLeave')"></el-input>
                  </el-form-item>
                </div>
              </div>
            </div>
          </div>
        </div>
      </el-form>
      <div id="perform-footer" class="grade-container flex-box flex-align-center" style="justify-content: end;"
        v-if="isInFlow && actionType != 'create' && actionType != 'edit'">
        <div class="flex-box" style="padding: 0 5px;">
          <div style="text-align: center;">岗效：</div>
          <div class="weight">
            <span>{{ jobPerf }}</span>
          </div>
        </div>
        <div class="flex-box" style="padding: 0 5px;">
          <div style="text-align: center;">总分：</div>
          <div class="weight">
            <span>{{ totalScoreWeight - 0 }}</span>
          </div>
        </div>
      </div>
    </el-card>
    <monthHistoryDialog :visible.sync="monthHistoryVisible" :initiatorId="initiatorId"></monthHistoryDialog>
    <doubleShowDetailDialog :visible.sync="doubleShowDetailVisible" v-if="doubleShowDetailVisible" :value='doubleShowDetailValue'></doubleShowDetailDialog>
    <div style="height:50px;" v-if="!isInFlow"></div>
    <ExaminerDialog :visible.sync="examinerVisible" :examinerId="form.examiner.id" v-if="examinerVisible"
      @select="handleExaminerSelect" />
    <UploadDialog :visible.sync="importDialogVisible" @success="handleImportSuccess" assessmentCycle="month" />
    <relatedPerfDialog :visible.sync="relatedPerfVisible" :relateRow.sync="relateRow" @comfirmRelate="comfirmRelate"
      :currentRelativId="currentRelativId"></relatedPerfDialog>
    <!-- 关联任务 -->
    <relateTaskDialog :currentMonth="dateForm.currentMonth" :visible.sync="taskVisible" :relateRow.sync="relateRow"
      @selectRelativeTask="selectRelativeTask"></relateTaskDialog>
    <!-- 完成详情 -->
    <el-dialog title="指标完成记录" :visible="completeVisible" :close-on-click-modal="false" :append-to-body="true"
      @close="closecompleteDialog" style="height: 85vh;overflow:hidden;">
      <div style="height: 48vh;overflow:auto;">
        <el-table :data="detailContentList" border style="width: 100%">
          <el-table-column prop="assessmentTime" label="时间">
            <template slot-scope="scope">
              <div v-for="val in scope.row.yearKpi.kpiSplitItems" :key="val.id">
                <span class="time-tag-style" v-if="val.kpiSplitType == 'quarterly'">第{{ val.targetTime }} 季度</span>
                <span class="time-tag-style" v-else>{{ val.targetTime }} 月</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="date" label="完成标准">
            <template slot-scope="scope">
              <div v-for="val in scope.row.yearKpi.kpiSplitItems" :key="val.id">
                {{ val.content }}
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="remark" label="指标完成情况">
            <template slot-scope="scope">{{ scope.row.remark }}</template>
          </el-table-column>
          <el-table-column prop="assessmentRemarks" label="完成情况描述">
            <template slot-scope="scope">
              {{ scope.row.assessmentRemarks }}
            </template>
          </el-table-column>
          <el-table-column prop="score" label="领导评分（70%)" width="180">
            <template slot-scope="scope">
              {{ scope.row.score }}
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="closecompleteDialog" type="primary">确 定</el-button>
      </div>
    </el-dialog>
    <el-dialog v-if="viewVisible" :visible="viewVisible" title="" :close-on-click-modal="false" width="80%"
      @close='handleClose' class="examiner-dialog" append-to-body>
      <div>
        <el-tabs type="border-card" v-model="activeName">
          <el-tab-pane :name="item.name" v-for="(item, index) in tabList" :key="index">
            <span slot="label"> {{ item.label }}<i :style="{ 'color': item.color }" style="font-style: normal;">({{
            item.number }})</i></span>
            <TaskTable :planId="kpiId" :taskType="item.name" :fetchUrl="fetchUrl" v-if="(activeName == item.name)"
              v-bind="$attrs" from='targetView' />
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-dialog>
    <!-- 完成情况描述放大后输入 -->
    <el-dialog v-if="bigInputVisible" :visible="bigInputVisible" :title="showBigInput.ext.t" :close-on-click-modal="false" width="80%"
      @close="closeBigInput" append-to-body style="height:100%;">
      <div style="position:relative;">
        <el-input :disabled="isDisabled(showBigInput.ext.s)" class="bigInputClass" type="textarea" :autosize="{minRows:10}" style="width: 100%;" v-model="bigInputContent"></el-input>
        <span style="position: absolute;z-index: 1;right: 10px;bottom:0px;color:#999;font-size: 12px;font-weight: 100;">{{ bigInputContent.length }}</span>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="closeBigInput">取 消</el-button>
        <el-button @click="confirmBigInput" type="primary">确 定</el-button>
      </div>
    </el-dialog>
    <!-- <div class="footer-bt" v-if="isReInitiate">
      <div class="footer-inner" >
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
  </div>
</template>

<script>
import Api from '@/api';
import ExaminerDialog from '../TargetBook/WorkTarget/components/ExaminerDialog.vue';
import UploadDialog from '../TargetBook/components/UploadDialog';
import Sortable from 'sortablejs';
import { localstorageGet, localstorageSet } from '@/utils/auth.js';
import { deepClone } from '@/utils';
import relatedPerfDialog from './relatedPerfDialog';
import TaskTable from '@/views/ProjectManage/developProgress/components/TaskTable.vue';
import relateTaskDialog from '../TargetBook/components/relateTaskDialog';
import eleupload from '@/components/EleUpload';
import math from '@/utils/math.js';
import monthHistoryDialog from './monthHistoryDialog.vue';
import doubleShowDetailDialog from './doubleShowDetailDialog.vue';
export default {
  name: 'MonthlyPerf',
  components: { UploadDialog, ExaminerDialog, relatedPerfDialog, TaskTable, relateTaskDialog, eleupload, monthHistoryDialog, doubleShowDetailDialog },
  data() {
    return {
      that: this,
      dutyLevelType: 'ordinary',
      fileAttachIds: [],
      initiatorId: '',
      relatedPerfVisible: false,
      currentRelativId: '',
      dateForm: {
        currentMonth: '',//new Date().getFullYear() + '-' + (new Date().getMonth() + 1),
      },
      forReload: new Date().getTime(),
      company: '',
      editId: '',
      isGoback: false,
      deleteDisable: false,
      currentRowIndex: null,
      editType: 1,
      importDialogVisible: false,
      examinerVisible: false,
      targetList: [],
      totalScoreWeight: '',
      downloadExcelUrl: 'https://file.runshihua.com/files//200001/66a384f04e8a4868a8205fcb23ab7448.xlsx', // 下载模板
      form: {
        fileList: '', //附件列表，用于校验
        attachFile: [], // 附件
        currentMonthWork: '', // 本月工作重点计划
        currentMonthDetail: '', // 本月工作重点完成情况
        nextMonthWork: '', // 次月重点工作计划
        changeSalary: '', // 调薪
        askForLeave: '', // 请假
        advantage: '', // 值得肯定的地方
        disadvantage: '', // 不足与改进
        manageType: 'manager_target', // 指标类型
        name: '',
        departmentName: '',
        examiner: {
          id: '', // 审核人id
          name: ''
        },
        targetTime: '',
        assessmentCycle: '月度', // 考核周期
        extraPointsCondition: '', // 加分说明
        extraPointsKey: '', // 加分事由
        extraPointsValue: '', // 分值
        recoupPonitsKey: '', // 补分事由
        recoupPonitsValue: '', // 补分值
        deductPointsCondition: '', // 扣分说明
        deductPointsKey: '', // 扣分事由
        deductPointsValue: '', // 扣分值
        selfEvaluation: '', // 自我评价
        leaderEvaluation: '', // 领导评价
        punishPonitsValue: '',
        punishPonitsKey: '',
        rewardPonitsKey: '',
        rewardPonitsValue: '',
        subsidy: '20',
        workDays: '21',
        keyPerformanceIndicatorsList: [
          // {
          //   indicatorsType: {
          //     id: ''
          //   },
          //   targetItemOne: '', // 关键职责
          //   targetItemTwo: '', // KPI
          //   content: '', // 不传
          //   criteriaItems: [{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''}],
          //   weight: '',
          //   assessmentMethod: '',
          //   assessmentTime: '',
          //   assessmentRemarks: '',
          //   score: '', // 领导评分
          //   maxScore: 3, // 0、2、3分
          //   pretendScore: '', // 自评分
          //   // score: '',
          //   plans: [],
          //   yearKpi: null
          // }
        ]
      },
      jobPerf: '0%',
      dateRules: {
        currentMonth: [{
          required: true,
          trigger: 'change',
          message: '请选择月份'
        }]
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
        assessmentCycle: [{
          required: true,
          trigger: 'change',
          message: '请选择考核周期'
        }],
        indicatorsTypeId: [{
          required: true,
          trigger: 'change',
          message: '请选择指标项' // message: '请选择目标项目(一级)'
        }],
        content: [{
          type: 'string',
          required: true,
          trigger: 'blur',
          message: '请输入具体目标项目内容'
        }],
        targetItemOne: [{
          required: true,
          trigger: 'blur',
          message: '请输入内容'
        }],
        targetItemTwo: [{
          required: true,
          trigger: 'blur',
          message: '请输入内容'
        }],
        pretendScore: [{
          required: true,
          trigger: 'blur',
          message: '请输入内容'
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
        maxScore: [{
          required: true,
          trigger: 'blur',
          message: '请输入分值'
        }],
        selfEvaluation: [{
          required: true,
          trigger: 'blur',
          message: '请输入自我评价'
        }],
        // askForLeave: [{
        //   required: true,
        //   trigger: 'blur',
        //   message: '考勤及补助'
        // }],
        fileList: [{
          required: true,
          message: '请上传月度工作总结'
        }],
        advantage: [{
          required: true,
          trigger: 'change',
          message: ' '
        }],
        nextMonthWork: [{
          required: true,
          trigger: 'change',
          message: ' '
        }],
        disadvantage: [{
          required: true,
          trigger: 'change',
          message: ' '
        }]
      },
      isInFlow: true,
      permisionList: [],
      completeVisible: false,
      detailContentList: [],
      tableHeight: 300,
      viewVisible: false,
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
      activeName: 'waiting_send',
      fetchUrl: '',
      kpiId: '',
      taskVisible: false,
      relateRow: {},
      selectRelativeTaskList: [],
      scaleVal: localstorageGet('month_scaleVal') || 10,
      kpiWidth: '115', // 动态计算kpi的宽度
      monthHistoryVisible: false,
      doubleShowDetailVisible: false,
      doubleShowDetailValue: {},
      bigInputContent: '', //放大后的输入框
      bigInputVisible:false
    };
  },
  directives: {
    fillHeight: { // v-fillHeight自适应父容器高度指令
      inserted: function (el) {
        var parentHeight = el.parentNode.parentNode.clientHeight;
        el.style.maxHeight = `${parentHeight - 5}px`;
      },
      componentUpdated: function (el) {
        var parentHeight = el.parentNode.parentNode.clientHeight;
        el.style.maxHeight = `${parentHeight - 5}px`;
      }
    },
    fillHeightTextarea: {
      inserted: function(el, binding) {
        console.log(el, 'insertddd ')
        var elHeight = el.parentElement.parentElement.clientHeight;
        var aHeight = el.parentElement.querySelector('article')?.clientHeight || 5;
        const textInput = el.querySelector('textarea');
        if (textInput) {
          setTimeout(() => {
            binding.value.$nextTick(() => {
              console.log(`${elHeight - aHeight}px`);
              textInput.style.height = `${elHeight - aHeight}px`;
            });
          }, 100);
        }
      }
    }
  },
  computed: {
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
    totalWeight() {
      let totalWeight = 0;
      if (this.form.keyPerformanceIndicatorsList.length) {
        const val = this.form.keyPerformanceIndicatorsList;
        val.forEach((item, index) => {
          totalWeight = math.add(totalWeight, item.weight) //((item.weight * 100) - 0);
        });
      }
      return totalWeight//.toFixed(2);
    }
  },
  watch: {
    bizId(val){
      if(val){
        this.editType = 2;
        this.editId = val
        this.createInit()
      }
    },
    'form.fileList': {
      handler(val) {
        if (val.length >= 1) {
          this.$refs.form.clearValidate('fileList');
        }
      },
    },
    forReload: {
      handler() {
        this.$nextTick(() => {
          (this.actionType == 'create' || this.isReInitiate) && this.rowDrop();
        });
      },
      immediate: true
    },
  },
  props: {
    bizId: {
      type: String,
      default: ''
    },
    actionType: {
      type: String,
      default: 'create'
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    flowProxyId: { // 流程id
      type: String,
      default: ''
    },
    isReInitiate: {
      type: Boolean,
      default: false
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
    isHistoryComponent:{
      type: Boolean,
      default: false
    },
    isTranspondFlow:{
      type: Boolean,
      default: false
    },
  },
  inject: {
    prevStepHandle: { value: 'prevStepHandle', default: null },
    sumbitFlow: { value: 'sumbitFlow', default: null },
    submitFlowFinal: { value: 'submitFlowFinal', default: null }
  },
  mounted() {
    if (this.actionType == 'create') {
      this.$refs?.datepicker?.focus();
    };
    this.company = (this.actionType == 'create') ? localstorageGet('companyName') : '';
    this.zoom(0);
    // this.$nextTick(() => {
    //   let mainForm = document.querySelector('.main-form')
    //   if (mainForm) {
    //     mainForm.style.zoom = this.scaleVal / 10
    //   }
    //   this.calculateKpiWidth()
    // })
    window.onresize = () => {
      this.calculateKpiWidth()
      // this.calculateFoot()
      setTimeout(x=>{
        this.calculateFoot()
      },500)
    }
  },
  beforeDestroy() {
    //销毁
    if(!this.isHistoryComponent){
      window.onresize = null
    }
  },
  created() {
    if (this.$route?.query?.editType) {
      this.editType = this.$route.query.editType;
      this.isInFlow = false;
    }
    if (this.actionType !== 'create') {
      this.editType = 2;
    }
    if (this.actionType == 'create' || this.isReInitiate) {
      this.getIndicatorsTypeList();
      this.getPersonInfo();
    }
    if (this.editType == 2) {
      this.editId = (!this.$route.query.fromMsg && this.$route.query.id) || this.bizId;
      this.createInit()
    } else {
      this.$nextTick(() => {
        this.calcateInputHeight()
      })
    }
    console.log(this.isInFlow,'this.isInFlow++',this.actionType)
    if (this.isInFlow) {
      if (this.actionType == 'examine') {
        this.getInputPermision().then(res => {
          this.permisionList = res;
        });
      } else if (this.actionType == 'create' || this.actionType == 'edit') {
        this.getCreateData();
      } else if (this.actionType == 'preview') {
        this.permisionList = [];
      }
    } else {
      this.permisionList = ['name', 'company', 'departmentName',
        'examinerName', 'assessmentCycle', 'target', 'targetItemOne',
        'targetItemTwo', 'content', 'weight', 'nextMonthWork', 'pretendScore',
        'assessmentMethod', 'assessmentTime', 'disadvantage', 'advantage',
        'assessmentRemarks', 'maxScore', 'extraPointsCondition', 'askForLeave', 'changeSalary',
        'extraPointsKey', 'extraPointsValue', 'recoupPonitsKey', 'recoupPonitsValue', 'deductPointsCondition',
        'deductPointsKey', 'deductPointsValue', 'selfEvaluation', 'leaderEvaluation'
      ];
    }
    if (this.$attrs.isHistoryDialog) return;
    this.$bus.$off('monthly_perf_before_handle');
    this.$bus.$on('monthly_perf_before_handle', (val,that) => {
      this.examneObj = that //把审核组件实例传过来,方便后面把loading取消
      if (this.status == 'not_submitted') {
        this.postData(that?.isTemporarySave ? 'nocheck' : undefined);
      } else {
        this.status = val;
        this.submitData('form', that?.isTemporarySave ? 'nocheck' : undefined).then(res => {
          const obj = {
            status: 'success',
            name: `${this.dateForm.currentMonth}月度绩效考核-${this.workTargetData?.userName || localstorageGet('userName')}`
          };
          if (this.isReInitiate) this.bindBatchFileByIds(this.editId); // 重新发起月度绩效考核id绑定附件文件ids
          this.$bus.$emit('submitBeforeHandleOk', obj);
        }).catch(err => {
          const obj = {
            status: 'fail'
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        });
      }
    });
  },
  filters: {
    // findName: function (val) {
    //   console.log(this, 'this');
    //   return 233;
    //   // var fi;
    //   // if (this.targetList?.length) {
    //   //   fi = this.targetList.find(item => item.id == val);
    //   // }
    //   // if (fi) {
    //   //   return fi.name;
    //   // }
    // }
  },
  destroyed() {
    // 解决Vue $on能拿到数据但是无法更新data数据
    if (this.isGoback) {
      this.$bus.$emit('targetType', 2);
    }
  },
  methods: {
    onKeydown(event, index) {
      if (event.key == 'ArrowUp' || event.key == 'ArrowDown') {
        this.keypress(event, index);
        event.preventDefault();
      }
    },
    closeBigInput(){
      this.bigInputVisible = false
    },
    showBigInput(row, ext) {
      this.showBigInput.ext = ext;
      this.bigInputVisible = true
      this.bigInputContent = row[ext.k]
      this.currentBigRow = row
    },
    confirmBigInput() {
      this.currentBigRow[this.showBigInput.ext.k] = this.bigInputContent
      this.closeBigInput()
    },
    doubleClickShow(val, title) {
      this.doubleShowDetailValue = { val, title };
      this.doubleShowDetailVisible = true;
    },
    createInit(){
      console.log('createInit')
      this.getAttachmentList(this.editId);
      this.getWorkTargetDetail(this.editId).then(res => {
        const data = this.workTargetData = res.data;
        if (!this.initiatorId) this.initiatorId = data?.user?.id || undefined;
        const keyPerformanceIndicatorsList = data?.keyPerformanceIndicatorsList || [];
        keyPerformanceIndicatorsList.forEach(el => {
          // if (el.weight <= 1) el.weight = el.weight * 100
          if (el.kpiSplitItems && el.kpiSplitItems.length) el.sys = true;
          if (el.yearKpi) {
            el.kpiSplitItems = el.yearKpi?.kpiSplitItems || [];
          }
          if (el.criteriaItems) {
            var l = el.criteriaItems.length;
            for (let i = l;i < 5;i++) {
              el.criteriaItems.push({ criteria: '', score: '' });
            }
          }
          console.log(el.criteriaItems, 'el.criteriaItems');
        });
        if (data?.keyPerformanceIndicatorsList) {
          this.insertData(data);
        }
        if(data.relationFileDatas && data.relationFileDatas.length) { this.onlyViewFile(data.relationFileDatas) };
        //如果是审核，需要把滚动条调整到右边
        if (!this.isReInitiate) {
          this.$nextTick(() => {
            // 加载完成后把滚动条放到右边
            if (document.querySelector('.main-table .el-table__body-wrapper')) document.querySelector('.main-table .el-table__body-wrapper').scrollLeft = 2000;
            // // 加载完成后把滚动条下移显示横向滚动条
            // if (document.querySelector('.el-dialog__body .left-side'))document.querySelector('.el-dialog__body .left-side').scrollTop = 120;
          });
        }
      });
    },
    onlyViewFile(list) {
      const attachFile = list.map(item => {
        return {
          id: item.fileId,
          fileName: item.fileName,
          fileUrl: item.fileUrl,
          absolutelyFileUrl: item.fileUrl
        };
      });
      this.form.attachFile = attachFile;
      this.form.fileList = attachFile;
    },
    checkHistory(){
      this.monthHistoryVisible = true
    },
    handleInputChange(index) {
      if (!this.form['keyPerformanceIndicatorsList.' + index + '.weight']) {
        this.form['keyPerformanceIndicatorsList.' + index + '.weight'] = 0
      }
    },
    //计算kpi栏目的宽度，以便对齐
    calculateKpiWidth() {
      //计算kpi宽度
      let toPformDom = document.querySelector('.top-form')
      let width = toPformDom.clientWidth / 2
      if (this.isDisabled('annualTarget')) {
        this.kpiWidth = width - 600 // this.kpiWidth = width - 359
      }
      // else{
      //   this.kpiWidth = width - 344
      // }

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
      if (event.keyCode == 13 || event.keyCode == 40 || event.keyCode == 38) {
        //回车,光标下移
        let targetClass = 'el-table__cell'
        let pnode = this.getParents(event.target, targetClass)
        let classTag = pnode.className
        let tdClass = classTag.replace(targetClass, '').trim()
        let nodeList = document.querySelectorAll(`td.${tdClass} input`)
        nodeList = Array.from(nodeList);
        nodeList.push(document.querySelector('.superiorEvaluationGradeClass textarea'));
        let len = nodeList?.length
        if (len - 1 >= index) {
          var v = event.keyCode == 38 ? -1 : 1;
          nodeList[index + v]?.focus()
          // if(!nodeList[index + v] && event.keyCode == 40){}
        }
      }
      /[+\-eE]/.test(event.key) && event.preventDefault();
    },
    zoom(val) {
      let value = Number(this.scaleVal) + Number(val)
      value = value >= 12 ? 12 : value <= 10 ? 10 : value
      let mainForm = document.querySelector('.main-form')
      if (mainForm) {
        this.scaleVal = value
        mainForm.style.zoom = value / 10
        localstorageSet('month_scaleVal', value)
        this.$nextTick(() => {
          this.calculateKpiWidth()
        })
      }
    },
    upChange(fileList) {
      this.form.fileList = fileList
    },
    handleBeforePlan() {
      if (this.dateForm.currentMonth && this.actionType == 'create') {
        // const bt = new Date(this.dateForm.currentMonth);
        // const year = bt.getFullYear();
        // const mon = bt.getMonth();
        // if (mon) {
        //   beforetime = year + '' + mon;
        // } else {
        //   beforetime = (year - 1) + '12';
        // }
        // const bt = this.dateForm.currentMonth.split('-');
        // const year = bt[0];
        // const mon = bt[1];
        console.log(this.dateForm.currentMonth, 'this.dateForm.currentMonth--打印月度绩效当前月份去查下月份');
        const [year, mon] = this.dateForm.currentMonth.split('-').map(Number);
        var beforetime;
        if (mon === 1) {
          beforetime = (year - 1) + '12';
        } else {
          beforetime = year + '' + (mon - 1);
        }
        this.fetchData(beforetime).then(res => {
          if (res.isSuccess && res.data && res.data[0].kpiDynamicItemList) {
            var data = res.data[0];
            if (data.kpiDynamicItemList) {
              var next = data.kpiDynamicItemList.find(i => i.kpiDynamicType == 'important_work_next_month');
              next && (this.form.currentMonthWork = next.desc);
            }
          } else {
            this.form.currentMonthWork = '';
          }
        });
      }
    },
    findName: function (val) {
      var fi;
      if (this.targetList?.length) {
        fi = this.targetList.find(item => item.id == val);
      }
      if (fi) {
        return fi.name;
      }
    },
    getSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        // console.log('column',column)
        if (column.label == '权重得分') {
          let weightScore = 0;
          data.forEach((it, idx) => {
            weightScore = math.add(this.calculateWeightScore(idx), weightScore);
          });
          //
          sums[index] = weightScore;
          return;
        }
        // if(column.label.indexOf('自评分')>-1){
        //   let selfScore = 0
        //   data.forEach(it=>{
        //     if(isNaN(it.pretendScore)){
        //       selfScore = ''
        //       return
        //     }
        //     selfScore = math.add(selfScore,it.pretendScore)
        //   })
        //   sums[index] = selfScore
        //   return
        // }
        // if(column.label.indexOf('领导评分')>-1){
        //   let score = 0
        //   data.forEach(it=>{
        //     score = math.add(score,it.score)
        //   })
        //   sums[index] = score
        //   return
        // }
        if (index === 0) {
          sums[index] = '';
          return;
        }
        if (index === 4) {
          let sub = data.reduce((prev, cur) => {
            return math.add(prev, cur.weight);
          }, 0);
          sub = Number(sub);
          sums[index] = <div class={'text-center'}>{sub}%</div>;
        }
      });
      return sums;
    },
    selectRelativeTask(data) {
      this.relateRow.plans = data;
    },
    relativeTask(row) {
      this.taskVisible = true;
      this.relateRow = row;
    },
    calculateHeight() {
      let bodyHeight;
      let dom = document.querySelector('.el-dialog__body');
      if (!dom) dom = document.querySelector('.el-card__body');
      if (dom) {
        bodyHeight = dom.clientHeight;
        this.tableHeight = bodyHeight - 220;
      }
    },
    handleClose() {
      this.viewVisible = false;
    },
    viewDetail(row) {
      this.tabList[0].number = row.notSubmit;
      this.tabList[1].number = row.pendingReview;
      this.tabList[2].number = row.finished;
      this.kpiId = row.id;
      this.fetchUrl = Api.taskManage.taskArrange.getTargetCompleteInfo;
      this.activeName = 'waiting_send';
      this.viewVisible = true;
    },
    getCreateData() {
      this.handleBeforePlan();
      this.getPermisionForCreate().then(res => {
        this.permisionList = res;
        this.permisionList.push('fileList')
        if (this.actionType == 'create') {
          // 按照月度查询月度任务
          this.getKpiListByTargetTime();
        }
      });
    },
    closecompleteDialog() {
      this.completeVisible = false;
    },
    kpiDetailContent(row) {
      this.completeVisible = true;
      this.$nextTick(() => {
        let data = {};
        if (this.actionType == 'create') {
          data = {
            assessmentCycle: 'year',
            id: row.id
          };
          if (row.kpiSplitItems && row.kpiSplitItems.length) {
            data.kpiSplitItems = [{ id: row.kpiSplitItems[0].id }];
            // data.kpiSplitItems[0].id = row.kpiSplitItems[0].id
            if (row.kpiSplitItems[0].kpiSplitItemWeights && row.kpiSplitItems[0].kpiSplitItemWeights.length) {
              data.kpiSplitItems[0].kpiSplitItemWeights = [{ id: row.kpiSplitItems[0].kpiSplitItemWeights[0].id }];
            }
          }
        } else {
          data = {
            assessmentCycle: 'month',
            id: row.id
          };
        }
        // return
        this.$axios.post(Api.performance.getFinishedMonthKpiByKpiId, { data }, res => {
          if (res.isSuccess) {
            const data = res?.data || [];
            this.detailContentList = data;
          }
        });
      });
    },
    getKpiListByTargetTime() {
      if (!this.dateForm.currentMonth) return;
      const currentTime = this.dateForm.currentMonth.split('-');
      const targetTime = currentTime[0] + ('0' + currentTime[1]).substr(-2, 2);
      const data = {
        targetTime
      };
      this.$axios.post(Api.performance.getKpiListByTargetTime, { data }, res => {
        const data = res?.data || [];
        this.form.keyPerformanceIndicatorsList = this.form.keyPerformanceIndicatorsList.filter(item => {
          return !item.currentMonth || (item.currentMonth && item.currentMonth == this.dateForm.currentMonth);
        });
        if (data.length) {
          data.forEach(item => {
            item.sys = true;
            item.weight = '';
            item.score = '';
            item.currentMonth = this.dateForm.currentMonth;
            item.assessmentRemarks = '';
            const plans = item.plans || [];
            plans.forEach(el => {
              el.sys = true;
            });
            if (item.criteriaItems) {
              var l = item.criteriaItems.length;
              for (let i = l;i < 5;i++) {
                item.criteriaItems.push({ criteria: '', score: '' });
              }
            }
            this.form.keyPerformanceIndicatorsList.push(item)
          });
          this.originData = deepClone(this.form.keyPerformanceIndicatorsList);
        }
        // else {
        //   this.handleInsert();
        // }

        this.$nextTick(() => {
          this.calculateHeight();
        });
      });
    },
    downLoadTemplate(code) {
      const data = {
        code
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
    openRelative(row, scope) {
      this.comfirmRelate.currentIndex = scope.$index;
      // this.currentIndex = index;
      // const id = this.form.keyPerformanceIndicatorsList[this.currentIndex]?.yearKpi?.id;
      // scope.row
      // console.log('id',id)
      // this.currentRelativId = id;
      // this.taskVisible = true
      this.relateRow = row;
      this.relatedPerfVisible = true;
    },
    removeRelative(index) {
      this.$confirm('确认要取消关联?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$set(this.form.keyPerformanceIndicatorsList[index], 'yearKpi', {});
      }).catch(() => { });
    },
    comfirmRelate(obj) {
      // this.$set(this.form.keyPerformanceIndicatorsList, this.comfirmRelate.currentIndex, obj); //关联目标责任书不修改
      this.$set(this.relateRow, 'yearKpi', obj);
      // this.$set(this.form.keyPerformanceIndicatorsList[this.currentIndex].yearKpi, 'id', obj.id);
      // this.form.keyPerformanceIndicatorsList[this.currentIndex].yearKpi = {
      //   id:obj.id
      // }
      this.relatedPerfVisible = false;
    },
    getPermisionForCreate() {
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      return new Promise((resolve, reject) => {
        this.$axios.post(
          // Api.schedule.flowTemplateFindById,
          // Api.schedule.getFlowInstanceTemplateNode,
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
            resolve(enableList);
          }
        );
      });
    },
    // calculateScore(){
    // let totalScoreWeight = 0,totalWeight=0
    //   if(this.form.keyPerformanceIndicatorsList.length){
    //     let val = this.form.keyPerformanceIndicatorsList
    //     val.forEach((item,index)=>{
    //       let weightScore = this.calculateWeightScore(index)
    //       totalScoreWeight += (weightScore - 0)
    //       totalWeight += ((item.weight*100)-0)
    //     });
    //   }
    //   totalScoreWeight = totalScoreWeight + ((this.form.extraPointsValue || 0) - 0) - this.form.deductPointsValue
    //   this.totalScoreWeight = totalScoreWeight.toFixed(2)
    //   this.totalWeight = totalWeight.toFixed(2)
    // },
    checkPosive(val, key) {
      const score = val < 0 ? 0 : val;
      this.form[key] = score;
      this.debounceGetVal();
    },
    debounceGetVal() {
      if (this.setTime) clearTimeout(this.setTime);
      this.setTime = setTimeout(() => {
        this.getPerformanceVal();
      }, 400);
    },
    getPerformanceVal() {
      const keyPerformanceIndicatorsList = deepClone(this.form.keyPerformanceIndicatorsList)
      keyPerformanceIndicatorsList.forEach(item => {
        if (item.weight == '') item.weight = 0
        item.weight = math.divide(item.weight, 100); // item.weight /= 100
      })
      const data = {
        assessmentCycle: 'month',
        keyPerformanceIndicatorsList: keyPerformanceIndicatorsList,//this.form.keyPerformanceIndicatorsList,
        recoupPonitsValue: this.form.recoupPonitsValue || 0,
        extraPointsValue: this.form.extraPointsValue || 0,
        deductPointsValue: this.form.deductPointsValue || 0,
        deductPointsKey: this.form.deductPointsKey,
        rewardPonitsValue: this.form.rewardPonitsValue || 0,
        punishPonitsValue: this.form.punishPonitsValue || 0
        // KeyPerformanceIndicatorsList.yearKpi.id
        // KeyPerformanceIndicatorsList:{
        //   yearKpi:{
        //     id:this.editId
        //   }
        // }
      };
      this.$axios.post(
        Api.performance.getPerfVal,
        { data }
      ).then(res => {
        if (res.isSuccess) {
          const kpi = res?.data?.totalKpi;
          if (kpi) {
            // this.jobPerf = parseInt(res.data.totalKpi * 100) + '%'; // .toFixed(2)
            // this.totalScoreWeight = res.data.totalScore;
            this.jobPerf = math.multiply(res?.data?.totalKpi || 0, 100) + '%';
          } else if (kpi === 0) {
            this.jobPerf = '0%';
          } else {
            this.jobPerf = '0%';
          }
          const score = res.data.totalScore;
          if (score) {
            this.totalScoreWeight = score.toFixed(2);
          } else {
            this.totalScoreWeight = '';
          }
        }
      });
    },
    maxVal(val, index, max, key) {
      if (String(val).includes('.')) {
        this.form.keyPerformanceIndicatorsList[index][key] = parseInt(Number(val)) || 0;
      }
      if (val >= max) {
        this.form.keyPerformanceIndicatorsList[index][key] = max;
      }
      if (val < 0) {
        this.form.keyPerformanceIndicatorsList[index][key] = 0;
      }
      if (!String(val).endsWith('.') && isNaN(val)) {
        this.form.keyPerformanceIndicatorsList[index][key] = Number(val) || 0;
      }
      this.debounceGetVal();
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
              // flowNodeFieldPowerTemplateList
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName;
              });
            }
            if(this.flowNodeProxyId == 'f0f8f81f94be4aed9419d66d5c9f1ff5' || this.flowNodeProxyId == '6b46c7d2dbbd48d482d0ecfe445cba51'){
              if(enableList.indexOf('score')==-1)enableList.push('score')
            }
            resolve(enableList);
          }
        );
      });
    },
    handleImportSuccess(data) {
      var kpiList = data.kpiList;
      kpiList.forEach(item => {
        if (!item.indicatorsType) {
          item.indicatorsType = {};
          item.indicatorsType.id = '';
        }
        item.content = item.assessmentMethod;
        item.weight = math.multiply(item.weight, 100);
        if (item.criteriaItems) {
          var l = item.criteriaItems.length;
          for (let i = l;i < 5;i++) {
            item.criteriaItems.push({ criteria: '', score: '' });
          }
        }
      });
      this.form.targetTime = data.targetTime;
      this.form.extraPointsCondition = data.extraPointsCondition;
      this.form.extraPointsKey = (data.extraPointsKey && data.extraPointsKey > 0) ? data.extraPointsKey : '';
      this.form.extraPointsValue = (data.extraPointsValue && data.extraPointsValue > 0) ? data.extraPointsValue : 0; // 分值
      this.form.recoupPonitsKey = (data.recoupPonitsKey && data.recoupPonitsKey > 0) ? data.recoupPonitsKey : '';
      this.form.recoupPonitsValue = (data.recoupPonitsValue && data.recoupPonitsValue > 0) ? data.recoupPonitsValue : 0; // 分值
      this.form.deductPointsCondition = data.deductPointsCondition; // 扣分说明
      this.form.deductPointsKey = (data.deductPointsValue && data.deductPointsValue > 0) ? data.deductPointsKey : ''; // 扣分事由
      if (data.kpiDynamicItemList) {
        this.setkpiDynamicItem2(data.kpiDynamicItemList);
      };
      this.form.deductPointsValue = (data.deductPointsValue && data.deductPointsValue > 0) ? data.deductPointsValue : 0; // 扣分值
      this.form.selfEvaluation = data.selfEvaluation; // 自我评价
      this.form.leaderEvaluation = data.leaderEvaluation;
      this.form.punishPonitsKey = data.punishPonitsKey;
      this.form.rewardPonitsValue = data.rewardPonitsValue || 0;
      this.form.rewardPonitsKey = data.rewardPonitsKey;
      this.form.punishPonitsValue = data.punishPonitsValue || 0;
      this.form.salarvStructure = data.punishPonitsValue || 0;
      this.form.subsidy = data.subsidy;
      this.form.workDays = data.workDays;
      this.$nextTick(() => {
        this.$refs.form.clearValidate()
        this.form.keyPerformanceIndicatorsList.push(...(kpiList || []));
        this.$nextTick(() => {
          this.calcateInputHeight()
        })
      })

    },
    getAttachmentList(id) { // 1、根据业务id获取附件文件回显
      if (this.actionType != 'create' && this.actionType != 'edit') return;
      this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      }).then(res => {
        if (res.isSuccess) {
          const list = res.data;
          const attachFile = list.map(item => {
            return {
              id: item.fileId,
              fileName: item.fileName,
              fileUrl: item.fileUrl,
              absolutelyFileUrl: item.fileUrl
            };
          });
          this.form.attachFile = attachFile;
          this.form.fileList = attachFile
        }
      });
    },
    getWorkTargetDetail(id) {
      return this.$axios.post( // 2、查询月度绩效考核数据回显
        Api.performance.getWorkTargetDetail2,
        {
          data: {
            id// : this.editId,
          }
        }
      );
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
    getCompanyName(id) {
      const data = {
        id
      };
      return this.$axios.post(Api.frameworkInfo.getParentCompanyList, { data });
    },
    setkpiDynamicItem2(data) {
      var cur = data.find(i => i.kpiDynamicType == 'important_work_month');
      var next = data.find(i => i.kpiDynamicType == 'important_work_next_month');
      var salary = data.find(i => i.kpiDynamicType == 'change_salary');
      var leave = data.find(i => i.kpiDynamicType == 'ask_for_leave');
      var advantage = data.find(i => i.kpiDynamicType == 'advantage');
      var disadvantage = data.find(i => i.kpiDynamicType == 'disadvantage');
      // cur && (this.form.currentMonthWork = cur.desc);
      cur && (this.form.currentMonthDetail = cur.recordValue);
      next && (this.form.nextMonthWork = next.desc);
      salary && (this.form.changeSalary = salary.desc);
      leave && (this.form.askForLeave = leave.desc);
      advantage && (this.form.advantage = advantage.desc);
      disadvantage && (this.form.disadvantage = disadvantage.desc);
    },
    setkpiDynamicItem(data) {
      var cur = data.find(i => i.kpiDynamicType == 'important_work_month');
      var next = data.find(i => i.kpiDynamicType == 'important_work_next_month');
      var salary = data.find(i => i.kpiDynamicType == 'change_salary');
      var leave = data.find(i => i.kpiDynamicType == 'ask_for_leave');
      var advantage = data.find(i => i.kpiDynamicType == 'advantage');
      var disadvantage = data.find(i => i.kpiDynamicType == 'disadvantage');
      cur && (this.form.currentMonthWork = cur.desc);
      cur && (this.form.currentMonthDetail = cur.recordValue);
      next && (this.form.nextMonthWork = next.desc);
      salary && (this.form.changeSalary = salary.desc);
      leave && (this.form.askForLeave = leave.desc);
      advantage && (this.form.advantage = advantage.desc);
      disadvantage && (this.form.disadvantage = disadvantage.desc);
    },
    insertData(data) {
      this.company = data.companyName;
      this.form.userId = data.user.id;
      this.dutyLevelType = data.user?.duty?.dutyLevel?.dutyLevelType || 'ordinary';
      this.form.name = data.userName;
      this.form.departmentName = data.depName;
      this.form.assessmentCycle = '月度';// data.assessmentCycle
      data.keyPerformanceIndicatorsList.sort((a, b) => {
        return a.sort - b.sort;
      });
      data.keyPerformanceIndicatorsList.forEach(el => {
        if (!el.indicatorsType) el.indicatorsType = { id: '' };
        if (this.isReInitiate) { el.score = ''; }
        el.weight = math.multiply(el.weight,100)
      });
      // this.form.keyPerformanceIndicatorsList = []//data.keyPerformanceIndicatorsList;
      this.form.keyPerformanceIndicatorsList = data.keyPerformanceIndicatorsList;
      this.form.targetTime = data.targetTime;
      this.form.extraPointsCondition = data.extraPointsCondition;
      this.form.extraPointsKey = data.extraPointsKey;
      this.form.extraPointsValue = data.extraPointsValue || 0; // 分值
      this.form.recoupPonitsKey = data.recoupPonitsKey;
      this.form.recoupPonitsValue = data.recoupPonitsValue || 0; // 分值
      this.form.deductPointsCondition = data.deductPointsCondition; // 扣分说明
      this.form.deductPointsKey = data.deductPointsKey; // 扣分事由
      if (data.kpiDynamicItemList) {
        this.setkpiDynamicItem(data.kpiDynamicItemList);
      };
      this.form.deductPointsValue = data.deductPointsValue || 0; // 扣分值
      this.form.selfEvaluation = data.selfEvaluation; // 自我评价
      this.form.leaderEvaluation = data.leaderEvaluation;
      this.form.punishPonitsKey = data.punishPonitsKey;
      this.form.rewardPonitsValue = data.rewardPonitsValue || 0;
      this.form.rewardPonitsKey = data.rewardPonitsKey;
      this.form.punishPonitsValue = data.punishPonitsValue || 0;
      this.form.salarvStructure = data.punishPonitsValue || 0;
      this.form.subsidy = data.subsidy;
      this.form.workDays = data.workDays;
      if (data.targetTime) {
        const targetTime = data.targetTime + '';
        const year = targetTime.substr(0, 4);
        const month = targetTime.substr(4, 2);
        // month  = month < 10 ? '0'+ month : month
        this.dateForm.currentMonth = year + '-' + month;
      }
      this.getPerformanceVal();
      // this.getCompanyListOfOnDuty(data.createrId).then(res => {
      //   if (res.isSuccess) {
      //     const data = res.data;
      //     this.form.name = data.name;
      //     const companyId = data.companyId;
      //     const departmentId = data.userDutyVos[0]?.departmentId || '';
      //     if (companyId && departmentId) {
      //       this.getCompanyName(companyId).then(res => {
      //         if (res.isSuccess) {
      //           const find = res.data.find(item => item.id == companyId);
      //           if (find) {
      //             this.company = find.name;
      //           }
      //         }
      //       });
      //       this.getDepartmentList(companyId).then(res => {
      //         if (res.isSuccess) {
      //           const find = res.data.find(item => item.id == departmentId);
      //           if (find) {
      //             this.form.departmentName = find.departmentName;
      //           }
      //         }
      //       });
      //     }
      //   }
      // });
      this.$nextTick(() => {
        //计算table高度
        this.calculateHeight();
        //计算输入的高度
        this.calcateInputHeight()
        //
        // this.calculateFoot()
        setTimeout(x=>{
          this.calculateFoot()
        },500)

      });
    },
    calculateFoot() {
      console.log('calculateFoot')
      if (this.$route.query && this.$route.query.type == "month") {
      } else {
        //改变底部栏目，悬浮展示
        let footDom = document.querySelector('#perform-footer')
        if (footDom && !this.$attrs.isHistoryDialog) {
          let topDom = document.querySelector('.top-form')
          let position = footDom.getBoundingClientRect()
          // footDom.style.position = 'fixed'
          if (!this.isTranspondFlow) { // 如果是转发流程就不要这个fixed布局了
            footDom.style.position = 'fixed'
          }
          // footDom.style.left = position.left + 'px'
          // footDom.style.width = topDom.getBoundingClientRect().width + 'px'
          footDom.style.width = topDom.offsetWidth + 'px'
          footDom.style.bottom = '20px'
        }
      }
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
          this.form.userId = data.id;
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
          checkCanUpdate: false
          // manageType: ""
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
      this.$router.go(-1);
      // this.$router.push({
      //   path: '/performanceManage/targetBook',
      // });
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
    tableRowClassName({ row, rowIndex }) {
      row.row_index = rowIndex;
      return 'kpi-table-row-' + rowIndex;
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
    // 拖拽行
    rowDrop() {
      // 要侦听拖拽响应的DOM对象
      const tbody = document.querySelector('.main-table .el-table__body tbody');
      const that = this;
      Sortable.create(tbody, {
        handle: '.drag-handle',
        onEnd({ newIndex, oldIndex }) {
          const currentRow = that.form.keyPerformanceIndicatorsList.splice(oldIndex, 1)[0];
          that.form.keyPerformanceIndicatorsList.splice(newIndex, 0, currentRow);
          that.forReload = new Date().getTime();
          that.$nextTick(() => {
            that.clearValidate();
            that.calcateInputHeight()
          })
        }
      });
    },
    // 插入行
    handleInsert(index) {
      var curIndex = (typeof index == 'number') ? index : this.currentRowIndex;
      this.$refs['dateForm'].validate(res => {
        if (res) {
          const itemData = deepClone(
            {
              indicatorsType: {
                id: ''
              },
              targetItemOne: '',
              targetItemTwo: '', // KPI
              content: '', // 不传
              criteriaItems: [{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''},{ criteria: '', score: ''}],
              weight: '',
              assessmentMethod: '',
              assessmentTime: '',
              assessmentRemarks: '',
              score: '', // 领导评分
              maxScore: 3, // 0、2、3分
              pretendScore: '', // 自评分
              // score: '',
              plans: [],
              yearKpi: null
            });
          if (!curIndex && curIndex != 0) {
            this.form.keyPerformanceIndicatorsList.push(itemData);
          } else {
            this.form.keyPerformanceIndicatorsList.splice(curIndex + 1, 0, itemData);
          }
          this.$nextTick(() => {
            this.clearValidate();
            //计算输入的高度
            this.calcateInputHeight()
          })
        }
      })
      // if (!this.dateForm.currentMonth) return this.$message.error('请先选择考核月份')
    },
    calcateInputHeight() {
      let allInputObj = document.querySelectorAll('.main-table td.el-table__cell')
      allInputObj.forEach(item => {
        let height = item.clientHeight
        let input = item.querySelector('input')
        if (input && input.name != 'notSetHeight') {
          input.style.height = `${height - 7}px`
        }
        let textInput = item.querySelector('textarea')
        if (textInput && textInput.name != 'notSetHeight') {
          textInput.style.height = `${height - 7}px`
        }
        return
      })
    },
    // 删除行
    handleDel(index) {
      var curIndex = (typeof index == 'number') ? index : this.currentRowIndex;
      if (curIndex === null) {
        this.$message.error('请选中一条数据');
        return;
      }
      this.$confirm('确认删除行！', '提示', {
        closeOnClickModal: false,
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
          if (curIndex === null) {
            this.$message.error('请选中一条数据');
            return;
          }
          this.form.keyPerformanceIndicatorsList.splice(curIndex, 1);
          if (this.form.keyPerformanceIndicatorsList.length <= 1) {
            curIndex = null;
          } else {
            if (curIndex != 0) {
              curIndex = curIndex - 1;
            }
            this.$refs.table.setCurrentRow(this.form.keyPerformanceIndicatorsList[curIndex]);
          }
        }
        this.$nextTick(() => {
          this.clearValidate();
        });
      });
    },
    getBeforeKpiGroup() { // 新导入最近一次数据new
      this.$axios.post(Api.performance.getBeforeKpiGroup,
        { data: { manageType: 'work_and_manager_target', assessmentCycle: 'month', targetTime: this.dateForm.currentMonth.replace('-', '') } }, (res) => {
          if (res.isSuccess) {
            var data = res.data;
            data.kpiList.forEach(item => {
              if (!item.indicatorsType) {
                item.indicatorsType = { id: '' };
              }
              item.score = '';
              item.weight = math.multiply(item.weight , 100)
              if (item.criteriaItems) {
                var l = item.criteriaItems.length;
                for (let i = l;i < 5;i++) {
                  item.criteriaItems.push({ criteria: '', score: '' });
                }
              }
            });
            this.form.targetTime = data.targetTime;
            this.form.extraPointsCondition = data.extraPointsCondition;
            this.form.deductPointsCondition = data.deductPointsCondition;
            // this.form.extraPointsKey = data.extraPointsKey;
            // this.form.extraPointsValue = data.extraPointsValue || 0; // 分值
            // this.form.deductPointsKey = data.deductPointsKey; // 扣分事由
            // this.form.deductPointsValue = data.deductPointsValue || 0; // 扣分值
            if (data.kpiDynamicItemList) {
              // var cur = data.kpiDynamicItemList.find(i => i.kpiDynamicType == 'important_work_month');
              // cur && (this.form.currentMonthDetail = cur.recordValue);
              // var next = data.kpiDynamicItemList.find(i => i.kpiDynamicType == 'important_work_next_month');
              // var advantage = data.kpiDynamicItemList.find(i => i.kpiDynamicType == 'advantage');
              // var disadvantage = data.kpiDynamicItemList.find(i => i.kpiDynamicType == 'disadvantage');
              // next && (this.form.nextMonthWork = next.desc);
              // advantage && (this.form.advantage = advantage.desc);
              // disadvantage && (this.form.disadvantage = disadvantage.desc);
            };
            this.form.selfEvaluation = data.selfEvaluation; // 自我评价
            // this.form.leaderEvaluation = data.leaderEvaluation;
            // this.form.punishPonitsKey = data.punishPonitsKey;
            // this.form.rewardPonitsValue = data.rewardPonitsValue || 0;
            // this.form.rewardPonitsKey = data.rewardPonitsKey;
            // this.form.punishPonitsValue = data.punishPonitsValue || 0;
            this.form.salarvStructure = data.punishPonitsValue || 0;
            this.form.subsidy = data.subsidy;
            this.form.workDays = data.workDays;
            var indicatorsList = data.kpiList || [];
            this.form.keyPerformanceIndicatorsList = this.form.keyPerformanceIndicatorsList.filter(item=>item?.indicatorsType?.id)
            this.form.keyPerformanceIndicatorsList.push(...indicatorsList)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 导入最近一次数据 没调用，改成上面的getBeforeKpiGroup方法来导入数据
    importRecentData() {
      this.$refs['dateForm'].validate(res => {
        if (res) {
          this.getBeforeKpiGroup();
        }
      })
    },
    fetchData(targetTime) {
      const param = {
        data: {
          manageType: 'work_and_manager_target',
          targetTime: targetTime || undefined
        }
      };

      return this.$axios.post(
        Api.performance.getKpiGroupList,
        param
        // res => {
        //   if (res.isSuccess) {
        //     this.tableData = res.data ? res.data : [];
        //   } else {
        //     this.$message.error(res.message);
        //   }
        // }
      );
    },
    // 导入数据
    handleImport() {
      this.$refs['dateForm'].validate(res => {
        if (res) {
          this.importDialogVisible = true;
        }
      })
      // if (!this.dateForm.currentMonth) return this.$message.error('请先选择考核月份')
    },
    cancel() {
      if (this.$route?.params?.from) {
        this.$router.go(-1);
      } else {
        this.prevStepHandle();
      }
    },
    submit(status) {
      if (status == 0) {
        this.status = 'not_submitted';
        this.submitData('form', 'nocheck').then(() => {
          this.bindBatchFileByIds(this.editId); // 更新月度绩效考核id绑定附件文件ids
          this.$message.success('操作成功');
          this.prevStepHandle();
        });
      } else {
        this.status = status;
        if (status == 'under_review') {
          this.sumbitFlow('submit');
        } else {
          this.sumbitFlow('draft');
        }
      }
      // 提交之前的校验,之后自动调用 postData方法提交业务和绑定流程
    },
    save() {
      this.status = 'not_submitted';
      this.submitData('form', 'nocheck').then((res) => {
        this.$message.success('操作成功');
        this.bindBatchFileByIds(res?.id || this.editId); // 更新月度绩效考核id绑定附件文件ids
        this.$router.go(-1);
      });
    },
    // 提交流程,绑定业务
    postData(status, batchCode) {
      let state;
      if (this.status == 'not_submitted') {
        state = 'nocheck';
      }
      this.submitData('form', state, batchCode).then(res => {
        const name = `${this.dateForm.currentMonth}月度绩效考核-${localstorageGet('userName')}`;
        this.submitFlowFinal(true, res.id, '', '', name);
        this.bindBatchFileByIds(res.id); // 提交新增月度绩效考核id绑定附件文件ids
      }).catch(err => { console.log('err', err); });
    },
    // 提交
    submitData(form, needValidate, batchCode) {
      this.fileAttachIds = this.$refs.eleupload.getFileId();
      // needValidate === undefined 什么都不传的时候说明是正式提交需要校验，传值，就不校验
      return new Promise((resolve, reject) => {
        let result = true;
        if (needValidate === undefined) {
          for (let item of this.$refs['dateForm'].fields) {
            let propArr = item.prop.split('.')
            let propName = propArr[propArr.length-1]
            if(this.permisionList.indexOf(propName)> -1){
              this.$refs['dateForm'].validateField(item.prop, (error) => {
                if (error) {
                  result = false;
                }
              });
            }
          }
          for (const item of this.$refs[form].fields) {
            let propArr = item.prop.split('.')
            let propName = propArr[propArr.length-1]
            if(this.permisionList.indexOf(propName)> -1){
              this.$refs[form].validateField(item.prop, (error) => {
                if (error) {
                  result = false;
                }
              });
            }
            if (this.permisionList.indexOf('target') > -1 && item.prop.endsWith('indicatorsType.id')) {
              this.$refs[form].validateField(item.prop, (error) => {
                if (error) {
                  result = false;
                }
              });
            }
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
        }
        if (result) {
          // 总权重不能大于1
          if (this.totalWeight != 100 && needValidate === undefined) {
            this.$message.error('总权重必须为100%');
            this.$parent.$parent.$parent.submitLoading = false
            if(this.examneObj)this.examneObj.submitLoading = false
            reject();
            return;
          }
          let postUrl = Api.performance.postKpiGroup;
          // let postUrl = 'http://192.168.1.171:9035/planCenterApi/kpiGroup/save'
          if (this.actionType == 'create') {
            this.form.keyPerformanceIndicatorsList.forEach((item, index) => {
              const id = item.id;
              item.sort = index;
              if (this.originData) {
                const find = this.originData.find(el => el.id == id);
                // item.yearKpi = find;
                if (find) {
                  item.yearKpi = deepClone(item);
                }
              }
            });
          }
          const params = {
            manageType: 'work_and_manager_target', // 指标类型
            assessmentCycle: 'month', // this.form.assessmentCycle,
            keyPerformanceIndicatorsList: deepClone(this.form.keyPerformanceIndicatorsList),
            kpiDynamicItemList: [{
              desc: this.form.currentMonthWork, // 本月工作重点，如果动态获取上月的月度绩效内容，这个值，可以不填
              recordValue: this.form.currentMonthDetail, // 本月工作重点完成情况
              kpiDynamicType: 'important_work_month'// 固定值
            }, {
              desc: this.form.nextMonthWork, // 下个月重点工作
              recordValue: '', // 不填
              kpiDynamicType: 'important_work_next_month'// 固定值
            }, {
              desc: this.form.changeSalary, // 调薪
              recordValue: '',
              kpiDynamicType: 'change_salary'// 调薪固定值
            }, {
              desc: this.form.askForLeave, // 请假
              recordValue: '',
              kpiDynamicType: 'ask_for_leave'// 请假固定值
            }, {
              desc: this.form.advantage,
              kpiDynamicType: 'advantage'
            }, {
              desc: this.form.disadvantage,
              kpiDynamicType: 'disadvantage'
            }]
          };
          if (this.editType == 2) {
            params.id = this.editId;
            postUrl = Api.performance.updateKpiGroup;
            // postUrl = 'http://192.168.1.171:9035/planCenterApi/kpiGroup/update';
            params.leaderEvaluation = this.form.leaderEvaluation; // 领导评价，新增不传，领导审批的时候提交
            // params.targetTime=this.form.targetTime//目标时间，新增不传
          }
          // 20237
          const currentMonth = this.dateForm.currentMonth;
          params.targetTime = currentMonth.replace('-', '');// '20237'//'2023-07-01'//this.currentMonth+'-01'//目标时间，新增不传
          params.kpiGroupStatus = this.status;
          params.extraPointsCondition = this.form.extraPointsCondition;
          params.extraPointsKey = this.form.extraPointsKey;
          params.extraPointsValue = this.form.extraPointsValue || 0;
          params.recoupPonitsKey = this.form.recoupPonitsKey;
          params.recoupPonitsValue = this.form.recoupPonitsValue || 0;
          params.deductPointsCondition = this.form.deductPointsCondition;
          params.deductPointsKey = this.form.deductPointsKey;
          params.deductPointsValue = this.form.deductPointsValue || 0;
          params.selfEvaluation = this.form.selfEvaluation;
          params.punishPonitsKey = this.form.punishPonitsKey;
          params.rewardPonitsValue = this.form.rewardPonitsValue || 0;
          params.rewardPonitsKey = this.form.rewardPonitsKey;
          params.punishPonitsValue = this.form.punishPonitsValue || 0;
          params.subsidy = this.form.subsidy;
          params.workDays = this.form.workDays;
          params.keyPerformanceIndicatorsList.forEach((item, index) => {
            item.weight = math.divide(item.weight, 100); // item.weight = item.weight / 100
          });
          if (this.actionType == 'examine') {
            // params.targetTime://目标时间，新增不传
          }

          this.$axios.post(
            postUrl,
            {
              batchCode,
              data: params
            },
            res => {
              if (res.isSuccess) {
                resolve(res.data);
                // this.$message.success(this.editType == 2 ? "编辑成功" : "创建成功")
              } else {
                reject();
                this.$message.error(res.message);
              }
              // this.goback()
            }
          );
        } else {
          reject();
          this.$message.error('有必填项未填，请检查月度绩效表格');
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          //滑动到第一个报错的dom
          this.$nextTick(() => {
            // 校验报错页面定位
            setTimeout(() => {
              const element = document.getElementsByClassName('is-error')[0];
              element.scrollIntoView({
                behavior: 'smooth',
                block: 'center'
              });
            }, 200);
          })
        }
      });
    },
    bindBatchFileByIds(relationId) { // 多个文件绑定业务id
      const fileIds = this.$refs.eleupload?.getFileId() || this.fileAttachIds;
      if (fileIds && fileIds.length) {
        const data = {
          relationId,
          fileIds
        };
        this.$axios.post(
          Api.budgetManage.saveBatchFile,
          { data }
        );
      }
    }
  }
};
</script>
<style lang="scss" scoped>
::v-deep .el-textarea .el-input__count {
  background: transparent;
}

::v-deep .elFormExpanded {
  width: 100%;

  .el-form-item__content {
    width: 100%;

    .el-textarea {
      width: 100%;
    }
  }
}

.el-table,
.tb-row {
  ::v-deep tbody td {
    padding: 1.5px;

    .cell {
      padding: 1.5px;

      button {
        padding: 0;
      }
    }
  }

  ::v-deep .el-form-item {
    margin-right: 0;
  }

  ::v-deep .el-textarea textarea {
    // border: none;
    // background-color: transparent;
    // padding: 0px;
    // resize: none;
    padding: 3px 2px;
  }

  ::v-deep .el-input input {
    // border: none;
    // background-color: transparent;
    // padding: 0px;
    padding: 3px 2px;
  }
}

.target-book-container {
  ::-webkit-scrollbar {
    width: 6px;
  }

  .btn-box {
    border: 1px solid #666;
    border-bottom: 0;
    padding: 5px 10px;
  }

  ::v-deep .el-form-item__error {
    padding-top: 0;
    margin-top: 3px;
  }

  .grade-container {
    border: 1px solid #666;
    line-height: 40px;

    .weight {
      span {
        display: inline-block;
        width: 80px;
      }
    }

    .grade {
      width: 50px;
      text-align: center;
      border-left: 1px solid #666;
    }
  }
}

h3 {
  text-align: center;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px 0;

  .el-form-item {
    margin-bottom: 0;
    margin-right: 0;
    font-size: 16px;
  }

}

.el-table td.el-table__cell div {
  margin-right: 0 !important;
  margin-bottom: 0 !important;
  width: 100%;

  .el-form-item__content {
    width: 100%;
  }
}

.footer-bt {
  text-align: center;
  background: transparent;
  width: 100%;
  position: absolute;
  left: 0;
  bottom: 0;
  z-index: 1000;
  pointer-events: none;

  .footer-inner {
    background: #fff;
    padding: 25px 0;
    pointer-events: all;
    padding-top: 0;
  }
}

::v-deep .el-input.is-disabled .el-input__inner,
::v-deep .el-textarea.is-disabled .el-textarea__inner {
  color: #333 !important;
  // background:transparent !important;
  // border:none;
}

::v-deep .el-input.is-disabled .el-input-group__append {
  border: none !important;
  background: #fff;
}


::v-deep input::-webkit-outer-spin-button,
::v-deep input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  appearance: none;
  margin: 0;
}

.tb-row {
  border-bottom: 1px solid #666;
  flex-shrink: 0;

  .tb-title {
    width: 134px;
    text-align: center;
    flex-shrink: 0;
    line-height: normal;
    color: #333 !important;
    font-weight: bold;
    font-size: 14px;
  }

  >div {
    text-align: center;
    align-items: center;
    flex-shrink: 1;
    // height: 100%;
    display: inline-block;
    font-size: 14px;
  }

  textarea {
    font-size: 14px;
  }
}

.border-right {
  border-right: 1px solid #666;
}

.border-left {
  border-left: 1px solid #666;
}

.tb-col {
  height: 100%;
}

::v-deep .no-inline .el-form-item__content,
::v-deep .no-inline .el-form-item--mini {
  display: block !important;
}

::v-deep .no-inline .el-form-item--mini.el-form-item {
  margin-bottom: 0 !important;
}

::v-deep .el-table td.el-table__cell div .time-tag-style {
  color: #fff;
  border-radius: 16px;
  background: rgb(47, 194, 91);
  text-align: center;
  padding: 2px 6px;
}

.text-center {
  text-align: center;
}

$inputbcolor: #d4d4d4;
$bcolor: #666;
$back: #333;
$fontColor: #333;

::v-deep .box-card.el-card {
  border: none;
}

::v-deep .box-card {
  .el-input-group__append {
    padding: 0 5px !important;
    color: $fontColor;
  }

  .el-table .el-form-item--mini .el-form-item__content {
    width: 100%;
  }

  .el-form-item__label {
    color: $back !important;
    font-weight: 900 !important;
  }

  .el-table thead {
    color: $back !important;
    font-size: 14px;
  }

  .el-textarea.is-disabled .el-textarea__inner,
  .el-input.is-disabled .el-input__inner {
    background: #fff;
    border: none;
    resize: none;

  }

  .el-input__inner,
  .el-textarea__inner {
    background: #fff;
    border-color: $inputbcolor;
    color: $fontColor;
    font-size: 14px;
  }

  .el-input__inner:focus,
  .el-textarea__inner:focus {
    border-color: #1989FA;
  }

  .el-form-item.is-error .el-input__inner,
  .el-form-item.is-error .el-textarea__inner {
    border-color: #ff0000;
  }

  // .el-textarea__inner:focus
  .el-table th.el-table__cell.is-leaf,
  .el-table td.el-table__cell {
    border-color: $bcolor;
    background: #fff;
  }

  .el-table--group,
  .el-table--border {
    border-color: $bcolor;
  }

  .el-table--border::after,
  .el-table::before {
    background: $bcolor;
  }

  .el-table thead.is-group th.el-table__cell {
    border-color: $bcolor;
    background: #fff;
  }

  .el-input.is-disabled .el-input__icon {
    display: none;
  }

  .el-input input {
    font-size: 14px;
  }

  .top-form {
    border: 1px solid $bcolor;
    border-bottom: none;
    font-weight: 900;
    display: flex;

    .el-form-item {
      margin-bottom: 0;
      padding: 5px 0;
      padding-left: 10px;
      border-right: 1px solid #666;
      margin-right: 0;
    }
  }

  .current-row td {
    background: rgba(230, 230, 240, 0.8) !important;
  }

  .el-table__row td:first-child {
    cursor: pointer;
  }

  .el-date-editor input {
    font-weight: bold;
    padding-right: 0px;
    padding-left: 25px;
    text-align: center;
  }

  .el-table__body .cell {
    padding: 1.5px;
    font-size: 14px;
    color: $fontColor;
  }

  .el-card__body {
    padding: 10px;
  }

  .el-table th.el-table__cell>.cell {
    padding-left: 5px;
    padding-right: 5px;
  }

  .el-table__header,
  .el-table__body,
  .el-table__footer {
    font-weight: 900;
    font-size: 15px;
  }

  .file-div {
    border: 1px solid $bcolor;
    border-bottom: none;
    display: flex;
    align-items: center;

    .title {
      width: 151px;
      border-right: 1px solid $bcolor;
      padding: 8px 0 8px 10px;
      color: $fontColor;
      font-weight: bold;
    }

    .el-form-item--mini.el-form-item {
      margin-bottom: 0;
    }

    .el-form-item__error {
      top: 5px;
      left: 100%;
      width: 110px;
      font-weight: bold;
    }

    .attach-ul .attach-li {
      margin-top: 0px;
    }
  }

  .el-textarea .el-input__count {
    font-size: 10px;
    bottom: -4px;
    font-weight: 100;
  }

  #perform-footer {
    background: #fff;
    z-index: 1;
  }

  .weight-class input {
    text-align: center !important;
  }

  .danwei .el-form-item__content {
    width: calc(100% - 100px);
  }
}

.zoom {
  display: flex;
  position: absolute;
  top: 6px;
  right: 0;
  z-index: 1;

  .zoom-button {
    color: #409EFF;
    font-size: 22px;
    padding: 0 4px;
    cursor: pointer;
    user-select: none;
    width: 30px;
    text-align: center;
  }

  .zoom-button.disabled {
    cursor: not-allowed;
  }

  >span {
    display: inline-block;
    width: 50px;
    text-align: center;
    line-height: 36px;
  }
}

.check-history {
  position: absolute;
  top: 6px;
  left: 0;
  z-index: 1;
  font-size: 15px;
  cursor: pointer;
  color: #409EFF;
}
::v-deep .bigInputClass .el-textarea__inner{
  max-height:56vh !important;
  font-size:16px;
}
.row-remove-plus {
  position: absolute;
  inset: 0;
  i {
    // display: block;
    font-size: 15px;
  }
  i:hover{
    cursor: pointer;
    filter: opacity(0.75);
  }
}
</style>
