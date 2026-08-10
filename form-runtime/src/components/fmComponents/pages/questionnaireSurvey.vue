<template>
  <div class='app' id="questionnaire-survey1">
    <fill :questionData="questionData" :isPreview="true"></fill>
  </div>
</template>

<script>
const fill = () => import('@/views/QuestionnaireManage/components/fill.vue');
export default {
  name: '',
  props: ['value'],
  components: { fill },
  // inject: ['QuestionnaireManageMyRef', 'flowDialog'],
  inject: {
    QuestionnaireManageMyRef: {
      default: null
    },
    flowDialog: {
      default: null
    }
  },
  data () {
    return {
      questionData: null,
      generateForm: null,
      myRef: null
    };
  },
  created() {
    this.myRef = this.QuestionnaireManageMyRef;
    console.log(this.myRef, 'myRef');
    if (this.myRef) {
      this.questionData = { ...this.myRef.step1Data, ...this.myRef.step2Data };
      console.log(this.questionData, 'this.questionData');
    } else {
      console.log(this.value, 'this.value');
      this.getById(this.value.otherBizId);
    }
    // if (myRef) {
    //   if (myRef.editRow) {
    //     // 编辑
    //     console.log(myRef.editRow, 'myRef.editRow');
    //   } else {
    //     // 新建
    //   }
    // } else {
    //   // 审批
    // }
  },
  mounted() {
    window.abb = this;
    var draft = document.querySelector('.save_draft_button');
    if (draft) { draft.style.display = 'none'; }
    var print = document.querySelector('.to_print_button');
    if (print) { print.style.display = 'none'; }
    var attach = document.querySelector('.el-dialog__body .flex-box.attach');
    if (attach) { attach.style.display = 'none'; }
    var bindFlow = document.querySelector('.el-dialog__body .custom-container');
    if (bindFlow) { bindFlow.style.display = 'none'; }
    if (this.flowDialog) {
      this.generateForm = this.flowDialog.$refs.generateForm || this.flowDialog.$refs.OtherSteps2.$refs.generateForm;
      if (this.generateForm) {
        console.log(this.generateForm, 'this.generateForm');
        this.init();
        // var val = this.generateForm.getValue('questionnaireSurvey');
        // if (val && val.otherBizId) {
        //   this.getById(val.otherBizId);
        // }
      }
    }
  },
  methods: {
    init() {
      var t = this.generateForm;
      t.postData = (arg) => {
        if (arg.init) {
          return this.submitForm(arg);
        }
      };
    },
    submitForm(arg) {
      return new Promise((resolve, reject) => {
        (async _ => {
          if (!this.myRef) {
            reject('请在问卷调查页面发起该流程');
            return;
          }
          const res = await this.myRef.saveSurvey({ status: '1', examineStatus: '0' }, { batchCode: arg.param.batchCode });
          if (res.isSuccess) {
            var row = this.myRef.editRow;
            var otherBizId = (row && !row.isCopy) ? row.id : res.data.id;
            arg.param.formDataMongoVo.data.questionnaireSurvey = { otherBizId };
            arg.param.data.flowInstanceBizRelevanceList.push({ otherBizId, otherBiz: 'questionnaire_investigate' });
            arg.param.data.name = this.myRef.step1Data.questionnaireTitle + '-' + this.$store.state.user.userName;
            resolve(res.data);
          } else {
            reject(new Error(res.message));
          }
        })();
      });
    },
    getById(id) {
      this.$axios.post('/web/warehouse/api/questionnaireSurvey/findById', { data: { id }}, res => {
        if (res.isSuccess) {
          this.questionData = res.data;
          console.log(this.questionData, 'this.questionData');
        }
      });
    }
  },
  computed: {},
  watch: {}
};
</script>
<style lang='scss' scoped>
.app{

}

</style>
<style lang="scss">
[data-id="questionnaireSurvey"] {
    // text-align: initial !important;
    // #questionnaire-survey {
    //     text-align: initial !important;
    // }
    // .title {
    //     text-align: initial !important;
    // }
    // .el-form-item__label {
    //     text-align: initial !important;
    // }
    // .el-form-item__content {
    //     text-align: initial !important;
    // }
    // input {
    //     text-align: initial !important;
    // }
    // textarea {
    //     text-align: initial !important;
    // }
}
#questionnaire-survey1 {
    text-align: initial !important;
    .div,input,textarea,h2,.el-form-item__content,.el-form-item__label{
        text-align: initial !important;
    }
}
</style>
