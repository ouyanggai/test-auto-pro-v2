<!-- 发货单 -->
<template>
  <div :style="{'width':formWidth}" class="form-container">
    <div class="title">
      <h3>{{ formTitle }}</h3>
    </div>
    <el-row class="attach-div">
      <el-col :span="20" ><el-button type="primary" plain @click ="chooseList" v-if="operaType == 'create' || operaType == 'edit'">选择发货通知单</el-button></el-col>
      <!-- <el-col :span="2" :offset="2"><el-input placeholder="编号" v-model="code"></el-input></el-col> -->
    </el-row>
    <!-- 主要信息 -->
    <el-card>
      <CommonForm :formConfig="formMainConfig" ref="mainForm" :initForm="initForm"></CommonForm>
      <SimpleTable :tableConfig="mainTableConfig" :tableData="mainTableData" ref="mainTable"></SimpleTable>
      <!-- <CommonForm :formConfig="addressFormConfig" :borderTop="'none'" ref="addressForm"></CommonForm> -->

    </el-card>
    <CommonFooter v-if="operaType =='create' || operaType =='edit'"   @submit="submit" @reSubmit="reSubmit" :isReInitiate="isReInitiate"></CommonFooter>
    <Flow ref="flow" v-bind="$attrs"></Flow>
    <!-- 发货单列表 -->
    <el-dialog title="发货单" :visible="listVisible" v-if="listVisible" :close-on-click-modal="false" :append-to-body="true"
      width="1124px" @close="closeDialog('listVisible')">
      <div v-show="step == 1">
        <SimpleTable :tableConfig="listConfig" :tableData="listConfigData" ref="step1"></SimpleTable>
      </div>
      <div v-show="step == 2">
        <SimpleTable :tableConfig="listStep2Config" :tableData="listStep2Data" ref="step2" :multipleSelection.sync="multipleSelection"></SimpleTable>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="closeDialog('listVisible')" >取 消</el-button>
        <el-button type="primary" @click="nextStep" v-if="step == 1">下一步</el-button>
        <el-button type="primary" @click="prevStep" v-if="step == 2">上一步</el-button>
        <el-button type="primary" @click="confirm" v-if="step == 2">确 定</el-button>
      </div>
    </el-dialog>
    <!-- 项目设备材料明细end -->


    <!-- 分配合同 -->
    <!-- <el-dialog title="分配合同" :visible="contractVisible" v-if="contractVisible" :close-on-click-modal="false" :append-to-body="true"
      width="880px" @close="closeDialog('contractVisible')">
        <SimpleTable :tableConfig="contractConfig" :tableData="[{},{}]"></SimpleTable>
        <el-button type="primary" icon="el-icon-circle-plus" circle size=""></el-button>
      <div slot="footer" class="dialog-footer">
        <el-button @click="closeDialog('contractVisible')">取 消</el-button>
        <el-button type="primary">确 定</el-button>
      </div>
    </el-dialog> -->
    <!-- 分配合同end -->

  </div>
</template>

