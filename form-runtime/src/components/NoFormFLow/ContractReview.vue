<!-- 合同评审表 -->
<template>
  <div :style="{'width':formWidth}" class="form-container">
    <div class="title">
      <h3>{{ formTitle }}</h3>
    </div>
    <el-row class="attach-div">
      <el-col :span="14" >
        <div style="min-height:10px;">
          <elupload ref="elupload"  title="上传" v-if="operaType == 'create'"></elupload>
          <elupload ref="elupload"  title="上传" v-else-if="operaType == 'preview'" :attachFile="attachFile" :showOnly="true"></elupload>
          <elupload ref="elupload"  title="上传" v-else-if="operaType == 'edit'" :attachFile="attachFile"></elupload>
          <elupload ref="elupload"  title="上传" v-else-if="operaType == 'examine'" :attachFile="attachFile" :showOnly="true"></elupload>
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
      <CommonForm :formConfig="formMainConfig" :initForm="initForm" :width="formWidth" ref="mainForm"></CommonForm>
    </el-card>
    <CommonFooter  @submit="submit" @reSubmit="reSubmit" v-if="operaType =='create' || operaType =='edit'" :isReInitiate="isReInitiate"></CommonFooter>
    <Flow ref="flow" v-bind="$attrs"></Flow>
  </div>
</template>

