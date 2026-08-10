<!--
 * @Descripttion: 审批意见
 * @Author: zhengzetao
 * @Date: 2022-08-17
-->

<template>
  <div style="position: relative">
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
    <el-form
      v-if="isExamine"
      :model="approveForm"
      :rules="rules"
      ref="approveForm"
    >
      <el-form-item label="审批意见">
        <el-input
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 7 }"
          maxlength="200"
          show-word-limit
          v-model="approveForm.approveMessage"
          placeholder="请填写审批意见"
        ></el-input>
      </el-form-item>
    </el-form>
    <div class="button-bottom-div">
      <template v-if="isExamine">
        <!-- <el-button class="print-btn" style="margin-left:10px;" type="primary" @click="testOut">测试</el-button>
        <el-button class="print-btn" style="margin-left:10px;" type="primary" @click="testOut2">测试2</el-button> -->
        <el-button
          type="success"
          @click="submitCheckBefore('pass')"
        >同意</el-button>
        <el-button
          v-if="!isTranspondFlow"
          type="warning"
          @click="clickRollBack()"
        >回退上一节点</el-button>
        <el-button
          v-if="!isTranspondFlow"
          type="danger"
          @click="submitCheckBefore('no_pass')"
        >不同意</el-button>
        <el-button
          v-if="!isTranspondFlow"
          type="primary" plain
          @click="temporaryStorage"
        >暂存</el-button>
        <el-button
          v-if="!isTranspondFlow"
          type="success"
          plain
          @click="addCounterSign"
        >加签</el-button>
        <el-popover
          placement="top"
          width="200"
          trigger="click"
          style="margin-left: 3px;"
          >
          <h3>请选择以下打印内容</h3>
          <el-checkbox-group v-model="printCheckList" style="margin: 10px 0px;">
            <el-checkbox label="表单" disabled></el-checkbox>
            <el-checkbox label="发起人附言"></el-checkbox>
            <el-checkbox label="流程日志"></el-checkbox>
          </el-checkbox-group>
          <!-- 尝试新插件2025.1.2 -->
          <el-button class="print-btn" type="primary" style="float:right;" @click="handlePrint">确 认</el-button>
          <!-- 以前用的插件 -->
          <!-- <el-button class="print-btn" v-print="'#formContainer'" style="float:right;" type="primary" @click="handlePrint">确 认</el-button> -->
          <el-button slot="reference" type="primary" class="to_print_button">打 印</el-button>
        </el-popover>
      </template>
      <template v-else-if="isReInitiate">
        <!-- 待发流程--重新发起 -->
        <el-button class="save_draft_button" type="primary" @click="saveSubmit('save')">保存草稿</el-button>
        <el-button type="primary" @click="saveSubmit('submit')" >提交</el-button>
      </template>
      <template v-else>
        <!-- 第一种写法 -->
        <!-- <el-button v-print="printViewInfo" style="margin-left:10px;" type="primary">打 印</el-button> -->
        <!-- 第二种写法 -->
        <!-- <el-button class="print-btn" style="margin-left:10px;" type="primary"
          v-print="'#formContainer'" @click="handlePrint">打 印</el-button> -->
        <el-button
          :type="outTracking ? 'warning' : 'primary'"
          plain
          @click="setTracking"
          style="margin-right: 5px;"
          v-if="!isTimeoutPage && !hideTrackingButton"
        >{{ outTracking ? '取消跟踪':'设为跟踪' }}</el-button>
        <el-popover
          placement="top"
          width="200"
          trigger="click"
          >
          <h3>请选择以下打印内容</h3>
          <el-checkbox-group v-model="printCheckList" style="margin: 10px 0px;">
            <el-checkbox label="表单" disabled></el-checkbox>
            <el-checkbox label="发起人附言"></el-checkbox>
            <el-checkbox label="流程日志"></el-checkbox>
          </el-checkbox-group>
          <!-- 尝试新插件2025.1.2 -->
          <el-button class="print-btn" type="primary" style="float:right;" @click="handlePrint">确 认</el-button>
          <!-- 以前用的插件 -->
          <!-- <el-button class="print-btn" v-print="'#formContainer'" style="float:right;" type="primary" @click="handlePrint">确 认</el-button> -->

          <!-- 下面是12.31之前的 -->
          <!-- <button ref="simulateClick" v-print="'#formContainer'" style="visibility: hidden;">模拟点击</button>
          <el-button class="print-btn" style="float:right;" type="primary" @click="handlePrint">确 认</el-button> -->
          <!-- v-print="'#formContainer'" -->
          <el-button slot="reference" type="primary" class="to_print_button">打 印</el-button>
        </el-popover>

        <el-button
          type="primary"
          plain
          @click="closeCheck"
        >关闭</el-button>
      </template>
    </div>

    <!-- 新版审批人自选，老版因为流程配置时节点配置了公司权限。新版只能选全集团的人 -->
    <PersonSelectDialog
      v-if="oldNodeChooseVisible"
      :visible.sync="oldNodeChooseVisible"
      @getSelectPerson="getSelectPerson"
      :nextNodeName="nextNodeName"
      :nextNodeProxyId="currentChooseNodeId"
      :nodeAuditType="nodeAuditType"
      :countersignNum="countersignNum"
      @contractPayClose="contractPayClose"
    />
    <!-- 2025.1.2审批人自选更换成审批人范围选择 -->
    <RangePersonSelect :visible.sync="nodeChooseVisible" v-if="nodeChooseVisible" @getSelectPerson="getSelectPerson"
    :nextNodeName="nextNodeName" :nextNodeProxyId="currentChooseNodeId" :nodeAuditScopeList="nodeAuditScopeList"
    :nodeAuditType="nodeAuditType" :countersignNum="countersignNum" @contractPayClose="contractPayClose"/>

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
     :flowNodeProxyId="flowNodeProxyId" :flowProxyId="flowProxyId" :flowInstanceId="flowInstanceId" :isNoForm="false"
      :tFlowNodeProxyId="tFlowNodeProxyId" :tFlowProxyId="tFlowProxyId" @confirm="confirmAddCounterSign"></AddCounterSign>
      <!-- :outFlowNodeTemplate="outFlowNodeTemplate" -->
  </div>
</template>

<script>
console.log('====')
console.log(111,window.$vue)
var abc = window.$vue;
import Api from '@/api';
import { resolve } from 'path';
// import NextNodeDialog from './nextNodeDialog.vue';
import PersonSelectDialog from './PersonSelectDialog.vue';
import RangePersonSelect from './RangePersonSelect.vue';
import { localstorageGet } from '@/utils/auth';
import mixin from './mixin.js'
import { deepClone } from "@/utils";
import extendedAttributeUsers from "@/components/extendedAttributeUsers.vue"
import AddCounterSign from './AddCounterSign.vue';
import moment from 'moment';
import ApprovalOpinionPhraseDrawer from './ApprovalOpinionPhraseDrawer.vue';

