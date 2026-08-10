<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>部门月度预算</h2>
      <div class="inner-container">
        <el-card class="box-card" shadow="never">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row style="margin-bottom:30px" :gutter="20">
              <el-col :span="12">
                <el-form-item label="公司名称：" prop="companyId">
                  <el-select v-model="form.companyId" placeholder="请选择" style="margin-right:6%;" @change="companyChange"
                    :disabled="type != 'init' ">
                    <el-option v-for="item in originDepartData" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                    <el-option v-if="params.companyName && !originDepartData.some(item=>item.id == form.companyId )" :label="params.companyName" :value="params.companyId"></el-option>
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="部门：" prop="projectId">
                  <el-select v-model="form.projectId" placeholder="请选择" style="margin-right:6%;" @change="departChange"
                  :disabled="type != 'init'">
                    <el-option v-for="item in departOptions" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px" :gutter="20">
              <el-col :span="12">
                <el-form-item label="月份：" prop="month">
                  <el-date-picker style="width: 100%;" v-model="form.month" type="month" placeholder="选择月" value-format="yyyy-MM"
                    :disabled="type == 'edit'  || type == 'detail'  || operaType == 'reEdit'" @change="monthChange">
                  </el-date-picker>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="预算金额(元)：" prop="money">
                  <!-- <el-input type="number" v-model="form.money" :disabled="type == 'detail'"></el-input> -->
                  <el-input-number v-model="form.money" :min="0.00" :precision="2" :step="0.1"
                    :controls="false" v-focusSelect :disabled="true" style="width: 100%;">
                  </el-input-number>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px" :gutter="20">
              <el-col :span="12">
                <el-form-item label="预算金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" maxlength="5000" show-word-limit :disabled="isDisabled('remarks')">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
          <div v-if="type == 'detail'">
            <eleupload ref="eleupload" :showOnly="true" :attachFile="attachFile" :uploadLimit="1"></eleupload>
          </div>
          <div v-else-if="type == 'edit' && isExamine">
            <eleupload ref="eleupload" :showOnly="true" :attachFile="attachFile" @clearAllFile="clearAllFile" :uploadLimit="1"></eleupload>
          </div>
          <div v-else-if="type == 'edit' && !isExamine">
            <eleupload ref="eleupload" :attachFile="attachFile" @clearAllFile="clearAllFile" :uploadLimit="1"></eleupload>
          </div>
          <div v-else-if="type == 'init'">
            <eleupload ref="eleupload" :uploadLimit="1"></eleupload>
          </div>

        </el-card>
        <el-card class="box-card" style="margin-top:30px" shadow="never">
          <div>
            <h3 style="display:inline-block;margin-bottom:20px;">
              预算详情
            </h3>
            <!-- <el-button style="margin-left:30px" type="primary" icon="el-icon-plus" @click="addDepartPlan">新增</el-button> -->
          </div>
          <div v-if="datas.length">
            <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;"
            shadow="never">
            <template slot="header">
              <div class="card-title" @click.stop.prevent="() => { }">
                <el-input :value="val.departName" :disabled="true"></el-input>
              </div>
            </template>
            <el-collapse v-model="val['activeNames']">
              <el-collapse-item name="1">
                <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                  <el-table-column type="index" label="编号" width="65px">
                  </el-table-column>
                  <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType">
                    <template slot-scope="scope">
                      <el-input v-model="scope.row.budgetType" :disabled="true"></el-input>
                    </template>
                  </el-table-column>
                  <!-- <el-table-column prop="relateProjId" label="是否关联项目" width="320">
                    <template slot-scope="scope">
                      <div>
                        <el-radio v-model="scope.row.isRelateProj" :label="true" :disabled="true">是
                        </el-radio>
                        <el-radio v-model="scope.row.isRelateProj" :label="false" :disabled="true">否
                        </el-radio>
                        <el-select v-model="scope.row.relateProjId" placeholder="请选择" :disabled="true">
                          <el-option v-for="item in projectOptions" :key="item.id" :label="item.name" :value="item.id">
                          </el-option>
                        </el-select>
                      </div>
                    </template>
                  </el-table-column> -->
                  <el-table-column prop="budgetMoney" label="预算金额(元)" width="160">
                    <template slot-scope="scope">
                      <el-input-number v-model="scope.row.budgetMoney" :disabled="isDisabled('budgetMoney')"
                        :precision="2" :step="0.1" :controls="false" v-focusSelect @blur="calculateTotalBudget">
                      </el-input-number>
                    </template>
                  </el-table-column>
                  <!-- <el-table-column fixed="right" label="操作" width="100" v-if="type != 'detail'">
                    <template slot-scope="scope">
                      <el-button @click="planDelete(index, scope.$index)" type="text" size="small"><i
                          class="el-icon-delete-solid delete-icon"></i></el-button>
                    </template>
                  </el-table-column> -->
                </el-table>
                <!-- <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(index)" v-if="type != 'detail'"></i> -->
                <!-- <el-button
                icon="el-icon-circle-plus"
                circle
                style="margin-top:10px;"
                @click="addPlan(index)"
              >
              </el-button> -->
              </el-collapse-item>
            </el-collapse>
          </el-card>
          </div>
          <el-empty description="暂无数据" v-else></el-empty>
        </el-card>
      </div>
    </div>
    <!-- <div class="footer-bt" v-if="!operaType">
      <el-button @click="prevStepHandle" plain>取 消</el-button>
      <el-button type="primary" plain @click="submit(0)">保 存</el-button>
      <el-button type="primary" @click="submit(1)" >提 交</el-button>
    </div> -->
    <div class="footer-bt" v-if="type != 'detail' && operaType !='reEdit' && isFlow && !isExamine">
      <div class="footer-inner">
        <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
        <el-button @click="prevStepHandle" plain>取 消</el-button>
        <el-button type="primary" plain @click="submit(0)">保 存</el-button>
        <el-button type="primary" @click="submit(1)">提 交</el-button>
      </div>
    </div>
    <div class="footer-bt" v-if="(type == 'init' || type == 'edit' || type == 'append')  && !isFlow">
      <div class="footer-inner">
        <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
        <el-button @click="prevStepHandle" plain>取 消</el-button>
        <el-button type="primary" plain @click="submit(0)">保 存</el-button>
        <el-button type="primary" @click="submit(1)">提 交</el-button>
      </div>
    </div>
    <div class="footer-bt" v-if="type == 'detail' && !isFlow">
      <el-button @click="cancel" plain>关 闭</el-button>
    </div>
    <PersonSelectDialog :visible.sync="nodeChooseVisible" @getSelectPerson="getSelectPerson" v-if="nodeChooseVisible">
    </PersonSelectDialog>
    <BranchChoose :branchChooseVisible.sync="branchChooseVisible" :manualChooseNodes="manulNodeList"
      @showSelectPerson="showSelectPerson" @saveBranchChooseNode="saveBranchChooseNode"
      @clearCheckboxPersonGroup="clearCheckboxPersonGroup" :chooseBranchNodeList="checkboxPersonGroup"></BranchChoose>
    <BranchPralleChoose @parlleSubmit="parlleSubmit" @parlleChoosePerson="parlleChoosePerson"
      :pralleNodeVisible.sync="pralleNodeVisible" :pralleNodeList="pralleNodeList"
      :checkboxPersonGroup="checkboxPersonGroup" :parlleNodePerson="parlleNodePerson">
    </BranchPralleChoose>
  </div>