<script>
import Api from '@/api';
import elupload from '@/components/EleUpload'
import {deepClone,capitalMoney} from '@/utils'
import {localstorageGet} from '@/utils/auth'
import Flow from './components/Flow'
import CommonForm from './components/CommonForm'
import formMainConfig from './config/ContractReviewConfig'
import CommonFooter from './components/CommonFooter'
import mixin from './mixin/mixin'
export default {
  name:'contract_review',
  components: {elupload,Flow,CommonForm,CommonFooter},
  props: ['operaType','isReInitiate','otherBizId','flowNodeProxyId'],
  mixins:[mixin],
  data() {
    return {
      formWidth:'1080px',
      formTitle:'合同评审表',
      formMainConfig:deepClone(formMainConfig),
      companyCode:'',
      projectCode:'',
      attachFile:[],
      originData:{},
      initForm:{}
    };
  },
  created() {
    this.loadRelTreeList()
    this.initForm = this.initFormList(formMainConfig)
    this.assignValue(this.formMainConfig,'contractSum','inputEvent',this.capitalMoney)
    if(this.operaType == 'create'){
      this.initForm['handledBy'] = localstorageGet('userName')
      this.setPlanList()
      // this.getBuyPlanList().then(res=>{
      //   if(res.isSuccess){
      //     var dataList = res.data?.dataList || []
      //     var data = dataList.filter(item=>{
      //         return item.examineStatus == 1
      //       })
      //     var contractList = data.map(item=>{
      //       return {
      //         label:item.projectName,
      //         value:item.id,
      //       }
      //     })
      //     this.contractList = contractList
      //     this.initForm['handledBy'] = localstorageGet('userName')
      //     this.assignValue(this.formMainConfig,'contractNameId','options',contractList)
      //     this.assignValue(this.formMainConfig,'contractNameId','changeEvent',this.contractChoose)
      //   }else{
      //     this.$message.error('未找到可用合同')
      //   }
      // })
    }else{
      if(this.otherBizId){
        this.getContractReviewById().then(res=>{
          //根据数据回显表单
          if(res.isSuccess){
            if(this.operaType != 'edit'){
              this.formMainConfig[0][1].type = 'input'
              this.formMainConfig[0][1].prop = 'contractName'
            }else{
              this.setPlanList()
            }
            this.insertDataToForm(res.data)
            this.initForm['contractSumUp'] = capitalMoney(res.data.contractSum)
            this.initForm['contractName'] = res.data.contractName
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
          }else{
            this.$message.error(res.message)
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
  watch: {},
  computed: {},
  methods: {
    setPlanList(){
      this.getBuyPlanList().then(res=>{
        if(res.isSuccess){
          var dataList = res.data?.dataList || []
          var data = dataList.filter(item=>{
              return item.examineStatus == 1
            })
          var contractList = data.map(item=>{
            return {
              label:item.projectName,
              value:item.id,
            }
          })
          this.contractList = contractList
          this.assignValue(this.formMainConfig,'contractNameId','options',contractList)
          this.assignValue(this.formMainConfig,'contractNameId','changeEvent',this.contractChoose)
        }else{
          this.$message.error('未找到可用合同')
        }
      })
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
                if(data.indexOf(prop)>-1)el.disabled = false
              }
            }
          }
        })
      })
      return config
    },
    insertDataToForm(data){
      this.originData = data
      this.bizId = data.id
      var attachment = data.attachment
      if(attachment){
        this.getBatchFile(data.id).then(res=>{
          if(res.isSuccess){
            this.attachFile = res.data || []
          }
        })
      }
      this.insertFormConfigData(data)
    },
    insertFormConfigData(data){
      var initForm = deepClone(this.initForm)
      for(let key in initForm){
        initForm[key] = data[key]
      }
      this.initForm = initForm
      var contractNameId = ''
      if(data.procureSectionInfoVoList && data.procureSectionInfoVoList.length)contractNameId = data.procureSectionInfoVoList[0]?.procureId || ''
      else contractNameId = data.contractName
      this.$set(this.initForm,'contractNameId',contractNameId)
      this.companyCode = data.companyCode
      this.projectCode = data.projectCode
    },
    getContractReviewById(){
      var data = {
        id:this.otherBizId
      }
      return this.$axios.post(Api.noForm.getContractReviewById,{data})
    },
    getBuyPlanList(){
      var data = {
        data:{
          projectName:'',
          projectId:this.$store.state.user.projectId,
          examineStatus:'1'
        },
        pagination: true,
        pages: 1,
        size: 9999
      }
      return this.$axios.post(Api.noForm.getBuyPlanList,data)
    },
    contractChoose(val){
      this.procureId = val
      this.findBuyPlanById(val).then(res=>{
        if(res.isSuccess){
          var data = res.data || {}
          var procureCompanyId = data.procureCompanyId
          this.initForm['contractNameId'] = data.id
          this.initForm['firstPartyId'] = procureCompanyId
          this.companyCode = data.companyCode
          this.projectCode = data.projectCode
          if(data.procureSectionInfoVoList.length){
            var procureSectionInfo = data.procureSectionInfoVoList[0]
            var procureWay = procureSectionInfo.procureWay
            this.initForm['contractTalk'] = procureWay
          }
        }
      })
    },
    findBuyPlanById(id){
      var data = {id,projectId:this.$store.state.user.projectId}
      return this.$axios.post(Api.noForm.findBuyPlanById,{data})
    },
    // assignValue(arr,prop,key,val){
    //   arr.forEach(item=>{
    //     item.forEach(el=>{
    //       if(el.children){
    //         var children = el.children
    //         this.assignValue(children,prop,key,val)
    //       }else{
    //         if(el?.prop == prop){
    //           el[key] = val
    //         }
    //       }
    //     })
    //   })
    // },
    async processData(){
      let mainData = await this.$refs.mainForm.getData()
      if(!mainData)return false
      mainData.procureId = this.procureId
      var contractNameId = mainData.contractNameId
      if(this.operaType == 'create'){
        var find = this.contractList.find(item=>item.value == contractNameId)
        mainData.contractName = find.label
      }
      delete mainData.contractSumUp
      mainData.companyCode = this.companyCode
      mainData.projectCode = this.projectCode
      mainData.attachment = this.$refs.elupload.getFileId().join(',')
      mainData.examineStatus = '0'
      if(this.operaType == 'create')delete mainData.id
      return mainData
    },
    async processSaveData(){
      var mainData = await this.processData()
      if(!mainData)return false
      var originData = deepClone(this.originData)
      // console.log('originData',originData)
      for(let key in originData){
        if(mainData[key] !== undefined){
          originData[key] = mainData[key]
        }
      }
      // mainData.examineStatus = originData.examineStatus
      // mainData.status = originData.status
      originData.id = this.originData.id
      // console.log('originData',originData)
      // console.log('originData',this.originData)
      // return false
      return originData
    },
    capitalMoney(val){
      var numUp=''
      if(val !== ''){
        var numVal = val - 0
        if(numVal == numVal){ //非NAN
          numUp = capitalMoney(numVal)
        }
      }else{
        numUp = ''
      }
      this.initForm['contractSumUp'] = numUp
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
            this.assignValue(this.formMainConfig,'firstPartyId','options',relativeList)
            this.assignValue(this.formMainConfig,'secondPartyId','options',relativeList)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }
  },
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
