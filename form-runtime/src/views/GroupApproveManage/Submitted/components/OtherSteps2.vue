<!--
 * @Descripttion:
 * @Author: 处理公司其他类型的表单（有表单）
 * @Date: 2021-11-02 11:44:35
-->
<template>
  <div class="form-container" v-loading="loading" style="padding-bottom: 50px;">
    <!-- 公共上传附件 -->
    <AttachList v-if="isShowAttachList" ref="attachList" :models="models" :uploading="uploading" :enableData="enableData" :disabledData="disabledData"
      :jsonData="jsonData" :isShowList="true" fromPage="startFlow" />
    <!-- 公共关联流程 -->
    <FormCommonFlowPost ref="flowListSelect"/>

    <fm-generate-form :data="jsonData" :value="editData" @on-change="onChange" @on-file-upload="onFileUpload"
      @openCompanyPersonFramwork="openCompanyPersonFramwork" @openRelationOrganizationDialog="openRelationOrganizationDialog"
      ref="generateForm" :custom-fields="customFields" @viewRequestFrom="viewApprovalForm2" @viewFrom="viewApprovalForm" @openGeneralList="openGeneralList" >
    </fm-generate-form>

    <!-- 自己已发的流程列表 -->
    <!-- <MySendFlowList v-if="flowListVisible" :visible.sync="flowListVisible" @confirmFlow="confirmFlow"></MySendFlowList> -->

    <!-- 集团人员选择 -->
    <IndicatorHeaderDialog :visible.sync="indicatorHeaderVisible" :fielSelectType="fielSelectType" :companyId="companyId" :selectUserCompanyId="selectUserCompanyId" :departmentId="departmentId" :relDepartmentId="relDepartmentId"
      v-if="indicatorHeaderVisible" :isRelative="isRelative" @selectHeader="selectHeader" />
      <!-- 集团人管岗位关联选择 -->
      <RelationOrganizationDialog v-if="relationOrganizationVisible" :visible.sync="relationOrganizationVisible" :fielSelectType="fielSelectType" @selectValue="selectValue" ></RelationOrganizationDialog>
    <!-- 查看表单 -->
    <ApprovalFormDialog v-if="viewForm.visible" :visible.sync="viewForm.visible" :flowInstanceId="viewForm.flowInstanceId" :formId="viewForm.formId"></ApprovalFormDialog>

    <!-- 公共列表 -->
    <generalList v-if="generalVisible" :visible.sync="generalVisible" :config="config" @confirmChoose="confirmChoose">
    </generalList>
  </div>
</template>

<script>
import AttachList from '@/components/AttachList/index.vue';
import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue'
import RelationOrganizationDialog from '@/components/RelationOrganizationDialog.vue'
import MySendFlowList from './MySendFlowList.vue'
import ApprovalFormDialog from '../../Submitted/components/ApprovalFormDialog.vue'
// import FormCommonFlowPost from '@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue'

import generalList from '@/components/generalList.vue'