<script>
import Api from '@/api';
import {localstorageGet} from '@/utils/auth'
import CommonFooter from './components/CommonFooter'
import Flow from './components/Flow'
import SimpleTable from './components/SimpleTable.vue';
import CommonForm from './components/CommonForm'
import {formMainConfig,mainTableConfig,addressFormConfig,listConfig,listStep2Config} from './config/InvoiceConfig'
import mixin from './mixin/mixin'
import {deepClone} from '@/utils'
export default {
  name:'invoice',
  components: {CommonFooter,Flow,SimpleTable,CommonForm},
  props: ['operaType','isReInitiate','otherBizId','flowNodeProxyId'],
  mixins:[mixin],
  data() {
    return {
      formWidth:'1080px',
      formTitle:'发货单',
      initForm:{},
      formMainConfig:deepClone(formMainConfig),
      mainTableConfig:deepClone(mainTableConfig),
      mainTableData:[],
      listVisible:false,
      // contractVisible:false,
      detailVisible:false,
      addressFormConfig,
      step:1,
      listConfig:deepClone(listConfig),
      listConfigData:[],
      listStep2Config:deepClone(listStep2Config),
      listStep2Data:[],
      // listDetailConfig,
      multipleSelection:[],
      // handledBy:'',
      // telephoneNumber:'',
    };
  },
  created() {
    this.initForm = this.initFormList(formMainConfig)
    // console.log('this.initForm',this.initForm)
    // this.contractReviewList().then(res=>{
      // var list = this.contractList = res
      // this.assignValue(this.formMainConfig,'reviewId','options',list)
      // this.assignValue(this.formMainConfig,'reviewId','changeEvent',this.contractChoose)
    // })
    this.listStep2Config.column[4].slot.inputEvent = this.inputEvent
    if(this.otherBizId){
      this.getInvoiceById().then(res=>{
        if(res.isSuccess){
          this.originData = res.data
          this.insertDataToForm(res.data)
          // console.log('this.operaType',this.operaType)
          if(this.operaType == 'examine'){
            this.getInputPermision().then(res=>{
              let data  = res || []
              this.setDisableData(this.formMainConfig,data)
              this.mainTableConfig.column[4].slot.disabled = false
            })
          }else if(this.operaType == 'preview'){
            this.setDisableData(this.formMainConfig,[])
          }
        }
      })
    }else{
      this.initForm.projectName = localstorageGet('projectName')
      if(this.operaType == 'preview'){
        this.setDisableData(this.formMainConfig,[])
      }
    }
  },
  mounted() {},
  watch: {
  },
  computed: {},
  methods: {
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
      this.initForm.projectName = data.project.name
      this.initForm.handledBy = data.director.id
      this.initForm.telephoneNumber = data.telephoneNumber
      var orderReceiptItemList = data.orderReceiptItemList
      this.initForm.firstParty = data.firstParty.name
      var companyId = data.firstParty.id
      this.getCompanyPersonnelTree(companyId).then(list=>{
        this.assignValue(this.formMainConfig,'handledBy','options',list)
      })
      var mainTableData = orderReceiptItemList.map(item=>{
        var obj = {
          orderItemId:item.orderItemId,
          id:item.id,
          name:item.orderItem.materialDeviceType.name,
          modelNumber:item.orderItem.materialDeviceType.modelNumber,
          unit:item.orderItem.unit,
          purchaseAmount:item.purchaseAmount,
          arrivalAmount:item.arrivalAmount,
          reviewAmount:item.reviewAmount,
          brand:item.orderItem.brand,
          remark:item.orderItem.remark
        }
        return obj
      })
      this.mainTableData = mainTableData
      //materialDeviceType

    },
    getInvoiceById(){
      var data= {
        id:this.otherBizId
      }
      return this.$axios.post(Api.noForm.getInvoiceById,{data})
    },
    inputEvent(val,index){
      this.listStep2Data[index].leftAmount = this.listStep2Data[index].purchaseAmount - val
    },
    chooseList(){
      this.getInvoicNoticeList().then(res=>{
        if(res.isSuccess){
          var data = res?.data || []
          var listConfigData = data.map(item=>{
            return {
              id:item.id,
              handledBy:item.handledBy,
              orderCode:item.orderCode,
              purchaseCompany:item.purchaseCompany,
              projectName:item.project.name,
              purchaseCompany:item.firstParty.name,
              firstParty:item.firstParty.name,
              companyId:item.firstParty.id
            }
          })
          this.listConfigData = listConfigData
          this.listVisible = true
        }
      })
    },
    getInvoicNoticeList(){
      var data = {
        project:{
          id:localstorageGet('projectId')
        }
      }
      return this.$axios.post(Api.noForm.getInvoicNoticeList,{data})
    },
    closeDialog(key){
      this[key] = false
      if(key == 'listVisible')this.step = 1
    },
    nextStep(){
      if(!this.$refs.step1.radio){
        return this.$message.error('请选择一条发货通知单')
      }
      var selected = this.selectedRadio = this.$refs.step1.radio
      var find = this.listConfigData.find(item=>item.id == selected)
      if(find)this.orderCode = find.orderCode
      if(!selected){
        return this.$message.error('请选择发货单')
      }
      this.getInvoicNoticeById(selected).then(res=>{
        if(res.isSuccess){
          var data = res.data
          var orderItemList = data.orderItemList

          var listStep2Data = orderItemList.map(item=>{
            var obj = {
                      orderItemId:item.id,
                      name:item.materialDeviceType.name,
                      modelNumber:item.materialDeviceType.modelNumber,
                      purchaseAmount:item.purchaseAmount,
                      arrivalAmount:item.arrivalAmount || '',
                      unit:item.materialDeviceType.unit,
                      brand:item.brand,
                      remark:item.remark
                    }
            var arrivalAmount = obj.arrivalAmount ? obj.arrivalAmount : 0
            obj.leftAmount = obj.purchaseAmount - arrivalAmount
            obj.leftAmount = obj.leftAmount < 0 ? 0 : obj.leftAmount
            return obj
          })
          this.listStep2Data = listStep2Data
          this.step = 2
        }
      })
      // console.log('selected',selected)

    },
    getInvoicNoticeById(id){
      var data = {
        id
      }
      return this.$axios.post(Api.noForm.getInvoicNoticeById,{data})
    },
    prevStep(){
      this.step = 1
    },
    showListDetail(){
      this.detailVisible = true
    },
    async confirm(){
      var ref = true
      // ref = await this.$refs.step2.validateData()
      // if(!ref)return false
      if(!this.multipleSelection.length){
        return this.$message.error('请至少选择一条发货通知单')
      }
      for(let i=0;this.multipleSelection[i];i++){
        if(!this.multipleSelection[i].arrivalAmount){
          this.$message.error('发货量为必填')
          ref = false
        }
      }
      if(!ref)return false
      this.multipleSelection.forEach(item=>{
        item.arrivalAmount = item.arrivalAmount-0
      })
      // console.log('this.multipleSelection',this.multipleSelection)
      this.mainTableData = deepClone(this.multipleSelection)
      var find = this.listConfigData.find(item=>item.id == this.selectedRadio)
      if(find && find.companyId){
        this.getCompanyPersonnelTree(find.companyId).then(list=>{
            this.assignValue(this.formMainConfig,'handledBy','options',list)
        })
      }
      this.initForm.firstParty = find.firstParty
      this.closeDialog('listVisible')
    },


    async processData(){
      if(!this.mainTableData.length){
        this.$message.error('请先选择发货通知单')
        return false
      }
      var mainData = await this.$refs.mainForm.getData()
      if(!mainData)return false
      let r = await this.$refs.mainTable.validateData()
      if(!r)return false
      mainData.orderReceiptItemList = this.mainTableData.map(item=>{
        var obj = {
          orderItemId:item.orderItemId,
          reviewAmount:item.reviewAmount,
          arrivalAmount:item.arrivalAmount
        }
        if(this.operaType == 'edit' || this.operaType == 'examine')obj.id = item.id
        return obj
      })
      mainData.orderNotice = {
        id:this.selectedRadio
      }
      mainData.orderCode = this.orderCode || ''
      mainData.director = {
        id:mainData.handledBy
      }
      delete mainData.firstParty
      delete mainData.handledBy
      if(this.operaType == 'create') delete mainData.id
      return mainData
    },
    async processSaveData(){

      var mainData = await this.processData()
      mainData.id = this.originData.id
      mainData.orderCode = this.originData.orderCode
      mainData.orderNotice = {
        id:this.originData.orderNotice.id
      }
      return mainData
    }
  },
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
<style scoped>
::v-deep .el-card__body .el-table th.el-table__cell.is-leaf,
::v-deep .el-card__body .el-table td.el-table__cell,
::v-deep .el-card__body .el-table--border,
::v-deep .el-card__body .el-table::before, ::v-deep .el-card__body .el-table--group::after, ::v-deep .el-card__body .el-table--border::after
{
  border-color: #999 !important;
}
::v-deep .el-card__body .el-table--border{
  border-top: transparent;
}
::v-deep .el-card__body .el-table--border::after,::v-deep .el-card__body .el-table--border::before{
  background: #999;
}
::v-deep .child-row.el-row{
  border: none;
}
</style>
