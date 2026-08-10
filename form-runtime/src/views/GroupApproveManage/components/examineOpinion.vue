<!--
 * @Descripttion: 审批意见
 * @Author: zhengzetao
 * @Date: 2022-08-17
-->

<template>
  <div style="text-align: center;position: relative">
    <div
      v-if="isExamine"
      class="opinion-toolbar"
    >
      <ApprovalOpinionPhraseDrawer v-model="approveForm.approveMessage" />
      <el-checkbox
        v-model="tracking"
        class="opinion-toolbar__checkbox"
      >跟踪此流程</el-checkbox>
    </div>
    <el-form v-if="isExamine" class="mt-10" :model="approveForm" :rules="rules" ref="approveForm">
      <el-form-item label="审批意见">
        <el-input type="textarea" :autosize="{ minRows: 2, maxRows: 7 }" maxlength="200" show-word-limit v-model="approveForm.approveMessage"
          placeholder="请填写审批意见"></el-input>
      </el-form-item>
    </el-form>
    <div class="button-bottom-div">
      <template v-if="isExamine">
        <el-button type="success" @click="submitCheckBefore('pass')" :loading="submitLoading">同意</el-button>
        <el-button type="warning" v-if="!isTranspondFlow" @click="clickRollBack()">回退上一节点</el-button>
        <el-button type="danger" v-if="!isTranspondFlow" @click="submitCheckBefore('no_pass')" :loading="noSubmitLoading">不同意</el-button>
        <el-button v-if="!isTranspondFlow" type="primary" plain @click="temporarySaveBizData">暂存</el-button>
        <el-button v-if="!isTranspondFlow" type="success" plain @click="addCounterSign"
        >加签</el-button>
        <!-- <el-button type="primary" plain @click="printPage" v-if="printSet.has(searchFlowType)">打印</el-button> -->
        <template v-if="printSet.has(searchFlowType)">
            <el-popover
            placement="top"
            width="200"
            trigger="click"
            style="margin-left: 3px;"
            v-if="searchFlowType == 'expense_budget' || searchFlowType == 'add_event_flow'"
            >
            <h3>请选择以下打印内容</h3>
            <el-checkbox-group v-model="printCheckList" style="margin: 10px 0px;">
              <el-checkbox label="表单" disabled></el-checkbox>
              <el-checkbox label="发起人附言"></el-checkbox>
              <el-checkbox label="流程日志"></el-checkbox>
            </el-checkbox-group>
            <el-button class="print-btn" v-print="'#formContainer'" style="float:right;" type="primary"  @click="printPage">确 认</el-button>
            <el-button type="primary" slot="reference" >打印</el-button>
          </el-popover>
          <el-button type="primary" @click="printPage" v-else>打印</el-button>
        </template>
      </template>
      <template v-else-if="isReInitiate">
        <el-button type="primary" @click="saveSubmit('save')"
          v-if="searchFlowType != 'group_finance' && searchFlowType != 'staff_annual_assessment'">保存草稿</el-button>
        <el-button type="primary" @click="saveSubmit('submit')">提交</el-button>
      </template>
      <template v-else>
        <!-- <el-button type="primary" @click="printPage" v-if="printSet.has(searchFlowType)">打印</el-button> -->
        <el-button
          :type="outTracking ? 'warning' : 'primary'"
          plain
          @click="setTracking"
          style="margin-right: 5px;"
          v-if="!isTimeoutPage && !hideTrackingButton"
        >{{ outTracking ? '取消跟踪':'设为跟踪' }}</el-button>
        <template v-if="printSet.has(searchFlowType)">
            <el-popover
            placement="top"
            width="200"
            trigger="click"
            style="margin-left: 3px;"
            v-if="searchFlowType == 'expense_budget' || searchFlowType == 'add_event_flow'"
            >
            <h3>请选择以下打印内容</h3>
            <el-checkbox-group v-model="printCheckList" style="margin: 10px 0px;">
              <el-checkbox label="表单" disabled></el-checkbox>
              <el-checkbox label="发起人附言"></el-checkbox>
              <el-checkbox label="流程日志"></el-checkbox>
            </el-checkbox-group>
            <el-button class="print-btn" v-print="'#formContainer'" style="float:right;" type="primary"  @click="printPage">确 认</el-button>
            <el-button type="primary" slot="reference" >打印</el-button>
          </el-popover>
          <el-button type="primary" @click="printPage" v-else>打印</el-button>
        </template>
        <el-button type="primary" plain @click="closeCheck">关闭</el-button>
      </template>
    </div>
    <!-- 审批节点选择：建立流程时如果选择了审批人自选，点击通过需要选下一节点审批人 -->
    <!-- <NextNodeDialog :visible.sync="nodeChooseVisible" :nextNodeName="nextNodeName" :nextNodeProxyId="nextNodeProxyId"
      @getSelectPerson="getSelectPerson" v-if="nodeChooseVisible">
    </NextNodeDialog> -->
    <!-- 新版审批人自选，老版因为流程配置时节点配置了公司权限。新版只能选全集团的人 -->
    <PersonSelectDialog
      v-if="oldNodeChooseVisible"
      :visible.sync="oldNodeChooseVisible"
      @getSelectPerson="getSelectPerson"
      :nextNodeName="nextNodeName"
      :nextNodeProxyId="currentChooseNodeId"
      :nodeAuditType="nodeAuditType"
      :countersignNum="countersignNum"
    />

    <!-- 2025.1.2审批人自选更换成审批人范围选择 -->
    <RangePersonSelect :visible.sync="nodeChooseVisible" v-if="nodeChooseVisible" @getSelectPerson="getSelectPerson"
    :nextNodeName="nextNodeName" :nextNodeProxyId="currentChooseNodeId" :nodeAuditScopeList="nodeAuditScopeList"
    :nodeAuditType="nodeAuditType" :countersignNum="countersignNum"/>

    <!-- 为并行节点选择自选审批人 -->
    <el-dialog
      v-if="parallelChooseVisible"
      width="400px"
      title="并行节点自选"
      class="nodePerson"
      :visible="parallelChooseVisible"
      :before-close="handleCloseParallelChoose"
      :close-on-click-modal="false"
      append-to-body
    >
      <div
        v-for="chooseNode in parallelNodeChooseList"
        :key="chooseNode.id"
      >
        <span style="line-height: 28px;">{{ chooseNode.nodeName }}: </span>
        <span
          v-for="audit in nextAuditorList"
          :key="audit.id"
        >
          <span v-if="chooseNode.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
        </span>
        <el-button
          v-if="chooseNode.auditType == 'run_node_choose'"
          type="text"
          size="mini"
          @click="chooseParallelNode(chooseNode)"
        >
        {{ nextAuditorList.findIndex(el=>el.nodeProxyId == chooseNode.nextNodeTemplateId) > -1 ? '重新选择' : '选择人员' }}</el-button>
      </div>
      <span
        slot="footer"
        class="dialog-footer"
      >
        <el-button
          type="primary"
          @click="handleSaveParallelChooseNode"
        >提 交</el-button>
      </span>
    </el-dialog>
    <!-- 手动选择分支 -->
    <el-dialog
      v-if="branchChooseVisible"
      width="500px"
      title="选择流程分支"
      class="nodePerson"
      :visible="branchChooseVisible"
      :before-close="handleCloseBranchChoose"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-radio-group
        v-model="chooseBranchNode"
        @change="handleChangeChooseBranch"
      >
        <el-radio
          v-for="(branch, index) in manualChooseNodes"
          :key="index"
          :label="branch"
          class="radio-choose-item"
        >
          <span>{{ branch.branchName }}-{{ branch.nodeName }} : </span>
          <span v-if="branch.auditType == 'run_node_choose'">
            <span v-for="(audit, index) in nextAuditorList" :key="index">
              <span v-if="branch.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
            </span>
            <el-button type="primary" :disabled="branch.nextNodeTemplateId != chooseBranchNode.nextNodeTemplateId"
              @click="handleChooseBranchNode(branch.branchName, branch.nodeName,branch.nextNodeTemplateId,branch)">选择审批人</el-button>
          </span>
          <span v-if="branch.auditType == 'initiator'">由发起人审批</span>
          <span v-if="branch.auditType == 'assign'">由流程配置人员审批</span>
          <span v-if="branch.nodeType == 'empty'">
            空节点
            <span v-for="(audit, index) in chooseBranchNodeList" :key="index">
              <span v-if="branch.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
            </span>
          </span>
          <!-- <span v-if="branch.auditType == 'branched_passage_manager'">分管副总</span>
          <span v-if="branch.auditType == 'department_supervisor'">部门主任</span> -->
        </el-radio>
      </el-radio-group>
      <span
        slot="footer"
        class="dialog-footer"
      >
        <el-button
          type="primary"
          @click="handleSaveBranchChooseNode"
        >提 交</el-button>
      </span>
    </el-dialog>
    <!-- 扩展属性选人 -->
    <extended-attribute-users :nextNodeProxyId="nextNodeProxyId" :visible.sync="extendedVisible" :personList="extendedPersonList" @confirmChooseExtendPerson="confirmChooseExtendPerson"></extended-attribute-users>

    <!-- 流程加签 -->
    <AddCounterSign v-if="addCounterSignVisible" :visible.sync="addCounterSignVisible" ref="AddCounterSign"
     :flowNodeProxyId="flowNodeProxyId" :flowProxyId="flowProxyId" :flowInstanceId="flowInstanceId" :isNoForm="true"
      :tFlowNodeProxyId="tFlowNodeProxyId" :tFlowProxyId="tFlowProxyId" @confirm="confirmAddCounterSign"></AddCounterSign>
      <!-- :outFlowNodeTemplate="outFlowNodeTemplate" -->
  </div>
