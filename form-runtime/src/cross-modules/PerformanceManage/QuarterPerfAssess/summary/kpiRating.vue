<template>
  <div class="table-merge" id="quarter-perf-assess">
    <el-form :model="formData" ref="formRef" @submit.native.prevent="getSubmitData">
      <h3 id="title" class="font-large">
        <span class="font-large">{{ formData.title }}</span>
      </h3>
      <table class="mytable" v-if="true">
        <tr>
          <th style="width:50px">序号</th>
          <th>姓名</th>
          <th>公司</th>
          <th>职位</th>
          <th>状态</th>
          <th>分数</th>
          <th>排名</th>
          <th>评定</th>
          <th>操作</th>
        </tr>
        <tr v-for="(item, index) in formData.userList" :key="index">
          <td>{{index+1}}</td>
          <td>{{item.userName || '-'}}</td>
          <td>{{item.companyName || '-'}}</td>
          <td>{{item.dutyName || '-'}}</td>
          <td>
            <!-- {{item.kpiGroupStatus}} -->
            <span v-if="!item.id">-</span>
            <el-tag v-else :type="flowOptions[item.kpiGroupStatus].tag">{{ flowOptions[item.kpiGroupStatus].statusName }}</el-tag>
          </td>
          <td>{{item.totalScore || '-'}}</td>
          <td style="width:100px">
            <el-form-item :prop="`userList.${index}.rating`" :rules="permission.includes('rating') && item.id ? rules.isNeed : rules.isNoNeed">
              <el-input-number :disabled="!permission.includes('rating') || actionType == 'preview'" :min="0" :max="1000" v-model="item.rating" placeholder="" :controls="false" style="width:90px"/>
            </el-form-item>
          </td>
          <td style="width:100px">
            <el-form-item :prop="`userList.${index}.ratingLevel`" :rules="permission.includes('ratingLevel') && item.id ? rules.isNeed : rules.isNoNeed">
              <el-select placeholder=" " v-model="item.ratingLevel" :disabled="!permission.includes('ratingLevel') || actionType == 'preview'" clearable>
                <el-option v-for="item in [{ label: 'A', value: 'level_a' }, { label: 'B', value: 'level_b' }, { label: 'C', value: 'level_c' }, { label: 'D', value: 'level_d' }, { label: 'E', value: 'level_e' }]"
                :key="item.value" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
          </el-form-item>
          </td>
          <td style="width:100px">
            <el-button v-if="item.id" type="text" @click="handleView(item)">查看</el-button>
            <el-button v-if="item.kpiGroupStatus != 'pass' && permission.includes('deleteRow') && actionType != 'preview'"
             type="text" @click="handleDelete(index, formData.userList)">删除</el-button>
          </td>
        </tr>
        <tr v-if="formData.userList.length == 0">
          <td colspan="9">暂无数据</td>
        </tr>
      </table>
    </el-form>
  </div>
</template>

