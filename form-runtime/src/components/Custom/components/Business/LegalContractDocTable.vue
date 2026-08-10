<!--
 * @Descripttion: 合同合规性评审-合同文件
 * @Author: zhengzetao
 * @Date: 2024-05-20 15:35:38
-->
<!-- 流程审批的状态区别，后续可能会有改动。只做参考
审核：
isReInitiate-legsl false
businessId-legal 7a6023df037a41e8baa76b6043faf15f
isExamine-legal true
isFlowInitiate-legsl fslse
重发：
isReInitiate-legal true
businessId-legal 7a6023df037a41e8baa7666043faf15f
isExamine-legal false
isFlowInitiate-legsl false
草稿：
isReInitiate-legsl true
businessId-legal 7a6023df037a41e8baa7666043faf15f
isExamine-legsl false
isFlowInitiate-legal fslse
详情：
isReInitiate-legal fslse
businessId-legal 7a6023df037a41e8baa76b6043faf15f
isExamine-legal false
isFlowInitiate-legal false
发起：
isReInitiate--legal false
businessId--legal
isExamine--legal true
isFlowInitiate--legal true -->
<template>
  <div>
    <!-- <div v-if="contractFileData.length>0"> -->
    <div v-if="isFlowInitiate && !contractFileData.length">
      <!-- 上传合同文件暂时放开限制2024.12.19 -->
      <!-- accept=".doc,.docx" -->
      <div v-if="formPage && !contractFileData.length" style="text-align: right;">
        文件正在获取中，请稍等...
      </div>
      <el-upload v-else ref="upload" :action="pdfAction" :data="fileData"
        :before-upload="beforeAvatarUpload" :on-success="handleAvatarSuccess" :on-remove="handleRemove"
        :before-remove="beforeRemove" :file-list="fileList" :limit="1" style="float:right;margin-right:10px;">
        <el-button size="small" type="primary" icon="el-icon-upload" class="upload-template">上传合同文件</el-button>
      </el-upload>
    </div>
    <div v-else style="width: 100%;margin:0 auto;margin-top: 1px;">
      <!-- 合同最新文件：显示一个 -->
      <el-table
        :data="contractFileData"
        border
        class="legalTableClass"
        style="width: 100%;margin:0 auto;border:1px solid rgb(153, 153, 153);border-top: none;border-right: none;">
        <el-table-column
          prop="contractName"
          label="合同文件">
        </el-table-column>
        <el-table-column
          prop="updateDate"
          label="更新时间">
        </el-table-column>
        <el-table-column
          fixed="right"
          label="操作"
          width="160">
          <template slot-scope="scope">
            <!-- <el-button type="text" size="small" :disabled="!isExamine && !isReInitiate" @click="test(scope.row)">test</el-button> -->
            <el-button type="text" size="small" :disabled="isTranspondFlow || editOnlineLoading || !isExamine && !isReInitiate" @click="newEdit(scope.row)">{{editOnlineLoading ? '加载中...' : '在线编辑'}}</el-button>
            <el-button type="text" size="small" @click="beforeGetFile('downLoad',scope.row)" :disabled="viewLoading || !scope.row.fileUrl">下载</el-button>
            <el-button type="text" size="small" :disabled="viewLoading || !scope.row.fileUrl" @click="beforeGetFile('view',scope.row)">查看</el-button>
            <!-- <el-button type="text" size="small" :disabled="!scope.row.fileUrl" @click="viewFile(scope.row.fileUrl)">查看</el-button> -->
          </template>
        </el-table-column>
      </el-table>

      <!-- 合同历史版本表格 -->
      <div v-if="contractHistoryData.length">
        <!-- border-bottom: 2px solid rgb(153, 153, 153) -->
        <div style="color:#1989FA;border:1px solid rgb(153, 153, 153);border-top: none;height: 54px;padding: 14px 20px;text-align: left;">合同历史同版本记录</div>
        <el-table
          :data="contractHistoryData"
          border
          class="legalTableClass"
          style="width: 100%;margin:0 auto;border:1px solid rgb(153, 153, 153);border-top: none;border-right: none;">
          <el-table-column
            prop="contractName"
            label="合同名称">
          </el-table-column>
          <el-table-column
            prop="createName"
            label="更新人">
          </el-table-column>
          <el-table-column
            prop="updateDate"
            label="更新时间">
          </el-table-column>
          <el-table-column
            fixed="right"
            label="操作"
            width="120">
            <template slot-scope="scope">
              <el-button type="text" size="small" :disabled="!scope.row.fileUrl" @click="downLoadFile(scope.row)">下载</el-button>
              <el-button type="text" size="small" :disabled="!scope.row.fileUrl" @click="viewFile(scope.row.fileUrl)">查看</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- 在线编辑合同文件 -->
    <el-dialog :visible="excelVisible" v-if="excelVisible" :close-on-click-modal="false" :append-to-body="true"
      @close="closeDialog" :show-close="false" :fullscreen="true" custom-class="excel-div">
      <!-- style="display: flex;flex-direction: row-reverse;padding: 5px 75px 5px 0px;" -->
      <div>
        <el-form
        ref="form"
        :inline="true"
        label-width="120px"
        class="template-form"
      >
        <el-row :gutter="20">
          <el-col :span="18">
            <el-form-item label="文件名称：" prop="newFileName" style="width:100%;">
              <el-input v-model="newFileName" style="width:220px;" placeholder="请输入文件名称">
                <template slot="append">{{ fileSuffix }}</template>
              </el-input>
              <!-- <el-input v-model.trim="newFileName" style="width:220px"/> -->
              <!-- <el-button v-if="pageTemplateId" style="margin-left:15px;" type="primary" @click="comparisonFile">与模板合同对比</el-button> -->
              <el-button v-if="templateId || pageTemplateId" style="margin-left:15px;" type="primary" @click="comparisonFile">与模板合同对比</el-button>
              <el-button type="primary" style="margin: 0 15px;" @click="saveExcel">保存</el-button>
            </el-form-item>
          </el-col>
          <!-- <el-col :span="10"> -->
            <!-- isExamine &&  -->
            <!-- <el-button v-if="templateId && businessId" type="primary" @click="comparisonFile">与模板合同对比</el-button> -->
          <!-- </el-col> -->
        </el-row>
      </el-form>
        <!-- <el-button type="primary" @click="exportExcel" plain>导出Excel</el-button> -->
      </div>
      
      <div class="word-online">
        <!-- 在线编辑 -->
        <officeExcel :iframeOrigin="iframeOrigin" :fileUrl="fileUrl" :callbackUrl="callbackUrl" :mode="mode" :documentType="documentType"
          :title="title" :token="token" :user="user" :excelKey="excelKey" ref="officeExcelRef">
        </officeExcel>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import { viewFileUrl } from '@/config/env.js';