</template>

<script>
import eleupload from '@/components/EleUpload';
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import Api from '@/api';
import { deepClone } from '@/utils';
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog';
import BranchChoose from '@/views/BudgetManage/CompanyBudget/components/branchChoose';
import BranchPralleChoose from '@/views/BudgetManage/CompanyBudget/components/branchPralleChoose';
import numFunc from '@/utils/number'; // 重写toFixed
Number.prototype.toFixed = numFunc;
import math from '@/utils/math.js'
export default {
  name: 'GroupMonthlyBudgetIndex',
  components: { eleupload, PersonSelectDialog, BranchChoose, BranchPralleChoose },
  props: ['operaType', 'id', 'param', 'flowNodeProxyId', 'showType', 'isExamine','flowInstanceId','flowProxyId','actionType','createrId'],
  data() {
    return {
      valDepart: '',
      value: '',
      value3: '',
      textarea2: '',
      planShow: false,
      form: {
        type: 2,
        companyId:'',//: localstorageGet('companyId'),
        projectId: '',
        month: '',
        money: '',
        remarks: '',
        departName: ''
      },
      datas: [
        {
          departName: '',
          departId: '',
          activeNames: '1',
          budget: [
          ]
        }
      ],
      originDepartData: [{
        id: localstorageGet('companyId'),
        name: localstorageGet('companyName')
      }],
      // companyOption: [{
      //   id: localstorageGet('companyId'),
      //   name: localstorageGet('companyName')
      // }],
      departOptions: [],
      projectOptions: [],
      mainRule: {
        companyId: [{ required: true, message: '请选择公司', trigger: 'change' }],
        projectId: [{ required: true, message: '请选择部门', trigger: 'change' }],
        month: [{ required: true, message: '请选择预算月份', trigger: 'change' }],
        money: [{ required: true, message: '请输入预算金额', trigger: 'blur' },
        // {
        //   pattern: /^(?!(0[0-9]{0,}$))[0-9]{1,}[.]{0,}[0-9]{0,}$/,
        //   message: '预算金额要大于0',
        //   trigger: 'blur'
        // }
      ]
      },
      hasInfo: false, // 选择月度和部门后是否有预算信息
      selectFlowType: 'depart_monthly_budget',
      type: 'init',
      attachFile: [],

      nodeChooseVisible: false,
      nextNodeName: '',
      nextNodeProxyId: '',
      checkboxPersonGroup: [],
      enableData: [],
      params:{},
      pickerOptions: {
        disabledDate(date) {
          // disabledDate 文档上：设置禁用状态，参数为当前日期，要求返回 Boolean
          const currentYear = new Date().getFullYear();
          return (
            date.getTime() < new Date(currentYear, 0, 1).getTime() ||
            date.getTime() > new Date(currentYear, 11, 31).getTime()
          );
        }
      },
      flowId: '',
      branchChooseVisible: false,
      manulNodeList: [],

      pralleNodeVisible: false,
      pralleNodeList: [],
      parlleNodePerson: {},
      isGroupMember:localstorageGet('companyId') == localstorageGet('topCompanyId') ? true:false, //是否是集团人员登录
    };
  },
  inject: {
    prevStepHandle: { value: 'prevStepHandle', default: 'cancel' },
    sumbitFlow: { value: 'sumbitFlow', default: null },
    submitFlowFinal: { value: 'submitFlowFinal', default: null }
  },
  computed: {
    isFlow() {
      // return false
      if (this.operaType) {
        return true;
      } else {
        return false;
      }
    },
  },
  watch: {
    $route: { //
      handler() {
        if (!this.isFlow) {
          this.init();
          this.type = this.showType || 'init';
          if (this.type != 'init') {
            const paramsStr = this.$route.params.str;
            if (paramsStr) {
              this.params = JSON.parse(paramsStr);
              this.dataInit();
            }
          }
        }
      },
      // immediate: true,
      deep: true
    },
    isFlow: {
      async handler(newVal) {
        if (newVal) {
          this.$bus.$off('depart_monthly_budget_before_handle');
          this.$bus.$on('depart_monthly_budget_before_handle', (val,that) => {
            this.busVal = val;
            this.examneObj = that //把审核组件实例传过来,方便后面把loading取消
            this.flowSubmit();
          });
          const that = this;
          // if (this.flowNodeProxyId) this.getPermision();
          if (this.actionType == 'examine') {
            this.getInputPermision()
          } else if (this.actionType == 'create' || this.actionType == 'edit') {
            this.getPermision();
          } else if (this.actionType == 'preview') {
            this.enableData = []
          }
          this.getMonthBudgetList().then(res => {
            // console.log('res',res)
            const params = res.data.dataList[0];
            this.$emit('updateParams', params);
          }).then(async function () {
            that.params = that.param;
            await that.getBudgetTypeOfGroup(that.params.companyId);
            that.departInfo = that.departOptions.find(item => {
              return item.id == that.params.projectId;
            });
            that.type = that.operaType;
            if (that.showType)that.type = that.showType;
            if (that.operaType == 'check') {
              that.type = 'detail';
            } else {
              that.type = that.showType;
              if (that.operaType == 'reEdit' || that.operaType == 'edit')that.type = 'edit';
            }
            that.init();
            that.dataInit();
          });
        } else {
          this.form.companyId = localstorageGet('companyId')
          // await this.getFlowType()
          this.init();
          this.type = this.showType || 'init';
          if (this.$route.query.type) {
            this.type = this.$route.query.type;
          }
          if (this.type != 'init') {
            const paramsStr = this.$route.params.str;
            if (paramsStr) {
              localstorageSet('monthDetailParams', paramsStr);
              this.params = JSON.parse(paramsStr);
              console.log(' this.params ', this.params )
              await this.getBudgetTypeOfGroup(this.params.companyId);
              this.dataInit();
            } else {
              const str = localstorageGet('monthDetailParams');
              if (str) {
                this.params = JSON.parse(str);
                await this.getBudgetTypeOfGroup(this.params.companyId);
                this.dataInit();
              }
            }
          }
          this.$once('hook:beforeDestroy', () => {
            localstorageRemove('monthDetailParams');
          });
        }
      },
      immediate: true
    }
  },
  async created() {

  },
  mounted() {
  },
  methods: {
    // 获取输入的权限
    getInputPermision() {
      this.$axios.post(
        Api.qualityManage.findApprovePermission,
        {
          data: {},
          nodeProxyId: this.flowNodeProxyId
        },
        (res) => {
          let enableList = [];
          if (res.data && res.data.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeFieldPowerTemplateList || [];
            // flowNodeFieldPowerTemplateList
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
          }
          let mainRule = deepClone(this.mainRule)
          for(let key in mainRule){
            if(enableList.indexOf(key) == -1){
              delete this.mainRule[key]
            }
          }
          this.enableData = enableList
        }
      );
    },
    isDisabled(key) {
      const checkKey = key => {
        const index = this.enableData.findIndex(item => item == key);
        if (index > -1) {
          return false;
        } else {
          return true;
        }
      };
      if (this.operaType == 'edit') {
        // return false;
        return checkKey(key)
      } else {
        if (this.type == 'detail') {
          return true;
        } else {
          return false;
        }
      }
    },
    calculateTotalBudget() {
      let total = 0;
      this.datas.forEach(item => {
        const budget = item.budget || [];
        let temp = budget.reduce((prev, cur) => {
          cur.budgetMoney = cur.budgetMoney || 0
          return math.add(prev , cur.budgetMoney )
        }, 0);
        total = math.add(total,temp)
      });
      this.form.money = total;
    },
    async actionData() {
      this.init();
      if (this.type != 'init') {
        const paramsStr = this.$route.params.str;
        if (paramsStr) {
          localstorageSet('monthDetailParams', paramsStr);
          this.params = JSON.parse(paramsStr);
          await this.getBudgetTypeOfGroup(this.params.companyId);
          this.dataInit();
        } else {
          const str = localstorageGet('monthDetailParams');
          if (str) {
            this.params = JSON.parse(str);
            await this.getBudgetTypeOfGroup(this.params.companyId);
            this.dataInit();
          } else {
            await this.getBudgetTypeOfGroup(this.params.companyId);
            this.dataInit();
          }
        }
      }
    },
    getPermision() {
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      this.$axios.post(
        url,
        {
          data: {
            id: this.flowProxyId // 流程id
          }
        },
        (res) => {
          let enableList = [];
          if (res.data && res.data.flowNodeTemplate && res.data.flowNodeTemplate.childFlowNodeTemplate && res.data.flowNodeTemplate.childFlowNodeTemplate.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeTemplate.childFlowNodeTemplate.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
          }
          let mainRule = deepClone(this.mainRule)
          for(let key in mainRule){
            if(enableList.indexOf(key) == -1){
              delete this.mainRule[key]
            }
          }
          this.enableData = enableList
        }
      );
    },
    getMonthBudgetList() {
      const query = {
        // type: '2',
        stringList:[2,5],
        id: this.id
      };
      return this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        }
      );
    },
    dataInit() {
      this.form = {
        type: 2,
        companyId: this.params.companyId,
        projectId: this.params.projectId,
        month: this.params.budgetTime.substr(0, 7),
        money: math.multiply(this.params.money,10000),
        remarks: this.params.remarks,
        departName: this.getDepartNameByDepartId(this.params.projectId)// this.params.departmentName == '公司领导' ? '公司固定费用' : this.params.departmentName
      };

      this.bizId = this.params.id;
      this.getFileByBizId(this.bizId);

      const datas = [];
      const budgetDetailsVos = this.params.budgetDetailsVos;
      for (let i = 0; budgetDetailsVos[i]; i++) {
        if(budgetDetailsVos[i].budgetTypeVo){
          const departId = budgetDetailsVos[i].departmentId;
          const index = datas.findIndex(item => item.departId == departId);
          const budgetTypeVo = budgetDetailsVos[i].budgetTypeVo;
          const isRelateProj = !!budgetDetailsVos[i].projectId;
          let budgetMoney = budgetDetailsVos[i].money || 0
          const tmp = {
            budgetId: budgetDetailsVos[i].budgetId,
            budgetDetailsId: budgetDetailsVos[i].id,
            budgetTypeId: budgetTypeVo.id,
            budgetType: budgetTypeVo.name,
            isRelateProj,
            relateProjName: budgetDetailsVos[i].projectName,
            relateProjId: budgetDetailsVos[i].projectId,
            budgetMoney: math.multiply(budgetMoney,10000),
            canDelete: false,
            disabled: true
          };
          if (index > -1) {
            datas[index].budget.push(tmp);
          } else {
            const obj = {
              id: this.params.id,
              departName: this.getDepartNameByDepartId(this.params.projectId), // this.params.departmentName,
              departId: this.params.projectId,
              activeNames: '1',
              disabled: true
            };
            obj.budget = [];
            obj.budget.push(tmp);
            datas.push(obj);
          }
        }
      }
      this.datas = datas;
    },
    getDepartNameByDepartId(id) {
      const depart = this.departOptions.find(item => item.id == id);
      let departName = '';
      if (depart) {
        departName = depart.name;
      }
      return departName;
    },
    init() {
      // this.getCompanyTree();
      // this.getParentCompanyList();
      if(!this.departOptions.length)this.companyChange(this.form.companyId) //根据公司id获取公司项目和部门
      this.getProjectVosByCompanyId(this.form.companyId);
    },
    getBudgetTypeOfGroup(companyId) {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            const data = res.data || [];
            const find = data.find(item => item.companyVo.id == companyId);
            if (find) {
              this.centralizedApiVos = find.centralizedApiVos[0];
              this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
              this.generateDepartOption();
              resolve()
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
      })
    },
    generateDepartOption() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo } = item;
        departOptions.push({
          id:sysDepartmentVo.id,
          name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item=>{
        departOptions.push({
          id:item.projectVo.id,
          name:item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject:true
        })
      })
      this.departOptions = departOptions
    },
    getProjectVosByCompanyId(companyId) {
      this.$axios.post(
        Api.annualBudget.getProjectVosByCompanyId,
        {
          data: {
            companyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.projectOptions = res.data;
          }
        }
      );
    },
    async companyChange(val) {
      // this.form.projectId = ''
      // await this.getFlowType()
      await this.getBudgetTypeOfGroup(val);
      this.departOptions.forEach(item => {
        item.hasSelect = false;
      });
    },
    getCompanyById(id) {
      const company = this.originDepartData.find(item => {
        return item.id == id;
      });
      return company;
    },
    departChange(id) {
      this.departInfo = this.departOptions.find(item => {
        return item.id == id;
      });
      // console.log('this.departInfo',this.departInfo)
      // return
      this.form.departName = this.departInfo.name;
      // this.datas[0].departId = id;
      // console.log('this.datas',this.datas)
      const promiseResolve = this.getMonthBudget();
      if (promiseResolve) {
        promiseResolve.then(res => {
          if (res.isSuccess) {
            const data = res.data;
            if (data && data.dataList && data.dataList.length) {
              this.getBudgetListResolve(data.dataList[0]);
            } else {
              // this.getYearBudget();
              this.getBudgetType()
            }
          } else {
            // 选择的月份没有月度预算，则可以新建，需要去年度预算拉取数据，自动填写归口信息
            // this.getYearBudget();
            this.getBudgetType()
          }
        });
      }
    },
    monthChange() {
      const promiseResolve = this.getMonthBudget();
      if (promiseResolve) {
        promiseResolve.then(res => {
          if (res.isSuccess) {
            const data = res.data;
            if (data && data.dataList && data.dataList.length) {
              this.getBudgetListResolve(data.dataList[0]);
            } else {
            // this.getYearBudget();
            this.getBudgetType()
            }
          } else {
            // 选择的月份没有月度预算，则可以新建，需要去年度预算拉取数据，自动填写归口信息
            // this.getYearBudget();
            this.getBudgetType()
          }
        });
      }
    },
    getBudgetListResolve(list) {
      if (list.examineStatus == 2) {
        this.$confirm(`该部门在${this.form.month}的预算已被驳回或撤销，请到待办页面选择重新发起审批`, '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '重新选择',
          type: 'warning'
        }).then(() => {
          // this.type = 'edit';
          // this.params = list;
          // // this.toEditPage(list);
          // this.actionData();
          this.$parent.$parent.$parent.handleClose(true)
          this.$parent.$parent.$parent.$parent.activeName='dueout'
        }).catch(() => {
          this.form.projectId = '';
          this.form.month = '';
        });
      } else {
        if (list.status == 0) {
          this.$confirm(`该部门在${this.form.month}的预算有已保存的草稿，请到待办页面进行编辑`, '提示', {
            closeOnClickModal: false,
            confirmButtonText: '确定',
            cancelButtonText: '重新选择',
            type: 'warning'
          }).then(() => {
            this.$parent.$parent.$parent.handleClose(true)
            this.$parent.$parent.$parent.$parent.activeName='dueout'
            // this.type = 'edit';
            // this.params = list;
            // this.actionData();
            // this.toEditPage(list);
          }).catch(() => {
            this.form.projectId = '';
            this.form.month = '';
          });
        } else if (list.status == 2) {
          this.type = 'edit';
          this.params = list;
          this.actionData();
        } else {
          this.$alert(`该部门在${this.form.month}月度已做预算，请重新选择部门或者月度`, '提示').then(() => {
            this.datas = [];
            this.form.projectId = '';
            this.form.month = '';
          }).catch(() => {
            this.datas = [];
            this.form.projectId = '';
            this.form.month = '';
          });
        }
      }
    },
    //获取部门归口
    getBudgetType(){
          // 获取公司下的部门列表项目列表和归口
      // return new Promise((resolve, reject) => {
        let data = {
          data:{
            annually:this.form.month.substr(0, 4),//new Date().getFullYear(),
            companyId:this.form.companyId,
            departmentId:this.form.projectId,
            stringList:[1,2],
            listString:[1,3]
          }
        }
        this.$axios.post(Api.budgetManage.getBudgetList,data,res=>{
          let list = res?.data?.dataList || []
          let departmentList = []
          list.forEach(item=>{
            let departmentId = item.departmentId
            let index = departmentList.findIndex(it=>it.id == departmentId)
            let name = `${item.name}`
            let child = {
              budgetTypeId:item.id,
              budgetType:name,
              budgetMoney:0,
              }
            if(index == -1){
              departmentList.push({
                id:departmentId,
                departName:item.departmentName,
                activeNames: "1",
                budget:[child]
              })
            }else{
              departmentList[index].budget.push(child)
            }
          })
          this.datas = departmentList
        })
      // })
    },
    async getProjectBudget(query) {
      query.type = 3;
      query.endTime = query.budgetTime;
      delete query.budgetTime;
      await this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        },
        res => {
          if (res.isSuccess) {
            const data = res.data || {};
            this.projectList = data.dataList || [];
          } else {
            // this.$message.error(res.message)
          }
        }
      );
    },
    getMonthBudget() {
      if (this.form.projectId && this.form.month) {
        const query = {
          budgetTime: `${this.form.month}-01 00:00:00`,
          companyId: this.form.companyId,
          projectId: this.form.projectId,
          type: 2 // 月度预算
        };
        if(this.departInfo.isProject)query.type = 5
        return this.$axios.post(
          Api.annualBudget.budgetList,
          {
            data: query
          }
        );
      } else {
        return false;
      }
    },
    toEditPage(list) {
      // let paramStr = {};
      // paramStr = JSON.stringify(list);
      // this.$router.push({
      //   path: '/groupBudgetManage/monthlyBudget/addMonthlyBudget',
      //   name: 'GroupAddMonthlyBudget',
      //   params: { str: paramStr },
      //   query: { type: 'edit' }
      // });
      this.$emit('changeComponent', { list, type: 'edit_depart_monthly_budget' });
    },
    addPlan(index) {
      const plan = {
        budgetType: '',
        isRelateProj: false,
        relateProjName: '',
        relateProjId: '',
        budgetMoney: 0
      };
      this.datas[index].budget.push(plan);
    },
    planDelete(i, j) {
      this.datas[i].budget.splice(j, 1);
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        const v = item || 0;
        val = math.add(val, v - 0)
      });
      return val;
    },
    summary(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (index === 0) {
          // 只找第一列放合计
          sums[index] = '合计:';
          return;
        }
        if (column.property === 'budgetMoney') {
          const values = data.map(item => item[column.property]);
          let total = (this.sumNums(values)).toFixed(2)
          total = this.numberCommas(total)
          sums[index] = '￥' + total;
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },
    /**
     *
     * @param {*} status
     * @param {*} noNeedCheckFlow
     * 获取流程，即调用getFlowFindById方法的条件
     * 1非草稿提交预算 2有对应的流程id 3noNeedCheckFlow 参数为false
     */
    async submit(status, noNeedCheckFlow) {
      // if (this.flowId && status == 1 && !noNeedCheckFlow) await this.getFlowFindById()
      // if (!this.canSubmit && status == 1) return false
      if(this.type == 'init' && !this.datas.length){
        return this.$message.error('公司在该年度暂无预算归口，请先新建该年度公司预算');
      }
      this.$refs.ruleForm.validate(res => {
        if (res) {
          if (!this.datas.length) {
            this.$parent.$parent.$parent.submitLoading = false
            if(this.examneObj)this.examneObj.submitLoading = false
            return this.$message.error('预算不可为空');
          }
          this.form.status = status;
          this.form.processId = '';
          this.form.projectId = this.form.projectId;
          this.form.examineStatus = 0;
          if (this.type == 'edit') this.form.id = this.params.id;
          const budgetDetailsVos = [];
          let totalBueget = 0;
          let childrenSort = 0;
          for (let i = 0; this.datas[i]; i++) {
            const budget = this.datas[i].budget;
            for (let j = 0; budget[j]; j++) {
              const tmpObj = {};
              childrenSort += 1;
              tmpObj.departmentId = this.form.projectId;
              tmpObj.type = this.form.type;
              tmpObj.projectId = budget[j].relateProjId;
              tmpObj.money = budget[j].budgetMoney || 0;
              tmpObj.sort = childrenSort;
              tmpObj.budgetTypeId = budget[j].budgetTypeId;
              if (budget[j].budgetId) tmpObj.budgetId = budget[j].budgetId;
              // if (!tmpObj.money) {
              //   this.$message.error('预算详情金额不可为0');
              //   return false;
              // }
              totalBueget = math.add(totalBueget , (tmpObj.money - 0))
              tmpObj.budgetTypeVo = {};
              const budgetTypeVo = {};
              budgetTypeVo.name = budget[j].budgetType;
              if (budgetTypeVo.name == '') {
                this.$parent.$parent.$parent.submitLoading = false
                if(this.examneObj)this.examneObj.submitLoading = false
                this.$message.error('所有费用预算类型均为必填');
                return false;
              }
              budgetTypeVo.parentId = 1;
              budgetTypeVo.departmentId = this.form.projectId;
              budgetTypeVo.type = this.form.type;
              budgetTypeVo.type = 2
              if(this.departInfo.isProject)budgetTypeVo.type = 5
              budgetTypeVo.annually = this.form.month.substr(0, 4);
              budgetTypeVo.status = 0;// this.form.status
              tmpObj.type = 2
              if(this.departInfo.isProject){
                tmpObj.projectId=this.departInfo.id
                tmpObj.type=5
              }
              tmpObj.budgetTypeVo = budgetTypeVo;
              budgetDetailsVos.push(tmpObj);
            }
          }
          this.form.budgetDetailsVos = budgetDetailsVos;
          this.form.budgetTime = `${this.form.month}-01 00:00:00`;
          const fileId = this.$refs.eleupload.getFileId();
          this.form.enclosure = fileId[0] || '';
          // if (totalBueget.toFixed(6) != this.form.money.toFixed(6)) {
          //   return this.$message.error('预算总金额和预算详情金额不符');
          // }
          this.form.type = 2
          if(this.departInfo.isProject)this.form.type = 5
          if (status == 1) this.sumbitFlow();
          else {
            this.postData();
          }
          // this.postData(this.form);
        }else{
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
        }
      });
    },
    postData(status,batchCode) {
      var data = deepClone(this.form);
      if(this.departInfo.isProject)data.type = 5
      // let action = Api.annualBudget.costBudgetSave;
      // if (this.type == 'edit') action = Api.annualBudget.initBudgetUpdate;
      this.transeMoney(data)
      let action = Api.annualBudget.costBudgetSaveTask
      // if(id)data.id = id
      this.$axios.post(action, { data,batchCode }, res => {
        if (res.isSuccess) {
          let relationId;
          if (this.type == 'edit') {
            this.businessId = relationId = data.id;
            this.businessData = data;
          } else {
            this.businessId = relationId = res.data.id;
            this.businessData = res.data;
          }
          this.businesstotal = data.money;
          if (data.enclosure) {
            const fileId = data.enclosure;
            this.bindFileById(relationId, fileId).then(() => {
              this.processReturnData(data, res);
            });
          } else {
            this.processReturnData(data, res);
          }
        } else {
          this.$parent.$parent.$parent.submitLoading = false
          if(this.examneObj)this.examneObj.submitLoading = false
          this.$message.error(res.message);
        }
      }
      );
    },
    cancel() {
      if (this.type == 'detail') {
        this.$router.go(-1);
      } else {
        this.$confirm('确认取消?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$router.go(-1);
        });
      }
    },
    async processReturnData(data, res) {
      let id = res.data.id;
      if (this.type == 'edit') id = this.params.id;
      let userName = localstorageGet('userName')
      if(this.createrId){
        let userInfo = await this.getUserInfo()
        if(userInfo)userName = userInfo.name
      }
      const name = `${localstorageGet('companyName')}/${this.departInfo.name}${this.form.month}月度预算￥${this.form.money}元-${userName}`;
      if (data.status == 1) {
        this.submitFlowFinal(true, id, '', data.money,name);
        // this.submitFinal(id, data.money).then(response => {
        //   if (response.isSuccess) {
        //     this.$message.success('操作成功');
        //     this.$router.push({ path: '/groupBudgetManage/monthlyBudget/index' });
        //   } else {
        //     this.failSubmitFlow(response, res)
        //   }
        // }).catch(err => {
        //   this.failSubmitFlow(err, res)
        // })
      } else {
        // this.$message.success('操作成功');
        // this.prevStepHandle();
        this.submitFlowFinal(true, id, '', data.money,name,'draft')
      }
    },
    bindFileById(relationId, fileId) {
      const data = {
        relationId,
        fileId
      };
      return this.$axios.post(
        Api.schedule.saveAttachment,
        { data }
      );
    },
    // 根据业务id获取文件
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
          data: {
            relationId: id
          }
        }).then(res => {
        if (res.isSuccess) {
          const list = res.data;
          const attachFile = list.map(item => {
            // return {
            //   id: item.id,
            //   fileName: item.fileName,
            //   fileUrl: item.fileUrl
            // };
            return item
          });
          this.attachFile = attachFile;
        }
      });
    },
    submitFinal(id, total) {
      const param = {
        data: {
          flowProxyId: this.flowId,
          flowInstanceBizRelevanceList: [
            {
              otherBiz: this.selectFlowType,
              otherBizId: id // 保存后返回的id
            }
          ]
        },
        formDataMongoVo: {
          data: {
            money: total,
            initiatorRange: this.$store.state.user.userId
          }
        }
      };
      if (this.checkboxPersonGroup.length) {
        const nextAuditorList = [];
        this.checkboxPersonGroup.map(item => {
          nextAuditorList.push({
            bizId: item.id
          });
        });
        param.nextAuditorList = nextAuditorList;
      }
      if (this.isPrall) {
        const nextAuditorList = [];
        Object.keys(this.parlleNodePerson).forEach(it => {
          this.parlleNodePerson[it].forEach(item => {
            nextAuditorList.push({
              bizId: item.id,
              nodeProxyId: item.nodeProxyId
            });
          });
        });
        param.nextAuditorList = nextAuditorList;
      }

      if (this.needChooseSpecialNode) {
        param.fixedExecuteNodeId = this.branchChoose.nextNodeTemplateId;
      }

      return this.$axios.post(
        Api.schedule.saveFlowInstance,
        param
      );
    },
    // 查出固定的流程id
    async getFlowType() {
      const data = {
        typeId: '',
        flowName: '',
        nextNodeName: '',
        flowNodeType: '',
        nextNodeProxyId: '',
        flowStatus: 'enable',
        auditWay: this.selectFlowType,
        useScope: 'invest'
      };
      await this.$axios.post(
        Api.schedule.getFlowTemplateList,
        {
          data,
          formTemplateBizRelevanceList: [
            // {
            //   otherBiz: 'customerCode',
            //   otherBizId: this.$store.state.user.customerCode
            // },
            {
              otherBiz: 'company',
              otherBizId: this.form.companyId
            }
          ],
          platformCode: '999999',
          ignoreTemplateData: true,
          pagination: true,
          pages: 1,
          size: 1
        },
        res => {
          if (res.isSuccess) {
            if (!res.data.length) {
              this.flowId = null;
              this.$message.error('暂无流程，可保存为草稿');
            } else {
              this.flowId = res.data[0].id; // 流程id
            }
          }
        }
      );
    },
    // async getFlowFindById() {
    //   await this.$axios.post(
    //     Api.schedule.flowTemplateFindById,
    //     {
    //       data: {
    //         id: this.flowId
    //       }
    //     },
    //     res => {
    //       if (res.isSuccess) {
    //         const data = res.data;
    //         this.needChooseSpecialNode = false;
    //         this.isPrall = false;
    //         if (data.flowNodeTemplate && data.flowNodeTemplate.childFlowNodeTemplate) {
    //           if (data.flowNodeTemplate.childFlowNodeTemplate.branchExecuteType == 'custom_choose') { // 手动分支
    //             this.needChooseSpecialNode = true;
    //             const childFlowNodeTemplate = data.flowNodeTemplate.childFlowNodeTemplate;
    //             this.manulNodeList = childFlowNodeTemplate.conditionNodes.map(item => {
    //               return {
    //                 nextNodeTemplateId: item.nextNodeTemplateId,
    //                 nodeName: item?.childFlowNodeTemplate?.nodeName,
    //                 nodeType: item?.childFlowNodeTemplate?.type, // 为处理空节点
    //                 branchName: item.name,
    //                 auditType: item?.childFlowNodeTemplate?.flowNodeAuditConfig?.auditType
    //               };
    //             });
    //             this.branchChooseVisible = true;
    //             this.canSubmit = false;
    //           } else {
    //             if (data.flowNodeTemplate.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') { // 选择人员
    //               this.checkboxPersonGroup = [];
    //               this.nodeChooseVisible = true;
    //               this.canSubmit = false;
    //             } else {
    //               if (data.flowNodeTemplate.childFlowNodeTemplate.type == 'parallel') { // 下一个并行节点，需要判断是否有自选的
    //                 const parallelNodes = data.flowNodeTemplate.childFlowNodeTemplate.parallelNodes;
    //                 const pralleNodeList = [];
    //                 parallelNodes.forEach(item => {
    //                   if (item.childFlowNodeTemplate && item.childFlowNodeTemplate.flowNodeAuditConfig && item.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') {
    //                     pralleNodeList.push({
    //                       nodeName: item.childFlowNodeTemplate.nodeName,
    //                       // nodeId: item.nodeTemplateId
    //                       nodeId: item.nextNodeTemplateId
    //                     });
    //                   }
    //                 });
    //                 if (pralleNodeList.length) { // 并行节点需要选人
    //                   this.canSubmit = false;
    //                   this.isPrall = true;
    //                   this.pralleNodeList = pralleNodeList;
    //                   this.pralleNodeVisible = Boolean(pralleNodeList.length);
    //                 } else { // 并行不需要选人，直接走掉
    //                   this.canSubmit = true;
    //                 }
    //               } else {
    //                 this.canSubmit = true;
    //               }
    //             }
    //           }
    //         }
    //       }
    //     }
    //   );
    // },
    getSelectPerson(data) {
      if (this.isPrall) { // 并行选人员
        const tmp = data.checkboxPersonGroup.map(item => {
          return {
            name: item.name,
            id: item.id,
            nodeProxyId: this.currentParallNodeId
          };
        });
        this.$set(this.parlleNodePerson, this.currentParallNodeId, tmp);
      } else {
        this.checkboxPersonGroup = data.checkboxPersonGroup.map(item => {
          const obj = {
            name: item.name,
            id: item.id
          };
          if (this.needChooseSpecialNode) {
            obj.nextNodeTemplateId = this.nextNodeTemplateId;
          }
          return obj;
        });
        if (!this.needChooseSpecialNode) {
          this.canSubmit = true;
          this.submit(1, true); // 直接提交，不需要获取流程数据
        }
      }
    },
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
    },
    updateStatusWhenFlowFail(data) { // 流程发起失败后，修改业务状态为草稿
      data.status = 0;
      const action = Api.annualBudget.initBudgetUpdate;
      this.$axios.post(action, { data }, res => {
        if (res.isSuccess) {
          this.processReturnData(data);
        } else {
          this.$message.error(res.message);
        }
      });
    },
    failSubmitFlow(response, res) {
      // 如果提交审核失败，调用更新接口，把预算状态改成草稿
      this.$message.error(response.message + '，预算将转成草稿状态，你可以再次提交审核');
      this.updateStatusWhenFlowFail(res.data);
    },
    showSelectPerson(nextNodeTemplateId) {
      this.nextNodeTemplateId = nextNodeTemplateId;
      this.nodeChooseVisible = true;
    },
    saveBranchChooseNode(data) {
      if (data.auditType == 'run_node_choose') {
        if (!this.checkboxPersonGroup.length) {
          return this.$message.warning('该分支需选择审批人，请先选择');
        }
      }
      this.branchChoose = data;
      this.branchChooseVisible = false;
      this.canSubmit = true;
      this.submit('1', true);
    },
    clearCheckboxPersonGroup() {
      this.checkboxPersonGroup = [];
    },
    parlleChoosePerson(nodeId) {
      this.currentParallNodeId = nodeId;
      this.nodeChooseVisible = true;
    },
    parlleSubmit() {
      let hasChooseAll = true;
      this.pralleNodeList.forEach(item => {
        const nodeId = item.nodeId;
        if (!this.parlleNodePerson[nodeId]) {
          hasChooseAll = false;
        } else {
          if (this.parlleNodePerson[nodeId].length <= 0) {
            hasChooseAll = false;
          }
        }
      });
      if (!hasChooseAll) {
        return this.$message.warning('每个节点均需要选择审批人');
      }
      this.pralleNodeVisible = false;
      this.canSubmit = true;
      this.submit('1', true);
    },
    flowSubmit() {
      this.form.status = 1;
      this.form.processId = '';
      this.form.projectId = this.form.projectId;
      this.form.examineStatus = 0;
      this.form.id = this.params.id;
      const budgetDetailsVos = [];
      let totalBueget = 0;
      const i = 0;
      const budget = this.datas[i].budget;
      let childrenSort = 0;
      for (let j = 0; budget[j]; j++) {
        const tmpObj = {};
        childrenSort += 1;
        tmpObj.departmentId = this.form.projectId;
        tmpObj.type = this.form.type;
        tmpObj.projectId = budget[j].relateProjId;
        tmpObj.money = budget[j].budgetMoney || 0;
        tmpObj.budgetTypeId = budget[j].budgetTypeId;
        tmpObj.sort = childrenSort;
        if (budget[j].budgetId) tmpObj.budgetId = budget[j].budgetId;
        // if (!tmpObj.money) {
        //   this.$message.error('预算详情金额不可为0');
        //   return false;
        // }
        totalBueget = math.add(totalBueget ,(tmpObj.money - 0))
        tmpObj.budgetTypeVo = {};
        const budgetTypeVo = {};
        budgetTypeVo.name = budget[j].budgetType;
        if (budgetTypeVo.name == '') {
          this.$message.error('所有费用预算类型均为必填');
          return false;
        }
        budgetTypeVo.parentId = 1;
        budgetTypeVo.departmentId = this.form.projectId;
        budgetTypeVo.type = this.form.type;
        budgetTypeVo.type = 2
        if(this.departInfo.isProject)budgetTypeVo.type = 5
        budgetTypeVo.annually = this.form.month.substr(0, 4);
        budgetTypeVo.status = 0;// this.form.status
        tmpObj.budgetTypeVo = budgetTypeVo;
        tmpObj.type=2
        if(this.departInfo.isProject){
          tmpObj.projectId=this.departInfo.id
          tmpObj.type=5
        }
        budgetDetailsVos.push(tmpObj);
      }
      this.form.budgetDetailsVos = budgetDetailsVos;
      this.form.budgetTime = `${this.form.month}-01 00:00:00`;
      const fileId = this.$refs.eleupload.getFileId();
      this.form.enclosure = fileId[0] || '';
      if (totalBueget.toFixed(6) != this.form.money.toFixed(6)) {
        return this.$message.error('预算总金额和预算详情金额不符');
      }
      this.form.type = 2
      if(this.departInfo.isProject)this.form.type = 5
      // const action = Api.annualBudget.initBudgetUpdate;
      let action = Api.annualBudget.costBudgetSaveTask
      let data = deepClone(this.form)
      this.transeMoney(data)
      this.$axios.post(action, { data }, async res => {
        const fileId = this.form.enclosure;
        this.bindFileById(this.form.id, fileId);
        let name = localstorageGet('userName')
        if(this.createrId){
          let userInfo = await this.getUserInfo()
          console.log('userInfo',userInfo)
          if(userInfo)name = userInfo.name
        }
        const obj = {
          status: 'success',
          val: this.busVal,
          total: this.form.money,
          id: this.form.id,
          name : `${this.params.companyName || localstorageGet('companyName')}/${this.departInfo.name}${this.form.month}月度预算￥${this.form.money}元-${name}`
        };
        if (!res.isSuccess) {
          obj.status = 'fail';
        }
        this.$bus.$emit('submitBeforeHandleOk', obj);
      });
    },
    getUserInfo(){
      let data = {
        id: this.createrId,
        flag: "company"
      }
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.user.getUserInfoById,{data},res=>{
          if(res.isSuccess){
            resolve(res.data)
          }
        })
      })
    },
    numberCommas(x){
      if(x){
        var res = x.toString().replace(/\d+/, function (n) {
          return n.replace(/(\d)(?=(\d{3})+$)/g, function ($1) {
            return $1 + ",";
          });
        });
        return res;
      }else{
        return x
      }
    },
    //金额转成万元提交
    transeMoney(data){
      var divide = 10000
      data.money = math.divide(data.money,divide)
      let budgetDetailsVos = data?.budgetDetailsVos || []
      budgetDetailsVos.forEach(el=>{
        el.money = math.divide(el.money,divide)
      })
      let costBudgetVoList = data?.costBudgetVoList || []
      costBudgetVoList.forEach(el=>{
        el.money = math.divide(el.money,divide)
        let budgetDetailsVos = el?.budgetDetailsVos
        budgetDetailsVos.forEach(item=>{
          item.money = math.divide(item.money,divide)
        })
      })
    },
  }
};
</script>
<style lang="scss" scoped src="@/views/BudgetManage/CompanyBudget/components/style/style.scss"></style>
