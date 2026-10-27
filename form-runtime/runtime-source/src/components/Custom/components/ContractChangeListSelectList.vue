<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-09-20 10:04:22
-->
<!--
 * @Descripttion: 流程列表选择弹窗
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible" :title="'选择'+typeList[fieldType]['name']" width="80%" top="100px" fullscreen :close-on-click-modal="false"
     :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
     <div>
        <el-input v-if="JSON.parse(myValue)&&JSON.parse(myValue).flowType=='government_dock_need'" clearable style="width:180px;margin-right: 10px;" v-model.trim="searchForm.name" placeholder="查询标题名称"></el-input>
        <el-input v-else style="width:180px;margin-right: 10px;" v-model.trim="searchForm.name" :placeholder="'查询'+typeList[fieldType]['name']+'名称'"></el-input>
        <el-input v-if="fieldType == 'contract'" style="width:180px;margin-right: 10px;" v-model.trim="searchForm.number" :placeholder="'查询'+typeList[fieldType]['name']+'编号'"></el-input>
        <el-input v-if="fieldType == 'contract'" style="width:180px;margin-right: 10px;" v-model.trim="searchForm.contractBody" :placeholder="'查询'+typeList[fieldType]['name']+'主体'"></el-input>

        <el-button type="primary" @click="searchList">查询</el-button>

        <div v-if="fieldType == 'contract'">
          <dy-table
            ref="contract"
            :height="'50vh'"
            :fetchData="getList"
            :keys="colKey1"
            :actions="actions1"
            :list="myFlowList"
            :isPagination="true"
            :pagination="pagination"
            @rowClick="isRowClick"
            ></dy-table>
        </div>
        <div v-if="fieldType == 'flow'">
          <!-- :height="'50vh'" -->
          <dy-table
            ref="flow"
            :height="'50vh'"
            :fetchData="getList"
            :keys="colKey2"
            :actions="actions2"
            :list="myFlowList"
            :isPagination="true"
            :pagination="pagination"
            @rowClick="isRowClick"
          ></dy-table>
        </div>
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>

    <!-- 流程-查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId"
    :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId"></EnterpriseExamineDialog>

    <!-- 查看流程 -->
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
    :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId"></CheckFlowNodeDetail>

    <el-dialog
      title="合同版本记录"
      :visible.sync="versionDialogVisible"
      width="620px"
      append-to-body
      class="contract-version-dialog"
    >
      <div v-if="versionRecords.length" class="contract-version-list">
        <div
          v-for="record in versionRecords"
          :key="record.__key"
          class="contract-version-item"
          @click="openVersionRecord(record)"
        >
          <span>{{ record.effectiveDate || record.createDate || '-' }}</span>
          <span>{{ record.changeTypeName || '补充协议' }}</span>
          <span>{{ record.agreementNumber || '-' }}</span>
          <i class="el-icon-arrow-right"></i>
        </div>
      </div>
      <div v-else class="contract-version-empty">暂无生效补充协议变更记录</div>
    </el-dialog>


  </div>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import mixin from './GeneralListSelectShow/mixin.js';
import { approveManageFlowStatus, deepClone,getObjById } from '@/utils';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import DyTable from '@/components/DyTable';
import { localstorageSet,localstorageGet } from '@/utils/auth';

