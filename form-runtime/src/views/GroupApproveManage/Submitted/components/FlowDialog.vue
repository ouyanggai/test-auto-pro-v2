<!--
 * @Descripttion: 提起流程弹窗(enterprise:公司其他类型的表单)
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div>
    <el-dialog title="发起流程" :fullscreen="true" :visible="visible" :append-to-body="appendToBody" :close-on-click-modal="false"  center
      @close='handleClose' :close-on-press-escape="showUpStep" :modal="false" custom-class="flow-dialog">
      <!-- :close-on-press-escape="showUpStep" :show-close="showUpStep" -->
      <!-- <el-steps :active="stepsActive" align-center>
        <el-step title="选择流程"></el-step>
        <el-step title="填写信息"></el-step>
      </el-steps> -->
      <div class="flow-content">
        <Steps1 v-show="stepsActive == 1" ref="steps1" :sFlowTypeList="sFlowTypeList" @getFlowType="getFlowType" @nextStep="nextStepHandle"
                :flowJson.sync="flowJson" />
          <Steps2 v-if="stepsActive == 2 && !isForm" ref="steps2" :selectFlowType="selectFlowType"
            :selectFlowName="selectFlowName" :flowId="flowId" :parallelChooseNodes.sync="parallelChooseNodes"
            :manualChooseNodes.sync="manualChooseNodes" :flowType="flowType" :flowProjectId.sync="flowProjectId"/>
          <OtherSteps2 v-if="stepsActive == 2 && isForm" ref="OtherSteps2" :formId="formId"
            :flowId="flowId" :selectFlowType="selectFlowType" :steps="stepsActive" :parallelChooseNodes.sync="parallelChooseNodes"
            :manualChooseNodes.sync="manualChooseNodes" :contractId="businessId" :companyId="companyId" @fmDone="fmDone" :flowProjectId.sync="flowProjectId"/>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button type="primary" icon='el-icon-view' v-if="(stepsActive == 2 && (isForm || (selectFlowType == 'expense_budget' || selectFlowType == 'company_monthly_budget' || selectFlowType =='group_finance' || selectFlowType == 'staff_annual_assessment')))" @click="showFlowNodeDetail()">查看流程</el-button>
        <el-button v-show="stepsActive == 1" @click="handleClose">取 消</el-button>
        <el-button v-show="(stepsActive != 1 && showUpStep && (isForm || selectFlowType == 'expense_budget'|| selectFlowType == 'company_monthly_budget' || selectFlowType == 'staff_annual_assessment'))&&!isLending" @click="prevStepHandle">上一步</el-button>
        <el-button v-show="stepsActive == 1" type="primary" @click="nextStepHandle">下一步</el-button>
        <!-- <el-button v-show="stepsActive != 1" type="primary" @click="handleBeforeSubmit">提 交</el-button> -->
        <el-button class="save_draft_button" v-show="(stepsActive != 1 && isForm)&&!isLending" type="primary" @click="sumbitFlow('draft')">保存草稿</el-button>
        <el-button v-show="stepsActive != 1 && (selectFlowType == 'expense_budget'|| selectFlowType == 'company_monthly_budget' || selectFlowType =='group_finance')" type="primary" @click="sumbitFlow('draft')">保存草稿</el-button>
        <el-button v-show="stepsActive != 1 && (isForm || selectFlowType == 'expense_budget' || selectFlowType == 'company_monthly_budget'|| selectFlowType =='group_finance' || selectFlowType == 'staff_annual_assessment')" type="primary" @click="sumbitFlow('submit')">提 交</el-button>
        <!-- <el-button @click="test">test</el-button> -->
      </div>
    </el-dialog>

    <!-- 审批节点选择：建立流程时如果选择了审批人自选，点击提交需要选下一节点审批人 -->
    <!-- <NextNodeDialog :visible.sync="nodeChooseVisible" :nextNodeName="nextNodeName" :nextNodeProxyId="nextNodeProxyId"
             @getSelectPerson="getSelectPerson" v-if="nodeChooseVisible">
           </NextNodeDialog> -->

    <!-- 新版审批人自选，老版因为流程配置时节点配置了公司权限。新版只能选全集团的人 -->
    <PersonSelectDialog :visible.sync="oldNodeChooseVisible" v-if="oldNodeChooseVisible" @getSelectPerson="getSelectPerson"
      :nextNodeName="nextNodeName" @showFlowNodeDetail="showFlowNodeDetail" :nextNodeProxyId="currentChooseNodeId"
      :nodeAuditType="nodeAuditType" :countersignNum="countersignNum"/>
    <!-- 2025.1.2审批人自选更换成审批人范围选择 -->
    <RangePersonSelect :visible.sync="nodeChooseVisible" v-if="nodeChooseVisible" @getSelectPerson="getSelectPerson"
      :nextNodeName="nextNodeName" @showFlowNodeDetail="showFlowNodeDetail" :nextNodeProxyId="currentChooseNodeId"
      :nodeAuditScopeList="nodeAuditScopeList" :nodeAuditType="nodeAuditType" :countersignNum="countersignNum"/>

    <!-- 并行审批人自选 -->
    <el-dialog v-if="parallelChooseVisible" width="400px" title="并行节点自选" class="nodePerson"
      :visible="parallelChooseVisible" :before-close="handleCloseParallelChoose" :close-on-click-modal="false"
      append-to-body>
      <div v-for="chooseNode in parallelChooseNodes" :key="chooseNode.nextNodeTemplateId">
        <span style="line-height: 28px;">{{ chooseNode.nodeName }} <span>：</span> </span>
        <span v-for="(audit, index) in nextAuditorList" :key="index">
          <span v-if="chooseNode.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
        </span>
        <!-- <span v-for="audit in chooseNode.nodeAuditList" :key="audit.id"> {{ audit.name }} </span> -->
        <el-button type="text" size="mini" @click="chooseParallelNode(chooseNode)">
          {{ nextAuditorList.findIndex(el=>el.nodeProxyId == chooseNode.nextNodeTemplateId) > -1 ? '重新选择' : '选择人员' }}</el-button>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleSaveParallelChooseNode">提 交</el-button>
      </span>
    </el-dialog>
    <!-- 手动分支选择 -->
    <el-dialog v-if="branchChooseVisible" width="500px" title="选择流程分支" class="nodePerson" :visible="branchChooseVisible"
      :before-close="handleCloseBranchChoose" :close-on-click-modal="false" append-to-body>
      <el-radio-group v-model="chooseBranchNode" @change="handleChangeChooseBranch" @input="handleInputChangeChooseBranch">
        <el-radio v-for="(branch, index) in manualChooseNodes" :key="index" :label="branch" class="radio-choose-item">
          <span>{{ branch.branchName }}-{{ branch.nodeName }}<span v-if="branch.auditType == 'run_node_choose'">：</span></span>
          <span v-if="branch.auditType == 'run_node_choose'">
            <span v-for="(audit, index) in nextAuditorList" :key="index">
              <span v-if="branch.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
            </span>
            <el-button type="primary" :disabled="branch.nextNodeTemplateId != chooseBranchNode.nextNodeTemplateId"
              @click="handleChooseBranchNode(branch.branchName ,branch.nodeName,branch.nextNodeTemplateId,branch)" v-if="branch.auditType == 'run_node_choose'">选择审批人</el-button>
          </span>
          <span v-if="branch.auditType == 'initiator'">由发起人审批</span>
          <span v-if="branch.auditType == 'assign'">由流程配置人员审批</span>
          <span v-if="branch.nodeType == 'empty'">
            空节点
            <!-- 空节点一个节点如果自选，这里显示选择的人 -->
            <span v-for="(audit, index) in chooseBranchNodeList" :key="index">
              <span v-if="branch.nextNodeTemplateId == audit.nodeProxyId">{{ audit.name }} </span>
            </span>
          </span>
        </el-radio>
      </el-radio-group>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleSaveBranchChooseNode">提 交</el-button>
      </span>
    </el-dialog>
    <!-- 针对目标责任书流程 选择工作还是管理指标 -->
    <el-dialog :visible="chooseVisible" title="选择目标责任书类型" :close-on-click-modal="false" width="300px" @close='closeChoose'
      class="examiner-dialog" append-to-body>
      <div style="padding:25px;">
        <el-radio-group v-model="flowChoosed">
          <el-radio v-for="item in [{type:'workTarget',name:'工作指标'},{type:'manageTarget',name:'管理指标'},]" :label="item.type" >{{ item.name }}</el-radio>
        </el-radio-group>
      </div>
      <span slot="footer">
        <el-button @click="closeChoose">取 消</el-button>
        <el-button type="primary" @click="confirmChoose">确 定</el-button>
      </span>
    </el-dialog>
    <!-- 扩展属性选人 -->
    <extended-attribute-users :nextNodeProxyId="nextNodeProxyId" :visible.sync="extendedVisible" :personList="extendedPersonList" @confirmChooseExtendPerson="confirmChooseExtendPerson"></extended-attribute-users>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
     :flowId="flowId" :initiatorId="initiatorId" :isDraft="true" :nextNodeProxyId="nextNodeProxyId"></CheckFlowNodeDetail>

     <!-- 盖章评审合同流程打开的合同列表 -->
     <ContractList v-if="contractListVisible" :visible.sync="contractListVisible" @confirmContract="confirmContract"></ContractList>
  </div>
</template>

