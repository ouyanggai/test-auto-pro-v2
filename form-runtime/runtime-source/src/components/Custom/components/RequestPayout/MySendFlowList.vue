<!--
 * @Descripttion: 流程列表选择弹窗
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible" title="选择请款单" width="80%" top="100px" :close-on-click-modal="false"
     :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
     <div>
        <!-- <el-input style="width:240px;margin-right: 10px;" v-model.trim="serachName" clearable placeholder="查询流程名称">
        </el-input>
        <el-button type="primary" @click="getList">查询</el-button> -->
        <dy-table
          :fetchData="getList"
          :keys="colKey"
          :actions="actions"
          :list="myFlowList"
          :isPagination="true"
          :pagination="pagination"
          :showCheckBox="true"
          @selectDataEvent="selectDataEvent"
        ></dy-table>
        <!-- @rowClick="isRowClick" -->
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId" 
    :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" />

  </div>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import { localstorageGet } from '@/utils/auth';
import { formatMoney } from '@/utils/index';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import DyTable from '@/components/DyTable';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: '',
  components: {DyTable,EnterpriseExamineDialog},
  model: {
    prop: 'myValue', // value
    // event: 'changeMyValue' // input
  },
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
  },
  data() {
    return {
      viewName:'',

      // 流程数据
      btnVisible:false,
      selectFlowType:'',
      examineDialogVisible: false,
      flowId: '', // 绑定的业务id
      flowInstanceId: '', // 流程实例id
      formId: '',
      flowNodeProxyId: '',
      jobTaskId: '',
      isExamine: false,
      isReInitiate: false,
      businessId: '',

      myFlowList: [],
      serachName: '',
      pagination: {
        total: 0,
        pages: 1,
        current: 1,
        size: 10
      },
      colKey: {
        name: {
          label: '名称',
          showTooltip: true,
          minWidth: '120',
        },
        payCompanyName: {
          label: '付款单位',
          showTooltip: true,
          minWidth: '280',
        },
        // name: {
        //   label: '标题',
        //   showTooltip: true,
        //   minWidth: '280',
        // },
        // flowName: {
        //   label: '流程名称',
        //   showTooltip: true,
        //   minWidth: '150',
        // },
        payMoney: {
          label: '请款金额（元）',
          // showTooltip: true,
          minWidth: '150',
        },
        hasMoney: {
          label: '已还金额（元）',
          // showTooltip: true,
          minWidth: '150',
        },
        notMoney: {
          label: '未还金额（元）',
          // showTooltip: true,
          // handle: (scope, createElement) => {
          //   return createElement('span', scope.row.notMoney.toFixed(2));
          // },
          minWidth: '150',
        },
        freezeMoney: {
          label: '冻结金额（元）',
          // showTooltip: true,
          // handle: (scope, createElement) => {
          //   return createElement('span', scope.row.notMoney.toFixed(2));
          // },
          minWidth: '150',
        },
        // createDate: {
        //   label: '发起时间',
        //   minWidth: '160',
        //   showTooltip: true
        // },
      },
      actions: [
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
      rowData:{},
      selectData:[]
    };
  },
  computed: {
  },
  watch: {
    async myValue(newVal, oldVal) {
      console.log('w====atch',newVal)
      
      if (!this.myFlowList.length) {
        this.myFlowList = await this.getList();
      }
      let obj = this.myFlowList.find(x=>x.id == this.myValue);
      this.viewName = obj?.name || '';
      this.rowData = obj;
    },
    visible(newVal){
      this.init();
    }
  },
  created() { 
    
  },
  mounted() {
    // this.init();
  },
  methods: {
    selectDataEvent(data){
      this.selectData = data
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    async init() {
      // if (!this.myFlowList.length) {
      //   this.myFlowList = await this.getList();
      // }
      this.myFlowList = await this.getList();
      let obj = this.myFlowList.find(x=>x.id == this.myValue);
      
      this.viewName = obj?.name || '';
      this.rowData = obj;
    },
    getList() { // 获取公司部门架构数据
      return new Promise((resolve,reject)=>{
        // const data = {
        //   flowName: this.serachName,
        //   useScope: 'invest',
        //   auditWayList: [],
        //   statusList:['end'],
        //   flowInstanceBizRelevanceList: [
        //     // {
        //     //   otherBiz: 'company',
        //     //   otherBizId:''
        //     // },
        //     {
        //       otherBiz: 'expense_loan',
        //       otherBizId:''
        //     },
        //   ],
        // };
        // this.$axios.post(
        //   Api.schedule.getFlowInstanceList,
        //   {
        //     data,
        //     pagination: true,
        //     pages: this.pagination.pages,
        //     size: this.pagination.size
        //   },
        //   res => {
        //     if (res.isSuccess) {
        //       this.pagination.total = res.total || 0;
        //       resolve(res.data || [])
        //       // this.myFlowList = res.data || [];
        //       if(res.data.length==0) return
        //       const ids = []
        //       res.data.map(item=>{
        //         const otherBiz = item.flowInstanceBizRelevanceList.filter(i=> i.otherBiz =='expense_loan')[0].otherBizId
        //         ids.push(otherBiz)
        //       })
        //       this.getDetailedMoney(ids)
        //     } else {
        //       this.$message.error(res.message);
        //     }
        //   }
        // );
        const doc = document.getElementsByClassName('form-container')
        const values = doc[0].__vue__.$refs.generateForm.getValues();
        console.log('values2',values)
        let companyId = ''
        if(values.travelPersonnelVoList){ //差旅报销
          companyId = values.expenseCompanyId
          if(!values.expenseCompanyId){
          return this.$message.warning('请先选择报销单位！')
          }
        }else if(values.applicationFundsVo_payCompanyId){//还款单
          companyId = values.applicationFundsVo_payCompanyId
        } else if(values.RequestPayoutList){ //请款单
          companyId = parseJsonObject(values.myCompanyName).id || '';
          if(!companyId){
            return this.$message.warning('请先选择收款单位！')
          }
        } else  {
          return this.$message.warning('请先选择收款单位！') 
        }
        
        const data={
          data:{
            companyId:companyId,
            status:'end',
            userId:localstorageGet('userId'),
            type:'7'
          },
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size,
        }
        this.$axios.post(
          '/web/expenseReimbursement/loanMoney',
          data,
          res => {
            if (res.isSuccess) {
              this.pagination.total = res.total || 0;
              resolve(res.data || [])
              const  list = []
              res.data.map(item=>{
                  item.name = '请款单'
                  item.payCompanyName = item.applicationFundsVo&&item.applicationFundsVo.payCompanyName
                  item.payMoney = item.amountRecordVo?item.amountRecordVo.payMoney:0
                  item.notMoney = item.amountRecordVo?item.amountRecordVo.notMoney:0
                  item.hasMoney = (item.payMoney*1000 - item.notMoney*1000)/1000
                  item.freezeMoney = item.amountRecordVo?item.amountRecordVo.freezeMoney:0
                  list.push(item)
                })
                this.myFlowList = list
            } else {
              this.$message.error(res.message);
            }
          }
        );
        
      })
    },
    // getDetailedMoney(ids){
    //   this.$axios.post(
    //       Api.budgetManage.detailedMoney,
    //       {
    //         data:{customerCode:this.$store.state.user.customerCode},
    //         grouping:true,
    //         ids:ids
    //       },
    //       res => {
    //         if (res.isSuccess) {
    //           res.data.map(k=>{
    //             this.myFlowList.map(item=>{
    //               if(item.flowInstanceBizRelevanceList.filter(i=> i.otherBiz =='expense_loan')[0].otherBizId == k.expenseReimbursementId){
    //                 item.payMoney = k.payMoney
    //                 item.notMoney = k.notMoney
    //                 item.hasMoney = (k.payMoney*1000 - k.notMoney*1000)/1000
    //                 item.freezeMoney = k.freezeMoney
    //               }
    //             })
    //           })
              
    //         } else {
    //           this.$message.error(res.message);
    //         }
    //       }
    //     );
    // },
    isRowClick(row) {
      this.rowData = row.row;
    },  
    confirm() {
      if(this.selectData.length==0){
        this.$message.warning('请选择一条流程！')
        return;
      }
      if(this.selectData.length > 1){
        this.$message.warning('只能选择一条流程！')
        return;
      }
      this.$emit('selectFlow', this.selectData);
      this.viewName = this.selectData[0].name
      this.handleClose();
      // if (!Object.keys(this.rowData).length) {
      //   this.$message.warning('请选择一条流程！')
      //   return;
      // }
      // if (this.rowData) {
      //   this.$emit('selectFlow', this.rowData);
      // }
      // this.viewName = this.rowData.name
      // this.handleClose();
    },
    async previewHandle(row, type){
      const data = {
          flowName: '',
          useScope: 'invest',
          auditWayList: [],
          statusList:['end'],
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'request_funds',
              otherBizId:row.id
            },
          ],
        };
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data,
            pagination: true,
            pages: this.pagination.pages,
            size: this.pagination.size
          },
          res => {
            if (res.isSuccess) {
              const flow = res.data[0];
              this.selectFlowType = flow.auditWay;
              this.isExamine = false;
              this.isReInitiate = false;
              this.flowId = flow.flowProxyId;
              this.formId = flow.formProxyId;
              this.flowNodeProxyId = flow.currentNodeProxyId;
              this.flowInstanceId = flow.id;
              this.jobTaskId = flow.jobTaskId;
              this.examineDialogVisible = true;
              const find = flow.flowInstanceBizRelevanceList.find(item => item.otherBiz == flow.auditWay);
              this.businessId = find?.otherBizId || '';
            } else {
              this.$message.error(res.message);
            }
          }
        );
      // this.selectFlowType = row.auditWay;
      // this.isExamine = false;
      // this.isReInitiate = false;
      // this.flowId = row.flowProxyId;
      // this.formId = row.formProxyId;
      // this.flowNodeProxyId = row.currentNodeProxyId;
      // this.flowInstanceId = row.id;
      // this.jobTaskId = row.jobTaskId;
      // this.examineDialogVisible = true;
      // const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
      // this.businessId = find?.otherBizId || '';
    }
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
