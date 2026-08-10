<template>
  <div class="questionnaires-container">
    <!-- 头部信息区域 -->
    <div class="header-section">
      <h2 class="title">{{ questions.questionnaireTitle }}</h2> <!-- -{{ (questions.fullUserInfoVo && questions.fullUserInfoVo.name) || $store.state.user.userName }} -->
      <el-row>
        <el-col :span="7">
          <div class="info-item">
            <span class="label">版块类型：</span>
            <span class="value">{{ questions.plateTypeVo && questions.plateTypeVo.plateTypeName }}</span>
          </div>
        </el-col>
        <el-col :span="7">
          <div class="info-item">
            <span class="label">发起部门：</span>
            <span class="value">{{ questions.depName }}</span>
          </div>
        </el-col>
        <el-col :span="10">
          <div class="info-item">
            <span class="label">调査期限：</span>
            <span class="value">{{ questions.startTime }} ~ {{ questions.endTime }}</span>
          </div>
        </el-col>
      </el-row>
      <el-row v-if="!editRow">
        <el-col :span="25">
          <div class="info-item">
            <span class="label">发布范围：</span>
            <span class="value">{{ questions.releaseRangeVo ? questions.releaseRangeVo.selectContent : ''}}</span>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 投票设置区域 -->
    <div class="vote-settings" v-show="false">
      <div class="settings-item">
        <span class="label">投票方式：</span>
        <el-radio-group v-model="questions.voteWay">
          <el-radio label="1">实名</el-radio>
          <el-radio label="0">匿名</el-radio>
        </el-radio-group>
      </div>
      <!-- <div class="settings-item2" style="margin-left:0">
        <el-checkbox v-model="showVoted">发起人版块管理员可查看已投票人</el-checkbox>
      </div> -->
      <div class="settings-item2" style="margin-left:0">
        <el-checkbox v-model="questions.viewBefore">允许提交前查看调查结果</el-checkbox>
      </div>
      <div class="settings-item2">
        <el-checkbox v-model="questions.viewAfter">允许提交后查看调查结果</el-checkbox>
      </div>
      <div class="fill-mask"></div>
    </div>

    <!-- 问卷主体 -->
    <div class="questionnaire-body" style="position: relative;">
      <!-- <p class="description">感谢您能抽出几分钟时间来参加本次答题，现在我们就马上开始吧！</p> -->
      <!-- <p class="description">感谢您积极参与集团培训！请您对本次培训进行效果反馈，您的反馈对我们提升工作质量至关重要。感谢您对我们工作支持!</p> -->
      <p class="description">{{ questions.remarks || ' ' }}</p>
      <el-form :model="questions" label-position="top" ref="formRef" class="question-form"><!-- :rules="rules" -->
        <div v-for="(topic, index) in questions.topicVoList" :key="index">
          <el-form-item :label="index + 1 + '、'+ topic.topicDescribe" :prop="`topicVoList.${index}.${(topic.topicType == 'radio' || topic.topicType == 'checkbox') ? 'optionId' : 'otherContent'}`"
          :rules="rules.validateVal">
            <el-radio-group v-if="topic.topicType == 'radio'" v-model="topic.optionId" style="width:100%;" @input="radioChange" :disabled="editDisabled"><!-- :rules="rules.aaa"  getRules(topic)-->
              <div v-for="(option, index) in topic.optionVoList" :key="index">
                <el-radio :label="option.id">
                  <span style="white-space:pre-line;">{{ option.optionContent }}</span>
                  <el-image v-if="option.urls" class="image-item" :src="option.urls" :preview-src-list="[option.urls]"></el-image>
                </el-radio>
                <br/>
                <el-input v-if="(option.type == '1' && option.id == topic.optionId) || (option.type == '1' && isPreview)" v-model="option.otherContent" type="textarea" :rows="2" style="margin-top:3px" class="customInput" :disabled="editDisabled"></el-input>
                <div style="height:10px;"></div>
              </div>
            </el-radio-group>
            <el-checkbox-group v-else-if="topic.topicType == 'checkbox'" v-model="topic.optionId" style="width:100%;"  :disabled="editDisabled">
              <div v-for="(option, index) in topic.optionVoList" :key="index">
                <el-checkbox :label="option.id">
                  <span style="white-space:pre-line;">{{ option.optionContent }}</span>
                  <el-image v-if="option.urls" class="image-item" :src="option.urls" :preview-src-list="[option.urls]"></el-image>
                </el-checkbox>
                <br/>
                <el-input v-if="(option.type == '1' && topic.optionId.includes(option.id)) || (option.type == '1' && isPreview)" v-model="option.otherContent" type="textarea" :rows="2" style="margin-top:-5px" class="customInput" :disabled="editDisabled"></el-input>
                <div style="height:0px;"></div>
              </div>
            </el-checkbox-group>
            <el-input v-else-if="topic.topicType == 'input'" type="text" placeholder="请输入内容" v-model="topic.otherContent" :disabled="editDisabled"></el-input>
            <el-input v-else-if="topic.topicType == 'textarea'" type="textarea" :rows="4" placeholder="请输入内容" v-model="topic.otherContent" :disabled="editDisabled"></el-input>
            <div style="height:10px;"></div>
          </el-form-item>
        </div>
      </el-form>
      <!-- <div class="fill-mask" v-if="isPreview"></div> -->
    </div>
  </div>
