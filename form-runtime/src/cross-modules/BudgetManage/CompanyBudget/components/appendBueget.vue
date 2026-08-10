<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>追加公司年度预算</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row :gutter="20" style="margin-bottom:30px">
              <el-col :span="12">
                <el-form-item label="公司名称：" prop="companyId">
                  <el-select v-model="form.companyId" placeholder="请选择" @change="companyChange" >
                    <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="追加年度：" prop="annual">
                  <el-date-picker v-model="form.annual" type="year" value-format="yyyy" @change="annualChange">
                  </el-date-picker>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20">
              <el-col :span="12">
                <el-form-item label="追加金额(万)：" prop="money">
                  <el-input-number v-model="form.money" :min="0" :precision="6" :step="0.1" :controls="false">
                  </el-input-number>
                </el-form-item>

              </el-col>
              <el-col :span="12">
                <el-form-item label="追加金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20">
              <el-col :span="24">
                <eleupload ref="eleupload"></eleupload>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
        <el-card class="box-card" style="margin-top:30px">
          <div>
            <h3 style="display:inline-block;margin-bottom:20px;">
              预算详情
            </h3>
            <el-button style="margin-left:30px" type="primary" icon="el-icon-plus" @click="addDepartPlan">新增</el-button>
          </div>
          <el-form :model="{ datas }" :rules="subRule" ref="subForm">
            <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;"
              shadow="hover">
              <template slot="header">
                <div class="card-title" @click.stop.prevent="() => { }">
                  <el-form-item :prop="'datas.' + index + '.departId'" :rules="subRule.departId">
                    <el-select v-model="val.departId" placeholder="请选择" style="margin-right:10%;" @change="departChange"
                      :disabled="val['disabled']">
                      <el-option v-for="item in departOptions" :key="item.id" :label="item.name" :value="item.id"
                        :disabled="item.hasSelect">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </div>
              </template>
              <div class="el-icon-close" style="position:absolute;right:5px;top:5px;" @click="deleteDepartPlan(index)"
                v-if="!val['disabled']">
              </div>
              <el-collapse v-model="val['activeNames']">
                <el-collapse-item name="1">
                  <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                    <el-table-column type="index" label="编号" width="65px">
                    </el-table-column>
                    <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType" width="180px">
                      <template slot-scope="scope">
                        <el-input v-model="scope.row.budgetType" :disabled="scope.row.disabled" maxlength="20"></el-input>
                      </template>
                    </el-table-column>
                    <el-table-column prop="relateProjId" label="是否关联项目" width="320px">
                      <template slot-scope="scope">
                        <div>
                          <el-radio v-model="scope.row.isRelateProj" :label="true"
                            @change="v => radioChange(v, index, scope.$index)" :disabled="scope.row.disabled">是
                          </el-radio>
                          <el-radio v-model="scope.row.isRelateProj" :label="false"
                            @change="v => radioChange(v, index, scope.$index)" :disabled="scope.row.disabled">
                            否
                          </el-radio>
                          <el-select v-model="scope.row.relateProjId" placeholder="请选择" style="margin-right:10%;"
                            :disabled="!scope.row.isRelateProj || scope.row.disabled">
                            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name"
                              :value="item.id">
                            </el-option>
                          </el-select>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column prop="appendMoney" label="本次追加(万)" width="160">
                      <template slot-scope="scope">
                        <el-input-number v-model="scope.row.appendMoney" :precision="6" :step="0.1" :controls="false">
                        </el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="budgetMoney" label="预算总额(万)" width="160">
                      <template slot-scope="scope">
                        <el-input-number :value="(scope.row.budgetMoney - 0)" :disabled="true" :precision="6"
                          :step="0.1" :controls="false">
                        </el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column fixed="right" label="操作" width="80px">
                      <template slot-scope="scope">
                        <el-button @click="planDelete(index, scope.$index)" type="text" size="small"
                          v-if="scope.row.canDelete"><i class="el-icon-delete-solid delete-icon"></i></el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(index)"></i>
                </el-collapse-item>
              </el-collapse>
            </el-card>
          </el-form>
        </el-card>
      </div>
    </div>
    <div class="footer-bt">
      <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
      <el-button @click="prevStepHandle" plain>取 消</el-button>
      <el-button type="primary" plain @click="submit(0)">保 存</el-button>
      <el-button type="primary" @click="submit(1)">提 交</el-button>
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
import {
  localstorageGet
} from '@/utils/auth';
import Api from '@/api';
import mixin from './mixins/mixin';
import { deepClone } from '@/utils/index';
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog';
import BranchChoose from '@/views/BudgetManage/CompanyBudget/components/branchChoose';
import BranchPralleChoose from '@/views/BudgetManage/CompanyBudget/components/branchPralleChoose';
import numFunc from '@/utils/number'; // 重写toFixed
Number.prototype.toFixed = numFunc;
export default {
  name: 'AppendAnnualBudget',
  mixins: [mixin],
  // props: ['params'],
  components: { eleupload, PersonSelectDialog, BranchChoose, BranchPralleChoose },
  data() {
    return {
      form: {
        companyId: '',
        annual: '',
        remark: '',
        type: '1'
      },
      params: {},
      dateOption: [],
      datas: [
      ],
      companyOption: [],
      departOptions: [],
      projectOptions: [],
      addPlanTemp: {
        budgetDetailsId: '',
        budgetTypeId: '',
        budgetType: '',
        isRelateProj: false,
        relateProjName: '',
        relateProjId: '',
        budgetMoney: 0,
        appendMoney: 0,
        canDelete: true,
        disabled: false
      },
      // selectFlowType: 'company_annual_budget',
      selectFlowType: 'add_annual_budget',

      nodeChooseVisible: false,
      nextNodeName: '',
      nextNodeProxyId: '',
      checkboxPersonGroup: [],
      flowId: ''
    };
  },
  inject: ['prevStepHandle', 'sumbitFlow', 'submitFlowFinal'],
  async created() {
    // this.getCompanyTree(this.params.companyId);
    // await this.getFlowType()
    await this.getParentCompanyList();
    this.getCompanyListOfOnDuty().then(res => {
      if (res.isSuccess) {
        const dutyCompanyOption = res.data;
        const companyOptions = [];
        dutyCompanyOption.forEach(item => {
          const dutyCompanyId = item.id;
          const index = this.companyOption.findIndex(el => el.id == dutyCompanyId);
          if (index > -1) {
            companyOptions.push(deepClone(this.companyOption[index]));
          }
        });
        this.companyOption = companyOptions;
      } else {
        this.$message.error('人员暂无公司岗位');
      }
    });
  },
  watch: {
    companyOption(val) {
      if (val.length) {
        const mainCompany = val.find(item => {
          return item.flag == 'mainDutyCompany';
        });
        if (mainCompany) {
          this.form.companyId = mainCompany.id;
          this.disableCompany = true;
          // this.companyChange(this.form.companyId);
        }
      }
    }
  },
  mounted() {
  },
  methods: {
    getCompanyListOfOnDuty() {
      return this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {});
    },
    annualChange() {
      if (this.form.companyId && this.form.annual) {
        this.getBuegetInfo(this.form.companyId, this.form.annual);
      }
    },
    async getBuegetInfo(companyId, annual) {
      const query = {
        companyId, // localstorageGet('companyId'),
        budgetTime: `${annual}-01-01 00:00:00`,
        type: 1
      };
      await this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        }).then(res => {
        if (res.isSuccess) {
          const data = res.data;
          // console.log('data',data)
          if (data && data.dataList && data.dataList.length) {
            const list = data.dataList[0];
            if (list.budgetDetailsVos.length) {
              if (list.appendCostBudgetVos.length) { // 有追加的,需要遍历追加的数据
                var appendCostBudgetVos = list.appendCostBudgetVos;
                var isDraft = false; var isFlowBack = false; var isInFlow = false;

                appendCostBudgetVos.forEach(item => {
                  if (item.status == 0) {
                    isDraft = true;
                  }
                  if (item.examineStatus == 0) {
                    isInFlow = true;
                  }
                  if (item.examineStatus == 2) {
                    isFlowBack = true;
                  }
                });
                if (isDraft) {
                  this.$confirm(`该公司在${this.form.annual}年度的预算有已保存草稿，是否要载入数据开始编辑`, '提示', {
                    closeOnClickModal: false,
                    confirmButtonText: '确定',
                    cancelButtonText: '重新选择',
                    type: 'warning'
                  }).then(() => {
                    this.toEditPage(list);
                  }).catch(() => {
                    this.form.annual = '';
                  });
                } else {
                  if (isInFlow) {
                    this.$message.error('该年度预算正在审核中，暂时无法追加');
                    this.form.annual = '';
                  } else if (isFlowBack) {
                    this.$confirm(`该公司在${this.form.annual}年度的追加预算已被驳回，是否要载入数据重新编辑`, '提示', {
                      closeOnClickModal: false,
                      confirmButtonText: '确定',
                      cancelButtonText: '重新选择',
                      type: 'warning'
                    }).then(() => {
                      this.toEditPage(list);
                    }).catch(() => {
                      this.form.annual = '';
                    });
                  } else {
                    this.params = list;
                    this.initData();
                  }
                }
              } else { // 没有追加的
                if (list.examineStatus == 0 && list.status == 1) {
                  this.$message.error('该年度预算正在审核中，暂时无法追加');
                  this.form.annual = '';
                } else if (list.examineStatus == 0 && list.status == 0) { // 载入草稿
                  this.$message.error('该年度预算处于草稿状态，请使用新建年度预算流程进行编辑提交');
                  this.form.annual = '';
                  // this.toEditPage(list);
                } else if (list.examineStatus == 1) {
                  this.params = list;
                  this.initData();
                }
              }
            } else {
              this.$message.error('该年度暂无预算或者预算正在审核中，暂时无法追加');
              this.form.annual = '';
            }
          } else {
            this.$message.error('该年度暂未新建预算，无法追加');
            this.form.annual = '';
          }
        }
      });
    },
    toEditPage(list) { // 编辑，也就是再次发起审批
      this.$emit('changeComponent', { list, type: 'edit_company_annual_budget' });
    },
    initData() {
      this.form = {
        budgetId: this.params.id,
        companyId: this.params.companyId,
        companyName: this.params.companyName,
        annual: this.params.budgetTime.substr(0, 4),
        money: '',
        remarks: this.params.remarks
      };
      const datas = [];
      // let obj = {}
      const budgetDetailsVos = this.params.budgetDetailsVos;
      for (let i = 0; budgetDetailsVos[i]; i++) {
        const departId = budgetDetailsVos[i].departmentId;
        const index = datas.findIndex(item => item.departId == departId);
        const budgetTypeVo = budgetDetailsVos[i].budgetTypeVo;
        const isRelateProj = !!budgetDetailsVos[i].projectId;
        // let appendMoneys = budgetDetailsVos[i].money//this.getAppendMoney(budgetDetailsVos[i].budgetTypeId, appendCostBudgetVos)
        const tmp = {
          budgetDetailsId: budgetDetailsVos[i].id,
          budgetTypeId: budgetTypeVo.id,
          budgetType: budgetTypeVo.name,
          isRelateProj,
          relateProjName: budgetDetailsVos[i].projectName,
          relateProjId: budgetDetailsVos[i].projectId,
          budgetMoney: budgetDetailsVos[i].money || 0, // + appendMoneys,
          appendMoney: 0,
          canDelete: false,
          disabled: true
        };
        if (index > -1) {
          datas[index].budget.push(tmp);
        } else {
          const obj = {
            planId: datas.length + 1,
            departName: this.getDepartNameById(budgetDetailsVos[i].departmentId),
            departId: budgetDetailsVos[i].departmentId,
            activeNames: '1',
            disabled: true
          };
          obj.budget = [];
          obj.budget.push(tmp);
          datas.push(obj);
        }
      }
      const appendCostBudgetVosArr = this.params.appendCostBudgetVos || [];
      for (let i = 0; appendCostBudgetVosArr[i]; i++) {
        const appendBudgetDetailsVos = appendCostBudgetVosArr[i].appendBudgetDetailsVos || {};
        for (let j = 0; appendBudgetDetailsVos[j]; j++) {
          const budgetTypeId = appendBudgetDetailsVos[j].budgetTypeId;
          const obj = this.findBudgetById(datas, budgetTypeId);
          if (obj.has) {
            const appends = appendBudgetDetailsVos[j].money;
            datas[obj.x].budget[obj.y].budgetMoney += (appends - 0);
          } else {
            const isRelateProj = !!appendBudgetDetailsVos[j].processId;
            const tmp = {
              // 新增追加归口给空，追加原有归口取原有id
              budgetDetailsId: appendBudgetDetailsVos[j].id,
              budgetTypeId: appendBudgetDetailsVos[j].budgetTypeId,
              budgetType: appendBudgetDetailsVos[j].budgetTypeVo.name,
              isRelateProj,
              relateProjName: appendBudgetDetailsVos[j].projectName,
              relateProjId: appendBudgetDetailsVos[j].projectId,
              budgetMoney: appendBudgetDetailsVos[j].money || 0, // + appendMoneys,
              appendMoney: 0,
              canDelete: false,
              disabled: true
            };
            const departId = appendBudgetDetailsVos[j].budgetTypeVo.departmentId;
            const idx = datas.findIndex(el => el.departId == departId);
            if (idx > -1) {
              datas[idx].budget.push(tmp);
            } else {
              const obj = {
                planId: datas.length + 1,
                departName: this.getDepartNameById(departId),
                departId,
                activeNames: '1',
                disabled: true
              };
              obj.budget = [];
              obj.budget.push(tmp);
              datas.push(obj);
            }
          }
        }
      }

      // 1.3 计算金额调剂的 ------------------------------ 通过budgetDetailsId 去绑定
      const budgetAdjustVo = this.params.budgetAdjustVo || [];
      budgetAdjustVo.forEach(it => {
        if (it.examineStatus == 1) {
          const adjustMoneyVos = it.adjustMoneyVos;
          // console.log('adjustMoneyVos', adjustMoneyVos)
          adjustMoneyVos.forEach(item => {
            const budgetDetailsId = item.budgetDetailsId;
            const checkObj = this.findBudgetById(datas, budgetDetailsId, 'budgetDetailsId');
            if (checkObj.has) {
              const increaseOrReduce = (item.increaseOrReduce - 0);
              datas[checkObj.x].budget[checkObj.y].budgetMoney += increaseOrReduce;
            } else {
              const departId = item.budgetDetailsVo.departmentId;
              const isRelateProj = !!item.budgetDetailsVo.projectId;
              const tmp = {
                budgetDetailsId: item.budgetDetailsId,
                budgetTypeId: item.budgetDetailsVo.budgetTypeId,
                budgetType: item.budgetDetailsVo.budgetTypeVo.name,
                isRelateProj,
                relateProjName: item.budgetDetailsVo.projectName || '',
                relateProjId: item.budgetDetailsVo.projectName,
                budgetMoney: item.money || 0, // + appendMoneys,
                appendMoney: 0,
                canDelete: false,
                disabled: true
              };
              // 是否有相同的部门
              const idx = datas.findIndex(el => el.departId == departId);
              if (idx > -1) {
                datas[idx].budget.push(tmp);
              } else {
                const obj = {
                  planId: datas.length + 1,
                  departName: this.getDepartNameById(departId),
                  departId,
                  activeNames: '1',
                  disabled: true
                };
                obj.budget = [];
                obj.budget.push(tmp);
                datas.push(obj);
              }
            }
          });
        }
      });
      this.datas = datas;
    },
    findBudgetById(datas, budgetTypeId, key) {
      if (!key) key = 'budgetTypeId';
      let x; let y; let has = false;
      for (let i = 0; datas[i]; i++) {
        const budget = datas[i].budget;
        const index = budget.findIndex(item => item[key] == budgetTypeId);
        if (index > -1) {
          x = i, y = index, has = true;
          break;
        }
      }
      return { has, x, y };
    },
    getAppendMoney(budgetTypeId, appendCostBudgetVos) {
      let total = 0;
      appendCostBudgetVos.forEach(item => {
        const appendBudgetDetailsVos = item.appendBudgetDetailsVos;
        const index = appendBudgetDetailsVos.findIndex(it => it.budgetTypeId == budgetTypeId);
        if (index > -1) {
          total = total + (appendBudgetDetailsVos[index].money - 0);
        }
      });
      return total;
    },
    radioChange(val, i, j) {
      if (val) { // 关联项目
        const companyId = this.form.companyId;
        if (!companyId) {
          this.datas[i].budget[j].isRelateProj = false;
          return this.$message.error('请先选择公司，再关联项目');
        }
        // this.getProjectVosByCompanyId(companyId)
      } else { // 不关联
        this.datas[i].budget[j].relateProjId = '';
      }
    },
    getCompanyTree(id) { // 获取公司部门架构数据
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: 3,
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            const treeData = res.data[0] || {};
            this.originDepartData = {};
            if (treeData && treeData.childrenList) {
              this.originDepartData = treeData.childrenList;
              // this.companyChange(id);
              this.getProjectVosByCompanyId(id);
              this.initData();
              // console.log('departOptions', this.departOptions)
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },

    departChange() {
      this.departOptions.forEach(item => {
        item.hasSelect = false;
      });
      this.datas.forEach(item => {
        // this.departOptions[index].hasSelect = false
        const departId = item.departId;
        const index = this.departOptions.findIndex(it => it.id == departId);
        if (index > -1) {
          this.departOptions[index].hasSelect = true;
        }
      });
    },

    addDepartPlan() {
      this.departChange();
      const index = this.datas.length;
      const addPlanTemp = deepClone(this.addPlanTemp);
      const budget = [addPlanTemp];
      const departPlan = {
        planId: index,
        departName: '',
        departId: '',
        activeNames: '1',
        disabled: false,
        budget
      };
      this.datas.push(departPlan);
      // this.$nextTick(() => {
      //   this.departChange();
      // });
    },
    addPlan(index) {
      const plan = deepClone(this.addPlanTemp);
      this.datas[index].budget.push(plan);
    },
    /**
     * @param {} status 提交状态0草稿1提交
     * @param {*} noNeedCheckFlow
     * 获取流程，即调用getFlowFindById方法的条件
     * 1非草稿提交预算 2有对应的流程id 3noNeedCheckFlow 参数为false*
     */
    async submit(status, noNeedCheckFlow) {
      let err = false;
      this.$refs.ruleForm.validate(res => {
        if (!res) err = true;
      });
      this.$refs.subForm.validate(res => {
        if (!res) err = true;
      });
      if (err) return false;
      // console.log(this.datas)
      // console.log(3)
      // return
      if (!this.datas.length) {
        return this.$message.error('预算不可为空');
      }
      this.form.type = 1;
      this.form.status = status;
      this.form.processId = '';
      this.form.projectId = '';
      this.form.examineStatus = 0;
      this.form.sort = this.params.appendCostBudgetVos.length + 1;
      // let budgetDetailsVos = []
      const appendBudgetDetailsVos = [];
      let totalBueget = 0;
      // console.log(this.datas)
      // return
      let childrenSort = 0;
      for (let i = 0; this.datas[i]; i++) {
        const budget = this.datas[i].budget;
        for (let j = 0; budget[j]; j++) {
          if (budget[j].appendMoney == 0) continue;
          const tmpObj = {};
          // budgetDetailsId: budgetDetailsVos[i].id,
          // budgetTypeId: budgetTypeVo.id,
          childrenSort += 1;
          tmpObj.budgetDetailsId = budget[j].budgetDetailsId || '';
          tmpObj.budgetTypeId = budget[j].budgetTypeId || '';
          tmpObj.departmentId = this.datas[i].departId;
          tmpObj.type = this.form.type;
          tmpObj.projectId = budget[j].relateProjId;
          // tmpObj['money'] = budget[j].budgetMoney
          tmpObj.money = budget[j].appendMoney;
          tmpObj.sort = childrenSort;
          // tmpObj.sort = this.params.appendCostBudgetVos.length;
          if (!tmpObj.money && budget[j].canDelete) {
            this.$message.error('新增的追加预算金额不可为0');
            return false;
          }
          totalBueget = totalBueget + (tmpObj.money - 0);
          tmpObj.budgetTypeVo = {};
          const budgetTypeVo = {};
          budgetTypeVo.name = budget[j].budgetType;
          if (budgetTypeVo.name == '') {
            this.$message.error('所有费用预算类型均为必填');
            return false;
          }
          const idx = budget.findIndex((el, k) => k != j && el.budgetType == budgetTypeVo.name);
          if (idx > -1) {
            this.$message.error('相同部门下不可有相同归口');
            return false;
          }
          budgetTypeVo.parentId = 1;
          budgetTypeVo.departmentId = this.datas[i].departId;
          budgetTypeVo.type = this.form.type;
          budgetTypeVo.annually = this.form.annual;
          budgetTypeVo.status = 0;// this.form.status
          tmpObj.budgetTypeVo = budgetTypeVo;
          appendBudgetDetailsVos.push(tmpObj);
        }
      }
      this.form.appendBudgetDetailsVos = appendBudgetDetailsVos;
      this.form.budgetTime = `${this.form.annual}-01-01 00:00:00`;
      const fileId = this.$refs.eleupload.getFileId();
      this.form.enclosure = fileId[0] || '';
      if (totalBueget.toFixed(6) != this.form.money.toFixed(6)) {
        return this.$message.error('追加总金额和追加详情金额不符');
      }
      if (status == 1) this.sumbitFlow();
      else {
        this.postData();
      }
    },
    postData(data) {
      var data = this.form;
      this.$axios.post(Api.annualBudget.appendCostBudgetSave, { data }, res => {
        if (res.isSuccess) {
          if (data.enclosure) {
            // 绑定文件
            const relationId = res.data.id;
            const fileId = data.enclosure;
            this.bindFileById(relationId, fileId).then(r => {
              this.processReturnData(data, res);
            });
          } else {
            this.processReturnData(data, res);
          }
        } else {
          this.$message.error(res.message);
        }
      }
      );
    },
    processReturnData(data, res) {
      if (data.status == 1) {
        // 提交审核
        this.submitFlowFinal(true, res.data.id, '', data.money);
      } else {
        this.$router.push({ path: '/groupBudgetManage/companyBudget/index' });
      }
    },
    updateStatusWhenFlowFail(data) { // 流程发起失败后，修改业务状态为草稿
      data.status = 0;
      const action = Api.annualBudget.costBudgetUpdate;
      this.$axios.post(action, { data }, res => {
        if (res.isSuccess) {
          this.processReturnData(data);
        } else {
          this.$message.error(res.message);
        }
      });
    }
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss">

</style>
