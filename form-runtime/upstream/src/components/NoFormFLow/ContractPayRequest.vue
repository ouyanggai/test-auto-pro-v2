<!-- 合同付款申请表 -->
<template>
  <div :style="{'width':formWidth}" class="form-container">
    <div class="title">
      <h3>{{ formTitle }}</h3>
    </div>
    <el-row class="attach-div" >
      <el-col :span="14" >
        <div style="min-height:10px;">
          <elupload ref="elupload"  title="上传支付凭证" v-if="operaType == 'create'"></elupload>
          <elupload ref="elupload"  title="上传支付凭证" v-else-if="operaType == 'preview'" :attachFile="attachFile" :showOnly="true"></elupload>
          <elupload ref="elupload"  title="上传支付凭证" v-else-if="operaType == 'edit'" :attachFile="attachFile"></elupload>
          <elupload ref="elupload"  title="上传支付凭证" v-else-if="operaType == 'examine'" :attachFile="attachFile" :showOnly="true"></elupload>
        </div>
      </el-col>
      <el-col :span="5">
        <div class="col-div">
          公司编号：<el-input placeholder="公司编号" v-model="companyCode" maxlength="200" style="width: 60%;" :disabled="true"></el-input>
        </div>
      </el-col>
      <el-col :span="5">
        <div class="col-div">
          项目编号：<el-input placeholder="项目编号" v-model="projectCode" maxlength="200" style="width: 60%;" :disabled="true"></el-input>
        </div>
      </el-col>
    </el-row>
    <!-- 主要信息 -->
    <el-card>
      <CommonForm :formConfig="formMainConfig" :width="formWidth" ref="mainForm" :initForm="initForm"></CommonForm>
    </el-card>
    <!-- 材料设备明细详情 -->
    <el-dialog title="选择合同的结算金额" :visible="detailVisible" v-if="detailVisible" :close-on-click-modal="false" :append-to-body="true"
      width="980px" @close="closeDialog('detailVisible')" style="height: 85vh;overflow:hidden;">
      <div style="height: 48vh;overflow:auto;">
        <SimpleTable :tableConfig="listDetailConfig" :tableData="tableData" :multipleSelection.sync="multipleSelection"></SimpleTable>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="closeDialog('detailVisible')" plain>取 消</el-button>
        <el-button @click="confirm" type="primary">确 定</el-button>
      </div>
    </el-dialog>
    <!-- 材料设备明细详情end -->

    <CommonFooter  @submit="submit" @reSubmit="reSubmit" v-if="operaType =='create' || operaType =='edit'" :isReInitiate="isReInitiate"></CommonFooter>
    <Flow ref="flow" v-bind="$attrs"></Flow>
  </div>
</template>

