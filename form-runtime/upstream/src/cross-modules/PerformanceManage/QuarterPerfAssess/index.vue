<template>
  <div class="table-merge" id="quarter-perf-assess">
    <el-form :model="formData" ref="formRef" @submit.native.prevent="getSubmitData">
      <h3 id="title" class="font-large">
        <el-form-item prop="year" :rules="permission.includes('year') ? rules.isNeed : rules.isNoNeed" style="display:inline-block;" class="font-large">
          <el-date-picker type="year" value-format="yyyy" v-model="formData.year" :clearable="false" style="width:60px;" class="font-large"
          :disabled="!permission.includes('year') || actionType == 'preview'"></el-date-picker><span class="font-large">年</span>
        </el-form-item>
        <el-form-item prop="quarter" :rules="permission.includes('quarter') ? rules.isNeed : rules.isNoNeed" style="display:inline-block;" class="font-large">
          <el-select ref="quarterPicker" style="width:50px;" placeholder=" " v-model="formData.quarter" :disabled="!permission.includes('quarter') || actionType == 'preview'" class="font-large">
            <el-option
              v-for="item in [{ label: '一', value: 1 }, { label: '二', value: 2 }, { label: '三', value: 3 }, { label: '四', value: 4 }]"
              :key="item.value" :label="item.label" :value="item.value">
            </el-option>
          </el-select><span class="font-large">季度目标计划表</span>
        </el-form-item>
      </h3>
      <table class="mytable header-table">
        <tr>
          <td style="width:100px;" class="fontWeighted">姓名</td>
          <td>
            <!-- <el-form-item prop="userName" :rules="permission.includes('user') ? rules.isNeed : rules.isNoNeed">
              <el-input v-model="formData.userName" :disabled="!permission.includes('user')"></el-input>
            </el-form-item> -->
            <div>{{ formData.userName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">公司</td>
          <td>
            <div>{{ formData.companyName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">部门</td>
          <td>
            <div>{{ formData.departmentName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">岗位</td>
          <td>
            <div>{{ formData.dutyName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">公示状态</td>
          <td>
            <el-tag :type="publicityColor[formData.noticeStatus || '']" effect="dark" size="medium"> {{ publicity[formData.noticeStatus || ''] || '-'}} </el-tag>
          </td>
        </tr>
        <tr>
          <td colspan="12">
            <div style="text-align:left;margin:3px">
              <el-button type="primary" size="small" @click="addRow" :disabled="!permission.includes('addRow') || actionType == 'preview'">添加行</el-button>
              <el-button type="primary" plain size="small" :disabled="!permission.includes('importData') || actionType == 'preview'" @click="handleImport">导入数据</el-button>
              <el-link type="primary" style="margin-left:10px;" :disabled="!permission.includes('downloadExcel') || actionType == 'preview'" @click="downLoadTemplate">下载导入模板</el-link>
            </div>
          </td>
        </tr>
      </table>
      <el-table id="my-main-table" :showSummary="true" :summaryMethod="getSummaries" :data="formData.workPlans"
        class="quarter-table-view-container" border style="width: 100%;">
        <el-table-column label="编号" width="40" align="center" style="position: relative;" class-name="main-table-drag-handle">
          <template slot-scope="scope">
            <div> {{ scope.$index + 1 }}</div>
            <div class="row-remove-plus" v-if="permission.includes('addDelRow') && actionType != 'preview'">
              <i class="el-icon-remove" title="删除行" style="color:#f56c6c" @click="DelFirst(scope, formData.workPlans)"></i>
              <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="InsertFirst(scope, formData.workPlans)"></i>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="年度目标及完成进度" align="center">
          <template slot-scope="scope">
            <el-form-item :prop="`workPlans.${scope.$index}.ultimateAchieve`" :rules="permission.includes('ultimateAchieve') ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="scope.row.ultimateAchieve" placeholder="年度目标及完成进度" :autosize="{ minRows: 3, maxRows: 20 }"
               :disabled="!permission.includes('ultimateAchieve') || actionType == 'preview'"></el-input>
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="目标" align="center">
          <template slot-scope="scope">
            <el-form-item :prop="`workPlans.${scope.$index}.phasedAchieve`" :rules="permission.includes('phasedAchieve') ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="scope.row.phasedAchieve" placeholder="目标" :autosize="{ minRows: 3, maxRows: 20 }"
               :disabled="!permission.includes('phasedAchieve') || actionType == 'preview'"></el-input>
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="目标界定、定义等" align="center">
          <template slot-scope="scope">
            <!-- <el-tooltip effect="light" :hide-after="10*1000">
          <div slot="content">
            <pre>{{ '目标界定、定义等 \r\n jsdiofsjdof' }}{{ text1 }}</pre>
          </div>
          <el-button>Top center</el-button>
        </el-tooltip> -->
            <el-form-item :prop="`workPlans.${scope.$index}.appraisalMethod`" :rules="permission.includes('appraisalMethod') ? rules.isNeed : rules.isNoNeed">
              <el-input type="textarea" v-model="scope.row.appraisalMethod" placeholder="目标界定、定义等" :autosize="{ minRows: 3, maxRows: 20 }"
              :disabled="!permission.includes('appraisalMethod') || actionType == 'preview'"></el-input>
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="权重" width="60" align="center" prop="weight">
          <template slot-scope="scope">
            <el-form-item :prop="`workPlans.${scope.$index}.weight`" :rules="permission.includes('weight') ? rules.isNeed : rules.isNoNeed">
              <el-input-number v-model="scope.row.weight" placeholder="权重" :controls="false" style="width:100%" :min="0" :max="100"
              :disabled="!permission.includes('weight') || actionType == 'preview'"></el-input-number>
            </el-form-item>
          </template>
        </el-table-column>
        <el-table-column label="目标值" align="center">
          <template slot="header" slot-scope="scope">
            <div>目标值</div>
            <div style="white-space:nowrap;">挑战值 | 达标值 | 底限值</div>
          </template>
          <template slot-scope="scope">
            <el-form-item :prop="`workPlans.${scope.$index}.high_difficulty`" :rules="permission.includes('targetValue') ? rules.isNeed : rules.isNoNeed">
              <!-- <el-input v-model="scope.row.high_difficulty" :disabled="!permission.includes('targetValue')">
                <template slot="prepend">挑战值:</template>
              </el-input> -->
              <el-tooltip effect="light" content="挑战值" placement="top">
                <el-input v-model="scope.row.high_difficulty" :disabled="!permission.includes('targetValue') || actionType == 'preview'"
                 type="textarea" placeholder="挑战值" :rows="1" :autosize="{ minRows: 1, maxRows: 4 }"></el-input>
              </el-tooltip>
            </el-form-item>
            <el-form-item :prop="`workPlans.${scope.$index}.intermediate_difficulty`" :rules="permission.includes('targetValue') ? rules.isNeed : rules.isNoNeed">
              <!-- <el-input v-model="scope.row.intermediate_difficulty" :disabled="!permission.includes('targetValue')">
                <template slot="prepend">达标值:</template>
              </el-input> -->
              <el-tooltip effect="light" content="达标值" placement="top">
                <el-input v-model="scope.row.intermediate_difficulty" :disabled="!permission.includes('targetValue') || actionType == 'preview'"
                 type="textarea" placeholder="达标值" :rows="1" :autosize="{ minRows: 1, maxRows: 4 }"></el-input>
              </el-tooltip>
            </el-form-item>
            <el-form-item :prop="`workPlans.${scope.$index}.easy`" :rules="permission.includes('targetValue') ? rules.isNeed : rules.isNoNeed">
              <!-- <el-input v-model="scope.row.easy" :disabled="!permission.includes('targetValue')">
                <template slot="prepend">底限值:</template>
              </el-input> -->
              <el-tooltip effect="light" content="底限值" placement="top">
                <el-input v-model="scope.row.easy" :disabled="!permission.includes('targetValue') || actionType == 'preview'"
                 type="textarea" placeholder="底限值" :rows="1" :autosize="{ minRows: 1, maxRows: 4 }"></el-input>
              </el-tooltip>
            </el-form-item>
            <!-- <div class="target-item"></div> -->
          </template>
        </el-table-column>
        <el-table-column label="达成目标核心计划" align="center" min-width=230>
          <template slot="header" slot-scope="scope">
            <div>达成目标核心计划</div>
            <div>
              做什么&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;达成什么结果&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;完成日期
            </div>
          </template>
          <template slot-scope="scope">
            <!-- <div style="width:100%;" v-for="(plan, index) in scope.row.items" :key="index">
              <span style="display:inline-block;width: 33.33%;">{{ scope.$index + 1 }}.{{ index + 1 }}<span style="display:inline-block;width:5px;color:#47a1fb"></span>{{ plan.doWhat }}</span>
              <span style="display:inline-block;width: 33.33%;">{{ plan.result }}</span>
              <span style="display:inline-block;width: 33.33%;">{{ plan.endTime }}</span>
            </div> -->
            <div v-for="(plan, index) in scope.row.items" :key="index" class="plan-item">
              <el-row :gutter="5">
                <el-col :span="2">
                  <span class="plan-id">{{ scope.$index + 1 }}.{{ index + 1 }}</span>
                  <div class="row-remove-plus2" v-if="permission.includes('addDelPlan') && actionType != 'preview'">
                    <i class="el-icon-remove" title="删除行" style="color:#f56c6c" @click="DelSecond(scope, index)"></i>
                    <i class="el-icon-circle-plus" title="插入行" style="color:#47a1fb" @click="InsertSecond(scope, index)"></i>
                  </div>
                </el-col>
                <el-col :span="22">
                  <el-row :gutter="3">
                    <el-col :span="9" title="做什么">
                      <el-form-item :prop="`workPlans.${scope.$index}.items.${index}.doWhat`" :rules="permission.includes('planItems') ? rules.isNeed : rules.isNoNeed">
                        <el-input type="textarea" v-model="plan.doWhat" placeholder="做什么" :rows="1" :autosize="{ minRows: 1, maxRows: 5 }" size="mini"
                        :disabled="!permission.includes('planItems') || actionType == 'preview'"></el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :span="9" title="达成结果">
                      <el-form-item :prop="`workPlans.${scope.$index}.items.${index}.result`" :rules="permission.includes('planItems') ? rules.isNeed : rules.isNoNeed">
                        <el-input type="textarea" v-model="plan.result" placeholder="达成结果" :rows="1" :autosize="{ minRows: 1, maxRows: 5 }" size="mini"
                        :disabled="!permission.includes('planItems') || actionType == 'preview'"></el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :span="6" title="完成日期">
                      <el-form-item :prop="`workPlans.${scope.$index}.items.${index}.endTime`" :rules="permission.includes('planItems') ? rules.isNeed : rules.isNoNeed">
                        <el-input type="textarea" v-model="plan.endTime" placeholder="完成日期" :rows="1" :autosize="{ minRows: 1, maxRows: 5 }" size="mini"
                        :disabled="!permission.includes('planItems') || actionType == 'preview'"></el-input>
                      </el-form-item>
                    </el-col>
                  </el-row>
                </el-col>
              </el-row>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <!-- <div style="height:1px;background-color:#a5a0a0" :class="{ 'footer-table': (formData.workPlans && formData.workPlans.length > 0) ? true : false }"></div> -->
      <table class="mytable" :class="{ 'footer-table': (formData.workPlans && formData.workPlans.length > 0) ? true : false }">
        <tr>
          <td style="width:25%;" class="fontWeighted">上级签字</td>
          <td>{{ formData.leaderConfirmTime || '-' }}</td>
        </tr>
        <tr>
          <td style="width:25%;" class="fontWeighted">本人签字</td>
          <td>{{ formData.myselfConfirmTime || '-' }}</td>
        </tr>
      </table>
    </el-form>
    <Comment v-if="quarterData.id && quarterData.noticeStatus !== 'not_started'" :quarterData="quarterData"></Comment>
  </div>
</template>

<script>
import Comment from './components/comment.vue';
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
var quarterDefaultData = _ => {
  return {
    year: '',
    quarter: '',
    get targetTime() {
      return this.year + '' + this.quarter;
    },
    userName: '',
    userId: '',
    companyId: '',
    companyName: '',
    departmentName: '',
    departmentId: '',
    dutyName: '',
    dutyId: '',
    kpiGroupStatus: 'under_review',
    noticeStatus: 'not_started',
    leaderConfirmTime: '',
    myselfConfirmTime: '',
    subjectComments: [],
    workPlans: [rowDefaultData()]
  };
};
export default {
  name: 'QuarterPerfAssess',
  components: { Comment },
  props: {
    operaType: { // 新增：add，编辑：edit reEdit，查看：check
      type: String,
      default: ''
    },
    actionType: { // preview edit examine  create
      type: String,
      default: ''
    },
    permission: {
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
    }
  },
  data() {
    return {
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
    window.abb2 = this;
    this.formData = this.quarterData;
    if (this.formData.otherBizId) {
      Object.defineProperty(this.formData, 'targetTime', { get() { return this.year + '' + this.quarter; } });
    } else {
      this.formData.year = new Date().getFullYear() + '';
      this.$nextTick(() => {
        this.$refs?.quarterPicker?.$el?.click();
      });
      this.getUserInfo();
    }
    console.log(this.formData, 'this.formData');
  },
  mounted() {
    this.$nextTick(() => {
      if (this.permission.includes('addDelRow') && this.actionType != 'preview') {
        this.initDrag();
      }
    });
  },
  methods: {
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
    getSubmitData(temporarySave) {
      return new Promise((resolve, reject) => {
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
        weightSum = math.add(weightSum, Number(item.weight || 0) || 0);
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
.quarter-table-view-container ::v-deep .el-table__footer .cell {
  font-size: 14px;
  font-weight: bold;
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
  background-color: #f2f2f2;
}

#title {
  text-align: center !important;
  margin-bottom: 5px;
}

.header-table {
  margin-bottom: -1px;
}

.footer-table {
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
  margin-left: -10px;

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