import Api from '@/api';
import { baseUrl } from '@/config/env';
import customJson from '@/components/Custom/customJson'
import { localstorageSet,localstorageGet } from '@/utils/auth';
import mixin from './mixin';
const FormCommonFlowPost = () => import('@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue');
export default {
  name: 'OtherSteps2',
  components: {
    MySendFlowList,
    IndicatorHeaderDialog,
    AttachList,
    ApprovalFormDialog,
    RelationOrganizationDialog,
    FormCommonFlowPost,
    generalList
  },
  mixins:[mixin],
  props: {
    flowId: {
      type: [Number, String],
      default: ''
    },
    formId: {
      type: [Number, String],
      default: ''
    },
    steps: {
      type: [Number, String],
      default: ''
    },
    selectFlowType: {
      type: String,
      default: ''
    },
    contractId: { // 合同Id
      type: String,
      default: ''
    },
    companyId: { // 公司Id
      type: String,
      default: ''
    },
  },
  data() {
    return {
      flowListVisible:false,
      customFields: customJson,
      loading: false,
      jsonData: {},
      editData: {
      },
      indicatorHeaderVisible: false,
      currentField: '',
      currentRowIndex: '',
      currentTable: '',
      fielSelectType: '',
      relDepartmentId:'',
      // companyId:'',
      models: [],
      uploading: [],
      enableData: [],
      disabledData: [],
      formPersonFields: [],
      selectData: null,
      setDataProps:'',
      setId:{
        user_name:'user_id',
        company_name:'company_id',
        department_name:'department_id',
        post_name:'post_id',
        mentor_name:'mentor_id',
        department_manager_name:'department_manager_id',
      },
      viewForm:{
        flowInstanceId:'',
        formId:'',
        visible:false
      },
      selectUserCompanyId:'',
      departmentId:'',
      relationOrganizationVisible:false,
      generalVisible:false,
      isRelative:true,
      config:{}
    };
  },
  watch: {
    flowId: {
      handler(val) {
        if (val) {
          console.log(val,'++++++')
          this.getFormDetail();
        }
      },
      immediate: true
    },
    jsonData: {
      handler(val) {
        if (val.list) {
          console.log('val.list',val.list)
          this.$nextTick(() => {
            if (this.selectFlowType == 'contract_compliance_review') { // 合同合规性评审
              // 从我的合同页面过来，发起流程(这些route获取的逻辑后面可以放到表单里，在这里不用显示太多代码)
              const params = this.$route.params;
              // const params = this.$route.query;
              console.log('从我的合同页面过来，回显数据',this.$route)
              if (params && params.from) {
                if (params.from == 'contract_compliance_review') {
                  // 合同合规性表单-表单控制自定义字段
                  this.$refs.generateForm.setOptions(['custom_contractLegalField'], {
                    extendProps: {
                      isFlowInitiate: true,
                      businessId: params.contractId,
                      companyId: params.companyId,
                      pageTemplateId: params.templateId,
                      formPage: params.formPage,
                      isExamine: true, // 是否审核状态
                    }
                  })
                  setTimeout(x=>{
                    // 合规性评审发起时需要回显数据
                    let custom_contractLegalField = this.$refs.generateForm.getComponent('custom_contractLegalField') // 合同盖章评审表中自定义的业务组件
                    custom_contractLegalField.initContract(this.$refs.generateForm,params);
                    // custom_contractLegalField.showMyInfo(this.$refs.generateForm,'发起');
                  },200)
                }
              } else { // 正常的发起流程
                // 合同合规性表单-表单控制自定义字段
                this.$refs.generateForm.setOptions(['custom_contractLegalField'], {
                  extendProps: {
                    isFlowInitiate: true,
                    businessId:'',
                    // isExamine: false, // 是否审核状态
                  }
                })
                // setTimeout(x=>{
                //   let custom_contractLegalField = this.$refs.generateForm.getComponent('custom_contractLegalField') // 合同合规评审表中自定义的业务组件
                //   custom_contractLegalField.showMyInfo(this.$refs.generateForm,'发起');
                // },200)
              }
            } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审
              localstorageSet('isValueInit', false);// 盖章表单由于初始化有赋值，所以在合同主体处需要这个进行判断
              // 合同盖章评审表单-表单控制自定义字段
              this.$refs.generateForm.setOptions(['custom_contractSealField'], {
                extendProps: {
                  isFlowInitiate: true,
                  businessId: this.contractId,
                  companyId: this.companyId,
                }
              })
              setTimeout(x=>{
                // 合同盖章评审发起时需要回显数据
                let custom_contractSealField = this.$refs.generateForm.getComponent('custom_contractSealField') // 合同盖章评审表中自定义的业务组件
                console.log('custom_contractSealField',custom_contractSealField)
                // 隐藏合规评审，暂时注释
                console.log(3333)
                custom_contractSealField.getContractInfo(this.$refs.generateForm,this.contractId);
                console.log(444)
              },200)
            }

          });
        }
      },
      immediate: true
    },
    'editData'(val) {
      if (val) {
        this.$nextTick(() => {
          this.$refs.generateForm.refresh();
        });
      }
      // if (val) {
      //   if (this.editType != 3) {
      //     this.$refs.makingform.setJSON(this.form.jsonData);
      //   } else {
      //     this.$nextTick(() => {
      //       this.$refs.generateForm.refresh();
      //     });
      //   }
      // }
    }
  },
  computed:{
    isShowAttachList(){
      const noShowFileType=['travel_expense','expense_loan','expense_repayment','request_funds','contract_compliance_review','contract_seal_review', 'expense_budget_noBelong']
      // this.selectFlowType =='travel_expense'||
      if(noShowFileType.includes(this.selectFlowType)){
        return false
      }else{
        return true
      }
    }
  },
  created() { },
  mounted() {
  },
  methods: {
    openGeneralList(obj){
      this.config = obj
      this.generalVisible = true
    },

    viewApprovalForm(data){
      console.log(data,'data+++')
      const parmas = {
          flowName: '',
          useScope: 'invest',
          auditWayList: ['expense_loan'],
          statusList:['end'],
          // id:data.formProxyId
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'expense_loan',
              otherBizId:data?.argument?.row?.expenseReimbursementId
            },
          ],
        };
        if(this.selectFlowType == 'assets_transfer'){
          delete parmas.flowInstanceBizRelevanceList
          parmas.initiator = "all"
          parmas.id = data.id
          parmas.auditWayList= []
          parmas.statusList = ['run','end']
        }
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data:parmas,
            pagination: true,
            pages: 1,
            size: 999
          },
          res => {
            if (res.isSuccess) {
              const flow = res.data[0];
              // this.selectFlowType = flow.auditWay;
              // this.isExamine = false;
              // this.isReInitiate = false;
              // this.flowId = flow.flowProxyId;
              // this.formId = flow.formProxyId;
              // this.flowNodeProxyId = flow.currentNodeProxyId;
              // this.flowInstanceId = flow.id;
              // this.jobTaskId = flow.jobTaskId;
              // this.examineDialogVisible = true;
              // const find = flow.flowInstanceBizRelevanceList.find(item => item.otherBiz == flow.auditWay);
              // this.businessId = find?.otherBizId || '';
              this.viewForm.formId = flow.formProxyId
              this.viewForm.flowInstanceId = flow.id;
              this.viewForm.visible = true
            } else {
              this.$message.error(res.message);
            }
          }
        );
      // console.log(this.viewForm,'this.viewForm++',data)
      // // // this.$refs.generateForm.getValue()
      // // console.log(this.$refs.generateForm.getValue(data.argument.group),'++++')
      // const row = this.$refs.generateForm.getValue(data.argument.group)
      // const currentIndex =  data.argument.rowIndex
      // this.viewForm.formId = row[currentIndex].formProxyId
      // this.viewForm.flowInstanceId = row[currentIndex].processId
      // this.viewForm.visible = true
      // console.log(this.viewForm,'this.viewFrom+++')
    },
    viewApprovalForm2(data){
      console.log(data,'data+++')
      const parmas = {
          flowName: '',
          useScope: 'invest',
          auditWayList: ['request_funds'],
          statusList:['end'],
          // id:data.formProxyId
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'request_funds',
              otherBizId:data?.argument?.row?.expenseReimbursementId
            },
          ],
        };
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data:parmas,
            pagination: true,
            pages: 1,
            size: 999
          },
          res => {
            if (res.isSuccess) {
              const flow = res.data[0];
              this.viewForm.formId = flow.formProxyId
              this.viewForm.flowInstanceId = flow.id;
              this.viewForm.visible = true
            } else {
              this.$message.error(res.message);
            }
          }
        );
    },
    // 打开我已发起流程列表
    // openMyFlow(){
    //   this.flowListVisible = true;
    // },
    // confirmFlow(row){
    //   console.log('flow',row);
    // },
    selectHeader(data, selectCompany,depart) { // 人员选择选择赋值
      this.selectData = data;
      if(selectCompany){
        this.$refs.generateForm.setData({ handleCompany: selectCompany.name });
      }
      // 员工试用期考核表用
      console.log(this.currentField,'this.currentField++++++')
      if(this.currentField=='company_name'){
        if(data.type==1){
          this.$refs.generateForm.setData({ currentCompanyId: data.id });
        }else{
          this.$refs.generateForm.setData({ currentCompanyId: selectCompany.id });
        }
      }
      if(this.currentField=='departmentName'){
        this.$refs.generateForm.setData({ departmentId: data.id,departmentName:data.name });
        //如果选择部门，则公司id和名称赋值给固定field
        this.$refs.generateForm.setData({ company_name: selectCompany.name });
        this.$refs.generateForm.setData({ currentCompanyId: selectCompany.id });
      }
      console.log('this.currentField',this.currentField)
      if(this.currentField=='company_dept'){
        let obj = {},currentField = 'userName'
        if(data.type == 1 && selectCompany.type == 1){//这个是选择公司
          obj['type'] = 'company'
          obj[currentField + '_dept'] = '';
          obj[currentField + '_deptid'] = '';
          obj[currentField + '_company'] = data.name;
          obj[currentField + '_companyid'] = data.id;
        }
        if(data.type == 2 && selectCompany.type == 1){ //选择部门
          obj['type'] = 'dep'
          obj[currentField + '_dept'] = data.name;
          obj[currentField + '_deptid'] = data.id;
          obj[currentField + '_company'] = selectCompany.name;
          obj[currentField + '_companyid'] = selectCompany.id;
        }
        this.$refs.generateForm.setData(obj);
        return
      }
      //如果选择人员，则部门公司信息全部带上
      if(this.currentField=='person'){
        this.$refs.generateForm.setData({ personId: data.id });
        if(depart){
          this.$refs.generateForm.setData({ departmentName: depart.name });
          this.$refs.generateForm.setData({ departmentId: depart.id });
        }
        //如果选择部门，则公司id和名称赋值给固定field
        this.$refs.generateForm.setData({ company_name: selectCompany.name });
        this.$refs.generateForm.setData({ currentCompanyId: selectCompany.id });
      }
      if (this.currentTable) { // 子表单赋值
        const tableArr = this.$refs.generateForm.getValue(this.currentTable);
        tableArr[this.currentRowIndex][this.currentField] = data?.name;
        tableArr[this.currentRowIndex][this.currentField+'_id'] = data?.id
        tableArr[this.currentRowIndex][this.currentField+'_dept'] = depart?.name
        tableArr[this.currentRowIndex][this.currentField+'_deptid'] = depart?.id
        // 设置id
        if(this.setId.hasOwnProperty(this.currentField)){
          tableArr[this.currentRowIndex][this.setId[this.currentField]] = data.id;
          if(this.currentField=='company_name'||this.currentField=='department_name'){
            tableArr[this.currentRowIndex]['post_name'] = ''
            tableArr[this.currentRowIndex]['post_id'] = ''

          }
        }
        const obj = {};
        obj[this.currentTable] = tableArr;
        this.$refs.generateForm.setData(obj);
      } else {
        const obj = {};
        obj[this.currentField] = data.name;
        obj[this.currentField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
        obj.user_phone = data.companyPhone;//退休通知单需要
        if(this.fielSelectType == 'selectCompany'){
          if(this.currentField.indexOf('_company') > -1){
            let currentField = this.currentField.replace('_company','')
            obj[currentField] = '';
            obj[currentField + '_id'] = ''
            obj[currentField + '_dept'] = '';
            obj[currentField + '_deptid'] = '';
            obj[currentField + '_company'] = data.name;
            obj[currentField + '_companyid'] = data.id;
          }
        }else if(this.fielSelectType == 'department'){
          if(this.currentField.indexOf('_dept') > -1){
            let currentField = this.currentField.replace('_dept','')
            obj[currentField] = '';
            obj[currentField + '_id'] = ''
            obj[currentField + '_dept'] = data.name;
            obj[currentField + '_deptid'] = data.id;
            obj[currentField + '_company'] = selectCompany.name;
            obj[currentField + '_companyid'] = selectCompany.id;
          }else{
            obj[this.currentField] = data.name;
          }
        }else if(this.fielSelectType == 'company'){
           let copyField = this.currentField.split('_')[0] // 只要_id或者_name前面的字符标识
           obj[copyField + '_name'] = data.name;
           obj[copyField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
           obj[copyField + '_dept'] = depart.name;
           obj[copyField + '_deptid'] = depart.id;
           obj[copyField + '_company'] = selectCompany.name;
           obj[copyField + '_companyid'] = selectCompany.id;
           // obj[this.currentField] = data.name;
           // obj[this.currentField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
           // obj[this.currentField + '_dept'] = depart.name;
           // obj[this.currentField + '_deptid'] = depart.id;
           // obj[this.currentField + '_company'] = selectCompany.name;
           // obj[this.currentField + '_companyid'] = selectCompany.id;
        }

        // obj[this.currentField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
        this.$refs.generateForm.setData(obj);
        // 设置id
        if(this.setId.hasOwnProperty(this.currentField)){
          console.log(99999,this.currentField)
          this.$refs.generateForm.setData({[this.setId[this.currentField]]:data.id});
          if(this.currentField=='company_name'||this.currentField=='department_name'){
            const restObj = {
              post_name:'',
              post_id:'',
            }
            this.$refs.generateForm.setData(restObj);
          }
        }
      }

      // 人员增补审批表  获取
      if(this.fielSelectType=='duty'&&this.setDataProps){
        this.$axios.post(
        Api.frameworkInfo.getDutyList,
        {
          data: {
            departmentId: data.parentId
          }
        },
        res => {
          if (res.isSuccess) {
            const currentPost = res.data.filter(item=>{
              return item.id == data.id
            })
            this.$refs.generateForm.setData({[this.setDataProps]:currentPost[0]?.total});
            console.log(currentPost,'currentPost++')
          } else {
            this.$message.error(res.message);
          }
        }
        )

      }
    },
    selectValue(post,department,company){
      if(this.currentField=='post_name'){
        const obj = {
          post_name:post.name,
          post_id:post.id,
          department_name:department.name,
          department_id:department.id,
          company_name:company.name,
          company_id:company.id,
        }
        this.$refs.generateForm.setData(obj);
      }
      if(this.currentField=='company_department_post_name'){
        const obj = {
          company_department_post_name:company.name+'/'+department.name+'/'+post.name,
          post_id:post.id,
          department_id:department.id,
          company_id:company.id,
        }
        this.$refs.generateForm.setData(obj);
      }
    },
    // changeScore(data){
    //   const formData = this.$refs.generateForm.getValues();

    //   const targetContent = ['workPerformance','workAttitude','workCompetence','workEfficiency','selfDiscipline','teamwork','executive','learningAbility']
    //   // ['工作绩效','工作态度','工作能力','工作效率','自律性','团队合作','执行力','学习能力']
    //   const arr=[]
    //   Object.keys(formData).map(item=>{

    //     targetContent.map(k=>{
    //       if(item.includes(k)){
    //       arr.push(item)
    //     }
    //     })
    //   })
    //   const list = []
    //   targetContent.map(item=>{
    //     const obj = {[`${item}`]:{}}
    //     arr.map(k=>{
    //       if(k.includes(item)){
    //         const g = k.split('_')[1]
    //         const sort = k.split('_')[2]
    //         obj[item][g]=formData[k]
    //         if(sort){
    //           obj[item].sort = sort
    //         }
    //       }
    //     })
    //     list.push(obj)
    //   })
    //   console.log(list,'list++++++')
    //   this.$refs.generateForm.setData({score_list:list});
    // },
    openRelationOrganizationDialog(data){
      console.log(77777)
      const { field, rowIndex, table,group } = data.argument;
      this.selectUserCompanyId = data.userCompanyId
      this.fielSelectType = data.fielSelectType;
      this.companyId =data.companyId||'';
      this.setDataProps = data.setData||'';
      this.currentField = field;
      this.currentRowIndex = rowIndex;
      this.currentTable = table||group;
      this.relationOrganizationVisible = true;
    },
    openCompanyPersonFramwork(data, data2) {
      console.log(8888)
      const { field, rowIndex, table,group,isRelative } = data.argument;
      this.selectUserCompanyId = data.userCompanyId
      this.fielSelectType = data.fielSelectType;
      this.companyId = this.$refs.generateForm.getValue('currentCompanyId')||data.companyId||'';
      this.setDataProps = data.setData||'';
      this.currentField = field;
      this.currentRowIndex = rowIndex;
      this.currentTable = table||group;
      if(this.fielSelectType=='duty'){ //岗位部门关联
        console.log(this.$refs.generateForm.getValues(),this.currentTable)
        if(this.currentTable){
         const  currentTable = this.$refs.generateForm.getValue(this.currentTable)
          this.departmentId = currentTable[this.currentRowIndex].department_id
        }else{
          this.departmentId = this.$refs.generateForm.getValue('department_id')||this.$refs.generateForm.getValue('company_id')
        }
        console.log(this.departmentId,'this.departmentId')
      }else{
        this.departmentId = ''
      }
      // 人员需求表部门关联
      // console.log(this.$refs.generateForm.getValue('handlingCompanyAndDepartment'),'5555555555555555')
      const handlingCompanyAndDepartment =this.$refs.generateForm.getValue('handlingCompanyAndDepartment')
      if(handlingCompanyAndDepartment){
        console.log(77777)
        const company = JSON.parse(handlingCompanyAndDepartment)
        if(company.type==2){
          this.relDepartmentId = company.id
        }
        this.companyId =company.companyId||company.id
      }else{
        this.relDepartmentId = ''
      }
      if(isRelative === false)this.isRelative = false
      this.indicatorHeaderVisible = true;
    },
    async getFormDetail() {
      console.log('getFormDetail')
      this.loading = true;
      // const enableData = this.enableData = await this.getFlowFindById();
      // 区分一下需要隐藏和不需要设置隐藏的字段
      const allPowerFieldData = await this.getFlowFindById();
      const enableData = this.enableData = allPowerFieldData.enableData;
      const hideData = allPowerFieldData.hideData;// 隐藏字段注释
      // console.log('enableData',enableData)
      // console.log('hideData',hideData)

      this.$axios.post(
        Api.schedule.getFormDetail,
        {
          data: {
            id: this.formId
          }
        },
        (res) => {
          this.loading = false;
          if (res.isSuccess) {
            const { data } = res;
            let disabledData = [];
            if (data) {
              disabledData = this.disabledData = data.fieldsTemplateList.map(item => (item.name ?? '').replaceAll('_$$_', '.'));
              console.log('this.disabledData',this.disabledData)
              console.log('this.enableData',this.enableData)
              // this.jsonData = JSON.parse(data.templateData);
              let copyTemplateData = JSON.parse(data.templateData);
              this.setRequireByPermission(copyTemplateData.list);
              this.jsonData = JSON.parse(JSON.stringify(copyTemplateData));
              // console.log('this.jsonData',this.jsonData)
              this.$nextTick(() => {
                this.$refs.generateForm.refresh();
                this.$refs.generateForm.disabled(disabledData, true);
                this.$refs.generateForm.disabled(enableData, false);
                this.$refs.generateForm.hide(hideData);// 隐藏字段注释

                // this.$refs.generateForm.disabled(["input1","purpose"], false);
                // this.$refs.generateForm.disabled(['input2','invoiceInformationVoList.purpose'], false)
                this.$emit('fmDone') //formaking加载完
              });
            } else {
              disabledData = [];
              this.jsonData = {
                config: {},
                list: []
              };
              this.$nextTick(() => {
                this.$refs.generateForm.refresh();
                this.$emit('fmDone') //formaking加载完
              });
            }
          }
        }
      );
    },
    // 递归表单，根据配置的表单权限，修改表单是否需要校验配置（如果某个表单字段本身设置必填，但是没有配置权限，那么会调整为非必填；如果本身未设置必填，就算有权限，也不需要必填）
    setRequireByPermission(genList) {
      // 1、子表单暂时未考虑子表单+这种容器的情况
      genList.map((item, key) => {
        if (item.type == 'table') { // 子表单容器（不支持布局嵌套）
          if (item.tableColumns) {
            // console.log('item.tip',item.options.customClass.indexOf('subFormNoNeedPermission')>-1)
            if (item.options.customClass.indexOf('subFormNoNeedPermission') == -1) { // 子表单如果不需要控制每列权限，就要加这个样式
              item.tableColumns.forEach(y=>{
                if (!this.enableData.includes(y.model + '_col')) { // _col后缀用于配合制作多一个表格用于控制子表单每列权限
                  this.$set(y.options,'required',false);
                  this.$set(y,'rules',[]);
                }
              })
            }
          }
        } else if (item.type == 'grid') {
          item.columns.map(val => {
            this.setRequireByPermission(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.setRequireByPermission(i.list);
            });
          });
        } else if (item.type == 'inline') {
          this.setRequireByPermission(item.list);
        } else {
          if (item.model) {
            if (!this.enableData.includes(item.model.replaceAll('_$$_', '.'))) {
              // 下面两行代码足够覆盖所有场景，即用下面两行；不够用可把注释打开，根据不同场景进行配置。
              // if (item.options.validatorCheck) { // 自定义校验规则
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.validatorCheck && item.options.required) { // 自定义校验规则还点了必填
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.patternCheck && item.options.required) { // 正则校验
              //   this.$set(item.options,'patternCheck',false);
              // } else if (item.options.dataTypeCheck && item.options.required) { // 表单内置的校验规则
              //   this.$set(item.options,'dataTypeCheck',false);
              // }
              this.$set(item.options,'required',false);
              this.$set(item,'rules',[]);
              // console.log('item2',item)
            }
          }
        }
      });
    },
    getFlowFindById() {
      console.log('getFlowFindById')
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.schedule.flowTemplateFindById,
          {
            data: {
              id: this.flowId
            }
          },
          (res) => {
            console.log(8888,res)
            if (res.isSuccess) {
              this.flowTemplateData = res.data; // 保存流程模板数据
              let formTemplateBizRelevanceVoList = res?.data?.formTemplateBizRelevanceVoList
              if(formTemplateBizRelevanceVoList.find(el=>el.otherBiz == 'company')){
                this.$emit('update:flowProjectId','')
              }

              this.setFormPersonFields(res.data.flowNodeTemplate); // 节点指定表单人员，进行处理
              let flowNodeFieldPowerTemplateList = res.data?.flowNodeTemplate?.flowNodeFieldPowerTemplateList || []
              // const enableData = flowNodeFieldPowerTemplateList.map(item => item.formFieldTemplateEnglishName);
              const enableData = flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower != 'hide').map(item => (item.formFieldTemplateEnglishName ?? '').replaceAll('_$$_', '.'));
              const hideData = flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower == 'hide').map(item => item.formFieldTemplateEnglishName);// 隐藏字段注释



              if (res.data.flowNodeTemplate.childFlowNodeTemplate.type == 'parallel') {
                // 处理发起人后并行节点审批人自选
                const parallelChooseNodes = [];
                res.data.flowNodeTemplate.childFlowNodeTemplate.parallelNodes.forEach(parallelNode => {
                  const auditType = parallelNode.childFlowNodeTemplate.flowNodeAuditConfig.auditType;
                  if (auditType == 'run_node_choose') {
                    // 取出其中的审批人自选节点
                    parallelChooseNodes.push({
                      nodeName: parallelNode.childFlowNodeTemplate.nodeName,
                      nextNodeTemplateId: parallelNode.nextNodeTemplateId,
                      auditType: auditType,
                      nodeAuditList: []
                    });
                  } else if (auditType == 'department_supervisor' || auditType == 'branched_passage_manager') {
                    // 取出其中的主管审批和副总审批的类型节点
                    parallelChooseNodes.push({
                      nodeName: parallelNode.childFlowNodeTemplate.nodeName,
                      nextNodeTemplateId: parallelNode.nextNodeTemplateId,
                      auditType: auditType,
                      nodeAuditList: []
                    });
                  }
                });

                this.$emit('update:parallelChooseNodes', parallelChooseNodes);
              }
              if (res.data.flowNodeTemplate.childFlowNodeTemplate.branchExecuteType == 'custom_choose') {
                // 处理发起人后的手动分支选择
                const manualChooseNodes = [];
                res.data.flowNodeTemplate.childFlowNodeTemplate.conditionNodes.forEach(branch => {
                  manualChooseNodes.push(
                    {
                      nextNodeTemplateId: branch.nextNodeTemplateId,
                      nodeName: branch.childFlowNodeTemplate.nodeName,
                      nodeType: branch.childFlowNodeTemplate.type, // 为处理手动分支
                      branchName: branch.name,
                      auditType: branch.childFlowNodeTemplate.flowNodeAuditConfig.auditType
                    }
                  );
                });
                this.$emit('update:manualChooseNodes', manualChooseNodes);
              }
              if(res.data.auditWay=='probation_assessment'){ //员工试用期考核表
                  this.$nextTick(() => {
                    this.$refs.generateForm.hide('probation_employee_approval_form_name')
                  });
                }
              resolve({
                hideData,// 隐藏字段注释
                enableData
              });
              // resolve(enableData);
            }
          }
        );
      });
    },
    // 发起人下一个节点，如果auditType等于"form_person"
    setFormPersonFields(data) {
      if (data.childFlowNodeTemplate?.flowNodeAuditConfig?.auditType == "form_person") {
        this.formPersonFields.push({
          bizId: data.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
          nodeProxyId: data.childFlowNodeTemplate.id
        });
      }
      if (data.childFlowNodeTemplate.conditionNodes && data.childFlowNodeTemplate.conditionNodes.length) {
        data.childFlowNodeTemplate.conditionNodes.forEach(i => {
          if (i.childFlowNodeTemplate.flowNodeAuditConfig&&i.childFlowNodeTemplate.flowNodeAuditConfig.auditType == "form_person") {
            this.formPersonFields.push({
              bizId: i.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
              nodeProxyId: i.childFlowNodeTemplate.id
            });
          }
        });
      }
      if (data.childFlowNodeTemplate.parallelNodes && data.childFlowNodeTemplate.parallelNodes.length) {
        data.childFlowNodeTemplate.parallelNodes.forEach(j => {
          if (j.childFlowNodeTemplate.flowNodeAuditConfig&&j.childFlowNodeTemplate.flowNodeAuditConfig.auditType == "form_person") {
            this.formPersonFields.push({
              bizId: j.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
              nodeProxyId: j.childFlowNodeTemplate.id
            });
          }
        });
      }
      console.log(this.formPersonFields, 'this.formPersonFields--zxf');
    },
    confirmChoose(chooseData){
      if(typeof(chooseData) == 'object'){
        this.$refs.generateForm.setData({[this.config.field]:JSON.stringify(chooseData)});
      }else{
        this.$refs.generateForm.setData({[this.config.field]:chooseData});
      }
    },
    onFileUpload(obj) {
      this.uploading = obj;
    },
    onChange(field, val, models) {
      this.models = models;
    }
  }
};
</script>

<style lang="scss" scoped>
::v-deep .el-form-item--small .el-form-item__error{
  z-index: 10 !important;
}
::v-deep .el-form-item__error{
  z-index: 10 !important;
}
// ::v-deep .fm-form .fm-report-table__table .fm-report-table__td {
//   padding: 3px !important;
// }
::v-deep .el-input.onlyRead .el-input__inner {
  border: none;
  cursor: not-allowed;
}
</style>
