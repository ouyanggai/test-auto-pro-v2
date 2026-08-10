<!-- 采购需求申请 -->
<template>
  <div :style="{'width':formWidth}" class="form-container">
    <div class="title">
      <h3>{{ formTitle }}</h3>
    </div>
    <el-row class="attach-div">
      <el-col :span="20" >
        <div style="min-height:10px;">
          <!-- <el-button type="primary" plain @click="showListChoose" v-if="operaType!='preview'">{{ buttonTitle }}</el-button> -->
          <el-dropdown trigger='click' @command="handleCommand" v-if="operaType!='preview' && operaType != 'examine'" size="medium">
            <el-button type="primary">
              {{ buttonTitle }}<i class="el-icon-arrow-down el-icon--right"></i>
            </el-button>
            <el-dropdown-menu slot='dropdown'>
              <el-dropdown-item command='device'>设备</el-dropdown-item>
              <el-dropdown-item command='material'>材料</el-dropdown-item>
            </el-dropdown-menu>
          </el-dropdown>
        </div>
      </el-col>
      <!-- <el-col :span="4" ><el-input placeholder="编号" v-model="code"></el-input></el-col> -->
      <el-col :span="4">
        <div class="col-div">
          编号：<el-input placeholder="编号" v-model="code" maxlength="200" style="width: 60%;"></el-input>
        </div>
      </el-col>
    </el-row>
    <!-- 主要信息 -->
    <el-card>
      <CommonForm :initForm="initForm" :formConfig="formMainConfig" :width="formWidth"  ref="mainForm" :tableConfig="formTableConfig" :tableData="formTableData"></CommonForm>
    </el-card>
    <!-- 材料设备明细详情 -->
    <el-dialog :title="title" :visible="detailVisible" v-if="detailVisible" :close-on-click-modal="false" :append-to-body="true"
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
    <CommonFooter  @submit="submit" @reSubmit="reSubmit" v-if="operaType =='create' || operaType =='edit'"  :isReInitiate="isReInitiate"></CommonFooter>
    <Flow ref="flow" v-bind="$attrs"></Flow>

  </div>
</template>

