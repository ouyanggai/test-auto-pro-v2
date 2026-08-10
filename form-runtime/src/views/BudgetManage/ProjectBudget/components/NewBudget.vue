<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>{{ title }}</h2>
      <div class="inner-container">
        <el-radio-group size="medium" v-model="tab" v-if="tabList.length" @change="radioChange"
          style="margin-bottom: 10px;">
          <el-radio-button label="0">项目预算</el-radio-button>
          <el-radio-button v-for="(val, index) in tabList" :label="(val['index'] + 1)" :key="index">{{ val['label'] }}
          </el-radio-button>
        </el-radio-group>
        <el-card class="box-card">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <!-- <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule' :disabled="type == 'detail'"> -->
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row style="margin-bottom:30px">
              <el-col :span="8">
                <el-form-item label="项目名称：" prop="projectId">
                  <el-input v-model="form.projectName" :disabled="isDisabled('project')" v-if="type == 'detail'">
                  </el-input>
                  <el-select v-model="form.projectId" placeholder="请选择" @change="changeProject" v-else
                    :disabled="isDisabled('project') || operaType == 'reEdit'">
                    <el-option v-for="(item, index) in projectOptions" :key="index" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="7">
                <el-form-item label="预算金额(万)：" prop="money">
                  <el-input-number v-model="form.money" :min="0.000000" :precision="6" :step="0.1" :controls="false"
                    :disabled="isDisabled('money') || tab > 0">
                  </el-input-number>
                </el-form-item>
              </el-col>

              <el-col :span="9">
                <el-form-item label="项目周期：" prop="dateRange">
                  <el-date-picker v-model="form.dateRange" type="daterange" range-separator="-" start-placeholder="开始"
                    end-placeholder="结束" value-format="yyyy-MM-dd" :disabled="isDisabled('dateRange') || tab > 0">
                  </el-date-picker>
                  <!-- <el-input v-model="form.money"></el-input> -->
                </el-form-item>
              </el-col>
            </el-row>
            <el-row v-if="type == 'append' || (type == 'edit' && editType == 'appendEdit') || tab > 0"
              style="margin-bottom:30px">
              <el-col :span="8">
                <el-form-item label="追加金额(万)：" prop="appendMoney">
                  <el-input-number v-model="form.appendMoney" :min="0.000000" :precision="6" :step="0.1"
                    :controls="false" :disabled="isDisabled('appendMoney')"></el-input-number>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="追加年度：" prop="appendYear">
                  <el-date-picker v-model="form.appendYear" type="year" value-format="yyyy" :disabled="true">
                  </el-date-picker>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px">
              <el-col :span="12">
                <el-form-item label="预算金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" :disabled="isDisabled('remarks')" >
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row>
              <el-col :span="24">
                <eleupload ref="eleupload" v-if="type == 'init' || type == 'append'"></eleupload>
                <eleupload ref="eleupload" v-else-if="type == 'detail'" :showOnly="true" :attachFile="attachFile">
                </eleupload>
                <eleupload ref="eleupload" v-else-if="type == 'edit'" :attachFile="attachFile"
                  @clearAllFile="clearAllFile">
                </eleupload>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
        <el-card class="box-card" style="margin-top:30px" v-for="(value, key, index) in datas" :key="index">
          <div>
            <h3 style="display:inline-block;margin-bottom:20px;">
              <span v-if="key == 'budgetDetailsVos'">费用预算计划</span>
              <span v-else>追加当年公司预算</span>
            </h3>
            <el-date-picker v-model="form.companyAppendAnnual" type="year" style="margin-left:30px;width: 100px;"
              v-if="key == 'companyBudgetVos'" value-format="yyyy" :disabled="true">
            </el-date-picker>
            <el-button style="margin-left:30px;" type="primary" icon="el-icon-plus" @click="addDepartPlan(key)"
              v-show="type != 'detail'">新增</el-button>
          </div>
          <!-- <el-card v-for="(val, index) in value" :key="index" style="margin-bottom:20px;position:relative;" -->
          <el-card style="margin-bottom:20px;position:relative;" shadow="hover" v-for="(val, index) in value"
            :key="index">
            <template slot="header">
              <div class="card-title" @click.stop.prevent="() => { }">
                <!-- <el-select v-model="val.departmentId" placeholder="请选择" style="margin-right:1id0%;"
                  @change="departChange(key, index)" :disabled="type == 'detail' || val['isOrigin']"> -->
                <el-select v-model="val.departmentId" placeholder="请选择" style="margin-right:10%;"
                  @change="departChange(key, index)" :disabled="isDisabled('department', val['isOrigin'])">
                  <!-- <el-select v-model="val.departmentId" placeholder="请选择" style="margin-right:10%;"
                    @change="departChange(key, index)"> -->
                  <el-option v-for="(item, index) in departOptions[key]" :key="index" :label="item.departmentName"
                    :value="item.id" :disabled="item.hasSelect">
                  </el-option>
                </el-select>
              </div>
            </template>
            <div class="el-icon-close" @click="deleteDepartPlan(index, key)" v-show="type != 'detail'">
            </div>
            <el-collapse v-model="val['activeNames']">
              <el-collapse-item name="1">
                <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                  <el-table-column type="index" label="编号" width="65px">
                  </el-table-column>
                  <el-table-column prop="name" label="费用预算类型（一级）" class-name="budgetType">
                    <template slot-scope="scope">
                      <!-- <el-input v-model="scope.row.name" :disabled="type == 'detail' || scope.row.isOrigin"></el-input> -->
                      <el-input v-model="scope.row.name" :disabled="isDisabled('name', scope.row.isOrigin)" maxlength="20"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column prop="appendMoney" label="本次追加(万)"
                    v-if="(type == 'append' || (type == 'edit' && editType == 'appendEdit'))">
                    <template slot-scope="scope">
                      <el-input-number :min="0.000000" :precision="6" :step="0.1" :controls="false"
                        v-model="scope.row.appendMoney" :disabled="isDisabled('detailAppendMoney')">
                      </el-input-number>
                    </template>
                  </el-table-column>
                  <el-table-column prop="money" label="预算总额(万)" v-if="type == 'append'">
                    <template slot-scope="scope">
                      <el-input-number :min="0.000000" :precision="6" :step="0.1" :controls="false"
                        :disabled="type == 'detail' || type == 'append' || editType == 'appendEdit'"
                        :value="(scope.row.money - 0)"></el-input-number>
                    </template>
                  </el-table-column>
                  <el-table-column prop="money" label="预算金额(万)" v-else>
                    <template slot-scope="scope">
                      <el-input-number :min="0.000000" :precision="6" :step="0.1" :controls="false"
                        v-model="scope.row.money" :disabled="isDisabled('detailMoney')"></el-input-number>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="80px">
                    <template slot-scope="scope">
                      <el-button @click="planDelete(key, index, scope.$index)" type="text" size="small"
                        v-show="type != 'detail' && !scope.row.isOrigin">
                        <i class="el-icon-delete-solid delete-icon"></i>
                      </el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(key, index)" v-show="type != 'detail'"></i>
              </el-collapse-item>
            </el-collapse>
          </el-card>
        </el-card>

      </div>
      <!-- <div class="footer-bt" v-show="isFlow && !isExamine || type != 'detail'"> -->
      <div class="footer-bt" v-show="type != 'detail' && operaType !='reEdit' && isFlow && !isExamine">
        <div class="footer-inner">
          <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
          <el-button @click="prevStepHandle" plain>取 消</el-button>
          <el-button type="primary" plain @click="submit(0)">保 存</el-button>
          <el-button type="primary" @click="submit(1)">提 交</el-button>
        </div>
      </div>
      <div class="footer-bt" v-show="(type == 'init' || type == 'edit' || type == 'append')  && !isFlow">
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
  </div>
