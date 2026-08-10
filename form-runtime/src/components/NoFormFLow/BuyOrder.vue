<!-- 采购单 -->
<template>
  <div class="form-container" >
    <div>
      <div class="title">
        <h3>{{ formTitle }}</h3>
      </div>
      <el-row class="attach-div">
        <el-col :span="20">
          <div style="min-height:10px;">
            <!-- <el-button type="primary" plain @click ="chooseList" v-if="operaType != 'preview' && operaType != 'examine'">选择采购申请单</el-button> -->
            <el-dropdown trigger='click' @command="handleCommand" v-if="operaType != 'preview' && operaType != 'examine'"
              size="medium">
              <el-button type="primary">
                选择采购申请单<i class="el-icon-arrow-down el-icon--right"></i>
              </el-button>
              <el-dropdown-menu slot='dropdown'>
                <el-dropdown-item command='device'>设备</el-dropdown-item>
                <el-dropdown-item command='material'>材料</el-dropdown-item>
              </el-dropdown-menu>
            </el-dropdown>
          </div>
        </el-col>
        <!-- <el-col :span="2" :offset="2"><el-input placeholder="编号" v-model="orderCode" :disabled="operaType!='create'"></el-input></el-col> -->
        <el-col :span="3" :offset="1">
          <div class="col-div">
            编号：<el-input placeholder="编号" v-model="orderCode" maxlength="200" style="width: 60%;"
              :disabled="operaType != 'create' && operaType != 'edit'"></el-input>
          </div>
        </el-col>
      </el-row>
      <!-- 主要信息 -->
      <!-- <div style="width: 100%;text-align:center;"> -->
      <el-card>
        <div :style="{ 'min-width': formWidth }">
          <SimpleTable :tableConfig="formMainConfig" :tableData="mainTableData"></SimpleTable>
        </div>
        <!-- <CommonForm :formConfig="formMainConfig" :width="formWidth" ref="mainForm"></CommonForm> -->
        <el-row style="margin-top: 10px;" :style="{ 'width': formWidth }">
          <el-col :span="3">
            <div class="col-div">
              采购单位：<el-input placeholder="采购单位" v-model="purchaseCompany" :disabled="operaType != 'create' && operaType != 'edit'"
                style="width: 60%;"></el-input>
            </div>
          </el-col>
          <el-col :span="3" style="margin: 0 5px;">
            <div class="col-div">
              经办人：
              <el-select v-model="handledBy" filterable placeholder="请选择" :disabled="operaType != 'create' && operaType != 'edit'" style="width: 60%;">
                <el-option
                  v-for="item in personList"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
              <!-- <el-input placeholder="经办人" v-model="handledBy" :disabled="operaType != 'create' && operaType != 'edit'"
                style="width: 60%;"></el-input> -->
            </div>
          </el-col>
          <el-col :span="4">
            <div class="col-div">
              订货时间：
              <el-date-picker placeholder='订货时间' v-model="orderTime" type="date" format='yyyy-MM-dd'
                value-format='yyyy-MM-dd' style="width: 60%;" :disabled="operaType != 'create' && operaType != 'edit'">
              </el-date-picker>
            </div>
          </el-col>
        </el-row>
      </el-card>
      <!-- </div> -->
      <CommonFooter @submit="submit" @reSubmit="reSubmit" v-if="operaType == 'create' || operaType == 'edit'"
        :isReInitiate="isReInitiate"></CommonFooter>
      <Flow ref="flow" v-bind="$attrs"></Flow>

      <!-- 项目设备材料明细 -->
      <el-dialog title="材料采购申请单列表" :visible="listVisible" v-if="listVisible" :close-on-click-modal="false"
        :append-to-body="true" width="1124px" @close="closeDialog('listVisible')">
        <div v-show="step == 1">
          <SimpleTable :tableConfig="listConfig" :tableData="listTableData" ref="step1"
            :multipleSelection.sync="multipleSelection"></SimpleTable>
        </div>
        <div v-show="step == 2">
          <SimpleTable :tableConfig="listStep2Config" :tableData="listStep2Data" ref="step2"></SimpleTable>
        </div>
        <div slot="footer" class="dialog-footer">

          <el-button @click="closeDialog('listVisible')">取 消</el-button>
          <el-button type="primary" @click="nextStep" v-if="step == 1">下一步</el-button>
          <el-button type="primary" @click="prevStep" v-if="step == 2">上一步</el-button>
          <el-button type="primary" @click="confirm" v-if="step == 2">确 定</el-button>
        </div>
      </el-dialog>
      <!-- 项目设备材料明细end -->

      <!-- 材料设备明细详情 -->
      <el-dialog :title="title" :visible="detailVisible" v-if="detailVisible" :close-on-click-modal="false" :append-to-body="true"
        width="980px" @close="closeDialog('detailVisible')">
        <SimpleTable :tableConfig="listDetailConfig" :tableData="listDetailTableData"></SimpleTable>
        <div slot="footer" class="dialog-footer">
          <el-button @click="closeDialog('detailVisible')">关 闭</el-button>
        </div>
      </el-dialog>
      <!-- 材料设备明细详情end -->

      <!-- 分配合同 -->
      <el-dialog title="分配合同" :visible="contractVisible" v-if="contractVisible" :close-on-click-modal="false" :append-to-body="true"
        width="1180px" @close="closeContractDialog('contractVisible')">
        <div>
          <h4>
            采购总量{{ currentRow.applyAmount }} 已分配{{ hasAssignCount }}
          </h4>
        </div>
        <SimpleTable :tableConfig="contractConfig" :tableData="currentRow.contractData" style="margin: 10px 0;"
          ref='contractTable'></SimpleTable>
        <el-button type="primary" plain icon="el-icon-circle-plus" circle @click="addRow"></el-button>
        <div slot="footer" class="dialog-footer">
          <!-- <el-button @click="closeDialog('contractVisible')">取 消</el-button> -->
          <el-button @click="closeContractDialog('contractVisible')">取 消</el-button>
          <el-button type="primary" @click="confirmContractAssign">确 定</el-button>
        </div>
      </el-dialog>
      <!-- 分配合同end -->
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth'
import CommonFooter from './components/CommonFooter'
import Flow from './components/Flow'
import SimpleTable from './components/SimpleTable.vue';
import { formMainConfig, listConfig, listDetailConfig, listStep2Config, contractConfig, contractAssignTemp } from './config/BuyOrderConfig'
import { deepClone } from '@/utils'
import mixin from './mixin/mixin'
export default {
  name: 'buy_order',
  components: { CommonFooter, Flow, SimpleTable },
  props: ['operaType', 'isReInitiate', 'otherBizId', 'flowNodeProxyId'],
  mixins: [mixin],
  data() {

    return {
      formWidth: '1580px',
      formTitle: '设备/材料采购单',
      orderCode: '',
      formMainConfig: deepClone(formMainConfig),
      listVisible: false,
      listConfig: deepClone(listConfig),
      multipleSelection: [],
      step: 1,
      listTableData: [],
      detailVisible: false,
      listDetailConfig: deepClone(listDetailConfig),
      listDetailTableData: [],
      listStep2Config: deepClone(listStep2Config),
      listStep2Data: [],
      contractVisible: false,
      contractConfig: deepClone(contractConfig),
      contractData: [],
      currentRow: {},
      contractAssignTemp: deepClone(contractAssignTemp),
      postData: {},
      mainTableData: [],
      purchaseCompany: '',
      handledBy: '',//localstorageGet('userName'),
      orderTime: '',//now,
      personList:[]
      // hasAssignCount:0

    };
  },
  created() {
    this.listConfig.action.buttons[0].clickEvent = this.showListDetail
    this.listStep2Config.action.buttons[0].clickEvent = this.showAssignContract
    this.contractConfig.column[0].slot.changeEvent = this.chooseConstract
    this.contractConfig.action.buttons[0].clickEvent = this.deleteContractAssign
    this.getCompanyPersonnelTree(localstorageGet('companyId')).then(list=>{
      this.personList = list
    })
    if (this.operaType != 'preview') {
      this.handledBy = localstorageGet('userId')
      this.purchaseCompany = localstorageGet('companyName')
      var now = new Date().getFullYear() + '-' + (new Date().getMonth() + 1) + '-' + new Date().getDate()
      this.orderTime = now
    }
    if (this.otherBizId) {
      this.getBuyOrderById().then(res => {
        if (res.isSuccess) {
          this.insertDataToForm(res.data)
          if (this.operaType == 'examine') {
            this.getInputPermision().then(res => {
              let data = res || []
              this.setDisableData(data)
            })
          } else if (this.operaType == 'preview') {
            this.setDisableData([])
          }
        }
      })
    }
    // this.listConfig = listConfig
  },
  mounted() { },
  watch: {
  },
  computed: {
    title() {
      if (this.inventoryType == 'device') {
        return '设备采购申请单详情'
      } else {
        return '材料采购申请单详情'
      }
    },
    hasAssignCount() {
      var assign = 0
      if (this.currentRow.contractData && this.currentRow.contractData.length) {
        this.currentRow.contractData.forEach(item => {
          assign += (item.purchaseAmount - 0) || 0
        })
      }
      return assign
    }
  },
  methods: {
    handleCommand(val) {
      this.inventoryType = val
      this.chooseList(val)
      // this.showListChoose(val)
    },
    getBuyOrderById() {
      var data = {
        id: this.otherBizId
      }
      return this.$axios.post(Api.noForm.getBuyOrderById, { data })
    },
    chooseList(inventoryType) {
      this.getBuyOrderList(inventoryType).then(res => {
        if (res.isSuccess) {
          var list = res.data || []
          var listTableData = list.map(item => {
            var obj = { ...item }
            obj.contractName = item.project.name
            obj.contractNumber = item.project.code || '--'
            return obj
            // console.log('item',item)
            // return {
            //   id:item.id,
            //   code:item.code,
            //   projectAddress:item.projectAddress
            // }
          })
          this.listTableData = listTableData
          this.listVisible = true
        }
      })
    },
    getBuyOrderList(inventoryType) {
      var data = {
        inventoryType,
        project: {
          id: localstorageGet('projectId') || ''
        }
      }
      return this.$axios.post(Api.noForm.getBuyOrderList, { data })
    },
    closeDialog(key) {
      this[key] = false
      if (key == 'listVisible') this.step = 1
    },
    nextStep() {
      if (!this.multipleSelection.length) {
        return this.$message.error('请选择明细')
      } else {
        var ids = this.multipleSelection.map(item => {
          return item.id
        })
        this.generateStep2Data(ids).then(res => {
          if (res.isSuccess) {
            var list = res.data || []
            // console.log('list',list)
            var listStep2Data = list.map(item => {
              return {
                id: item.id,
                applyAmount: item.applyAmount,
                orderAmount: item.applyAmount,
                modelNumber: item.materialDeviceType.modelNumber,
                purchaseAmount:0, //已分配量
                materialDeviceTypeId: item.materialDeviceTypeId,
                unit: item.materialDeviceType.unit,
                name: item.materialDeviceType.name,
                remark: item.remark,
                materialDevice: item
              }
            })
            this.listStep2Data = listStep2Data
            this.step = 2
          }
        })
      }
    },
    generateStep2Data(ids) {
      var data = {
        ids
      }
      return this.$axios.post(Api.noForm.getMergeDemandTableByIds, { data })
    },
    prevStep() {
      this.step = 1
    },
    showListDetail({ row, index }) {
      this.listDetailTableData = row.demandItemList.map(item => {
        return {
          name: item.materialDeviceType.name,
          modelNumber: item.materialDeviceType.modelNumber,
          unit: item.materialDeviceType.unit,
          applyAmount: item.applyAmount,
          remark: item.remark
        }
      })
      this.detailVisible = true
    },
    showAssignContract(data) {
      var index = this.currentRowIndex = data.index
      // this.listStep2Data[index].contractData = []
      if (!this.listStep2Data[index].materialDevice.contractData) this.$set(this.listStep2Data[index].materialDevice, 'contractData', [])
      this.currentRow = deepClone(this.listStep2Data[index].materialDevice)
      if (!this.currentRow.contractData.length) this.currentRow.contractData.push(deepClone(this.contractAssignTemp))
      this.contractReviewList().then(list => {
        this.contractConfig.column[0].slot.options = list
        this.$nextTick(() => {
          this.contractVisible = true
        })
      })
    },
    contractReviewList() {
      var data = {
        project: {
          id: localstorageGet('projectId')
        },
        materialDeviceType: {
          id: this.currentRow.materialDeviceTypeId
        }
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.noForm.getMyInventory, { data }).then(res => {
          if (res.isSuccess) {
            var dataList = this.contractList = res?.data || []
            var list = dataList.map(item => {
              var contractReview = item.contractReview
              return {
                value: contractReview.id,
                label: contractReview.contractName
              }
            })
            resolve(list)
          }
        })
      })
    },
    chooseConstract(val, index) {
      var find = this.find = this.contractList.find(item => item.contractReview.id == val)
      // console.log('find',find)
      this.currentRow.contractData[index].secondParty = find.secondParty.name || ''
      this.currentRow.contractData[index].contractName = find.contractReview.contractName
      this.currentRow.contractData[index].contractReview = {
        id: find.contractReview.id,
        firstPartyId: find.contractReview.firstPartyId,
        secondPartyId: find.contractReview.secondPartyId
      }

      var inventoryItemList = find.inventoryItemList.filter(item => {
        return item.materialDeviceTypeId == this.currentRow.materialDeviceTypeId
      })
      var inventoryItem = inventoryItemList[0]
      this.currentRow.contractData[index].unitPriceIncludingTax = inventoryItem.unitPriceIncludingTax || ''
      this.currentRow.contractData[index].unitPriceNotIncludingTax = inventoryItem.unitPriceNotIncludingTax || ''
      this.currentRow.contractData[index].inventoryItemId = inventoryItem.id
    },
    getInfoByContractId(contractId, materialDeviceTypeId) {
      var data = {
        contractReview: {
          id: contractId
        },
        materialDeviceType: {
          id: materialDeviceTypeId
        }
      }
      return this.$axios.post(Api.noForm.getByContractId, { data })
    },
    addRow() {
      this.currentRow.contractData.push(deepClone(this.contractAssignTemp))
    },
    deleteContractAssign({ row, index }) {
      this.currentRow.contractData.splice(index, 1)
    },
    async confirmContractAssign() {
      if (this.currentRow.applyAmount != this.hasAssignCount) {
        this.$message.error('采购量与采购总量需要相等')
        return false
      }
      if (this.currentRow.contractData.length) {
        var ref = false
        ref = await this.$refs.contractTable.validateData()
        if (!ref) return false
        var purchaseAmount = 0
        this.currentRow.contractData.forEach(item=>{
          purchaseAmount+=(item.purchaseAmount-0)
        })
        this.listStep2Data[this.currentRowIndex].materialDevice.contractData = deepClone(this.currentRow.contractData)
        this.listStep2Data[this.currentRowIndex].purchaseAmount = purchaseAmount

        this.closeDialog('contractVisible')
      } else {
        this.$message.error('请给物料分配合同')
      }
    },
    closeContractDialog(key) {
      this.closeDialog(key)
      this.$nextTick(() => {
        this.currentRow.contractData = []
      })
    },
    confirm() {
      var ref = true
      for (let i = 0; this.listStep2Data[i]; i++) {
        if (!this.listStep2Data[i].materialDevice.contractData || !this.listStep2Data[i].materialDevice.contractData.length) {
          this.$message.error('有物料未分配合同')
          ref = false
          break;
        }
        // if(){
        //   this.$message.error('有物料未分配合同')
        //   ref = false
        //   break;
        // }
      }
      if (!ref) return false
      var data = deepClone(this.listStep2Data)
      this.genMainForm(data)
      this.closeDialog('listVisible')
    },
    insertDataToForm(data) {
      this.originData = data
      this.handledBy = data.handledBy.id
      this.purchaseCompany = data.purchaseCompany || localstorageGet('companyName')
      this.orderTime = data.orderTime.substr(0, 10)
      this.orderCode = data.orderCode
      var purchaseOrderItemList = data.purchaseOrderItemList
      var list = purchaseOrderItemList.map(item => {
        var totalPriceIncludingTax
        if (item.unitPriceIncludingTax) totalPriceIncludingTax = item.unitPriceIncludingTax * item.purchaseAmount
        return {
          id: item.id,
          arrivalTime: item.arrivalTime.substr(0, 10),
          unit: item.unit,
          brand: item.brand,
          purchaseAmount: item.purchaseAmount,
          qualityRequirement: item.qualityRequirement,
          remark: item.remark,
          taxRate: item.taxRate,
          unitPriceNotIncludingTax: item.unitPriceNotIncludingTax,
          unitPriceIncludingTax: item.unitPriceIncludingTax,
          materialDeviceTypeId: item.materialDeviceTypeId,
          deliveryAddress: item.deliveryAddress,
          totalPriceIncludingTax,
          contractName: item.contractReview.contractName,
          secondParty: item.company.name,
          name: item.materialDeviceType.name,
          modelNumber: item.materialDeviceType.modelNumber,
          contractReview: item.contractReview,
        }
      })
      this.mainTableData = list
    },
    genMainForm(data) {
      var list = []
      data.forEach(item => {
        var materialDevice = item.materialDevice || []
        var rowObj = {
          name: materialDevice.materialDeviceType.name,
          materialDeviceTypeId: materialDevice.materialDeviceType.id,
          modelNumber: materialDevice.materialDeviceType.modelNumber,
          qualityRequirement: '',
          unit: materialDevice.materialDeviceType.unit,
        }
        materialDevice.contractData.forEach(el => {
          // var find = this.contractList.find(it => it.value == el.contractId)
          // console.log('find',find)
          rowObj.purchaseAmount = el.purchaseAmount
          rowObj.contractName = el.contractName//find ? find.label : ''
          rowObj.secondParty = el.secondParty
          rowObj.arrivalTime = el.arrivalTime
          rowObj.deliveryAddress = el.deliveryAddress

          if (el.unitPriceIncludingTax) rowObj.unitPriceIncludingTax = el.unitPriceIncludingTax

          if (el.unitPriceNotIncludingTax) rowObj.unitPriceNotIncludingTax = el.unitPriceNotIncludingTax
          if (rowObj.unitPriceIncludingTax) rowObj.totalPriceIncludingTax = rowObj.unitPriceIncludingTax * rowObj.purchaseAmount
          // else rowObj.totalPriceIncludingTax = ''
          rowObj.taxRate = ''
          rowObj.brand = ''
          rowObj.remark = ''
          rowObj.contractReview = el.contractReview
          rowObj.inventoryItem = {
            id: el.inventoryItemId
          }
          list.push(deepClone(rowObj))
        })
        // materialDevice
      })
      this.mainTableData = list
    },
    setDisableData(data) {
      var formMainConfig = deepClone(this.formMainConfig)
      formMainConfig.column.forEach(row => {
        if (row.children && row.children.length) {
          row.children.forEach(item => {
            if (item.slot !== undefined) {
              var prop = item.prop
              item.slot.disabled = true
              if (data.indexOf(prop) > -1) item.slot.disabled = false
            }
          })
        } else {
          if (row.slot !== undefined) {
            var prop = row.prop
            row.slot.disabled = true
            if (data.indexOf(prop) > -1) row.slot.disabled = false
          }
        }
      })
      this.formMainConfig = formMainConfig
    },
    async processData() {
      var mainData = {
        orderCode: this.orderCode,
        handledBy: {
          id:this.handledBy
        },
        orderTime: this.orderTime + ' 00:00:00',
        purchaseCompany: this.purchaseCompany,
        purchaseOrderItemList: [],
        demandTableList: [],
        project: {
          id: localstorageGet('projectId')
        }
      }
      mainData.demandTableList = this.multipleSelection.map(item => {
        return {
          id: item.id
        }
      })
      var purchaseOrderItemList = this.mainTableData.map(item => {
        // console.log('item.contractReview',item)
        var totalPriceNotIncludingTax = null
        if (item.unitPriceNotIncludingTax) totalPriceNotIncludingTax = item.unitPriceNotIncludingTax * item.purchaseAmount

        var obj = {
          materialDeviceTypeId: item.materialDeviceTypeId,
          purchaseAmount: item.purchaseAmount,
          unit: item.unit,
          qualityRequirement: item.qualityRequirement,
          arrivalTime: item.arrivalTime + ' 00:00:00',
          deliveryAddress: item.deliveryAddress,
          unitPriceIncludingTax: item.unitPriceIncludingTax || null,
          unitPriceNotIncludingTax: item.unitPriceNotIncludingTax || null,
          totalPriceIncludingTax: item.totalPriceIncludingTax || null,
          totalPriceNotIncludingTax,
          taxRate: item.taxRate,
          brand: item.brand,
          remark: item.remark,

          contractReview: deepClone(item.contractReview),
          // inventoryItem//:deepClone(item.inventoryItem)
        }
        if (item.inventoryItem) {
          obj.inventoryItem = deepClone(item.inventoryItem)
        }
        if (item.contractReview) {
          obj.contractReview = deepClone(item.contractReview)
        }
        if (item.inventoryItem) {
          obj.inventoryItem = deepClone(item.inventoryItem)
        }
        if (this.operaType == 'edit' || this.operaType == 'examine') {
          obj.id = item.id
        }
        return obj
      })
      mainData.inventoryType = this.inventoryType || ''
      mainData.purchaseOrderItemList = purchaseOrderItemList
      return mainData
    },
    async processSaveData() {
      var mainData = await this.processData()
      mainData.id = this.originData.id
      mainData.inventoryType = this.originData.inventoryType || ''
      if (this.originData.demandTableList) mainData.demandTableList = this.originData.demandTableList
      return mainData
    }
  },
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
<style lang="scss" scoped>
::v-deep .el-card__body {
  overflow: auto;
}

</style>
