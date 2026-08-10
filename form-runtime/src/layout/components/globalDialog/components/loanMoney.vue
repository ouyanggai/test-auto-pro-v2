<template>
  <div class="container">
    <el-dialog
      :title="propData.repay == 'repayLoan' ? '关联借款单' : '关联请款单'"
      :visible="visible"
      :close-on-click-modal="false"
      append-to-body
      :width="propData.chooseData ? '55%' : '70%'"
      top='10vh'
      @close='handleClose'
    >
      <div style="height:50vh;margin:0 5px">
        <!-- <el-button type="primary" @click="handleAdd" v-if="!propData.chooseData">添加</el-button> -->
        <dy-table
          :showRadio="false"
          showCheckBox
          maxTableHeight="400"
          :actions="tableAction"
          :keys="propData.repay == 'repayLoan' ? colKey2 : colKey"
          :fetchData="getTableData"
          :list="tableData"
          :pagination="tablePagination"
          @rowClick="handleRowClick"
           ref="dyTable"
          :isPagination="true"/>
      </div>
      <div slot="footer" class="dialog-footer" style="text-align: center;">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="handleChoose">确 定</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone, capitalMoney } from '@/utils'
import DyTable from '@/components/DyTable';
import math from '@/utils/math.js'
export default {
  name: '',
  components: { DyTable },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    propData: {
      type: Object,
      default: _ => ({ chooseData: false })
    }
  },
  data() {
    return {
      colKey: {
        flowName: {
          label: '流程名称',
          minWidth: '200',
          showTooltip: true,
        },
        payCompanyName: {
          label: '付款单位',
          minWidth: '200',
          showTooltip: true,
        },
        payMoney: {
          label: '请款金额（元）',
          minWidth: '120',
        },
        alreadyMoney: {
          label: '已核销金额（元）',
          minWidth: '120',
        },
        notMoney: {
          label: '未核销金额（元）',
          minWidth: '120',
        },
        // initiator:{
        //   label:'申请人',
        //   minWidth:'100',
        // },
        // createDate:{
        //   label:'提交申请时间',
        //   minWidth:'140',
        // },
      },
      colKey2: {
        flowName: {
          label: '流程名称',
          minWidth: '200',
          showTooltip: true,
        },
        payCompanyName: {
          label: '付款单位',
          minWidth: '200',
          showTooltip: true,
        },
        payMoney: {
          label: '借款金额（元）',
          minWidth: '120',
        },
        alreadyMoney: {
          label: '已还金额（元）',
          minWidth: '120',
        },
        notMoney: {
          label: '未还金额（元）',
          minWidth: '120',
        },
        // initiator:{
        //   label:'申请人',
        //   minWidth:'100',
        // },
        // createDate:{
        //   label:'提交申请时间',
        //   minWidth:'140',
        // },
      },
      tableAction: [
        {
          label: '详情',
          actionFixed: 'right',
          size: 'medium',
          action: (row) => {
            this.$fm2.show("flowDetail", {
              data: {
                id: row.processId, // 流程实例id
                // flowInstanceBizRelevanceList: [{
                //   otherBiz: undefined, // 业务类型
                //   otherBizId: row.id // 业务id
                // }]
              }
            });
            // this.showDetail(row)
          }
        },
      ],
      tableData: [],
      dialogForm: {
        userId: this.$store.state.user.userId,
        unitName: '',
        bank: '',
        lineNumber: ''
      },
      editRow: null,
      clickRow: null,
      dialogRules: {
        unitName: [{ required: true, message: '请输入', trigger: 'blur' }],
        bank: [{ required: true, message: '请输入', trigger: 'blur' }],
        lineNumber: [{ required: true, message: '请输入', trigger: 'blur' }]
      },
      tableColKey: {
        unitName: '单位或姓名',
        bank: '开户行',
        lineNumber: '账户'
      },
      tableAction2: this.propData.chooseData ? [] : [
        {
          label: '编辑',
          width: '150',
          action: row => {
            this.editRow = row;
            this.dialogForm = {
              userId: this.$store.state.user.userId,
              id: row.id,
              unitName: row.unitName,
              bank: row.bank,
              lineNumber: row.lineNumber
            };
            this.addEditVisible = true;
          }
        },
        {
          label: '删除',
          action: row => {
            this.handleDelete(row);
          }
        }
      ],
      tableTableData: [],
      tablePagination: {
        pages: 1,
        size: 10,
        total: 10
      },
      addEditVisible: false

    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    console.log(this.propData, 'propData');
    // this.$fm.show('commonAccounts', { chooseData: true }).then(dialog => {
    //   dialog.$on('confirmed', (res) => {
    //     console.log(res, 'res'); // 选中的数据
    //   });
    // });
  },
  methods: {
    handleRowClick({ row, event, column }) {
      // this.clickRow = row;
    },
    handleChoose() {
      this.clickRow = deepClone(this.$refs.dyTable.selectDatas);
      this.$emit('confirmed', this.clickRow);
      this.handleClose();
    },
    handleDelete(row) {
      this.$confirm('确定删除?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          this.deleteRow(row.id);
        })
        .catch(() => { });
    },
    deleteRow(id) {
      this.$axios.post(
        'web/measuring/api/userInformation/delete',
        {
          data: { id }
        },
        res => {
          if (res.isSuccess) {
            this.getTableData();
          }
        }
      );
    },
    handleAdd() {
      this.editRow = null;
      this.dialogForm = {
        userId: this.$store.state.user.userId,
        unitName: '',
        bank: '',
        lineNumber: ''
      };
      this.addEditVisible = true;
    },
    addOrEdit() {
      this.$refs.dialogForm.validate((valid) => {
        if (valid) {
          this.$axios.post(
            this.editRow
              ? '/web/measuring/api/userInformation/update'
              : '/web/measuring/api/userInformation/save',
            {
              data: this.dialogForm
            },
            (res) => {
              if (res.isSuccess) {
                this.addEditVisible = false;
                this.getTableData();
              } else {
                this.getTableData();
              }
            }
          );
        }
      });
    },
    getTableData() {
      let type = this.propData.repay == 'repayLoan' ? '3' : '2'
      let data = {
        data: {
          companyId: this.propData.companyId,//"8fe922a5da21445a8a26aba74d0af5e1",   报销公司ide
          status: "end",       //流程状态是否完结
          userId: this.propData.createrId || localstorageGet('userId'), //"1915188dd40f47e1a19cfa6b8a7ac563",   用户id
          type//:     请借款类型2请款3借款
        },
        pagination: true,
        pages: this.tablePagination.pages,
        size: this.tablePagination.size
      }
      this.$axios.post(Api.budgetManage.loanMoney, data, async res => {
        if (res.isSuccess) {
          let list = res.data || []
          let ids = [], tableData = []
          if (list.length) {
            list.forEach(item => {
              ids.push(item.id)
            })
            let auditWay = this.propData.repay == 'repayLoan' ? 'expense_loan' : 'request_funds'
            let flowList = await this.getInstanceId(ids, auditWay,'end')
            // this.repayList = deepClone(flowList)
            tableData = list.map(el => {
              let id = el.id
              let find = flowList.find(item => {
                return item.flowInstanceBizRelevanceList.find(el => el.otherBizId == id)
              })
              let processId = find.id
              let alreadyMoney = math.subtract(el.amountRecordVo['payMoney'], el.amountRecordVo['notMoney'])// - item['freezeMoney']
              let obj = {
                id,
                payCompanyName: el.applicationFundsVo.payCompanyName,
                processId,
                flowName: find.name,
                initiator: find.initiator,
                payMoney: el.amountRecordVo.payMoney,//item.formCellData['applicationFundsVo_payMoney'],
                createDate: el.createDate,
                notMoney: el.amountRecordVo.notMoney,
                freezeMoney: el.amountRecordVo.freezeMoney,
                alreadyMoney,
                expenseReimbursementId: el.applicationFundsVo.expenseReimbursementId,
                max: math.subtract(el.amountRecordVo.notMoney, el.amountRecordVo.freezeMoney)
              }
              return obj
            })
          }
          this.tableData = tableData
          this.tablePagination.total = res?.total || 0
          console.log(this.tableData, 'this.tableData');
        } else {
          // this.$message.error(res.message)
        }
      })
    },
    getInstanceId(ids, type,taskStatus) {
      let otherBiz = type
      const flowInstanceBizRelevanceList = [{
        otherBiz,            //流程类型
        otherBizIdList: ids, //业务id array
      }];
      const data = {
        useScope: 'invest',
        // taskStatus,//: 'end',
        initiator: 'all',
        flowInstanceBizRelevanceList
      };
      if(taskStatus){
        data.taskStatus = taskStatus
      }
      let api = Api.schedule.getFlowInstanceList
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, pagination: false }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data)
            } else {
              resolve([])
            }
          }
        });
      });
    },
    getTableData2() {
      this.$axios.post(
        '/web/measuring/api/userInformation/list',
        {
          data: {
            userId: this.$store.state.user.userId
          },
          pagination: true,
          current: this.tablePagination.pages,
          size: this.tablePagination.size
        },
        res => {
          if (res.isSuccess) {
            var { data } = res;
            this.tableData = data?.dataList || [];
            this.tablePagination.total = data.total || 0;
          }
        }
      );
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep{
  .dytable-view-container{
    padding:0;
  }
}
::v-deep .el-dialog {
  // margin-top: 5vh !important;
}
</style>