</template>

<script>
import Api from "@/api";
// import NextNodeDialog from './nextNodeDialog.vue';
import PersonSelectDialog from "./PersonSelectDialog.vue";
import RangePersonSelect from './RangePersonSelect.vue';
import mixin from "./mixin.js";
import { localstorageGet } from "@/utils/auth";
import math from '@/utils/math.js'
import { deepClone } from "@/utils";
import AddCounterSign from './AddCounterSign.vue';
import extendedAttributeUsers from "@/components/extendedAttributeUsers.vue"
import ApprovalOpinionPhraseDrawer from './ApprovalOpinionPhraseDrawer.vue';

export default {
  name: "ExamineOpinion",
  mixins: [mixin],
  components: { PersonSelectDialog,RangePersonSelect,extendedAttributeUsers,AddCounterSign,ApprovalOpinionPhraseDrawer},
  data() {
    return {
      approveForm: {
        approveMessage: "", // 审批信息
      },
      rules: {
        approveMessage: [
          { required: true, message: "请填写审批意见", trigger: "blur" },
        ],
      },
      oldNodeChooseVisible: false,
      nodeChooseVisible: false,
      checkboxRersonGroup: [],
      parallelChooseVisible: false,
      parallelNextNodeTemplateId: "",
      nextAuditorList: [],
      parallelNodeChooseList: [],
      branchChooseVisible: false,
      chooseBranchNode: {},
      chooseBranchNodeList: [],
      manualChooseNodes: [],
      hasUpdate: false,
      submitLoading: false,
      noSubmitLoading: false,
      paralleNodeName: '',
      printSet: new Set(['expense_budget', 'monthly_perf_companySum', 'monthly_perf_departmentSum', 'reserveMonthSum','add_event_flow']),
      printCheckList:['流程日志'],
      returnNextNodeProxyId:'',
      pageNextNodeProxyId:'',
      nextNodeName:'',
      currentChooseNodeId:'',
      flowNodeType:'',
      nextNodeProxyId:'',
      chooseBranchList:[],

      // 范围
      nodeAuditScopeList:[],
      nodeAuditType:'',
      countersignNum:'',
      //扩展属性选人
      extendedVisible:false,
      extendedPersonList:[],
      // 是否跟踪此事项
      tracking:false,
      // 加签弹窗
      addCounterSignVisible:false,
      tFlowNodeProxyId:'',
      tFlowProxyId:'',
    };
  },
  props: {
    id: {
      // 流程id
      type: String,
      default: "",
    },
    isExamine: {
      type: Boolean,
      default: true,
    },
    isReInitiate: {
      type: Boolean,
      default: false,
    },
    initiatorId: {
      type: String,
      default: "",
    },
    jobTaskId: {
      type: String,
      default: "",
    },
    // flowNodeType: {
    //   type: String,
    //   default: "",
    // },
    // nextNodeName: {
    //   type: String,
    //   default: "",
    // },
    // nextNodeProxyId: {
    //   type: String,
    //   default: "",
    // },
    searchFlowType: {
      type: String,
      default: "",
    },
    flowProxyId: {
      // 流程id
      type: String,
      default: "",
    },
    flowInstanceId: {
      type: String,
      default: "",
    },
    noFormFlowInstanceId: {
      type: String,
      default: "",
    },
    formId: {
      type: String,
      default: "",
    },
    lastCountersignFlag: {
      type: Boolean,
      default: true,
    },
    auditPassLogicFlag: {
      type: Boolean,
      default: true,
    },
    flowName: {
      type: String,
      default: "",
    },
    isTranspondFlow: {
      type: Boolean,
      default: false,
    },
    formAndTranspondData: {
      type: Object,
      default: function(){
        return {}
      },
    },
    copyTemplateData: {
      type: Object,
      default: function(){
        return {};
      }
    },
    flowInstanceBizRelevanceList: {
      type: Array,
      default: function(){
        return [];
      }
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    outTracking:{
      type:Boolean,
      default:false
    },
    hideTrackingButton: {
      type: Boolean,
      default: false
    },
    outFlowNodeTemplate:{
      type: Object,
      default:function(){
        return {};
      }
    },
    isTimeoutPage:{ //是否是超时跳过的页面
      type:Boolean,
      default:false
    }
  },
  computed: {},
  watch: {
    outTracking: {
      handler(val) {
        this.tracking = this.normalizeTrackingValue(val);
      },
      immediate: true
    }
  },
  provide() {
    return { jusgeCustomChoose: () => { } };
  },
  created() {
    this.parallelNodeChooseList = [];
    this.manualChooseNodes = [];
    this.parallelNodeChooseList = this.$attrs.parallelNodeChooseList;
    this.manualChooseNodes = this.$attrs.manualChooseNodes;
    this.$bus.$off("submitBeforeHandleOk");
    this.$bus.$on("submitBeforeHandleOk", (obj) => {
      if(this.isTemporarySave){ //暂存
        this.afterTemporarySaveBizData()
        return
      }
      this.emitBudgetInfo = obj;
      console.log('this.isReInitiate',this.isReInitiate)
      if (this.isReInitiate) {
        if (obj.status == "success") {
          // console.log('created,submitFinal')
          // return;
          this.submitFinal(true, obj.id, this.searchFlowType);
        }
      } else {
        // console.log('obj',obj)
        // return
        if (obj.status == "success") {
          this.hasUpdate = true;
          this.handleSubmitCheck("pass", true);
        } else {
          this.hasUpdate = false;
        }
      }
    });

    this.$bus.$off("close");
    this.$bus.$on("close", () => {
      this.closeCheck();
    });

    this.queryStorageFormData();
  },
  mounted() { },
  methods: {
    normalizeTrackingValue(value) {
      if (value === undefined || value === null) {
        return false;
      }
      if (typeof value === 'boolean') {
        return value;
      }
      if (typeof value === 'number') {
        return value !== 0;
      }
      if (typeof value === 'string') {
        const normalized = value.trim().toLowerCase();
        if (['1', 'true', 'yes', 'y', 'on', '是', '跟踪', '已跟踪', '已设为跟踪'].includes(normalized)) {
          return true;
        }
        if (['0', 'false', 'no', 'n', 'off', '否', '', '不跟踪', '未跟踪', '取消跟踪'].includes(normalized)) {
          return false;
        }
        return false;
      }
      return false;
    },
    confirmAddCounterSign(data){
      this.tFlowNodeProxyId = data.flowNodeProxyId;
      this.tFlowProxyId = data.flowProxyId;
      console.log('tFlowNodeProxyId1',this.tFlowNodeProxyId)
      console.log('tFlowProxyId1',this.tFlowProxyId)
      this.$emit('updateList')
    },
    // 加签
    addCounterSign(){
      this.addCounterSignVisible = true;
    },
    printPage() {
      // console.log('打印')
      this.$emit("printPage",this.printCheckList);
    },
    closeCheck() {
      this.approveForm.approveMessage = "";
      this.$emit("postExamine");
    },
    async saveSubmit(type) {
      // console.log('saveSubmit')

      // type=='save' 保存草稿  type=='submit'提交待发流程
      if (type == "submit") {
        if (this.searchFlowType == "DRAWINGS") {
          // 文件和图纸审核
          this.submitTask(type);
        } else {
          // console.log('9')
          this.saveBizData();
        }
      } else if (type == "save") {
        console.log('保存草稿')
        //如果是费用报销
        if (this.searchFlowType == 'expense_budget') {
          const expensesClaimForm =
          this.$parent.$parent.$refs.ExpensesClaimForm ||
          this.$parent.$refs.ExpensesClaimForm;
        const originFileList = expensesClaimForm.originFileList;
        const submitPromise = expensesClaimForm.submit('draft');
        if (submitPromise) {
          // 只需保存修改不需要最终提交
          submitPromise.then((res) => {
            if (res.isSuccess) {
              res.data.expenseDetailList.forEach((item, index) => {
                if (item.attachmentIds || originFileList[index]?.length) {
                  // 新增的文件
                  const attachmentIdsList = item.attachmentIds.split(",");
                  // （点击审核进来页面的时候会把文件列表保存在originFileList里，后续的文件列表操作会在审核接口提交后，此处返回一个最新文件列表，然后对比后获取需要绑定业务id的文件id）
                  // const newFileIdArr = attachmentIdsList.filter(
                  //   (x) => !originFileList[index].map((y) => y.id).includes(x)
                  // ); // 获取现在文件列表中在原文件列表中不存在的id，用来绑定业务id
                  // if (newFileIdArr.length) {
                  //   this.bindBatchFileByIds(item.id, newFileIdArr); // 业务id绑定文件
                  // }

                  // 需要删除的文件
                  // const deleteAttachmentIdsList = item.attachmentIds.split(",");
                  // const deleteFileIdArr = originFileList[index]
                  //   .map((y) => y.id)
                  //   .filter((x) => !deleteAttachmentIdsList.includes(x)); // 获取现在文件列表中在原文件列表中不存在的id，用来绑定业务id
                  // console.log('submitPromise----res',deleteAttachmentIdsList)
                  // if (deleteAttachmentIdsList.length) {
                  //   this.deleteBatchFileByIds(item.id, deleteAttachmentIdsList); // 业务id绑定文件
                  // }

                  this.bindBatchFileByIds(item.id, attachmentIdsList); // 业务id绑定文件
                }
              })
              this.saveExpenseBudgetCommonFlowDraft().then(() => {
                this.$message.success("提交成功！");
                this.closeCheck(true);
              }).catch(() => {});
            } else {
              this.$message.error(res.message);
            }
          });
        }
        }else if(this.searchFlowType == 'company_monthly_budget'){

          const companyMonthlyFinance = this.$parent.$parent.$refs.company_monthly_budget || this.$parent.$refs.company_monthly_budget;
          companyMonthlyFinance.submit('draft').then(res=>{
            if(res){
              // id, type, money, name,method
              this.emitBudgetInfo = res;
              // this.submitFinal(false, res.id,this.selectFlowType ,res.money,res.name);
              this.handleSaveDraft()
            }else{
              this.$message.error('保存失败')
            }
          })
        } else if (this.searchFlowType == 'add_event_flow'){
          this.handleSaveDraft()
        } else {
          console.log('5555')
          // 保存草稿
          // 1 修改业务状态为草稿
          var component =
            this.$parent.$parent.$refs[this.searchFlowType] ||
            this.$parent.$refs[this.searchFlowType];
          await component.submit(0);
          console.log('777')
          // 2 修改流程状态为草稿 ?? 有问题
          this.handleSaveDraft()
        }
      }
    },
    getFlowListSelect(){
      const parentRefs = this.$parent?.$refs || {};
      const grandParentRefs = this.$parent?.$parent?.$refs || {};
      return grandParentRefs.flowListSelect || parentRefs.flowListSelect;
    },
    buildExpenseBudgetCommonFlowList(){
      const flowListSelect = this.getFlowListSelect();
      return flowListSelect && flowListSelect.dataShowList ? flowListSelect.dataShowList : [];
    },
    shouldSaveExpenseBudgetCommonFlowDraft(){
      const oldHasCommonFlow = this.flowInstanceBizRelevanceList.some(item => item.otherBiz == 'commonFlow');
      const newHasCommonFlow = this.buildExpenseBudgetCommonFlowList().length > 0;
      return oldHasCommonFlow || newHasCommonFlow;
    },
    buildExpenseBudgetFlowInstanceBizRelevanceList(){
      const list = deepClone(this.flowInstanceBizRelevanceList || []);
      const flowInstanceBizRelevanceList = list
        .filter(item => item.otherBiz != 'commonFlow')
        .map(item => ({
          otherBiz: item.otherBiz,
          otherBizId: item.otherBizId
        }));
      this.buildExpenseBudgetCommonFlowList().forEach(item => {
        flowInstanceBizRelevanceList.push({
          otherBiz: 'commonFlow',
          otherBizId: item.id
        });
      });
      return flowInstanceBizRelevanceList;
    },
    saveExpenseBudgetCommonFlowDraft(){
      if (!this.shouldSaveExpenseBudgetCommonFlowDraft()) return Promise.resolve();
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.schedule.saveFlowInstance,
          {
            data: {
              id: this.flowInstanceId,
              flowInstanceBizRelevanceList: this.buildExpenseBudgetFlowInstanceBizRelevanceList()
            }
          },
          res => {
            if (res.isSuccess) {
              resolve();
            } else {
              this.$message.error(res.message);
              reject(res);
            }
          }
        );
      });
    },
    getNewEventMongo(){
      let newEventVue = this.$parent.$parent.$refs.add_event_flow;
      const reg = /^<p><br><\/p>$/;
      // const reg = /(<\/?.*?>)/gi;
      const contentText = newEventVue.$refs.richEditorRef.contentHtml.replace(reg, '');
      if (!contentText) {
        this.$message.error('请输入内容！');
        return;
      }
      const content = newEventVue.$refs.richEditorRef.contentHtml;
      // richEditorRef在没有输入值时，还会返回空行标签，修复bug
      newEventVue.form.content = content.indexOf('<p><br></p>') == 0 && content.length == 11 ? '' : newEventVue.$refs.richEditorRef.contentHtml;
      // console.log('newEventVue.$refs',newEventVue.$refs)
      newEventVue.form.flowList = newEventVue.$refs.flowListSelect.dataShowList;
      console.log('newEventVue.form', newEventVue.form)
      return newEventVue.form;
    },
    // 保存草稿
    handleSaveDraft() {
      const param = {
        data: {
          id: this.flowInstanceId,
          formProxyId: this.formId,
          status: "draft",
        },
      }

      if (this.searchFlowType == 'add_event_flow'){
        console.log('保存草稿参数',this.$parent.$parent.$refs.add_event_flow)

        param.formDataMongoVo = {
          data:{}
        };
        param.formDataMongoVo.data = this.getNewEventMongo();
        // param.formDataMongoVo.data = newEventVue.form;
      }

      // return;
      this.$axios.post(
        Api.schedule.saveFlowInstance,
        param,
        res => {
          if (res.isSuccess) {
            this.$message.success('保存成功！');
            this.closeCheck();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getSelectPerson(data) {
      if (data.checkboxPersonGroup && data.checkboxPersonGroup.length > 0) {
        this.nextAuditorList = this.nextAuditorList.filter(el=>el.nodeProxyId != this.currentChooseNodeId) //清除当前节点id的旧的人员
        data.checkboxPersonGroup.forEach(item=>{
          this.nextAuditorList.push({
              name:item.name,
              bizId:item.id,
              auditDetailType:"personnel",
              nodeProxyId:item.nodeProxyId
            })
        })
      }else{
        this.$message.warning('至少选择一位审批人');
        return false;
      }
      //如果是单独选人分支，选完后提交
      if(!this.manualChooseNodes?.length && !this.parallelNodeChooseList?.length){
        this.saveBizData();
      }
      return
    },
    submitTask(type) {
      console.log('审核无表单-submitTask')
      const param = {
        data: {
          jobTaskId: this.jobTaskId,
          auditRecord: {
            auditStatus: type,
            executeDesc: this.approveForm.approveMessage,
          },
          id: this.flowInstanceId
        },
        nextAuditorList: null,
        formDataMongoVo: {
          data: {} // null,
        },
        tracking:this.tracking
      };
      this.submitLoading = true;

      if (this.isTranspondFlow) {
        param.formDataMongoVo.data = this.formAndTranspondData;
      }

      this.$axios.post(Api.qualityManage.submitTask, param, (res) => {
        this.submitLoading = false
        if (res.isSuccess) {
          this.$message.success("提交成功");
          this.$listeners.postExamine();
          if (!this.nextNodeName) {
            if (this.fileType == "FILES") {
              // 文件审核更新文件状态
            } else {
              this.$axios.post(
                "/web/project/api/drawing/updateDrawingStatus",
                {
                  data: {
                    drawingIds: this.fileIdS,
                    drawingStatus: "approved",
                    customerCode: this.$store.state.user.customerCode,
                  },
                },
                (res) => { }
              );
            }
          }
        } else {
          // this.$message.error(res.message);
        }
      }).catch(() => {
        this.submitLoading = false
      })
    },
    /*  getSelectPerson(data) {
      console.log('getSelectPerson', data);
      this.checkboxPersonGroup = data.checkboxPersonGroup.map(x => x.id);
      this.submitCheckBefore('pass');
    }, */
    // 判断是否审批自选
    // 判断是否审批自选
    // checkIsOptional(type) {
    //   // console.log('checkIsOptional')
    //   if (this.flowNodeType == "run_node_choose") {
    //     // console.log('6')
    //     // 自选节点
    //     // console.log('this.checkboxPersonGroup',this.checkboxPersonGroup)

    //     if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
    //       if (this.searchFlowType == "annual_perf") { // 年度目标责任书在重新发起前先校验表单完整性，再拉起自选节点
    //         if (this.$parent.$parent.$refs.annual_perf.validateForm('form')) {
    //           this.nodeChooseVisible = true;
    //         }
    //       } else {
    //         this.nodeChooseVisible = true;
    //       }
    //     } else {
    //       // console.log('8')
    //       if (this.searchFlowType == "DRAWINGS") {
    //         // 文件和图纸审核
    //         this.submitTask(type);
    //       } else {
    //         // console.log('9')
    //         this.saveBizData();
    //       }
    //     }
    //   } else if (
    //     this.flowNodeType == "department_supervisor" ||
    //     this.flowNodeType == "branched_passage_manager"
    //   ) {
    //     // console.log('11')
    //     // 节点-发起人部门主管或者分管副总
    //     this.getSuperVisorOrLeader(this.flowNodeType);
    //   } else {
    //     if (this.searchFlowType == "DRAWINGS") {
    //       // 文件和图纸审核
    //       this.submitTask(type);
    //     } else {
    //       this.saveBizData();
    //     }
    //     // if (this.isReInitiate) {
    //     // 重新发起
    //     // this.saveBizData();
    //     // } else {
    //     //   this.handleSubmitCheck('pass', true);
    //     // }
    //   }
    // },
    submitCheckBefore(type) {
      let msg = '不同意后，流程“终止，并驳回给发起人”，是否确认不同意?'
      let msgTitle = '终止流程'
      if(type == 'pass'){
        msgTitle = '提示'
        msg = '确认同意？'
      }
      this.$confirm(msg, msgTitle).then(()=>{
        this.$refs.approveForm.validate((valid) => {
        if (valid) {
          // const mayEditFlowArr = [
          //   'company_annual_budget',
          //   'project_setup_budget',
          //   'depart_monthly_budget',
          //   'add_annual_budget',
          //   'add_project_budget'
          // ];
          // if (mayEditFlowArr.indexOf(this.searchFlowType) > -1 && !this.hasUpdate) {
          //   // 年度 月度 项目预算可能出现修改，需要先提交修改
          //   let emitType = this.searchFlowType;
          //   if (this.searchFlowType == 'add_annual_budget') {
          //     emitType = 'company_annual_budget';
          //   }
          //   if (this.searchFlowType == 'add_project_budget') {
          //     emitType = 'project_setup_budget';
          //   }
          //   this.$bus.$emit(emitType + '_before_handle', type);
          // } else {
          // return
          if (type == "pass") {
            // if (
            //   this.parallelNodeChooseList &&
            //   this.parallelNodeChooseList.length > 0
            // ) {
            //   // 并行节点中的审批人自选、主管、副总 类型的审批节点被传过来了
            //   // 判断并行节点中是否包含审批人自选
            //   const flag = this.parallelNodeChooseList.some(
            //     (item) => item.auditType == "run_node_choose"
            //   );
            //   if (flag) {
            //     this.parallelChooseVisible = true;
            //   } else {
            //     this.handleSaveParallelChooseNode();
            //   }
            //   return false;
            // } else if (
            //   this.manualChooseNodes &&
            //   this.manualChooseNodes.length &&
            //   this.auditPassLogicFlag
            // ) {
            //   // } else if (row.auditPassLogicFlag && row.branchExecuteType == 'custom_choose' && row.nextAuditNodeList.length > 1) {
            //   // 手动分支选择分支
            //   this.branchChooseVisible = true;
            //   return false;
            // } else {
            //   // 点击通过的时候需要判断是否是自选节点--普通节点
            //   this.checkIsOptional(type);
            // }
            if (this.searchFlowType == "DRAWINGS") {
              // 文件和图纸审核
              this.submitTask(type);
            } else {
              this.saveBizData();
            }
          } else {
            this.handleSubmitCheck(type);
          }
          // }
        } else {
          return false;
        }
      });
      }).catch(()=>{})
    },
    // 审批通过或者不通过
    handleSubmitCheck(type, flag) {
      console.log('审核无表单-handleSubmitCheck')
      // console.log(11111112222,this.copyTemplateData)
      // return;
      const param = {
        data: {
          jobTaskId: this.jobTaskId,
          auditRecord: {
            auditStatus: type,
            executeDesc: this.approveForm.approveMessage,
          },
          id: this.flowInstanceId
        },
        formDataMongoVo: {
          data: {}
        },
        tracking:this.tracking
      };
      if(this?.emitBudgetInfo?.name)param.data.name = this?.emitBudgetInfo?.name
      if (type == "pass") {
        if (flag) {
            // param.nextAuditorList = this.nextAuditorList;
          param.nextAuditorList = this.nextAuditorList.map(item=>{
            item.auditDetailType = "personnel"
            if(item.nodeProxyId === undefined)item.nodeProxyId = this.pageNextNodeProxyId
            return item
          });
        }
        if (this.chooseBranchList && this.chooseBranchList.length) {
          // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
          //chooseBranchList去重
          this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
          this.chooseBranchList.forEach(item=>{
            param.nextAuditorList.push({nodeProxyId:item})
          })
        }
        // 传这个字段为了多分支流程那边的判断；调剂单不用做分支判断
        if (this.searchFlowType == "expense_budget") {
          const infoForm =
            this.$parent.$parent.$refs.ExpensesClaimForm?.infoForm ||
            this.$parent.$refs.ExpensesClaimForm.infoForm;
          let expendTypeTotalMoney = 0
          infoForm.expenseBudgetList.forEach(item => {
            expendTypeTotalMoney = math.add(expendTypeTotalMoney, item.money)
          });
          let expendDetailTotalMoney = 0
          infoForm.expenseDetailList.forEach(item => {
            expendDetailTotalMoney = math.add(expendDetailTotalMoney, item.money)
          });
          let taxPreNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item => {
            taxPreNumTotalMoney = math.add(taxPreNumTotalMoney, item.money)
            // pre = math.add(pre + cur.money)
            // return pre
          });
          let taxNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item => {
            taxNumTotalMoney = math.add(taxNumTotalMoney, item.tax)
          });

          let taxTotalNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item => {
            taxTotalNumTotalMoney = math.add(taxTotalNumTotalMoney, item.totalAmount)
          })

          let accountInfoTotalMoney = 0
          infoForm.expenseInAccountInfoList.forEach(item => {
            accountInfoTotalMoney = math.add(accountInfoTotalMoney, item.money)
          });

          param.formDataMongoVo = {
            data: {},
          };
          // 这个formDataMongoVo用来存放需要用来判断字段走哪条分支。data下面传的字段要对应表单里面的字段（对应上数据字典的字段）
          param.formDataMongoVo.data.expendTypeNum = expendTypeTotalMoney;
          param.formDataMongoVo.data.expendDetailNum = expendDetailTotalMoney;
          param.formDataMongoVo.data.taxPreNum = taxPreNumTotalMoney;
          param.formDataMongoVo.data.taxNum = taxNumTotalMoney;
          param.formDataMongoVo.data.taxTotalNum = taxTotalNumTotalMoney;
          param.formDataMongoVo.data.accountInfoNum = accountInfoTotalMoney;
          param.formDataMongoVo.data.expenseCompanyId = infoForm.expenseCompanyName
          // param.formDataMongoVo = {
          //   data: {
          //     expendTypeNum: expendTypeTotalMoney
          //   }
          // };
          let userName = infoForm.userName || ''
          param.data.name = `${this.flowName}-${userName}-${Number(expendDetailTotalMoney.toFixed(2))}元`;

        } else if (
          [
            "company_annual_budget",
            "depart_monthly_budget",
            "project_setup_budget",
            "add_annual_budget",
            "add_project_budget",
          ].indexOf(this.searchFlowType) > -1
        ) {
          param.formDataMongoVo = {
            data: {
              money: this.emitBudgetInfo.total,
              initiatorRange: this.$store.state.user.userId,
            },
          };
        } else if (this.searchFlowType == 'annual_perf' || this.searchFlowType == 'year_kpi_work_target') {
          param.formDataMongoVo.data.name = this.$parent.$parent.$refs[this.searchFlowType].createrId;
        } else if (this.searchFlowType == 'staff_annual_assessment') {
           let customName = '';
           const parent1 = this.$parent
           const parent2 = this.$parent.$parent
           const steps2 = parent2?.$refs?.steps2 || parent1?.$refs?.steps2
           const annualAssessmentForm =
               parent2?.$refs?.staff_annual_assessment ||
               parent1?.$refs?.staff_annual_assessment ||
               steps2?.$refs?.staff_annual_assessment

           console.log('%c [ annualAssessmentForm handleSubmitCheck ]', 'font-size:13px; background:pink; color:#bf2c9f;', annualAssessmentForm)

           if (annualAssessmentForm) {
                const data = annualAssessmentForm.detail || annualAssessmentForm.rawData || {};
                const year = data.year || '';
                const userName = data.userName || '';
                if (year && userName) {
                    customName = `${year}年年终绩效考核-${userName}`;
                }
           }

           if (customName) {
             param.data.name = customName
           }
        } else {
          param.formDataMongoVo = {
            data: {} // null,
          };
        }
      }

      if (this.searchFlowType == 'monthly_perf') {
        param.formDataMongoVo.data.userInfo = (this.$parent.$parent.$refs.monthly_perf?.form || this.$parent.$refs.monthly_perf?.form)?.userId;
      } else if (this.searchFlowType == "add_event_flow"){
        param.formDataMongoVo.data = this.copyTemplateData;
      }

      if (this.isTranspondFlow) { // 如果是转发的流程，把传进来表单数据一起传过去。但是不覆盖之前相同字段的值
        let originFormData = deepClone(param.formDataMongoVo.data);
        const merged = {
          ...originFormData,
          ...Object.keys(this.formAndTranspondData).reduce((acc, key) => {
            if (!originFormData.hasOwnProperty(key)) {
              acc[key] = this.formAndTranspondData[key];
            }
            return acc;
          }, {})
        };
        console.log('merged',merged)
        param.formDataMongoVo.data = merged;

        delete param.data.name // 如果是转发数据，直接删除名称
      }

      console.log('param',param)
      // return
      if (type == 'pass') this.submitLoading = true
      if (type == 'no_pass') this.noSubmitLoading = true
      this.$axios.post(Api.qualityManage.submitTask, param, (res) => {
        this.submitLoading = false
        this.noSubmitLoading = false
        if (res.isSuccess) {
          if(this.$refs.attachList){
            // 存档管理流程审核修改文件状态
            const attachList = this.$refs.attachList.attachList
            attachList.map(item=>{
              const file = item.data
              if(type=='no_pass'){
                file.fileStatus = '3'
              }else{
                file.fileStatus = '1'
              }
              this.updateFileStatus({id:file.id,fileName:file.fileName.split('.')[0],fileStatus:file.fileStatus})
            })
          }

          // this.handleBeforeSave();
          this.$message.success("提交成功");
          this.nodeChooseVisible = false;
          //绑定文件到对应的节点，如果有
          this.$emit('bindFile',res?.data?.auditRecord)
          this.$emit("postExamine");
        } else {
          this.submitLoading = false
          this.noSubmitLoading = false
          this.flowErrorHandle(res)
        }
      }).catch(() => {
        this.submitLoading = false
        this.noSubmitLoading = false
      })
    },
    // 重新发起流程需要校验公司，避免流程发起后撤回，切换公司重新发起流程造成问题
    checkFlowPermission() {
      return new Promise((resolve,reject)=>{
          const param = {
          data: {
            flowProxyId: this.flowProxyId,
            checkPermissions: 'first',
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'company',
                otherBizId: localstorageGet('companyId')
              }
            ]
          }
        };
        this.$axios.post(
          Api.schedule.saveFlowInstance,
          param
        ).then(res=>{
          if(res.isSuccess){
            resolve(true)
          }else{
            resolve(false)
          }
        })
      })
    },
    // 最终的提起流程审批接口--无表单流程
    async submitFinal(customChooseFlag, id, type) {
      let r = await this.checkFlowPermission()
      if(r === false)return this.$message.error('暂无权限发起流程')
      // 这个是再次提交审核的接口
      const param = {
        // data: {
        //   flowProxyId: this.flowProxyId,//this.flowId,
        //   flowInstanceBizRelevanceList: [
        //     {
        //       otherBiz: this.searchFlowType,
        //       otherBizId: id // 保存后返回的id
        //       // otherBiz: 'expense_budget',
        //     }
        //     // { // 关联项目才需要传(从项目入口进入才需要传)
        //     //   otherBiz: 'project',
        //     //   otherBizId: this.$store.state.user.projectId
        //     // }
        //   ]
        // },
        data: {
          id: this.noFormFlowInstanceId,
        },
        formDataMongoVo: {
          data: {
            initiatorRange: this.$store.state.user.userId, // 提审的时候每张表单都传一下发起人范围，条件判断有用
          },
        },
      };

      // 传这个字段为了多分支流程那边的判断；调剂单不用做分支判断
      if (type == "expense_budget") {
        const infoForm =
          this.$parent.$parent.$refs.ExpensesClaimForm?.infoForm ||
          this.$parent.$refs.ExpensesClaimForm.infoForm;
        let expendTypeTotalMoney = 0
        infoForm.expenseBudgetList.forEach(item => {
          expendTypeTotalMoney = math.add(expendTypeTotalMoney, item.money)
        });
        let expendDetailTotalMoney = 0
        infoForm.expenseDetailList.forEach(item => {
          expendDetailTotalMoney = math.add(expendDetailTotalMoney, item.money)
        });
        let taxPreNumTotalMoney = 0
        infoForm.taxInfoList.forEach(item => {
          taxPreNumTotalMoney = math.add(taxPreNumTotalMoney, item.money)
          // pre = math.add(pre + cur.money)
          // return pre
        });
        let taxNumTotalMoney = 0
        infoForm.taxInfoList.forEach(item => {
          taxNumTotalMoney = math.add(taxNumTotalMoney, item.tax)
        });

        let taxTotalNumTotalMoney = 0
        infoForm.taxInfoList.forEach(item => {
          taxTotalNumTotalMoney = math.add(taxTotalNumTotalMoney, item.totalAmount)
        })

        let accountInfoTotalMoney = 0
        infoForm.expenseInAccountInfoList.forEach(item => {
          accountInfoTotalMoney = math.add(accountInfoTotalMoney, item.money)
        });
        // 这个formDataMongoVo用来存放需要用来判断字段走哪条分支。data下面传的字段要对应表单里面的字段（对应上数据字典的字段）
        param.formDataMongoVo.data.expendTypeNum = expendTypeTotalMoney;
        param.formDataMongoVo.data.expendDetailNum = expendDetailTotalMoney;
        param.formDataMongoVo.data.taxPreNum = taxPreNumTotalMoney;
        param.formDataMongoVo.data.taxNum = taxNumTotalMoney;
        param.formDataMongoVo.data.taxTotalNum = taxTotalNumTotalMoney;
        param.formDataMongoVo.data.accountInfoNum = accountInfoTotalMoney;
        param.formDataMongoVo.data.expenseCompanyId = infoForm.expenseCompanyName
        param.data.name = `${this.flowName}-${localstorageGet(
          "userName"
        )}-${Number(expendDetailTotalMoney.toFixed(2))}元`;
      } else if (this.searchFlowType == 'add_event_flow') {
        // console.log('保存草稿参数',this.$parent.$parent.$refs.add_event_flow)
        // console.log(1,this.getNewEventMongo())
        // console.log(3,param.formDataMongoVo.data)
        param.data.name = this.getNewEventMongo().name;
        param.formDataMongoVo.data = Object.assign(param.formDataMongoVo.data,this.getNewEventMongo());
        // param.formDataMongoVo.data = newEventVue.form;
      } else if (this.searchFlowType == 'annual_perf' || this.searchFlowType == 'year_kpi_work_target') {
        param.formDataMongoVo.data.name = this.$parent.$parent.$refs[this.searchFlowType].createrId;
      } else if (this.searchFlowType == 'staff_annual_assessment') {
        param.formDataMongoVo = {
          data: {} // null,
        };
        if (this.emitBudgetInfo.name) {
          param.data.name = this.emitBudgetInfo.name
        } else {
          const parent1 = this.$parent
          const parent2 = this.$parent.$parent
          const steps2 = parent2?.$refs?.steps2 || parent1?.$refs?.steps2
          const annualAssessmentForm =
            parent2?.$refs?.staff_annual_assessment ||
            parent1?.$refs?.staff_annual_assessment ||
            steps2?.$refs?.staff_annual_assessment
          if (annualAssessmentForm && annualAssessmentForm.detail) {
            const year = annualAssessmentForm.detail.year || '';
            const userName = annualAssessmentForm.detail.userName || '';
            param.data.name = `${year}年年终绩效考核-${userName}`;
          }
        }
      } else {
        param.formDataMongoVo = {
          data: {} // null,
        };
        if (this.emitBudgetInfo.name) {
          param.data.name = this.emitBudgetInfo.name
        } else {
          if (this.emitBudgetInfo && this.emitBudgetInfo.total) {
            param.data.name = `${this.flowName}-${localstorageGet("userName")}￥${this.emitBudgetInfo.total}元`;
          } else {
            param.data.name = `${this.flowName}-${localstorageGet("userName")}`;
          }
        }
      }
      if (this.searchFlowType == 'monthly_perf') {
        param.formDataMongoVo.data.userInfo = (this.$parent.$parent.$refs.monthly_perf?.form || this.$parent.$refs.monthly_perf?.form)?.userId;
      }
      // console.log('this.nextAuditorList',this.nextAuditorList)
      // if (flag) {
          // param.nextAuditorList = this.nextAuditorList;
        param.nextAuditorList = this.nextAuditorList.map(item=>{
          item.auditDetailType = "personnel"
          if(item.nodeProxyId === undefined)item.nodeProxyId = this.pageNextNodeProxyId
          return item
        });
      // }
      if (this.chooseBranchList && this.chooseBranchList.length) {
        this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
        this.chooseBranchList.forEach(item=>{
          param.nextAuditorList.push({nodeProxyId:item})
        })
      }

      console.log('this.isTranspondFlow',this.isTranspondFlow)
      if(this.isTranspondFlow){ // 如果是转发数据，直接删除名称
        delete param.data.name
      }
      console.log('param',param)
      // return;
      param.data.companyId = localstorageGet('companyId')
      this.$axios
        .post(
          // Api.schedule.saveFlowInstance,
          Api.schedule.saveFlowInstanceAgain,
          param
        )
        .then((res) => {
          this.submitLoading = false
          if (res.isSuccess) {
            this.$message.success("提交成功！");
            this.$emit('success');
            this.closeCheck(true);

          } else {
            this.submitLoading = false
            this.noSubmitLoading = false
            this.flowErrorHandle(res)
          }
        })
        .catch((err) => {
          this.flowErrorHandle(err)
          this.submitLoading = false
          this.noSubmitLoading = false
        })
    },
    // 单个文件绑定业务id
    bindFileById(relationId, fileId) {
      const data = {
        relationId,
        fileId,
      };
      return this.$axios.post(Api.schedule.saveAttachment, { data });
    },
    // 多个文件绑定业务id
    bindBatchFileByIds(relationId, fileIds) {
      const data = {
        relationId,
        fileIds,
      };
      return this.$axios.post(Api.budgetManage.saveBatchFile, { data });
    },
    // 删除多个文件
    deleteBatchFileByIds(relationId, fileIds) {
      const data = {
        relationId,
        fileIds,
      };
      return this.$axios.post(Api.budgetManage.deleteBatchFile, { data });
    },

    // 发起人-并行节点自选审批人保存后提交
    // async handleSaveParallelChooseNode() {
    //   this.nextAuditorList = [];
    //   let superVisorId = '';
    //   let managerId = '';
    //   const isHasSuperVisor = this.parallelChooseNodes.some(node => node.auditType == 'department_supervisor');
    //   const isHasViceManager = this.parallelChooseNodes.some(node => node.auditType == 'branched_passage_manager');
    //   if (isHasSuperVisor) {
    //     superVisorId = await this.getSuperVisorOrLeaderId('department_supervisor');
    //   }
    //   if (isHasViceManager) {
    //     managerId = await this.getSuperVisorOrLeaderId('branched_passage_manager');
    //   }
    //   let hasNoChoose = false;// 是否有未选择审批人的节点
    //   this.parallelChooseNodes.forEach(item => {
    //     if (item.auditType == 'run_node_choose') {
    //       if (!item.nodeAuditList.length) {
    //         hasNoChoose = true;
    //       }
    //       item.nodeAuditList.forEach(auditNode => {
    //         this.nextAuditorList.push(
    //           {
    //             bizId: auditNode.id,
    //             nodeProxyId: item.nextNodeTemplateId
    //           }
    //         );
    //       });
    //     } else if (item.auditType == 'department_supervisor' || item.auditType == 'branched_passage_manager') {
    //       // 为并行中的主管和副总添加审批人节点参数传到提交
    //       this.nextAuditorList.push(
    //         {
    //           bizId: item.auditType == 'department_supervisor' ? superVisorId : managerId,
    //           nodeProxyId: item.nextNodeTemplateId
    //         }
    //       );
    //     } else {
    //       if (!item.nodeAuditList.length) {
    //         hasNoChoose = true;
    //       }
    //     }
    //   });
    //   if (hasNoChoose) {
    //     this.$message.warning('当前并行分支下有节点审批人未指定');
    //     return false;
    //   } else {
    //     this.parallelChooseVisible = false;
    //     if (!this.formExist) {
    //       // 表单流程
    //       this.enterpriseHandleSubmit(true);
    //     } else {
    //       // 无表单流程
    //       this.handleBeforeSave(true);
    //     }
    //   }
    // },

    // 发起人-并行节点自选审批人保存后提交
    async handleSaveParallelChooseNode() {
      let has = true
      this.parallelNodeChooseList.forEach(item=>{
        if(item.auditType == 'run_node_choose'){
          let nextNodeTemplateId = item.nextNodeTemplateId
          let index = this.nextAuditorList.findIndex(el=>el.nodeProxyId == nextNodeTemplateId)
          if(index == -1)has = false
        }
      })
      if(!has){
        return this.$message.warning('该分支需选择审批人，请先选择');
      }
      this.parallelChooseVisible = false;
      this.saveBizData();
      return
    },
    // 先保存业务数据
    saveBizData() {
      console.log('saveBizData')
      this.submitLoading = true
      if (this.searchFlowType == "expense_budget") {
        //保存费用报销单
        // 费用报销单
        // return;
        const expensesClaimForm =
          this.$parent.$parent.$refs.ExpensesClaimForm ||
          this.$parent.$refs.ExpensesClaimForm;
        const originFileList = expensesClaimForm.originFileList;
        // console.log('originFileList', originFileList);
        const submitPromise = expensesClaimForm.submit();
        if (submitPromise) {
          // 只需保存修改不需要最终提交
          submitPromise.then((res) => {
            if (res?.isSuccess) {
              // this.$message.success('提交成功！');
              // this.$emit('postExamine');
              // this.handleClose();
              res.data.expenseDetailList.forEach((item, index) => {
                if (item.attachmentIds || originFileList[index]?.length) {
                  // 新增的文件
                  const attachmentIdsList = item.attachmentIds.split(",");
                  // （点击审核进来页面的时候会把文件列表保存在originFileList里，后续的文件列表操作会在审核接口提交后，此处返回一个最新文件列表，然后对比后获取需要绑定业务id的文件id）
                  // const newFileIdArr = attachmentIdsList.filter(
                  //   (x) => !originFileList[index].map((y) => y.id).includes(x)
                  // ); // 获取现在文件列表中在原文件列表中不存在的id，用来绑定业务id
                  // if (newFileIdArr.length) {
                  //   this.bindBatchFileByIds(item.id, newFileIdArr); // 业务id绑定文件
                  // }

                  // 需要删除的文件
                  // const deleteAttachmentIdsList = item.attachmentIds.split(",");
                  // const deleteFileIdArr = originFileList[index]
                  //   .map((y) => y.id)
                  //   .filter((x) => !deleteAttachmentIdsList.includes(x)); // 获取现在文件列表中在原文件列表中不存在的id，用来绑定业务id
                  // console.log('submitPromise----res',deleteAttachmentIdsList)
                  // if (deleteAttachmentIdsList.length) {
                  //   this.deleteBatchFileByIds(item.id, deleteAttachmentIdsList); // 业务id绑定文件
                  // }

                  this.bindBatchFileByIds(item.id, attachmentIdsList); // 业务id绑定文件
                }
              });
              if(this.isTemporarySave){ //暂存
                this.afterTemporarySaveBizData()
                return
              }
              // return
              if (this.isReInitiate) {
                // console.log('saveBizData,submitFinal')
                // return;
                // console.log('保存完成，提交流程',obj)
                this.submitFinal(true, res.data.id, "expense_budget");
              } else {
                this.handleSubmitCheck("pass", true);
              }
            } else {
              if(res?.message)this.$message.error(res.message);
              this.submitLoading = false
            }
          });
        } else {
          this.submitLoading = false
        }
      }else if(this.searchFlowType == 'company_monthly_budget'){
        const companyMonthlyFinance = this.$parent.$parent.$refs.company_monthly_budget || this.$parent.$refs.company_monthly_budget;
          companyMonthlyFinance.submit('submit').then(res=>{
            if(res){
              if(this.isTemporarySave){ //暂存
                this.afterTemporarySaveBizData()
                return
              }
              // id, type, money, name,method
              this.emitBudgetInfo = res;
              if (this.isReInitiate) {
                // console.log('saveBizData,submitFinal')
                // return;
                // console.log('保存完成，提交流程',obj)
                this.submitFinal(true, res.id, this.selectFlowType);
              } else {
                this.handleSubmitCheck("pass", true);
              }
            }else{
              this.$message.error('保存失败')
            }
          })
      } else if (this.searchFlowType == "add_event_flow") {
        console.log('created,submitFinal',this.isReInitiate)
        const NewEvent = this.$parent.$parent.$refs.add_event_flow || this.$parent.$refs.add_event_flow;
        console.log('this.$parent',this.$parent)
        console.log('NewEvent',NewEvent)

        NewEvent.submit().then(res=>{
          console.log('NewEvent.submit',res)
          if(this.isTemporarySave){ //暂存
            this.afterTemporarySaveBizData()
            return
          }
          if(res){
            this.$bus.$emit('submitBeforeHandleOk', {
              status:'success'
            });
          }else{
            // this.$message.error('保存失败')
          }
        })

        // if (this.isReInitiate) {
          // console.log('11111',this.$parent.$parent.$refs.add_event_flow)
          // this.saveNewEvent();
          // this.submitFinal(true, null, this.searchFlowType);
          // if (obj.status == "success") {
            // console.log('created,submitFinal')
            // return;
            // this.submitFinal(true, obj.id, this.searchFlowType);
          // }
        // } else {
          // console.log('obj',obj)
          // return
          // if (obj.status == "success") {
          //   this.hasUpdate = true;
            // this.handleSubmitCheck("pass", true);
          // } else {
          //   this.hasUpdate = false;
          // }
        // }


      } else if (this.searchFlowType == 'staff_annual_assessment') {

        const parent1 = this.$parent
        const parent2 = this.$parent.$parent
        const steps2 = parent2?.$refs?.steps2 || parent1?.$refs?.steps2
        const annualAssessmentForm =
          parent2?.$refs?.staff_annual_assessment ||
          parent1?.$refs?.staff_annual_assessment ||
          steps2?.$refs?.staff_annual_assessment
        if (annualAssessmentForm && typeof annualAssessmentForm.postData === 'function') {
          annualAssessmentForm.setOpinion(this.approveForm.approveMessage)
          if (this.isReInitiate) {
             if (this.nextAuditorList && this.nextAuditorList.length > 0) {
                  this.$bus.$emit('staff_annual_assessment_before_handle', 'pass');
             } else {
                 this.checkStaffAnnualAssessmentSelection().then(res => {
                      if (!res.isSuccess && res.data && res.data.errorType === 'run_node_choose') {
                          this.flowErrorHandle(res);
                      } else {
                          this.$bus.$emit('staff_annual_assessment_before_handle', 'pass');
                      }
                  })
             }
          } else {
            // 使用流程实例ID作为batchCode
            annualAssessmentForm.postData('pass', this.flowInstanceId, (error, result) => {
              if (error) {
                this.submitLoading = false
                this.$message.error(error || '保存失败')
              } else {
                if (this.isTemporarySave) {
                  this.afterTemporarySaveBizData()
                  return
                }
                this.handleSubmitCheck("pass", true);
              }
            }, this.approveForm.approveMessage)
          }
        } else {
          if (this.isTemporarySave) {
            this.afterTemporarySaveBizData()
            return
          }
          this.handleSubmitCheck("pass", true);
        }
      } else {
        let emitType = this.searchFlowType;
        if (this.searchFlowType == "add_annual_budget") {
          emitType = "company_annual_budget";
        }
        if (this.searchFlowType == "add_project_budget") {
          emitType = "project_setup_budget";
        }
        console.log(emitType + "_before_handle")
        this.$bus.$emit(emitType + "_before_handle", "under_review", this);
      }
    },
    temporarySaveBizData() { // 暂存业务数据不走流程审批提交，参考月度绩效汇总保存业务数据后直接执行第二个回调参数
      this.isTemporarySave = true
      this.saveBizData()
    },
    async afterTemporarySaveBizData(){
      // this.$bus.$emit(this.searchFlowType + '_before_handle', 'temporarySave', () => {
      //调用暂存的流程接口，保存审批意见之类的
      await this.temporaryStorage()
      this.$message.success('暂存成功！');
      this.isTemporarySave = false
      this.closeCheck();
      // });
    },
    checkStaffAnnualAssessmentSelection() {
       return new Promise(resolve => {
           const param = {
             data: {
               id: this.flowInstanceId,
               checkPermissions: 'first'
             }
           };
           this.$axios.post(Api.schedule.saveFlowInstanceAgain, param, res => {
               resolve(res);
           });
       });
    },
    // 暂存实例
    temporaryStorage(){
      // console.log('temporaryStorage')
      // let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      // let value = generateForm.getValues();
      // const  editData = this.$parent.$parent.editData

      let param = {
        data:{
          id: this.flowInstanceId,
          currentNodeProxyId: this.flowNodeProxyId,
          auditRecord:{
            executeDesc: this.approveForm.approveMessage
          }
        },
        formDataMongoVo: { //暂存的时候不涉及选择分支，可以不存储mongo
        }
      }
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.storageFormData,
          param,
          async (res) => {
            if (res.isSuccess) {
              resolve()
            } else {
            }
          }
        );
      })
    },
    // 查询表单暂存数据
    queryStorageFormData(){
      let param = {
        data:{
          id: this.flowInstanceId,
          currentNodeProxyId: this.flowNodeProxyId,
        }
      }
      this.$axios.post(
        Api.schedule.queryStorageFormData,
        param,
        async (res) => {
          if (res.isSuccess) {
            if (res.data) {
              this.approveForm.approveMessage = res.data.auditDesc;
            }
          } else {
          }
        }
      );
    },
    // 新增事项（重新发起）
    saveNewEvent(){
      console.log('saveNewEvent-重新发起')
      // console.log('flowListSelect', this.$refs.flowListSelect.dataShowList)
      // console.log(1,this.$refs.flowListSelect)
      console.log('NewEvent',this.$refs.add_event_flow)
      let newEventVue = this.$refs.add_event_flow;

      // return;
      const reg = /^<p><br><\/p>$/;
      // const reg = /(<\/?.*?>)/gi;
      const contentText = newEventVue.$refs.richEditorRef.contentHtml.replace(reg, '');
      if (!contentText) {
        this.$message.error('请输入内容！');
        return;
      }
      if(!Object.keys(newEventVue.newParam).length){
        this.$message.error('请编辑流程！');
        return;
      }
      const content = newEventVue.$refs.richEditorRef.contentHtml;
      // richEditorRef在没有输入值时，还会返回空行标签，修复bug
      newEventVue.form.content = content.indexOf('<p><br></p>') == 0 && content.length == 11 ? '' : newEventVue.$refs.richEditorRef.contentHtml;
      console.log('newEventVue.$refs',newEventVue.$refs)
      newEventVue.form.flowList = newEventVue.$refs.flowListSelect.dataShowList;

      newEventVue.newParam.data.name = newEventVue.form.name; // 流程实例名称
      newEventVue.newParam.flowTemplateProtocol.data.flowName = '自定义直接流程';
      newEventVue.newParam.flowTemplateProtocol.data.formTemplateVo.name = newEventVue.form.name;
      newEventVue.newParam.flowTemplateProtocol.data.formTemplateVo.templateData = JSON.stringify(newEventVue.form);

      let find = newEventVue.typeList.find(x=>x.name.indexOf('新建事项')>-1);
      newEventVue.newParam.flowTemplateProtocol.data.typeId = find.id;
      console.log('NewEvent.newParam',newEventVue.newParam)
      // return;
      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.directInitiation,
        newEventVue.newParam,
        res => {
          this.loading = false;
          if (res.isSuccess) {
            this.$message.success('提交成功');
            this.$emit('fetchData')
            this.handleClose();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 自选节点
    chooseParallelNode(node) {
      console.log('chooseParallelNode-无表单',node)
      this.parallelNextNodeTemplateId = node.nextNodeTemplateId || node.id;
      this.nextNodeName = node.nodeName
      this.currentChooseNodeId = this.parallelNextNodeTemplateId
      this.nodeAuditScopeList = node?.nodeAuditScopeList || []
      this.nodeAuditType = node.nodeAuditType;
      this.countersignNum = node.countersignNum;
      // this.nodeChooseVisible = true;
      if (node.nodeAuditScopeList) { // 选了审批人范围，并配置人员
        this.nodeChooseVisible = true;
      } else { // 此处弹窗选择组织架构所有人，用于流程没有配置审批人范围人员，兼容以前数据
        this.oldNodeChooseVisible = true;
      }
    },
    // 发起人-手动选择分支-自选审批人后保存提交
    handleSaveBranchChooseNode() {

      if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        let nextNodeTemplateId = this.chooseBranchNode.nextNodeTemplateId
        let has = this.nextAuditorList.some(item=>{
          return item.nodeProxyId == nextNodeTemplateId
        })
        if(!has && this.chooseBranchNode.auditType == 'run_node_choose'){
          return this.$message.warning('该分支需选择审批人，请先选择');
        }
        this.saveBizData();
      } else {
        this.$message.warning('请选择流程分支');
      }
    },
    // 手动选择分支
    handleChooseBranchNode(node1, node2,nodeId,branch) {
      this.nextNodeName = node2
      this.currentChooseNodeId = nodeId
      this.nodeAuditScopeList = branch?.nodeAuditScopeList || []
      this.nodeAuditType = branch.nodeAuditType;
      this.countersignNum = branch.countersignNum;
      console.log('branch', branch)
      // this.nodeChooseVisible = true;
      if (branch.nodeAuditScopeList) { // 选了审批人范围，并配置人员
        this.nodeChooseVisible = true;
      } else { // 此处弹窗选择组织架构所有人，用于流程没有配置审批人范围人员，兼容以前数据
        this.oldNodeChooseVisible = true;
      }
    },
    handleChangeChooseBranch(branch) {
      this.manualChooseNodes.forEach(el=>{
        let nodeProxyId = el.nextNodeTemplateId
        this.nextAuditorList = this.nextAuditorList.filter(el=>el.nodeProxyId != nodeProxyId)
        this.chooseBranchList = this.chooseBranchList.filter(el=>el != nodeProxyId)
      })
      // this.chooseBranchNodeList = [];
      if(branch.auditType != 'run_node_choose' ){
        this.chooseBranchList.push(branch.nextNodeTemplateId)
      }
    },
    handleCloseParallelChoose() {
      //关闭并行弹框，取消并行已经选择的人
      this.parallelNodeChooseList.forEach(el=>{
        let nextNodeTemplateId = el.nextNodeTemplateId
        this.nextAuditorList = this.nextAuditorList.filter(item=>item.nodeProxyId != nextNodeTemplateId)
      })
      // this.parallelChooseNodes = []
      this.parallelNodeChooseList = []
      this.parallelChooseVisible = false;
    },
    handleCloseBranchChoose() {
      // manualChooseNodes
      this.chooseBranchNode = {};
      //取消所有选择的人员
      // this.nextAuditorList = []
      //关闭并行弹框，取消手动分支已经选择的人
      this.manualChooseNodes.forEach(el=>{
        let nextNodeTemplateId = el.nextNodeTemplateId
        this.nextAuditorList = this.nextAuditorList.filter(item=>item.nodeProxyId != nextNodeTemplateId)
      })
      //取消选择分支，这里去掉已经选择的分支信息
      this.chooseBranchList = []
      //所有手动分支置空
      this.manualChooseNodes = []
      this.branchChooseVisible = false;
    },
    confirmChooseExtendPerson(data){
      let obj = {
        nodeProxyId:data.nodeProxyId,
        bizId:data.id,
        name:data.name,
        auditDetailType:"personnel"
      }
      this.nextAuditorList = [obj]
      this.extendedVisible = false;
      this.saveBizData();
      return
    },
    //流程错误处理
    flowErrorHandle(res){
      if (!res.data) {
        this.$message.warning(res.message);
        return;
      }
      if (res.data) {
          if (res.data.errorType == 'custom_choose') {
            // 条件分支跟上手动，需要用户选择分支
            const branchNodes = res.data.branchNodes;
            this.manualChooseNodes = [];
            branchNodes.map((x, index) => {
              this.manualChooseNodes.push({
                nextNodeTemplateId: x.id,
                nodeName: x.nodeName,
                nodeType: x.type, // 为处理空节点
                branchName: '分支' + (index + 1),
                auditType: x.flowNodeAuditConfig.auditType,
                nodeAuditScopeList:x.flowNodeAuditConfig.nodeAuditScopeList,
                nodeAuditType: x.flowNodeAuditConfig.type,
                countersignNum: x.flowNodeAuditConfig.countersignNum,
              });
            });
            this.branchChooseVisible = true;
            this.tempManu = true;
          }else if(res.data.errorType == 'parallel_choose'){
            let hasChoose = false,parallelNodeChooseList = []
            res.data.branchNodes.forEach(parallelNode => {
              if (parallelNode.flowNodeAuditConfig.auditType == 'run_node_choose') {
                hasChoose = true;
                parallelNodeChooseList.push(
                  {
                    auditType: 'run_node_choose',
                    nodeName: parallelNode.nodeName,
                    nextNodeTemplateId: parallelNode.flowNodeAuditConfig.nodeTemplateId,
                    nodeAuditList: [],
                    nodeAuditScopeList: parallelNode.flowNodeAuditConfig.nodeAuditScopeList,
                    nodeAuditType: parallelNode.flowNodeAuditConfig.type,
                    countersignNum: parallelNode.flowNodeAuditConfig.countersignNum,
                  }
                );
              }
            })
            if(hasChoose){
              // this.parallelChooseNodes = parallelChooseNodes
              this.parallelNodeChooseList = parallelNodeChooseList
              this.parallelChooseVisible = true;
            }

          } else if(res.data.errorType == 'run_node_choose'){
            console.log('自选1')
            this.flowNodeType = 'run_node_choose';
            this.nextNodeProxyId = res?.data?.node?.id || ''
            this.nextNodeName = res?.data?.node?.nodeName || ''
            this.currentChooseNodeId = res?.data?.node?.id
            this.nodeAuditScopeList = res?.data?.node?.flowNodeAuditConfig?.nodeAuditScopeList || []
            this.nodeAuditType = res?.data?.node?.flowNodeAuditConfig?.type;
            this.countersignNum = res?.data?.node?.flowNodeAuditConfig?.countersignNum;
            if (res.data?.node) {
              if (res.data.node.flowNodeAuditConfig.nodeAuditScopeList) { // 选了审批人范围，并配置人员
                this.nodeChooseVisible = true;
              } else { // 此处弹窗选择组织架构所有人，用于流程没有配置审批人范围人员，兼容以前数据
                this.oldNodeChooseVisible = true;
              }
            } else { // 目前是流程配置了岗级，如果没有匹配到岗级会弹窗选择组织架构所有人
              this.oldNodeChooseVisible = true;
            }
          } else if(res.data.errorType == 'extended_attribute_users'){ //扩展属性不唯一，需要选择
            this.extendedPersonList = res.data?.fullUserInfoList || []
            this.extendedVisible = true
            this.nextNodeProxyId = res?.data?.nodeProxy?.id || ''
          }  else {
            this.flowNodeType = 'run_node_choose';
            this.$message({
              message: res.message,
              type: 'warning',
              customClass: 'errorMessage'
            });
            this.nextNodeProxyId = res?.data?.node?.id || ''
            this.nextNodeName = res?.data?.node?.nodeName || ''
            this.currentChooseNodeId = res?.data?.node?.id
            this.nodeChooseVisible = true;
          }
        }
    }
    // getSuperVisorOrLeader(nodeAuditType, nextNodeId) {
    //   // 获取发起人部门主管或分管副总id--重新发起或者提交审批
    //   const url =
    //     nodeAuditType == "department_supervisor"
    //       ? Api.schedule.getSupervisor
    //       : Api.schedule.getDeptLeader;
    //   this.$axios.post(
    //     url,
    //     {
    //       data: {
    //         id: this.initiatorId, // 发起人id
    //       },
    //     },
    //     (res) => {
    //       if (res.isSuccess) {
    //         var id = res?.data?.id || "";
    //         if (id) {
    //           const nextAuditor = {
    //             bizId: id,
    //           };
    //           if (nextNodeId) {
    //             // 手动分支-选择主管或副总类型节点
    //             nextAuditor.nodeProxyId = nextNodeId;
    //           }
    //           this.nextAuditorList = [nextAuditor];
    //           // if (this.isReInitiate) {
    //           // 重新发起
    //           this.saveBizData();
    //           // } else {
    //           //   this.handleSubmitCheck('pass', true);
    //           // }
    //         } else {
    //           //没有主管，降级，用户自选审批节点
    //           if (this.nextAuditorList.length) {
    //             this.saveBizData();
    //           } else {
    //             this.nodeChooseVisible = true;
    //           }
    //           // if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
    //           //   this.nodeChooseVisible = true;
    //           //   return;
    //           // }else{
    //           //     // 重新发起
    //           //     this.saveBizData();
    //           // }
    //         }
    //       }
    //     }
    //   );
    // },
    // getSuperVisorOrLeaderId(nodeAuditType) {
    //   // 并行分支获取副总和主管的id作为下一审批人传参
    //   return new Promise((resolve, reject) => {
    //     const url =
    //       nodeAuditType == "department_supervisor"
    //         ? Api.schedule.getSupervisor
    //         : Api.schedule.getDeptLeader;
    //     this.$axios.post(
    //       url,
    //       {
    //         data: {
    //           id: this.initiatorId, // 发起人id
    //         },
    //       },
    //       (res) => {
    //         if (res.isSuccess) {
    //           resolve(res.data.id);
    //         }
    //       }
    //     );
    //   });
    // },
  },
};
</script>
<style lang="scss" scoped>
.errorMessage {
  z-index: 3000 !important;
}

.radio-choose-item {
  display: inline-block;
  width: 460px;
  margin: 5px 0;
  white-space: pre-wrap;
  line-height: 20px;
}

.button-bottom-div {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;

  >button {
    margin-bottom: 3px;
    margin-left: 5px;
  }
}

.opinion-toolbar {
  position: absolute;
  top: 15px;
  right: 0;
  z-index: 20;
  display: flex;
  align-items: center;
}

.opinion-toolbar__checkbox {
  margin-left: 10px;
}

::v-deep {
  .el-button--danger {
    background-color: #dc0000;
    border-color: #dc0000;
  }
}
</style>
