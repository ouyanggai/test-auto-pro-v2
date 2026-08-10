<template>
  <div class="framework-manage-setting-container">
    <div>
      <el-form :model="selectForm" inline>
        <el-form-item
          label="单位："
          style="margin-left: 10px"
          v-if="this.activeName == 'company'"
        >
        <el-select v-model="selectForm.companyId" placeholder="请选择" style="width: 300px" clearable>
            <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id" >
            </el-option>
        </el-select>
          <!-- <el-input
            v-model="selectForm.companyId"
            style="width: 300px"
            clearable
            placeholder="付款单位"
          /> -->
        </el-form-item>
        <el-form-item label="请款人：" v-else>
          <!-- <selectPersonTree
            v-model="selectForm.userId"
            :treedata="companyPersonTreeData"
            style="width: 300px"
          ></selectPersonTree> -->
          <el-input
            @focus="selectPersonFocus"
            @clear="selectForm.userId = ''"
            v-model="selectForm.userName"
            style="width: 300px"
            clearable
            placeholder="请款人"
          />
        </el-form-item>
        <el-button
          style="margin-left: 20px"
          type="primary"
          size="mini"
          @click="getList"
          >查询</el-button
        >
        <el-button
          style="margin-left: 20px"
          type="primary"
          size="mini" v-if="this.activeName == 'company'"
          @click="exportTableData"
          >导出</el-button
        >
      </el-form>
      <div class="main-right-personTable">
        <dy-table
          max-table-height="600"
          :keys="personColKey"
          ref="companyPersonInfoTable"
          :fetch-data="getList"
          :list="tableList"
          :actions="personColKeyActionis"
          :pagination="pagination"
          :isPagination="false"
          style="padding: 0px"
        />
      </div>
    </div>

    <el-dialog
      v-if="changePasswordVisible"
      :visible="changePasswordVisible"
      :title="isEdit ? '编辑' : '新增'"
      width="600px"
      :close-on-click-modal="false"
      @close="handleClose"
    >
      <el-form ref="addJobForm" :model="updateInfo" label-width="110px">
        <el-form-item
          label="名称:"
          prop="name"
          :rules="{ required: true, message: '名称不能为空', trigger: 'blur' }"
        >
          <el-input
            v-model="updateInfo.name"
            placeholder="请输入名称"
            maxlength="30"
          />
        </el-form-item>
        <el-form-item
          label="等级:"
          prop="level"
          :rules="{ required: true, message: '等级不能为空', trigger: 'blur' }"
        >
          <el-input
            v-model="updateInfo.level"
            placeholder="请输入等级数字"
            maxlength="30"
            type="number"
          />
        </el-form-item>
        <el-form-item
          label="等级标识:"
          prop="levelDiv"
          :rules="{
            required: true,
            message: '等级标识不能为空',
            trigger: 'blur',
          }"
        >
          <el-input
            v-model="updateInfo.levelDiv"
            placeholder="请输入等级标识"
            maxlength="30"
          />
        </el-form-item>
        <el-form-item
          v-if="isEdit"
          label="是否启用:"
          prop="levelDiv"
          :rules="{
            required: true,
            message: '是否启用不能为空',
            trigger: 'blur',
          }"
        >
          <el-select
            v-model="updateInfo.enableType"
            style="width: 100%"
            maxlength="30"
          >
            <el-option
              v-for="i in [
                { value: 'disable', name: '否' },
                { value: 'enable', name: '是' },
              ]"
              :key="i.value"
              :label="i.name"
              :value="i.value"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <span slot="footer">
        <el-button @click="handleClose">关 闭</el-button>
        <!-- <el-button type="primary" @click="updatePassword">确 定</el-button> -->
      </span>
    </el-dialog>

    <el-dialog
      v-if="companyDepartDutyVisible"
      :visible="companyDepartDutyVisible"
      :title="isTotalTitle ? '请款明细' : '未还请款'"
      width="85%"
      :close-on-click-modal="false"
      @close="closeCompanyDepartDutyDialog"
    >
      <div style="zoom: 0.99">
        <dy-table
          max-table-height="300"
          :keys="panyMoneyDetailcolkey"
          ref="companyPersonInfoTable"
          :fetch-data="(_) => {}"
          :list="panMoneyDetailList"
          :actions="panyMoneyDetailColKeyAction"
        />
      </div>
      <span slot="footer">
        <el-button @click="closeCompanyDepartDutyDialog">关 闭</el-button>
        <!-- <el-button type="primary" @click="confirmCompanyDepartDuty">确 定</el-button> -->
      </span>
    </el-dialog>
    <el-dialog
      v-if="returnMoneyVisible"
      :visible="returnMoneyVisible"
      :title="'已还明细'"
      width="85%"
      :close-on-click-modal="false"
      @close="returnMoneyVisible = false"
    >
      <div style="zoom: 0.99">
        <dy-table
          max-table-height="300"
          :keys="returnMoneyDetailcolkey"
          ref="companyPersonInfoTable"
          :fetch-data="(_) => {}"
          :list="returnMoneyDetailList"
          :actions="panyMoneyDetailColKeyAction"
        />
      </div>
      <span slot="footer">
        <el-button @click="returnMoneyVisible = false">关 闭</el-button>
        <!-- <el-button type="primary" @click="confirmCompanyDepartDuty"
          >确 定</el-button
        > -->
      </span>
    </el-dialog>
        <!-- 无表单流程查看 -->
        <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
        :isReInitiate="isReInitiate" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
        :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
        :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
        :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
        :initiatorId="initiatorId" :actionType="actionType" :flowType="flowType" :flowName="flowName"/>

        <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
        <EnterpriseExamineDialog v-if="examineDialogVisible" :isExamine="isExamine" :flowId="flowId" :formId="formId"
        :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :btnVisible="btnVisible"
        :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" :companyId="companyId"
        :showRightSide="showRightSide" :parallelNodeChooseList="parallelNodeChooseList" :manualChooseNodes="manualChooseNodes"
        :flowName="flowName" :isReInitiate="isReInitiate" :flowNodeType="flowNodeType" :initiatorId="initiatorId" />

        <!-- 查看流程 -->
        <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
    :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId"></CheckFlowNodeDetail>
  </div>
