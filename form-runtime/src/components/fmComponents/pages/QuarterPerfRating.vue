<template>
  <div class='app' id="quarterPerfAssess-flowPage1">
      <kpiRating class="data-container" ref="quarterRef" :quarterData="quarterData" :permission="permission" :operaType="operaType" :actionType="actionType" :propData="propData"></kpiRating>
      <!-- <QuarterPrint class="print-container" :quarterData="quarterData"></QuarterPrint> -->
  </div>
</template>
<script>
import { deepClone } from '@/utils';
const kpiRating = () => import('@/views/PerformanceManage/QuarterPerfAssess/summary/kpiRating.vue');
export default {
  name: '',
  props: ['value', 'propData'],
  components: { kpiRating },
  inject: {
    flowDialog: {
      default: null
    }
  },
  data () {
    return {
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' },
      permission: [],
      quarterData: undefined,
      generateForm: null,
      actionType: 'create',
      operaType: 'add',
      flowName: ''
    };
  },
  created() {
    // var draft = document.querySelector('.save_draft_button');
    // if (draft) { draft.style.display = 'none'; }
    // var print = document.querySelector('.to_print_button');
    // if (print) { print.style.display = 'none'; }
    // var attach = document.querySelector('.el-dialog__body .flex-box.attach');
    // if (attach) { attach.style.display = 'none'; }
    // var bindFlow = document.querySelector('.el-dialog__body .custom-container');
    // if (bindFlow) { bindFlow.style.display = 'none'; }
    console.log(this.value, 'this.value 222');
    if (this.value) {
      this.quarterData = deepClone(this.value);
    }
  },
  mounted() {
    window.abb = this;
    if (this.flowDialog) {
      console.log(this.flowDialog, 'this.flowDialog--888');
      this.operaType = this.flowDialog.operaType;
      this.actionType = this.flowDialog.actionType;
      this.flowName = this.flowDialog.selectFlowName || this.flowDialog.flowName;
      this.generateForm = this.flowDialog.$refs.generateForm || this.flowDialog.$refs.OtherSteps2.$refs.generateForm;
      this.permission = this.flowDialog.enableData || this.flowDialog.$refs.OtherSteps2.enableData;
      console.log(this.permission, 'permission--新绩效考核权限');
      if (this.generateForm) {
        console.log(this.generateForm, 'this.generateForm');
        this.init();
        var val = this.generateForm.getValue('quarterPerfAssess');
        if (val && val.otherBizId) {
          this.getById(val.otherBizId);
        }
      }
    }
  },
  methods: {
    init() {
      this.generateForm.postData = (arg) => {
        if (this.value && this.value.otherBizId) { // if (arg.init) {
          return this.submitForm(arg, 'update');
        } else {
          return this.submitForm(arg, 'save');
        }
      };
    },
    submitForm(arg, type) {
      return new Promise((resolve, reject) => {
        (async _ => {
          console.log(arg, 'arggggg');
          var { ajaxData, data } = await this.$refs.quarterRef.getSubmitData(arg.temporary || arg.clickMethod === 'draft');
          console.log(data, 'data');
          console.log(ajaxData, 'ajaxData');
          // reject('test');
          // return;
          if (arg.clickMethod == 'draft') {
            ajaxData.kpiGroupStatus = 'not_submitted';
          }
          this.$axios.post(`/web/plan/api/kpi2UserGroupSummary/update`, { data: ajaxData, batchCode: arg.param.batchCode },
            res => {
              if (res.isSuccess) {
                if (arg.clickMethod == 'draft') {
                  this.$bus.$emit('kpiRatingSaveDraftSuccess');
                  return;
                }
                arg.param.formDataMongoVo.data.quarterPerfAssess = data;
                arg.param.data.name = data.title;
                if (type === 'save') {
                  arg.param.formDataMongoVo.data.userId = data.userId;
                  arg.param.formDataMongoVo.data.userName = data.userName;
                  arg.param.formDataMongoVo.data.userId__formPersonId = data.userId;
                  arg.param.formDataMongoVo.data.quarterPerfAssess.otherBizId = data.rowId;
                  arg.param.data.flowInstanceBizRelevanceList.push({ otherBizId: data.rowId, otherBiz: 'kpi2_rating_level_evaluate' });
                };
                resolve(data);
              } else {
                reject(new Error(res.message));
              }
            });
        })();
      });
    },
    getById(id) {
      return;
      this.$axios.post('/web/plan/api/workPlanGroup/findById', { data: { id }}, res => {
        if (res.isSuccess) {
          this.$set(this.quarterData, 'noticeStatus', res.data.noticeStatus);
          this.$set(this.quarterData, 'subjectComments', res.data.publishedSubject?.subjectComments || []);
        }
      });
      var vals = this.generateForm.getValues();
      var leader = vals.auto_audit_info_leaderConfirmTime?.trim() || '-';
      var myself = vals.auto_audit_info_myselfConfirmTime?.trim() || '-';
      this.$set(this.quarterData, 'leaderConfirmTime', leader);
      this.$set(this.quarterData, 'myselfConfirmTime', myself);
    }
  },
  computed: {},
  watch: {}
};
</script>
<style lang='scss' scoped>
.print-container {
  display: none;
}
@media print {
  // #quarterPerfAssess-flowPage1 {
  //   width: 1300px !important;
  // }
  // ::v-deep {
  //   #my-main-table {
  //     .el-table__header-wrapper table {
  //       width: 100% !important;
  //     }
  //     .el-table__body-wrapper table {
  //       width: 100% !important;
  //     }
  //     .el-table__footer-wrapper table {
  //       width: 100% !important;
  //     }
  //   }
  // }
  .data-container {
    // display: none;
  }
  .print-container {
    // display: block;
  }
}
.app{

}

</style>
<style lang="scss">
#quarterPerfAssess-flowPage1 {
    text-align: initial !important;
    .div,input,textarea,h2,.el-form-item__content,.el-form-item__label{
        text-align: initial !important;
    }
     .el-textarea__inner, input {
      font-size: 14px;
    }
}
</style>