</template>

<script>
import { deepClone } from '@/utils';
export default {
  props: ['questionData', 'isPreview', 'editRow'],
  data() {
    return {
      rules: {
        validateVal: [
          { required: true, message: '回答不能为空', trigger: 'change' },
          { validator: this.validatePass, trigger: 'change' },
          // { min: 2, max: 50, message: '长度在 2 到 50 个字符', trigger: 'blur' }
        ],
      },
      questions: {},
      url: 'http://192.168.1.220/files/54e991e06b9b4f55b1747ee71fd0d79a/200001/2024-11-15/e69c68ecb98a43daac7221509ddb46b8.png',
      voteType: '实名',
      showVoted: true,
      showBeforeSubmit: false,
      showAfterSubmit: true,
      singleChoice: '',
      multipleChoice: [],
      textAnswer: ''
    };
  },
  watch: {
    questionData: {
      handler(current, old) {
        console.log(current, old, 'questionData发生变化');
        this.setQuestions(current);
      },
      immediate: true,
      // deep: true
    }
  },
  computed: {
    editDisabled() {
      return this.editRow?.answer != '0' || this.questionData.status != '2';
    }
  },
  methods: {
    radioChange(val) {
      console.log(val, 'radioChange-val');
    },
    getRules(item) {
      // console.log(item, 'item')
      return [
        { required: true, message: '回答不能为空', trigger: 'blur' },
        {
          validator: (rule, value, callback) => {
            if (item.topicType == 'radio' || item.topicType == 'checkbox') {
              var ids = typeof(item.optionId) == 'string' ? [item.optionId] : item.optionId;
              var ops = item.optionVoList.filter(op => ids?.includes(op.id));
              var checkState = ids.some(i => {
                var fi = ops.find(op => op.id == i);
                if (fi && fi.type == '1' && !fi.otherContent?.trim()) {
                  return true;
                } else {
                  return false;
                }
              });
              // console.log(checkState, item.topicType, value, 'checkState');
              if (checkState) {
                callback(new Error('选项填空内容不能为空'));
              } else {
                callback();
              }
            }
          },
          trigger: 'blur'
        }
      ];
    },
    validatePass(rule, value, callback) {
      // console.log(rule, value, callback, 'rule, value, callback')
      var index = rule.field.split('.')?.[1];
      console.log(index, 'index');
      var topic = this.questions.topicVoList[index];
      if (topic.topicType == 'radio' || topic.topicType == 'checkbox') {
        var ids = (typeof topic.optionId == 'string') ? [topic.optionId] : topic.optionId;
        var ops = topic.optionVoList.filter(op => ids.includes(op.id));
        var checkState = ids.some(i => {
          var fi = ops.find(op => op.id == i);
          if (fi && fi.type == '1' && !fi.otherContent?.trim()) {
            return true;
          } else {
            return false;
          }
        });
        // console.log(checkState, topic.topicType, value, 'checkState');
        if (checkState) {
          callback(new Error('选项填空内容不能为空'));
        } else {
          callback();
        }
      } else {
        callback();
      }
    },
    submitForm() {
      this.$refs.formRef.validate((valid) => {
        if (valid) {
          alert('提交成功');
        } else {
          alert('表单校验失败');
        }
      });
    },
    getSaveFillOutData() {
      return new Promise((resolve, reject) => {
        this.$refs.formRef.validate(valid => {
          if (valid) {
            var param = this.getFillOutData();
            console.log(param, 'param');
            resolve(param);
          } else {
            reject(new Error('校验失败'));
            return false;
          }
        });
      });
    },
    getFillOutData() {
      var qs = deepClone(this.questions);
      console.log(qs, 'qs')
      var param = {
        data: {
          // id: '数据id',
          questionnaireId: qs.id,
          answerOptionVoList: [
            // {
            //   topicId: '题目id',
            //   optionId: '选项id',
            //   otherContent: ' 填写内容'
            // }
          ]
        }
      };
      var arr = param.data.answerOptionVoList;
      qs.topicVoList.forEach(i => {
        if (i.topicType == 'radio') {
          var findOp = i.optionVoList.find(op => op.id == i.optionId);
          arr.push({ topicId: i.topicId, optionId: i.optionId, otherContent: findOp?.otherContent || '', writeContent: findOp?.optionContent || '' });
        } else if (i.topicType == 'checkbox') {
          i.optionId.forEach(optId => {
            var findOpt = i.optionVoList.find(op => op.id == optId);
            arr.push({ topicId: i.topicId, optionId: optId, otherContent: findOpt?.otherContent || '', writeContent: findOpt?.optionContent || '' });
          });
        } else if (i.topicType == 'input' || i.topicType == 'textarea') {
          arr.push({ topicId: i.topicId, optionId: i.topicId, otherContent: i.otherContent, writeContent: i.topicDescribe });
        }
      });
      return param;
    },
    setQuestions(current) {
      if (!current) return;
      var data = deepClone(current);
      data = { ...data, ...this.setViewType(data.viewResult) };
      data.topicVoList && data.topicVoList.forEach(i => {
        i.topicId = i.id;
        i.optionId = '';
        i.otherContent = '';
        if (i.topicType == 'checkbox') {
          i.optionId = [];
          i.optionVoList && i.optionVoList.filter((op1, index1) => {
            op1.id = op1.id || index1;
            return op1.answerOptionVoList && op1.answerOptionVoList.length > 0;
          }).forEach(op2 => {
            op2.otherContent = op2.answerOptionVoList[0].otherContent || '';
            i.optionId.push(op2.id);
          });
        } else {
          i.optionVoList && i.optionVoList.find((op, index) => {
            op.id = op.id || index;
            if (op.answerOptionVoList && op.answerOptionVoList.length > 0) {
              i.optionId = op.answerOptionVoList[0].optionId || '';
              i.otherContent = op.otherContent = op.answerOptionVoList[0].otherContent || '';
              return true;
            }
          });
        }
        // i.optionId = '';
        // i.otherContent = '';
        // i.optionVoList && i.optionVoList.forEach(j => {
        //   j.otherContent = '';
        // });
      });
      console.log(data, 'data-初始值');
      this.questions = data;
    },
    setViewType(viewResult) {
      if (viewResult == '0') {
        return { viewBefore: false, viewAfter: false };
      } else if (viewResult == '1') {
        return { viewBefore: true, viewAfter: false };
      } else if (viewResult == '2') {
        return { viewBefore: false, viewAfter: true };
      } else if (viewResult == '3') {
        return { viewBefore: true, viewAfter: true };
      }
    },
    goBack(e) {
      this.$emit('closeDialog');
      if (history.state.dialog == 'fill') {
        history.go(-1); // window.history.back();
      }
    }
  },
  created() {
    window.abb = this;
    // console.log(this.questionData, 'this.questionData准备填写');
  },
  mounted() {
    // window.history.pushState({ dialog: 'fill' }, null, '');
    // window.addEventListener('popstate', this.goBack, false);
  },
  destroyed() {
    // window.removeEventListener('popstate', this.goBack, false);
    // this.goBack();
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-form-item.is-error .customInput .el-textarea__inner {
  border: 1px solid #DCDFE6;
}
.image-item {
  vertical-align: middle;
  height:50px;
  margin:6px;
}
.questionnaires-container {
  max-width: 1000px;
  margin: 0px auto;
  padding: 15px;
  background: white;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.1);
}

.header-section {
  // margin-bottom: 30px;
}

.title {
  color: #333;
  margin-bottom: 20px;
}

.info-item {
  margin-bottom: 15px;
  font-size: 14px;
}

.label {
  color: #666;
  // margin-right: 10px;
}

.value {
  color: #333;
}

.vote-settings {
  margin-bottom: 10px;
  padding: 15px;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  position: relative;
}
.fill-mask {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: transparent;
  z-index: 1;
}

.settings-item {
  margin: 10px 0;
}
.settings-item2 {
  display: inline-block;
  margin-left: 20px;
}

.description {
  color: #666;
  margin-bottom: 15px;
  font-size: 14px;
}

.el-form-item__label {
  font-weight: bold;
  color: #333 !important;
  padding: 0 !important;
}
span,div,p,.description{
  font-size: 15.5px;
}
::v-deep.question-form .el-form-item__label{
  color: black;
  font-size: 15.5px;
}
</style>