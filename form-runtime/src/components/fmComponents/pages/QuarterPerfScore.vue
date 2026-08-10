<template>
  <div class='app' id="quarterPerfAssess-flowPage1">
      <Score class="data-container" ref="quarterRef" :quarterData="quarterData" :perm="permission" :operaType="operaType" :actionType="actionType" :propData="propData" :isReEditInitiator="isReEditInitiator"></Score>
      <!-- <QuarterPrint class="print-container" :quarterData="quarterData"></QuarterPrint> -->
  </div>
</template>
<script>
import { deepClone } from '@/utils';
import { localstorageGet } from '@/utils/auth';
const Score = () => import('@/views/PerformanceManage/QuarterPerfAssess/Score/index.vue');
// const QuarterPrint = () => import('@/views/PerformanceManage/QuarterPerfAssess/components/quarterPrint.vue');
export default {
  name: '',
  props: ['value', 'propData'],
  components: { Score },
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
      flowName: '',
      isReEditInitiator: false // 撤回重新发起时，当前登录人是否为发起人
    };
  },
  created() {
    // var draft = document.querySelector('.save_draft_button');
    // if (draft) { draft.style.display = 'none'; }
    var print = document.querySelector('.to_print_button');
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
          console.log('%c [ this.flowDialog ]-49', 'font-size:13px; background:pink; color:#bf2c9f;', this.flowDialog)
    if (this.flowDialog) {

      console.log(this.flowDialog, 'this.flowDialog--888');
      this.operaType = this.flowDialog.operaType;
      this.actionType = this.flowDialog.actionType;
      this.flowName = this.flowDialog.selectFlowName || this.flowDialog.flowName;
      this.generateForm = this.flowDialog.$refs.generateForm || this.flowDialog.$refs.OtherSteps2.$refs.generateForm;
      this.permission = this.flowDialog.enableData || this.flowDialog.$refs.OtherSteps2.enableData;
      if (this.flowDialog.currentPendingNodeName && this.flowDialog.currentPendingNodeName === '发起人绩效考核组组长'){
        this.permission.push('kpi_score', 'kpi_bossScore', 'tpi_score', 'tpi_bossScore');
      }
      console.log(this.permission, 'permission--新绩效考核权限');
      if (this.generateForm) {
        console.log(this.generateForm, 'this.generateForm');
        this.init();
        var val = this.generateForm.getValue('quarterPerfAssess');
        if (val && val.otherBizId) {
          this.getById(val.otherBizId);
          if (this.flowDialog.isReInitiate && !this.flowDialog.isExamine && this.flowDialog.flowStatus == 'withdraw') {
            console.log('------临时清空分数-----');
            this.$refs.quarterRef.resetScore();
          }
        }
      }
      // 撤回后重新发起时，判断当前登录人是否为发起人
      if (this.operaType === 'reEdit') {
        const checkFlowNode = this.flowDialog.$data?.checkFlowNode || [];
        const initiatorNode = checkFlowNode.find(item => item.auditNodeName === '发起人');
        const currentUserName = localstorageGet('userName');
        this.isReEditInitiator = !!(initiatorNode && initiatorNode.executorName === currentUserName);
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
          var { ajaxData, data } = await this.$refs.quarterRef.getSubmitData(arg.temporary);
          console.log(data, 'data');
          console.log(ajaxData, 'ajaxData');
          if (arg.clickMethod == 'draft') {
            ajaxData.kpiGroupStatus = 'not_submitted';
          }
          // reject('test');
          // return;
          // ajaxData.targetTime = '10011';
          this.$axios.post(`/web/plan/api/kpi2Group/${type}`, { data: ajaxData, batchCode: arg.param.batchCode },
            res => {
              if (res.isSuccess) {
                arg.param.formDataMongoVo.data.quarterPerfAssess = data;
                arg.param.data.name = `${data.year}年${this.quarterEnums[data.quarter]}季度绩效考核表-${data.userName}`;
                if (type === 'save') {
                  arg.param.formDataMongoVo.data.userId = data.userId;
                  arg.param.formDataMongoVo.data.userName = data.userName;
                  arg.param.formDataMongoVo.data.userId__formPersonId = data.userId;
                  arg.param.formDataMongoVo.data.quarterPerfAssess.otherBizId = res.data.id;
                  arg.param.formDataMongoVo.data.quarterPerfAssess.id = res.data.id;
                  arg.param.data.flowInstanceBizRelevanceList.push({ otherBizId: res.data.id, otherBiz: 'kpi2_appraise' });
                };
                arg.param.formDataMongoVo.data.allScore = data.allScore;
                // reject('test');
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
      this.$axios.post('/web/plan/api/kpi2Group/findById', { data: { id }}, res => {
        if (res.isSuccess) {
          // res.data.kpiList[0].items[0].score = 3;
          // res.data.kpiList[0].items[1].score = null;
          // this.$refs.quarterRef.setTargetsData(res);
        }
      });
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
