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
        <el-input v-if="parsedMyValue.flowType=='government_dock_need'" clearable style="width:180px;margin-right: 10px;" v-model.trim="searchForm.name" placeholder="查询标题名称"></el-input>
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


  </div>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import mixin from './mixin.js';
import { approveManageFlowStatus, deepClone,getObjById } from '@/utils';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import DyTable from '@/components/DyTable';
import { localstorageSet,localstorageGet } from '@/utils/auth';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: '',
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
          // width: '140px',
        },
        contractNumber: {
          label: '合同编号',
          showTooltip: true,
          // width: '140px',
        },
        newContractBody: {
          label: '合同主体',
          showTooltip: true,
          // width: '140px',
        },
        contractSum: '合同金额(元)',
        contractProjectName: {
          label: '合同项目名称',
          showTooltip: true,
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
            this.checkFlowDetail(row);
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
      contractBodyList:[]
    };
  },
  computed: {
    // parsedMyValue 让列表弹窗在字段尚未配置值时仍能完成初始化和查询。
    parsedMyValue () {
      return parseJsonObject(this.myValue)
    },
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
        console.log(123, this.parsedMyValue)
        this.$nextTick(async x=>{
          console.log('2222111',this.$refs[this.fieldType])
          if (this.$refs[this.fieldType]){
            this.$refs[this.fieldType].doLayout();
          }

          if (this.parsedMyValue.formType == 'contract_receipt_form') { // 收款登记表特殊处理
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
      let getMyNewVal = parseJsonObject(newVal);
      
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
      if (this.fieldType == 'contract'){
        this.myFlowList.forEach(item=>{
          // 处理合同主体数据转换
          this.dealBodyList(item);
        })
      }
    },
    "pagination.size": async function(newVal, oldVal){
      this.myFlowList = await this.getList();
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
    handleClose() {
      this.$emit('update:visible', false);
    },
    async init() {
      console.log('init')
      if(!this.fieldSelectType) return;
      let getMyNewVal = parseJsonObject(this.myValue);
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
      const myValue = this.parsedMyValue
      console.log('getList************', myValue, this)
      // console.log('getList2',this.myValue)
      // console.log('getList22',typeof this.myValue)
      // console.log('this.fieldType',this.fieldType)
      return new Promise(async (resolve,reject)=>{
        let url='',data = {};
        // initiatorCompanyId = ''
        let selectCompanyId = myValue.selectCompanyId || '';
        let initiatorCompanyId = myValue.initiatorCompanyId || this.$store.state.user.companyId;
        let initiatorId = myValue.initiatorId || '';
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
          if (myValue.paymentType){
            data.contractSubtableVo.paymentType = myValue.paymentType; // 收付款分类 0收 1付 2其它 3收付款合同
          }
        } else if  (this.fieldType == 'flow'){ // 流程
          let flowType = myValue.flowType || '';
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
          const userId = myValue.userId || ''
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
      this.rowData = row.row;
    },
    async confirm() {
      if (!Object.keys(this.rowData).length) {
        this.$message.warning('请选择一条数据！')
        return;
      }
      const myValue = this.parsedMyValue
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
            projectName:this.rowData.projectName || '', // 合同项目名称
            projectId:this.rowData.projectId || '', // 合同项目id
            initiatorCompanyId:this.rowData.companyId, // 发起人公司id(也可以拿this.value的值)
            cumulativeCollectMoney: this.rowData.contractLedgerDataVo?.cumulativeCollectMoney || 0, // 累计已收款
            paymentType: myValue.paymentType || '', // 付款类型
            formType: myValue.formType || '',
            initiatorId: myValue.initiatorId || '',
            selectCompanyId: myValue.selectCompanyId || '',
            rowData:JSON.stringify(flowData[0])
          }}
        } else if (this.fieldType == 'flow'){
          let flowObj = { // 这个发起人公司暂时无用，先留着
            initiatorCompanyId: myValue.initiatorCompanyId || this.$store.state.user.companyId,
            flowType: myValue.flowType || '',
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

    // 点击查看合同流程详情
    async checkFlowDetail(row){
      let flowData = await this.getAllFormFlowDataByBizId(row.id);
      console.log('flowData',flowData)
      this.previewReserveHandle(flowData[0],'contract_seal_review')
    },
    // 查看详情（本身列表不是流程）
    previewReserveHandle(row,type){
      console.log('previewReserveHandle',row)
      if (type == 'contract_seal_review') { // 合同盖章
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
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find.otherBizId;
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        this.examineDialogVisible = true;
      }
    },
    // 获取表单流程数据（传多条业务id获取数据）
    getAllFormFlowDataByBizId(id) {
      console.log('getAllFormFlowDataByBizId')
      let data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'contract_seal_review',
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
</style>