export default {
  name: 'ContractChangeListSelectList',
  components: {DyTable,EnterpriseExamineDialog,CheckFlowNodeDetail},
  model: {
    prop: 'myValue', // value
    // event: 'changeMyValue' // input
  },
  mixins:[mixin],
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    myValue: { // value
      type: [String], // [Array, String, Number]
      default() {
        return '';
      }
    },
    fieldSelectType: {
      type: String,
      default: ''
    },
    allowTerminatedContract: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      // 通用数据
      viewName:'',
      rowData:{},
      typeList: {
        'contract': {
          name:'合同',
        },
        'flow': {
          name:'流程',
        },
      },
      myFlowList: [],
      searchForm:{
        name:'',
        number:'',
        contractBody:'',
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      // 合同数据
      colKey1: {
        contractName: {
          label: '合同名称',
          showTooltip: true,
          minWidth: '220',
          handle: (scope, createElement) => this.renderContractName(scope, createElement)
        },
        contractNumber: {
          label: '合同编号',
          showTooltip: true,
          minWidth: '180',
        },
        currencyName: {
          label: '币种',
          minWidth: '90',
          align: 'center',
          handle: (scope, createElement) => createElement('span', this.currencyName(scope.row))
        },
        latestOriginalAmount: {
          label: '合同金额（原币）',
          minWidth: '140',
          align: 'right',
          handle: (scope, createElement) => createElement('span', this.amountText(scope.row, 'original'))
        },
        latestContractSum: {
          label: '合同金额（人民币）',
          minWidth: '150',
          align: 'right',
          handle: (scope, createElement) => createElement('span', this.amountText(scope.row, 'rmb'))
        },
        newContractBody: {
          label: '合同主体',
          showTooltip: true,
          minWidth: '180',
        },
        contractProjectName: {
          label: '合同项目名称',
          showTooltip: true,
          minWidth: '180',
        },
        // cumulativePaymentMoney: {
        //   label:'累计付款金额(元)',
        //   handle:(scope,createElement)=>{
        //     let result = scope.row?.contractLedgerDataVo?.cumulativePaymentMoney || 0
        //     return <span>{result}</span>
        //   }
        // },
      },
      actions1: [
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.showContractDetail(row);
          }
        },
      ],
      // 流程数据
      colKey2: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '280',
        },
        flowName: {
          label: '流程名称',
          showTooltip: true,
          minWidth: '150',
        },
        createDate: {
          label: '发起时间',
          minWidth: '160',
          showTooltip: true
        },
        status: {
          label: '流程状态',
          minWidth: '100',
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.status));
          }
        },
      },
      actions2: [
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.previewHandle(row, false);
          }
        },
      ],
      contractBodyList:[],
      versionDialogVisible: false,
      versionRecords: []
    };
  },
  computed: {
    fieldType(){
      console.log('this.fieldSelectType',this.fieldSelectType)
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
      console.log('copyType',copyType)
      var str = '';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        if (result) {
          str = item;
          break;
        }
      }
      console.log('str',str)
      return str
    }
  },
  watch: {
    visible(val){
      if (val) {
        console.log('visible-流程列表',val)
        console.log(123,JSON.parse(this.myValue))
        this.$nextTick(async x=>{
          console.log('2222111',this.$refs[this.fieldType])
          if (this.$refs[this.fieldType]){
            this.$refs[this.fieldType].doLayout();
          }

          if (JSON.parse(this.myValue) && JSON.parse(this.myValue).formType == 'contract_receipt_form') { // 收款登记表特殊处理
            this.myFlowList = await this.getList();
            if (this.fieldType == 'contract'){
              this.myFlowList.forEach(item=>{
                // 处理合同主体数据转换
                this.dealBodyList(item);
              })
              console.log('this.myFlowList111',this.myFlowList)
            }
          }
        })
      }
    },
    async myValue(newVal, oldVal) {
      console.log('w====atch',newVal)
      let getMyNewVal = newVal == '' ? newVal : JSON.parse(newVal);
      
      if (!this.myFlowList.length) {
        this.myFlowList = await this.getList();
        if (this.fieldType == 'contract'){
          this.myFlowList.forEach(item=>{
            // 处理合同主体数据转换
            this.dealBodyList(item);
          })
          console.log('this.myFlowList111',this.myFlowList)
        }
      }
      console.log('this.myFlowList',this.myFlowList)
      console.log('getMyNewVal',(getMyNewVal?.id || getMyNewVal))
      let obj = this.myFlowList.find(x=>x.id == (getMyNewVal?.id || getMyNewVal));
      // if (this.fieldType == 'flow') {
      //   obj = this.myFlowList.find(x=>x.flowProxyId == (getMyNewVal?.id || getMyNewVal));
      // } else {
      //   obj = this.myFlowList.find(x=>x.id == (getMyNewVal?.id || getMyNewVal));
      // }
      console.log('obj-watch',obj)
      console.log('obj-watch',obj)
      this.viewName = obj?.name||obj?.contractName || '';
      // console.log('this.viewName-watch',this.viewName)
      this.rowData = obj;
    },
    "pagination.pages": async function(newVal, oldVal){
      this.myFlowList = await this.getList();
      this.rowData = {};
      if (this.fieldType == 'contract'){
        this.myFlowList.forEach(item=>{
          // 处理合同主体数据转换
          this.dealBodyList(item);
        })
      }
    },
    "pagination.size": async function(newVal, oldVal){
      this.myFlowList = await this.getList();
      this.rowData = {};
      if (this.fieldType == 'contract'){
        this.myFlowList.forEach(item=>{
          // 处理合同主体数据转换
          this.dealBodyList(item);
        })
      }
    }
  },
  created() {
    this.getContractBodyList();
  },
  mounted() {
    this.init();
  },
  methods: {
    isTerminated(row) {
      const status = row && (row.lifecycleStatus || row.terminationStatus || row.contractStatus || row.status);
      return String(status || '').toUpperCase() === 'TERMINATED' || String(status || '') === '已终止';
    },
    currencyName(row) {
      const names = { CNY: '人民币', USD: '美元', HKD: '港币', EUR: '欧元', GBP: '英镑', JPY: '日元', KRW: '韩元' };
      const code = String(row && (row.currencyCode || row.currency || '') || '').toUpperCase();
      return row && (row.currencyName || row.currencyLabel) || names[code] || code || '-';
    },
    amountValue(row, kind) {
      const keys = kind === 'original'
        ? ['latestOriginalAmount', 'effectiveOriginalAmount', 'currentOriginalAmount', 'originalAmount']
        : ['latestContractSum', 'effectiveContractSum', 'currentContractSum', 'contractSum'];
      for (let index = 0; index < keys.length; index += 1) {
        const value = row && row[keys[index]];
        if (value !== undefined && value !== null && value !== '') return value;
      }
      return '';
    },
    amountText(row, kind) {
      const value = this.amountValue(row, kind);
      return value === '' ? '-' : value;
    },
    renderContractName(scope, createElement) {
      const children = [createElement('span', scope.row.contractName || '-')];
      if (this.isTerminated(scope.row)) {
        children.push(createElement('el-tag', {
          props: { type: 'danger', size: 'mini' },
          style: { marginLeft: '6px' }
        }, ['已终止']));
      }
      return createElement('span', children);
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    async init() {
      console.log('init')
      if(!this.fieldSelectType) return;
      let getMyNewVal = this.myValue == '' ? this.myValue : JSON.parse(this.myValue);
      if (!this.myFlowList.length) {
        this.myFlowList = await this.getList();
        console.log('this.myFlowList2',this.myFlowList)
        if (this.fieldType == 'contract'){
          this.myFlowList.forEach(item=>{
            // 处理合同主体数据转换
            this.dealBodyList(item);
          })
        }
      }
      // console.log('this.myFlowList',this.myFlowList)
      // console.log('getMyNewVal',getMyNewVal)
      // let obj = this.myFlowList.find(x=>x.id == (getMyNewVal?.id || getMyNewVal));
      let obj = this.myFlowList.find(x=>x.id == (getMyNewVal?.id || getMyNewVal));
      // if (this.fieldType == 'flow') {
      //   obj = this.myFlowList.find(x=>x.flowProxyId == (getMyNewVal?.id || getMyNewVal));
      // } else {
      //   obj = this.myFlowList.find(x=>x.id == (getMyNewVal?.id || getMyNewVal));
      // }
      // console.log('obj',obj)
      if (obj) {
        this.viewName = obj?.name||obj?.contractName || '';
      } else {
        this.viewName = getMyNewVal.name;
      }

      this.rowData = obj;
    },
    // 处理合同主体数据转换
    dealBodyList(item){
      console.log('dealBodyList')
      let bodyStr = '';
      if (item.contractBody){
        let contractBodyListCopy = JSON.parse(JSON.stringify(this.contractBodyList));
        let contractBodyArrByDou = item.contractBody.split(',');
        for(var i = 0;i<contractBodyArrByDou.length;i++) {
          let contractBodyArrByFen = contractBodyArrByDou[i].split(':');
          let firstBody = getObjById(contractBodyListCopy,contractBodyArrByFen[0],'dictDataVos','dictValue')
          if (firstBody){
            bodyStr+=` ${firstBody.dictLabel}:${contractBodyArrByFen[1]},`
          }
        }
      }

      let newBodyStr = bodyStr.slice(0,-1);
      if (newBodyStr == '') {
        this.$set(item,'newContractBody',item.contractBody) // 原来的合同主体（兼容原来合同主体不在数据字典里）；同时兼容相关方的主体
      } else {
        this.$set(item,'newContractBody',newBodyStr) // 新组装的合同主体
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
    async searchList(){
      console.log('searchList')
      this.pagination.pages = 1;
      this.myFlowList = await this.getList();
      if (this.fieldType == 'contract'){
        this.myFlowList.forEach(item=>{
          // 处理合同主体数据转换
          this.dealBodyList(item);
        })
      }
      // console.log('await this.getList()',await this.getList())
    },
    getList() { // 获取列表数据
      console.log('getList************',JSON.parse(this.myValue),this)
      // console.log('getList2',this.myValue)
      // console.log('getList22',typeof this.myValue)
      // console.log('this.fieldType',this.fieldType)
      return new Promise(async (resolve,reject)=>{
        let url='',data = {};
        // initiatorCompanyId = ''
        let selectCompanyId = this.myValue != '' && JSON.parse(this.myValue).selectCompanyId ? JSON.parse(this.myValue).selectCompanyId : '';
        let initiatorCompanyId = this.myValue != '' && JSON.parse(this.myValue).initiatorCompanyId ? JSON.parse(this.myValue).initiatorCompanyId : this.$store.state.user.companyId;
        let initiatorId = this.myValue != '' && JSON.parse(this.myValue).initiatorId ? JSON.parse(this.myValue).initiatorId : '';
        let fcqParameterList = []
        if (this.fieldType == 'contract') { // 合同(没有传paymentType是查已通过盖章评审的合同，传了paymentType查收款和付款合同，并且已通过盖章评审)
          url = Api.contractManage.contractInfo.getContractList;
          data = {
            contractName: this.searchForm.name,
            contractNumber: this.searchForm.number,
            contractBody: this.searchForm.contractBody,
            // userId:this.$store.state.user.userId,
            // companyId: initiatorCompanyId,
            // companyId: selectCompanyId,
            // companyId: selectCompanyId,
            status:'1',
            fileStatus:'1', // 盖章版文件是否上传0否1是
            examineStatus:'1',
            contractSubtableVo:{
              stampStatus:"1",
              stampExamineStatus:"1",
            },
          }
          if (initiatorId) {
            let depIdList = await this.getInitiatorDepId(initiatorCompanyId);
            console.log('depIdList',depIdList)
            data.depList = depIdList;
            // data.depId = initiatorDepartmentId;
          } else if (selectCompanyId) {
            data.companyId = selectCompanyId;
          } else if (selectCompanyId) {
            data.companyId = selectCompanyId;
          } else {
            data.companyId = initiatorCompanyId;
          }
          if (JSON.parse(this.myValue).paymentType){
            data.contractSubtableVo.paymentType = JSON.parse(this.myValue).paymentType; // 收付款分类 0收 1付 2其它 3收付款合同（后端代码默认叠加3，就是选1就是1，3）
          }
        } else if  (this.fieldType == 'flow'){ // 流程
          let flowType = JSON.parse(this.myValue).flowType;
          // console.log('获取流程列表',flowType)
          url = Api.schedule.getFlowInstanceList;
          data = {
            flowName: this.searchForm.name,
            name: '',
            useScope: 'invest',
            auditWayList: [flowType],
            statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'company',
                otherBizId: this.$store.state.user.companyId
                // otherBizId: initiatorCompanyId
              }
            ],
            initiator: "all",
          };
          const userId = JSON.parse(this.myValue).userId
          if (userId) {
            if(flowType&&flowType=='government_dock_need'){
              // 政务对接需求统计流程移除这个请求中fcqParameterList数据
              fcqParameterList = [];
              data.flowName = '';
              data.name = this.searchForm.name
            } else {
              data.flowName = this.searchForm.name;
              data.name = '';
              fcqParameterList = [{
                condition:'like',
                name:'data.user_id',
                type:'string',
                value:userId
              }]
            }
          }else{
            fcqParameterList = [];
            if(flowType&&flowType=='government_dock_need'){
              // 政务对接需求统计流程移除这个请求中fcqParameterList数据
              data.flowName = '';
              data.name = this.searchForm.name
            } else {
              data.flowName = this.searchForm.name;
              data.name = '';
            }
          }
        }
        this.$axios.post(
          url,
          {
            data,
            fcqParameterList,
            pagination: true,
            pages: this.pagination.pages,
            size: this.pagination.size
          },
          res => {
            if (res.isSuccess) {
              if (this.fieldType == 'contract'){
                this.pagination.total = res.data.total || 0;
                resolve(res.data.dataList || [])
              } else if (this.fieldType == 'flow'){
                this.pagination.total = res.total || 0;
                resolve(res.data || [])
              }
            } else {
              this.$message.error(res.message);
            }
          }
        );

      })
    },
    getInitiatorDepId(initiatorCompanyId){
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          "/web/user/api/user/getUserInfoById",
          {
            data: {
              id:localstorageGet('userId'),
              flag:'company',
            },
          },
          (res) => {
            if (res.isSuccess) {
              let departArr = res.data.deptDutyVos.filter(x=> x.companyVo.id == initiatorCompanyId)
              let departIdArr = departArr.map( x=> x.deptVo.id )
              resolve(departIdArr)
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    isRowClick(row) {
      console.log('isRowClick')
      console.log('isRowClick',row.row)
      if (this.isTerminated(row.row) && !this.allowTerminatedContract) {
        this.rowData = {};
        this.$message.warning('该合同已终止，无法发起变更申请');
        return;
      }
      this.rowData = row.row;
    },
    async confirm() {
      if (!this.rowData || !Object.keys(this.rowData).length) {
        this.$message.warning('请选择一条数据！')
        return;
      }
      if (this.isTerminated(this.rowData) && !this.allowTerminatedContract) {
        this.$message.warning('该合同已终止，无法发起变更申请');
        return;
      }
      // flowName
      if (this.rowData) {
        let data = {
          // id: this.fieldType == 'flow' ? this.rowData.flowProxyId : this.rowData.id,
          id: this.rowData.id,
          name:this.rowData?.name || this.rowData.contractName,
        };
        if (this.fieldType == 'contract'){
          let flowData = await this.getAllFormFlowDataByBizId(this.rowData.id);
          console.log('flowData--1111',flowData)
          console.log('合同-this.rowData',this.rowData)
          data = {...data,...{
            classificationName:this.rowData.classificationName,
            cumulativePaymentMoney:this.rowData?.contractLedgerDataVo?.cumulativePaymentMoney || 0, // 累计付款金额
            contractNumber:this.rowData.contractNumber, // 合同编号
            contractSum:this.rowData.contractSum, // 合同金额
            currencyCode:this.rowData.currencyCode || this.rowData.currency || '', // 币种编码
            currencyName:this.currencyName(this.rowData) === '-' ? '' : this.currencyName(this.rowData), // 币种名称
            latestOriginalAmount:this.amountValue(this.rowData, 'original'), // 当前合同原币金额
            latestContractSum:this.amountValue(this.rowData, 'rmb'), // 当前合同人民币金额
            terminated:this.isTerminated(this.rowData), // 是否已终止
            terminationApplicationId:this.rowData.terminationApplicationId || '', // 终止申请id，用于查询终止结算差额
            projectName:this.rowData.projectName || '', // 合同项目名称
            projectId:this.rowData.projectId || '', // 合同项目id
            initiatorCompanyId:this.rowData.companyId, // 发起人公司id(也可以拿this.value的值)
            cumulativeCollectMoney: this.rowData.contractLedgerDataVo?.cumulativeCollectMoney || 0, // 累计已收款
            paymentType: JSON.parse(this.myValue).paymentType ? JSON.parse(this.myValue).paymentType :'', // 付款类型
            formType: JSON.parse(this.myValue).formType ? JSON.parse(this.myValue).formType :'',
            initiatorId: this.myValue != '' && JSON.parse(this.myValue).initiatorId ? JSON.parse(this.myValue).initiatorId : '',
            selectCompanyId:this.myValue != '' && JSON.parse(this.myValue).selectCompanyId ? JSON.parse(this.myValue).selectCompanyId : '',
            rowData:JSON.stringify(flowData[0])
          }}
        } else if (this.fieldType == 'flow'){
          let flowObj = { // 这个发起人公司暂时无用，先留着
            initiatorCompanyId: this.myValue != '' && JSON.parse(this.myValue).initiatorCompanyId ? JSON.parse(this.myValue).initiatorCompanyId : this.$store.state.user.companyId,
            flowType: JSON.parse(this.myValue).flowType ? JSON.parse(this.myValue).flowType :'',
            rowData:JSON.stringify(this.rowData)
          }
          data = {...data,...flowObj}
        }

        this.$emit('selectFlow', JSON.stringify(data));
      }
      // console.log('this.rowData',this.rowData)
      this.viewName = this.rowData?.name || this.rowData.contractName
      this.handleClose();
    },

    async showContractDetail(row) {
      const records = await this.getEffectiveChangeRecords(row.id);
      if (records.length) {
        this.versionRecords = records.map((record, index) => ({
          ...record,
          __key: record.id || record.sourceApplicationId || `${row.id}-${index}`,
          changeTypeName: this.changeTypeName(record.changeType)
        }));
        this.versionDialogVisible = true;
        return;
      }
      await this.checkFlowDetail(row);
    },
    changeTypeName(type) {
      const value = String(type || '').toUpperCase();
      return value === 'TERMINATION' || value === 'CONTRACT_TERMINATION' ? '合同终止' : '补充协议';
    },
    async getEffectiveChangeRecords(contractId) {
      const result = await new Promise(resolve => {
        this.$axios.post('/web/measuring/api/contractChange/listByContract', {
          data: { contractId },
          pagination: false
        }, res => resolve(res));
      });
      if (!result || !result.isSuccess) return [];
      const data = result.data;
      const records = Array.isArray(data) ? data : (data && (data.dataList || data.records)) || [];
      return records.filter(record => {
        const type = String(record.changeType || '').toUpperCase();
        const status = String(record.status || '').toUpperCase();
        return (!type || type === 'SUPPLEMENT' || type === 'CONTRACT_SUPPLEMENT_CHANGE')
          && (!status || status === 'EFFECTIVE');
      }).sort((left, right) => {
        const leftTime = new Date(left.effectiveDate || left.createDate || left.applyTime || 0).getTime();
        const rightTime = new Date(right.effectiveDate || right.createDate || right.applyTime || 0).getTime();
        return rightTime - leftTime;
      });
    },
    async openVersionRecord(record) {
      const applicationId = record.sourceApplicationId || record.applicationId || record.id;
      if (!applicationId) {
        this.$message.warning('该版本缺少变更申请编号');
        return;
      }
      const flowData = await this.getAllFormFlowDataByBizId(applicationId, 'contract_supplement_change');
      if (!flowData.length) {
        this.$message.warning('未找到该补充协议的变更详情');
        return;
      }
      this.versionDialogVisible = false;
      this.previewReserveHandle(flowData[0], 'contract_supplement_change');
    },
    // 点击查看合同流程详情
    async checkFlowDetail(row){
      const flowData = await this.getAllFormFlowDataByBizId(row.id, 'contract_seal_review');
      if (flowData.length) this.previewReserveHandle(flowData[0], 'contract_seal_review');
    },
    // 查看详情（本身列表不是流程）
    previewReserveHandle(row,type){
      console.log('previewReserveHandle',row)
      if (type == 'contract_seal_review' || type == 'contract_supplement_change') {
        this.currentRowFlowData = row;

        this.selectFlowType = row.auditWay;
        this.flowId = row.flowProxyId;
        this.flowInstanceId = row.id;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.currentNodeProxyId;
        this.jobTaskId = row.jobTaskId;
        this.isExamine = false;
        this.isReInitiate = false;
        this.btnVisible = false;
        const relevance = Array.isArray(row.flowInstanceBizRelevanceList) ? row.flowInstanceBizRelevanceList : [];
        const find = relevance.find(item => item.otherBiz == type || item.otherBiz == row.auditWay);
        if (!find) return;
        this.businessId = find.otherBizId;
        const company = relevance.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        this.examineDialogVisible = true;
      }
    },
    // 获取表单流程数据（传多条业务id获取数据）
    getAllFormFlowDataByBizId(id, otherBiz = 'contract_seal_review') {
      console.log('getAllFormFlowDataByBizId')
      let data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        flowInstanceBizRelevanceList: [
          {
            otherBiz,
            otherBizId: id
          },
        ],
        initiator:"all"
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        )
      })
    },

  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  .dialog-container {
    // height: 600px;
    height: 48vh;
    overflow-y: auto;
  }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}

.contract-version-list {
  border-top: 1px solid #ebeef5;
}

.contract-version-item {
  display: flex;
  align-items: center;
  min-height: 46px;
  padding: 0 8px;
  border-bottom: 1px solid #ebeef5;
  color: #606266;
  cursor: pointer;
}

.contract-version-item span {
  flex: 1;
}

.contract-version-item span:nth-child(2) {
  text-align: center;
}

.contract-version-item span:nth-child(3) {
  text-align: right;
}

.contract-version-item i {
  margin-left: 12px;
  color: #409eff;
}

.contract-version-empty {
  padding: 24px;
  color: #909399;
  text-align: center;
}
</style>