</template>

<script>
import eleupload from '@/components/EleUpload';
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import Api from '@/api';
import { deepClone } from '@/utils/index';
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog';
import BranchChoose from '@/views/BudgetManage/CompanyBudget/components/branchChoose';
import BranchPralleChoose from '@/views/BudgetManage/CompanyBudget/components/branchPralleChoose';
const NUMCN = [
  '一', '二', '三', '四', '五', '六', '七', '八', '九', '十'
];
export default {
  name: 'AddProjectBudget',
  components: { eleupload, PersonSelectDialog, BranchChoose, BranchPralleChoose },
  props: ['operaType', 'id', 'param', 'flowProxyId', 'flowNodeProxyId', 'showType', 'selectFlowType', 'isExamine'],
  data() {
    const PLANTEMP = {
      name: '',
      isRelateProj: false,
      relateProjName: '',
      relateProjId: '',
      appendMoney: 0,
      money: 0
    };
    return {
      mainRule: {
        projectId: [{ required: true, message: '请选择项目', trigger: 'change' }],
        dateRange: [{ required: true, message: '请选择预算时间', trigger: 'change' }],
        money: [{ required: true, message: '请输入预算金额', trigger: 'blur' }, {
          pattern: /^(?!(0[0-9]{0,}$))[0-9]{1,}[.]{0,}[0-9]{0,}$/,
          message: '预算金额要大于0',
          trigger: 'blur'
        }],
        appendMoney: [{ required: true, message: '请输入追加金额', trigger: 'blur' }, {
          pattern: /^(?!(0[0-9]{0,}$))[0-9]{1,}[.]{0,}[0-9]{0,}$/,
          message: '预算金额要大于0',
          trigger: 'blur'
        }],
        appendYear: [{ required: true, message: '请选择追加年份', trigger: 'blur' }]
      },
      form: {
        companyId: localstorageGet('companyId'),
        annual: new Date().getFullYear(),
        companyAppendAnnual: new Date().getFullYear() + '',
        money: '',
        remarks: '',
        // status: '', //0草稿1提交
        // examineStatus: '',
        type: '3',
        dateRange: '',
        projectId: '',
        appendYear: '',
        projectName: ''
      },
      title: '项目预算',
      dateOption: [],
      planTemp: PLANTEMP,
      datas: {
        budgetDetailsVos: [{
          departName: '',
          departmentId: '',
          activeNames: '0',
          budget: [
            deepClone(PLANTEMP)
          ]
        }], // 费用预算
        companyBudgetVos: [{
          departName: '',
          departmentId: '',
          activeNames: '1',
          budget: [
            deepClone(PLANTEMP)
          ]
        }]
      },
      companyOption: [],
      departOptions: [],
      projectOptions: [],
      type: this.showType || 'init',
      tab: '',
      tabList: [],
      editType: '', // appendEdit 追加编辑  initEdit 初始编辑
      hasInfo: false,
      // selectFlowType: 'project_setup_budget',

      attachFile: [],

      nodeChooseVisible: false,
      checkboxPersonGroup: [],
      flowId: '',
      enableData: [],

      // flowId: '',
      branchChooseVisible: false,
      manulNodeList: [],

      pralleNodeVisible: false,
      pralleNodeList: [],
      parlleNodePerson: {}
    };
  },
  inject: {
    prevStepHandle: { value: 'prevStepHandle', default: 'cancel' },
    sumbitFlow: { value: 'sumbitFlow', default: null },
    submitFlowFinal: { value: 'submitFlowFinal', default: null }
  },
  created() {
    window.abb = this
    // saveDaft
    // this.
  },
  computed: {
    isFlow() {
      // 是否是流程回显 查看 审核
      if (this.operaType || this.showType) {
        return true;
      } else {
        return false;
      }
    },
    isDisabled() {
      const checkKey = key => {
        const index = this.enableData.findIndex(item => item == key);
        if (this.isExamine) {
          if (index > -1) {
            return false;
          } else {
            return true;
          }
        } else {
          if (this.type == 'edit') {
            return false;
          } else if (this.type == 'detail') {
            return true;
          }
        }
      };
      return (key, isOrigin) => {
        if (this.isFlow) {
          if (this.tabList.length) { // 有追加的情况
            if (this.tab == this.tabList.length) { // 最后一次追加
              return checkKey(key);
            } else {
              return true;
            }
          } else {
            return checkKey(key);
          }
        } else {
          if (this.type == 'init') {
            return false;
          } else if (this.type == 'detail') {
            return true;
          } else if (this.type == 'append') {
            if (['project', 'money', 'dateRange', 'detailMoney'].indexOf(key) > -1) {
              return true;
            }
          } else if (this.type == 'edit') {
            if (this.editType == 'appendEdit') {
              // if (['department', 'project', 'money', 'dateRange', 'detailMoney'].indexOf(key) > -1) {
              if (['project', 'money', 'dateRange', 'detailMoney'].indexOf(key) > -1) {
                return true;
              } else {
                return false;
              }
            }
          } else {
            if (isOrigin || key == 'project') {
              return true;
            } else {
              return false;
            }
          }
        }
      };
    }
  },
  watch: {
    // $route: { //
    //   async handler() {
    //     await this.getDepartByCompanyId(this.form.companyId);
    //     this.getAllCompany();
    //     this.type = this.$route.query.type || 'init';
    //     let paramsStr = this.$route.params.str;
    //     if (paramsStr) {
    //       let params = this.params = JSON.parse(paramsStr);
    //       let appendCostBudgetVos = params.appendCostBudgetVos;
    //       this.editTypeData(appendCostBudgetVos, params);
    //     }
    //   },
    //   // immediate: true,
    //   deep: true
    // },
    isFlow: {
      async handler() {
        this.type = this.showType || 'init';
        if (this.isFlow) {
        // 监听提交按钮，做编辑操作
          this.$bus.$off('project_setup_budget_before_handle');
          this.$bus.$on('project_setup_budget_before_handle', val => {
            this.busVal = val;
            this.flowSubmit();
          });
          // 流程回显 查看 审核
          let params = this.param;// res.data.dataList[0];
          if (!params.id) {
            var res = await this.getProjectBudgetById();
            // console.log('res',res)
            params = res.data.dataList[0];
            this.type = 'detail';
          }
          // if (!params.id) {
          //   this.$message.error('暂无流程数据');
          //   return false;
          // }
          this.$emit('updateParams', params);
          this.form.companyId = params.companyId;

          this.getAllCompany();

          if (this.operaType) this.type = 'edit';// this.showType;
          this.params = params;
          const appendCostBudgetVos = params.appendCostBudgetVos || [];
          this.tabList = appendCostBudgetVos.map((item, i) => {
            const obj = {
              label: `第${NUMCN[i]}次追加`,
              index: i
            };
            return obj;
          });
          this.tab = this.tabList.length;
          await this.getDepartByCompanyId(this.form.companyId);
          // if (this.flowNodeProxyId) this.getPermision()
          this.title = '项目预算';
          if (this.operaType == 'check') {
            this.type = 'detail';
            this.detailDataInit(params);
          } else if (this.operaType == 'edit' || this.operaType == 'reEdit') {
            this.type = 'edit';
            this.title = '项目预算编辑';
            this.detailDataInit(params);
          }
          if (this.showType == 'append') {
            this.type = 'edit';// this.showType
            this.title = '追加项目预算';
            const appendCostBudgetVos = params.appendCostBudgetVos;
            this.editTypeData(appendCostBudgetVos, params);
          }
        } else {
          this.actionData();
        }
      },
      immediate: true
    }
  },
  methods: {
    async actionData() {
      // this.type = this.$route.query.type || 'init';
      if (this.$route.query.type) {
        this.type = this.$route.query.type;
        const paramsStr = this.$route.params.str;
        this.params = JSON.parse(paramsStr);
      }
      if (this.type == 'init') { // 新建
        this.title = this.$route.meta.title = '项目立项预算';
        if (this.selectFlowType == 'add_project_budget') {
          this.title = '追加项目预算';
        }
        this.form.companyId = '';
        this.getAllCompany(); // 获取项目列表
      } else { // 编辑 追加
        // let paramsStr = this.$route.params.str;
        // if (!paramsStr) {
        //   // 本地持久化
        //   paramsStr = localstorageGet('projectDetailParams');
        // }
        // if (paramsStr) {
        // localstorageSet('projectDetailParams', paramsStr);
        const params = this.params; // = JSON.parse(paramsStr);
        this.form.companyId = this.params.companyId;
        await this.getDepartByCompanyId(this.form.companyId);
        this.getAllCompany();
        const appendCostBudgetVos = params.appendCostBudgetVos;
        switch (this.type) {
          case 'append':
            // 追加项目预算
            this.title = this.$route.meta.title = '追加项目预算';
            this.dataInit(params);
            break;
          case 'detail':
            // 查看
            this.title = this.$route.meta.title = '项目预算';
            this.tab = 0;
            this.tabList = appendCostBudgetVos.map((item, i) => {
              const obj = {
                label: `第${NUMCN[i]}次追加`,
                index: i
              };
              return obj;
            });
            this.detailDataInit(params);
            break;
          case 'edit':
            // 编辑
            this.editTypeData(appendCostBudgetVos, params);
        }
        // }
        // this.$once('hook:beforeDestroy', () => {
        //   localstorageRemove('projectDetailParams');
        //   });
      }
    },
    getPermision() {
      this.$axios.post(
        Api.qualityManage.findApprovePermission,
        {
          data: {

          },
          nodeProxyId: this.flowNodeProxyId
        },
        (res) => {
          let enableList = [];
          if (res.data && res.data.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
          }
          this.enableData = enableList;
        }
      );
    },
    async getProjectBudgetById() {
      const query = {
        id: this.param.id
      };
      if (this.operaType)query.id = this.id;
      return await this.$axios.post(
        Api.annualBudget.findBudgetById,
        {
          data: query
        }
      );
    },
    editTypeData(appendCostBudgetVos, params) {
      if (!appendCostBudgetVos.length) { // 没有追加
        this.editType = 'initEdit';
        this.title = this.$route.meta.title = '项目立项预算';
        this.bizId = params.id;
        this.formData = params;
        // this.selectFlowType = 'project_setup_budget'
        this.dataInit(params);
      } else {
        this.editType = 'appendEdit';
        this.title = this.$route.meta.title = '追加项目预算';
        // this.selectFlowType = 'add_project_budget'
        // this.dataInit(params);
        let nopassIndex = appendCostBudgetVos.findIndex(el => el.examineStatus == 2);
        if (nopassIndex == -1) {
          nopassIndex = appendCostBudgetVos.length - 1;
        }
        const currentAppendCostBudgetVos = this.formData = appendCostBudgetVos[nopassIndex];
        currentAppendCostBudgetVos.projectId = params.projectId;
        currentAppendCostBudgetVos.endTime = params.endTime;
        currentAppendCostBudgetVos.budgetTime = params.budgetTime;
        currentAppendCostBudgetVos.projectName = params.projectName;
        currentAppendCostBudgetVos.companyId = params.companyId;
        currentAppendCostBudgetVos.appendMoney = currentAppendCostBudgetVos.money;
        currentAppendCostBudgetVos.money = params.money;
        currentAppendCostBudgetVos.companyAppendAnnual = currentAppendCostBudgetVos.companyBudgetVos[0].appendBudgetDetailsVos[0].budgetTypeVo.annually;
        currentAppendCostBudgetVos.appendYear = currentAppendCostBudgetVos.appendBudgetDetailsVos[0]?.budgetTypeVo?.annually || currentAppendCostBudgetVos.companyAppendAnnual;

        this.createFormData(currentAppendCostBudgetVos); // 生成页面上半部分数据
        // 初始化this.datas
        const copyParams = deepClone(params);
        // 先删除当前需要编辑的那一项，算好总价格
        copyParams.appendCostBudgetVos.splice(nopassIndex, 1);
        const companyBudgetVosParams = copyParams.companyBudgetVos[0];
        if (companyBudgetVosParams && companyBudgetVosParams.budgetDetailsVos) {
          this.datas.companyBudgetVos = this.createDatas(companyBudgetVosParams.budgetDetailsVos, 'companyBudgetVos');
          this.form.companyBudgetVosId = companyBudgetVosParams.id;
        }
        // 处理项目预算部分
        const budgetDetailsVos = copyParams.budgetDetailsVos;
        this.datas.budgetDetailsVos = this.createDatas(budgetDetailsVos, 'budgetDetailsVos');
        // 把需要编辑的追加数据加入合并入this.datas
        // let appendBudgetDetailsVos = currentAppendCostBudgetVos.appendBudgetDetailsVos //项目的
        // 公司
        this.checkAppendBudget(copyParams.appendCostBudgetVos, 'companyBudgetVos');
        // 项目
        this.checkAppendBudget(copyParams.appendCostBudgetVos, 'budgetDetailsVos');
        this.checkAppendBudget([currentAppendCostBudgetVos], 'companyBudgetVos', true);
        this.checkAppendBudget([currentAppendCostBudgetVos], 'budgetDetailsVos', true);
        this.bizId = currentAppendCostBudgetVos.id;
      }
      this.getFileByBizId(this.bizId);
    },
    async changeProject(projectId) {
      const index = this.projectOptions.findIndex(el => el.id == projectId);
      if (index > -1) {
        const companyId = this.projectOptions[index].companyId;
        await this.getProjectBudget();
        await this.getDepartByCompanyId(companyId);
        if (this.form.companyId != '') {
          if (this.form.companyId != companyId) {
            // 清空所有部门
            this.form.companyId = companyId;
            this.$message.info('所选项目所在公司和原项目所在公司不同，部门信息将被清除');
            for (const k in this.datas) {
              this.datas[k].forEach(item => {
                item.departName = '';
                item.departmentId = '';
              });
            }
          }
        } else {
          this.form.companyId = companyId;
        }
        // await this.getFlowType()
      } else {

      }
    },
    async getProjectBudget() {
      const currentYear = new Date().getFullYear();
      const query = {
        endTime: `${currentYear}-08-31 00:00:00`,
        type: 3,
        projectId: this.form.projectId
      };
      await this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        },
        res => {
          if (res.isSuccess) {
            const data = res.data;
            if (data && data.dataList && data.dataList.length) {
              const list = data.dataList[0];
              // console.log('list',list)
              var appendCostBudgetVos = list.appendCostBudgetVos;
              if (list.status == 0) { // 草稿
                if (appendCostBudgetVos.length > 0 && this.selectFlowType == 'add_project_budget') { // 追加的编辑
                  this.type = 'edit';
                  this.params = list;
                  this.actionData();
                } else if (appendCostBudgetVos.length <= 0 && this.selectFlowType == 'project_setup_budget') { // 新建的编辑
                  this.type = 'edit';
                  this.params = list;
                  this.actionData();
                } else {
                  var msg = '项目已有立项预算';
                  if (appendCostBudgetVos.length <= 0)msg = '请先新建项目立项预算';
                  this.$alert(msg, '提示');
                  this.form.projectId = '';
                }
              } else {
                if (list.examineStatus == 1) { // 审核通过的
                  if (appendCostBudgetVos.length >= 0 && this.selectFlowType == 'add_project_budget') { // 正常追加
                    var hasAppendEdit = false;
                    appendCostBudgetVos.forEach(item => {
                      if (item.examineStatus == 0)hasAppendEdit = true;
                    });
                    if (hasAppendEdit) {
                      this.type = 'edit';
                    } else {
                      this.type = 'append';
                    }
                    this.params = list;
                    this.actionData();
                  } else if (appendCostBudgetVos.length >= 0 && this.selectFlowType == 'project_setup_budget') {
                    this.form.projectId = '';
                    this.$alert('项目已有立项预算', '提示');
                  }
                } else if (list.examineStatus == 0) {
                  if (appendCostBudgetVos.length >= 1) { // 有追加审核或者草稿
                    var hasAppendDraft = false;
                    appendCostBudgetVos.forEach(item => {
                      if (item.status == 0)hasAppendDraft = true;
                    });
                    if (hasAppendDraft) {
                      if (this.selectFlowType == 'project_setup_budget') {
                        this.form.projectId = '';
                        this.$alert('项目已有立项预算', '提示');
                      } else {
                        this.type = 'edit';
                        this.params = list;
                        this.actionData();
                      }
                    } else {
                      this.form.projectId = '';
                      this.$alert('预算正在审核中', '提示');
                    }
                  } else {
                    if (list.status == 0) { // 草稿，直接载入
                      this.type = 'edit';
                      this.params = list;
                      this.actionData();
                    } else {
                      this.form.projectId = '';
                      this.$alert('预算正在审核中', '提示');
                    }
                    // console.log('this.selectFlowType',this.selectFlowType)
                    // if(this.selectFlowType == 'project_setup_budget'){

                    // }
                  }
                }
              }
              this.hasInfo = true;
            } else {
              if (this.selectFlowType == 'add_project_budget') {
                this.form.projectId = '';
                this.$alert('请先新建项目立项预算', '提示');
              }
              this.hasInfo = false;
            }
          } else {
            this.hasInfo = false;
          }
        }
      );
    },
    toEditPage(list, type) {
      this.$emit('changeComponent', { list, type: type || 'edit_project_setup_budget' });
    },
    radioChange(val) {
      const len = this.tabList.length; let isShowLog = false;
      this.detailDataInit(this.params);

      if (this.isFlow) { // 在流程里面，需要处理是否显示流程日志的问题
        if (val == len) {
          isShowLog = true;
        } else {
          isShowLog = false;
        }
        this.$emit('showLog', isShowLog);
      }
    },
    detailDataInit(params) {
      if (this.tab == 0) {
        this.title = '项目立项预算';
        this.createFormData(params);
        this.bizId = params.id;
        // 先处理公司追加部分
        const companyBudgetVosParams = params.companyBudgetVos[0];
        if (companyBudgetVosParams && companyBudgetVosParams.budgetDetailsVos) {
          this.datas.companyBudgetVos = this.createDatas(companyBudgetVosParams.budgetDetailsVos, 'companyBudgetVos');
          this.form.companyAppendAnnual = companyBudgetVosParams.budgetTime.substr(0, 4);
          this.form.companyBudgetVosId = companyBudgetVosParams.id;
        } else {
          this.datas.companyBudgetVos = [];
        }
        // 处理项目预算部分
        const budgetDetailsVos = params.budgetDetailsVos;
        this.datas.budgetDetailsVos = this.createDatas(budgetDetailsVos, 'budgetDetailsVos');
      } else {
        this.title = '追加项目预算';
        // appendMoney
        this.form.dateRange = [
          params.budgetTime.substr(0, 10),
          params.endTime.substr(0, 10)
        ];
        this.form.projectId = params.projectId;
        this.form.projectName = params.projectName;
        const appendCostBudgetVos = this.params.appendCostBudgetVos[this.tab - 1];
        this.form.money = appendCostBudgetVos.money;
        this.form.remarks = appendCostBudgetVos.remarks;
        if (appendCostBudgetVos.appendBudgetDetailsVos.length) {
          this.form.companyAppendAnnual = appendCostBudgetVos.appendBudgetDetailsVos[0].budgetTypeVo.annually || appendCostBudgetVos.companyBudgetVos[0].budgetTime.substr(0, 4);// .budgetTypeVo.annually
        } else {
          this.form.companyAppendAnnual = appendCostBudgetVos.companyBudgetVos[0].budgetTime.substr(0, 4);// .budgetTypeVo.annually
        }
        this.form.appendMoney = appendCostBudgetVos.money;
        // if(appendCostBudgetVos.appendBudgetDetailsVos && appendCostBudgetVos.appendBudgetDetailsVos.length){
        this.form.appendYear = new Date().getFullYear() + '';// appendCostBudgetVos.appendBudgetDetailsVos[0].budgetTypeVo.annually//this.form.companyAppendAnnual
        // }else{

        // }

        this.bizId = appendCostBudgetVos.id;
        if (appendCostBudgetVos.companyBudgetVos.length) {
          this.datas.companyBudgetVos = this.createDatas(appendCostBudgetVos.companyBudgetVos[0].appendBudgetDetailsVos, 'companyBudgetVos');
          this.form.companyBudgetVosId = appendCostBudgetVos.companyBudgetVos[0].id;
        }
        this.datas.budgetDetailsVos = this.createDatas(appendCostBudgetVos.appendBudgetDetailsVos, 'budgetDetailsVos');
      }
      this.getFileByBizId(this.bizId);
    },
    dataInit(params) { // 追加最外面一层的数据需要变化
      this.createFormData(params);
      // 先处理公司追加部分
      const companyBudgetVosParams = params.companyBudgetVos[0];
      if (companyBudgetVosParams && companyBudgetVosParams.budgetDetailsVos) {
        this.datas.companyBudgetVos = this.createDatas(companyBudgetVosParams.budgetDetailsVos, 'companyBudgetVos');
        this.form.companyAppendAnnual = companyBudgetVosParams.budgetTime.substr(0, 4);
        this.form.companyBudgetVosId = companyBudgetVosParams.id;
      }
      // 处理项目预算部分
      const budgetDetailsVos = params.budgetDetailsVos;
      this.datas.budgetDetailsVos = this.createDatas(budgetDetailsVos, 'budgetDetailsVos');

      // 如果有追加的预算
      // 获取追加部分的数据
      const appendCostBudgetVos = params.appendCostBudgetVos || [];
      if (appendCostBudgetVos.length) {
        // 公司
        this.checkAppendBudget(appendCostBudgetVos, 'companyBudgetVos');

        // 项目
        this.checkAppendBudget(appendCostBudgetVos, 'budgetDetailsVos');
      }

      this.departChange('companyBudgetVos');
      this.departChange('budgetDetailsVos');
    },
    createFormData(params) {
      // 处理顶部form数据
      let annual = '';
      if (params.companyBudgetVos[0] && params.companyBudgetVos[0].budgetDetailsVos && params.companyBudgetVos[0].budgetDetailsVos[0]) {
        annual = params.companyBudgetVos[0].budgetDetailsVos[0].budgetTypeVo.annually;
      }
      params.companyBudgetVos[0];
      this.form = {
        companyAppendAnnual: new Date().getFullYear() + '', // params.companyAppendAnnual,
        companyId: params.companyId,
        annual,
        money: params.money,
        remarks: params.remarks,
        type: '3',
        dateRange: [
          params.budgetTime.substr(0, 10),
          params.endTime.substr(0, 10)
        ],
        appendMoney: 0,
        projectId: params.projectId,
        appendYear: new Date().getFullYear() + '',
        projectName: params.projectName
      };
      if (params.appendMoney) {
        this.form.appendMoney = params.appendMoney;
      }
    },
    createDatas(data, key) {
      const budgetDetailsVos = data;
      const budgetDetailsVosBudget = [];
      budgetDetailsVos.forEach(item => {
        const obj = this.createBudgetElement(item);
        if (this.type == 'append' || this.type == 'edit') {
          obj.budgetId = item.budgetId || '';
        }
        //
        const departmentId = item.departmentId;
        const index = budgetDetailsVosBudget.findIndex(it => it.departmentId == departmentId);

        if (index > -1) {
          budgetDetailsVosBudget[index].budget.push(obj);
        } else {
          const budgetVos = {
            // id: companyBudgetVosParams.id,
            // budgetProjectId: companyBudgetVosParams.budgetProjectId,
            departName: this.getDepartNameByDepartId(departmentId),
            departmentId: departmentId,
            activeNames: '1',
            isOrigin: true,
            budget: []
          };
          if (this.type == 'edit') {
            budgetVos.isOrigin = false;
          }
          // if (key == 'companyBudgetVos') {
          //   budgetVos.companyBudgetVosId = id
          // }
          if (key == 'budgetDetailsVos' && this.type == 'init') budgetVos.activeNames = '0';
          budgetVos.budget.push(obj);
          budgetDetailsVosBudget.push(budgetVos);
        }
      });

      return budgetDetailsVosBudget;
    },
    checkAppendBudget(appendCostBudgetVos, keyName, isAppend) {
      // 如果包含追加的数据
      // return
      const datas = this.datas;
      if (appendCostBudgetVos.length > 0) {
        appendCostBudgetVos.forEach(item => { // 每一个item就是 一次 追加的对象
          let appendBudgetDetailsVos;
          if (keyName == 'companyBudgetVos') {
            const companyBudgetVos = item.companyBudgetVos;
            // item.companyBudgetVos.
            if (companyBudgetVos.length) {
              appendBudgetDetailsVos = companyBudgetVos[0].appendBudgetDetailsVos || [];
            } else appendBudgetDetailsVos = [];
          } else {
            appendBudgetDetailsVos = item.appendBudgetDetailsVos;
          }
          appendBudgetDetailsVos.forEach((el, index) => { // el 公司某次追加的某一项预算
            const departmentId = el.departmentId;
            const idx = datas[keyName].findIndex(its => its.departmentId == departmentId);
            if (idx > -1) { // 有这个部门，需要判断当前预算归口是否有，如果有把金额加进去，没有则新建budget插入到最后
              const budgetTypeId = el.budgetTypeId;
              const checkObj = this.findBudgetById(this.datas[keyName], budgetTypeId);// budgetDetailsVosBudget.findIndex(it => it.departmentId == departmentId)
              if (checkObj.has) {
                if (isAppend) {
                  datas[keyName][checkObj.x].budget[checkObj.y].appendMoney = el.money;
                } else {
                  const appends = el.money;
                  datas[keyName][checkObj.x].budget[checkObj.y].money += (appends - 0);
                }
              } else {
                datas[keyName][idx].budget.push(this.createBudgetElement(el, isAppend));
              }
            } else { // 初始预算没有此部门，则全部新建就行
              const budgetVos = {
                departName: this.getDepartNameByDepartId(departmentId),
                departmentId: departmentId,
                activeNames: '1',
                budget: []
              };
              budgetVos.budget.push(this.createBudgetElement(el, isAppend));
              datas[keyName].push(budgetVos);
            }
          });
        });
      }
    },
    createBudgetElement(el, isAppend) {
      const isRelateProj = el.projectId != '';
      const obj = {
        // departmentId: '',
        id: el.id,
        // budgetId: el.budgetId, ?????
        name: el.budgetTypeVo.name,
        isRelateProj,
        relateProjId: el.projectId,
        money: el.money || 0,
        appendMoney: 0,
        budgetDetailsId: el.id,
        budgetTypeId: el.budgetTypeVo.id,
        isOrigin: true
      };
      if (this.type == 'edit') {
        obj.budgetId = el.budgetId;
        obj.isOrigin = false;
      }
      if (isAppend) {
        obj.appendMoney = el.money;
        obj.money = 0;
      }

      return obj;
    },
    findBudgetById(datas, budgetTypeId) {
      let x; let y; let has = false;
      for (let i = 0; datas[i]; i++) {
        const budget = datas[i].budget;
        const index = budget.findIndex(item => item.budgetTypeId == budgetTypeId);
        if (index > -1) {
          x = i, y = index, has = true;
          break;
        }
      }
      return { has, x, y };
    },
    // 获取主岗和副岗公司
    getAllCompany() {
      this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {}).then(res => {
        if (res.isSuccess) {
          const companyList = res.data || [];
          companyList.forEach(item => {
            this.getProjectVosByCompanyId(item.id);
          });
        }
      });
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
            res.data.forEach(el => {
              el.companyId = companyId;
              this.projectOptions.push(el);
            });
          }
        }
      );
    },
    getDepartNameByDepartId(id) {
      const depart = this.originDepart.find(item => item.id == id);
      let departName = '';
      if (depart) {
        departName = depart.departmentName;
      }
      return departName;
    },

    getDepartByCompanyId(companyId) {
      console.log('companyId', companyId);
      return new Promise((resove, reject) => {
        this.$axios.post(
          Api.annualBudget.getDepartByCompanyId,
          {
            data: {
              id: companyId
            }
          },
          res => {
            if (res.isSuccess) {
              const data = res.data;
              if (data && data.departmentVos) {
                const originDepart = data.departmentVos;
                originDepart.forEach(item => {
                  if (item.departmentName == '公司领导') item.departmentName = '公司固定费用';
                });
                let index = -1;
                originDepart.forEach((item, i) => {
                  if (item.departmentName == '公司固定费用') index = i;
                });
                if (index > -1) {
                  originDepart.unshift(originDepart.splice(index, 1)[0]);
                }
                this.originDepart = originDepart;
                this.departOptions = {
                  budgetDetailsVos: deepClone(originDepart),
                  companyBudgetVos: deepClone(originDepart)
                };
                resove();
                // this.departOptions = data.departmentVos
              }
            } else {
              this.$message.error(res.message);
              reject();
            }
          }
        );
      });
    },

    departChange(key, idx) {
      if (this.departOptions[key]) {
        this.departOptions[key].forEach(it => {
          it.hasSelect = false;
        });
        this.datas[key].forEach(item => {
          const departmentId = item.departmentId;
          const index = this.departOptions[key].findIndex(it => it.id == departmentId);
          if (index > -1) {
            this.departOptions[key][index].hasSelect = true;
          }
        });

        if (idx !== undefined) {
          this.datas[key][idx].activeNames = '1';
        }
      }
    },
    addPlan(key, index) {
      this.datas[key][index].budget.push(deepClone(this.planTemp));
    },

    addDepartPlan(key) {
      const departPlan = {
        departName: '',
        departmentId: '',
        activeNames: key == 'budgetDetailsVos' ? '0' : '1',
        budget: [
          deepClone(this.planTemp)
        ]
      };
      this.datas[key].push(departPlan);
      this.departChange(key);
    },
    planDelete(key, i, j) {
      if (key == 'companyBudgetVos') { // 追加公司当年预算 删除行 最少有一行
        if (this.datas[key][i].budget.length <= 1) {
          return this.$message.error('最后一行不可删除');
        } else {
          this.datas[key][i].budget.splice(j, 1);
        }
      } else {
        this.datas[key][i].budget.splice(j, 1);
      }

      // this.departChange()
    },
    deleteDepartPlan(index, key) {
      this.datas[key].splice(index, 1);
      this.departChange(key);
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        const v = item || 0;
        val += v - 0;
      });
      return val;
    },
    summary(param) {
      const { columns, data } = param;
      const sums = [];
      let appendTotal = 0;
      columns.forEach((column, index) => {
        if (index === 0) {
          // 只找第一列放合计
          sums[index] = '合计:';
          return;
        }
        if (column.property === 'money') {
          const values = data.map(item => item[column.property]);
          sums[index] = '￥' + ((this.sumNums(values) - 0)).toFixed(6);
        } else if (column.property === 'appendMoney') {
          const values = data.map(item => item[column.property]);
          appendTotal = this.sumNums(values);
          sums[index] = '￥' + (appendTotal.toFixed(6));
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },
    createPostData(status) {
      let data = {};
      const fileId = this.$refs.eleupload.getFileId();
      const enclosure = fileId[0] || '';
      data = {
        companyId: this.form.companyId,
        projectId: this.form.projectId,
        money: this.form.money,
        remarks: this.form.remarks,
        enclosure,
        budgetTime: this.form.dateRange[0] + ' 00:00:00',
        endTime: this.form.dateRange[1] + ' 23:59:59',
        status,
        examineStatus: 0,
        type: this.form.type
      };
      // 计算排序
      let len = 0;
      if (this.params && this.params.appendCostBudgetVos) {
        len = this.params.appendCostBudgetVos.length;
      }
      data.sort = len + 1;
      if (this.type == 'append') {
        data.money = this.form.appendMoney;
        // data.id = this.params.id
        data.budgetId = this.params.id;
        data.appendBudgetDetailsVos = [];
        data.companyBudgetVos = [];
      } else if (this.type == 'edit') {
        data.id = this.params.id;// this.formData.id
        data.budgetId = this.params.id;
        data.budgetDetailsVos = [];
        data.companyBudgetVos = [];
        if (this.editType == 'appendEdit') {
          data.money = this.form.appendMoney;
          data.appendBudgetDetailsVos = [];
        }
      } else {
        data.budgetDetailsVos = [];
        data.companyBudgetVos = [];
        // budgetId
      }
      for (const key in this.datas) {
        const vos = this.datas[key]; // {budgetDetailsVos:[],companyBudgetVos:[]}
        if (key == 'budgetDetailsVos') { // 项目预算
          let childrenSort = 0;
          vos.forEach(item => {
            if (item.departmentId != '') {
              const budget = item.budget;
              budget.forEach((el, j) => {
                if (el.money > 0 || el.appendMoney > 0) {
                  childrenSort += 1;
                  const obj = {
                    departmentId: item.departmentId,
                    budgetDetailsId: el.budgetDetailsId || '',
                    budgetTypeId: el.budgetTypeId || '',
                    projectId: '',
                    money: el.money,
                    type: this.form.type,
                    sort: childrenSort,
                    budgetTypeVo: {
                      projectId: this.form.projectId,
                      name: el.name,
                      parentId: 1,
                      departmentId: item.departmentId,
                      type: this.form.type,
                      annually: this.form.companyAppendAnnual, // this.form.dateRange[0].substr(0, 4), // this.form.annual,
                      status: 0
                    }
                  };
                  if (this.type == 'append') {
                    obj.money = el.appendMoney;
                    obj.budgetTypeVo.annually = this.form.appendYear;
                    data.appendBudgetDetailsVos.push(obj);
                  } else if (this.type == 'edit') {
                    obj.budgetId = el.budgetId || '';
                    if (this.editType == 'appendEdit') {
                      obj.money = el.appendMoney;
                      obj.budgetTypeVo.annually = this.form.appendYear;
                      data.appendBudgetDetailsVos.push(obj);
                    } else {
                      data.budgetDetailsVos.push(obj);
                    }
                  } else {
                    data[key].push(obj);
                  }
                }
              });
            }
          });
        } else { // 公司追加 页面下半部分
          if (vos.length) {
            const companyBudgetVos = {
              companyId: this.form.companyId,
              budgetTime: this.form.companyAppendAnnual + '-01-01 00:00:00',
              money: 0,
              budgetProjectId: '', // this.form.projectId,
              id: this.form.companyBudgetVosId
            };
            if (this.type == 'append') {
              delete companyBudgetVos.id;
            }
            if (this.type == 'append' || (this.type == 'edit' && this.editType == 'appendEdit')) {
              companyBudgetVos.appendBudgetDetailsVos = [];
            } else {
              companyBudgetVos.budgetDetailsVos = [];
            }
            if (this.type == 'edit' && this.editType == 'appendEdit') {
              companyBudgetVos.id = this.formData.companyBudgetVos[0].id;
            }
            let totalMoeny = 0;
            let childrenSort = 0;
            vos.forEach(item => {
              const budget = item.budget;
              budget.forEach(el => {
                let money = (el.money - 0);
                if (this.type == 'append') {
                  money = (el.appendMoney - 0);
                } else if (this.type == 'edit') {
                  if (this.editType == 'appendEdit') {
                    money = (el.appendMoney - 0);
                  }
                }
                console.log('monye', money);
                if (money > 0) {
                  childrenSort += 1;
                  totalMoeny += money;
                  const obj = {
                    departmentId: item.departmentId,
                    budgetDetailsId: el.budgetDetailsId || '',
                    budgetTypeId: el.budgetTypeId || '',
                    projectId: '',
                    money,
                    type: this.form.type,
                    sort: childrenSort,
                    budgetTypeVo: {
                      projectId: this.form.projectId,
                      name: el.name,
                      parentId: 1,
                      departmentId: item.departmentId,
                      type: this.form.type,
                      annually: this.form.companyAppendAnnual,
                      status: 0
                    }
                  };
                  if (this.type == 'append' || (this.type == 'edit' && this.editType == 'appendEdit')) {
                    if (status != 0) obj.budgetId = el.budgetId || '';
                    obj.budgetTypeVo.annually = this.form.appendYear;
                    companyBudgetVos.appendBudgetDetailsVos.push(obj);
                  } else if (this.type == 'edit') {
                    if (status != 0) obj.budgetId = el.budgetId;
                    companyBudgetVos.budgetDetailsVos.push(obj);
                  } else {
                    companyBudgetVos.budgetDetailsVos.push(obj);
                  }
                }
              });
            });
            companyBudgetVos.money = totalMoeny;
            data[key].push(companyBudgetVos);
          }
        }
      }
      return data;
    },
    /**
     *
     * @param {*} status
     * 获取流程，即调用getFlowFindById方法的条件
     * 1非草稿提交预算 2有对应的流程id 3noNeedCheckFlow 参数为false
     */
    async submit(status) {
      // if (status == 1) await this.getFlowType()
      // if (!this.flowId && status == 1) return false
      // return
      let err = false;
      this.$refs.ruleForm.validate(res => {
        if (!res) err = true;
      });
      if (err) {
        return this.$message.error('有必填项未填');;
      }
      const data = this.createPostData(status);
      const res = this.checkPostData(data);
      // console.log('data----1',data)
      // return
      this.postDatas = data;
      if (res) {
        if (status == 1) this.sumbitFlow();
        else {
          this.postData();
        }
      }
    },
    postData() {
      var data = this.postDatas;
      let action = Api.annualBudget.costBudgetSave;
      if (this.type == 'append') {
        action = Api.annualBudget.appendCostBudgetSave;
      } else if (this.type == 'edit') {
        action = Api.annualBudget.initBudgetUpdate;
        if (this.editType == 'appendEdit') {
          data.id = this.formData.id;// data.budgetId;
          action = Api.annualBudget.costBudgetUpdate;
        }
      }
      // console.log('data-----2',data)
      // return
      this.$axios.post(action, { data }, res => {
        if (res.isSuccess) {
          let id; // 绑定文件的id
          let resData; // 交给提交后处理数据的函数 append/init 取返回数据 edit取提交的数据
          if (this.type == 'init' || this.type == 'append') {
            id = res.data.id;
            resData = res.data;
          } else {
            id = data.id;
            resData = data;
          }
          if (
            (this.type == 'init' && data.enclosure) ||
            (this.editType == 'initEdit' && data.enclosure && data.enclosure != this.params.enclosure) ||
            (this.editType == 'appendEdit' && data.enclosure && data.enclosure != this.currentAppendCostBudgetVos.enclosure)
          ) {
            this.bindFileById(id, data.enclosure).then(() => {
              this.businessId = id;
              this.businesstotal = data.money;
              this.businessData = resData;
              this.processReturnData(data.status, id, data.money, resData);
            });
          } else {
            this.businessId = id;
            this.businesstotal = data.money;
            this.businessData = resData;
            this.processReturnData(data.status, id, data.money, resData);
          }
        } else {
          this.$message.error(res.message);
        }
      }
      );
    },
    checkPostData(data) {
      // console.log("data[key]",data)
      const res = true;
      if (data.money == 0) {
        let msg = '预算金额';
        if (this.type == 'append') msg = '追加金额';
        this.$message.error(`${msg}不可为0`);
        return false;
      }
      const companyBudgetVos = data.companyBudgetVos[0];
      console.log('companyBudgetVos', companyBudgetVos);
      if (companyBudgetVos.companyId) {
        if (!this.form.companyAppendAnnual) {
          this.$message.error('追加当年公司预算需要选择年份');
          return false;
        }
        const companyBudgetDetailsVos = companyBudgetVos.budgetDetailsVos || companyBudgetVos.appendBudgetDetailsVos || [];
        let hasDepart = false;
        for (let i = 0; companyBudgetDetailsVos[i]; i++) {
          if (companyBudgetDetailsVos[i].departmentId) {
            hasDepart = true;
          }
          if (!companyBudgetDetailsVos[i].budgetTypeVo.name) {
            this.$message.error('所有费用预算类型（一级）为必填');
            return false;
          }
          if (companyBudgetDetailsVos[i].money == 0 && this.type == 'init') {
            this.$message.error('新建预算金额不可为0');
            return false;
          }
        }
        if (!hasDepart) {
          this.$message.error('至少选择一个部门且部门预算不可为0');
          return false;
        }
      } else {
        this.$message.error('至少选择一个部门且部门预算不可为0');
        return false;
      }
      return res;
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
    processReturnData(status, id, data) {
      if (status == 1) {
        this.submitFlowFinal(true, id, '', data.money || data);
        // this.submitFinal(id, total).then(response => {
        //   if (response.isSuccess) {
        //     this.$message.success('操作成功');
        //     this.$router.push({ path: '/groupBudgetManage/projectBudget/index' });
        //   } else {
        //     this.failSubmitFlow(response, data)
        //   }
        // }).catch(err => {
        //   this.failSubmitFlow(err, data)
        // });
      } else {
        this.$message.success('操作成功');
        this.prevStepHandle();
        // this.$router.push({ path: '/groupBudgetManage/projectBudget/index' });
      }
    },
    failSubmitFlow(response, data) {
      this.checkboxPersonGroup = [];
      // 如果提交审核失败，调用更新接口，把预算状态改成草稿
      this.$message.error(response.message + '，预算将转成草稿状态，你可以再次提交审核');
      this.updateStatusWhenFlowFail(data);
    },
    updateStatusWhenFlowFail(data) { // 流程发起失败后，修改业务状态为草稿
      data.status = 0;
      if (data.companyBudgetVos && data.companyBudgetVos.length) {
        data.companyBudgetVos[0];
      }
      let action = Api.annualBudget.initBudgetUpdate;
      if (this.type == 'append') { // 追加的时候提交审核失败
        action = Api.annualBudget.costBudgetUpdate;
      }
      if (this.type == 'edit') {
        // action = Api.annualBudget.initBudgetUpdate;
        if (this.editType == 'appendEdit') {
          action = Api.annualBudget.costBudgetUpdate;
        }
      }
      this.$axios.post(action, { data }, res => {
        if (res.isSuccess) {
          this.processReturnData(0);
        } else {
          this.$message.error(res.message);
        }
      });
    },
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
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
            return {
              id: item.id,
              fileName: item.fileName,
              fileUrl: item.fileUrl
            };
          });
          this.attachFile = attachFile;
        }
      });
    },
    async getFlowType() {
      const param = {
        data: {
          typeId: '',
          flowName: '',
          nextNodeName: '',
          flowNodeType: '',
          nextNodeProxyId: '',
          flowStatus: 'enable',
          auditWay: this.selectFlowType,
          useScope: 'invest'
        },
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
      };

      await this.$axios.post(
        Api.schedule.getFlowTemplateList,
        param,
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
    // 判断是否审批自选
    // checkIsOptional() {
    //   let flag = false;
    //   if (this.flowNodeType == 'run_node_choose') { // 自选节点
    //     if (!this.checkboxPersonGroup.length) {
    //       this.nodeChooseVisible = true;
    //       flag = true;
    //     }
    //   }
    //   return flag;
    // },

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
    // 流程里面的提交
    flowSubmit() {
      if (this.tabList.length != this.tab) {
        this.tab = this.tabList.length;
        this.radioChange(this.tab);
      }
      let type = 'init'; let tabIndex = 0; let len;
      if (this.params.appendCostBudgetVos.length) { // 有追加的,只计算最后一个tab的数据
        type = 'append';
        len = this.params.appendCostBudgetVos.length;
        tabIndex = len + 1;
      } else { // 没有追加
      }
      let data = {};
      const fileId = this.$refs.eleupload.getFileId();
      const enclosure = fileId[0] || '';
      data = {
        companyId: this.form.companyId,
        projectId: this.form.projectId,
        money: this.form.money,
        remarks: this.form.remarks,
        enclosure,
        budgetTime: this.form.dateRange[0] + ' 00:00:00',
        endTime: this.form.dateRange[1] + ' 23:59:59',
        status: 1,
        examineStatus: 0,
        type: this.form.type,
        budgetId: this.params.id
      };
      // 计算排序
      if (type == 'append') {
        let len = 0;
        len = this.params.appendCostBudgetVos.length;
        data.sort = len;
        data.money = this.form.appendMoney;
        data.id = this.params.appendCostBudgetVos[len - 1].id;
        // data.id = this.params.id
        data.appendBudgetDetailsVos = [];
        data.companyBudgetVos = [];
      } else {
        data.id = this.params.id;
        data.budgetDetailsVos = [];
        data.companyBudgetVos = [];
      }

      for (const key in this.datas) {
        const vos = this.datas[key]; // {budgetDetailsVos:[],companyBudgetVos:[]}
        if (key == 'budgetDetailsVos') { // 项目预算
          let childrenSort = 0;
          vos.forEach(item => {
            if (item.departmentId != '') {
              const budget = item.budget;
              budget.forEach(el => {
                if (el.money > 0) {
                  childrenSort += 1;
                  const obj = {
                    departmentId: item.departmentId,
                    budgetDetailsId: el.budgetDetailsId || '',
                    budgetTypeId: el.budgetTypeId || '',
                    projectId: '',
                    money: el.money,
                    type: this.form.type,
                    budgetId: el.budgetId || '',
                    sort: childrenSort,
                    budgetTypeVo: {
                      projectId: this.form.projectId,
                      name: el.name,
                      parentId: 1,
                      departmentId: item.departmentId,
                      type: this.form.type,
                      annually: this.form.companyAppendAnnual, // this.form.appendYear,//dateRange[0].substr(0, 4), // this.form.annual,
                      status: 0
                    }
                  };
                  if (type == 'append') {
                    obj.budgetTypeVo.annually = this.form.appendYear;
                    // obj.money = el.appendMoney;
                    data.appendBudgetDetailsVos.push(obj);
                  } else {
                    data.budgetDetailsVos.push(obj);
                  }
                }
              });
            }
          });
        } else { // 公司追加 页面下半部分
          if (vos.length) {
            const companyBudgetVos = {
              companyId: this.form.companyId,
              budgetTime: this.form.companyAppendAnnual + '-01-01 00:00:00',
              money: 0,
              budgetProjectId: '', // this.form.projectId,
              id: this.form.companyBudgetVosId
            };
            if (type == 'append') {
              companyBudgetVos.appendBudgetDetailsVos = [];
            } else {
              companyBudgetVos.budgetDetailsVos = [];
            }
            let totalMoeny = 0;
            let childrenSort = 0;
            vos.forEach(item => {
              const budget = item.budget;
              budget.forEach(el => {
                let money = (el.money - 0);
                if (this.type == 'append') {
                  money = (el.appendMoney - 0);
                } else if (this.type == 'edit') {
                  if (this.editType == 'appendEdit') {
                    money = (el.appendMoney - 0);
                  }
                }
                if (money > 0) {
                  childrenSort += 1;
                  totalMoeny += money;
                  const obj = {
                    departmentId: item.departmentId,
                    budgetDetailsId: el.budgetDetailsId || '',
                    budgetTypeId: el.budgetTypeId || '',
                    projectId: '',
                    money,
                    type: this.form.type,
                    sort: childrenSort,
                    budgetTypeVo: {
                      projectId: this.form.projectId,
                      name: el.name,
                      parentId: 1,
                      departmentId: item.departmentId,
                      type: this.form.type,
                      annually: this.form.companyAppendAnnual,
                      status: 0
                    }
                  };
                  if (type == 'append') {
                    obj.budgetId = el.budgetId || '';
                    obj.budgetTypeVo.annually = this.form.appendYear;
                    companyBudgetVos.appendBudgetDetailsVos.push(obj);
                  } else {
                    companyBudgetVos.budgetDetailsVos.push(obj);
                  }
                }
              });
            });
            companyBudgetVos.money = totalMoeny;
            data[key].push(companyBudgetVos);
          }
        }
      }

      let action = Api.annualBudget.initBudgetUpdate;
      if (type == 'append') {
        action = Api.annualBudget.costBudgetUpdate;
      }
      this.$axios.post(action, { data }, res => {
        // return
        const obj = {
          status: 'success',
          val: this.busVal,
          total: data.money
        };
        if (!res.isSuccess) {
          obj.status = 'fail';
        }
        this.$bus.$emit('submitBeforeHandleOk', obj);
      });
    }
  }
};
</script>
<style lang="scss" scoped src="@/views/BudgetManage/CompanyBudget/components/style/style.scss">
table {
  border-collapse: separate;
  border-spacing: 0;
}
</style>