<script>
import Api from '@/api';
import {localstorageGet} from '@/utils/auth'
import elupload from '@/components/EleUpload'
import Flow from './components/Flow'
import CommonForm from './components/CommonForm'
import {formMainConfig,listDetailConfig,tableConfig} from './config/BuyDemandConfig'
import CommonFooter from './components/CommonFooter'
import SimpleTable from './components/SimpleTable.vue';
import mixin from './mixin/mixin'
import {deepClone} from '@/utils'
export default {
  name:'buy_demand',
  components: {elupload,Flow,CommonForm,CommonFooter,SimpleTable},
  props: ['operaType','isReInitiate','otherBizId','flowNodeProxyId'],
  mixins:[mixin],
  data() {
    return {
      formWidth:'1080px',
      formTitle:'项目设备/材料采购需求申请表',
      formMainConfig:deepClone(formMainConfig),
      code:'',
      initForm:{},
      detailVisible:false,
      multipleSelection:[],
      listDetailConfig:deepClone(listDetailConfig),
      tableData:[{},{}],
      contractList:[],
      formTableConfig:deepClone(tableConfig),
      formTableData:[],
      inventoryType:''
    };
  },
  created() {
    this.initForm = this.initFormList(formMainConfig)
    this.contractReviewList().then(list=>{
      this.contractList = list
      this.assignValue(this.formMainConfig,'reviewId','options',list)
      this.assignValue(this.formMainConfig,'reviewId','changeEvent',this.contractChoose)
      if(this.otherBizId){
        if(this.operaType != 'edit' && this.operaType != 'create'){
          this.assignValue(this.formMainConfig,'reviewId','type','input')
          this.assignValue(this.formMainConfig,'reviewId','prop','contractName')
          this.assignValue(this.formMainConfig,'contractName','rules','')
        }
        this.getBuyDemandById().then(res=>{
          // console.log('res',res)
          if(res.isSuccess){
            this.originData = res.data
            this.insertDataToForm(res.data)
            if(this.operaType == 'examine'){
              this.getInputPermision().then(res=>{
                let data  = res || []
                this.setDisableData(data)
              })
            }else if(this.operaType == 'preview'){
              this.setDisableData([])
            }
          }
        })
      }else{
        this.getProjectData().then(res=>{
          if (res.isSuccess) {
            this.initForm.projectCode = res.data.code
          }
        })
        if(this.operaType == 'preview'){
          this.setDisableData([])
        }
      }
      // this.assignValue(this.formMainConfig,'selectList','clickEvent',this.showChooseList)
    })
    // this.formMainConfig[0][1].changeEvent = this.projectChange
  },
  mounted() {},
  watch: {},
  computed: {
    title(){
      if(this.inventoryType == 'device' ){
        return '项目设备明细'
      }else{
        return '项目材料明细'
      }
    },
    buttonTitle(){
      if(this.formTableData.length <=0 ){
        return '选择设备/材料明细'
      }else{
        return '选择设备/材料明细'
      }
    }
  },
  methods: {
    getProjectData() {
      return this.$axios.post(
        Api.myProject.getProjectDetail,
        {
          data: {
            id: this.$store.state.user.projectId
          }
        }
      );
    },
    handleCommand(val){
      this.inventoryType = val
      this.showListChoose(val)
    },
    insertDataToForm(data){
      var demandItemList = data.demandItemList || []
      var formTableData = demandItemList.map(item=>{
        return {
          // id:item.id,
          bizId:item.id,
          materialDeviceTypeId:item.materialDeviceTypeId,
          materialDeviceTypeStoreId:item.materialDeviceTypeId,
          name:item.materialDeviceType.name,
          modelNumber:item.materialDeviceType.modelNumber,
          applyAmount:item.applyAmount,
          unit:item.materialDeviceType.unit,
          remark:item.remark
        }
      })
      if(this.operaType != 'edit'){
        delete this.formTableConfig.column[3].slot
        delete this.formTableConfig.column[4].slot
      }
      this.formTableData = formTableData

      this.initForm.handledBy = data.handledBy
      this.initForm.handlingDepartment = data.handlingDepartment
      this.initForm.projectManager = data.projectManager
      this.initForm.projectAddress = data.projectAddress
      this.initForm.contractName = data.contractReview.contractName
      this.initForm.reviewId = data.contractReview.id
      this.initForm.contractNumber = data.contractReview.contractNumber
      this.initForm.firstParty = data.firstParty.name
      this.code = data.code
      this.inventoryType = data.inventoryType
      this.initForm.projectCode = data.project.code || ''
      this.initForm.remark = data.remark
      // this.initForm.reviewId = '36003992b2594353863dc125dfe5dc5b'
      // this.contractChoose(this.initForm.reviewId)

    },
    setDisableData(data){
      var formMainConfig = deepClone(this.formMainConfig)
      formMainConfig.forEach(row=>{
        this.checkDisable(row,data)
      })
      this.formMainConfig = formMainConfig
    },
    checkDisable(row,data){
      row.forEach(el=>{
        if(el.type != 'label'){
          if(this.operaType == 'preview'){
            el.disabled = true
          }else if(this.operaType == 'examine'){
            el.disabled = true
            let prop = el.prop
            if(data.indexOf(prop)>-1)el.disabled = false
          }
        }
      })
    },
    getBuyDemandById(){
      var data= {
        id:this.otherBizId
      }
      return this.$axios.post(Api.noForm.getBuyDemandById,{data})
    },
    contractReviewList(){
      var data = {
        data:{
          contractName:'',
          examineStatus:1,
          secondPartyId:localstorageGet('companyId'),
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
            var list = this.contractList = dataList.filter(item=>{
              item.value = item.id,
              item.label = item.contractName
              return item.contractType == 'server' || item.contractType == 'engineering'
            })
            resolve(list)
          }
        })
      })
    },
    contractChoose(val){
      var find = this.find = this.contractList.find(item=>item.value == val)
      if(find){
        this.initForm.contractNumber = find.contractNumber
        // this.initForm.projectCode = find.projectCode
        this.initForm.firstParty = find.firstParty
      }
    },
    showListChoose(inventoryType){
      this.getDeviceMaterialList(inventoryType).then(res=>{
        if(res.isSuccess){
          var list = res.data || []
          var tableData = list.map(item=>{
            return {
              id:item.id,
              name:item.materialDeviceTypeStore.name,
              modelNumber:item.materialDeviceTypeStore.modelNumber,
              contractAmount:item.contractAmount,
              unit:item.materialDeviceTypeStore.unit || '',
              materialDeviceTypeStoreId:item.materialDeviceTypeStore.id,
              remark:''
            }
          })
          this.tableData = tableData
          this.$nextTick(()=>{
            this.detailVisible = true
          })
        }
      })
    },
    getDeviceMaterialList(inventoryType){
      var data= {
        inventoryType,
        project:{
          id:localstorageGet('projectId')
        }
      }
      return this.$axios.post(Api.noForm.getDeviceMaterialList,{data})
    },
    confirm(){
      var formTableData = deepClone(this.multipleSelection)
      formTableData.forEach(item=>{
        item.applyNum = 0
      })
      this.formTableData = formTableData

      this.closeDialog('detailVisible')
    },
    closeDialog(key){
      this[key] = false
    },
    async processData(){
      let mainData = await this.$refs.mainForm.getData()
      if(!mainData)return false
      mainData.code = this.code
      if(this.operaType == 'create') delete mainData.id
      mainData.project = {
        id:localstorageGet('projectId') || ''
      }
      mainData.contractReview = this.find

      let r = await this.$refs.mainForm.$refs.simpleTable[0].validateData()
      if(!r)return false
      mainData.demandItemList = []
      if(!this.formTableData.length){
        this.$message.error('请选择设备/材料明细')
        return false
      }
      var arr = this.formTableData.map(item=>{
        var obj = {
          materialDeviceTypeId:item.materialDeviceTypeStoreId,//选中设备材料项中materialDeviceTypeStore的id
          applyAmount:item.applyAmount,//申请量
          unit:item.unit,//单位
          remark:item.remark || '',//备注
        }
        if(this.operaType == 'edit' || this.operaType == 'examine')if(item.bizId)obj.id = item.bizId //更新
        return obj
      })
      mainData.inventoryType = this.inventoryType || ''
      delete mainData.firstParty
      if(arr.length)mainData.demandItemList = arr
      return mainData
    },
    async processSaveData(){
      var mainData = await this.processData()
      mainData.id = this.originData.id
      mainData.inventoryType = this.inventoryType || ''
      delete mainData.contractReview
      delete mainData.contractName
      delete mainData.contractNumber
      delete mainData.firstParty
      // mainData.orderNotice = {
      //   id:this.originData.orderNotice.id
      // }
      return mainData
    }
  },
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
<style scoped>
  .el-table thead{
    color: #303133;
  }
</style>