</template>
<script>
import Api from "@/api";
import selectPersonTree from "./selectPersonTree.vue";
import { localstorageGet } from "@/utils/auth";
// 引入公共表单组件
import DyTable from "@/components/DyTable";
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import math from '@/utils/math.js'
// import AssignPermissions from "../Info/components/AssignPermissions";
// import AddPersonDialog from "../Info/children/FrameworkSetting/components/addPersonDialog";
import Base64 from "base-64";
export default {
  name: "",
  components: {
    DyTable,
    selectPersonTree,
    ExamineDialog,
    EnterpriseExamineDialog,
    CheckFlowNodeDetail
    // AddPersonDialog,
  },
  props: {
    category: {
      type: String,
      default: "1",
    },
    activeName: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      typeObject: { 0: '报销单', 1: '差旅单', 2: '请款单', 3: '借款单', 4: '还款单', 5: '合同单', 7: '保证金还款单' },
      companyOption: [],
      companyPersonTreeData: [],
      selectForm: {
        userId: "",
        userName: '',
        companyId: "",
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10,
      },
      allUserList: [],
      companyDepartDutyList: [],
      companyDepartDutyForm: {
        companyDepartDutyValue: "",
      },
      companyDepartDutyVisible: false,
      operaPersonRowBtnType: "",

      // 分配权限的人员id
      userId: "",
      // 权限id--带入
      roleId: "",
      // 人员的权限--带入时
      rolePermission: [],
      // 权限名称--带入
      rolePermissionName: "",
      // 选中的部门id传给子组件
      departmentId: "",
      // 选中的部门的所属公司id传给添加人员子组件
      companyId: "",
      // 选中人员item传给子组件
      pickPersonRow: {},
      AssignPermissionsVisible: false,
      treeData: [],

      // 人员信息
      personOperaType: "", // 人员信息操作类型
      panyMoneyDetailcolkey: {
        expenseUserName: "请款人",
        type: {
          label: "名称",
          handle: (scope, createElement) => {
            return <span>{this.typeObject[scope.row.type || 0]}</span>;
          }
        },
        createDate: {
          label: "发起时间",
          showTooltip: true,
        },
        payMoney: {
          label: "请款金额（元）",
          handle: (scope, createElement) => {
            return <span>{scope.row.amountRecordVo?.payMoney || 0}</span>;
          },
        },
        returnMoney: {
          label: "已核销金额（元）",
          handle: (scope, createElement) => {
            return (
              <span>
                {math.subtract(scope.row.amountRecordVo?.payMoney || 0, scope.row.amountRecordVo?.notMoney || 0) || 0}
                {/* {scope.row.amountRecordVo?.payMoney -
                  scope.row.amountRecordVo?.notMoney || 0} */}
              </span>
            );
          },
        },
        freezeMoney: {
          label: "冻结金额（元）",
          handle: (scope, createElement) => {
            return <span>{scope.row.amountRecordVo?.freezeMoney || 0}</span>;
          },
        },
        notMoney: {
          label: "未核销金额（元）",
          handle: (scope, createElement) => {
            return <span>{scope.row.amountRecordVo?.notMoney || 0}</span>;
          },
        },
      },
      panyMoneyDetailColKeyAction: [
        {
          label: "详情",
          action: (row) => {
            this.handleView(row);
            // this.companyDepartDutyVisible = true;
            // this.updateInfo = row;
          },
        },
      ],
      personColKey: {
        name: this.activeName == "company" ? "付款单位" : "请款人",
        payMoney: {
          label: "请款金额",
          handle: (scope, createElement) => {
            return (
              <el-button
                type="text"
                onClick={(_) => {
                  this.totalOrNotReturnClicked(scope.row, "total");
                }}
              >
                {scope.row.payMoney}
              </el-button>
            );
          },
        },
        returnMoney: {
          label: "已核销金额",
          handle: (scope, createElement) => {
            return (
              <el-button
                type="text"
                onClick={(_) => {
                  this.returnMoneyClicked(scope.row);
                }}
              >
                {scope.row.returnMoney}
              </el-button>
            );
          },
        },
        notMoney: {
          label: "未核销金额",
          handle: (scope, createElement) => {
            return (
              <el-button
                type="text"
                onClick={(_) => {
                  this.totalOrNotReturnClicked(scope.row, "not");
                }}
              >
                {scope.row.notMoney}
              </el-button>
            );
          },
        },
        // enableType: {
        //   label: '公司/部门/岗位',
        //   width: '400px',
        //   toolTipContent: (scope) => {
        //     if (scope.row.companyDepartDuty) {
        //       return scope.row.companyDepartDuty;
        //     } else {
        //       return '无';
        //     }
        //   }
        // },
        // // roleName: '权限',
        // phone: '手机号',
        // enableType: {
        //   // 1不是，2是
        //   label: "是否启用",
        //   handle: function (scope, createElement) {
        //     return createElement(
        //       "span",
        //       scope.row.enableType == "disable" ? "否" : "是"
        //     );
        //   },
        // },
      },
      personColKeyActionis: [
        // {
        //   label: "详情",
        //   action: (row) => {
        //     this.companyDepartDutyVisible = true;
        //     // this.updateInfo = row;
        //   },
        // },
        // {
        //   label: "修改",
        //   action: (row) => {
        //     this.isEdit = true;
        //     this.changePasswordVisible = true;
        //     this.updateInfo = row;
        //     this.$refs.addJobForm && this.$refs.addJobForm.resetFields();
        //     // this.operaPersonRowBtnType = '查看';
        //     // this.operaCompanyDepartDuty(row);
        //   }
        // }
      ],
      importDepartDialogVisible: false,
      addPersonDialogVisible: false,
      updaterId: "",
      // 修改密码
      changePasswordVisible: false,
      updateInfo: {
        name: "",
        level: "",
        levelDiv: "",
      },
      dutyLevelList: [],
      isEdit: false,
      tableList: [],
      panMoneyDetailList: [],
      isTotalTitle: true,
      returnMoneyVisible: false,
      returnMoneyDetailList: [],
      returnMoneyDetailcolkey: {
        expenseUserName: "还款人",
        type: {
          label: "名称",
          handle: (scope, createElement) => {
            return <span>{this.typeObject[scope.row.type || 0]}</span>;
          }
        },
        createDate: {
          label: "发起时间",
          showTooltip: true,
        },
        payMoney: {
          label: "冲请款金额（元）",
          handle: (scope, createElement) => {
            return (
              <span>{scope.row.accountDetailedVoList[0]?.thisMoney || 0}</span>
            );
          },
        },
      },


    //   selectFlowType: '',
      ExpensesClaimFormVisible:false,
      operaType:'',
    //   flowId:'',
    //   flowInstanceId:'',
    //   flowNodeType:'',
      showFlowLog:'',
    //   parallelNodeChooseList:'',
    //   manualChooseNodes:'',
    //   formId:'',
      searchFlowType:'',
    //   flowNodeProxyId:'',
      noFormFlowInstanceId:'',
      flowProxyId:'',
    //   initiatorId:'',
      actionType:'',
    //   flowType:'',
    //   flowName:'',
    //   isExamine:false,
    //   isReInitiate:false,



    currentRowFlowData:{},
    checkViewFlowDetailVisible: false,
    isReInitiate:false,
    btnVisible:false,
    flowTemplateVisible:false,
    approveDialogVisible:false,
    flowJson:{},
    selectFlowType:'',
    flowType:'', // 默认为合同盖章评审
    flowTempList:[],
    examineDialogVisible: false,
    initiatorId: '', // 发起人id
    flowName: '',
    flowNodeType: '',
    flowId: '', // 绑定的业务id
    flowInstanceId: '', // 流程实例id
    formId: '',
    flowNodeProxyId: '',
    jobTaskId: '',
    businessId: '',
    companyId: '',
    isExamine: false,
    showRightSide:true,
    parallelNodeChooseList: [],
    manualChooseNodes: []
    };
  },
  computed: {},
  watch: {
    // $route: {
    //   handler: function(route) {
    //     if (route.name == 'FrameworkSetting') {
    //       this.showFlag = true;
    //     } else {
    //       this.showFlag = false;
    //     }
    //   },
    //   immediate: true
    // },
    addPersonDialogVisible() {
      this.getPageUserVoListOfGroup();
    },
    // AssignPermissionsVisible() {
    //   this.getPageUserVoListOfGroup();
    // },
  },
  created() {
    this.getCompanyTree();
    this.getParentCompanyList();
  },
  mounted() {
    window.abb = this;
    // this.getCustomerTree();
  },
  updated() {},
  methods: {
    exportTableData(exportCycle) {
      var param = this.getList();
      this.$axios.post(
        '/web/expenseReimbursement/expenseExcel',
        param,
        res => {
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            // const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            // link.download = '年度考核汇总.xlsx';
            link.click();
            // document.body.removeChild(link);
            window.URL.revokeObjectURL(link.href);
            link.remove();
          }
        }, '', { responseType: 'blob' }
      );
    },
    selectPersonFocus() {
      this.$fm.show("orgTree", { type: "company" }).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          console.log(res, 'ress')
          this.selectForm.userId = res.id;
          this.selectForm.userName = res.name;
        });
      });
    },
    // 查看流程
    async handleCheckFlow() {
        let row = this.currentRowFlowData;
        // 查看流程
        this.selectFlowType = row.auditWay;
        this.flowId = row.flowProxyId;
        this.flowInstanceId = row.id;
        this.initiatorId = row.createrId;
        this.checkViewFlowDetailVisible = true;
    },
    handleView(row) {
      // 查询当前绑定的流程，调用查看弹窗
      this.getInstanceId(row.id).then(data => {
        if (data) {
          this.previewHandle(data);
        } else {
          this.$message.error('流程已删除');
        }
      });
    },
    previewHandle(row) {
        console.log(row, 'previewHandle -- row');
        if (row.formExist == 'noForm') {
            this.currentRowFlowData = row;
            this.selectFlowType = row.auditWay;
            this.formExist = row.formExist;
            this.operaType = 'check';
            this.actionType = 'preview';
            this.isExamine = false;
            this.isReInitiate =false
            if (row.flowInstanceBizRelevanceList.length == 1) {
            this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
            } else {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
            this.flowId = find.otherBizId;
            }
            this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
            this.searchFlowType = row.auditWay;
            this.flowProxyId = row.flowProxyId
            this.ExpensesClaimFormVisible = true;

        } else {
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
    getInstanceId(id, type,taskStatus) {
      let otherBiz = type
      const flowInstanceBizRelevanceList = [{
        otherBiz,
        otherBizId: id,
      }];
      const data = {
        useScope: 'invest',
        // taskStatus:'waiting_send',
        // statusList:["await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end","draft"],//: 'waiting_send',
        initiator: 'all',
        // auditWayList: this.sFlowTypeList,
        flowInstanceBizRelevanceList
      };
      let api
      if(taskStatus == 'edit'){
        data.taskStatus = "waiting_send"
        api = Api.approveManage.getTaskList
      }else{
        api = Api.schedule.getFlowInstanceList
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, size: 1, pagination: true, pages: 1 }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data[0])
            } else {
              resolve()
            }
          }
        });
      });
    },
    getParentCompanyList() {
      // 查询公司列表
      this.$axios.post(
        Api.frameworkInfo.getCompanyFrameworkData,
        {
          data: {
            id: localstorageGet("companyId"), // 当前用的公司id
            flag: 1,
          },
        },
        (res) => {
          var arr = [];
          var fn = (list) => {
            list.forEach((item) => {
              if (item.type == 1) {
                arr.push({
                  id: item.id,
                  name: item.name,
                  type: item.type,
                });
                if (item.childrenList && item.childrenList.length) {
                  fn(item.childrenList);
                }
              }
            });
          };
          fn(res.data);
          this.companyOption = arr;
        }
      );
    },
    getCompanyTree() {
      // 获取公司部门架构数据
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: 3,
            id: localstorageGet("companyId"), // 公司id
          },
        },
        (res) => {
          if (res.isSuccess) {
            this.companyPersonTreeData = res?.data || [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getCompanyTree2() {
      this.$axios.post(
        Api.frameworkInfo.getCompanyFrameworkData,
        {
          data: {
            flag: "4", // 返回一二级公司
            id: localstorageGet("companyId"), // 公司id
          },
        },
        (res) => {
          if (res.isSuccess) {
            this.companyPersonTreeData = res?.data || [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    returnMoneyClicked(row) {
      this.$axios.post(
        "/web/expenseReimbursement/repaidMoney",
        {
          data: {
            status: "end",
            userId: row.userId,
            companyId: row.companyId,
            type: '2'
          },
        },
        (res) => {
          if (res.isSuccess) {
            this.returnMoneyDetailList = res.data || [];
            this.returnMoneyVisible = true;
          }
        }
      );
    },
    totalOrNotReturnClicked(row, type) {
      console.log(row, "panMoneyClicked");
      this.isTotalTitle = type == "total";
      this.$axios.post(
        "/web/expenseReimbursement/loanTotal",
        {
          data: {
            status: "end",
            userId: row.userId,
            companyId: row.companyId,
            type: '2'
          },
          notMoneyList: this.isTotalTitle,
        },
        (res) => {
          if (res.isSuccess) {
            this.panMoneyDetailList = res.data || [];
            this.companyDepartDutyVisible = true;
          }
        }
      );
    },
    addDutyLevel() {
      this.updateInfo = {
        name: "",
        level: "",
        levelDiv: "",
      };
      this.isEdit = false;
      this.changePasswordVisible = true;
      this.$refs.addJobForm && this.$refs.addJobForm.resetFields();
    },
    getDutyLevelList() {
      this.$axios.post(
        Api.postInfo.dutyLevel,
        {
          data: {},
          pagination: true,
          current: this.pagination.pages,
          size: this.pagination.size,
        },
        (res) => {
          if (res.isSuccess) {
            this.dutyLevelList = res.data || [];
            this.pagination.total = res.total;
          }
        }
      );
    },
    getList() {
      var param = {
        data: {
          status: "end",
          userId: this.selectForm.userId,
          companyId: this.selectForm.companyId,
          type: "2",
        },
        companyDetailed: this.activeName == "company",
      };
      this.$axios.post(
        "/web/expenseReimbursement/loanList",
        param,
        (res) => {
          if (res.isSuccess) {
            var data = res.data || [];
            var entries = Object.entries(data);
            this.tableList = entries.map((i) => {
              return {
                name: i[0],
                companyId:
                  this.activeName == "company" ? i[1][i[0]] : i[1].companyId,
                userId: this.activeName == "person" ? i[1][i[0]] : "",
                notMoney: i[1].notMoney,
                payMoney: i[1].payMoney,
                returnMoney: math.subtract(i[1].payMoney || 0, i[1].notMoney || 0) // i[1].payMoney - i[1].notMoney,
              };
            });
            console.log(this.tableList, "this.tableList");
          }
        }
      );
      return param;
    },
    // 赋值公司、部门和岗位下拉列表
    operaCompanyDepartDuty(row) {
      this.pickPersonRow = row;
      this.companyDepartDutyList = [];
      this.pickPersonRow.companyDeptVoList.forEach((x) => {
        this.companyDepartDutyList.push({
          value: x.id + "/" + x.deptVo.id + "/" + x.dutyVo.id,
          name:
            x.name +
            " / " +
            x.deptVo.departmentName +
            " / " +
            x.dutyVo.dutyName,
        });
      });
      this.companyDepartDutyVisible = true;
    },
    // 确认选择公司、部门和岗位
    confirmCompanyDepartDuty() {
      const arr = this.companyDepartDutyForm.companyDepartDutyValue.split("/");
      this.updaterId = arr[0];
      this.companyId = arr[0];
      this.departmentId = arr[1];
      // console.log('this.companyId',this.companyId)
      // console.log('this.departmentId',this.departmentId)
      if (this.operaPersonRowBtnType == "更新资料") {
        this.importAddPersonDialog();
        this.personOperaType = "edit";
      } else if (this.operaPersonRowBtnType == "分配权限") {
        this.userId = this.pickPersonRow.id;
        this.roleId = this.pickPersonRow.roleId;
        this.getRolePermission(this.pickPersonRow);
        this.AssignPermissionsVisible = true;
      } else if (this.operaPersonRowBtnType == "查看") {
        this.importAddPersonDialog();
        this.personOperaType = "check";
      }
      this.closeCompanyDepartDutyDialog();
    },
    // 查询
    search() {
      this.pagination.pages = 1;
      this.getPageUserVoListOfGroup();
    },
    // 获取集团所有公司人员列表
    getPageUserVoListOfGroup() {
      this.$axios.post(
        Api.frameworkManageInfo.getPageUserVoListOfGroup,
        {
          data: {
            name: this.selectForm.name,
            phone: this.selectForm.phone,
            // companyId: localstorageGet('companyId') // 公司id
          },
          current: this.pagination.pages,
          size: this.pagination.size,
          pagination: true,
          // pagination: true,
          // current: this.page,
          // size: this.size
        },
        (res) => {
          if (res.isSuccess) {
            // this.personInfoList = res.data.dataList;
            this.allUserList = res.data?.dataList || [];
            this.allUserList.forEach((y) => {
              y.companyDepartDuty = "";
              y.companyDeptVoList.forEach((x) => {
                y.companyDepartDuty +=
                  x.name +
                  " / " +
                  x.deptVo.departmentName +
                  " / " +
                  x.dutyVo.dutyName +
                  "<br/>";
              });
            });
            this.pagination.total = res.data.total;
            // this.total = res.data.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    closeCompanyDepartDutyDialog() {
      this.companyDepartDutyForm = {
        companyDepartDutyValue: "",
      };
      this.companyDepartDutyVisible = false;
    },
    // 客户组织架构
    getCustomerTree() {
      console.log("getCustomerTree11");
      const params = {
        data: {
          clienteleId: this.$store.getters.customerCode, // 查客户组织架构，带用户id
        },
      };
      this.$axios.post(
        Api.frameworkInfo.getCompanyFrameworkData,
        params,
        (res) => {
          if (res.isSuccess) {
            // console.log('getCustomerTree22')
            this.treeData = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取人员权限
    getRolePermission(row) {
      this.userId = row.id;
      this.$axios.post(
        Api.frameworkInfo.getRoleResourceTree,
        {
          data: {
            userId: this.userId,
            departmentId: this.departmentId,
          },
        },
        (res) => {
          this.rolePermission = res.data;
          if (res.isSuccess) {
            this.rolePermissionName = row.roleName;
            this.roleId = row.roleId;
          } else {
            this.rolePermissionName = "";
            this.roleId = "";
            return null;
          }
        }
      );
    },
    // 点击添加人员
    importAddPersonDialog() {
      this.addPersonDialogVisible = true;
      this.personOperaType = "add";
    },
    handleClose() {
      this.updateInfo = {
        name: "",
        level: "",
        levelDiv: "",
      };
      this.changePasswordVisible = false;
    },
    updatePassword() {
      this.$refs.addJobForm.validate((valid) => {
        if (valid) {
          this.$axios.post(
            this.isEdit
              ? "/web/user/api/dutyLevel/update"
              : "/web/user/api/dutyLevel/save",
            {
              data: this.updateInfo,
            },
            (res) => {
              if (res.isSuccess) {
                this.handleClose();
                this.getDutyLevelList();
              } else {
                this.getDutyLevelList();
              }
            }
          );
        }
      });
    },
    // 重置表单
    resetForm(formName) {
      if (this.$refs[formName] !== undefined) {
        this.$refs[formName].resetFields();
      }
    },
  },
};
</script>
  <style scoped lang="less">
.grid {
  width: 100%;
  display: grid;
  grid-template-rows: 35px;
  // grid-template-columns: 33% auto 33%;
  grid-template-columns: 1fr 1fr 1fr;
  .title {
    width: 110px;
    display: inline-block;
    text-align: right;
  }
}
.framework-manage-setting-container {
  height: 100%;
  padding: 14px;
  cursor: default;

  .main-right-panel {
    margin-left: 180px;
    border-left: 2px solid #e4e7ed;
    height: 100%;
    overflow-y: auto;
    padding-left: 10px;
    background-color: #fff;

    // background-color: #f0f3f5;
    .main-right-personTable {
      // padding: 5px 20px;
      overflow: hidden;
      // margin-top: 10px;
    }
  }

  ::v-deep .el-dialog.is-fullscreen {
    // width: 95%;
    // height: 95%;
    // margin: 20px auto;
  }
}

.el-dialog.is-fullscreen .el-dialog__body {
  max-height: initial !important;
  min-height: initial !important;
  height: calc(100% - 30px);
}
</style>