import { localstorageGet } from '@/utils/auth';
import officeExcel from '@/components/OfficeExcel'
import { generateRandomId,generateUid,viewFile,getObjById,deepClone,formatNewTime } from "@/utils";
import moment from 'moment';

import Api from '@/api';
import { baseUrl,onlyOfficeUrl } from '@/config/env';
import axios from 'axios'; //单独引入axios，用于excel的保存，避免返回格式不同方法报错

/* eslint-disable */
export default {
  name: 'LegalContractDocTable',
  components: {officeExcel},
  props: {
    // disabled: { // value
    //   type: [Boolean], // [Array, String, Number]
    //   default() {
    //     return false;
    //   }
    // },
    isFlowInitiate: { // 是否是流程发起阶段
      type: Boolean,
      default: false
    },
    businessId: { // 业务id
      type: String,
      default: ''
    },
    companyId: { // 公司id
      type: String,
      default: ''
    },
    pageTemplateId: { // 模板id（我的合同带过来）
      type: String,
      default: ''
    },
    isExamine: { // 是否审核状态
      type: Boolean,
      default: true
    },
    isReInitiate: { // 是否草稿重新发起状态
      type: Boolean,
      default: false
    },
    isTranspondFlow: { // 是否是转发数据
      type: Boolean,
      default: false
    }
    // contractSubmitType: { // 合同提交状态
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      contractRefFileList:[],
      relatedPartyList:[],
      contractHistoryData:[],
      contractFileData:[],
      fileList: [],
      // files: [],
      fileData: {
        fileType: 'ordinaryFile'
      },
      excelVisible: false,
      documentType:'word',
      fileUrl: '',
      callbackUrl: '',
      iframeOrigin: `${onlyOfficeUrl}/web-apps/apps/`,
      token: '',
      user: {},
      bindRandomId: generateRandomId(32), // 不管第一次上传还是每次点击在线编辑，进来页面只会有一个统一临时id
      // bindRandomId: generateUid(), // 不管第一次上传还是每次点击在线编辑，进来页面只会有一个统一临时id
      excelKey: '',
      mode:'edit', // view
      newFileName:'',
      fileSuffix:'',
      fileId:'',
      currentDealContractId:'',
      currentDealFileBizId:'',
      // legalContractSaveFlag:false, // true为第一次上传文件和在线编辑文件有改动，需要进行文件修改记录保存
      fromOutsidePage:false, // 从其他页面进入流程审批
      thatInstance:null,
      contractBodyList:[],
      templateId:'',
      templateUrl:'',
      fileRelationId:'',
      fileVersion:'',
      formPage:'',
      hasClickSave: false,
      batchCode:'',
      viewLoading:false,
      editOnlineLoading:false,
      uploadBindRandomId:'',
      originContractTalk:''
    };
  },
  watch:{
  },
  computed: {
    pdfAction() {
      const sid = this.$store.state.user.token;
      return `${baseUrl}/web/file/api/file/uploadFile?sid=${sid}&platformCode=200001`;
    },
    title() {
      let title = this.mode == 'edit' ? '编辑模式' : '只读模式'
      return title
    },
  },
  created() {
    this.getContractBodyList();
  },
  async mounted() {
    
    // console.log('legalMounted')
    console.log('isReInitiate--legal',this.isReInitiate)
    console.log('businessId--legal',this.businessId)
    console.log('isExamine--legal',this.isExamine)
    console.log('isFlowInitiate--legal',this.isFlowInitiate)
    console.log('fromOutsidePage--legal',this.fromOutsidePage)
    // console.log('pageTemplateId',this.pageTemplateId)
    console.log('this.formPage--legal',this.formPage)
    console.log('this.isTranspondFlow--legal',this.isTranspondFlow)

    // if (this.businessId) { // 有合同id
    //   this.getContractDetail();
    // //   let result = await this.getContractDetail();
    // //   this.templateId = result?.contractSubtableVo?.templateId || '';
    // //   console.log('result',result?.contractSubtableVo?.templateId)
    // }

    this.getRelatedPartyByUser();
    if (!this.isFlowInitiate) { // 不是发起流程
      this.getLegalContractLogList();
      this.getHadUploadFileBizId();
    } else {
      console.log('发起流程')
      // 25.3.6注释
      // if (this.pageTemplateId) {
      //   let fileRes = await this.getFileByBizId(this.pageTemplateId);
      //   console.log('fileRes',fileRes)
      //   this.templateUrl = fileRes?.data[0]['fileUrl'];
      // }
    }
  },
  methods: {
    comparisonFile() { // 对比文档方法
      var docEditor = this.$refs.officeExcelRef.docEditor;
      let needSlice = this.templateUrl.lastIndexOf('.')
      let fileSuffix = this.templateUrl.slice(needSlice)
      console.log('fileSuffix',fileSuffix)
      console.log('this.token,',this.token,)
      console.log('this.templateUrl',this.templateUrl)
      // docEditor.setRevisedFile({
      docEditor.setRequestedDocument({
        c: "compare",
        fileType: fileSuffix,
        // fileType: 'docx',
        token: this.token,
        url: this.templateUrl // 对比文档链接
        // url: 'http://192.168.1.220/files/54e991e06b9b4f55b1747ee71fd0d79a/200001/2025-01-10/3458d16910db4fe38d7bbb94b32535de.docx' // 对比文档链接
      });
    },
    // 获取合同详情
    // getContractDetail(){
    //   // return new Promise((resolve, reject) => {
    //     this.$axios.post(
    //       Api.contractManage.contractInfo.getContractDetail,
    //       {
    //         data: {
    //           id: this.businessId
    //         }
    //       },
    //       async res => {
    //         if (res.isSuccess) {
    //           this.templateId = res.data?.contractSubtableVo?.templateId || '';
    //           console.log('有模板',this.templateId)
    //           if (this.templateId) {
    //             let fileRes = await this.getFileByBizId(this.templateId);
    //             console.log('fileRes',fileRes)
    //           }
    //           // resolve();
    //         } else {
    //           // this.$message.error(res.message);
    //         }
    //       }
    //     );
    //   // });
    // },
    initFormInstance(generateFormRefs){
      this.thatInstance = generateFormRefs;
      // console.log(111,generateFormRefs)
      // console.log('this.thatInstance',this.thatInstance)
    },
    // 发起流程时表单回显数据
    initContract(generateFormRefs,params){
      console.log('initContract');
      console.log('合规性评审表单带来params',params)
      if( params?.contractId) this.fromOutsidePage = true; // 从外部页面进来，并且已有合同id
      // this.firstDealContractFileList(params.fileId,params.contractName,params.fileUrl,'')
      this.firstDealContractFileList(params.fileId,params.fileName,params.fileUrl,'',params.fileRelationId,params.contractFileVersion,params.formPage,params.contractId)
      generateFormRefs.setData({'contractName':params.contractName,'contractNumber':params.contractNumber});
    },
    // 初次上传合同文件的统一处理
    firstDealContractFileList(fileId,contractName,fileUrl,updateDate,fileRelationId,contractFileVersion,formPage,contractId){
      console.log('firstDealContractFileList-contractName',contractName)
      this.newFileName = contractName;
      console.log('this.newFileName',this.newFileName)
      this.fileId = fileId;
      // this.legalContractSaveFlag = true;
      this.fileRelationId = fileRelationId;
      this.newFileName = contractName;
      this.formPage = formPage;
      this.contractId = contractId;
      console.log('this.formPage',this.formPage)
      // if (this.formPage && this.$parent.$parent.$parent.$parent.$parent.$parent.$parent.title == '发起流程') { // 表单内点了编辑就不要回到上个页面，因为会导致在线编辑文档空白
      //   this.$parent.$parent.$parent.$parent.$parent.$parent.$parent.showClose = false;
      // }

      let that = this;
      // let copyRow = deepClone(row)

      if (this.formPage) {
        const pollData = async () => {
          // contractRefFileTypeVos这里面可以判断fileType等于"合同文件"，下面直接取第一个元素
          let newFileRes = await that.getFileByBizId(this.fileRelationId);
          console.log('newFileRes',newFileRes)
          
          let lastPointIndex = newFileRes.data.length ? newFileRes.data[0]['fileName'].lastIndexOf('.') : ''
          let fileName = newFileRes.data.length ? newFileRes.data[0]['fileName'].slice(0,lastPointIndex) : ''
          console.log('fileName',fileName)
          let lastVersionIndex = fileName.lastIndexOf('_')
          let fileVersion = fileName.slice(lastVersionIndex+1) // 文件上的版本号
          let listFileVersion = contractFileVersion.split('_')[1]; // 合同列表上的版本号
          console.log('fileVersion',fileVersion)
          console.log('listFileVersion',listFileVersion)
          console.log('大于', fileVersion > listFileVersion)
          
          // 判断合同列表存的版本号和文件名称带的版本号一样或者大于，就不用轮询；不一样代表onlyOffice保存过，可能有延时，需要轮询到版本号一样为止
          if (fileVersion == listFileVersion || fileVersion > listFileVersion) {
            // 数据已经有值，停止轮询
            clearInterval(intervalId);
            // 处理有值的数据
            console.log('数据准备就绪:', newFileRes.data[0]?.fileId);

            this.contractFileData = [{
              // updateDate: updateDate,
              updateDate: newFileRes.data[0]['updateDate'],
              contractName:contractName,
              fileUrl:newFileRes.data[0]['fileUrl'],
              fileId:newFileRes.data[0]['fileId'],
              fileRelationId:newFileRes.data[0]['relationId'],
              contractFileVersion:contractFileVersion,
              templateId: this.pageTemplateId,
            }]
            console.log('this.contractFileData',this.contractFileData)

          } else {
            // 数据还是空值，继续轮询
            console.log('数据还没准备好，继续等待...');
          }
        };
        // 设置轮询间隔，单位毫秒
        const pollInterval = 1000; // 0.5秒
        // 开始轮询
        const intervalId = setInterval(pollData, pollInterval);
      } else {
        this.contractFileData = [{
          updateDate: updateDate,
          contractName:contractName,
          // contractName:contractName,
          fileUrl:fileUrl,
          fileId:fileId,
          fileRelationId: fileRelationId,
          contractFileVersion: contractFileVersion,
          templateId: ''
        }]
        this.fileRelationId = fileRelationId;
      }
    },
    // 根据合同获取已上传过的文件集合，包括类型和业务id
    getHadUploadFileBizId(){
      this.getFileByContractId(this.businessId).then(y=>{
        this.contractRefFileList = y.data.contractRefFileTypeApiVo?.contractRefFileTypeVos || [];
        // console.log('获取临时存储的文件的id-this.contractRefFileList',this.contractRefFileList)
      })
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
    // 查询合同合规性审查日志记录列表
    async getLegalContractLogList(){
      console.log('getLegalContractLogList')
      console.log('this.businessId',this.businessId)
      this.$axios.post(
        Api.contractManage.contractInfo.getLegalContractLogList,
        {
          data: {
            contractId:this.businessId
          },
          "current": 1,
          "size": 10,
          "pagination": true
        },
        res => {
          if (res.isSuccess) {
            res.data.forEach(async x=>{  // 历史数据获取文件
              await this.getFileByBizId(x.id).then(y=>{
                console.log('x.id',x.id)
                console.log('yyy',y)
                // if (y.data.length >0) {
                  x.fileUrl = y.data[0].fileUrl;
                  x.fileId = y.data[0].fileId;
                  // this.$set(x,'fileUrl',y.data[0].fileUrl)
                // } else {
                //   x.fileUrl = null;
                //   x.fileId = null;
                // }
              })
            })
            setTimeout(x=>{
              this.contractHistoryData = res.data;
              console.log('this.contractHistoryData',this.contractHistoryData)

              this.contractFileData = [];
              if( this.contractHistoryData.length>0) {

                this.contractFileData.push({
                  updateDate:res.data[0]['updateDate'],
                  contractName:res.data[0]['contractName'],
                  fileUrl:res.data[0]['fileUrl'],
                  fileId:res.data[0]['fileId'],
                  id:res.data[0]['id'], // 文件业务id
                  fileRelationId:res.data[0]['id'], // 文件业务id
                  contractId:res.data[0]['contractId'], // 合同id
                  contractFileVersion:res.data[0]['fileVersion'], // 版本号
                  templateId:res.data[0]['templateId'] // 合同模板id
                })
                this.fileRelationId = res.data[0]['id'];
                this.fileVersion = res.data[0]['fileVersion'];
                this.templateId = res.data[0]['templateId'];
                console.log('this.contractFileData2',this.contractFileData)
                // 名称拿历史记录最后一个，数据拿历史记录第一条
                let str = this.contractHistoryData[this.contractHistoryData.length-1]['contractName']
                this.contractFileData[0].contractName = str
              }
              console.log('this.contractHistoryData111',this.contractHistoryData)
            },1000)
          } else {
          }
        }
      );
    },
    // 业务-合同合规性审查日志记录列表-保存接口
    legalContractSave(contractId){
      console.log('=====legalContractSave-历史保存=====fileVersion',this.fileVersion)
      console.log('=====legalContractSave-历史保存=====contractId',contractId)
      let param = {
        "contractName": this.newFileName,
        "contractId": contractId,// 合同id
      }

      // if (this.isFlowInitiate) {
        param.id = this.fileRelationId; // 文件业务id
        param.fileVersion = this.fileVersion; // 历史版本号
        param.templateId = this.templateId || this.pageTemplateId;
        // param.id = this.bindRandomId; // 文件业务id
      // }
      return new Promise((resolve, reject) => {
        console.log('=====legalContractSave=====')
        // if (this.legalContractSaveFlag) { //  第一次上传文件和在线编辑文件有改动，需要进行文件修改记录保存
          this.$axios.post(
            Api.contractManage.contractInfo.saveLegalContractLog,
            {
              data: param
            },
            res => {
              resolve(res.data)
              // if (res.isSuccess) {
              // } else {
              // }
            }
          );
        // } else {
        //   resolve()
        // }
      });
    },
    // 业务-合同合规性审查日志记录列表-更新接口
    legalContractUpdate(){
      console.log('legalContractUpdate-历史更新',this.currentDealFileBizId)
      let param = {
        "contractName": this.newFileName,
        "id": this.currentDealFileBizId,
        "fileVersion": this.fileVersion,
        // "contractId": contractId,// 合同id
      }
      // if (this.isFlowInitiate) {
        // param.id = this.bindRandomId; // 文件业务id
      // }
      this.$axios.post(
        Api.contractManage.contractInfo.updateLegalContractLog,
        {
          data: param
        },
        res => {
          if (res.isSuccess) {
            this.getLegalContractLogList();
          } else {
          }
        }
      );
    },
    test(row){
      console.log('test--1',row)
      console.log('this.formPage',this.formPage)
    },
    async newEdit(row){
      console.log('点击了在线编辑')
      // this.$parent.$parent.$parent.$parent.$parent.$parent.$parent.showClose = false;
      console.log('this.$parent',this.$parent)
      this.editOnlineLoading = true;
      console.log('this.editOnlineLoading',this.editOnlineLoading)
      this.fileVersion = '_' + new Date().getTime();

      if (row.templateId) { // 每次编辑，如果能查到模板id，就加载url赋值。用于合同对比
        let fileRes = await this.getFileByBizId(row.templateId);
        console.log('fileRes',fileRes)
        this.templateUrl = fileRes?.data[0]['fileUrl'];
        this.templateId = row.templateId;
      }

      let that = this;
      let copyRow = deepClone(row)

      const pollData = async () => {
        // contractRefFileTypeVos这里面可以判断fileType等于"合同文件"，下面直接取第一个元素
        let newId = this.uploadBindRandomId ? this.uploadBindRandomId : this.fileRelationId;
        let newFileRes = await that.getFileByBizId(newId);
        // let newFileRes = await that.getFileByBizId(this.fileRelationId);
        console.log('newFileRes',newFileRes)
        
        let lastPointIndex = newFileRes.data.length ? newFileRes.data[0]['fileName'].lastIndexOf('.') : ''
        let fileName = newFileRes.data.length ? newFileRes.data[0]['fileName'].slice(0,lastPointIndex) : ''

        console.log('fileName',fileName)
        let lastVersionIndex = fileName.lastIndexOf('_')
        let fileVersion = fileName.slice(lastVersionIndex+1) // 文件上的版本号
        let listFileVersion = copyRow.contractFileVersion ? copyRow.contractFileVersion.split('_')[1] : ''; // 合同列表上的版本号
        console.log('fileVersion',fileVersion)
        console.log('listFileVersion',listFileVersion)
        console.log('大于', fileVersion > listFileVersion)
        
        // 判断合同列表存的版本号和文件名称带的版本号一样或者大于，就不用轮询；不一样代表onlyOffice保存过，可能有延时，需要轮询到版本号一样为止
        if (!listFileVersion || fileVersion == listFileVersion || fileVersion > listFileVersion) {
          // 数据已经有值，停止轮询
          clearInterval(intervalId);
          // 处理有值的数据
          console.log('数据准备就绪:', newFileRes.data[0]?.fileId);
          this.editOnlineLoading = false;

          copyRow.fileId = newFileRes.data[0]['fileId'];
          copyRow.fileUrl = newFileRes.data[0]['fileUrl'];
          copyRow.bizId = newFileRes.data[0]['relationId'];
          this.editOnline(copyRow); // 编辑的时候是取的合同数据，所以数据格式不同

        } else {
          // 数据还是空值，继续轮询
          console.log('数据还没准备好，继续等待...');
        }
      };
      // 设置轮询间隔，单位毫秒
      const pollInterval = 1000; // 0.5秒
      // 开始轮询
      const intervalId = setInterval(pollData, pollInterval);
    },
    // 在线编辑
    editOnline(row){
      // if (this.$parent.$parent.$parent.$parent.$parent.$parent.$parent.title == '发起流程') { // 表单内点了编辑就不要回到上个页面，因为会导致在线编辑文档空白
      //   this.$parent.$parent.$parent.$parent.$parent.$parent.$parent.showClose = false;
      // }
      // fileName
      console.log('editOnline--1',row)
      this.currentDealFileBizId = this.bindRandomId;
      this.fileRelationId = this.bindRandomId;
      console.log('this.fileRelationId',this.fileRelationId)
      this.currentDealContractId = row.contractId;
      this.fileId = row.fileId;
      this.fileUrl = row.fileUrl;
      this.newFileName = row.contractName
      let needSlice = row.fileUrl.lastIndexOf('.')
      this.fileSuffix = row.fileUrl.slice(needSlice)
      // this.newFileName = row.fileName.slice(0,needSlice)
      this.mode = 'edit'
      this.token = generateRandomId(32)
      this.excelKey = generateRandomId(24)
      let sid = localstorageGet('token')
      this.user = {
        name: localstorageGet('userName'),
        id: localstorageGet('userId'),
      }
      console.log('this.fileRelationId',this.fileRelationId)
      console.log('this.uploadBindRandomId',this.uploadBindRandomId)
      let newId = this.uploadBindRandomId ? this.uploadBindRandomId : this.fileRelationId;
      this.callbackUrl = `${baseUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.fileRelationId}&fileName=${this.newFileName+ this.fileVersion+this.fileSuffix}&fileId=${this.fileId}&fileRelationId=${newId}`
      
      // this.callbackUrl = `http://192.168.1.102:9999/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.bindRandomId}&fileName=${this.newFileName+ this.fileVersion+this.fileSuffix}&fileId=${this.fileId}&fileRelationId=${this.fileRelationId}`
      // this.callbackUrl = `${baseUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.bindRandomId}&fileName=${this.newFileName+ this.fileVersion+this.fileSuffix}`
      // this.callbackUrl = `${baseUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.bindRandomId}&fileName=${this.newFileName+ this.fileVersion+this.fileSuffix}&flag=${this.isCreate}&fileId=${this.fileId}&fileRelationId=${this.fileRelationId}`
      // this.callbackUrl = `${baseUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.bindRandomId}&fileName=${this.newFileName}`
      // this.callbackUrl = `http://192.168.1.102:9999/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${this.bindRandomId}`
      console.log('this.callbackUrl',this.callbackUrl)
      this.uploadBindRandomId = '';
      this.$nextTick(() => {
        this.excelVisible = true
      })
    },
    async saveExcel() {
      this.$message.warning("正在保存数据,请耐心等待");
      this.hasClickSave = true;
      // this.$message.success("生成报告成功");
      //强制保存数据
      let sid = localstorageGet('token')
      const fetchConfig = {
        url: `${baseUrl}/web/windPowerEconomyEvaluationModel/save?sid=${sid}&platformCode=200001`,
        method: 'POST',
        data: { "c": "forcesave", "key": this.excelKey }
      }
      let editRes = await axios(fetchConfig)

      this.dealContractFile();
      console.log('this.contractFileData222',this.contractFileData)
    },
    // 文档保存后统一处理
    dealContractFile(){
      // 文件名称的修改改成由历史列表存储
      // this.updateFile(this.fileId,this.newFileName).then(res=>{
      //   let needSlice = res.data['fileName'].lastIndexOf('.')
      //   this.$set(this.contractFileData[0],'updateDate',res.data['updateDate'])
      //   this.$set(this.contractFileData[0],'contractName',res.data['fileName'].slice(0,needSlice))
      // }) // 更新文件名称（有可能改名字）
      console.log('this.fileVersion',this.fileVersion)
      console.log('this.fileVersion2',this.fileVersion.split('_'))
      let time = this.fileVersion.split('_')[1]
      const date = moment(Number(time));
      const formattedDate = date.format('YYYY-MM-DD HH:mm:ss');
      this.$set(this.contractFileData[0],'contractName',this.newFileName)
      this.$set(this.contractFileData[0],'contractFileVersion',this.fileVersion)
      this.$set(this.contractFileData[0],'updateDate',formattedDate)
      if (this.formPage == 'useTemplate') {
        this.formPage = 'myContractEdit'
      }
      
      // this.$set(this.contractFileData[0],'contractFileVersion',this.contractFileVersion)
      this.excelVisible = false
    },
    loopGetFile() {
      // console.log('loopGetFile')
      return new Promise((resolve, reject) => {
        let res = null, timeout
        var fun = async () => {
          res = await this.getFileByBizId(this.bindRandomId)
          // console.log('res',res)
          if (!res.data.length) {
            timeout = setTimeout(() => {
              fun()
            }, 2000)
          }
          else {
            clearTimeout(timeout)
            resolve(res)
          }
        }
        fun()
      })
    },
    //根据业务id重命名文件
    updateFile(id,fileName) {
      return this.$axios.post(
        Api.user.updateFile, {
        data: {
          id: id,
          fileName:fileName
        }
      })
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
    //根据业务id获取文件
    getFileByBizId(id) {
      return this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      })
    },
    closeDialog() {
      this.excelVisible = false
    },
    beforeRemove(file, fileList) {
      console.log('beforeRemove')
      // if (!this.form?.id) return true;
      // console.log('file',file)
      // const id = file.response ? file.response.data.id : file.fileId;
      // return new Promise((resolve, reject) => {
      //   this.$confirm('是否确定删除文件?', '提示', {
      //     closeOnClickModal: false,
      //     confirmButtonText: '确定',
      //     cancelButtonText: '取消',
      //     type: 'warning'
      //   }).then(() => {
      //     this.$axios.post(
      //       Api.user.deleteBatchFile,
      //       {
      //         data: {
      //           relationId: this.form?.id,
      //           fileIds: [id]
      //         }
      //       },
      //       res => {
      //         if (res.isSuccess) {
      //           this.$message.success('解除关联成功');
      //           resolve();
      //         } else {
      //           reject(res.message);
      //         }
      //       }
      //     );
      //   }).catch((err) => {
      //     reject(err);
      //   });
      // });
    },
    handleRemove(file, fileList) {
      // console.log(file, 'file');
      this.fileList = fileList;
      const id = file.response ? file.response.data.id : file.fileId;
      this.contractFileData = this.contractFileData.filter(e => e != id);
    },
    // 文件上传
    handleAvatarSuccess(res, file) {
      console.log(res, file, '++++');
      if (res.code == 'RESP200') {
        let needSlice = res.data.originFileName.lastIndexOf('.')
        res.data.contractName = res.data.originFileName.slice(0,needSlice);
        console.log('res.data.contractName',res.data.contractName)
        res.data.fileUrl = res.data.absolutelyFileUrl;
        this.fileVersion = '_' + new Date().getTime();
        this.uploadBindRandomId = generateRandomId(32);
        this.bindFileById(this.uploadBindRandomId, res.data.id).then(x=>{
          let name = res.data.contractName+this.fileVersion
          this.updateFile(res.data.id, name)
        });
        
        this.firstDealContractFileList(res.data.id,res.data.contractName,res.data.fileUrl,res.data.updateDate,this.bindRandomId,this.fileVersion)
      } else {
        this.$message.error(`文件上传失败,请重新上传`);
      }
    },
    beforeAvatarUpload(file) {
      // console.log(file, '3333');
      if (this.fileList.length == 1) {
        this.$message.error(
          '只能上传一个文件'
        );
        return false;
      }
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
    // ===============表单内业务代码开始：合同关联手续文件、合同信息新增和修改、合同文件历史版本保存==================
    // 合同合规性评审业务逻辑：表单整体分为合同信息+手续文件+合同文件+合规审查日志
    async contractBusiness(generateFormRefs,that,flag,type){
        this.batchCode = that.batchCode;
        let isNeedValidate = that.clickMethod == 'draft' ? false : true  //是否进行表单验证，如果为true则进行表单验证，如果是false，不验证，草稿时候不验证
        console.log('======isNeedValidate======',isNeedValidate)
        if (!this.contractFileData.length) {
          this.$message.warning('请上传合同文件！')
          return;
        }

        console.log('=======contractBusiness=======',this.businessId)
        // 表单后面要想办法先验证
        generateFormRefs.getData(isNeedValidate).then(async x => {
          let value = generateFormRefs.getValues();
          this.deleteLegalProcedureFile(value,generateFormRefs);
          console.log('=====value====',value)
          console.log('=======this.fromOutsidePage======',this.fromOutsidePage);
          // let saveContractInfoResData = null;
          // if (!this.fromOutsidePage) { // 不是从外部页面进来，才需要调用合同保存。外部进来已经保存过合同
          let saveContractInfoResData = await this.saveOrModifyContractInfo(value,that,type);
          // }

          // console.log('saveContractInfoResData',saveContractInfoResData)
          // 发起流程的合同id保存合同信息后接口返回；审批或查看从外部传进来的businessId（从外部页面进来，并且已有合同id,进来流程审批也用businessId）
          let contractId = this.formPage ? this.businessId : (this.isFlowInitiate ? saveContractInfoResData.id : this.businessId);
          console.log('this.businessId',this.businessId)
          console.log('saveContractInfoResData.id',saveContractInfoResData.id)
          // let contractId = !this.isFlowInitiate || this.fromOutsidePage ? this.businessId : saveContractInfoResData.id;
          console.log('=======contractId======',contractId);
          that.businessId = contractId;
          if (this.formPage) { // 从我的合同那边过来
            // this.legalContractUpdate();
            if (this.hasClickSave) {
              let legalContractResData = await this.legalContractSave(contractId)
            }
          } else { // 直接在流程审批发起
            console.log('直接发起')
            console.log('contractId',contractId)
            if (this.isFlowInitiate) { // 必须点了在线编辑，并且点了保存按钮，进行审核都会保存一次历史记录
              let legalContractResData = await this.legalContractSave(contractId)
              if (this.uploadBindRandomId) { // 初次发起，点了上传文件按钮，然后没有点击在线编辑，直接提交需要单独自己关联文件
                let listData = this.contractFileData[0];
                this.bindFileById(this.bindRandomId, listData['fileId']).then(x=>{
                  let name = listData['contractName']+listData['contractFileVersion']
                  this.updateFile(listData['fileId'], name)
                });
              }
            } else {
              if (this.hasClickSave) {
                if (this.isReInitiate) { // 如果是草稿等重新发起，保存了内容不要每次新增合同日志, 换成修改日志
                  this.legalContractUpdate();
                } else {
                  let legalContractResData = await this.legalContractSave(contractId)
                }
              }
            }
          }
          
          
          this.legalProcedureBind(value,contractId,generateFormRefs);

          setTimeout(x=>{
            if (!this.isFlowInitiate) { // 不是发起流程:审批或重新发起
              console.log(1)
              if (that.isReInitiate) { // 重新发起
                console.log('重新发起')
                console.log('that.isDraftInReInitiateStatus',that.isDraftInReInitiateStatus)
                console.log(2)
                if (that.isDraftInReInitiateStatus) {
                  that.handleSaveDraft();
                } else {
                  this.originContractTalk = value.contractTalk;
                  that.handleRePostSubmit(flag)
                }
                // that.handleRePostSubmit(flag)
              } else { // 审批
                console.log(3)
                this.originContractTalk = value.contractTalk;
                that.handleSubmitCheck(flag,type)
              }
            } else { // 发起流程
              console.log(4)
              this.originContractTalk = value.contractTalk;
              that.enterpriseHandleSubmit(flag)
            }
          },200)

        }).catch(err=>{
          this.$message.error('请检查表单是否填写完毕')
          console.log('表单校验不通过',err)
        })
    },
    // 业务-保存和修改合同信息
    saveOrModifyContractInfo(formVal,that,type){
      return new Promise((resolve, reject) => {
        console.log('saveOrModifyContractInfo')
        console.log('formVal',formVal)
        
        let contractSubmitType = !this.isFlowInitiate ? type : that.clickMethod
        let url = this.formPage ? Api.contractManage.contractInfo.modifyContract : (this.isFlowInitiate ? Api.contractManage.contractInfo.saveContractInfo : Api.contractManage.contractInfo.modifyContract)
        // let url = !this.isFlowInitiate || this.fromOutsidePage ? Api.contractManage.contractInfo.modifyContract : Api.contractManage.contractInfo.saveContractInfo
        let param = {
          "projectId":formVal.project ? JSON.parse(formVal.project).id : '',
          "contractProjectName":formVal.project ? JSON.parse(formVal.project).name : '',
          "classificationId":!!formVal.classificationId.length ? formVal.classificationId[formVal.classificationId.length-1] : '',
          "contractName":formVal.contractName,
          "contractNumber":formVal.contractNumber,
          // "contractBody":"合同主体",//（合同主体需要前端逻辑整理）
          "contractContent":formVal.contractContent,
          "contractTalk":formVal.contractTalk,
          "contractSum":formVal.contractSum,
          "contractUserName":JSON.parse(formVal.contractUserName).name,
          "contractDepName":JSON.parse(formVal.contractDepName).name,
          "contractCompanyName":JSON.parse(formVal.contractCompanyName).name,
          "handledBy":JSON.parse(formVal.contractUserName).id,
          "payMethod": formVal.payMethod, //收付款方式
          "contractSubtableVo":{
            "businessTime":formVal.businessTime? formVal.businessTime+' 00:00:00' : '',
            "paymentType":formVal.paymentType,  //收付款分类  0收款1付款2其它 3收付款合同
            "handlingDepartment":JSON.parse(formVal.contractDepName).id,
            "handlingCompany":JSON.parse(formVal.contractCompanyName).id,
            "stampStatus":null,  // 状态:0草稿1提交,传null代表还没走审核流程（合同盖章状态）
            "stampExamineStatus":null, // 审核状态:0审核中1审核成功2审核失败,传null代表还没走审核流程（合同盖章状态）
            // "contractId":"合同id",（合规性-没有就不需要）
            // "entrustId":"关联委托id",（合规性-没有就不需要）
            // "templateId":"模板id",（合规性-没有就不需要）
          },
          // "status": this.contractSubmitType == '' || this.contractSubmitType == 'submit' ? "1" : "0",  // 状态:0草稿1提交（合规审查状态）
          "status": contractSubmitType == 'draft' ? "0":"1",  // 状态:0草稿1提交（合规审查状态）
          "examineStatus":"0",  // 审核状态:0审核中1审核成功2审核失败（合规审查状态）
          "companyId": this.$store.state.user.companyId,
          "fileVersion": this.fileVersion
        }
        // console.log(2)
        console.log(22222333444,formVal)
        console.log('formVal.contractTalk',formVal.contractTalk)
        console.log('formVal.FGWfangshi_file3_flow',formVal.FGWfangshi_file3_flow)
        console.log('this.originContractTalk',this.originContractTalk)

        // 下面几个需要关联各自流程
        if (formVal.contractTalk == '单一来源采购') {
          if (formVal.DYLYcaigou_file1_flow) {
            param.contractSubtableVo.processId = JSON.parse(formVal.DYLYcaigou_file1_flow).id // 流程id
            param.contractSubtableVo.flowName = JSON.parse(formVal.DYLYcaigou_file1_flow).name // 流程名称
          }
        } else if (formVal.contractTalk == '内部采购') {
          if (formVal.NBcaigou_file1_flow) {
            param.contractSubtableVo.processId = JSON.parse(formVal.NBcaigou_file1_flow).id // 流程id
            param.contractSubtableVo.flowName = JSON.parse(formVal.NBcaigou_file1_flow).name // 流程名称
          }
        } else if (formVal.contractTalk == '非挂网方式') {
          console.log('非挂网方式')
          // if (formVal.FGWfangshi_file3_flow && this.originContractTalk != formVal.contractTalk) {
          if (formVal.FGWfangshi_file3_flow) {
            param.contractSubtableVo.processId = JSON.parse(formVal.FGWfangshi_file3_flow).id // 流程id
            param.contractSubtableVo.flowName = JSON.parse(formVal.FGWfangshi_file3_flow).name // 流程名称
          }
        }
        // console.log(3)
        console.log('param',param)
        // if (!this.isFlowInitiate || this.fromOutsidePage ) {
        if (this.formPage || !this.isFlowInitiate) {
          param.id = this.businessId || this.contractId;
          delete param.companyId
        }

        let str = '',str2='';
        let contractBodyListCopy = JSON.parse(JSON.stringify(this.contractBodyList));
        if (formVal.mainBodyRadio == '手动填写'){
          formVal.mainBodySubform.forEach(x=>{
            // 处理合同主体数据转换（修复主体方名称为数字，导致合同管理模块页面后端查询不到的bug）
            let firstBody = getObjById(contractBodyListCopy,x.select,'dictDataVos','dictValue')
            str+=` ${firstBody.dictLabel}:${x.input},`
            // 另外存一份未转换的数据用于盖章评审表单的回显，带出数据
            str2+=`${x.select}:${x.input}`+','
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
        // console.log('str',str)
        param.contractBody = str;
        param.contractBodys = str2; // contractBodys存的主体数据用于盖章评审表单的发起流程数据回显

        console.log('currentPendingNodeName',that.currentPendingNodeName)
        if (that.currentPendingNodeName == '法务专员') {
          param.examineUserId = localstorageGet('userId');
          param.examineUserName = localstorageGet('userName');
        }

        // console.log(4)
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
    // 删除选择的合同谈判方式以外的其他手续文件
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
      let objArr = []
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
      console.log('=====legalProcedureBind2======');
      console.log('formVal',formVal)
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

      // 切换方式时，对已绑定流程的进行处理
      console.log('切换谈判方式2',contractTalkName)
      console.log('this.originContractTalk',this.originContractTalk)
      // let flag = this.originContractTalk != '' && this.originContractTalk != contractTalkName;
      // console.log('flag',flag)
      // if (contractTalkName == '单一来源采购' && flag) {
      //   generateFormRefs.setData({DYLYcaigou_file1_flow: JSON.stringify({
      //     'flowType': 'single_source_delegation', // 单一来源委托单类型
      //   })})
      // } else if (contractTalkName == '内部采购' && flag) {
      //   generateFormRefs.setData({NBcaigou_file1_flow: JSON.stringify({
      //     'flowType': 'internal_delegation',// 内部委托单类型
      //   })})
      // } else if (contractTalkName == '非挂网方式' && flag) {
      //   console.log('进来1111')
      //   generateFormRefs.setData({FGWfangshi_file3_flow: JSON.stringify({
      //     'flowType': 'proposed_supplier_review',// 拟选用供应商评审表类型
      //   })})
      // }

      let needBindFileList = {}; // 需要绑定的文件列表(遍历合规手续文件的字段表单里面有数据的存放进来，待上传)
      let contractRefFileTypeVos = []; // 合同id需要绑定的已绑定文件列表的临时bizId
      let temporaryBizId = '';
      for (var i in formVal) {
        if (i.indexOf(objList[contractTalkName]) > -1 && formVal[i].length>0) {
          needBindFileList[i] = formVal[i]
        }
      }
      // console.log('needBindFileList',needBindFileList)
      for (var j in needBindFileList) {
        let fileIdList = [];
        let ifHasTemIdObj = this.contractRefFileList.find(x=>x.fileType == j)
        // console.log('===ifHasTemIdObj===',ifHasTemIdObj)
        if (ifHasTemIdObj) { // 上传文件前需要判断这个类型是否已经绑定过了临时的业务id，如果已有就不要再次创建临时ID绑定（业务用原来的临时id）
          temporaryBizId = ifHasTemIdObj.bizId;
        } else { // 没有临时Id的所有文件遍历绑定
          temporaryBizId = generateUid()
          // temporaryBizId = generateRandomId(32)
        }

        if (Array.isArray(needBindFileList[j])) {
          needBindFileList[j].forEach(item=>{
            fileIdList.push(item.data?.fileId || item.data.id) // 数据不用在发起时回显的时候文件id取的id，不是fileId
          })

          // console.log('fileIdList',fileIdList)
          // console.log('temporaryBizId',temporaryBizId)
          this.saveBatchFile(temporaryBizId,fileIdList) // 文件批量绑定
          // 1、合规手续绑定临时id
          contractRefFileTypeVos.push({
            bizId:temporaryBizId,
            fileType:j,
          })
        }
      }
      // 2、合同文件绑定临时id
      let contractTemporaryId = '';
      let contract_ifHasTemIdObj = this.contractRefFileList.find(x=>x.fileType == '合同文件')
      console.log('contract_ifHasTemIdObj',contract_ifHasTemIdObj)
      if (contract_ifHasTemIdObj) { // 上传文件前需要判断这个类型是否已经绑定过了临时的业务id，如果已有就不要再次创建临时ID绑定（业务用原来的临时id）
        contractTemporaryId = contract_ifHasTemIdObj.bizId;
      } else { // 没有临时Id的所有文件遍历绑定
        contractTemporaryId = this.fileRelationId
        // contractTemporaryId = this.bindRandomId
      }
      contractRefFileTypeVos.push({
        bizId:contractTemporaryId,
        fileType:'合同文件',
      })

      // 3、文件绑定完临时id后，还需要和合同Id绑定
      await this.saveContractRefFile(contractId,contractRefFileTypeVos)
      // // 4、删除其他不需要的文件关联关系
      // if(needUnbindFileList.length) {
      //   this.deleteFileRelation(temporaryBizId,needUnbindFileList);
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
      let params = {
        relationId: bizId,
        fileIds: fileIds
      }
      this.$axios.post(
        Api.user.saveBatchFile,
        {
          data: params
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
    //根据合同id查文件类相关
    getFileByContractId(id) {
      return this.$axios.post(
        Api.contractManage.contractInfo.getFileByContractId, {
        data: {
          id: id
        }
      })
    },

    // ===============业务保存代码结束==================
    beforeGetFile(type,row){
      this.viewLoading = true;

      let that = this;
      let copyRow = deepClone(row)

      const pollData = async () => {
        let newFileRes = await that.getFileByBizId(this.fileRelationId);
        console.log('newFileRes',newFileRes)
        let lastPointIndex = newFileRes.data.length ? newFileRes.data[0]['fileName'].lastIndexOf('.') : ''
        let fileName = newFileRes.data.length ? newFileRes.data[0]['fileName'].slice(0,lastPointIndex) : ''
        console.log('fileName',fileName)
        let lastVersionIndex = fileName.lastIndexOf('_')
        let fileVersion = fileName.slice(lastVersionIndex+1) // 文件上的版本号
        let listFileVersion = copyRow.contractFileVersion ? copyRow.contractFileVersion.split('_')[1] : ''; // 合同列表上的版本号
        // console.log('fileVersion',fileVersion)
        // console.log('listFileVersion',listFileVersion)
        // console.log('大于', fileVersion > listFileVersion)
        
        // 判断合同列表存的版本号和文件名称带的版本号一样或者大于，就不用轮询；不一样代表onlyOffice保存过，可能有延时，需要轮询到版本号一样为止
        if (!listFileVersion || fileVersion == listFileVersion || fileVersion > listFileVersion) {
          // 数据已经有值，停止轮询
          clearInterval(intervalId);
          // 处理有值的数据
          console.log('数据准备就绪:', newFileRes.data[0]?.fileId);
          that.viewLoading = false;

          if (type == 'view') {
            that.viewFile(newFileRes.data[0]['fileUrl'])
          } else {
            that.downLoadFile({
              fileUrl: newFileRes.data[0]['fileUrl'],
              contractName: fileName
            })
          }
        } else {
          // 数据还是空值，继续轮询
          console.log('数据还没准备好，继续等待...');
        }
      };
      // 设置轮询间隔，单位毫秒
      const pollInterval = 1000; // 0.5秒
      // 开始轮询
      const intervalId = setInterval(pollData, pollInterval);
    },
    // 预览
    viewFile(url) {
      // window.open(`${viewFileUrl}/onlinePreview?sid=${localstorageGet('token')}&url=${encodeURIComponent(this.$Base64.encode(url))}`);
      viewFile(url)
    },
    // 下载文件
    downLoadFile(item) {
      console.log('item',item)
      let needSlice = item.fileUrl.lastIndexOf('.')
      let fileSuffix = item.fileUrl.slice(needSlice)
      console.log('download',fileSuffix)
      this.downLoadDocument(item.fileUrl, item.contractName + fileSuffix);
    },
    downLoadDocument(url, name = '下载文件') {
      if (!!window.ActiveXObject || 'ActiveXObject' in window) {
        // IE 无法识别 download 属性，用户自行保存
        this.$message.error(
          '当前浏览器不支持点击下载，请手动保存，或者切换到Google Chrome浏览器进行下载'
        );
      } else {
        const x = new XMLHttpRequest();
        x.open('GET', url, true);
        x.responseType = 'blob';
        x.onload = function () {
          const url = window.URL.createObjectURL(x.response);
          const a = document.createElement('a');
          a.href = url;
          a.download = name;
          a.click();
          a.remove();
        };
        x.send();
      }
    },
  },
};
</script>
<style lang="scss" scoped>
  
  .word-online {
    height: calc(-104px + 100vh) !important;
  }
  ::v-deep {
    .el-table th.el-table__cell.is-leaf, .el-table td.el-table__cell {
      // border-color: rgb(153, 153, 153);
      border-right: 2px solid rgb(153, 153, 153);
    }
    .el-table thead tr > :last-child.el-table__cell.is-leaf {
      border-right: 1px solid rgb(153, 153, 153) !important;
    }
    .el-table .el-table__header-wrapper,.el-table .el-table__fixed-right {
      // border-top: 2px solid rgb(153, 153, 153);
      border-left: 1px solid rgb(153, 153, 153);
    }
    .el-table--border .el-table__header-wrapper .cell,.el-table--border .el-table__body-wrapper .cell, .el-table--border .el-table__fixed-right .cell{
      text-align: center;
    }
    .fm-form .el-form-item {
      padding: 0px;
    }
    .excel-div .el-dialog__body {
      padding: 0;
      // height: calc(100% - 35px) !important;
      overflow-x: hidden;
      min-height: initial !important;
      max-height: initial !important;
    }

    .el-table.legalTableClass .el-table__header-wrapper thead tr th.el-table__cell { //11.7新增
      border-right: 3px solid rgb(153, 153, 153) !important;
    }
    // .el-table .el-table__header-wrapper{
    //   border-left: none;
    // }
    .el-dialog__headerbtn{
      z-index: 120;
    }
  }

  @media print {
    ::v-deep {

      // 测试
      // table {
      //     table-layout: auto;
      // }
      .el-table__header-wrapper .el-table__header {
          width: 100% !important;
      }
      .el-table__body-wrapper .el-table__body {
          width: 100% !important;
      }
      // 设置表格边框样式
      table {
          table-layout: auto;
          border: 1px solid rgb(153, 153, 153) !important;
          border-color: rgb(153, 153, 153) !important;
          border-collapse: collapse;
          font-size: 8px !important;
          color: rgb(153, 153, 153) !important;
          td {
              border: 1px solid  rgb(153, 153, 153) !important;
          }
          th {
              border: 1px solid rgb(153, 153, 153) !important;
              border-bottom: none !important;
              width: auto !important;
          }
      }
      // 清楚原来的边框样式
      .el-table--border::after,
      .el-table--group::after
      .el-table::before {
          background-color: transparent;
      }
      .el-table::before {
          height: 0px;
      }
      .el-table--border::after {
          width: 0px;
      }
      .el-table--border {
          border: none;
      }
      .el-table--group::after,
      .el-table--border::after {
          width: 0;
      }
      .el-table--group,
      .el-table--border border {
          border: none !important;
      }
    }
  }
</style>