<script>
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
    title: '',
    userList: []
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
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' },
      flowOptions: {
        not_submitted: {
          tag: 'warning',
          statusName: '草稿'
        },
        under_review: {
          tag: 'primary',
          statusName: '审核中'
        },
        rejected: {
          tag: 'danger',
          statusName: '驳回'
        },
        pass: {
          tag: 'success',
          statusName: '已通过'
        },
        finish: {
          tag: 'success',
          statusName: '已通过'
        },
        undefined: {
          tag: 'danger',
          statusName: '无'
        },
        null: {
          tag: 'danger',
          statusName: '无'
        }
      },
      switchVal: false,
      kpi2TargetTypes: { kpi: 'kpi', Ability_and_attitude: 'ability', temporary_task: 'temporary' },
      math,
      publicity: { not_started: '未公示', in_progress: '公示中', end: '公示结束' },
      publicityColor: { not_started: 'danger', in_progress: 'success', end: '' },
      tableMaxHeight: window.innerHeight * 0.85, // :max-height="tableMaxHeight"
      formData: {},
      rules: {
        isNeed: [
          { required: true, message: ' ' }
        ],
        notNeed: [
          { required: false, message: ' ' }
        ],
        answer: [
          { required: true, message: '回答内容不能为空', trigger: 'blur' },
          { min: 1, max: 500, message: '长度在 1 到 500 个字符', trigger: 'blur' }
        ]
      }
    };
  },
  created() {
    this.formData = this.quarterData;
    console.log(this.propData, 'propData--新绩效考核propData');// assessType: personal/company ; scoreType: once twice
    window.abb2 = this;
    if (this.formData.otherBizId) {
      // Object.defineProperty(this.formData, 'targetTime', { get() { return this.year + '' + this.quarter; } });
      this.initData({ id: this.formData.otherBizId }, 'edit');
    } else {
      var currentKpiRatingRow = JSON.parse(sessionStorage.getItem('currentKpiRatingRow'));
      this.initData(currentKpiRatingRow, 'save');
      // this.formData.year = new Date().getFullYear() + '';
      this.$nextTick(() => {
        // this.$refs?.quarterPicker?.$el?.click();
      });
      this.getUserInfo();
    }
    console.log(this.formData, 'this.formData');
  },
  mounted() {
    this.$nextTick(() => {
      if (this.permission.includes('addDelRow') && this.actionType != 'preview') {
        // this.initDrag();
      }
    });
  },
  beforeDestroy() {
    sessionStorage.removeItem('currentKpiRatingRow');
  },
  methods: {
    handleDelete($index, data) {
      if (data.length === 1) {
        this.$message.error('至少保留一条数据');
        return;
      }
      data.splice($index, 1);
      this.$refs.formRef.clearValidate();
    },
    handleView(row, type) {
      if (!row.id) return;
      this.$fm2.show('flowDetail', {
        data: {
          // id: row.id, // 流程实例id
          flowInstanceBizRelevanceList: [{
            otherBiz: 'kpi2_appraise', // 业务类型
            otherBizId: row.id // 业务id
          }]
        }
      });
    },
    initData(row, type = 'save') {
      // const month = new Date().getMonth() + 1;
      // const quarter = Math.ceil(month / 3);
      // var targetTime = String(row.targetTime);
      // this.formData.title = row.title + `第${this.quarterEnums[targetTime ? targetTime.slice(4) : quarter]}度绩效评定`;
      if (type == 'save') {
        this.formData.title = row.title + `-绩效评定`;
        this.formData.rowId = row.id;
      }
      this.$axios.post('/web/plan/api/kpi2UserGroupSummary/findById', { data: { id: row.id } }, res => {
        if (res.isSuccess) {
          res.data.kpi2Ggroups.forEach(v => {
            v.rating = v.rating || undefined;
            v.ratingLevel = v.ratingLevel == 'none' ? '' : v.ratingLevel;
          });
          this.formData.userList = res.data.kpi2Ggroups || [];
        }
      });
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
            var { data: { kpiList = [] } } = res;
            this.formData.workPlanGroupId = res.data.workPlanGroupId;
            kpiList.forEach(v => {
              this.formData[this.kpi2TargetTypes[v.kpi2TargetType]].list = v.items;
              if (v.kpi2TargetType == 'kpi') {
                this.formData.kpi.weight = math.multiply(v.items.reduce((sum, item) => sum + (Number(item.weight) || 0), 0), 100);
                this.formData.kpi.score = v.items.reduce((sum, item) => sum + (Number(item.score) || 0), 0);
                v.items.forEach(i => {
                  i.score = i.score || undefined;
                  i.bossScore = i.bossScore || undefined;
                });
              }
            });
          }
        }
      );
    },
    getSubmitData(temporarySave) {
      return new Promise((resolve, reject) => {
        // if (this.formData.kpi.list.length == 0) {
        //   this.$message.error('无业绩指标（KPI）');
        //   reject(false);
        //   return;
        // }
        // resolve(this.processData());
        // return;
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
        id: data.rowId || undefined,
        kpiGroupStatus: data.otherBizId ? undefined : 'under_review',
        kpi2Ggroups: data.userList.filter(v => (!!v.id)).map(v => {
          return {
            id: v.id,
            rating: v.rating || null,
            ratingLevel: v.ratingLevel || 'none'
          };
        })
      };
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
              // return i.companyVo.id == curCompanyId;
              return i.userDutyVo.dutyType == '1';
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
