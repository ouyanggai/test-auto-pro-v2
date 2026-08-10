<template>
  <div class="table-merge" id="quarter-perf-assess">
    <el-form :model="formData" ref="formRef" @submit.native.prevent="getSubmitData">
      <!-- <h3 id="title" class="font-large">
        <span class="font-large">{{`润世华集团${propData.assessType == 'personal' ? '个人' : '组织'}绩效考核表`}}</span>
      </h3> -->
       <h3 id="title" class="font-large">
        <el-form-item prop="year" style="display:inline-block;" class="font-large" :rules="!perm.includes('year') ? rules.isNeed : rules.isNoNeed">
          <el-date-picker type="year" value-format="yyyy" v-model="formData.year" :clearable="false" style="width:60px;" class="font-large" @change="getQuarterPlan"
        :disabled="!perm.includes('year') || actionType == 'preview'">
          </el-date-picker><span class="font-large">年</span>
        </el-form-item>
        <el-form-item prop="quarter" style="display:inline-block;" class="font-large" :rules="perm.includes('quarter') ? rules.isNeed : rules.isNoNeed">
          <el-select ref="quarterPicker" style="width:50px;" placeholder=" " v-model="formData.quarter" class="font-large" @change="getQuarterPlan"
        :disabled="!perm.includes('quarter') || actionType == 'preview'">
            <el-option
              v-for="item in [{ label: '一', value: 1 }, { label: '二', value: 2 }, { label: '三', value: 3 }, { label: '四', value: 4 }]"
              :key="item.value" :label="item.label" :value="item.value">
            </el-option>
          </el-select><span class="font-large">季度{{propData.assessType == 'personal' ? '个人' : '组织'}}绩效考核表</span>
        </el-form-item>
      </h3>
      <table class="mytable">
        <tr>
          <td style="width:100px;" class="fontWeighted" v-if="propData.assessType == 'personal'">姓名</td>
          <td v-if="propData.assessType == 'personal'">
            <!-- <el-form-item prop="userName" :rules="perm.includes('user') ? rules.isNeed : rules.isNoNeed">
              <el-input v-model="formData.userName" :disabled="!perm.includes('user')"></el-input>
            </el-form-item> -->
            <div>{{ formData.userName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">公司</td>
          <td>
            <div>{{ formData.companyName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted" v-if="propData.assessType == 'personal'">部门</td>
          <td v-if="propData.assessType == 'personal'">
            <div>{{ formData.departmentName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted" v-if="propData.assessType == 'personal'">岗位</td>
          <td v-if="propData.assessType == 'personal'">
            <div>{{ formData.dutyName }}</div>
          </td>
          <!-- <td style="width:110px;" class="fontWeighted">考核周期</td>
          <td style="width:150px">
            <el-select placeholder=" " v-model="formData.quarter" @change="getQuarterPlan">
              <el-option
                v-for="item in [{ label: '第一季度', value: '1' }, { label: '第二季度', value: '2' }, { label: '第三季度', value: '3' }, { label: '第四季度', value: '4' }]"
                :key="item.value" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
          </td> -->
        </tr>
      </table>
      <table class="mytable">
        <tr>
          <td class="fontWeighted" :colspan="propData.scoreType == 'twice' ? 10 : 9">
            <div>
               <span class="plan-title">1、关键业绩指标（KPI）</span>
               <span style="margin-left:20px;display:inline-flex;align-items:center;">
                 权重：<el-form-item :prop="`kpi.percent`" style="display:inline-block;" :rules="perm.includes('percent1') ? rules.isNeed : rules.isNoNeed">
                 <el-input-number :min="1" :max="100" v-model="formData.kpi.percent" placeholder="" :controls="false" style="width:60px"
                  :disabled="!perm.includes('percent1') || actionType == 'preview' || propData.assessType == 'company'" @change="sumScoreChange('kpi')"></el-input-number>
                  <!-- @change="sumScoreChange('kpi', propData.assessType == 'personal' ? 'ability' : 'temporary')" -->
                </el-form-item>
              </span>
            </div>
          </td>
        </tr>
        <tr>
          <th style="width:50px">序号</th>
          <th>指标名称</th>
          <th>指标定义/计算公式</th>
          <th>指标权重</th>
          <th>挑战值(5分)</th>
          <th>达标值(3分)</th>
          <th>底限值(1分)</th>
          <th>实际完成</th>
          <th v-if="propData.scoreType == 'once'">得分</th>
          <th v-if="propData.scoreType == 'twice'">主评分人(60%)</th>
          <th v-if="propData.scoreType == 'twice'">评分人(40%)</th>
          <!-- <th v-if="propData.scoreType == 'twice'">业务部门评分</th> -->
          <!-- <th v-if="propData.scoreType == 'twice'">管理部门评分</th> -->
        </tr>
        <tr v-if="formData.kpi.list.length == 0"><td :colspan="propData.scoreType == 'twice' ? 10 : 9">暂无数据</td></tr>
        <tr v-for="(k, index) in formData.kpi.list" :key="k.id">
          <td style="line-height:initial;">
            {{ index + 1 }}
            <!-- <div class="row-remove-plus">
              <i class="el-icon-remove" title="删除行" style="color:#f56c6c" @click="DelFirst(scope, formData.workPlans)"></i>
              <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="InsertFirst(scope, formData.workPlans)"></i>
            </div> -->
          </td>
          <td>{{ k.phasedAchieve}}</td><!-- {{ k.ultimateAchieve + " " + k.phasedAchieve}} -->
          <td>{{ k.appraisalMethod }}</td>
          <td>{{ k.weight }}</td>
          <td>{{ handleItem(k.values, 'high_difficulty') }}</td>
          <td>{{ handleItem(k.values, 'intermediate_difficulty') }}</td>
          <td>{{ handleItem(k.values, 'easy') }}</td>
          <td style="width:27%">
            <el-form-item :prop="`kpi.list.${index}.assessmentRemarks`" :rules="perm.includes('kpi_work') ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="k.assessmentRemarks" :autosize="{minRows:2,maxRows:10}"
              :disabled="!perm.includes('kpi_work') || actionType == 'preview'"></el-input>
              <i class="el-icon-zoom-in" title="放大查看/填写" style="position:absolute;right:2px;top:2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:medium;"
              @click.stop="showBigInput({data: k, value: k.assessmentRemarks, key: 'assessmentRemarks', auth: 'kpi_work'})" ></i>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'once'" style="width:60px">
            <el-form-item :prop="`kpi.list.${index}.score`" style="display:inline-block;" :rules="perm.includes('kpi_score') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="k.score" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('kpi',{data:k,key:'score'})"
              :disabled="!perm.includes('kpi_score') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`kpi.list.${index}.score`" style="display:inline-block;" :rules="perm.includes('kpi_score') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="k.score" placeholder="" :controls="false" style="width:100px" @change="sumScoreChange('kpi',{data:k,key:'score'})"
                :disabled="!perm.includes('kpi_score') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`kpi.list.${index}.bossScore`" style="display:inline-block;" :rules="perm.includes('kpi_bossScore') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="k.bossScore" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('kpi',{data:k,key:'bossScore'})"
              :disabled="!perm.includes('kpi_bossScore') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
        </tr>
        <tr>
          <td colspan="3" class="fontWeighted">权重合计</td>
          <td class="fontWeighted">{{ formData.kpi.weight }}</td>
          <td colspan="4" class="fontWeighted">此项指标得分</td>
          <td :colspan="propData.scoreType == 'twice' ? 2 : 1" class="fontWeighted">{{ processDecimal(formData.kpi.score) }}</td>
        </tr>
      </table>
      <table class="mytable" v-if="propData.assessType == 'personal'">
        <tr>
          <td class="fontWeighted" :colspan="propData.scoreType == 'twice' ? 7 : 6">
            <div>
               <span class="plan-title">
                {{'2、能力态度指标'}}
              </span>
              <span style="margin-left:20px;display:inline-flex;align-items:center;">
                 权重：<el-form-item :prop="`ability.percent`" style="display:inline-block;" :rules="perm.includes('percent2') ? rules.isNeed : rules.isNoNeed">
                 <el-input-number :min="1" :max="20" v-model="formData.ability.percent" placeholder="" :controls="false" style="width:60px"
                  :disabled="!perm.includes('percent2') || actionType == 'preview'" @change="sumScoreChange(propData.assessType == 'personal' ? 'ability' : 'temporary')"></el-input-number>
                </el-form-item>
              </span>
            </div>
          </td>
        </tr>
        <tr>
          <th style="width:50px">序号</th>
          <th>{{'指标名称'}}</th>
          <th style="width:22%">{{'指标内容'}}</th>
          <th>指标权重</th>
          <th>{{ '实际情况/提升建议'}}</th>
          <th v-if="propData.scoreType == 'once'">主管评分</th>
          <th v-if="propData.scoreType == 'twice'">主评分人(60%)</th>
          <th v-if="propData.scoreType == 'twice'">评分人(40%)</th>
          <!-- <th v-if="propData.scoreType == 'twice'">主管领导评分</th>
          <th v-if="propData.scoreType == 'twice'">管理部门评分</th> -->
        </tr>
        <tr v-if="formData.ability.list.length == 0"><td :colspan="propData.scoreType == 'twice' ? 7 : 6">暂无数据</td></tr>
        <tr v-for="(v, index) in formData.ability.list" :key="v.id">
          <td style="line-height:initial;">
            {{ index + 1 }}
            <!-- <div class="row-remove-plus">
              <i class="el-icon-remove" title="删除行" style="color:#f56c6c" @click="DelFirst(scope, formData.workPlans)"></i>
              <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="InsertFirst(scope, formData.workPlans)"></i>
            </div> -->
          </td>
          <td>{{ v.ultimateAchieve }}</td>
          <td>{{ v.appraisalMethod }}</td>
          <td>{{ v.weight }}</td>
          <td style="width:27%">
            <el-form-item :prop="`ability.list.${index}.assessmentRemarks`" :rules="perm.includes('tpi_improve') ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="v.assessmentRemarks" :autosize="{minRows:2,maxRows:10}"
             :disabled="!perm.includes('tpi_improve') || actionType == 'preview'"></el-input>
              <i class="el-icon-zoom-in" title="放大查看/填写" style="position:absolute;right:2px;top:2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:medium;"
              @click.stop="showBigInput({data: v, value: v.assessmentRemarks, key: 'assessmentRemarks', auth: 'tpi_improve'})" ></i>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'once'" style="width:60px">
            <el-form-item :prop="`ability.list.${index}.score`" style="display:inline-block;" :rules="perm.includes('tpi_score') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="v.score" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('ability',{data:v,key:'score'})"
            :disabled="!perm.includes('tpi_score') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`ability.list.${index}.score`" style="display:inline-block;" :rules="perm.includes('tpi_score') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="v.score" placeholder="" :controls="false" style="width:100px" @change="sumScoreChange('ability',{data:v,key:'score'})"
              :disabled="!perm.includes('tpi_score') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`ability.list.${index}.bossScore`" style="display:inline-block;" :rules="perm.includes('tpi_bossScore') ? rules.isNeed : rules.isNoNeed">
              <el-input-number :min="0" :max="5" v-model="v.bossScore" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('ability',{data:v,key:'bossScore'})"
              :disabled="!perm.includes('tpi_bossScore') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
        </tr>
        <tr>
          <td colspan="3" class="fontWeighted">权重合计</td>
          <td class="fontWeighted">{{ formData.ability.weight }}</td>
          <td class="fontWeighted">此项指标得分</td>
          <td :colspan="propData.scoreType == 'twice' ? 2 : 1" class="fontWeighted">{{ processDecimal(formData.ability.score) }}</td>
        </tr>
      </table>
      <table class="mytable" v-if="propData.assessType == 'company' && !formData.switchVal && (actionType=='create' || operaType=='reEdit')">
        <tr>
          <td style="text-align:left;">
            <el-button type="primary" size="mini" v-if="actionType=='create' || operaType=='reEdit'"
                @click="switchTempTask">新增临时任务</el-button>
          </td>
        </tr>
      </table>
      <table class="mytable" v-if="propData.assessType == 'company' && formData.switchVal">
        <tr>
          <td class="fontWeighted" :colspan="propData.scoreType == 'twice' ? 7 : 6">
            <div style="position: relative;">
              <span style="float:left;margin:1px;width:0;overflow:visible;">
                <el-button type="primary" size="mini" v-if="actionType=='create' || operaType=='reEdit'"
                @click="switchTempTask">删除所有临时任务</el-button>
              </span>
              <!-- <span style="position: absolute;left: 10px;top: 4px;" >
                  临时交办任务：<el-switch v-model="switchVal"></el-switch>
                </span> -->
               <span class="plan-title">
                {{'2、临时交办的重点工作任务指标（TPI）'}}
              </span>
              <span style="margin-left:20px;display:inline-flex;align-items:center;">
                权重：<el-form-item :prop="`temporary.percent`" :rules="perm.includes('percent2') ? rules.isNeed : rules.isNoNeed" style="display:inline-block;">
                  <el-input-number :min="1" :max="20" v-model="formData.temporary.percent" placeholder="" :controls="false" style="width:60px"
                  :disabled="!perm.includes('percent2') || actionType == 'preview'" @change="_=>{sumScoreChange(propData.assessType == 'personal' ? 'ability' : 'temporary');changeTempPercent();}"></el-input-number>
                </el-form-item>
              </span>
            </div>
          </td>
        </tr>
        <!-- <template v-if="formData.temporary.percent > 0"> -->
        <tr>
          <th style="width:50px">序号</th>
          <th>{{'重点工作任务' }}</th>
          <th style="width:22%">{{ '任务验收标准' }}</th>
          <th>指标权重</th>
          <th>{{ '实际完成' }}</th>
          <th v-if="propData.scoreType == 'once'">主管评分</th>
          <th v-if="propData.scoreType == 'twice'">主评分人(60%)</th>
          <th v-if="propData.scoreType == 'twice'">评分人(40%)</th>
          <!-- <th v-if="propData.scoreType == 'twice'">主管领导评分</th>
          <th v-if="propData.scoreType == 'twice'">管理部门评分</th> -->
        </tr>
        <tr v-if="formData.temporary.list.length == 0"><td :colspan="propData.scoreType == 'twice' ? 7 : 6">暂无数据</td></tr>
        <tr v-for="(v, index) in formData.temporary.list" :key="v.id">
          <td style="line-height:initial;">
            {{ index + 1 }}
            <div class="row-remove-plus no-print" v-if="perm.includes('insertDel') && actionType != 'preview'">
              <i class="el-icon-remove" title="删除行" style="color:#f56c6c" @click="DelTemporary(index, formData.temporary.list)"></i>
              <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="InTemporary(index, formData.temporary.list)"></i>
            </div>
          </td>
          <td>
            <el-form-item :prop="`temporary.list.${index}.ultimateAchieve`" :rules="perm.includes('tpi_work') && formData.switchVal ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="v.ultimateAchieve" :autosize="{minRows:2,maxRows:10}"
              :disabled="!perm.includes('tpi_work') || actionType == 'preview'"></el-input>
            </el-form-item>
          </td>
          <td>
            <el-form-item :prop="`temporary.list.${index}.appraisalMethod`" :rules="perm.includes('tpi_detail') && formData.switchVal ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="v.appraisalMethod" :autosize="{minRows:2,maxRows:10}"
          :disabled="!perm.includes('tpi_detail') || actionType == 'preview'"></el-input>
            </el-form-item>
          </td>
          <td style="width:50px">
            <el-form-item :prop="`temporary.list.${index}.weight`" :rules="perm.includes('tpi_weight') && formData.switchVal ? rules.isNeed : rules.isNoNeed" style="display:inline-block;">
              <el-input-number :max="100" v-model="v.weight" placeholder="" :controls="false" style="width:70px" @change="temporaryWeightChange"
              :disabled="!perm.includes('tpi_weight') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td style="width:27%">
            <el-form-item :prop="`temporary.list.${index}.assessmentRemarks`" :rules="perm.includes('tpi_improve') && formData.switchVal ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="v.assessmentRemarks" :autosize="{minRows:2,maxRows:10}"
              :disabled="!perm.includes('tpi_improve') || actionType == 'preview'"></el-input>
              <i class="el-icon-zoom-in" title="放大查看/填写" style="position:absolute;right:2px;top:2px;cursor:pointer;color:#409EFF;font-weight:600;font-size:medium;"
              @click.stop="showBigInput({data: v, value: v.assessmentRemarks, key: 'assessmentRemarks', auth: 'tpi_improve'})" ></i>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'once'" style="width:60px">
            <el-form-item :prop="`temporary.list.${index}.score`" :rules="perm.includes('tpi_score') && formData.switchVal ? rules.isNeed : rules.isNoNeed" style="display:inline-block;">
              <el-input-number :min="0" :max="5" v-model="v.score" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('temporary',{data:v,key:'score'})"
              :disabled="!perm.includes('tpi_score') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`temporary.list.${index}.score`" :rules="perm.includes('tpi_score') && formData.switchVal ? rules.isNeed : rules.isNoNeed" style="display:inline-block;">
              <el-input-number :min="0" :max="5" v-model="v.score" placeholder="" :controls="false" style="width:100px" @change="sumScoreChange('temporary',{data:v,key:'score'})"
              :disabled="!perm.includes('tpi_score') || actionType == 'preview'" ></el-input-number>
            </el-form-item>
          </td>
          <td v-if="propData.scoreType == 'twice'" style="width:60px">
            <el-form-item :prop="`temporary.list.${index}.bossScore`" :rules="perm.includes('tpi_bossScore') && formData.switchVal ? rules.isNeed : rules.isNoNeed" style="display:inline-block;">
              <el-input-number :min="0" :max="5" v-model="v.bossScore" placeholder="" :controls="false" style="width:90px" @change="sumScoreChange('temporary',{data:v,key:'bossScore'})"
              :disabled="!perm.includes('tpi_bossScore') || actionType == 'preview'"
              ></el-input-number>
            </el-form-item>
          </td>
        </tr>
        <tr>
          <td colspan="3" class="fontWeighted">权重合计</td>
          <td class="fontWeighted">{{ formData.temporary.weight }}</td>
          <td class="fontWeighted">此项指标得分</td>
          <td :colspan="propData.scoreType == 'twice' ? 2 : 1" class="fontWeighted">{{ processDecimal(formData.temporary.score) }}</td>
        </tr>
      </table>
      <table class="mytable">
        <tr>
          <td class="fontWeighted" style="width:50%">考核总分（{{ propData.assessType == 'personal' ? '1.关键业绩指标得分+2.能力态度指标得分' : '1.关键业绩指标得分+2.重点工作任务指标得分' }}）</td>
          <td style="width:50%" class="fontWeighted">{{ processDecimal(formData.allScore) }}</td>
        </tr>
      </table>
    </el-form>
    <el-dialog v-if="bigInputVisible" :visible="bigInputVisible" :title="''" :close-on-click-modal="true" width="80%"
      @close="bigInputVisible=false" append-to-body style="height:100%;">
      <div>
        <el-input :disabled="!perm.includes(bigInputData.auth) || actionType == 'preview'" class="bigInputClass" type="textarea" :autosize="{minRows:10}" style="width:100%;"
         v-model="bigInputData.value" show-word-limit maxlength="5000"></el-input>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="bigInputVisible=false">取 消</el-button>
        <el-button @click="confirmBigInput" type="primary">确 定</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
/* eslint-disable*/
// import Comment from './components/comment.vue';
import { deepClone } from '@/utils';
import { localstorageGet } from '@/utils/auth';
import Api from '@/api';
import math from '@/utils/math.js';
import Sortable from 'sortablejs';
var rowDefaultData = _ => {
  return {
    planNumber: '',
    ultimateAchieve: '',
    phasedAchieve: '',
    appraisalMethod: '',
    weight: undefined,
    high_difficulty: '',
    intermediate_difficulty: '',
    easy: '',
    items: [
      { itemNumber: '', doWhat: '', result: '', endTime: '' }
    ]
  };
};
// const month = new Date().getMonth() + 1;
// var quarter = (Math.floor(month / 3) + 1) + '';
var quarterDefaultData = _ => {
  return {
    year: new Date().getFullYear() + '',
    // year: '1985',
    quarter: '',
    get targetTime() {
      return this.year + '' + this.quarter;
    },
    userName: '',
    switchVal: false,
    userId: '',
    companyId: '',
    companyName: '',
    departmentName: '',
    departmentId: '',
    dutyName: '',
    dutyId: '',
    kpiGroupStatus: 'under_review',
    workPlanGroupId: '',
    allScore: 0,
    kpi: {
      id: undefined,
      type: 'kpi',
      list: [],
      percent: 80,
      weight: 0,
      score: 0
    },
    ability: {
      id: undefined,
      type: 'Ability_and_attitude',
      list: [
        { ultimateAchieve: '知识经验/技能', phasedAchieve: '-', values: [], appraisalMethod: '专业知识、行业经验、岗位经验、岗位技能、办公自动化、公文写作等基础能力', weight: 25, assessmentRemarks: '', score: undefined, bossScore: undefined },
        { ultimateAchieve: '团队意识/能力', phasedAchieve: '-', values: [], appraisalMethod: '个人的团队意识、协作意识，以及团队协作、沟通协调、工作指导/辅导他人等能力', weight: 25, assessmentRemarks: '', score: undefined, bossScore: undefined },
        { ultimateAchieve: '组织能力/潜力', phasedAchieve: '-', values: [], appraisalMethod: '个人的组织能力、分析判断、事务决策、管理方法、影响他人等能力', weight: 25, assessmentRemarks: '', score: undefined, bossScore: undefined },
        { ultimateAchieve: '价值观/工作态度', phasedAchieve: '-', values: [], appraisalMethod: '企业文化吻合程度，以及责任心、执行力、主动性、积极性、配合性等方面的表现', weight: 25, assessmentRemarks: '', score: undefined, bossScore: undefined }
      ],
      percent: 20,
      weight: 100,
      score: 0
    },
    temporary: {
      id: undefined,
      type: 'temporary_task',
      list: [
        { ultimateAchieve: '', phasedAchieve: '-', values: [], appraisalMethod: '', weight: undefined, assessmentRemarks: '', score: undefined, bossScore: undefined },
      ],
      percent: 20,
      weight: 0,
      score: 0
    },
    // assessmentCycle: 'quarterly',
    // kpi2Type: '' // company_kpi对应组织绩效，personal_kpi对应个人绩效
    // scoreScheme: '', // single_scorer对应单评分，two_scorer对应双评分

    // noticeStatus: 'not_started',
    // leaderConfirmTime: '',
    // myselfConfirmTime: '',
    // subjectComments: [],
    // workPlans: [rowDefaultData()]
  };
};
export default {
  name: 'QuarterPerfAssess',
  components: { },
  props: {
    operaType: { // 新增：add，编辑：edit reEdit，查看：check
      type: String,
      default: ''
    },
    actionType: { // preview edit examine  create
      type: String,
      default: ''
    },
    perm: {
      type: Array,
      default: _ => []
    },
    quarterData: {
      type: Object,
      default: _ => quarterDefaultData()
    },
    propData: {
      type: Object,
      default: _ => {}
    },
    isReEditInitiator: { // 撤回重新发起时，当前登录人是否为发起人
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      bigInputData: { data: { v: '' }, value: '', key: 'v', auth: '' },
      bigInputVisible: false,
      kpi2TargetTypes: { kpi: 'kpi', Ability_and_attitude: 'ability', temporary_task: 'temporary' },
      math,
      publicity: { not_started: '未公示', in_progress: '公示中', end: '公示结束' },
      publicityColor: { not_started: 'danger', in_progress: 'success', end: '' },
      tableMaxHeight: window.innerHeight * 0.85, // :max-height="tableMaxHeight"
      formData: { },
      rules: {
        isNeed: [
          { required: true, message: ' ', trigger: 'blur' }
        ],
        notNeed: [
          { required: false, message: ' ', trigger: 'blur' }
        ],
        answer: [
          { required: true, message: '回答内容不能为空', trigger: 'blur' },
          { min: 1, max: 500, message: '长度在 1 到 500 个字符', trigger: 'blur' }
        ]
      }
    };
  },
  created() {
    console.log(this.propData, 'propData--新绩效考核propData');// assessType: personal/company ; scoreType: once twice
    window.abb2 = this;
    this.formData = this.quarterData;
    if (this.formData.otherBizId) {
      Object.defineProperty(this.formData, 'targetTime', { get() { return this.year + '' + this.quarter; } });
      this.formData.switchVal = this.formData.switchVal === undefined ? true : this.formData.switchVal;
    } else {
      // this.formData.year = new Date().getFullYear() + '';
      this.formData.kpi.percent = this.propData.assessType == 'personal' ? 80 : 100;
      this.$nextTick(() => {
        this.$refs?.quarterPicker?.$el?.click();
      });
      this.getUserInfo();
    }
    console.log(this.formData, 'this.formData');
  },
  mounted() {
    this.$nextTick(() => {
      if (this.perm.includes('addDelRow') && this.actionType != 'preview') {
        // this.initDrag();
      }
      // 撤回后重新发起时，且当前登录人是发起人，清空打分数据
      if (this.operaType === 'reEdit' && this.isReEditInitiator) {
        this.clearScores();
      }
    });
  },
  methods: {
    // 清空打分数据（撤回后重新发起时调用）
    clearScores() {
      // 清空 KPI 打分
      this.formData.kpi.list.forEach(item => {
        item.score = undefined;
        item.bossScore = undefined;
      });
      this.formData.kpi.score = 0;

      // 清空能力态度指标打分
      this.formData.ability.list.forEach(item => {
        item.score = undefined;
        item.bossScore = undefined;
      });
      this.formData.ability.score = 0;

      // 清空临时任务打分
      this.formData.temporary.list.forEach(item => {
        item.score = undefined;
        item.bossScore = undefined;
      });
      this.formData.temporary.score = 0;

      // 清空总分
      this.formData.allScore = 0;

      // 清除表单验证
      this.$nextTick(() => {
        this.$refs.formRef?.clearValidate();
      });
    },
    showBigInput(data) {
      this.bigInputData = data;
      this.bigInputVisible = true;
    },
    confirmBigInput() {
      var { data, value, key } = this.bigInputData;
      data[key] = value;
      this.bigInputVisible = false;
      this.$refs.formRef.clearValidate();
    },
    processDecimal(num) {
      if (this.isMoreThanTwoDecimal(num)) {
        return num.toFixed(2);
      } else {
        return num;
      }
    },
    isMoreThanTwoDecimal(num) {
      if (typeof num !== 'number' || isNaN(num)) {
        return false;
      }
      const str = num.toString();
      if (str.indexOf('.') === -1) {
        return false;
      }
      const decimalPart = str.split('.')[1];
      return decimalPart.length > 2;
    },
    InTemporary($index, data) {
      data.splice($index + 1, 0, { ultimateAchieve: '', phasedAchieve: '-', values: [], appraisalMethod: '', weight: undefined, assessmentRemarks: '', score: undefined, bossScore: undefined },);
      this.$refs.formRef.clearValidate();
    },
    DelTemporary($index, data) {
      if (data.length === 1) {
        // this.$message.error('至少保留一条数据');
        this.switchTempTask();
        return;
      }
      data.splice($index, 1);
      this.temporaryWeightChange();
      // this.temporaryScoreChange();
      this.$refs.formRef.clearValidate();
    },
    switchTempTask() {
      this.formData.switchVal = !this.formData.switchVal;
      if (!this.formData.switchVal) {
        this.formData.temporary = {
          id: undefined,
          type: 'temporary_task',
          list: [
            { ultimateAchieve: '', phasedAchieve: '-', values: [], appraisalMethod: '', weight: undefined, assessmentRemarks: '', score: undefined, bossScore: undefined },
          ],
          percent: 20,
          weight: 0,
          score: 0
        };
        this.formData.kpi.percent = 100;
      } else {
        this.formData.kpi.percent = 80;
        this.formData.temporary.parent = 20;
      }
      this.sumScoreChange('kpi');
      this.sumScoreChange('temporary');
    },
    changeTempPercent() {
      this.formData.kpi.percent = math.subtract(100, this.formData.temporary.percent);
      this.sumScoreChange('kpi');
      this.sumScoreChange('temporary');
    },
    sumScoreChange(type = 'kpi', val) {
      var nums = [0, 1, 3, 5];
      if (val && nums.indexOf(val.data[val.key]) == -1) {
        val.data[val.key] = null;
        this.$nextTick(() => {
          this.$set(val.data, val.key, undefined);
        });
        this.$message.error('分值范围为0、1、3、5');
      }
      var sum = 0;
      if (this.propData.scoreType === 'twice') {
        this.formData[type].list.forEach(v => {
          var weight = math.divide(v.weight, 100) || 0;
          var score = math.multiply(Number(v.score) || 0, 0.6);
          var bossScore = math.multiply(Number(v.bossScore) || 0, 0.4);
          var sumScore = math.multiply(math.add(score || 0, bossScore || 0), weight);
          sum = math.add(sum, sumScore);
        });
      } else {
        this.formData[type].list.forEach(v => {
          var weight = math.divide(v.weight || 0, 100);
          var sumScore = math.multiply(Number(v.score) || 0, weight);
          sum = math.add(sum, sumScore);
        });
      }
      var percent = math.divide(this.formData[type].percent || 0, 100) || 0;
      sum = math.multiply(sum || 0, percent);
      this.formData[type].score = sum;
      this.countAllScore('kpi', this.propData.assessType == 'personal' ? 'ability' : 'temporary');
      // this.countAllScore('kpi', type == 'kpi' ? 'ability' : type);
    },
    temporaryWeightChange() {
      var sumAll = this.formData.temporary.list.reduce((sum, item) => {
        return math.add(sum, (Number(item.weight) || 0));
      }, 0);
      this.formData.temporary.weight = sumAll;
      this.sumScoreChange('temporary');
    },
    countAllScore(type1, type2) {
      var score1 = this.formData[type1].score;
      var score2 = this.formData[type2].score;
      this.formData.allScore = math.add(score1, score2);
    },
    handleItem(items, type) {
      if (!items || items.length === 0) return '';
      var item = items.find(v => v.workPlanTargetType == type);
      return item ? item.remark : '';
    },
    getQuarterPlan() {
      if (!this.formData.year || !this.formData.quarter) return;
      this.$axios.post('/web/plan/api/kpi2Group/drawUpKpi', { data: { targetTime: this.formData.targetTime } },
        res => {
          this.formData.kpi.list = [];
          this.formData.kpi.weight = 0;
          this.formData.kpi.score = 0;
          this.formData.workPlanGroupId = '';
          // this.formData.temporary_task = [];
          if (res.isSuccess) {
            this.setTargetsData(res);
          }
        }
      );
    },
    setTargetsData(res) {
      var { data: { kpiList = [] } } = res;
      this.formData.workPlanGroupId = res.data.workPlanGroupId;
      kpiList.forEach(v => {
        var targetType = this.kpi2TargetTypes[v.kpi2TargetType];
        this.formData[targetType].list = v.items;
        // if (v.kpi2TargetType == 'kpi') {
          var sumAll = v.items.reduce((sum, item) => {
            return math.add(sum, (Number(item.weight) || 0));
          }, 0);
          this.formData[targetType].weight = math.multiply(sumAll, 100);
          // var sumScore = v.items.reduce((sum, item) => {
          //   return math.add(sum, (Number(item.score) || 0));
          // }, 0);
          // this.formData.kpi.score = sumScore;
          v.items.forEach(i => {
            i.weight = math.multiply(i.weight, 100) || 0;
            i.score = i.score ?? undefined;
            i.bossScore = i.bossScore ?? undefined;
          });
          this.$nextTick(()=>{
            this.sumScoreChange('kpi');
            this.sumScoreChange(this.propData.assessType == 'personal' ? 'ability' : 'temporary')
          })
        // }
      });
    },
    resetScore() {
      var targetType = this.propData.assessType == 'personal' ? 'ability' : 'temporary'; 
      this.formData.kpi.list.forEach(v => {
        v.score = undefined;
        v.bossScore = undefined;
      });
      this.formData[targetType].list.forEach(v => {
        v.score = undefined;
        v.bossScore = undefined;
      });
      this.$nextTick(()=>{
        this.sumScoreChange('kpi');
        this.sumScoreChange(targetType);
      });
    },
    getSubmitData(temporarySave) {
      return new Promise((resolve, reject) => {
        if (this.formData.kpi.list.length == 0) {
          this.$message.error('无业绩指标（KPI）');
          reject(false);
          return;
        }
        if (temporarySave) {
          resolve(this.processData());
          return;
        }
        this.$refs.formRef.validate(valid => {
          if (valid) {
            resolve(this.processData());
          } else {
            reject(false);
            // resolve(false);
            this.$message.error('请填写完整数据');
            return false;
          }
        })
      })
    },
    processData() {
      const data = deepClone(this.formData);
      const ajaxData = {
        id: data.id || undefined,
        targetTime: data.year + '' + data.quarter,
        // kpiGroupStatus: data.id ? undefined : 'under_review',
        kpiGroupStatus: 'under_review',
        assessmentCycle: 'quarterly',
        workPlanGroup: { id: this.formData.workPlanGroupId },
        scoreScheme: this.propData.scoreType == 'once' ? 'single_scorer' : 'two_scorer',
        kpi2Type: this.propData.assessType == 'personal' ? 'personal_kpi' : 'company_kpi',
        kpiList: [
          {
            id: data.kpi.id || undefined,
            weight: math.divide(data.kpi.percent, 100),
            kpi2TargetType: data.kpi.type,
            totalScore: data.kpi.score,
            // items: data.kpi.list
            items: deepClone(data.kpi.list).map(item => {
              item.weight = math.divide(item.weight, 100);
              return item;
            })
          }
        ]
      };
      if (this.propData.assessType == 'personal') {
        ajaxData.kpiList.push({
          id: data.ability.id || undefined,
          weight: math.divide(data.ability.percent, 100),
          kpi2TargetType: data.ability.type,
          totalScore: data.ability.score,
          // items: data.ability.list,
          items: deepClone(data.ability.list).map(item => {
            item.weight = math.divide(item.weight, 100);
            return item;
          })
        });
      }
      if (this.propData.assessType == 'company' && data.switchVal) {
        ajaxData.kpiList.push({
          id: data.temporary.id || undefined,
          weight: math.divide(data.temporary.percent, 100),
          kpi2TargetType: data.temporary.type,
          totalScore: data.temporary.score,
          items: deepClone(data.temporary.list).map(item => {
            item.weight = math.divide(item.weight, 100);
            return item;
          })
        });
      }
      return { ajaxData, data };
    },
    initDrag() {
      const tbody = this.$el.querySelector('.quarter-table-view-container tbody');
      const _this = this;
      Sortable.create(tbody, {
        handle: '.main-table-drag-handle',
        animation: 200,
        ghostClass: 'sortable-ghost',
        onEnd({ newIndex, oldIndex }) {
          const currRow = _this.formData.workPlans.splice(oldIndex, 1)[0];
          _this.formData.workPlans.splice(newIndex, 0, currRow);
          var copy = _this.formData.workPlans;
          _this.formData.workPlans = [];
          _this.$nextTick(() => {
            _this.formData.workPlans = copy;
          });
        }
      });
    },
    processImport(data) {
      if (data && data.workPlans) {
        var workPlans = data.workPlans.map((item, index) => {
          var { ultimateAchieve, phasedAchieve, appraisalMethod, weight, items, values } = item;
          return {
            ultimateAchieve,
            phasedAchieve,
            appraisalMethod,
            weight: math.multiply(weight, 100),
            high_difficulty: values?.find(v => v.workPlanTargetType === 'high_difficulty')?.remark || '',
            intermediate_difficulty: values?.find(v => v.workPlanTargetType === 'intermediate_difficulty')?.remark || '',
            easy: values?.find(v => v.workPlanTargetType === 'easy')?.remark || '',
            items: items.map((it, idx) => {
              var { doWhat, result, endTime } = it;
              return { doWhat, result, endTime };
            })
          };
        });
        this.formData.workPlans = workPlans;
      }
      this.$refs.formRef.clearValidate();
    },
    handleImport() {
      var inputEl = document.createElement('input');
      inputEl.type = 'file';
      inputEl.accept = '.xlsx,.xls';
      inputEl.onchange = _ => {
        var files = inputEl.files;
        const formData = new FormData();
        formData.append('file', files[0]);
        this.$axios.post('/web/plan/api/workPlanGroup/uploadWorkPlan',
          formData,
          (res) => {
            inputEl = undefined;
            if (res.isSuccess) {
              this.processImport(res.data);
            } else {
              this.$message.error(res.message);
            }
          },
          { headers: { 'Content-Type': 'multipart/form-data'}}
        );
      };
      inputEl.click();
    },
    downLoadTemplate() {
      const data = {
        code: 'QuarterPerfAssessTemplate'
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
            // var fil = deptDutyVos?.filter(i => i.companyVo.id == curCompanyId);
            // if (fil && fil.length > 1) {
            //   deptDutyVo = fil.find(i => i.userDutyVo.dutyType == '1') || fil[0];
            // } else {
            //   deptDutyVo = fil[0];
            // }
            deptDutyVo = deptDutyVos?.find(i => {
              return i.companyVo.id == curCompanyId;
              // return i.userDutyVo.dutyType == '1';
            });
            var { deptVo = {}, dutyVo = {}, companyVo = {}} = deptDutyVo;
            this.formData.userName = data?.name || '';
            this.formData.userId = data?.id || '';
            this.formData.companyName = companyVo?.name || '';
            this.formData.companyId = companyVo?.id || '';
            this.formData.departmentName = deptVo?.departmentName || '';
            this.formData.departmentId = deptVo?.id || '';
            this.formData.dutyName = dutyVo?.dutyName || '';
            this.formData.dutyId = dutyVo?.id || '';
          }
        });
    },
    processData2() {
      const data = deepClone(this.formData);
      data.subjectComments = [];
      const ajaxData = {
        id: data.id || undefined,
        targetTime: data.year + '' + data.quarter,
        kpiGroupStatus: data.id ? undefined : 'under_review',
        workPlans: data.workPlans.map((item, index) => {
          var { ultimateAchieve, phasedAchieve, appraisalMethod, weight, high_difficulty, intermediate_difficulty, easy, items } = item;
          return {
            planNumber: index + 1,
            ultimateAchieve,
            phasedAchieve,
            appraisalMethod,
            weight: math.divide(weight, 100),
            values: [
              { workPlanTargetType: 'high_difficulty', remark: high_difficulty || '' },
              { workPlanTargetType: 'intermediate_difficulty', remark: intermediate_difficulty || '' },
              { workPlanTargetType: 'easy', remark: easy || '' }
            ],
            items: items.map((it, idx) => {
              var { doWhat, result, endTime } = it;
              return {
                itemNumber: (index + 1) + '.' + (idx + 1),
                doWhat,
                result,
                endTime
              };
            })
          };
        })
      };
      return { ajaxData, data };
    },
    InsertFirst(scope, data) {
      console.log(scope, 'scope23');
      var { row, $index } = scope;
      data.splice($index + 1, 0, rowDefaultData());
      this.$refs.formRef.clearValidate();
    },
    addRow() {
      this.formData.workPlans.push(rowDefaultData());
      this.$refs.formRef.clearValidate();
    },
    DelFirst(scope, data) {
      if (data.length === 1) {
        this.$message.error('至少保留一条数据');
        return;
      }
      var { row, $index } = scope;
      data.splice($index, 1);
      this.$refs.formRef.clearValidate();
    },
    InsertSecond(scope, index) {
      console.log(scope, 'scope23');
      var { row } = scope;
      row.items.splice(index + 1, 0, { itemNumber: '', doWhat: '', result: '', endTime: '' });
      this.$refs.formRef.clearValidate();
    },
    DelSecond(scope, index) {
      if (scope.row.items.length === 1) {
        this.$message.error('至少保留一条计划');
        return;
      }
      var { row } = scope;
      row.items.splice(index, 1);
      this.$refs.formRef.clearValidate();
      console.log(row.items, 'row.items');
    },
    getSummaries(param) {
      const { columns, data } = param;
      var weightSum = 0;
      data.forEach(item => {
        weightSum += Number(item.weight || 0) || 0;
      });
      var sums = [];
      sums = columns.map((item, index) => {
        switch (item.property) {
          case 'weight':
            return weightSum + '%';
        }
      });
      return sums;
    }
  }
};
</script>

<style scoped lang="scss">
.plan-title {
  display: inline-block;
  width: 280px;
  text-align: left;
}
::v-deep .font-large {
  font-size: large !important;
  font-weight: 600;
  input{
    font-size: large !important;
    font-weight: 600;
  }
}
.fontWeighted {
  font-weight: 700;
}
.sortable-ghost {
  opacity: 0.6;
  background: #c8ebfb;
}
// ::v-deep .el-table .main-table-drag-handle {
//   cursor: move !important;
// }
#quarter-perf-assess {
  font-size: 14px;
}
.plan-item {
  margin-bottom: 4px;
  padding: 3px;
  background: #f5f7fa;
  border-radius: 4px;
}

.plan-id {
  font-weight: bold;
  color: #409eff;
}
::v-deep .el-input-group__prepend {
    padding: 0px 5px;
}
::v-deep .quarter-table-view-container {
  border-color: #a5a0a0 !important;
  border-right: solid 1px;

  // border-bottom: solid 1px;
  tbody td {
    /* border-color: #a5a0a0; */
    /* text-align: center; */
  }

  thead th {
    border-color: #a5a0a0 !important;
    text-align: center;
    color: #606266;
    font-size: 14px;
    font-weight: 700;
  }

  .el-table__row:hover>td {
    background-color: transparent !important;
  }

  td {
    padding: 2px;

    .cell {
      padding: 2px;
    }
  }
}

;

::v-deep .el-table tbody tr:last-child .el-table__cell {
  border-bottom: 1px solid #a5a0a0 !important;
}

::v-deep .el-table--border .el-table__cell:first-child .cell {
  padding-left: 10px !important;
  padding-right: 10px !important;
}

// .table-merge {
//   padding: 7px;
// }
/*::v-deep #mytable.el-table td.el-table__cell{
    border-color: #F5F7FA;
}
::v-deep #mytable.el-table th.el-table__cell.is-leaf{
    border-color: #F5F7FA;
} */
.mytable {
  border-collapse: collapse;
  width: 100%;
}

.mytable th,
.mytable td {
  border: 1px solid #a5a0a0;
  padding: 2px;
  text-align: center;
  line-height: 25px;
}

.mytable th {
  // background-color: #f2f2f2;
  background-color: #f3f5f7;
}

#title {
  text-align: center !important;
  margin-bottom: 5px;
}

.mytable + .mytable {
  margin-top: -1px;
}

.row-remove-plus {
  // display: inline-block;
  // display: inline-flex;
  // justify-content: space-between;
  // position: absolute;
  // inset: 0;
  // width: 25px;
  // bottom: -4px;
  // bottom: 0;
  // right: 0;
  // width: 100%;
  white-space: nowrap;
  // margin-left: -10px;

  i {
    // display: block;
    font-size: 18px;
  }

  i:hover {
    cursor: pointer;
    filter: opacity(0.75);
  }
}

.row-remove-plus2 {
  // position: absolute;
  // left: 5%;
  margin-top: -5px;
  white-space: nowrap;

  i {
    font-size: 16px;
    padding: 1px;
  }

  i:hover {
    cursor: pointer;
    filter: opacity(0.75);
  }
}
</style>