<script>
import dayjs from 'dayjs'
import Steps1 from './Steps1';
import Steps2 from './Steps2';
import OtherSteps2 from './OtherSteps2';
// import NextNodeDialog from '../../components/nextNodeDialog.vue';
import PersonSelectDialog from '../../components/PersonSelectDialog.vue';
import RangePersonSelect from '../../components/RangePersonSelect.vue';
import { localstorageRemove, localstorageGet } from '@/utils/auth';
import CheckFlowNodeDetail from '../../components/CheckFlowNodeDetail.vue';
import ContractList from './ContractList.vue';
import math from '@/utils/math.js'
import { deepClone,generateRandomId } from "@/utils";
import Api from '@/api';
import extendedAttributeUsers from "@/components/extendedAttributeUsers.vue"
export default {
  name: 'FlowDialog',
  components: {
    Steps1,
    Steps2,
    OtherSteps2,
    PersonSelectDialog,
    RangePersonSelect,
    CheckFlowNodeDetail,
    ContractList,
    extendedAttributeUsers
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    sFlowTypeList: {
      type: Array,
      default: () => {
        return [];
      }
    },
    flowJson: {
      type: Object,
      default: () => {
        return null;
      }
    },
    flowType: {
      type: String,
      default: ''
    },
    closeAll:{
      type:Boolean,
      default:false
    },
    showUpStep:{
      type:Boolean,
      default:true
    },
    appendToBody:{
      type:Boolean,
      default:false
    },
    isLending:{ // 是否是资料借阅流程
      type:Boolean,
      default:false
    },
    //工作台传入的项目id，公司公共流程使用
    projectId:{
      type:String,
      default:''
    },
    needReSubmit:{
      type:Boolean,
      default:false
    }
  },
  provide() {
    return {
      flowDialog: this, // 传递到自定义组件使用zxf inject: ['flowDialog', 'enterpriseExamineDialog'],
      prevStepHandle: this.prevStepHandle,
      sumbitFlow: this.sumbitFlow,
      submitFlowFinal: this.submitFinal,
      batchCode: this.batchCode,
      jusgeCustomChoose: () => {
        if (this.formExist) {
          return this.jusgeCustomChoose;
        } else {
          return () => {};
        }
      }
    };
  },
  data() {
    return {
      actionType: 'create',
      operaType: 'add',
      operaFile:false,
      businessId:'',
      companyId:'',
      contractData:{},
      contractListVisible:false,
      formPersonFields: '',
      stepsActive: 1,
      clickMethod: 'submit',
      formId: '',
      flowId: '',
      flowNodeType: '',
      nextNodeName: '',
      nextNodeProxyId: '',
      selectFlowType: '',
      nodeChooseVisible: false,
      oldNodeChooseVisible: false,
      parallelChooseVisible: false,
      parallelChooseNodes: [], // 并行节点发起后选择审批人自选
      parallelNextNodeTemplateId: '',
      branchChooseVisible: false,
      manualChooseNodes: [], // 手动分支发起后选择分支
      chooseBranchNode: {},
      chooseBranchNodeList: [],
      nextAuditorList: [],
      checkboxPersonGroup: [],
      pageFlowTypeList: Object.keys(this.$store.state.settings.budgetPagesName),
      chooseVisible: false,
      flowChoosed: '',
      formExist: '',
      checkViewFlowDetailVisible: false,
      tempManu: false, // 如果是提交后，发现下一节点是条件分支，则需要用这个参数判断用户关闭选择是否清空手动列表
      fmComplete:false,
      paralleNodeName:'',
      submitLoading:false,
      draftLoading:false,
      flowProjectId:'',
      chooseBranchList:[], //选择手动分支的list，里面存的是节点id
      currentChooseNodeId:'',
      haveSaveBiz:false, //这个变量用于判断是否提交过业务，为了避免重复提交业务数据
      initiatorId:'' , //发起人
      nodeAuditScopeList:[],
      nodeAuditType:'',
      countersignNum:'',
      //生成批次号
      batchCode:generateRandomId(32),
      extendedVisible:false,
      extendedPersonList:[]
    };
  },
  computed: {
    // 是否是有表单 true是 false无表单
    isForm() {
      if (!this.formExist) {
        return true;
      } else {
        if (['travel_expense', 'contract_review', 'contract_pay_request', 'request_funds', 'refund_bid', 'invoice_apply'].indexOf(this.selectFlowType) > -1) {
          return true;
        } else {
          return false;
        }
      }
    }
  },
  watch: {
    $route: {
      handler: function (route) {
        if (route.name == 'myContract') {
          this.contractData = route.params;
        }
        this.businessId = route.params?.id || route.query?.id;
        this.companyId = route.params.companyId;
      },
      immediate: true
    },
    fmComplete:{
      handler(val){
        if(val){
          this.$emit('completeStep2')
        }
      }
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
    this.flowProjectId = this.projectId || localstorageGet('projectId')
    this.$bus.$off('submitBeforeHandleOk');
    this.$bus.$on('submitBeforeHandleOk', (obj) => {
      if (obj.status == 'success') {
        // this.hasUpdate = true;
        // this.emitBudgetInfo = obj;
        // this.handleSubmitCheck('pass', true);
        if (this.selectFlowType === 'staff_annual_assessment') {
           this.$emit('success');
           this.handleClose(true);
        }
      } else {
        this.hasUpdate = false;
      }
    });
  },
  mounted() {},
  methods: {
    test(){
    },
    fmDone(){
      //form表单加载完成的标志
      this.fmComplete = true
    },
    // *******项目拉过来的代码************
    // enterpriseHandleBeforeSubmit() {
    //   if (this.parallelChooseNodes && this.parallelChooseNodes.length) {
    //     // 并行节点中的审批人自选、主管、副总 类型的审批节点被传过来了
    //     // 判断并行节点中是否包含审批人自选
    //     const flag = this.parallelChooseNodes.some(item => item.auditType == 'run_node_choose');
    //     if (flag) {
    //       this.parallelChooseVisible = true;
    //     } else {
    //       this.handleSaveParallelChooseNode();
    //     }
    //     return false;
    //   } else if (this.manualChooseNodes && this.manualChooseNodes.length) {
    //     this.branchChooseVisible = true;
    //     // 手动分支选择分支
    //   } else {
    //     // 判断是否审批自选-普通节点
    //     this.checkIsOptional();
    //   }
    // },
    formMakingFormBusiness(flag){ // 有表单业务,最终提交接口前先调用业务接口
      let that = this;
      this.checkFlowPermission().then(res=>{
        if (res.isSuccess) {
          if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性表单业务保存
            let custom_contractLegalField = this.$refs.OtherSteps2.$refs.generateForm.getComponent('custom_contractLegalField') // 合规性评审表中自定义的合同文件列表
            custom_contractLegalField.contractBusiness(this.$refs.OtherSteps2.$refs.generateForm,this,flag)
          } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审表单业务保存
            let custom_contractSealField = this.$refs.OtherSteps2.$refs.generateForm.getComponent('custom_contractSealField') // 合同盖章评审表单中自定义的组件
            custom_contractSealField.contractBusiness(this.$refs.OtherSteps2.$refs.generateForm,this,flag)
          } else if (this.selectFlowType == 'cost_funds_transactions' || this.selectFlowType == 'cost_funds_invest') { // 请款单（资金往来和投资款)
            this.saveCostFundsBusiness(flag);
          } else {
            this.enterpriseHandleSubmit(flag);
          }         
          // else if (this.selectFlowType == 'refund_bid') { // 投标保证金退还请款
          //   this.saveBidBondRefundBusiness(flag);
          // }
        }
      }).catch(function (err) {
        that.$message.error('流程已被修改，请重新发起流程！')
        setTimeout(x=>{
          window.location.reload();
        },2000)
      });
    },
    // 请款单（资金往来和投资款）业务数据保存
    saveCostFundsBusiness(flag) {
      const generateForm = this.$refs.OtherSteps2.$refs.generateForm;
      const isNeedValidate = this.clickMethod == 'draft' ? false : true;
      generateForm.getData(isNeedValidate).then(value => {
        let depId = '';
        let depName = '';
        try {
          const depObj = value.requestDepName ? JSON.parse(value.requestDepName) : {};
          depId = depObj.id || '';
          depName = depObj.name || '';
        } catch (e) {}
        let expenseUserId = '';
        let expenseUserName = '';
        try {
          const userObj = value.requestUserName ? JSON.parse(value.requestUserName) : {};
          expenseUserId = userObj.id || '';
          expenseUserName = userObj.name || '';
        } catch (e) {}
        let applicationFundsVo_paymentAccount = '';
        try {
          applicationFundsVo_paymentAccount = value?.applicationFundsVo_paymentAccount ?? '';
        } catch (error) {}

        const businessData = {
          depId,
          depName,
          expenseUserId,
          expenseUserName,
          remarks: value.remark || '',
          payCompanyId: value.applicationFundsVo_payCompanyId || '',
          payCompanyName: value.applicationFundsVo_payCompanyName || '',
          payMethod: value.applicationFundsVo_payMethod || '',
          payMoney: value.applicationFundsVo_payMoney != null ? String(value.applicationFundsVo_payMoney) : '',
          name: value.applicationFundsVo_payName || '',
          openingBank: value.applicationFundsVo_openingBank || '',
          account: value.applicationFundsVo_account || '',
          paymentAccount: value.applicationFundsVo_paymentAccount || '',
          status: this.clickMethod == 'draft' ? '0' : '1',
          ...(this.clickMethod != 'draft' ? { examineStatus: '0' } : {}),
          type: this.selectFlowType === 'cost_funds_invest' ? '2' : '1',
        };
        if (this.businessId) {
          businessData.id = this.businessId;
        }
        // 请款单（资金往来和投资款两种请款单）添加付款账号后推送到金蝶
        if (applicationFundsVo_paymentAccount) {
          businessData.paymentAccount = applicationFundsVo_paymentAccount;
        }
        this.$axios.post(
          Api.formMaking.costFundsTransactionsSave,
          { data: businessData },
          res => {
            if (res.isSuccess) {
              this.businessId = res.data.id || res.data;
              this.enterpriseHandleSubmit(flag);
            } else {
              this.$message.error(res.message || '业务数据保存失败');
              this.submitLoading = false;
              this.draftLoading = false;
            }
          }
        );
      }).catch(err => {
        if (typeof err == 'string') this.$message.error(err);
        this.submitLoading = false;
        this.draftLoading = false;
      });
    },
    saveBidBondRefundBusiness(flag) {
      const generateForm = this.$refs.OtherSteps2.$refs.generateForm;
      const isNeedValidate = this.clickMethod == 'draft' ? false : true;
      generateForm.getData(isNeedValidate).then(value => {
        const pickValue = (...keys) => {
          for (const key of keys) {
            const current = value[key];
            if (current !== undefined && current !== null && current !== '') {
              return current;
            }
          }
          return '';
        };
        const parseSelectValue = (rawValue) => {
          if (!rawValue) {
            return {};
          }
          if (typeof rawValue === 'object') {
            return rawValue;
          }
          try {
            return JSON.parse(rawValue);
          } catch (e) {
            return {};
          }
        };
        const normalizeDateTime = (rawValue) => {
          if (rawValue == null || rawValue === '') {
            return '';
          }
          const d = dayjs(rawValue);
          if (!d.isValid()) {
            return '';
          }
          return d.format('YYYY-MM-DD HH:mm:ss');
        };
        const businessData = {
          companyId: pickValue('expenseCompanyId'),
          projectId: pickValue('project_name'),
          requestDate: normalizeDateTime(pickValue('cell_15')),
          payMethod: pickValue('cell_17'),
          payeeName: pickValue('collectionCompany'),
          openingBank: pickValue('cell_12'),
          account: pickValue('cell_13'),
          paymentAccount: pickValue('cell_11'),
          actualRefundAmount: pickValue('contractSum'),
          remark: pickValue('cell_14'),
          status: this.clickMethod == 'draft' ? '0' : '1',
          ...(this.clickMethod != 'draft' ? { examineStatus: '0' } : {}),
        };
        if (this.businessId) {
          businessData.id = this.businessId;
        }
        this.$axios.post(
          Api.formMaking.bidBondRefundSave,
          { data: businessData },
          res => {
            if (res.isSuccess) {
              this.businessId = res.data.id || res.data;
              this.enterpriseHandleSubmit(flag);
            } else {
              this.$message.error(res.message || '业务数据保存失败');
              this.submitLoading = false;
              this.draftLoading = false;
            }
          }
        );
      }).catch(err => {
        if (typeof err == 'string') this.$message.error(err);
        this.submitLoading = false;
        this.draftLoading = false;
      });
    },
    // 流程名称
    getBackFlowName(value) {
      let name = '';
      if(this.selectFlowType =='request_funds'||this.selectFlowType =='expense_loan'||this.selectFlowType =='expense_repayment'){ // 请款单  借款单 还款单
        name = this.selectFlowName + '-'+ value['expenseUserName']+ '-' + value['applicationFundsVo_payMoney'] + '元'
      }else if(this.selectFlowType =='travel_expense'){ // 差旅报销
        name = this.selectFlowName+ '-' + value['expenseUserName'] + '-' + value['expenseInAccountInfoList_total'] + '元'
      } else if (this.selectFlowType == 'contract_compliance_review' || this.selectFlowType == 'contract_seal_review') { // 合规和盖章评审
        let contract = value['contractNumber'] + value['contractName'] + '-' + value['contractSum'] + '元'
        name = this.selectFlowName + '-' + contract;
      } else if (this.selectFlowType == 'monthly_perf_reserveTalent'){ // 后备英才月度绩效
        let str = '';
        if (value['userNameStudent']) {
          str = '-' + JSON.parse(value.userNameStudent).name;
        } else if (value['studyUserName']) {
          str = '-' + value.studyUserName;
        }
        name = value['targetTime'] + this.selectFlowName + str
      } else if (this.selectFlowType == 'internship_trial_notice'){ // 实习试岗通知单
        let str = value['name'] ? '-' + value['name'] : '';
        name = this.selectFlowName + str;
      } else if (this.selectFlowType == 'contract_invoicing'){ // 合同开票申请单
        let contract = value['contractObj'] ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
        let sumNumber = value['invoiceMoney'] ? '-' + JSON.parse(value['invoiceMoney']) + '元' : '-' + 0 + '元';
        let str = value['hasContract'] == '有合同开票' ? (contract + sumNumber) : sumNumber;
        name = this.selectFlowName + str;
      }else if (this.selectFlowType == 'contract_receipt_form') { // 合同收款表单
        let contract = value['contractObj'] && JSON.parse(value['contractObj']).name ?  '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
        // let contract = value['contractObj'] ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
        let sumNumber = value['realityMoneyTotal'] ? '-' + JSON.parse(value['realityMoneyTotal']) + '元' : '-' + 0 + '元';
        let str = value['hasContract'] == '是' ? (contract + sumNumber) : sumNumber;
        name = this.selectFlowName + str;
      } else if (this.selectFlowType == 'contract_payment_form') { // 合同付款表单
        let contract = value['contractObj']&&JSON.parse(value['contractObj']).name ? '-' + value['contractNumber'] + JSON.parse(value['contractObj']).name : '';
        let sumNumber = value['applyPaymentAmount'] ? '-' + JSON.parse(value['applyPaymentAmount']) + '元' : '-' + 0 + '元';
        let str = contract + sumNumber;
        name = this.selectFlowName + str;
      } else if (this.selectFlowType == 'internal_delegation') { // 内部委托表单
        name = this.selectFlowName + '-' + value['subject'];
      } else if (this.selectFlowType == 'single_source_delegation') { // 单一来源委托表单
        name = this.selectFlowName + '-' + JSON.parse(value.project).name;
      } else if (this.selectFlowType == 'proposed_supplier_review') { // 拟选用供应商评审表
        let projectName = this.isJSON(value.project) ? JSON.parse(value.project).name : value.project;
        name = this.selectFlowName + '-' + projectName + '-' + value['serviceContent'];
        // name = this.selectFlowName + '-' + JSON.parse(value.project).name + '-' + value['serviceContent'];
      } else if(this.selectFlowType == 'personnel_requirements_schedule'){ //人员需求计划表
        if(value.handlingCompanyAndDepartment){
          const company = JSON.parse(value.handlingCompanyAndDepartment)
          name = this.selectFlowName + '-' + company.name;
        }else{
          name = this.selectFlowName + '-' + value['company_name'];
        }
      } else if(this.selectFlowType == 'personnel_addition_approval_form'){ //人员增补审批表
        name = this.selectFlowName + '-' + value['company_name'] + '-' + value['department_name'];
      } else if(this.selectFlowType == 'employee_conversion_report_evaluation_form'||this.selectFlowType == 'probation_employee_approval_form'||this.selectFlowType == 'probation_assessment'
      ||this.selectFlowType=='employee_demission_approval_form'||this.selectFlowType=='employee_retirement_notice'||this.selectFlowType=='employee_retirement_form'||this.selectFlowType=='retiree_reemployment_application_form'
      ||this.selectFlowType=='employee_demission_approval_form'||this.selectFlowType=='employee_retirement_notice'||this.selectFlowType=='employee_retirement_form'||this.selectFlowType=='retiree_reemployment_application_form'){
        //员工试用期考核表、员工转正述职报告评分表、试用期员工转正审批表、员工离职审批表、员工退休通知单、员工退休表、退休人员返聘申请表、员工离职审批表、员工退休通知单、员工退休表、退休人员返聘申请表
        name = this.selectFlowName + '-' + value['user_name'];
      } else if(this.selectFlowType == 'expense_budget'){
        name = this.selectFlowName + '-' + localstorageGet('userName') + '-' + value['expenseDetailList_total'] + '元';
      }else if(this.selectFlowType=='work_handover'){
        name = this.selectFlowName + '-' + value['user_name'];
      }else if(this.selectFlowType=='refund_bid'){ // 投标保证金退还请款
        name = this.selectFlowName + '-' + (value['collectionCompany'] || value['payeeName'] || value['collectionCompanyName'] || '');
      }else if(this.selectFlowType=='vehicle_regist'){
        var userName_dept = value['userName_dept']
        if(userName_dept)userName_dept = `/${userName_dept}`
        if(value['departKilometre']){
          name = `${this.selectFlowName}-${value['userName_company']}${userName_dept}-${value['specificAddress']}-${value['departKilometre']}公里`
        }else{
          name = `${this.selectFlowName}-${value['userName_company']}${userName_dept}-${value['specificAddress']}`
        }
      } else if (this.selectFlowType == 'assets_buy_apply') {
        name = `${this.selectFlowName}-${value['batchNumber']}${value['time']}${value['batch']}`
      } else if (this.selectFlowType == 'assets_transfer') {
        name = `${this.selectFlowName}-${value['receiveUserName']}`
      } else if (this.selectFlowType == 'cost_funds_transactions' || this.selectFlowType == 'cost_funds_invest') { // 请款单（资金往来和投资款）
        let payCompanyName = value['payCompanyName'] || value['applicationFundsVo_payCompanyName'] || '';
        let expenseUserName = value['expenseUserName'] || localstorageGet('userName');
        let payMoney = value['payMoney'] || value['applicationFundsVo_payMoney'] || '';
        name = `${this.selectFlowName}-${payCompanyName}-${expenseUserName}-${payMoney}元`
      } else if (this.selectFlowType == 'ticket_collection_register') {
        // 收票登记表
        name = `${this.selectFlowName}-${value['contractNumber']}${value['contractName']}-${value['invoiceMoney']}元`;
      } else if (this.selectFlowType == 'business_trip_application_form') {
        // 出差申请表
        console.log(value, "value");
        const dateField = value.businessTripDateRange || value.writeTime;
        const businessTripDateRange = Array.isArray(dateField) ? dateField.join('至') : (dateField || '');// 写入时间
        name = `${this.selectFlowName}-${JSON.parse(value['myUserName']).name}-出差日期${businessTripDateRange}`;
      } else if (this.selectFlowType == 'cost_funds_transfer') { // 资金调拨单（资金上划和资金下拨）
        // 下拨：transferCompanyId2__virtualName 有值；上划：transferCompanyId1__virtualName 有值
        let transferCompanyName = value['transferCompanyId2__virtualName'] || value['transferCompanyId1__virtualName'] || '';
        let money = value['money'] || '';
        name = `${this.selectFlowName}-${transferCompanyName}-${money}元`;
      } else {
        // name = this.selectFlowName + '-' + localstorageGet('companyName') + '-' + localstorageGet('userName')
        name = this.selectFlowName + '-' + localstorageGet('userName')
      }
      return name;
    },
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
    // 最终提交接口--表单流程
    async enterpriseHandleSubmit(flag) {
      if(this.clickMethod == 'draft')this.draftLoading = true
      else this.submitLoading = true
      // flag----------true:审批人自选   false:指定节点
      let isNeedValidate = this.clickMethod == 'draft' ? false : true  //是否进行表单验证，如果为true则进行表单验证，如果是false，不验证，草稿时候不验证

      this.$refs.OtherSteps2.$refs.generateForm.getData(isNeedValidate).then(async x => { // 24.12.25改为不用getData返回的value，使用getValues获取(原因要获取表单虚拟字段)。getData只做校验
        let value = this.$refs.OtherSteps2.$refs.generateForm.getValues();
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

        if(this.selectFlowType == 'staff_annual_performance'){
          const flowTemplateData = this.$refs.OtherSteps2.flowTemplateData;
          if (flowTemplateData && flowTemplateData.flowNodeTemplate) {
             this.traverseFlowNode(flowTemplateData.flowNodeTemplate, value);
          }
        }

        // return
        let flowInstanceBizRelevanceList = [{
          otherBiz: 'company',
          otherBizId: localstorageGet('companyId')
        }]

        if(this.selectFlowType == 'internal_delegation'){ //内部委托单存多一个受托单位公司，方便其他业务场景的搜索
          flowInstanceBizRelevanceList.push({
            otherBiz: 'company',
            otherBizId: JSON.parse(value.entrustedCompanyName).id
          })
        }

        if(this.selectFlowType == 'assets_transfer'){ //移交领用单，需要提交关联的id
          flowInstanceBizRelevanceList.push({
            otherBiz: 'relate_assets_buy_apply',
            otherBizId: value.flowId // 保存后返回的id
          })
        }
        if(this.flowProjectId){
          //如果是项目里面发起流程
          flowInstanceBizRelevanceList = [{
            otherBiz: "project", otherBizId: this.flowProjectId
          },{
            otherBiz: 'isProject',
            otherBizId: 'isProject'
          }]
        }
        if(this.businessId){
          flowInstanceBizRelevanceList.push({
            otherBiz: this.selectFlowType,
            otherBizId: this.businessId // 保存后返回的id
          })
        }
        // if(this.haveSaveBizId){
        //   flowInstanceBizRelevanceList.push({
        //     otherBiz: this.selectFlowType,
        //     otherBizId: this.haveSaveBizId // 保存业务id
        //   })
        // }

        // 关联公共流程
        // let commonFlow = this.$refs.OtherSteps2.$children[1]
        let commonFlow = this.$refs.OtherSteps2.$refs.flowListSelect;
        if (commonFlow && commonFlow.dataShowList.length) {
          commonFlow.dataShowList.forEach(item => {
            flowInstanceBizRelevanceList.push({
              otherBiz: 'commonFlow',
              otherBizId: item.id // 流程id
            })
          })
        }
        const param = {
          data: {
            name: this.getBackFlowName(value),
            flowInstanceBizRelevanceList,
          },
          formDataMongoVo: {
            data: value
          }
        };
        if(this.$refs.OtherSteps2.$children[0].attachList){
          param.formDataMongoVo.data.fileupload_common_file =this.$refs.OtherSteps2.$children[0].attachList
        }
        if(value.probation_employee_approval_form_id){
          param.data.flowInstanceBizRelevanceList.push({otherBizId:value.probation_employee_approval_form_id})
        }
        if (this.clickMethod == 'draft') {
          // 存草稿的状态
          param.data.status = 'draft';
        }
        if (this.formId == '') { // 无表单
          param.data.flowProxyId = this.flowId;
        } else {
          param.data.formProxyId = this.formId;
        }
        // if (flag) {
          // // 自选审批人的节点
          // param.nextAuditorList = this.nextAuditorList;
          //如果下一个节点是空节点，则需要删除下一个节点的id
          // if(this.chooseBranchNode.nodeType == 'empty'){
          //   param.nextAuditorList = this.nextAuditorList.map(item=>{
          //     return {
          //       bizId:item.bizId,
          //       auditDetailType:"personnel"
          //     }
          //   })
          // }else{
          // 2025.9.9后端说要去掉nextAuditorList
          param.nextAuditorList = this.nextAuditorList.map(item=>{
            if(item.nodeProxyId === undefined)item.nodeProxyId = this.nextNodeProxyId
            return item
          });
          // }
        // }
        if (this.chooseBranchList && this.chooseBranchList.length) {
          // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
          //chooseBranchList去重
          this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
          // 2025.9.9后端说要去掉nextAuditorList
          this.chooseBranchList.forEach(item=>{
            param.nextAuditorList.push({nodeProxyId:item})
          })
        }
        // if(!this.haveSaveBiz && !this.haveSaveBizId){
          //  && !this.haveSaveBizId（删除这个判断）
          // let submitDataResult = await this.$refs.OtherSteps2.$refs.generateForm.triggerEvent('beforeSubmitAndDraft', { flowDialog: this, clickMethod: this.clickMethod, param }); // 等待执行自定义业务完之后流程提交和存草稿zxf
          // return
          // if(this.operaFile){ // 表单需要操作文件（只针对一个文件上传控件，多个文件上传控件未考虑）(目前合同付款表单在用)
          //   this.formSaveFile(value,submitDataResult[0]['id']);
          // }
          // if(submitDataResult === false)return
        // }
        // this.haveSaveBiz = true //走下来表示业务已经提交
        //获取提交业务成功后，表单里面绑定号的业务id
        // let find = param.data.flowInstanceBizRelevanceList.find(item=>item.otherBiz == this.selectFlowType)
        // if(find){
        //   this.haveSaveBizId = find.otherBizId
        // }


        // 对表单参数的自动审核字段auto_audit_info添加一层判断，避免axios不识别将其过滤
        for(var i in param.formDataMongoVo.data){
          if(i.indexOf('auto_audit_info')>-1) {
            if (param.formDataMongoVo.data[i] == undefined){
              param.formDataMongoVo.data[i] = '';
            }
          }
        }
        // if (this.parallelChooseNodes && this.parallelChooseNodes.length) { // 并行多分支
        //   var nextAuditorList = [];
        //   this.parallelChooseNodes.forEach(item => {
        //     var nodeProxyId = item.nextNodeTemplateId || item.id;
        //     var personList = item.nodeAuditList;
        //     personList.forEach(el => {
        //       var bizId = el.id;
        //       nextAuditorList.push({
        //         nodeProxyId, bizId,name:el.name
        //       });
        //     });
        //   });
        //   param.nextAuditorList = nextAuditorList;
        // }
        //存档管理
        if(this.$refs.OtherSteps2.$refs.attachList){
          const attachList = this.$refs.OtherSteps2.$refs.attachList.attachList

          if (attachList &&attachList.length > 0) {
            attachList.map(item=>{
              const file = item.data
              if(param.data.flowInstanceBizRelevanceList){
                param.data.flowInstanceBizRelevanceList.push({otherBiz:'design_file_audit',otherBizId:file.id})
              }
              // else{
              //   param.flowInstanceBizRelevanceList.push({otherBiz:'fileData',otherBizId:file.id})
              // }

            })
          }
        }
        if (this.$refs.OtherSteps2.formPersonFields?.length) { // 下个节点设置表单人员 ----
          // var nextAuditorList2 = [];
          var fields = this.$refs.OtherSteps2.formPersonFields;
          if (typeof fields == 'string') { fields = [{ bizId: fields }]; }
          fields.forEach(el=>{
            let key = el.bizId
            if(value[key] === undefined){
              let field = key.replace('__formPersonId','')
              if (this.isJSON(value[field])) {
                let obj = JSON.parse(value[field])
                param.formDataMongoVo.data[key] = obj.id
              }else{
                param.formDataMongoVo.data[key] = value[field]
              }
            }
          })
          // var values = this.$refs.OtherSteps2.$refs.generateForm.getValues();
          // fields.forEach(i => {
          //   // 兼容两种人员选择器
          //   if (this.isJSON(values[i.bizId])) { // 如果是json字符串，需要进行转换
          //     nextAuditorList2.push({ bizId: JSON.parse(values[i.bizId]).id, nodeProxyId: i.nodeProxyId });
          //   } else {
          //     nextAuditorList2.push({ bizId: values[i.bizId], nodeProxyId: i.nodeProxyId });
          //   }
          //   // nextAuditorList2.push({ bizId: values[i.bizId], nodeProxyId: i.nodeProxyId });
          // });
          // param.nextAuditorList.push(...nextAuditorList2);
        }
        // return
        // await this.$refs.OtherSteps2.$refs.generateForm.triggerEvent('beforeSubmitAndDraftNoBiz', { flowDialog: this, clickMethod: this.clickMethod, param }); // 流程提交前执行无业务方法
        param.batchCode = this.batchCode;
        let submitDataResult = await this.$refs.OtherSteps2.$refs.generateForm.triggerEvent('beforeSubmitAndDraft', { flowDialog: this, clickMethod: this.clickMethod, param, init: true }); // 等待执行自定义业务完之后流程提交和存草稿zxf
        if (this.operaFile) { // 表单需要操作文件（只针对一个文件上传控件，多个文件上传控件未考虑）(目前合同付款表单在用)
          this.formSaveFile(value, submitDataResult[0]['id']);
        }
        if (submitDataResult === false) return
        //去重 param.data.flowInstanceBizRelevanceList
        let list = []
        param.data.flowInstanceBizRelevanceList.forEach(el=>{
          if(el.otherBiz == this.selectFlowType){
            if(el.otherBizId)list.push(el)
          }else{
            list.push(el)
          }
        })
        param.data.flowInstanceBizRelevanceList = list
        param.data.companyId = value.global_user_basic_information.companyId
        this.$axios.post(
          Api.schedule.saveFlowInstance,
          param,
          async (res) => {
            this.draftLoading = false
            this.submitLoading = false
            if (res.isSuccess) {
              await this.$refs.OtherSteps2.$refs.generateForm.triggerEvent('afterSaveFlowInstance', { flowDialog: this, clickMethod: this.clickMethod, flowInstanceData: res.data }); // 流程发起成功后执行表单业务
              this.afterSaveDeal()
            } else {
              this.flowErrorHandle(res)
            }
          }
        );
      }).catch(err=>{
        if (typeof err == 'string') this.$message.error(err);
        this.draftLoading = false;
        this.submitLoading = false;
      })
      .finally(() => {
        this.draftLoading = false;
        this.submitLoading = false;
      });
    },
    // 递归遍历流程树，处理form_person类型的节点
    traverseFlowNode(node, value) {
      if (!node) return;

      // Check current node
      if (node.flowNodeAuditConfig && node.flowNodeAuditConfig.auditType === 'form_person') {
        let fields = node.flowNodeAuditConfig.formPersonFields;
        if (fields) {
          // 支持逗号分隔的多个字段
          const fieldList = fields.split(',');
          fieldList.forEach(fieldKey => {
            fieldKey = fieldKey.trim();
            if (!fieldKey) return;

            // Check if field exists in value
            if (value[fieldKey] === undefined || value[fieldKey] === '' || value[fieldKey] === null) {
              const parts = fieldKey.split('__');
              if (parts.length > 0) {
                const sourceKey = parts[0];
                // Check if source key exists in value
                if (value[sourceKey] !== undefined) {
                  if (this.isJSON(value[sourceKey])) {
                    let obj = JSON.parse(value[sourceKey]);
                    value[fieldKey] = obj.id;
                  } else {
                    value[fieldKey] = value[sourceKey];
                  }
                }
              }
            }
          });
        }
      }

      // Recurse
      if (node.childFlowNodeTemplate) {
        this.traverseFlowNode(node.childFlowNodeTemplate, value);
      }
      if (node.conditionNodes && node.conditionNodes.length) {
        node.conditionNodes.forEach(child => this.traverseFlowNode(child.childFlowNodeTemplate, value));
      }
      if (node.parallelNodes && node.parallelNodes.length) {
        node.parallelNodes.forEach(child => this.traverseFlowNode(child.childFlowNodeTemplate, value));
      }
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
    formSaveFile(value,bizId){
      // 关联文件
      let fileList = value.uploadFile.map(x => {return x.data.id});
      this.saveFile(bizId,fileList);
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
    // 保存后统一处理
    afterSaveDeal(){
    if(this.$refs.OtherSteps2.$refs.attachList){
      const attachList = this.$refs.OtherSteps2.$refs.attachList.attachList
      if(attachList&&attachList.length > 0){
        const updateFile = []
        attachList.map(item=>{
          const file = item.data
          file.fileStatus = '2'
          const updateFileItem = this.updateFileStatus({id:file.id,fileName:file.fileName.split('.')[0],fileStatus:file.fileStatus})
          updateFile.push(updateFileItem)
        })
        Promise.all(updateFile).then(res=>{
          this.$emit('updateFileStatus');
          this.commonAction()
        })
      }else{
        this.commonAction()
      }
    }else{
      this.commonAction()
    }

    },
    commonAction(){
      this.$message.success(this.clickMethod == 'draft' ? '保存成功!' : '提交成功！');
      this.$emit('success');
      this.handleClose(true);
      this.chooseBranchNode = {};
      this.chooseBranchNodeList = [];
      this.handleCloseParallelChoose();
      this.handleCloseBranchChoose();
    },
    //  *******项目拉过来的代码结束*******
    sumbitFlow(method, otherObj) {
      this.clickMethod = method;
      this.otherObj = otherObj;
      // if (this.selectFlowType == 'enterprise') {
      if (!this.formExist) {
        // formmaking表单流程
        if (method == 'draft') {
          // 存草稿-无需验证下一节点
          // this.enterpriseHandleSubmit(false);
          this.formMakingFormBusiness(false)
        } else {

          const values = this.$refs.OtherSteps2.$refs.generateForm.getValues()
          if(this.selectFlowType =='travel_expense'){ // 差旅报销单
            const {expenseBudgetList_total,expenseInAccountInfoList_total,travelPersonnelVoList_total,expenseDetailList_total} = values
            let total = [expenseBudgetList_total,expenseInAccountInfoList_total,travelPersonnelVoList_total,expenseDetailList_total]
            total = Array.from(new Set(total))
            if(total.length>1){
              this.submitLoading = false
              return this.$message.error('请检查费用明细、入账信息、费用预算和出差人员合计金额是否相同')
            }
            if(values.expenseBudgetList.length==0){
              return this.$message.error('请添加费用预算类型！')
            }
            let canPass = true
            values.expenseBudgetList.forEach(el=>{
              if(!el.companyNumber)canPass = false
            })
            if(!canPass){
              return this.$message.error('费用预算类型序号不能为空！')
            }
            if(values.travelReimbursemenVo_isAccount==="true"&&values.accountDetailedVoList.length==0){
              return this.$message.error('请添加关联借款单！')
            }
          }else if(this.selectFlowType =='expense_loan'){// 借款单
            if(values.applicationFundsVo_payMoney!=values.expenseInAccountInfoList[0].money){
              return this.$message.error('请检查借款金额和入账信息金额是否相同！')
            }
          }else if(this.selectFlowType =='request_funds'){// 请款单
            if(values.applicationFundsVo_type=='2'&&(values.applicationFundsVo_payMoney!=values.totalMoney)){
              return this.$message.error('请检查请款金额和费用预算合计金额是否相同！')
            }
            if(values.applicationFundsVo_type=='2'&&values.expenseBudgetList.length==0){
              return this.$message.error('请添加费用预算类型！')
            }
            if(values.expenseBudgetList.length){
              let canPass = true
              values.expenseBudgetList.forEach(el=>{
                if(!el.companyNumber)canPass = false
              })
              if(!canPass){
                return this.$message.error('费用预算类型序号不能为空！')
              }
            }
          }else if(this.selectFlowType =='expense_repayment'){//还款单
            if(values.applicationFundsVo_payMoney==0){
                return this.$message.error('请检查关联借款单还款金额合计金额是否是大于0！')
              }
            // let total = 0
            // values.expenseInAccountInfoList.map(item=>{
            //   total += item.thisMoney
            // })
            // if(values.applicationFundsVo_payMoney!=total){
            //   return this.$message.error('请检查还款金额和关联借款单还款金额合计金额是否是同！')
            // }
          }else if(this.selectFlowType =='expense_guarantee_repayment'){//归还单
            // 校验：本次归还金额(payMoney) 必须等于 本次还款金额合计(RequestPayoutList中thisMoney的总和)
            let thisMoneyTotal = 0
            if(values.RequestPayoutList && values.RequestPayoutList.length > 0){
              values.RequestPayoutList.forEach(item => {
                thisMoneyTotal += Number(item.thisMoney || 0)
              })
            }
            if(values.payMoney != thisMoneyTotal){
              return this.$message.error('本次归还金额与本次还款金额合计不一致，请检查！')
            }
          }
          // else if (this.selectFlowType =='personnel_requirements_schedule'){ //人员需求必填校验
          //   values.person_list
          //   for(let i=0;i<values.person_list.length;i++){
          //     const item = values.person_list[i]
          //     if(!item.department_name||!item.post_name||!item.jobDuties||!item.major||!item.type||!item.onDuty){
          //       return this.$message.error('请检查部门、岗位名称、工作职责、专业、拟到岗时间和人员类型是否填写！')
          //     }
          //   }

          // }

          // 提交-需要验证下一节点
          // this.enterpriseHandleBeforeSubmit();
          this.formMakingFormBusiness(true)
        }
      } else {
        if (method == 'draft') {
          if(this.clickMethod == 'draft')this.draftLoading = true
          else this.submitLoading = true
          if (this.selectFlowType == 'expense_budget') { // 费用报销单保存草稿，先保存数据
            const expensesClaimForm = this.$refs.steps2.$refs.ExpensesClaimForm;
            const submitPromise = expensesClaimForm.submit('draft');
            if (submitPromise) {
              submitPromise.then(res => {
                if (res.isSuccess) {
                  res.data.expenseDetailList.forEach(x => {
                    if (x.attachmentIds) {
                      this.bindBatchFileByIds(x.id, x.attachmentIds.split(',')); // 绑定文件
                    }
                  });
                  this.clickMethod = 'draft'
                  this.submitFinal(false, res.data.id, 'expense_budget');
                } else {
                  this.$message.error(res.message);
                }
              });
            } else {
              this.parallelChooseVisible = false;
              this.branchChooseVisible = false;
            }
          } else if(this.selectFlowType == 'company_monthly_budget'){
            const companyMonthlyFinance = this.$refs.steps2.$refs.company_monthly_budget;
            companyMonthlyFinance.submit(this.clickMethod).then(res=>{
              if(res){
                // id, type, money, name,method
                this.otherObj = res
                this.submitFinal(false, res.id,this.selectFlowType ,res.money,res.name);
              }else{
                this.$message.error('保存失败')
              }
            })
          }else {
            const emitType = this.selectFlowType;
            this.$bus.$emit(emitType + '_before_handle',method);
          }
        } else {
          // 无表单流程-每次发起流程审批时（新增，非编辑），在保存接口和提交审批接口前需要加多一个权限校验。
          // 解决了问题：提审无权限后，费用明细中添加了脏数据（提审失败后保存了）
          this.checkFlowPermission().then(res=>{
            if (res.isSuccess) {
              this.jusgeCustomChoose();
            } else {
              // this.$message.error(res.message);
            }
          });
        }
      }
    },
    getFlowType(data) {

      // this.selectFlowType = data.type;
      this.selectFlowType = data.auditWay;
      this.selectFlowName = data.flowName;
      this.flowId = data.id;
      this.formExist = data.formExist;

      // 安全地获取 formId
      if (data.formTemplateList && data.formTemplateList.length > 0) {
        this.formId = data.formTemplateList[0].id;
      }

    },
    prevStepHandle() {
      if(this.closeAll){
        this.$emit('update:visible', false);
      }else{
        this.$emit('update:flowType', '');
        this.stepsActive--;
      }
    },
    nextStepHandle() {
      const { formId, flowId } = this.$refs.steps1;
      this.flowNodeType = this.$refs.steps1.flowNodeType;
      this.nextNodeProxyId = this.$refs.steps1.nextNodeProxyId;
      this.nextNodeName = this.$refs.steps1.nextNodeName;
      this.formPersonFields = this.$refs.steps1.formPersonFields;
      this.formId = formId;
      this.flowId = flowId;
      if (!flowId) {
        this.$message.error('请勾选流程');
        return;
      }
      if (this.selectFlowType == 'year_kpi_work_target' && !this.flowType) {
        this.$router.push('/manpowerResource/performanceManage/targetBook');
        return;
      }
      if (this.selectFlowType == 'annual_perf' && !this.flowType) {
        this.chooseVisible = true;
      }else {
        if (this.stepsActive++ > 2) this.stepsActive = 1;
      }
      if(this.stepsActive > 2) this.stepsActive = 2;

      // 隐藏合规评审，暂时注释
      // else if(this.selectFlowType == 'contract_seal_review') { // 盖章评审表单要拉起合规性评审列表页
      //   if (!Object.keys(this.contractData).length){
      //     this.contractListVisible = true;
      //   } else {
      //     if (this.stepsActive++ > 2) this.stepsActive = 1;
      //   }
      // }
    },
    confirmContract(row){
      this.contractData = row;
      this.businessId = row.id;
      let params = JSON.parse(JSON.stringify(row))
      let name = this.$route.name //获取当前路由的name,不写死，方便在其他页面调用
      // 给路由赋值参数，方便表单中使用。如果直接在当前路由添加params，无法添加。需要跳到其他路由再回来。使用query放在网址后面很丑
      this.$router.replace({ name: 'myContract', params: {} },()=>{
        this.$router.replace({ name, params: params })
      })
      this.nextStepHandle();
      setTimeout(x=>{
        this.contractData = {}
      },500)
    },
    closeChoose() {
      this.chooseVisible = false;
      this.flowChoosed = '';
    },
    confirmChoose() {
      if (!this.flowChoosed) {
        return this.$message.error('请选择目标责任书类型');
      }
      this.$emit('update:flowType', this.flowChoosed);
      this.closeChoose();
      this.$nextTick(() => {
        if (this.stepsActive++ > 2) this.stepsActive = 1;
      });
    },
    getSelectPerson(data) {
      //要不要反选，这里还要去重
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
      if(!this.manualChooseNodes?.length && !this.parallelChooseNodes?.length){
        // 如果是员工年终考核且之前跳过了postData，现在需要调用postData
        if (this.selectFlowType === 'staff_annual_assessment' && this.flowNodeType === 'run_node_choose') {
          // 选人完成后，现在调用postData保存数据
          var key = this.$refs.steps2.selectFlowTypeName;
          this.$refs.steps2.$refs[key].postData(1,this.batchCode, (err, res) => {
            if (!err && res && res.id) {
              // postData成功后，提交最终流程
              this.submitFinal(true, res.id, this.selectFlowType);
            }
          });
        } else if (!this.formExist) {
          // 表单流程
          this.formMakingFormBusiness(true)
        } else {
          this.handleBeforeSave(true);
        }
      }
      return
    },
    // 提交流程前的后端的校验
    checkFlowPermission() {
      const param = {
        data: {
          flowProxyId: this.flowId,
          checkPermissions: 'first',
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'company',
              otherBizId: localstorageGet('companyId')
            }
          ]
        }
      };
      if(this.flowProjectId){
        param.data.flowInstanceBizRelevanceList = [
          {otherBiz: "project", otherBizId: this.flowProjectId}
        ]
      }
      return this.$axios.post(
        Api.schedule.saveFlowInstance,
        param
      );
    },
    // 无表单流程-判断审批人自选的情况
    jusgeCustomChoose() {
      this.handleBeforeSave(true);
    },
    // 提交审批前先保存--无表单流程
    async handleBeforeSave(customChooseFlag) {
      if(this.clickMethod == 'draft')this.draftLoading = true
      else this.submitLoading = true
      //如果之前已经提交过业务，直接提交流程，无需再次提交业务
      // if(this.haveSaveBiz && this.haveSaveBizObject && this.haveSaveBizObject.id){
      //   //调用业务更新接口
      //   let arr = ['annual_perf','monthly_perf_blocSum','monthly_perf_departmentSum']
      //   if(arr.indexOf(this.selectFlowType) > -1){
      //     this.submitFinal(this.haveSaveBizObject.customChooseFlag, this.haveSaveBizObject.id, this.haveSaveBizObject.type,this.haveSaveBizObject.money, this.haveSaveBizObject.name,this.haveSaveBizObject.method);
      //     return
      //   }
      // }
      // if (this.selectFlowType == 'expense_reimbursement') { // 金额调剂单
      //   const adjustForm = this.$refs.steps2.$refs.CompanyAmountAdjustForm;
      //   const promise = adjustForm.submit();
      //   if (promise) {
      //     promise.then(res => {
      //       if (res.isSuccess) {
      //         if (res.data.enclosure) { // 绑定文件
      //           this.bindFileById(res.data.id, res.data.enclosure).then(() => {
      //             this.submitFinal(customChooseFlag, res.data.id);
      //           });
      //         } else {
      //           this.submitFinal(customChooseFlag, res.data.id);
      //         }
      //       } else {
      //         this.$message.error(res.message);
      //       }
      //     });
      //   } else {
      //     this.parallelChooseVisible = false;
      //     this.branchChooseVisible = false;
      //   }
      // } else
      // if (this.selectFlowType == 'group_finance') { // 涉及公司财务
      //   this.submitFinal(true, this.otherObj.id, 'group_finance', this.otherObj.money);
      // } else
      if (this.selectFlowType == 'expense_budget') { // 费用报销单
        const expensesClaimForm = this.$refs.steps2.$refs.ExpensesClaimForm;
        //先保存业务为草稿，再保存为正常业务
        var submitPromise
        // if(this.haveSaveBizObject?.id){
        submitPromise = expensesClaimForm.submit('',this.batchCode);
        // }else{
          // submitPromise = expensesClaimForm.submit('draft');
        // }
        if (submitPromise) {
          submitPromise.then(res => {
            if (res.isSuccess) {
              res.data.expenseDetailList.forEach(x => {
                if (x.attachmentIds) {
                  this.bindBatchFileByIds(x.id, x.attachmentIds.split(',')); // 绑定文件
                }
              });
              this.submitFinal(customChooseFlag, res.data.id, 'expense_budget');
            } else {
              this.submitLoading = false
              this.draftLoading = false
              this.$message.error(res.message);
            }
          }).catch(()=>{
            this.submitLoading = false
            this.draftLoading = false
          })
        } else {
          this.parallelChooseVisible = false;
          this.branchChooseVisible = false;
        }
      } else { // 其他流程
        // 如果是员工年终考核且需要选人，直接跳转到选人，不触发postData
        if (this.selectFlowType === 'staff_annual_assessment' && this.flowNodeType === 'run_node_choose') {
          // 直接触发选人逻辑，不调用postData
          if(this.needReSubmit){
            var key = this.$refs.steps2.selectFlowTypeName;
            this.$refs.steps2.$refs[key].postData(1,this.batchCode, null, null, this.needReSubmit);
          } else {
            this.submitFinal(true, null, this.selectFlowType);
          }

        } else {
          var key = this.$refs.steps2.selectFlowTypeName;
          this.$refs.steps2.$refs[key].postData(1,this.batchCode, null, null, this.needReSubmit);
        }
      }
    },
    // 最终的提起流程审批接口--无表单流程
    //@method 提交还是草稿
    submitFinal(customChooseFlag, id, type, money, name,method) {
      // this.haveSaveBizObject = {customChooseFlag, id, type, money, name,method} //这里是给无表单流程用的
      this.haveSaveBiz = true
      var flowInstanceBizRelevanceList = [
        {
          otherBiz: this.selectFlowType,
          otherBizId: id // 保存后返回的id
        },
        {
          otherBiz: 'company',
          otherBizId: localstorageGet('companyId')
        }
      ];
      if(this.flowProjectId){
        flowInstanceBizRelevanceList.push({
          otherBiz: 'isProject',
          otherBizId: 'isProject'
        })
      }
      if (this.selectFlowType == 'annual_perf') {
        flowInstanceBizRelevanceList.push({
          otherBiz: this.flowType,
          otherBizId: id
        });
      }
      if (this.selectFlowType == 'year_kpi_work_target') {
        flowInstanceBizRelevanceList.push({
          otherBiz: this.flowType,
          otherBizId: id
        });
      }
      if (type == 'group_finance' || type == 'company_monthly_budget') { //绑定子流程
        this.otherObj.subInstance.forEach(item => {
          flowInstanceBizRelevanceList.push(item);
        });
      }
      // 关联公共流程
      // let commonFlow = this.$refs.OtherSteps2.$children[1]
      let commonFlow = this.$refs.steps2.$refs.flowListSelect;
      if (commonFlow && commonFlow.dataShowList.length) {
        commonFlow.dataShowList.forEach(item => {
          flowInstanceBizRelevanceList.push({
            otherBiz: 'commonFlow',
            otherBizId: item.id // 流程id
          })
        })
      }
      const param = {
        data: {
          flowProxyId: this.flowId,
          flowInstanceBizRelevanceList
        },
        formDataMongoVo: {
          data: {
            money,
            initiatorRange: this.$store.state.user.userId // 提审的时候每张表单都传一下发起人范围，条件判断有用
          }
          // data: null
        }
      };
      if(method)this.clickMethod = method
      if (this.clickMethod == 'draft') {
        // 存草稿的状态
        param.data.status = 'draft';
      } else {
        // 传这个字段为了多分支流程那边的判断；调剂单不用做分支判断
        if (type == 'expense_budget') {
          const infoForm = this.$refs.steps2.$refs.ExpensesClaimForm.infoForm;
          let expendTypeTotalMoney = 0
          infoForm.expenseBudgetList.forEach(item => {
            expendTypeTotalMoney = math.add(expendTypeTotalMoney,item.money)
          });
          let expendDetailTotalMoney = 0
          infoForm.expenseDetailList.forEach(item => {
            expendDetailTotalMoney = math.add(expendDetailTotalMoney,item.money)
          });
          let taxPreNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item => {
            taxPreNumTotalMoney = math.add(taxPreNumTotalMoney,item.money)
            // pre = math.add(pre + cur.money)
            // return pre
          });
          let taxNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item=> {
            taxNumTotalMoney = math.add(taxNumTotalMoney,item.tax)
          });

          let taxTotalNumTotalMoney = 0
          infoForm.taxInfoList.forEach(item => {
            taxTotalNumTotalMoney = math.add(taxTotalNumTotalMoney ,item.totalAmount)
          })

          let accountInfoTotalMoney = 0
          infoForm.expenseInAccountInfoList.forEach(item => {
            accountInfoTotalMoney = math.add(accountInfoTotalMoney , item.money)
          });
          // 这个formDataMongoVo用来存放需要用来判断字段走哪条分支。data下面传的字段要对应表单里面的字段（对应上数据字典的字段）
          param.formDataMongoVo.data.expendTypeNum = expendTypeTotalMoney
          param.formDataMongoVo.data.expendDetailNum = expendDetailTotalMoney
          param.formDataMongoVo.data.taxPreNum = taxPreNumTotalMoney
          param.formDataMongoVo.data.taxNum = taxNumTotalMoney
          param.formDataMongoVo.data.taxTotalNum = taxTotalNumTotalMoney
          param.formDataMongoVo.data.accountInfoNum = accountInfoTotalMoney
          param.formDataMongoVo.data.expenseCompanyId = infoForm.expenseCompanyName
          param.data.name = `${this.selectFlowName}-${localstorageGet('userName')}-${Number(expendDetailTotalMoney.toFixed(2))}元`;
        } else if (this.selectFlowType == 'annual_perf' || this.selectFlowType == 'year_kpi_work_target') {
          param.formDataMongoVo.data.name = this.$refs.steps2.$refs[this.selectFlowType].createrId;
        } else if (this.selectFlowType == 'staff_annual_assessment') {
           let customName = '';
           const annualAssessmentForm = this.$refs.steps2.$refs.staff_annual_assessment;

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
           param.formDataMongoVo = {
            data: {
                initiatorRange: this.$store.state.user.userId
            }
           };
        } else {
          param.formDataMongoVo = {
            data: {
              money,
              initiatorRange: this.$store.state.user.userId // 提审的时候每张表单都传一下发起人范围，条件判断有用
            }
          };
          if (money === undefined) {
            param.data.name = `${this.selectFlowName}-${localstorageGet('userName')}`;
          }
        }
        // if(name)param.data.name = name
        // if (customChooseFlag) { // 自选审批人的节点
        //   // param.nextAuditorList = this.nextAuditorList;
        //   //如果下一个节点是空节点，则需要删除下一个节点的id
        //   if(this.chooseBranchNode.nodeType == 'empty'){
        //     param.nextAuditorList = this.nextAuditorList.map(item=>{
        //       return {
        //         bizId:item.bizId
        //       }
        //     })
        //   }else{
        //     param.nextAuditorList = this.nextAuditorList;
        //   }
        // }
        // if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        //   // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
        //   if(!param.nextAuditorList.length){
        //     param.nextAuditorList = [
        //       {nodeProxyId:this.chooseBranchNode.nextNodeTemplateId}
        //     ]
        //   }
        // }
        // if (this.parallelChooseNodes && this.parallelChooseNodes.length) { // 并行多分支
        //   var nextAuditorList = [];
        //   this.parallelChooseNodes.forEach(item => {
        //     var nodeProxyId = item.nextNodeTemplateId || item.id;
        //     var personList = item.nodeAuditList;
        //     personList.forEach(el => {
        //       var bizId = el.id;
        //       nextAuditorList.push({
        //         nodeProxyId, bizId,name:el.name
        //       });
        //     });
        //   });
        //   param.nextAuditorList = nextAuditorList;
        // }
        // 2025.9.9后端说要去掉nextAuditorList
        param.nextAuditorList = this.nextAuditorList.map(item=>{
          if(item.nodeProxyId === undefined)item.nodeProxyId = this.nextNodeProxyId
          return item
        });

          // }
        // }
        if (this.chooseBranchList && this.chooseBranchList.length) {
          // param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
          //chooseBranchList去重
          this.chooseBranchList = Array.from(new Set(this.chooseBranchList));
          // 2025.9.9后端说要去掉nextAuditorList
          this.chooseBranchList.forEach(item=>{
            param.nextAuditorList.push({nodeProxyId:item})
          })
        }
      }
      if(this.selectFlowType == 'monthly_perf') { param.formDataMongoVo.data.userInfo = this.$refs.steps2.$refs.monthly_perf.form.userId;}
      if (this?.otherObj?.name)param.data.name = this.otherObj.name;
      if (name)param.data.name = name;
      param.batchCode = this.batchCode
      param.data.companyId = localstorageGet('companyId')
      this.$axios.post(
        Api.schedule.saveFlowInstance,
        param,
        (res) => {
          this.submitLoading = false
          this.draftLoading = false
          if (res.isSuccess) {
            // if (this.selectFlowType == 'expense_budget' && this.clickMethod !='draft') { // 费用报销单
            //   const expensesClaimForm = this.$refs.steps2.$refs.ExpensesClaimForm;
            //   expensesClaimForm.submit(this.clickMethod,id).then(result=>{
            //     if(result.isSuccess){
            //       let msg = '提交成功！'
            //       this.$message.success(msg);
            //       this.$emit('success');
            //       this.handleClose(true);
            //     }else{
            //       this.parallelChooseVisible = false;
            //       this.branchChooseVisible = false;
            //     }
            //   })
            // }else{
              let msg = '提交成功！'
              if(this.clickMethod == 'draft')msg='保存成功'
              this.$message.success(msg);
              this.$emit('success');
              this.handleClose(true);
            // }
          } else {
            //删除除了费用报销之外的其他业务

            this.flowErrorHandle(res)
          }
        }
      ).catch(()=>{
        this.submitLoading = false
        this.draftLoading = false
      })
    },
    //删除对应的业务
    // deleteBiz(id, auditWay) {
    //   //
    //   const typeName = auditWay;
    //   // if (typeName == 'company_monthly_budget' || typeName == 'annual_perf' || typeName == 'monthly_perf'
    //   //     || typeName == 'depart_monthly_budget' || typeName == 'company_annual_budget' || typeName == 'contract_compliance_review'
    //   //     || typeName == 'contract_seal_review' || typeName == 'monthly_perf_reserveTalent' || typeName == 'contract_payment_form'
    //   //     || typeName == 'contract_invoicing'
    //   //   ) {
    //     // const index = obj.findIndex(item => item.otherBiz == typeName);
    //     // if (index > -1) {
    //       // obj[index].otherBizId;
    //       // const id = obj[index].otherBizId;
    //   if(typeName == 'annual_perf' || typeName == 'monthly_perf'){
    //     this.deletePerf(id);
    //   }else if(typeName == 'depart_monthly_budget' || typeName == 'company_annual_budget' || typeName == 'company_monthly_budget'){
    //     this.deleteBudget(id)
    //   } else if(typeName == 'contract_compliance_review'){ // 合规评审(删除流程后不删除业务数据，因为还有专门的草稿列表查看和重新发起)
    //     this.saveOrModifyContractInfo(id,"0",null)
    //   } else if(typeName == 'contract_seal_review'){ // 盖章评审(删除流程后不删除业务数据，因为还有专门的草稿列表查看和重新发起)
    //     this.saveOrModifyContractInfo(id,"1","1")
    //   } else if(typeName == 'monthly_perf_reserveTalent'){ // 后备英才轮岗考核表
    //     this.deleteReserveTalent(id)
    //   } else if(typeName == 'contract_payment_form'){ // 合同付款申请单
    //     this.deleteContractPayment(id)
    //   } else if(typeName == 'contract_receipt_form'){ // 合同收款申请单
    //     this.deleteContractCollection(id)
    //   } else if(typeName == 'contract_invoicing'){ // 合同开票申请单
    //     this.deleteContractInvoicing(id)
    //   } else if(typeName == 'project_approval_budget'){ //删除项目立项
    //     this.$axios.post(
    //       Api.formMaking.projectApprovalBudgetDelete,
    //       { data: { id: id }});
    //   }else if(typeName == 'assets_buy_apply' || typeName == 'assets_transfer' ||typeName == 'assets_handle' ||typeName == 'assets_allocate'){
    //     this.$axios.post(
    //       Api.formMaking.fixedAssetsDelete,
    //       { data: { id: id }});
    //   }else if(typeName == 'expense_budget'){
    //     this.$axios.post(Api.budgetManage.expenseReimbursementDelete,
    //       { data: { id: id }});
    //   }
    //     // }
    //   // }
    // },
    submitBindRelevance(flowInstanceBizRelevanceList,id){
      let data = {
        data:{
          id,
          flowInstanceBizRelevanceList
        }
      }
      return this.$axios.post(Api.schedule.saveBizRelevance,data)
    },
    handleClose(isSubmitClose) {
      if (this.$route.params.formPage && this.$route.name == 'myContract') { // 我的合同进入编辑文档界面后，再发起合规评审，关闭页面需要返回我的合同列表
        // if (this.$parent.formPage == 'myContractEdit' || this.$parent.formPage == 'newAdd' || this.$parent.formPage == 'useTemplate') {
          this.$parent.initiateSuccess();
        // }
      }

      //关闭弹窗去除缓存
      this.manualChooseNodes = []
      this.parallelChooseNodes = []
      this.submitLoading = false
      if(this.closeAll||this.isLending){
        this.$emit('update:visible', false);
      }else{
        if (this.$route?.params?.from) { //从别的页面跳转过来
          this.$router.go(-1);
        } else {
          if (this.stepsActive > 1 && !isSubmitClose) {
            this.prevStepHandle();
          } else {
            // 清除缓存
            localstorageRemove('flowSelectList');
            this.$emit('update:visible', false);
            this.clickMethod = 'submit';
          }
        }
      }
    },
    // 单个文件绑定业务id
    bindFileById(relationId, fileId) {
      const data = {
        relationId,
        fileId
      };
      return this.$axios.post(
        Api.schedule.saveAttachment,
        { data }
      );
    },
    // 多个文件绑定业务id
    bindBatchFileByIds(relationId, fileIds) {
      const data = {
        relationId,
        fileIds
      };
      return this.$axios.post(
        Api.budgetManage.saveBatchFile,
        { data }
      );
    },
    handleCloseParallelChoose() {
      //关闭并行弹框，取消并行已经选择的人
      this.parallelChooseNodes.forEach(el=>{
        let nextNodeTemplateId = el.nextNodeTemplateId
        this.nextAuditorList = this.nextAuditorList.filter(item=>item.nodeProxyId != nextNodeTemplateId)
      })
      this.parallelChooseNodes = []
      this.parallelChooseVisible = false;
    },
    handleCloseBranchChoose() {
      // manualChooseNodes
      this.chooseBranchNode = {};
      this.branchChooseVisible = false;
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
      if (this.tempManu) {
        this.tempManu = false;
        // this.manualChooseNodes = [];
      }
    },
    // 发起人-并行节点自选审批人保存后提交
    async handleSaveParallelChooseNode() {
      let has = true
      this.parallelChooseNodes.forEach(item=>{
        if(item.auditType == 'run_node_choose'){
          let nextNodeTemplateId = item.nextNodeTemplateId
          let index = this.nextAuditorList.findIndex(el=>el.nodeProxyId == nextNodeTemplateId)
          if(index == -1)has = false
        }
      })
      // let has = this.nextAuditorList.some(item=>{
      //   return item.nodeProxyId == nextNodeTemplateId
      // })
      if(!has){
        return this.$message.warning('该分支需选择审批人，请先选择');
      }
      this.parallelChooseVisible = false;
      if (!this.formExist) {
        // 表单流程
        this.formMakingFormBusiness(true)
      } else {
        // 无表单流程
        this.handleBeforeSave(true);
      }
    },
    // 并行节点自选
    chooseParallelNode(node) {
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
        if (!this.formExist) {
          // 表单流程
          this.formMakingFormBusiness(true)
        } else {
          this.handleBeforeSave(true);
        }
      } else {
        this.$message.warning('请选择流程分支');
      }
    },
    // 手动选择分支
    handleChooseBranchNode(node1,node2,nodeId,branch) {
      this.paralleNodeName =  node1+'-'+node2
      this.nextNodeName = node2
      this.currentChooseNodeId = nodeId

      this.nodeAuditScopeList = branch?.nodeAuditScopeList || []
      this.nodeAuditType = branch.nodeAuditType;
      this.countersignNum = branch.countersignNum;
      // this.nodeChooseVisible = true;
      if (branch.nodeAuditScopeList) { // 选了审批人范围，并配置人员
        this.nodeChooseVisible = true;
      } else { // 此处弹窗选择组织架构所有人，用于流程没有配置审批人范围人员，兼容以前数据
        this.oldNodeChooseVisible = true;
      }
    },
    handleInputChangeChooseBranch(val) {
      this.nextNodeName = val.nodeName;
      this.nextNodeProxyId = val.nextNodeTemplateId || val.id;
    },
    handleChangeChooseBranch(branch) {
      //切换后，从人员列表删除除当前手动分支所有的自选人节点的人员
      // let nodeProxyId = branch.nextNodeTemplateId
      this.manualChooseNodes.forEach(el=>{
        let nodeProxyId = el.nextNodeTemplateId
        this.nextAuditorList = this.nextAuditorList.filter(el=>el.nodeProxyId != nodeProxyId)
        this.chooseBranchList = this.chooseBranchList.filter(el=>el != nodeProxyId)
      })

      if(branch.auditType != 'run_node_choose' ){
        this.chooseBranchList.push(branch.nextNodeTemplateId)
      }
    },
    // 显示流程
    showFlowNodeDetail() {

      this.initiatorId = this.$store.state.user.userId;
      this.checkViewFlowDetailVisible = true;
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
      if (!this.formExist) {
        // 表单流程
        this.formMakingFormBusiness(true)
      } else {
        // 无表单流程
        this.handleBeforeSave(true);
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
            let hasChoose = false,parallelChooseNodes = []
            res.data.branchNodes.forEach(parallelNode => {
              if (parallelNode.flowNodeAuditConfig.auditType == 'run_node_choose') {
                hasChoose = true;
                parallelChooseNodes.push(
                  {
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
              this.parallelChooseNodes = parallelChooseNodes
              this.parallelChooseVisible = true;
            }

          } else if(res.data.errorType == 'run_node_choose'){
            this.flowNodeType = 'run_node_choose';
            this.nextNodeProxyId = res?.data?.node?.id || ''
            this.nextNodeName = res?.data?.node?.nodeName || ''
            this.currentChooseNodeId = res?.data?.node?.id
            this.nodeAuditScopeList = res?.data?.node?.flowNodeAuditConfig?.nodeAuditScopeList || []
            this.nodeAuditType = res?.data?.node?.flowNodeAuditConfig?.type
            this.countersignNum = res?.data?.node?.flowNodeAuditConfig?.countersignNum
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
          }else { // 目前没有进入这个分支，用做兜底
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
            this.nodeChooseVisible = true;
          }
        }
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body{
    max-height: unset !important;
    min-height: unset !important;
    padding: 0 5px !important;
}
::v-deep .flow-dialog .el-dialog__footer{
    padding: 0;
}
::v-deep .flow-dialog .el-dialog__header{
    padding: 5px 0 5px 0;
}
.flow-content {
  box-sizing: border-box;
  // height: calc(100vh - 250px);
  height: calc(100vh - 80px);
  overflow-y: auto;
  overflow-x: hidden;
}

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
::v-deep .dialog-fullscreen .el-dialog__body{
  height:90vh;
  overflow:hidden;
}

::v-deep {
  .fm-form {
    // 禁用状态的字体颜色优化
    .el-checkbox__input.is-disabled + span.el-checkbox__label,.el-radio__input.is-disabled + span.el-radio__label,.el-range-editor.is-disabled input,.el-range-editor.is-disabled .el-range-separator{
      color: #606266 !important;
    }
    .el-checkbox__input.is-disabled.is-checked .el-checkbox__inner::after {
      border-color: #606266 !important;
    }
    .el-radio__input.is-disabled.is-checked .el-radio__inner::after{
      background-color: #606266 !important;
    }
    .newFormMaking {
      // 2024.10.31整体样式优化
      .el-form-item.tableAddBorder .form-table {
        border: 1px solid rgb(153, 153, 153);
      }
      .el-table th.el-table__cell.is-leaf,.el-table td.el-table__cell {
        border-color: rgb(153, 153, 153);
      }
      .el-form-item.is-error {
        margin-bottom: 10px !important;
      }

      .el-form-item { // 11.7从formMaking的css剥离
        padding: 3px;
      }
      // .fm-report-table__td .fm-form-item .el-form-item {
      //   padding: 0px;
      //   border: none;
      // }
      .el-form-item.tableNoPadding{ // 11.7从formMaking的css剥离
        padding: 0px;
      }

      .el-table .el-table__header-wrapper thead th .cell{ // 11.29添加子表单表哥头居中
        text-align: center !important;
        color: #000 !important;
        font-size: 14px !important;
      }
      .el-table .el-table__header-wrapper th { // 2025.8.20加的样式，匹配子表单的合并表头
        background: transparent;
        border-bottom: 1px solid rgb(153, 153, 153);
      }

      .fm-form-item:has(.title){ // 表格头标题对齐
        vertical-align: middle !important;
      }
      .fm-form-item .el-form-item.title .el-date-editor .el-input__inner,
      .fm-form-item .el-form-item.title .el-input-number .el-input__inner,
      .fm-form-item .el-form-item.title .el-select .el-input__inner,
      .fm-form-item .el-form-item.title .custom-container .el-textarea__inner,
      .fm-form-item .el-form-item.title .custom-container .el-input__inner { // 表格头标题对齐（其他控件，非文字控件） { // 表格头标题对齐
        font-weight: 700;
        font-size: 28px !important;
        text-align:center;
        height: 36px;
      }

      .btnCenter .fm-upload-file { // 按钮居中（文件上传）
        text-align: center;
      }

      // 兼容老版本
      .form-table .el-table--border {
        border: none !important;
      }

      .fm-form-item:has(.title),.fm-form-item:has(.sub-title){
        margin-bottom: 10px !important;
      }

      // 子表单+按钮样式
      .form-subform-item .form-subform-index .el-tag {
        margin-top: 10px;
      }
      .form-subform-item.is-hover .form-subform-remove .el-popover__reference {
        margin-top: 10px;
      }

      // 文字组件必填红点样式
      .el-form-item.showRedPot .el-form-item__label::before {
        content: "*";
        color: red;
        margin-right: 5px;
      }
    }
  }
}


::v-deep {
  .fm-form .fm-report-table__table .el-table__cell .cell,.fm-form .form-table .el-table__cell .cell { // 子表单和表格单元格间距
    padding: 0;
  }
  .fm-form .fm-report-table__table .el-table__cell,.fm-form .form-table .el-table__cell { // 子表单和表格单元格间距
    padding: 0;
  }
  .fm-form .el-table th.el-table__cell.required>div.required::before{
    content: '*';
    color: #f56c6c;
    margin-right: 4px;
    background: transparent;
    vertical-align: top;
  }
  .fm-form .el-table--border .el-table__cell:first-child .cell {
    padding-left: 0px;
  }

  .fm-form .el-table th.el-table__cell.is-leaf,.fm-form .el-table td.el-table__cell { // 新版本分享
    border-color: rgb(153, 153, 153);
  }
  .fm-form .el-textarea__inner{
    overflow:auto;
  }
  .fm-form .hasAutosize.el-textarea .el-textarea__inner{
    overflow: hidden !important;
    padding: 2px !important;
  }

  .el-form-item.titleBold .el-input__inner { // 标题是选公司的组件样式（11.28）
    font-size: 28px !important;
    font-weight: 700 !important;
    height: 40px;
    line-height: 40px;
    border: none;
  }
  .el-form-item.title .el-input__inner {
    border: none;
  }

  // 老版本
  .fm-form .form-table .el-table--border {
    border: 1px solid rgb(153, 153, 153) !important;
  }

  // 自定义的通用信息组件，多行文本不用显示滚动条
  .fm-form .custom-container .info-textarea .el-textarea__inner {
    overflow: hidden !important;
    padding: 2px !important;
  }
}

@media screen and (max-width: 1440px) {
  ::v-deep {
    .el-dialog__body {
      .flow-content {
        overflow: auto;
      }
    }
  }
}

</style>