<script>
import Api from '@/api';
import {localstorageGet} from '@/utils/auth'
import elupload from '@/components/EleUpload'
import Flow from './components/Flow'
import CommonForm from './components/CommonForm'
import {formMainConfig,listDetailConfig} from './config/ContractPayRequestConfig'
import SimpleTable from './components/SimpleTable.vue';
import CommonFooter from './components/CommonFooter'
import mixin from './mixin/mixin'
import {deepClone} from '@/utils'
const TYPE = {
  server:{name:'服务',func:'getSettlementServe'},
  material:{name:'材料',func:'getEquipmentSettlementList'},
  engineering:{name:'工程',func:'getEngineSettlementList'},
  device:{name:'设备',func:'getEquipmentSettlementList'},
}
export default {
  name:'contract_pay_request',
  components: {elupload,Flow,CommonForm,CommonFooter,SimpleTable},
  props: ['operaType','isReInitiate','otherBizId','flowNodeProxyId'],
  mixins:[mixin],
  data() {
    return {
      formWidth:'1080px',
      formTitle:'合同付款申请表',
      formMainConfig:deepClone(formMainConfig),
      companyCode:'',
      projectCode:'',
      initForm:{},
      relativeList:[],
      detailVisible:false,
      listDetailConfig:deepClone(listDetailConfig),
      tableData:[],
      multipleSelection:[],
      attachFile:[],
      checkAmountReview:false, //是否需要校验复核金额
    };
  },
  created() {
    console.log('this.operaType',this.operaType)
    this.loadRelTreeList()
    this.initForm = this.initFormList(formMainConfig)
    this.initForm.application = localstorageGet('companyName')
    this.initForm.userName = localstorageGet('userName')
    if(this.operaType == 'create'){
      this.contractReviewList().then(res=>{
        var list = this.contractList = res
        this.assignValue(this.formMainConfig,'reviewId','options',list)
        this.assignValue(this.formMainConfig,'reviewId','changeEvent',this.contractChoose)
        this.assignValue(this.formMainConfig,'selectList','clickEvent',this.showChooseList)
      })
    }else{
      console.log('this.otherBizId',this.otherBizId)
      if(this.otherBizId){
        this.getContractPaymentById().then(res=>{
          if(res.isSuccess){
            this.contractReviewList().then(list=>{
              var data = res.data
              console.log('data123',data)
              var list = this.contractList = list
              console.log('list123',list)
              data.contractReviewVo.value = data.contractReviewVo.id; // 临时加
              data.contractReviewVo.label = data.contractReviewVo.contractName; // 临时加
              this.assignValue(this.formMainConfig,'reviewId','options',[data.contractReviewVo])
              console.log('this.formMainConfig',this.formMainConfig)
              // this.assignValue(this.formMainConfig,'reviewId','options',list)
              this.$nextTick(()=>{
                this.insertDataToForm(data)
                if(this.operaType == 'examine'){
                  this.getInputPermision().then(res=>{
                    let data  = res || []
                    var cloneConfig = deepClone(this.formMainConfig)
                    this.formMainConfig = this.setDisableData(cloneConfig,data)
                  })
                }else if(this.operaType == 'preview'){
                  //修改congfig
                  this.setDisableData(this.formMainConfig,[])
                }
              })
            })
          }else{

          }
        })
      }else{
        if(this.operaType == 'preview'){
          this.setDisableData(this.formMainConfig,[])
        }
      }
    }

  },
  mounted() {},
  watch: {
    relativeList(val){
      if(val && val.length && this.initForm.secondPartyId){
          var find = val.find(item=>item.value == this.initForm.secondPartyId)
          this.initForm.secondParty = find.label
        }
    },
    'initForm.secondPartyId':{
      handler(val){
        if(val && this.relativeList.length){
          var find = this.relativeList.find(item=>item.value == val)
          this.initForm.secondParty = find?.label || '没有账户'
        }
      }
    },
    'initForm.retentionMoney':{
      handler(val){
        if(val!=0){
          var rules = [{ required: true, message: '请输入预留质保金'}]
          this.assignValue(this.formMainConfig,'retentionMoneyDetail','rules',rules)
        }else{
          this.assignValue(this.formMainConfig,'retentionMoneyDetail','rules','')
        }
      },
      immediate:true
    }
  },
  computed: {},
  methods: {
    insertDataToForm(data){
      console.log('insertDataToForm',data)
      this.originData = data
      var attachment = data.voucher
      if(attachment){
        this.getBatchFile(data.id).then(res=>{
          if(res.isSuccess){
            this.attachFile = res.data || []
          }
        })
      }
      var contractReviewVo = data.contractReviewVo
      this.initForm.reviewId = data.reviewId
      this.initForm.contractType = contractReviewVo.contractType
      this.initForm.contractSum = contractReviewVo.contractSum
      this.initForm.paidAmount = data.paidAmount
      this.initForm.contractNumber = contractReviewVo.contractNumber
      this.initForm.residueAmount = this.initForm.contractSum - this.initForm.paidAmount
      this.initForm.monthAmount = data.monthAmount
      this.initForm.fuhe = this.initForm.paymentAmount = data.paymentAmount
      this.initForm.secondParty = contractReviewVo.secondParty
      this.initForm.paymentAccount = contractReviewVo.paymentAccount
      this.initForm.bank = contractReviewVo.bank
      this.initForm.lineNumber = contractReviewVo.lineNumber
      this.initForm.contractPerform = data.contractPerform
      this.initForm.qualityObjection = data.qualityObjection
      this.initForm.claimantMatter = data.claimantMatter
      this.initForm.retentionMoney = data.retentionMoney !=0 ? '1':'0'
      this.initForm.retentionMoneyDetail = data.retentionMoney
      this.initForm.payDate = data.payDate
      this.initForm.bankPayMethod = data.bankPayMethod
      this.initForm.bankPayMethod = contractReviewVo.bankPayMethod
      this.initForm.remarks = data.remarks
      this.companyCode = data.contractReviewVo.companyCode
      this.projectCode = data.contractReviewVo.projectCode
      this.initForm.amountReview = data.amountReview
    },
    getContractPaymentById(){
      var data = {
        id:this.otherBizId
      }
      return this.$axios.post(Api.noForm.getContractPaymentById,{data})
    },
    showChooseList(){
      var promise
      var func = this[TYPE[this.initForm.contractType].func]
      if(func && typeof(func) === 'function')promise = func.call(this)
      if(promise){
        promise.then(res=>{
          if(res.isSuccess){
            this.processListData(res.data || [])
            this.detailVisible = true
          }
        })
      }
    },
    getData(){
      const param = {
        data: {
          companyId: localstorageGet('companyId'),
          projectId: localstorageGet('projectId'),
          tableType: '7',
          endTime: '',//formatNewTime(month,'yyyy-MM-dd HH:mm:ss'),
          status: '1'
        },
        pages: 1,//this.currentPage,
        size: 999//this.pageSize
      };
      this.$axios.post(
        Api.engineeringSettlement.engineeringList,
        param,
        res => {
          if(res&&res.isSuccess){
            if(res.data.dataList&&res.data.dataList.length){
              res.data.dataList.forEach(item=>{
                item.name = item.scheduleName
                item.settle = item.settlementMoney
                item.initiator = item.userName
                item.relativeContract = item.contractName
              })
              console.log('res.data.dataList',res.data.dataList)
              this.tableData = res.data.dataList;
              // this.total = res.data.total;
            }else{
              this.tableData = [];
              // this.total = 0
            }
          }else{
            this.$message.error(res.message)
          }
        })
    },
    processListData(data){
      console.log('processListData')
      console.log('processListData1',this.initForm.contractType)
      if(this.initForm.contractType == 'engineering'){
        this.getData()
        // var ids = []
        // data.forEach(item=>{
        //   var flowInstanceBizRelevanceList = item.flowInstanceBizRelevanceList || []
        //   var find = flowInstanceBizRelevanceList.find(el=>el.otherBiz == 'quantities')
        //   if(find){
        //     ids.push(find.otherBizId)
        //     item.findId = find.otherBizId
        //   }
        // })
        // var query = {
        //   data:{},
        //   ids,
        //   pages:1,
        //   size:ids.length
        // }
        // this.$axios.post(Api.schedule.findIdsList,query).then(res=>{
        //   if(res.isSuccess){
        //     var list = res.data.dataList
        //     list.forEach(item=>{
        //       var id = item.id
        //       var index = data.findIndex(el=>el.findId == id)
        //       if(index > -1){
        //         data[index].settle = item.inspectionSum || 0
        //       }
        //     })
        //     this.tableData = data
        //   }
        // })
        // engineeringSettlement/engineering/list
      }else if(this.initForm.contractType == 'server'){
        var settlementServeVoList = data.settlementServeVoList
        var tableData = settlementServeVoList.map(item=>{
          return {
            id:item.id,
            name:item.contractName,
            settle:item.settlementMoney,
            initiator:item.userName,
            relativeContract:item.contractName,
            createDate:item.createDate
          }
        })
        this.tableData = tableData
      }else{
        var equipmentSettlementVoList = data.equipmentSettlementVoList
        var tableData = equipmentSettlementVoList.map(item=>{
          return {
            id:item.id,
            name:item.contractName,
            settle:item.settlementMoney,
            initiator:item.userName,
            relativeContract:item.projectName,
            createDate:item.createDate
          }
        })
        this.tableData = tableData
      }
    },
    getEquipmentSettlementList(){
      var data = {
        data : {
          settlementType:TYPE[this.initForm.contractType].name,
          projectId:localstorageGet('projectId'),
          contractId:this.initForm.reviewId
        },
        pagination: true,
        pages: 1,
        size: 9999
      }
      return this.$axios.post(Api.noForm.getEquipmentSettlementList,data)
    },
    getSettlementServe(){
      var data ={
        data:{
          projectId: localstorageGet('projectId'),
          companyId: localstorageGet('companyId')
        },
        pagination: true,
        pages: 1,
        size: 9999
      }
      return this.$axios.post(Api.noForm.serveList,data)
    },
    getEngineSettlementList(){
      var data = {
        useScope: "invest",
        auditWay:"quantities",
        initiator: "all",
        status:'end',
        flowInstanceBizRelevanceList:[
          {
            otherBiz: "project",
            otherBizId: this.$store.state.user.projectId
          }
        ]
      }
      return this.$axios.post(Api.schedule.getFlowInstanceList,{data})
    },
    contractChoose(val){
      var find = this.contractList.find(item=>item.value == val)
      console.log('contractChoose-find',find)
      if(find){
        this.initForm.contractNumber = find.contractNumber
        this.initForm.contractType = find.contractType
        this.initForm.contractSum = find.contractSum
        this.$set(this.initForm,'secondPartyId',find.secondPartyId)
        this.initForm.paymentAccount = find.paymentAccount
        this.initForm.bank = find.bank
        this.initForm.lineNumber = find.lineNumber
        this.initForm.bankPayMethod = find.bankPayMethod
        this.initForm.paidAmount = find.paidAmount || 0
        this.initForm.residueAmount = this.initForm.contractSum - this.initForm.paidAmount
        this.initForm.monthAmount = '';
        this.companyCode = find.companyCode
        this.projectCode = find.projectCode
        this.assignValue(this.formMainConfig,'selectList','disabled',false)
      }
    },
    contractReviewList(){
      var data = {
        data:{
          contractName:'',
          examineStatus:1,
          firstPartyId:localstorageGet('companyId'),
          projectId:this.$store.state.user.projectId
        },
        pagination: true,
        pages: 1,
        size: 9999
      }
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.noForm.contractReviewList,data).then(res=>{
          if(res.isSuccess){
            var dataList = res?.data?.dataList
            console.log('dataList',dataList)
            if (dataList) {
              var list = this.contractList = dataList.filter(item=>{
                item.value = item.id,
                item.label = item.contractName
                return item.examineStatus == 1
              })
              resolve(list)
            } else {
              resolve([])
            }
          }
        })
      })
    },
    loadRelTreeList() { // 获取相关方列表
      this.$axios.post(
        Api.noForm.loadRelTreeList,
        {
          data: {
            projectId:localstorageGet('projectId')
          },
          pages:1,
          size:999
        },
        res => {
          if (res.isSuccess) {
            let dataList = res.data?.dataList || []
            var relativeList = this.relativeList = dataList.map(item=>{
              return {
                label:item.name,
                value:item.id
              }
            })

          }
        }
      );
    },
    confirm(){
      var total = 0
      console.log('this.multipleSelection',this.multipleSelection)
      this.multipleSelection.forEach(item=>{
        total += (item.settle-0)
        // total += (item.settlementMoney-0)
      })
      this.initForm.monthAmount = Math.round(total*100)/100
      this.closeDialog('detailVisible')
    },
    closeDialog(key){
      this[key] = false
    },
    setDisableData(config,data){
      config.forEach(row=>{
        row.forEach(el=>{
          if(el.children && el.children.length){
            this.setDisableData(el.children,data)
          }else{
            if(el.type != 'label'){
              if(this.operaType == 'preview'){
                el.disabled = true
              }else if(this.operaType == 'examine'){
                el.disabled = true
                let prop = el.prop
                if(data.indexOf(prop)>-1){
                  el.disabled = false
                  if(prop == 'amountReview'){
                    this.checkAmountReview = true
                    // var rules = [{ required: true, message: '请输入金额复核'}]
                    // this.assignValue(this.formMainConfig,'amountReview','rules',rules)
                    // console.log('this.formMainConfig',this.formMainConfig)
                    // console.log('this.formMainConfig[6][1]',)
                    // this.$set(this.formMainConfig[6][1].children[1][1].children[1][2],'rules',rules)
                    // this.$forceUpdate()
                  }
                }
              }
            }
          }
        })
      })
      return config
    },
    async processData(){
      let mainData = await this.$refs.mainForm.getData()
      if(!mainData)return false
      mainData.attachment = mainData.voucher = this.$refs.elupload.getFileId().join(',')
      mainData.examineStatus = '0'
      mainData.retentionMoney = mainData.retentionMoneyDetail
      mainData.companyCode = this.companyCode
      mainData.projectCode = this.projectCode
      if(this.operaType == 'create')delete mainData.id
      return mainData
    },
    async processSaveData(){
      var mainData = await this.processData()
      if(!mainData)return false
      if(this.checkAmountReview){
        if(!mainData.amountReview){
          this.$message.error('需要填写复核金额')
          return false
        }
      }
      var originData = deepClone(this.originData)
      for(let key in originData){
        if(mainData[key] !== undefined){
          originData[key] = mainData[key]
        }
      }
      originData.attachment = mainData.voucher//this.$refs.elupload.getFileId().join(',')
      originData.id = this.originData.id
      return originData
    },
  },
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
<style lang="scss" scoped>
 .attach-div{
  display: flex;
  align-items: center;
}
</style>