export default {
  name: 'ExamineOpinion',
  mixins:[mixin],
  components: { PersonSelectDialog,RangePersonSelect,extendedAttributeUsers,AddCounterSign,ApprovalOpinionPhraseDrawer },
  data() {
    let self = this;
    return {
      // printViewInfo: {
      //   id: "formContainer", //打印区域的唯一的id属性
      //   popTitle: '配置页眉标题', // 页眉文字 （不设置时显示undifined）（页眉页脚可以在打印页面的更多设置的选项中取消勾选）
      //   extraHead: '打印，印刷', // 最左上方的头部文字，附加在head标签上的额外标签，使用逗号分割
      //   preview: false, // 是否启动预览模式，默认是false （开启预览模式ture会占满整个屏幕，不建议开启，除非业务需要）
      //   previewTitle: '预览的标题', // 打印预览的标题(预览模式preview为true时才显示)
      //   previewPrintBtnLabel: '预览结束，开始打印', // 打印预览的标题下方的按钮文本，点击可进入打印(预览模式preview为true时才显示)
      //   zIndex: 20002, // 预览窗口的z-index，默认是20002，最好比默认值更高
      //   previewBeforeOpenCallback (that) { console.log('正在加载预览窗口！'); console.log(that.msg, this) }, // 预览窗口打开之前的callback (预览模式preview为true时才执行) （that可以取到data里的变量值）
      //   previewOpenCallback () { console.log('已经加载完预览窗口，预览打开了！') }, // 预览窗口打开时的callback (预览模式preview为true时才执行)
      //   beforeOpenCallback () { console.log('开始打印之前！') }, // 开始打印之前的callback
      //   openCallback () { console.log('执行打印了！') }, // 调用打印时的callback
      //   closeCallback () { console.log('关闭了打印工具！') }, // 关闭打印的callback(无法区分确认or取消)
      //   clickMounted () { console.log('点击v-print绑定的按钮了！') },
      //   // url: 'http://localhost:8080/', // 打印指定的URL，确保同源策略相同
      //   // asyncUrl (reslove) {
      //   //   setTimeout(() => {
      //   //     reslove('http://localhost:8080/')
      //   //   }, 2000)
      //   // },
      //   standard: '',
      //   extarCss: ''
      // },

      printCheckList:['表单'],
      operaFile:false,
      isDraftInReInitiateStatus:false,
      approveForm: {
        approveMessage: '' // 审批信息
      },
      rules: {
        approveMessage: [
          { required: true, message: '请填写审批意见', trigger: 'blur' }
        ]
      },
      nextNodeName:'',
      nodeChooseVisible: false,
      oldNodeChooseVisible: false,
      parallelChooseVisible: false,
      parallelNextNodeTemplateId: '',
      checkboxRersonGroup: [],
      nextAuditorList: [],
      parallelNodeChooseList: [],
      branchChooseVisible: false,
      chooseBranchNode: {},
      chooseBranchNodeList: [],
      manualChooseNodes: [],
      hasUpdate: false,
      clickMethod:  'submit', // submit , draft
      submitLoading:  false,
      noSubmitLoading:  false,
      paralleNodeName:  '',
      pageNextNodeProxyId:'',
      chooseBranchList:[],
      currentChooseNodeId:'',

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
      // hasFirstAddCounterSign:false
    };
  },
  props: {
    id: { // 流程id
      type: String,
      default: ''
    },
    formId: { // 流程id
      type: String,
      default: ''
    },
    businessId: { // 业务id（如果有。）
      type: String,
      default: ''
    },
    isExamine: {
      type: Boolean,
      default: true
    },
    lastCountersignFlag: {
      type: Boolean,
      default: true
    },
    isReInitiate: {
      type: Boolean,
      default: false
    },
    initiatorId: {
      type: String,
      default: ''
    },
    jobTaskId: {
      type: String,
      default: ''
    },
    flowNodeType: {
      type: String,
      default: ''
    },
    // nextNodeName: {
    //   type: String,
    //   default: ''
    // },
    currentPendingNodeName: {
      type: String,
      default: ''
    },
    nextNodeProxyId: {
      type: String,
      default: ''
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
    flowName:  {
      type:  String,
      default:  ''
    },
    flowProxyId:  {
      type:  String,
      default:  ''
    },
    selectFlowType: { // 表单类型
      type: String,
      default: ''
    },
    auditPassLogicFlag: { // 并行节点是否最后一个人的审核
      type: Boolean,
      default: true
    },
    flowInstanceBizRelevanceList:{
      type: Array,
      default:function(){
        return [];
      }
    },
    // outFlowNodeTemplate:{
    //   type: Object,
    //   default:function(){
    //     return {};
    //   }
    // },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    isTranspondFlow: {
      type: Boolean,
      default: false
    },
    outTracking:{
      type:Boolean,
      default:false
    },
    hideTrackingButton: {
      type: Boolean,
      default: false
    },
    isTimeoutPage:{ //是否是超时跳过的页面
      type:Boolean,
      default:false
    }
  },
  computed: {},
  watch: {
    outTracking:{
      handler(val){
        this.tracking = this.normalizeTrackingValue(val)
      },
      immediate: true
    },
    oldNodeChooseVisible:{
      handler(val){
        if(!val){
          //手动分支取消选人，把节点id去掉
          let index = this.chooseBranchList.indexOf(el=>el == this.currentChooseNodeId)
          if(index > -1)this.chooseBranchList.splice(index,1)
        }
      }
    },
    nodeChooseVisible:{
      handler(val){
        if(!val){
          //手动分支取消选人，把节点id去掉
          let index = this.chooseBranchList.indexOf(el=>el == this.currentChooseNodeId)
          if(index > -1)this.chooseBranchList.splice(index,1)
        }
      }
    }
  },
  created() {
    console.log('this.flowProxyId122',this.flowProxyId)
    // 4.2因为发版原因暂时注释
    this.queryStorageFormData();

    this.parallelNodeChooseList = [];
    this.manualChooseNodes = [];
    // this.parallelNodeChooseList = this.$attrs.parallelNodeChooseList;
    // this.manualChooseNodes = this.$attrs.manualChooseNodes;
  },
  mounted() {
    this.pageNextNodeProxyId = this.nextNodeProxyId
    console.log('currentPendingNodeName2',this.currentPendingNodeName)
  },
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
    // testOut(){
    //   this.$emit('updateList')
    // },
    // testOut2(){
    //   console.log('testOut2',this.flowNodeProxyId)
    // },
    // handleClick(){
    //   console.log('我被点击')
    // },
    confirmAddCounterSign(data){
      this.tFlowNodeProxyId = data.flowNodeProxyId;
      this.tFlowProxyId = data.flowProxyId;
      console.log('tFlowNodeProxyId1',this.tFlowNodeProxyId)
      console.log('tFlowProxyId1',this.tFlowProxyId)
      this.$emit('updateList')

      // 加签bug调试暂时注释
      // 通知父组件重新加载数据
      // if (this.$parent && typeof this.$parent.forceRefresh === 'function') {
      //   this.$parent.forceRefresh();
      // }
      // // 加签bug调试暂时注释
      // // 通知Backlog组件更新数据
      // if (this.$parent && typeof this.$parent.$parent.fetchData === 'function') {
      //   // 延迟执行，确保加签操作完成
      //   setTimeout(() => {
      //     this.$parent.$parent.fetchData();
      //   }, 1000);
      // }
    },
    // 加签
    addCounterSign(){
      this.addCounterSignVisible = true;
    },
    // 查询表单暂存数据
    queryStorageFormData(){
      console.log('queryStorageFormData')
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
    contractPayClose(){
      console.log('close1');
      this.$emit('contractPayClose')
    },
    handlePrint(){
      this.$emit('handlePrint', this.printCheckList)
      // setTimeout(()=>{
      //   this.$refs.simulateClick.click();
      // },1000)

      this.$emit('print',true)
      setTimeout(()=>{
        this.$emit('print',false)
      },1000)
    },
    // 暂存实例
    async temporaryStorage(){
      console.log('temporaryStorage')
      let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      let value = generateForm.getValues();
      const  editData = this.$parent.$parent.editData

      let param = {
        data:{
          id: this.flowInstanceId,
          currentNodeProxyId: this.flowNodeProxyId,
          auditRecord:{
            executeDesc: this.approveForm.approveMessage
          }
        },
        formDataMongoVo: {
          data: Object.assign(editData,value)
        }
      }
      try {
        await generateForm.triggerEvent('beforeSubmitAndDraft', { param, temporary: true, flowDialog: this });
      } catch (error) {
        if (typeof error === "string") { this.$message.error(error); }
        return;
      }
      this.$axios.post(
        Api.schedule.storageFormData,
        param,
        async (res) => {
          if (res.isSuccess) {
            this.commonAction('暂存');
          } else {
          }
        }
      );
    },
    closeCheck() {
      this.approveForm.approveMessage = '';
      this.$emit('postExamine');
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
        if (this.isReInitiate) {
          // 重新发起
          // this.handleRePostSubmit(true);
          this.reSubmitFormMakingFormBusiness(true);
        } else {
          // this.handleSubmitCheck('pass', true);
          this.formMakingFormBusiness(true,  'pass')
        }
      }
      return
    },
    // 并行节点审批人处理逻辑-提交
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
      if (this.isReInitiate) {
        // 重新发起
        // this.handleRePostSubmit(true);
        this.reSubmitFormMakingFormBusiness(true);
      } else {
        // this.handleSubmitCheck('pass', true);
        this.formMakingFormBusiness(true,  'pass')
      }
      return
    },
    // 并行-自选节点
    chooseParallelNode(node) {
      console.log('chooseParallelNode',node)
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
    // 审批-手动分支选择后-点击提交
    handleSaveBranchChooseNode() {
      if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        let nextNodeTemplateId = this.chooseBranchNode.nextNodeTemplateId
        let has = this.nextAuditorList.some(item=>{
          return item.nodeProxyId == nextNodeTemplateId
        })
        if(!has && this.chooseBranchNode.auditType == 'run_node_choose'){
          return this.$message.warning('该分支需选择审批人，请先选择');
        }
        if (this.isReInitiate) {
          // 重新发起
          this.reSubmitFormMakingFormBusiness(true);
        } else {
          this.formMakingFormBusiness(true,  'pass')
        }
      } else {
        this.$message.warning('请选择流程分支');
      }
    },
    // 手动选择分支
    handleChooseBranchNode(node1, node2,nodeId,branch) {
      // this.paralleNodeName = this.nextNodeName + node1 + '-' + node2
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
      // if (branch.auditType != 'run_node_choose') {
      //   this.chooseBranchNodeList = [];
      // }
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
    // 点击审批：同意或不同意
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
            if (type == 'pass') {
              if (this.isReInitiate) {
                // 重新发起
                // this.handleRePostSubmit(true);
                this.reSubmitFormMakingFormBusiness(true);
              } else {
                // this.handleSubmitCheck('pass', true);
                this.formMakingFormBusiness(true,  'pass')
              }
            } else {
              // this.handleSubmitCheck(type);
              this.formMakingFormBusiness(null,  type)
            }
          } else {
            this.$message.error('请填写审批意见');
            return false;
          }
        });
      }).catch(()=>{})
    },
    // 业务id批量关联文件id
    saveFile(bizId,fileList){
      this.$axios.post(
        '/web/file/api/relationFile/saveBatch',
        {
          data: {
            relationId: bizId,
            fileIds: fileList
          }
        },
        (res) => {
          if (res.isSuccess) {
          } else {
          }
        }
      );
    },
    // 新增或删除文件
    addNewOrDeleteFile(allValues,bizId){
      let fileList = allValues.uploadFile.map(x => {return x.data.id});
      console.log('addNewOrDeleteFile-fileList',fileList)
      console.log('this.oldFileList',this.oldFileList)
      let compareObj = this.compareArrays(this.oldFileList,fileList);
      if (compareObj.added.length) {
        this.saveFile(bizId,compareObj.added);
      }
      if (compareObj.removed.length) {
        this.deleteFile(bizId,compareObj.removed);
      }
    },
    // 对比两个数组新增或减少
    compareArrays(a, b) {
      let added = b.filter(item => !a.includes(item));
      let removed = a.filter(item => !b.includes(item));
      return { added, removed };
    },
    // 批量删除业务id和文件id关联关系
    deleteFile(relationId,fileIds){
      this.$axios.post(
        '/web/file/api/relationFile/deleteByRelationIdAndFileIds',
        {
          data: {
            relationId:relationId,
            fileIds: fileIds
          }
        }
      );
    },
    // 统一处理有表单、有业务的逻辑（目前只有合规评审和盖章评审）
    dealFormBusiness(func,flag,type){
      if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性表单业务保存
        let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
        let custom_contractLegalField = generateForm.getComponent('custom_contractLegalField') // 合规性评审表中自定义的合同文件列表
        custom_contractLegalField.contractBusiness(generateForm,  this,  flag,  type)
      } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审表单业务保存
        let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
        let custom_contractSealField = generateForm.getComponent('custom_contractSealField') // 合同盖章评审表单中自定义的组件
        custom_contractSealField.contractBusiness(generateForm,  this,  flag,  type)
      } else {
        func(flag,  type);
      }
    },
    // 重新发起时的保存草稿
    reHandleSaveDraft(flag = false,  type)  {
      console.log('reHandleSaveDraft')
      this.dealFormBusiness(this.handleSaveDraft,flag,type)
      // if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractLegalField = generateForm.getComponent('custom_contractLegalField') // 合规性评审表中自定义的合同文件列表
      //   custom_contractLegalField.contractBusiness(generateForm,this,flag,type)
      // } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractSealField = generateForm.getComponent('custom_contractSealField') // 合同盖章评审表单中自定义的组件
      //   custom_contractSealField.contractBusiness(generateForm,this,flag,type)
      // } else {
      //   this.handleSaveDraft();
      // }
    },
    // 最终重新发起
    reSubmitFormMakingFormBusiness(flag,type){ // 有表单业务,最终提交接口前先调用业务接口
      this.submitLoading = true
      console.log('reSubmitFormMakingFormBusiness')
      this.dealFormBusiness(this.handleRePostSubmit,flag,type)
      // if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractLegalField = generateForm.getComponent('custom_contractLegalField') // 合规性评审表中自定义的合同文件列表
      //   custom_contractLegalField.contractBusiness(generateForm,this,flag,type)
      // } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractSealField = generateForm.getComponent('custom_contractSealField') // 合同盖章评审表单中自定义的组件
      //   custom_contractSealField.contractBusiness(generateForm,this,flag,type)
      // } else {
      //   this.handleRePostSubmit(flag);
      // }
    },
    // 最终审批(通过或者不通过)
    formMakingFormBusiness(flag,type){ // 有表单业务,最终提交接口前先调用业务接口
      console.log('formMakingFormBusiness')
      this.dealFormBusiness(this.handleSubmitCheck,flag,type)
      // if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractLegalField = generateForm.getComponent('custom_contractLegalField') // 合规性评审表中自定义的合同文件列表
      //   custom_contractLegalField.contractBusiness(generateForm,this,flag,type)
      // } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审表单业务保存
      //   let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      //   let custom_contractSealField = generateForm.getComponent('custom_contractSealField') //  合同盖章评审表单中自定义的组件
      //   custom_contractSealField.contractBusiness(generateForm,this,flag,type)
      // }  else {
      //   this.handleSubmitCheck(flag,type);
      // }
    },
    // 审批通过或者不通过
    handleSubmitCheck(flag,type) {
      console.log('=====handleSubmitCheck======')
      // flag----------true:审批人自选   false:指定节点
      let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      const  editData = this.$parent.$parent.editData
      this.clickMethod = 'submit'
      // console.log(1,editData)
      // console.log(1,generateForm.getValue('hide_contractId'))

      let validateType = type == 'pass' ? true : false; // 审核同意需要校验，不同意不用校验
      generateForm.getData(validateType).then(async x => { // 24.12.25改为不用getData返回的value，使用getValues获取(原因要获取表单虚拟字段)。getData只做校验
        let value = generateForm.getValues();

        if (validateType) { // 只有同意才进行下面功能
          if (this.selectFlowType == 'publication_commission') { // 出版委托单。项目经理审批更新日期
            if (JSON.parse(value['manageUserName']).id == localstorageGet('userId')) {
              value.manageUserDate = moment().format('YYYY-MM-DD');
            }
          }
          //年终考核单需要把意见写入签名区域
          if(this.selectFlowType == 'staff_annual_performance' || this.selectFlowType == 'staff_annual_assessment'){

            value.opinion = this.approveForm.approveMessage
          }
          console.log('this.selectFlowType',this.selectFlowType)
          if (this.selectFlowType == 'profession_indirect_provide') { // 专业提资单。提资专业负责人、接收专业负责人更新日期
            if (JSON.parse(value['proposeLeaderUserName']).id == localstorageGet('userId')) {
              value.proposeLeaderDate = moment().format('YYYY-MM-DD');
            }
            console.log(111,JSON.parse(value['receiveLeaderUserName']).id)
            console.log(222,localstorageGet('userId'))
            if (JSON.parse(value['receiveLeaderUserName']).id == localstorageGet('userId')) {
              value.receiveLeaderDate = moment().format('YYYY-MM-DD');
            }
          }
        }
        console.log('handleSubmitCheck-value',value);
        // return;
        if (this.selectFlowType == 'travel_expense') { // 差旅报销单
          const { expenseBudgetList_total, expenseInAccountInfoList_total, travelPersonnelVoList_total, expenseDetailList_total } = value
          let total = [expenseBudgetList_total, expenseInAccountInfoList_total, travelPersonnelVoList_total, expenseDetailList_total]
          total = Array.from(new Set(total))
          if (total.length > 1) {
            this.submitLoading = false
            return this.$message.error('请检查费用明细、入账信息、费用预算和出差人员合计金额是否相同')
          }
          if (value.expenseBudgetList.length == 0) {
            this.submitLoading = false
            return this.$message.error('请添加费用预算类型！')
          }
        } else if (this.selectFlowType == 'expense_loan') {// 借款单
          if (value.applicationFundsVo_payMoney != value.expenseInAccountInfoList[0].money) {
            this.submitLoading = false
            return this.$message.error('请检查借款金额和入账信息金额是否相同！')
          }
        } else if (this.selectFlowType == 'request_funds') {// 请款单
          if (value.applicationFundsVo_type == '2' && (value.applicationFundsVo_payMoney != value.totalMoney)) {
            this.submitLoading = false
            return this.$message.error('请检查请款金额和费用预算合计金额是否相同！')
          }
          if (value.applicationFundsVo_type == '2' && value.expenseBudgetList.length == 0) {
            this.submitLoading = false
            return this.$message.error('请添加费用预算类型！')
          }
        } else if (this.selectFlowType == 'expense_repayment') {//还款单
          if (value.applicationFundsVo_payMoney == 0) {
            this.submitLoading = false
            return this.$message.error('请检查关联借款单还款金额合计金额是否是大于0！')
          }
        }

        if (validateType) { // 只有同意才进行下面功能
          if (this.selectFlowType == 'publication_commission') { // 出版委托单。项目经理审批更新日期
            if (JSON.parse(value['manageUserName']).id == localstorageGet('userId')) {
              value.manageUserDate = moment().format('YYYY-MM-DD');
            }
          }
          console.log('this.selectFlowType',this.selectFlowType)
          if (this.selectFlowType == 'profession_indirect_provide') { // 专业提资单。提资专业负责人、接收专业负责人更新日期
            if (JSON.parse(value['proposeLeaderUserName']).id == localstorageGet('userId')) {
              value.proposeLeaderDate = moment().format('YYYY-MM-DD');
            }
            console.log(111,JSON.parse(value['receiveLeaderUserName']).id)
            console.log(222,localstorageGet('userId'))
            if (JSON.parse(value['receiveLeaderUserName']).id == localstorageGet('userId')) {
              value.receiveLeaderDate = moment().format('YYYY-MM-DD');
            }
          }
        }
        console.log('handleSubmitCheck-value',value);
        // return;
        const param = {
          data: {
            name: this.getBackFlowName(value),
            jobTaskId: this.jobTaskId,
            auditRecord: {
              auditStatus: type || '',
              executeDesc: this.approveForm.approveMessage
            },
            id: this.flowInstanceId
          },
          formDataMongoVo: {
            data: Object.assign(editData,value)
          },
          tracking:this.tracking
        };
        if(!this.getBackFlowName(value)){
          delete param.data.name
        }
        // if (this.auditPassLogicFlag && flag) {
        if (flag) {
            // param.nextAuditorList = this.nextAuditorList;
          param.nextAuditorList = this.nextAuditorList.map(item=>{
            item.auditDetailType = "personnel"
            if(item.nodeProxyId === undefined)item.nodeProxyId = this.pageNextNodeProxyId
            return item
          });
        }
        // //如果下一个节点是空节点，则需要删除下一个节点的id
        // if (this.chooseBranchNode.nodeType == 'empty') {

        //   param.nextAuditorList = this.nextAuditorList.map(item => {
        //     return {
        //       auditDetailType: "personnel",
        //       name:item.name,
        //       bizId: item.bizId
        //     }
        //   })
        // }
        // if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        //   // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
        //   if(!param.nextAuditorList.length){
        //     param.nextAuditorList = [
        //       {nodeProxyId:this.chooseBranchNode.nextNodeTemplateId}
        //     ]
        //   }
        // }
        if (this.chooseBranchList && this.chooseBranchList.length) {
          // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
          //chooseBranchList去重
          this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
          this.chooseBranchList.forEach(item=>{
            param.nextAuditorList.push({nodeProxyId:item})
          })
        }
        if(type == 'pass')this.submitLoading = true
        if(type == 'no_pass')this.noSubmitLoading = true
        let beforeSubmitAndDraftData = { flowDialog: this, clickMethod: this.clickMethod, param }
        if (this.businessId) beforeSubmitAndDraftData.businessId = this.businessId
        if(type == 'pass'){
          let submitDataResult = await generateForm.triggerEvent('beforeSubmitAndDraft', beforeSubmitAndDraftData); //同意的时候才去保存信息
          if(submitDataResult === false){
            this.submitLoading = false
            this.noSubmitLoading = false
            return
          }
        }
        // console.log('submitDataResult',submitDataResult[0])
        if(this.operaFile){ // 表单需要操作文件（只针对一个文件上传控件，多个文件上传控件未考虑）
          this.addNewOrDeleteFile(value,this.businessId)
        }
        // ---不用前端配置表单人员，后台直接取
        this.$emit('getFormPersonValue', param);
        await generateForm.triggerEvent('beforeSubmitAndDraftNoBiz', beforeSubmitAndDraftData); // 流程提交前执行无业务方法

        if(this.isTranspondFlow){ // 如果是转发数据，直接删除名称
          delete param.data.name
        }

        // 关联公共流程--流程审批目前后端没有做flowInstanceBizRelevanceList字段传参，不生效
        let flowInstanceBizRelevanceList = deepClone(this.flowInstanceBizRelevanceList);
        let newInstanceBizWrap = flowInstanceBizRelevanceList.filter(x=>x.otherBiz != 'commonFlow').map(y=>{return{otherBiz:y.otherBiz,otherBizId:y.otherBizId}});
        let commonFlow = this.$parent.$parent.$refs?.flowListSelect || this.$parent.$refs?.flowListSelect;
        console.log('commonFlow-审批流程',commonFlow)
        if (commonFlow && commonFlow.dataShowList.length) {
          commonFlow.dataShowList.forEach(item => {
            newInstanceBizWrap.push({
              otherBiz: 'commonFlow',
              otherBizId: item.id // 流程id
            })
          })
          param.data.flowInstanceBizRelevanceList = newInstanceBizWrap;
        }

        if((this.selectFlowType == 'cost_funds_transactions' || this.selectFlowType == 'cost_funds_invest') && !this.isTranspondFlow){
          const formData = param.formDataMongoVo && param.formDataMongoVo.data ? param.formDataMongoVo.data : {};
          const payCompanyName = formData['payCompanyName'] || formData['applicationFundsVo_payCompanyName'] || '';
          const expenseUserName = formData['expenseUserName'] || localstorageGet('userName');
          const payMoney = formData['payMoney'] || formData['applicationFundsVo_payMoney'] || '';
          if(payMoney !== ''){
            param.data.name = `${this.flowName}-${payCompanyName}-${expenseUserName}-${payMoney}元`;
          }
        }
        // return;

        this.$axios.post(
          Api.qualityManage.submitTask,
          param,
          (res) => {
            this.submitLoading = false
            this.noSubmitLoading = false
            if (res.isSuccess) {
              //if(this.$parent.$children[0]){
                // 存档管理流程审核修改文件状态
             // const attachList = this.$parent.$children[0].attachList||[]
              //console.log(attachList,'++++')
              //attachList.map(item=>{
              //  const file = item.data
              //  if(type=='no_pass'){
               //   file.fileStatus = '3'
               // }else{
                //  file.fileStatus = '1'
               // }
               // this.updateFileStatus({id:file.id,fileName:file.fileName.split('.')[0],fileStatus:file.fileStatus})
              //})
             // }
              //绑定文件到对应的节点，如果有
              this.$emit('bindFile',res?.data?.auditRecord)
              this.afterSaveDeal()
            } else {
              this.flowErrorHandle(res)
            }
          }
        );
      }).catch(e => {
        if (typeof e == 'string') this.$message.error(e); // this.$message.error('请检查表单是否填写完毕')
      }).finally(() => {
        this.submitLoading = false;
        this.noSubmitLoading = false;
      });
    },
    updateFileStatus(data){
      this.$axios.post(
        '/web/file/api/file/editFile',
        {
          data
        },
        (res) => {

        }
      );
    },
    // 点击重新发起或保存草稿
    saveSubmit(type) {
      // type=='save' 保存草稿  type=='submit'重新提交待发流程
      this.clickMethod = type =='save' ? 'draft' : 'submit';

      if (type == 'submit') {
        // 重新发起-需要判断下一节点类型
        // if (this.parallelNodeChooseList && this.parallelNodeChooseList.length > 0) {
        //   // 并行节点中的审批人自选、主管、副总 类型的审批节点被传过来了
        //   // 判断并行节点中是否包含审批人自选
        //   const flag = this.parallelNodeChooseList.some(item => item.auditType == 'run_node_choose');
        //   if (flag) {
        //     this.parallelChooseVisible = true;
        //   } else {
        //     this.handleSaveParallelChooseNode();
        //   }
        //   return false;
        // } else if (this.manualChooseNodes && this.manualChooseNodes.length && this.auditPassLogicFlag) {
        //   // 手动分支选择分支
        //   this.branchChooseVisible = true;
        //   return false;
        // } else {
        //   // 判断是否是审批人自选节点--普通节点
        //   this.checkIsOptional();
        // }
        if (this.isReInitiate) {
          // 重新发起
          // this.handleRePostSubmit(true);
          this.reSubmitFormMakingFormBusiness(true);
        } else {
          // this.handleSubmitCheck('pass', true);
          this.formMakingFormBusiness(true,  'pass')
        }
      } else if (type == 'save') {
        this.isDraftInReInitiateStatus = true;
        // 保存草稿
        this.reHandleSaveDraft();
        // this.handleSaveDraft();
      }
    },
    // 流程名称
    getBackFlowName(value){
      console.log('this.isTranspondFlow',this.isTranspondFlow)
      console.log(this.selectFlowType,'getBackFlowName',value)
      let name = '';
      // if (!this.isTranspondFlow) { // 如果是转发的流程，不需要修改流程名
        if(this.selectFlowType =='request_funds'||this.selectFlowType =='expense_loan'||this.selectFlowType =='expense_repayment'){ // 请款单  借款单 还款单
          name = this.flowName + '-'+ value['expenseUserName']+ '-' + value['applicationFundsVo_payMoney'] + '元'
        }else if(this.selectFlowType =='travel_expense'){ // 差旅报销
          name = this.flowName+ '-' + value['expenseUserName'] + '-' + value['expenseInAccountInfoList_total'] + '元'
        }else if (this.selectFlowType == 'contract_compliance_review' || this.selectFlowType == 'contract_seal_review') { // 合规和盖章评审
          let contract = value['contractNumber'] + value['contractName'] + '-' + value['contractSum'] + '元'
          name = this.flowName + '-' + contract;
        } else if (this.selectFlowType == 'monthly_perf_reserveTalent'){ // 后备英才月度绩效
          let str = '';
          if (value['userNameStudent'] && value['userNameStudent'] != 'undefined') {
            str = '-' + JSON.parse(value.userNameStudent).name;
          } else if (value['studyUserName']) {
            str = '-' + value.studyUserName;
          }
          name = value['targetTime'] + this.flowName + str
        } else if (this.selectFlowType == 'contract_invoicing'){ // 合同开票申请单
          let contract = value['contractObj'] ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
          let sumNumber = value['invoiceMoney'] ? '-' + JSON.parse(value['invoiceMoney']) + '元'  : '-' + 0 + '元';
          let str = value['hasContract'] == '有合同开票' ? (contract + sumNumber) : sumNumber;
          name = this.flowName + str;
        } else if (this.selectFlowType == 'contract_receipt_form') { // 合同收款表单
          let contract = value['contractObj'] && JSON.parse(value['contractObj']).name ?  '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
          // let contract = value['contractObj'] ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
          let sumNumber = value['realityMoneyTotal'] ? '-' + JSON.parse(value['realityMoneyTotal']) + '元' : '-' + 0 + '元';
          let str = value['hasContract'] == '是' ? (contract + sumNumber) : sumNumber;
          name = this.flowName + str;
        } else if (this.selectFlowType == 'contract_payment_form') { // 合同付款表单
          let contract = value['contractObj']&&JSON.parse(value['contractObj']).name ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
          let sumNumber = value['applyPaymentAmount'] ? '-' + JSON.parse(value['applyPaymentAmount']) + '元' : '-' + 0 + '元';
          let str = contract + sumNumber;
          name = this.flowName + str;
        } else if (this.selectFlowType == 'internal_delegation') { // 内部委托表单
          name = this.flowName + '-' + value['subject'];
        } else if (this.selectFlowType == 'single_source_delegation') { // 单一来源委托表单
          name = this.flowName + '-' + JSON.parse(value.project).name;
        } else if (this.selectFlowType == 'proposed_supplier_review') { // 拟选用供应商评审表
          let projectName = this.isJSON(value.project) ? JSON.parse(value.project).name : value.project;
          name = this.flowName + '-' + projectName + '-' + value['serviceContent'];

          // name = this.flowName + '-' + JSON.parse(value.project).name + '-' + value['serviceContent'];
        } else if(this.selectFlowType == 'personnel_requirements_schedule'){ //人员需求计划表
          // name = this.flowName + '-' + value['company_name'];
          if(value.handlingCompanyAndDepartment){
            const company = JSON.parse(value.handlingCompanyAndDepartment)
            name = this.flowName + '-' + company.name;
          }else{
            name = this.flowName + '-' + value['company_name'];
          }
        } else if(this.selectFlowType == 'personnel_addition_approval_form'){ //人员增补审批表
          // name = this.flowName + '-' + localstorageGet('userName');
          name = this.flowName + '-' + value['company_name'] + '-' + value['department_name'];
        } else if(this.selectFlowType == 'employee_conversion_report_evaluation_form'||this.selectFlowType == 'probation_employee_approval_form'||this.selectFlowType == 'probation_assessment'
        ||this.selectFlowType=='employee_demission_approval_form'||this.selectFlowType=='employee_retirement_notice'||this.selectFlowType=='employee_retirement_form'||this.selectFlowType=='retiree_reemployment_application_form'
        ||this.selectFlowType=='employee_demission_approval_form'||this.selectFlowType=='employee_retirement_notice'||this.selectFlowType=='employee_retirement_form'||this.selectFlowType=='retiree_reemployment_application_form'){
          //员工试用期考核表、员工转正述职报告评分表、试用期员工转正审批表、员工离职审批表、员工退休通知单、员工退休表、退休人员返聘申请表、员工离职审批表、员工退休通知单、员工退休表、退休人员返聘申请表
          name = this.flowName + '-' + value['user_name'];
        } else if(this.selectFlowType == 'expense_budget'){
          name = this.flowName + '-' + localstorageGet('userName') + '-' + value['expenseDetailList_total'] + '元';
        }else if(this.selectFlowType=='work_handover'){
          name = this.flowName + '-' + value['user_name'];
        }else if(this.selectFlowType=='refund_bid'){ // 投标保证金退还请款
          name = this.flowName + '-' + value['collectionCompany'];
        }else if(this.selectFlowType=='vehicle_regist'){
          var userName_dept = value['userName_dept']
          if(userName_dept)userName_dept = `/${userName_dept}`
          if(value['departKilometre']){
            name = `${this.flowName}-${value['userName_company']}${userName_dept}-${value['specificAddress']}-${value['departKilometre']}公里`
          }else{
            name = `${this.flowName}-${value['userName_company']}${userName_dept}-${value['specificAddress']}`
          }
        } else if(this.selectFlowType == 'assets_buy_apply'){
          name = `${this.flowName}-${value['batchNumber']}${value['time']}${value['batch']}`
        }else if(this.selectFlowType == 'assets_transfer'){
          name = `${this.flowName}-${value['receiveUserName']}`
        }else if(this.selectFlowType == 'staff_annual_performance'){
          name = value['year'] + value['title'] + '-' + value['userName']
          console.log('%c [ name ]-1171', 'font-size:13px; background:pink; color:#bf2c9f;', name)
        } else if (this.selectFlowType == 'ticket_collection_register') {
          // 收票登记表
          name = `${this.flowName}-${value['contractNumber']}${value['contractName']}-${value['invoiceMoney']}元`;
        } else if (this.selectFlowType == 'business_trip_application_form') {
          // 出差申请表
          console.log(value, "value")
          const dateField = value.businessTripDateRange || value.writeTime;
          const businessTripDateRange = Array.isArray(dateField) ? dateField.join('至') : (dateField || ''); // 写入时间
          name = `${this.flowName}-${JSON.parse(value['myUserName']).name}-出差日期${businessTripDateRange}`; // 流程名称-申请人-写入时间
        } else if (this.selectFlowType == 'cost_funds_transfer') { // 资金调拨单（资金上划和资金下拨）
          // 下拨：transferCompanyId2__virtualName 有值；上划：transferCompanyId1__virtualName 有值
          let transferCompanyName = value['transferCompanyId2__virtualName'] || value['transferCompanyId1__virtualName'] || '';
          let money = value['money'] || '';
          name = `${this.flowName}-${transferCompanyName}-${money}元`;
        } else {
          // name = this.flowName + '-' + localstorageGet('companyName') + '-' + localstorageGet('userName')

          if(this.isReInitiate){
            //如果是重新发起，重新搞标题
            name = this.flowName + '-' + localstorageGet('userName')
          }else{
            //如果是其他类型，审核的时候不修改名称了
            name = ''//this.flowName + '-' + localstorageGet('userName')
          }

        }
      // }
      return name;
    },
    // 是否JSON
    isJSON(str) {
      try {
        const result = JSON.parse(str);
        // JSON 格式必须是一个对象或数组
        return typeof result === 'object' && result !== null;
      } catch (e) {
        return false;
      }
    },
    // 重新发起流程需要校验公司，避免流程发起后撤回，切换公司重新发起流程造成问题
    checkFlowPermission() {
      return new Promise((resolve,reject)=>{
          const param = {
          data: {
            flowProxyId: this.id,
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
    // 重新发起提交
    async handleRePostSubmit(flag) {
      let r = await this.checkFlowPermission()
      if(r === false)return this.$message.error('暂无权限发起流程')
      console.log('=====handleRePostSubmit===')
      this.clickMethod = 'submit'
      console.log('flag',  flag)

      let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm
      const editData = this.$parent.$parent.editData
      generateForm.getData(true).then(async x => { // 24.12.25改为不用getData返回的value，使用getValues获取(原因要获取表单虚拟字段)。getData只做校验
        let value = generateForm.getValues();
        console.log('value', value)
        // return;
        value.global_user_basic_information = {
          userId: localstorageGet('userId'),
          userName: localstorageGet('userName'),
          companyId: localstorageGet('companyId'),
          companyName: localstorageGet('companyName'),
          departmentId: localstorageGet('userDepartmentId'),
          departmentName: localstorageGet('userDepartmentName'),
          dutyId: localstorageGet('dutyId'),
          dutyName: localstorageGet('dutyName'),
        }
        // return;
        console.log(value,'value888888888888',this.selectFlowType)
      if(this.selectFlowType =='travel_expense'){ // 差旅报销单
        const {expenseBudgetList_total,expenseInAccountInfoList_total,travelPersonnelVoList_total,expenseDetailList_total} = value
        let total = [expenseBudgetList_total,expenseInAccountInfoList_total,travelPersonnelVoList_total,expenseDetailList_total]
        total = Array.from(new Set(total))
        if(total.length>1){
          this.submitLoading = false
          return this.$message.error('请检查费用明细、入账信息、费用预算和出差人员合计金额是否相同')
        }
        if(value.expenseBudgetList.length==0){
            this.submitLoading = false
            return this.$message.error('请添加费用预算类型！')
          }
      }else if(this.selectFlowType =='expense_loan'){// 借款单
        if(value.applicationFundsVo_payMoney!=value.expenseInAccountInfoList[0].money){
          this.submitLoading = false
          return this.$message.error('请检查借款金额和入账信息金额是否相同！')
        }
      }else if(this.selectFlowType =='request_funds'){// 请款单
        if(value.applicationFundsVo_type=='2'&&(value.applicationFundsVo_payMoney!=value.totalMoney)){
          this.submitLoading = false
          return this.$message.error('请检查请款金额和费用预算合计金额是否相同！')
        }
        if(value.applicationFundsVo_type=='2'&&value.expenseBudgetList.length==0){
            this.submitLoading = false
            return this.$message.error('请添加费用预算类型！')
          }
      }else if(this.selectFlowType =='expense_repayment'){//还款单
        if(value.applicationFundsVo_payMoney==0){
          this.submitLoading = false
          return this.$message.error('请检查关联借款单还款金额合计金额是否是大于0！')
        }
        // let total = 0
        // value.expenseInAccountInfoList.map(item=>{
        //   total += item.thisMoney
        // })
        // if(value.applicationFundsVo_payMoney!=total){
        //   this.submitLoading = false
        //   return this.$message.error('请检查还款金额和关联借款单还款金额合计金额是否是同！')
        // }
      }else if(this.selectFlowType =='expense_guarantee_repayment'){//归还单
        // 校验：本次归还金额(payMoney) 必须等于 本次还款金额合计(RequestPayoutList中thisMoney的总和)
        let thisMoneyTotal = 0
        if(value.RequestPayoutList && value.RequestPayoutList.length > 0){
          value.RequestPayoutList.forEach(item => {
            thisMoneyTotal += Number(item.thisMoney || 0)
          })
        }
        if(value.payMoney != thisMoneyTotal){
          this.submitLoading = false
          return this.$message.error('本次归还金额与本次还款金额合计不一致，请检查！')
        }
      }
      // else if (this.selectFlowType =='personnel_requirements_schedule'){ //人员需求必填校验
      //   for(let i=0;i<value.person_list.length;i++){
      //     const item = value.person_list[i]
      //     if(!item.department_name||!item.post_name||!item.jobDuties||!item.major||!item.type||!item.onDuty){
      //       return this.$message.error('请检查部门、岗位名称、工作职责、专业、拟到岗时间和人员类型是否填写！')
      //     }
      //   }
      // }

        // 关联公共流程
        // let flowInstanceBizRelevanceList = []
        let flowInstanceBizRelevanceList = deepClone(this.flowInstanceBizRelevanceList);
        var bizId
        let find = flowInstanceBizRelevanceList.find(el=>el.otherBiz == this.selectFlowType)
        if(find)bizId = find.otherBizId
        let newInstanceBizWrap = flowInstanceBizRelevanceList.filter(x=>x.otherBiz != 'commonFlow').map(y=>{return{otherBiz:y.otherBiz,otherBizId:y.otherBizId}});
        let commonFlow = this.$parent.$parent.$refs?.flowListSelect || this.$parent.$refs?.flowListSelect;
        console.log('commonFlow-重新提交',commonFlow)
        if (commonFlow && commonFlow.dataShowList.length) {
          commonFlow.dataShowList.forEach(item => {
            newInstanceBizWrap.push({
              otherBiz: 'commonFlow',
              otherBizId: item.id // 流程id
            })
            // flowInstanceBizRelevanceList.push({
            //   otherBiz: 'commonFlow',
            //   otherBizId: item.id // 流程id
            // })
          })
        }
        if(this.selectFlowType == 'assets_transfer'){ //移交领用单，需要提交关联的id
          let find = newInstanceBizWrap.find(el=>el.otherBiz == 'relate_assets_buy_apply')
          if(find){
            find.otherBizId = value.flowId
          }else{
            newInstanceBizWrap.push({
              otherBiz: 'relate_assets_buy_apply',
              otherBizId: value.flowId // 保存后返回的id
            })
          }
        }
        const param = {
          data: {
            name: this.getBackFlowName(value),
            id: this.flowInstanceId,
            formProxyId: this.formId,
            flowInstanceBizRelevanceList: newInstanceBizWrap,  // 以前的草稿或者重新提交没有带这个参数，现在带上去不知道对其他有无影响
          },
          formDataMongoVo: {
            data: Object.assign(editData,value)
            // data: value
          }
        };
        if(!this.getBackFlowName(value)){

          delete param.data.name
        }
        // if (flag) {
        // param.nextAuditorList = this.nextAuditorList;
        param.nextAuditorList = this.nextAuditorList.map(item=>{
            item.auditDetailType = "personnel"
            if(item.nodeProxyId === undefined)item.nodeProxyId = this.pageNextNodeProxyId
            return item
          });
        // }
        //如果下一个节点是空节点，则需要删除下一个节点的id
        // if (this.chooseBranchNode.nodeType == 'empty') {
        //   param.nextAuditorList = this.nextAuditorList.map(item => {
        //     return {
        //       bizId: item.bizId,
        //       name:item.name
        //     }
        //   })
        // }
        // if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        //   // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
        //   if(!param.nextAuditorList.length){
        //     param.nextAuditorList = [
        //       {nodeProxyId:this.chooseBranchNode.nextNodeTemplateId}
        //     ]
        //   }
        // }
        if (this.chooseBranchList && this.chooseBranchList.length) {
          // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
          //chooseBranchList去重
          this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
          this.chooseBranchList.forEach(item=>{
            param.nextAuditorList.push({nodeProxyId:item})
          })
        }
        let beforeSubmitAndDraftData = { flowDialog: this, clickMethod: this.clickMethod, param, reInit: true } // reInit表示重新发起
        if(this.businessId)beforeSubmitAndDraftData.businessId = this.businessId
        if(bizId)beforeSubmitAndDraftData.businessId = bizId
        let submitDataResult = await generateForm.triggerEvent('beforeSubmitAndDraft', beforeSubmitAndDraftData); // 等待执行自定义业务完之后审批通过或者不通过zxf
        if(submitDataResult === false)return
        // console.log('submitDataResult',submitDataResult[0])
        if(this.operaFile){ // 表单需要操作文件（只针对一个文件上传控件，多个文件上传控件未考虑）
          this.addNewOrDeleteFile(value,this.businessId)
        }

        console.log('重新发起',param)

        if(this.isTranspondFlow){ // 如果是转发数据，直接删除名称
          delete param.data.name
        }
        // return;
        // 对表单参数的自动审核字段auto_audit_info添加一层判断，避免axios不识别将其过滤
        // for(var i in param.formDataMongoVo.data){
        //   if(i.indexOf('auto_audit_info')>-1) {
        //     if (param.formDataMongoVo.data[i] == undefined){
        //       param.formDataMongoVo.data[i] = '';
        //     }
        //   }
        // }
        await generateForm.triggerEvent('beforeSubmitAndDraftNoBiz', beforeSubmitAndDraftData); // 流程提交前执行无业务方法
        param.data.companyId = localstorageGet('companyId')
        if(this.selectFlowType == 'staff_annual_performance'){
          param.data.name = value['year'] + value['title'] + '-' + value['userName']
          // 遍历 formDataMongoVo.data，查找 formPersonId 后缀的字段并更新
          for (const key in param.formDataMongoVo.data) {
            if (key.endsWith('__formPersonId')) {
              const prefix = key.replace('__formPersonId', '');
              if (param.formDataMongoVo.data[prefix]) {
                param.formDataMongoVo.data[key] = param.formDataMongoVo.data[prefix];
              }
            }
          }
        }
        this.$axios.post(
          Api.schedule.saveFlowInstanceAgain,
          param,
          res => {
            if (res.isSuccess) {
              //绑定文件到对应的节点，如果有
              this.afterSaveDeal()
              // this.chooseBranchNode = {};
              // this.chooseBranchNodeList = [];
              // this.handleCloseParallelChoose();
              // this.handleCloseBranchChoose();
              // this.$emit('resetParallelNodeChooseList', []);
              // this.$message.success('提交成功！');
              // this.closeCheck();
            } else {
              this.submitLoading = false
              this.noSubmitLoading = false
              this.flowErrorHandle(res)
              // this.chooseBranchNode.nextNodeTemplateId = ""
              // if (!res.data) {
              //   this.$message.error(res.message);
              //   return;
              // }
              // this.$message.error(res.message);
              // if (res.data.errorType == 'custom_choose') {
              //   //条件分支跟上手动，需要用户选择分支
              //   let branchNodes = res.data.branchNodes
              //   this.manualChooseNodes = []
              //   branchNodes.map((x, index) => {
              //     this.manualChooseNodes.push({
              //       nextNodeTemplateId: x.id,
              //       nodeName: x.nodeName,
              //       nodeType: x.type, // 为处理空节点
              //       branchName: '分支' + (index + 1),
              //       auditType: x.flowNodeAuditConfig.auditType
              //     });
              //   })
              //   this.branchChooseVisible = true
              //   this.tempManu = true
              // }else if(res.data.errorType == 'parallel_choose'){
              //   let hasChoose = false,parallelChooseNodes = []
              //   res.data.branchNodes.forEach(parallelNode => {
              //     if (parallelNode.flowNodeAuditConfig.auditType == 'run_node_choose') {
              //       hasChoose = true;
              //       parallelChooseNodes.push(
              //         {
              //           auditType : 'run_node_choose',
              //           nodeName: parallelNode.nodeName,
              //           nextNodeTemplateId: parallelNode.flowNodeAuditConfig.nodeTemplateId,
              //           nodeAuditList: []
              //         }
              //       );
              //     }
              //   })
              //   if(hasChoose){
              //     this.parallelNodeChooseList = parallelChooseNodes
              //     this.parallelChooseVisible = true;
              //   }
              // } else {
              //   this.flowNodeType = 'run_node_choose';
              //   this.pageNextNodeProxyId = res?.data?.node?.id || ''
              //   this.chooseBranchNode.nextNodeTemplateId = this.nextNodeProxyId
              //   this.nodeChooseVisible = true;
              // }
            }
          }
        );
      }).catch(e => {
        if (typeof e == 'string') this.$message.error(e);
      }).finally(() => {
        this.submitLoading = false
        this.noSubmitLoading = false
      });
    },
    // 保存草稿
    handleSaveDraft() {
      let generateForm = this.$parent.$parent.$refs?.generateForm || this.$parent.$refs?.generateForm;
      this.clickMethod = 'draft'
      generateForm.getData(false).then(async x => { // 24.12.25改为不用getData返回的value，使用getValues获取(原因要获取表单虚拟字段)。getData只做校验
        let value = generateForm.getValues();

        //如果是 staff_annual_performance 需要重新拼接
        if(this.selectFlowType == 'staff_annual_performance'){
          value.additionalData.bizTitle = value['year'] + value['title'] + '-' + value['userName']
        }

        // 关联公共流程
        // let flowInstanceBizRelevanceList = []
        let flowInstanceBizRelevanceList = deepClone(this.flowInstanceBizRelevanceList);
        let newInstanceBizWrap = flowInstanceBizRelevanceList.filter(x=>x.otherBiz != 'commonFlow').map(y=>{return{otherBiz:y.otherBiz,otherBizId:y.otherBizId}});
        let commonFlow = this.$parent.$parent.$refs?.flowListSelect || this.$parent.$refs?.flowListSelect;
        console.log('commonFlow',commonFlow)
        if (commonFlow && commonFlow.dataShowList.length) {
          commonFlow.dataShowList.forEach(item => {
            newInstanceBizWrap.push({
              otherBiz: 'commonFlow',
              otherBizId: item.id // 流程id
            })
            // flowInstanceBizRelevanceList.push({
            //   otherBiz: 'commonFlow',
            //   otherBizId: item.id // 流程id
            // })
          })
        }
        const param = {
          data: {
            id: this.flowInstanceId,
            formProxyId: this.formId,
            status: 'draft',
            name: this.getBackFlowName(value),
            flowInstanceBizRelevanceList: newInstanceBizWrap
          },
          formDataMongoVo: {
            data: value
          }
        };
        let apiUrl = Api.schedule.saveFlowInstance
        // if(this.isReInitiate)apiUrl = Api.schedule.saveFlowInstanceAgain
        // await generateForm.triggerEvent('beforeSubmitAndDraft', { flowDialog: this, clickMethod: this.clickMethod, param });
        let beforeSubmitAndDraftData = { flowDialog: this, clickMethod: this.clickMethod, param }
        if(this.businessId)beforeSubmitAndDraftData.businessId = this.businessId
        let submitDataResult = await generateForm.triggerEvent('beforeSubmitAndDraft', beforeSubmitAndDraftData); //  // 等待执行自定义业务完之后保存草稿zxf
        // console.log('submitDataResult',submitDataResult[0])
        if(submitDataResult === false)return
        if(this.operaFile){ // 表单需要操作文件（只针对一个文件上传控件，多个文件上传控件未考虑）
          this.addNewOrDeleteFile(value,this.businessId)
        }
        await generateForm.triggerEvent('beforeSubmitAndDraftNoBiz', beforeSubmitAndDraftData); // 流程提交前执行无业务方法
        //去重 param.data.flowInstanceBizRelevanceList
        // let list = []
        // param.data.flowInstanceBizRelevanceList.forEach(el=>{
        //   if(el.otherBiz == this.selectFlowType){
        //     if(el.otherBizId)list.push(el)
        //   }else{
        //     list.push(el)
        //   }
        // })
        // param.data.flowInstanceBizRelevanceList = list
        //去重 param.data.flowInstanceBizRelevanceList(2025.9.17优化去重，付款单不知能否生效，待生产验证)
        const uniqueBizRelevanceList = [];
        const bizMap = new Map();

        param.data.flowInstanceBizRelevanceList.forEach(item => {
          if (item.otherBiz === 'commonFlow') {
            // commonFlow可以有多个，直接添加
            uniqueBizRelevanceList.push(item);
          } else {
            // 其他类型确保唯一性
            if (!bizMap.has(item.otherBiz)) {
              bizMap.set(item.otherBiz, true);
              uniqueBizRelevanceList.push(item);
            }
          }
        });
        param.data.flowInstanceBizRelevanceList = uniqueBizRelevanceList;
        if(this.selectFlowType == 'staff_annual_performance'){
          param.data.name = value['year'] + value['title'] + '-' + value['userName']
        }
        this.$axios.post(
          apiUrl,
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
      }).catch(e => {
        if (typeof e == 'string') this.$message.error(e);
      });
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
    // ===============业务保存代码开始==================
    // 合同业务逻辑
    // async contractBusiness(formVal){
    //   let saveContractInfoResData = await this.saveContractInfo(formVal);
    //   // 合规性后面再处理
    //   // let legalContractResData = await this.legalContractSave(saveContractInfoResData)
    //   this.afterSaveDeal()
    // },
    updateFileStatus(data){
      return new Promise((resolve, reject) => {
        this.$axios.post(
        '/web/file/api/file/editFile',
        {
          data
        },
        (res) => {
          if(res.isSuccess){
            resolve(res)
          }else{
            reject(res)
          }
        }
      );

      })

    },
    // 审核提交后统一处理
    afterSaveDeal(){
      if(this.$parent.$children[0]){
        const attachList = this.$parent.$children[0].attachList ||[]
        if(attachList&&attachList.length > 0){
          const updateFile = []
          attachList.map(item=>{
            const file = item.data
            file.fileStatus = '2'
            const updateFileItem = this.updateFileStatus({id:file.id,fileName:file.fileName.split('.')[0],fileStatus:file.fileStatus})
            updateFile.push(updateFileItem)
          })
          Promise.all(updateFile).then(res=>{
            this.commonAction()
          })
        }else{
          this.commonAction()
        }
      }else{
        this.commonAction()
      }


    },
    commonAction(type){
      this.chooseBranchNode = {};
      this.chooseBranchNodeList = [];
      this.handleCloseParallelChoose();
      this.handleCloseBranchChoose();
      this.$emit('resetParallelNodeChooseList', []);
      let show = type ? type+'成功！' : '提交成功！';
      this.$message.success(show);
      this.closeCheck();
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
      if (this.isReInitiate) {
        // 重新发起
        // this.handleRePostSubmit(true);
        this.reSubmitFormMakingFormBusiness(true);
      } else {
        // this.handleSubmitCheck('pass', true);
        this.formMakingFormBusiness(true,  'pass')
      }
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
          } else { // 目前没有进入这个分支，用做兜底
            console.log('自选2')
            this.flowNodeType = 'run_node_choose';
            this.$message({
              message: res.message,
              type: 'warning',
              customClass: 'errorMessage'
            });
            this.nextNodeProxyId = res?.data?.node?.id || ''
            this.nextNodeName = res?.data?.node?.nodeName || ''
            this.currentChooseNodeId = res?.data?.node?.id
            // this.nodeAuditScopeList = res?.data?.node?.flowNodeAuditConfig?.nodeAuditScopeList || []
            // this.nodeAuditType = res?.data?.node?.flowNodeAuditConfig?.type;
            // this.countersignNum = res?.data?.node?.flowNodeAuditConfig?.countersignNum;
            // console.log('nodeAuditScopeList2', this.nodeAuditScopeList)
            this.nodeChooseVisible = true;
          }
        }
    }
  }

};

</script>

<style lang='scss' scoped>
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

::v-deep .fm-form .fm-report-table__table .el-table__cell .cell {
  padding: 0;
}

::v-deep .el-table th.el-table__cell.required>div.required::before{
  content: '*';
  color: #f56c6c;
  margin-right: 4px;
  background: transparent;
  vertical-align: top;
}

::v-deep {
  .el-button--danger {
    background-color: #dc0000;
    border-color: #dc0000;
  }
}


.button-bottom-div {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  >button{
    margin-bottom: 3px;
    margin-left: 3px;
  }
}

.opinion-toolbar {
  position: absolute;
  top: 3px;
  right: 0;
  z-index: 20;
  display: flex;
  align-items: center;
}

.opinion-toolbar__checkbox {
  margin-left: 10px;
}
</style>
