<!--
 * @Descripttion: 合同盖章评审业务
 * @Author: zhengzetao
 * @Date: 2024-05-27 15:35:38
-->
<template>
  <div>
    <!-- 合同盖章评审业务逻辑 -->
  </div>
</template>

<script>
import { viewFileUrl } from '@/config/env.js';
import { localstorageSet,localstorageGet } from '@/utils/auth';
import { generateRandomId,findPathById,getObjById } from "@/utils";


import Api from '@/api';
import { baseUrl } from '@/config/env';

/* eslint-disable */
export default {
  name: 'ContractSealReviewBusiness',
  components: {},
  props: {
    isFlowInitiate: { // 是否是流程发起阶段
      type: Boolean,
      default: false
    },
    businessId: { // 业务id
      type: String,
      default: ''
    },
    // isExamine: { // 是否审核状态
    //   type: Boolean,
    //   default: true
    // },
    // isReInitiate: { // 是否草稿重新发起状态
    //   type: Boolean,
    //   default: false
    // }
    // contractSubmitType: { // 合同提交状态
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      contractRefFileList: [],
      contractList: [],
      relatedPartyList:[],
      currentContractInfo:{},
      thatInstance:null,
      contractBodyList:[],
      batchCode:'',
    };
  },
  watch:{
  },
  computed: {
    // pdfAction() {
    //   const sid = this.$store.state.user.token;
    //   return `${baseUrl}/web/file/api/file/uploadFile?sid=${sid}&platformCode=200001`;
    // },
  },
  created() {
    this.getContractBodyList();
  },
  mounted() {
    console.log('mounted')
    this.init();
  },
  methods: {
    initFormInstance(generateFormRefs){
      this.thatInstance = generateFormRefs;
      // console.log(111,generateFormRefs)
    },
    init(){
      // console.log('this.isExamine',this.isExamine)
      // console.log('this.isReInitiate',this.isReInitiate)
      console.log('this.isFlowInitiate',this.isFlowInitiate)
      console.log('this.businessId',this.businessId)
      this.getRelatedPartyByUser();

      if (!this.isFlowInitiate) { // 不是发起流程
        this.getContractInfo_check_auditing();
        // if (!this.isExamine && !this.isReInitiate) { // 两个状态为false时，代表已办的查看和已发的详情
        //   // 流程查看时，需要获取最新的盖章评审合同文件赋值表单（因为这个盖章文件有可能在其他地方重新上传了，但是表单提交流程时保存了数据，重新上传没更新）
        //   // this.getContractInfo_check_success();
        //   let fileRes = await this.getFileByContractId(this.businessId)
        //   console.log('fileRes--success',fileRes)
        // }
      } 
    },
    // 审批的时候，因为数据回显是模板带出来的，导致合同文件回显组件没有触发，赋值触发
    async initContractFile_inAudit(){
      console.log('initContractFile_inAudit')
      this.getContractInfo_check("1").then(async res=>{
        if (res.isSuccess) {
          let contractList = res.data.dataList || [];
          let currentContractInfo = contractList.find(x=>x.id == this.businessId)
          console.log("=====currentContractInfo=",currentContractInfo)
          // if (currentContractInfo.contractReviewLogVoList.length) {
          if (currentContractInfo?.contractReviewLogVoList) {
            let contractFile = await this.initContractFile(currentContractInfo.contractReviewLogVoList)
            console.log('contractFile',contractFile)
            console.log('thatInstance',this.thatInstance)
            this.thatInstance.setData({'seeFile':contractFile})
            console.log('setData1')
          }
          
        } else {
          this.$message.error(res.message);
        }
      });
    },
    // 已办的查看和已发的详情
    async getContractInfo_check_success(generateFormRefs){
      this.initContractFile_inAudit();
      console.log('getContractInfo_check_success')
      // 流程查看时，需要获取最新的盖章评审合同文件赋值表单（因为这个盖章文件有可能在其他地方重新上传了，但是表单提交流程时保存了数据，重新上传没更新）
      // this.getContractInfo_check_success();
      let fileRes = await this.getFileByContractId(this.businessId)
      console.log('fileRes--success',fileRes)
      let contractRefFileList = fileRes.contractRefFileTypeApiVo?.contractRefFileTypeVos || [];
      if (contractRefFileList && contractRefFileList.length>0) {
        contractRefFileList.forEach(async x=>{
          if (x.fileType == "sealContractFile") {
            // 2.根据业务id获取文件
            await this.getFileByBizId(x.bizId).then(z=>{
              let arr = []
              z.data.forEach(k=>{
                arr.push({
                  name: k?.originFileName || k.fileName,
                  formType: x.fileType,
                  status: "success",
                  url: k.fileUrl,
                  data: k
                })
              })
              console.log('arr',arr)
              // contractContend[x.fileType] = arr;
              generateFormRefs.setData({"sealContractFile":arr});
              console.log('setData2')
            })
          }
        })
      }
      
    },
    // 流程审批时获取合同信息-目的不是回显，而是获取信息里面的参数status、examineStatus传入最终保存合同信息的接口
    getContractInfo_check_auditing(){
      console.log('getContractInfo_check_auditing')
      this.getContractInfo_check("0").then(async res=>{
        if (res.isSuccess) {
          this.contractList = res.data.dataList || [];
          console.log('=======444555=========')
          console.log('this.businessId',this.businessId)
          console.log('this.contractList',this.contractList)
          console.log('this.thatInstance',this.thatInstance)
          // console.log('this.thatInstance.getValues()',this.thatInstance.getValues())

          

          let formVal = this.thatInstance.getValues();
          let contractTypeTree = await this.getContractTypeTree()
          console.log('contractTypeTree',contractTypeTree)
          let classId = formVal.classificationId instanceof Array ? (!!formVal.classificationId.length ? formVal.classificationId[formVal.classificationId.length-1] : '') : formVal.classificationId; // 是数组就取最后一个，不是就是取字符串
          console.log('classId',classId)

          let objResult = getObjById(contractTypeTree,classId,'children')
          console.log('objResult',objResult)

          if (!formVal['classificationId__virtualName']) { // 不存在虚拟字段，需要加上级联选择框的虚拟字段赋值，用于条件分支判断
            // formVal['classificationId__virtualName'] = objResult.name;
            this.thatInstance.setData({'classificationId__virtualName': objResult.name})
          }
          
          // 下面代码替代下面的文件请求
          console.log('formVal111',formVal)
          setTimeout(x=>{
            let seeFileData = formVal.seeFile;
            this.thatInstance.setData({'seeFile':seeFileData})
          },300)

          // this.currentContractInfo = this.contractList.find(x=>x.id == this.businessId)
          // console.log('this.currentContractInfo',this.currentContractInfo)
          // // if (!this.currentContractInfo.contractReviewLogVoList.length) {
          // if (this.currentContractInfo?.contractReviewLogVoList) {
          //   let contractFile = await this.initContractFile(this.currentContractInfo.contractReviewLogVoList)
          //   // console.log('contractFile',contractFile)
          //   console.log('thatInstance',this.thatInstance)
          //   this.thatInstance.setData({'seeFile':contractFile})
          //   console.log('setData3')
          //   console.log('this.thatInstance.getValues()',this.thatInstance.getValues())
          // }

          // 处理上面合同文件看不到的问题，暂时不用initContractFile这个方法
          let fileRes = await this.getFileByContractId(this.businessId)
          console.log('fileRes--4444',fileRes)
          // 这里暂时注释
          // let contractRefFileList = fileRes.contractRefFileTypeApiVo?.contractRefFileTypeVos || [];
          // if (contractRefFileList && contractRefFileList.length>0) {
          //   contractRefFileList.forEach(async x=>{
          //     // || x.fileType == "sealContractFile"
          //     if (x.fileType == "合同文件") {
          //       // 2.根据业务id获取文件
          //       await this.getFileByBizId(x.bizId).then(z=>{
          //         let arr = []
          //         z.data.forEach(k=>{
          //           arr.push({
          //             name:k.originFileName.split(".")[0],
          //             url:k.fileUrl,
          //           })
          //         })
          //         console.log('合同文件-arr',arr[0])
          //         this.thatInstance.setData({'seeFile': arr[0]})
          //       })
          //     }
          //   })
          // }


          // this.contractRefFileList审批时给这个数组赋值，用于后面判断是否已存在文件绑定的业务id
          this.contractRefFileList = fileRes.contractRefFileTypeApiVo?.contractRefFileTypeVos || [];
          console.log('this.contractRefFileList-2',this.contractRefFileList)

          // if (this.contractRefFileList.length>0) {
          //   this.contractRefFileList.forEach(async x=>{
          //     // 2.根据业务id获取文件
          //     await this.getFileByBizId(x.bizId).then(z=>{
          //       let arr = []
          //       z.data.forEach(k=>{
          //         arr.push({
          //           name: k.fileName,
          //           status: "success",
          //           url: k.fileUrl,
          //           data: k
          //         })
          //       })
          //       console.log('arr',arr)
          //     })
          //   })
          // }
        } else {
          this.$message.error(res.message);
        }
      });
    },
    getContractInfo_check(status) {
      console.log('getContractInfo_check',status)
      const params = {
        data: {
          // contractName: this.serachName,
          userId:this.$store.state.user.userId,
          companyId:this.$store.state.user.companyId,
          status:'1',  // 状态:0草稿1提交（合规审查状态） ；发起盖章了，合规应该走完了，应该就是写死1状态
          examineStatus:'1',// 审核状态:0审核中1审核成功2审核失败（合规审查状态）
          contractSubtableVo:{
            // stampStatus:'0',
            stampStatus:'1',// 状态:0草稿1提交（合同盖章状态）
            stampExamineStatus:status,// 审核状态:0审核中1审核成功2审核失败（合同盖章状态）
          }
        },
        pagination:false
      };
      return this.$axios.post(
        Api.contractManage.contractInfo.getContractList,
        params
      );
    },
    // 获取客户下相关方列表
    getRelatedPartyByUser(){
      // console.log('getRelatedPartyByUser')
      this.$axios.post(
        Api.contractManage.contractInfo.getRelatedPartyByUser,
        {
          data: {
          },
        },
        res => {
          if (res.isSuccess) {
            // console.log('获取客户下相关方列表',res.data)
            this.relatedPartyList = res.data || [];
          } else {
          }
        }
      );
    },
    
    // 初始化合同文件
    async initContractFile(voList){
      console.log('initContractFile',this.$route)
      let arr = this.$route.params?.contractReviewLogVoList || voList;
      // if (!arr.length) {
        let contractName = arr[arr.length-1]['contractName'];
        let fileRes = await this.getFileByBizId(arr[0]['id'])
        console.log('====fileRes===',fileRes)
        let fileObj = fileRes?.data[0] || {name:'',url:''};
        return new Promise((resolve, reject) => {
          resolve({
            name:contractName,
            url:fileObj['fileUrl'],
          });
        });
      // }
    },
    // 获取流程实例列表（查询流程详情，如果信息要很全，需要查列表匹配）
    async getFlowInstance(flowType,initiatorCompanyId) {
      const url = Api.schedule.getFlowInstanceList;
      let data = {
        flowName: '',
        useScope: 'invest',
        auditWayList: [flowType],
        statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'company',
            otherBizId: initiatorCompanyId
            // otherBizId: this.$store.state.user.companyId
          }
        ],
      };
      return new Promise((resolve, reject) => {
        this.$axios.post(
          url,
          {
            data,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data || [])
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 合同类型树
    getContractTypeTree(){
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.contractManage.typeSet.getEnableTreeList,{},res=>{
          if(res.isSuccess){
            let tree = res?.data || []
            resolve(tree)
          }
        })
      })
    },
    // 获取合同信息并在发起时回显合规性评审表保存的合同信息
    async getContractInfo(generateFormRefs,contractId) {
      console.log(111111111111111122221)
      console.log('报错-getContractInfo',this.$route.params)

      // 如果不隐藏合规评审，要删除
      // let that = this;
      // setTimeout(async x=>{
        // 下面几个需要关联各自流程
        let relativeFlowObj = {
          '单一来源采购': {
            flowType:'single_source_delegation',
            fieldName:'DYLYcaigou_file1_flow',
          },
          '内部采购': {
            flowType:'internal_delegation',
            fieldName:'NBcaigou_file1_flow',
          },
          '非挂网方式': {
            flowType:'proposed_supplier_review',
            fieldName:'FGWfangshi_file3_flow',
          },
        }
        let contractContend = generateFormRefs.getValues();
        for (var i in relativeFlowObj) { // 默认先给有关联流程功能的表单字段添加默认值
          let fieldName = relativeFlowObj[i]['fieldName'];
          let flowType = relativeFlowObj[i]['flowType'];
          contractContend[fieldName] = JSON.stringify({
            'flowType': flowType,
          })
        }
        generateFormRefs.setData(contractContend);
        // if (contractContend.contractTalk == '单一来源采购' || contractContend.contractTalk == '内部采购' || contractContend.contractTalk == '非挂网方式') { // 根据选择的谈判方式，如果需要关联流程，根据流程id查询详情赋值
        //   let flowType = relativeFlowObj[contractContend.contractTalk]['flowType'];
        //   let fieldName = relativeFlowObj[contractContend.contractTalk]['fieldName'];
        //   let flowList = await that.getFlowInstance(flowType,contractContend.companyId);
        //   console.log('flowList',flowList)
        //   console.log('22223333',contractContend.contractSubtableVo.processId)
        //   let flowDetail = flowList.find(x=>x.id == contractContend.contractSubtableVo.processId);
        //   // console.log('flowType',flowType)
        //   console.log('flowDetail',flowDetail)
        //   contractContend[fieldName] = JSON.stringify({
        //     'id': contractContend.contractSubtableVo.processId,
        //     'name': contractContend.contractSubtableVo.flowName,
        //     'flowType': flowType,
        //     'rowData': JSON.stringify(flowDetail),
        //     // 'rowData': JSON.stringify(contractContend),
        //   })
        //   console.log('contractContend2',contractContend)
        //   generateFormRefs.setData(contractContend);
        // }
      // },3000)

      // 隐藏合规评审，暂时注释
      // const params = {
      //   data: {
      //     // contractName: this.serachName,
      //     userId:this.$store.state.user.userId,
      //     companyId:this.$store.state.user.companyId,
      //     status:'1',
      //     examineStatus:'1',
      //     contractSubtableVo:{
      //       stampStatus:this.$route.params?.contractSubtableVo.stampStatus, // 发起的时候，只有草稿和空状态
      //       // stampStatus:this.$route.params?.contractSubtableVo.stampStatus == "2" ? , // 发起的时候，只有草稿和空状态
      //       stampExamineStatus:this.$route.params?.contractSubtableVo.stampExamineStatus,
      //       // stampStatus:'null',
      //       // stampExamineStatus:'null',
      //       // stampStatus:'0',
      //       // stampExamineStatus:'0',
      //     }
      //   },
      //   pagination:false

      // };
      // // console.log(11111,params)
      // this.$axios.post(
      //   Api.contractManage.contractInfo.getContractList,
      //   params,
      //   async (res) => {
      //     if (res.isSuccess) {
      //       this.contractList = res.data.dataList || [];
      //       console.log('this.contractList',this.contractList)
      //       console.log('contractId',contractId)
      //       let contractContend = this.contractList.find(x=>x.id == contractId)
      //       // console.log('contractContend',contractContend)
      //       // 合同发起-数据回显
      //       let contractFile = await this.initContractFile()
      //       // console.log('contractFile',contractFile)
      //       console.log('======contractContend=======',contractContend)
      //       // console.log('======contractId=======',contractId)
            
      //       let contractTypeTree = await this.getContractTypeTree()
      //       let classificationIdPath = findPathById(contractTypeTree,contractContend.classificationId)
      //       // console.log('======classificationIdPath=======',classificationIdPath)
      //       contractContend.classificationId = classificationIdPath;
      //       contractContend.classificationId__virtualName = contractContend.classificationName; // 盖章这里因为是回显，所以要加上级联选择框的虚拟字段赋值，用于条件分支判断
      //       console.log('发起时的文件',contractFile)
      //       contractContend.seeFile = contractFile;
      //       // console.log('contractContend2',contractContend)

      //       // 下面几个需要关联各自流程
      //       let relativeFlowObj = {
      //         '单一来源采购': {
      //           flowType:'single_source_delegation',
      //           fieldName:'DYLYcaigou_file1_flow',
      //         },
      //         '内部采购': {
      //           flowType:'internal_delegation',
      //           fieldName:'NBcaigou_file1_flow',
      //         },
      //         '非挂网方式': {
      //           flowType:'proposed_supplier_review',
      //           fieldName:'FGWfangshi_file3_flow',
      //         },
      //       }
      //       for (var i in relativeFlowObj) { // 默认先给有关联流程功能的表单字段添加默认值
      //         let fieldName = relativeFlowObj[i]['fieldName'];
      //         let flowType = relativeFlowObj[i]['flowType'];
      //         contractContend[fieldName] = JSON.stringify({
      //           'flowType': flowType,
      //         })
      //       }
      //       if (contractContend.contractTalk == '单一来源采购' || contractContend.contractTalk == '内部采购' || contractContend.contractTalk == '非挂网方式') { // 根据选择的谈判方式，如果需要关联流程，根据流程id查询详情赋值
      //         let flowType = relativeFlowObj[contractContend.contractTalk]['flowType'];
      //         let fieldName = relativeFlowObj[contractContend.contractTalk]['fieldName'];
      //         let flowList = await this.getFlowInstance(flowType,contractContend.companyId);
      //         // console.log('flowList',flowList)
      //         // console.log('22223333',contractContend.contractSubtableVo.processId)
      //         let flowDetail = flowList.find(x=>x.id == contractContend.contractSubtableVo.processId);
      //         // console.log('flowType',flowType)
      //         console.log('flowDetail',flowDetail)
      //         contractContend[fieldName] = JSON.stringify({
      //           'id': contractContend.contractSubtableVo.processId,
      //           'name': contractContend.contractSubtableVo.flowName,
      //           'flowType': flowType,
      //           'rowData': JSON.stringify(flowDetail),
      //           // 'rowData': JSON.stringify(contractContend),
      //         })
      //       }
      //       // console.log('1111-contractContend',contractContend)

      //       if (contractContend.firstPartyId && contractContend.secondPartyId) { // 如果这两个字段（甲乙方）有值，代表选择的是相关方
      //         contractContend.mainBodyRadio = '相关方'
      //         contractContend.mainBodyRelate1 = contractContend.firstPartyId // 甲方
      //         contractContend.mainBodyRelate2 = contractContend.secondPartyId // 乙方
      //       } else {
      //         contractContend.mainBodyRadio = '手动填写'
      //         // console.log('contractContend.contractBody',contractContend.contractBody)
      //         let newContractBodyArr = []
      //         let contractBodyArr = [];
      //         // let contractBodyArr = contractContend.contractBody.split(',');
      //         if (contractContend.contractBodys){ // 有contractBodys字段就用这个；没有就用原来的contractBody
      //           contractBodyArr = contractContend.contractBodys.split(',');
      //         } else {
      //           contractBodyArr = contractContend.contractBody.split(',');
      //         }
      //         console.log('contractBodyArr',contractBodyArr)
      //         contractBodyArr.forEach(x=>{
      //           if(x.length>0) {
      //             let arr = x.split(':');
      //             console.log('arr',arr)
      //             newContractBodyArr.push({
      //               "select": arr[0],
      //               "input": arr[1]
      //             })
      //           }
      //         })
      //         console.log('newContractBodyArr',newContractBodyArr)
      //         contractContend.mainBodySubform = newContractBodyArr;
      //       }
      //       // 项目回显
      //       contractContend.project = JSON.stringify({
      //         id:contractContend.projectId,
      //         name:contractContend.projectName,
      //       })
      //       // 经办人、经办公司、经办部门回显
      //       contractContend.contractDepName = JSON.stringify({
      //         id:contractContend.contractSubtableVo.handlingDepartment,
      //         name:contractContend.contractDepName,
      //       })
      //       contractContend.contractCompanyName = JSON.stringify({
      //         id:contractContend.contractSubtableVo.handlingCompany,
      //         name:contractContend.contractCompanyName,
      //       })
      //       contractContend.contractUserName = JSON.stringify({
      //         id:contractContend.handledBy,
      //         name:contractContend.contractUserName,
      //       })
      //       contractContend.paymentType = contractContend.contractSubtableVo.paymentType
      //       if (contractContend.contractSubtableVo.businessTime) {
      //         contractContend.businessTime = contractContend.contractSubtableVo.businessTime.split(' ')[0]
      //       }
            

      //       // 数据回显-合同盖章评审表发起才需要，合规性评审不需要，1、根据合同id获取所有临时业务id
      //       let fileRes = await this.getFileByContractId(contractContend.id)
      //       // console.log('fileRes--11',fileRes)
      //       this.contractRefFileList = fileRes.contractRefFileTypeApiVo?.contractRefFileTypeVos || [];
      //       console.log('this.contractRefFileList-1',this.contractRefFileList)
      //       // return;
      //       if (this.contractRefFileList.length>0) {
      //         this.contractRefFileList.forEach(async x=>{
      //           // 2.根据业务id获取文件
      //           await this.getFileByBizId(x.bizId).then(z=>{
      //             let arr = []
      //             z.data.forEach(k=>{
      //               arr.push({
      //                 // name: k.fileName,
      //                 name: k.originFileName,
      //                 status: "success",
      //                 url: k.fileUrl,
      //                 data: k,
      //                 c_time: 1718266907955,
      //                 formType: x.fileType,
      //                 key: 1718266907955+'_'+k.originFileName,
      //                 // key: 1718266907955+'_'+k.fileName,
      //                 size:k.fileSize
      //               })
      //             })
      //             contractContend[x.fileType] = arr;
      //             generateFormRefs.setData(contractContend);
      //             console.log('setData4')
      //             console.log('contractContend',contractContend)
      //           })
      //         })
      //       } else{
      //         generateFormRefs.setData(contractContend);
      //         console.log('setData5')
      //       }
            
      //     } else {
      //       this.$message.error(res.message);
      //     }
      //   }
      // );
    },
    // 获取合同主体字典列表
    getContractBodyList(){
      this.$axios.post(Api.admin.findByDictCode,{
        data:{
          dictCode:'contract_mainBody'
        },
        pagination:false,
      },res=>{
        if(res.isSuccess){
          this.contractBodyList = res.data.dataList;
        }
      })
    },
    // ====================业务开始========================
    // 合同合规性评审业务逻辑：表单整体分为合同信息+手续文件+合同文件+合规审查日志
    async contractBusiness(generateFormRefs,that,flag,type){
        console.log('contractBusiness')
        this.batchCode = that.batchCode;

        // let validateType = type == 'pass' ? true : false;
        // let isNeedValidate = that.clickMethod == 'draft' ? false : true  //是否进行表单验证，如果为true则进行表单验证，如果是false，不验证，草稿时候不验证
        console.log('qwe',type,that.clickMethod)
        let isNeedValidate = false;
        if (type == 'no_pass' || that.clickMethod == 'draft') {
          isNeedValidate = false
        } else {
          isNeedValidate = true
        }
        console.log('======isNeedValidate======',isNeedValidate)

        generateFormRefs.getData(isNeedValidate).then(async x => {
          let value = generateFormRefs.getValues();
          this.deleteLegalProcedureFile(value,generateFormRefs);
          console.log('value----111',value)
          // that在flowDialog文件传入
          
          // 隐藏合规评审，暂时注释
          // let contractId = !this.isFlowInitiate ? this.businessId : that.businessId; // 发起流程的合同id在选择盖章评审流程时的合同列表选择后传进来；审批或查看从外部传进来的businessId
          let saveContractInfoResData = await this.saveOrModifyContractInfo(value,that,type);
          // console.log('saveContractInfoResData',saveContractInfoResData)
          let contractId = this.formPage ? this.businessId : (this.isFlowInitiate ? saveContractInfoResData.id : this.businessId);// 隐藏合规评审，暂时注释
          console.log('contractId', contractId)
          that.businessId = contractId;// 隐藏合规评审，暂时注释

          this.legalProcedureBind(value,contractId,generateFormRefs); // 合同合规手续文件绑定
          // return;

          setTimeout(x=>{
            if (!this.isFlowInitiate) { // 不是发起流程:审批或重新发起
              // this.handleRePostSubmit(flag);
              if (that.isReInitiate) { // 重新发起
                console.log('重新发起')
                // that.handleRePostSubmit(flag)
                console.log('that.isDraftInReInitiateStatus',that.isDraftInReInitiateStatus)
                if (that.isDraftInReInitiateStatus) {
                  that.handleSaveDraft();
                } else {
                  that.handleRePostSubmit(flag,type)
                }
              } else { // 审批
                that.handleSubmitCheck(flag,type)
              }
            } else { // 发起流程
              that.enterpriseHandleSubmit(flag)
            }
          },200)
        }).catch(err=>{
          that.submitLoading = false
          that.noSubmitLoading = false
        })
    },
    // 业务-保存和修改合同信息
    saveOrModifyContractInfo(formVal,that,type){
      return new Promise((resolve, reject) => {
        console.log('saveOrModifyContractInfo')
        console.log('this.businessId',this.businessId)
        console.log('that.businessId',that.businessId)
        console.log('!this.isFlowInitiate',!this.isFlowInitiate)
        console.log('this.currentContractInfo',this.currentContractInfo)
        console.log('====formVal===',formVal)
        let contractId = !this.isFlowInitiate ? this.businessId : that.businessId;
        // let contractSubmitType = that.clickMethod // 隐藏合规评审，暂时注释
        let contractSubmitType = !this.isFlowInitiate ? type : that.clickMethod
        console.log('contractSubmitType',contractSubmitType)
        console.log('JSON.parse(formVal.contractUserName)',formVal.contractUserName)
        // return;
        let url = this.formPage ? Api.contractManage.contractInfo.modifyContract : (this.isFlowInitiate ? Api.contractManage.contractInfo.saveContractInfo : Api.contractManage.contractInfo.modifyContract)
         // 隐藏合规评审，暂时注释
        // let url = Api.contractManage.contractInfo.modifyContract; // 合同盖章评审表的合同信息已有，所以不管发起还是审批流程都是用的修改接口
        let param = {
          // "id": contractId, // 不管发起还是审批都有合同id 隐藏合规评审，暂时注释
          // "projectId":formVal.projectId,
          "projectId":formVal.project ? JSON.parse(formVal.project).id : '',
          "contractProjectName":formVal.project ? JSON.parse(formVal.project).name : '',
          "classificationId":formVal.classificationId instanceof Array ? (!!formVal.classificationId.length ? formVal.classificationId[formVal.classificationId.length-1] : '') : formVal.classificationId, // 是数组就取最后一个，不是就是取字符串
          // "classificationId":!!formVal.classificationId.length ? formVal.classificationId[formVal.classificationId.length-1] : '',
          // "contractGrade":formVal.contractGrade, // 合同等级不要了
          "contractName":formVal.contractName,
          "contractNumber":formVal.contractNumber,
          // "contractBody":"合同主体",//（合同主体需要前端逻辑整理）
          "contractContent":formVal.contractContent,
          "contractTalk":formVal.contractTalk,
          "contractSum":formVal.contractSum,
          // "contractUserName":formVal.contractUserName,
          // "contractDepName":formVal.contractDepName,
          // "contractCompanyName":formVal.contractCompanyName,
          "contractUserName": formVal.contractUserName ? JSON.parse(formVal.contractUserName).name : '',
          "contractDepName": formVal.contractDepName ? JSON.parse(formVal.contractDepName).name : '',
          "contractCompanyName": formVal.contractCompanyName ? JSON.parse(formVal.contractCompanyName).name : '',
          // "handledBy":formVal.handledBy,
          "handledBy": formVal.contractUserName ? JSON.parse(formVal.contractUserName).id : '',
          "payMethod": formVal.payMethod, //收付款方式
          "contractSubtableVo":{
            "businessTime":formVal.businessTime? formVal.businessTime+' 00:00:00' : '',
            "paymentType":formVal.paymentType,  //收付款分类  0收款1付款2其它 3收付款合同
            // "handlingDepartment":formVal.handlingDepartment,
            // "handlingCompany":formVal.handlingCompany,
            "handlingDepartment": formVal.contractDepName ? JSON.parse(formVal.contractDepName).id : '',
            "handlingCompany": formVal.contractCompanyName ? JSON.parse(formVal.contractCompanyName).id : '',
            "stampStatus": contractSubmitType == 'draft' ? "0":"1", // 状态:0草稿1提交（合同盖章状态）
            "stampExamineStatus":"0", // 审核状态:0审核中1审核成功2审核失败（合同盖章状态）
            // "contractId":"合同id",（合规性-没有就不需要）
            // "entrustId":"关联委托id",（合规性-没有就不需要）
            // "templateId":"模板id",（合规性-没有就不需要）
          },
          "status": this.currentContractInfo?.status || "1",  // 状态:0草稿1提交（合规审查状态） ；发起盖章了，合规应该走完了，应该就是写死1状态
          "examineStatus":this.currentContractInfo?.examineStatus || "0",  // 审核状态:0审核中1审核成功2审核失败（合规审查状态）
          "fileStatus": formVal.sealContractFile && formVal.sealContractFile.length > 0 ? "1" : "0", // 是否有合同盖章文件
          "companyId": this.$store.state.user.companyId, // 隐藏合规评审，暂时注释
          "depId": formVal.initiatorDepartmentId || ''
        }
        // console.log('param',param)

        // 隐藏合规评审，暂时注释
        if (this.formPage || !this.isFlowInitiate) {
          param.id = contractId;
          delete param.companyId
        }
        // if (!this.isFlowInitiate) {
        //   param.id = contractId;
        // }
        
        let str = '',str2='';
        let contractBodyListCopy = JSON.parse(JSON.stringify(this.contractBodyList));
        if (formVal.mainBodyRadio == '手动填写'){
          formVal.mainBodySubform.forEach(x=>{
            // 处理合同主体数据转换（修复主体方名称为数字，导致合同管理模块页面后端查询不到的bug）
            let firstBody = getObjById(contractBodyListCopy,x.select,'dictDataVos','dictValue')
            if (firstBody) {
              str+=` ${firstBody.dictLabel}:${x.input},`
              // 另外存一份未转换的数据用于盖章评审表单的回显，带出数据
              str2+=`${x.select}:${x.input}`+','
            }
          })
          str=str.substring(0,str.length-1);
          str2=str2.substring(0,str2.length-1);
          
          // formVal.mainBodySubform.forEach(x=>{
          //   str+=`${x.select}:${x.input}`+','
          // })
          // str=str.substring(0,str.length-1);
          // 如果选相关方，需要将下面两个字段置空
          param.firstPartyId = '' // 甲方
          param.secondPartyId = '' // 乙方
        } else if (formVal.mainBodyRadio == '相关方'){ // 相关方除了需要传contractBody中文的参数，还需要传甲方和乙方的id给后端
          let mainBodyRelateName1 = this.relatedPartyList.find(x=>x.id == formVal.mainBodyRelate1).name;
          let mainBodyRelateName2 = this.relatedPartyList.find(x=>x.id == formVal.mainBodyRelate2).name;
          str = `甲方:${mainBodyRelateName1},乙方:${mainBodyRelateName2}`
          param.firstPartyId = formVal.mainBodyRelate1 // 甲方
          param.secondPartyId = formVal.mainBodyRelate2 // 乙方
        }
        param.contractBody = str;
        param.contractBodys = str2; // contractBodys存的主体数据用于盖章评审表单的发起流程数据回显

        console.log('saveOrModifyContractInfo--param',param)
        let finalParam = {
          data: param
        }
        if (this.isFlowInitiate) { // 发起流程添加batchCode，草稿重新发起就不需要
          finalParam.batchCode = that.batchCode;
        }
        // return;
        this.$axios.post(
          url,
          finalParam,
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        );
      });
    },
    // 删除文件与业务关联关系接口
    deleteFileRelation(relationId,fileIds) {
      this.$axios.post(
        Api.user.deleteFileRelation,
        {
          data: {
            relationId:relationId,
            fileIds: fileIds
          }
        }
      );
    },
    // 删除选择的合同谈判方式以外的其他手续文件（25.2.25前好像改了合同手续关联的接口，好像是不需要调用删除，每次以最后一次保存的数据为准）
    deleteLegalProcedureFile(formVal,generateFormRefs){
      // 选中的合同谈判方式
      let contractTalkName = formVal.contractTalk
      // 合同谈判方式对应的文件表格字段或者文件上传字段类型名称
      let objList = {
        '公开招标':'GKzhaobiao',
        '非公开挂网':'FGKguawang',
        '非挂网方式':'FGWfangshi',
        '直签':'zhiqian',
        '单一来源采购':'DYLYcaigou',
        '内部采购':'NBcaigou',
        '框采':'kuangcai',
        '其他':'QTwenjian',
      }
      let objArr = [];
      let clearFileObj = {};
      let copyFormVal = JSON.parse(JSON.stringify(formVal))
      let needUnbindFileObj = {};
      for (var i in objList) {
        if (i !== contractTalkName) {
          objArr.push(objList[i])
        }
      }
      console.log('objArr',objArr)
      objArr.forEach(x=>{
        for (var i in formVal) {
          if (i.indexOf(x) > -1 && formVal[i].length>0) {
            // console.log('formVal[i]',formVal[i])
            needUnbindFileObj[i] = JSON.parse(JSON.stringify(copyFormVal[i]))
            clearFileObj[i] = [];
            formVal[i] = [];
          }
        }
      })
      console.log('needUnbindFileObj',needUnbindFileObj)
      console.log('this.contractRefFileList',this.contractRefFileList)
      // 清空表单内其他不需要的文件数据
      generateFormRefs.setData(clearFileObj);
      console.log('setData6')
      if(Object.keys(needUnbindFileObj).length && this.contractRefFileList.length) {
        // 如果已有文件业务id的文件，需要调用接口解除绑定
        this.contractRefFileList.forEach(item=>{
          if (item.fileType !== '合同文件' && needUnbindFileObj[item.fileType]) {
            let needUnbindFileList = needUnbindFileObj[item.fileType].map(x=>x.data?.fileId || x.data.id)
            console.log('needUnbindFileList',needUnbindFileList)
            this.deleteFileRelation(item.bizId,needUnbindFileList); // 解除文件业务id和文件id关系后，后续有需要可以加多一个解绑业务id和文件业务id的关系
          }
        })
      }
    },
    // 业务-合同合规手续文件绑定合同id
    async legalProcedureBind(formVal,contractId,generateFormRefs){
      console.log('legalProcedureBind')
      console.log('formVal111222',formVal)
      // return;
      // 选中的合同谈判方式
      let contractTalkName = formVal.contractTalk
      // 合同谈判方式对应的文件表格字段或者文件上传字段类型名称
      let objList = {
        '公开招标':'GKzhaobiao',
        '非公开挂网':'FGKguawang',
        '非挂网方式':'FGWfangshi',
        '直签':'zhiqian',
        '单一来源采购':'DYLYcaigou',
        '内部采购':'NBcaigou',
        '框采':'kuangcai',
        '其他':'QTwenjian',
      }

      // 切换方式时，对已绑定流程的进行处理(暂时注释-这里注释了，盖章才能正常绑定盖章合同)
      console.log('切换谈判方式',contractTalkName)
      // if (contractTalkName == '单一来源采购') {
      //   generateFormRefs.setData({DYLYcaigou_file1_flow: JSON.stringify({
      //     'flowType': 'single_source_delegation',
      //   })})
      // } else if (contractTalkName == '内部采购') {
      //   generateFormRefs.setData({NBcaigou_file1_flow: JSON.stringify({
      //     'flowType': 'internal_delegation',
      //   })})
      // } else if (contractTalkName == '非挂网方式') {
      //   generateFormRefs.setData({FGWfangshi_file3_flow: JSON.stringify({
      //     'flowType': 'proposed_supplier_review',
      //   })})
      // }
      
      let needBindFileList = {}; // 需要绑定的文件列表(遍历合规手续文件的字段表单里面有数据的存放进来，待上传)
      let contractRefFileTypeVos = []; // 合同id需要绑定的已绑定文件列表的临时bizId
      for (var i in formVal) {
        if (i.indexOf(objList[contractTalkName]) > -1 && formVal[i].length>0) {
          needBindFileList[i] = formVal[i]
        }
      }
      console.log('needBindFileList',needBindFileList)
      for (var j in needBindFileList) {
        let fileIdList = [],temporaryBizId = '';
        let ifHasTemIdObj = this.contractRefFileList.find(x=>x.fileType == j)
        // console.log('===ifHasTemIdObj===',ifHasTemIdObj)
        if (ifHasTemIdObj) { // 上传文件前需要判断这个类型是否已经绑定过了临时的业务id，如果已有就不要再次创建临时ID绑定（业务用原来的临时id）
          temporaryBizId = ifHasTemIdObj.bizId;
        } else { // 没有临时Id的所有文件遍历绑定
          temporaryBizId = generateRandomId(32)
        }

        if (Array.isArray(needBindFileList[j])) {
          needBindFileList[j].forEach(item=>{
            fileIdList.push(item.data?.fileId || item.data.id) // 数据不用在发起时回显的时候文件id取的id，不是fileId
            // fileIdList.push(item.data.id)
          })

          this.saveBatchFile(temporaryBizId,fileIdList) // 文件批量绑定
          // 1、合规手续绑定临时id
          contractRefFileTypeVos.push({
            bizId:temporaryBizId,
            fileType:j,
          })
        }
      }
      // 2、上传盖章合同文件（PDF）绑定临时id
      console.log('formVal11',formVal)
      console.log('this.contractRefFileList',this.contractRefFileList)
      // return;
      let sealFileTemporaryId = '';
      if (formVal['sealContractFile'].length>0) {
        let sealPdf_ifHasTemIdObj = this.contractRefFileList.find(x=>x.fileType == 'sealContractFile')
        console.log('sealPdf_ifHasTemIdObj',sealPdf_ifHasTemIdObj)
        let sealPdf_fileIdList = [];
        if (sealPdf_ifHasTemIdObj) { // 上传文件前需要判断这个类型是否已经绑定过了临时的业务id，如果已有就不要再次创建临时ID绑定（业务用原来的临时id）
          sealFileTemporaryId = sealPdf_ifHasTemIdObj.bizId;
        } else { // 没有临时Id的所有文件遍历绑定
          sealFileTemporaryId = generateRandomId(32)
        }
        formVal['sealContractFile'].forEach(item=>{
          sealPdf_fileIdList.push(item.data?.fileId || item.data.id) // 数据不用在发起时回显的时候文件id取的id，不是fileId
          // sealPdf_fileIdList.push(item.data.id)
        })
        console.log('sealFileTemporaryId',sealFileTemporaryId)
        this.saveBatchFile(sealFileTemporaryId,sealPdf_fileIdList)
        contractRefFileTypeVos.push({
          bizId:sealFileTemporaryId,
          fileType:'sealContractFile',
        })
      }
      // 隐藏合规评审，暂时注释
      // 2025.2.14后端将关联接口改成全量保存，所以这里要再获取一下合同文件参数，一起保存。
      // let oldContractFile = this.contractRefFileList.find(x=>x.fileType == '合同文件')
      // if (oldContractFile) {
      //   contractRefFileTypeVos.push(oldContractFile)
      // }

      // 隐藏合规评审，暂时注释
      let contractTemporaryId = '';
      let contract_ifHasTemIdObj = this.contractRefFileList.find(x=>x.fileType == '合同文件')
      console.log('contract_ifHasTemIdObj',contract_ifHasTemIdObj)
      if (contract_ifHasTemIdObj) { // 上传文件前需要判断这个类型是否已经绑定过了临时的业务id，如果已有就不要再次创建临时ID绑定（业务用原来的临时id）
        contractTemporaryId = contract_ifHasTemIdObj.bizId;
      } else { // 没有临时Id的所有文件遍历绑定
        // contractTemporaryId = this.fileRelationId
        contractTemporaryId = generateRandomId(32)
      }
      contractRefFileTypeVos.push({
        bizId:contractTemporaryId,
        fileType:'合同文件',
      })
      
      // return;
      console.log('contractRefFileTypeVos',contractRefFileTypeVos) 
      // 3、文件绑定完临时id后，还需要和合同Id绑定
      await this.saveContractRefFile(contractId,contractRefFileTypeVos)
      // 4、删除其他不需要的文件关联关系
      // if(needUnbindFileList.length) {
      //   this.deleteFileRelation(sealFileTemporaryId,needUnbindFileList);
      // }
    },
    // 保存合同关联的手续文件
    saveContractRefFile(contractId,contractRefFileTypeVos){
      let params = {
        contractRefFileTypeApiVo:{
          contractId:contractId,
          contractRefFileTypeVos:contractRefFileTypeVos,
        }
      }
      if (this.isFlowInitiate) { // 发起流程添加batchCode，草稿重新发起就不需要
        params.batchCode = this.batchCode;
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.contractManage.contractInfo.saveContractRefFile,
          {
            data: params
          },
          res => {
            if (res.isSuccess) {
              resolve();
              // this.$message.success('关联附件成功');
            } else {
              // this.$message.error(res.message);
            }
          }
        );
      });  
    },
    // 文件批量绑定
    saveBatchFile(bizId,fileIds){
      this.$axios.post(
        Api.user.saveBatchFile,
        {
          data: {
            relationId: bizId,
            fileIds: fileIds
          }
        },
        res => {
          if (res.isSuccess) {
            // this.$message.success('关联附件成功');
          } else {
            // this.$message.error(res.message);
          }
        }
      );
    },
    //根据业务id获取文件
    getFileByBizId(id) {
      return this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      })
    },
    //根据合同id查文件类相关
    getFileByContractId(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.contractManage.contractInfo.getFileByContractId,
          {
            data: {
              id: id
            }
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        );
      })
    },
    // getFileByContractId(id) {
    //   return this.$axios.post(
    //     Api.contractManage.contractInfo.getFileByContractId, {
    //     data: {
    //       id: id
    //     }
    //   })
    // },
    // ====================业务结束========================
    
  },
};
</script>
<style lang="scss" scoped>
  // ::v-deep {
  // }
</style>