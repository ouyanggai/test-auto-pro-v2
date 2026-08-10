<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2 style="text-align:center;padding:10px">公司年度预算</h2>
      <div class="inner-container">
        <el-radio-group size="medium" v-model="tab" v-if="tabList.length" @change="radioChange"
          style="margin-bottom: 10px;">
          <el-radio-button label="1">年度预算</el-radio-button>
          <el-radio-button v-for="val in tabList" :label="(val['index'] + 2)" :key="val['index']">{{ val['label'] }}
          </el-radio-button>
        </el-radio-group>
        <el-card class="box-card" shadow="never">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <!-- <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule' :disabled="isDisabled"> -->
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row :gutter="20" style="margin-bottom:30px">
              <el-col :span="11">
                <el-form-item :label="params.type == 4 || params.type == 6 ? '部门名称：' : '公司名称：'">
                  <el-input v-model="groupDepartName" :disabled="isDisabled('companyName') || type == 'edit'"
                    v-if="params.type == 4 || params.type == 6" style="width: 100%;"></el-input>
                  <el-input v-model="form.companyName" :disabled="isDisabled('companyName') || type == 'edit'" v-else
                    style="width: 100%;"></el-input>
                  <!-- <el-select v-model="form.companyId" placeholder="请选择" :disabled="isDisabled('companyName')">
                    <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select> -->
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="预算年度：">
                  <el-input v-model="form.annual" :disabled="true" style="width: 100%;"></el-input>
                  <!-- <el-date-picker v-model="form.annual" type="year" value-format="yyyy" @change="annualChange" :disabled="">
                  </el-date-picker> -->
                </el-form-item>
              </el-col>
              <el-col :span="7">
                <el-form-item label="预算金额(元)：" prop="money">
                  <!-- <el-input v-model="form.money"></el-input> -->
                  <el-input-number v-model="form.money" :precision="2" :step="0.1" :controls="false" :disabled="true"
                    style="width: 100%;">
                  </el-input-number>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20">
              <el-col :span="24">
                <el-form-item label="预算金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" :disabled="isDisabled('remarks')" maxlength="5000">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20">
              <el-col :span="24">
                <div v-if="type == 'detail'">
                  <eleupload ref="eleupload" :showOnly="true" :attachFile="attachFile"></eleupload>
                </div>
                <div v-else-if="type == 'edit'">
                  <eleupload ref="eleupload" :attachFile="attachFile" @clearAllFile="clearAllFile" :uploadLimit="1">
                  </eleupload>
                </div>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
        <el-card class="box-card" style="margin-top:30px" shadow="never">
          <div>
            <h3 style="display:inline-block;margin-bottom:20px;">
              预算详情
            </h3>
            <el-button style="margin-left:30px" type="primary" icon="el-icon-plus" @click="addDepartPlan"
              v-if="type != 'detail'">新增</el-button>
          </div>
          <el-form :model="{ datas }" :rules="subRule" ref="subForm">
            <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;"
              shadow="never">
              <template slot="header">
                <div class="card-title" @click.stop.prevent="() => { }">
                  <el-form-item :prop="'datas.' + index + '.departId'" :rules="subRule.departId">
                    <el-select v-model="val.departId" placeholder="请选择" style="margin-right:10%;width: 210px;"
                      :disabled="isDisabled('department') || hasUseMoney(val)">
                      <el-option v-for="item in departOptions" :key="item.id" :label="item.name" :value="item.id"
                        :disabled="datas.findIndex(it => it.departId == item.id) > -1">
                      </el-option>
                      <el-option v-if="!departOptions.some(item => item.id == val.departId)" :label="val.departName"
                        :value="val.departId"></el-option>
                    </el-select>
                  </el-form-item>
                </div>
              </template>
              <div class="el-icon-close" style="position:absolute;right:30px;top:19px;" @click="deleteDepartPlan(index)"
                v-if="!hasUseMoney(val) && operaType == 'reEdit' && datas.length > 1">
              </div>
              <el-collapse v-model="val['activeNames']">
                <el-collapse-item name="1">
                  <el-table :data="val['budget']" :show-summary="true" sum-text="合计(元)" :summary-method="summary"
                    border>
                    <el-table-column type="index" label="编号" width="65">
                    </el-table-column>
                    <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType" width="240px">
                      <template slot-scope="scope">
                        <el-select v-model="scope.row.templateId" placeholder="请选择" style="width: 180px;"
                          v-if="treeData.find(item => item.id == val.departId)"
                          @change="vals => budgetTypeChange(vals, val.departId, scope.row)" :disabled="operaType != 'reEdit'">
                          <el-option v-for="item in treeData.find(item => item.id == val.departId)['children']"
                            :key="item.expenseBudgetTypeTmplId" :label="item.name" :value="item.expenseBudgetTypeTmplId"
                            :disabled="val['budget'].findIndex(it => it.templateId == item.expenseBudgetTypeTmplId) > -1">
                          </el-option>
                          <el-option v-if="!hasValue(val.departId, scope.row.templateId)" :label="scope.row.budgetType"
                            :value="scope.row.templateId"></el-option>
                        </el-select>
                        <el-select v-model="scope.row.templateId" v-else>
                          <el-option :label="scope.row.budgetType" :value="scope.row.templateId"></el-option>
                        </el-select>
                      </template>
                    </el-table-column>
                    <!-- <el-table-column prop="relateProjId" label="是否关联项目" width="320px">
                      <template slot-scope="scope">
                        <div>
                          <el-radio v-model="scope.row.isRelateProj" :label="true"
                            :disabled="isDisabled('isRelateProj')"
                            @change="v => projRadioChange(v, index, scope.$index)">是
                          </el-radio>
                          <el-radio v-model="scope.row.isRelateProj" :label="false"
                            :disabled="isDisabled('isRelateProj')"
                            @change="v => projRadioChange(v, index, scope.$index)">
                            否
                          </el-radio>
                          <el-select v-model="scope.row.relateProjId" placeholder="请选择" style="margin-right:10%;"
                            :disabled="!scope.row.isRelateProj || isDisabled('isRelateProj')" v-if="isFlow">
                            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name"
                              :value="item.id">
                            </el-option>
                          </el-select>
                          <el-select v-model="scope.row.relateProjId" placeholder="请选择" style="margin-right:10%;"
                            :disabled="true" v-else="isFlow">
                            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name"
                              :value="item.id">
                            </el-option>
                          </el-select>
                        </div>
                      </template>
                    </el-table-column> -->
                    <el-table-column prop="useMoney" label="已使用金额(元)">
                      <template slot-scope="scope">
                        <el-input v-model="scope.row.useMoney" maxlength="20" :disabled="true"></el-input>
                      </template>
                    </el-table-column>
                    <el-table-column prop="budgetMoney" label="本次追加(元)" width="180" v-if="tab > 1">
                      <template slot-scope="scope">
                        <el-input-number :precision="2" :step="0.1" :controls="false" type="number"
                          v-model="scope.row.budgetMoney" :disabled="isDisabled('appendMoney')"
                          v-focusSelect></el-input-number>
                      </template>
                    </el-table-column>
                    <!-- <el-table-column prop="budgetMoney" label="预算总额(万)" width="180" v-show="tab > 1">
                      <template slot-scope="scope">
                        <el-input-number :precision="6" :step="0.1" :controls="false"
                          :value="(scope.row.budgetMoney - 0) + (scope.row.appendMoney - 0)"
                          :disabled="scope.row.disabled"></el-input-number>
                      </template>
                    </el-table-column> -->
                    <el-table-column prop="budgetMoney" label="预算金额(元)" width="180" v-else>
                      <template slot-scope="scope">
                        <el-input-number :precision="2" :step="0.1" :controls="false" v-model="scope.row.budgetMoney"
                          :disabled="isDisabled('budgetMoney')" @blur="calculateTotalBudget" v-focusSelect
                          :min="isDisabled('budgetMoney') ? 0 : scope.row.useMoney"></el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="remarks" label="备注">
                      <template slot-scope="scope">
                        <div v-if="type == 'detail'">
                          {{ scope.row.remarks }}
                        </div>
                        <div v-else>
                          <el-input type="textarea" autosize v-model="scope.row.remarks"
                            :disabled="isDisabled('budgetMoney')"></el-input>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column fixed="right" label="操作" v-if="operaType == 'reEdit'" width="80">
                      <template slot-scope="scope">
                        <!-- <el-button @click="planDelete(index, scope.$index)" type="text" size="small">删除</el-button> -->
                        <el-button @click="planDelete(index, scope.$index, scope.row, val)" type="text" size="small"
                          v-if="!scope.row.useMoney"><i class="el-icon-delete-solid delete-icon"></i></el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(index)" v-if="operaType == 'reEdit'"></i>
                </el-collapse-item>
              </el-collapse>
            </el-card>
          </el-form>
        </el-card>
      </div>
    </div>
    <div class="footer-bt" v-if="!isFlow">
      <el-button @click="cancel" plain>关 闭</el-button>
    </div>
    <!-- <el-button @click="submit" plain>提 交</el-button> -->
  </div>
</template>

<script>
// import eleupload from "@/components/EleUpload";
import {
  localstorageGet,
} from '@/utils/auth';
import Api from '@/api';
import mixin from './mixins/mixin'
import { deepClone } from '@/utils/index'
import eleupload from "@/components/EleUpload";
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
import math from '@/utils/math.js'
const NUMCN = [
  '一', '二', '三', '四', '五', '六', '七', '八', '九', '十',
]
export default {
  name: "DetailBudgetComponent",
  mixins: [mixin],
  props: ['params', 'id', 'flowNodeProxyId', 'isExamine', 'operaType', 'flowInstanceId', 'flowProxyId', 'actionType','createrId'],
  components: { eleupload },
  data() {
    return {
      tab: '1',
      tabList: [],
      form: {
        type: '1'
      },
      dateOption: [],
      datas: [
      ],
      departOptions: [],
      projectOptions: [],
      attachFile: [],
      companyOption: [],
      enableData: [],
      addPlanTemp: {
        budgetDetailsId: '',
        budgetTypeId: '',
        budgetType: '',
        isRelateProj: false,
        relateProjName: '',
        relateProjId: '',
        budgetMoney: 0,
        appendMoney: 0,
        useMoney: 0,
        canDelete: true,
        templateId: '',
        companyId: localstorageGet('companyId'),
        status: 0,
        remarks: ''
      },
      groupDepartName: ''
    };
  },
  created() {
    this.type = 'detail'
    if (this.operaType && this.operaType == 'reEdit') this.type = 'edit'
    if (this.id) { //流程
      this.$bus.$off('company_annual_budget_before_handle')
      this.$bus.$on('company_annual_budget_before_handle', (val, that) => {
        this.busVal = val
        this.examneObj = that //把审核组件实例传过来,方便后面把loading取消
        this.submit()
      });
      this.getCompanyBudget().then(res => {
        if (res.isSuccess) {
          if (res.data.dataList.length) {
            let list = res.data.dataList[0]
            let params = deepClone(list)
            if (this.isFlow) {
              let len = params?.appendCostBudgetVos?.length || 0
              this.tab = len + 1
              this.editeType = 'init'
              if (len) {
                this.editeType = 'editAppend'
              }
            }
            this.$emit('updateParams', params)
            this.$nextTick(() => {
              this.radioChange(this.tab)
            })
          } else {
            this.$message.error('未找到预算数据')
            return
          }
        }
      }).then(() => {
        if (this.actionType == 'examine') {
          this.getInputPermision()
        } else if (this.actionType == 'create' || this.actionType == 'edit') {
          this.getPermision();
        } else if (this.actionType == 'preview') {
          this.enableData = []
        }
        this.getBudgetTypeOfGroup()
      })
    } else {
      this.radioChange(0)
      // this.getCompanyTree(this.params.companyId)
      this.getBudgetTypeOfGroup()
    }

  },
  mounted() {
  },
  computed: {
    isFlow() {
      if (this.id) {
        return true
      } else {
        return false
      }
    },
  },
  methods: {
    isDisabled(key) {
      const checkKey = key => {
        if (this.type == 'edit') {
          return false
        } else {
          let index = this.enableData.findIndex(item => item == key)
          if (index > -1) {
            return false
          } else {
            return true
          }
        }
      }
      if (this.isFlow) {
        if (this.tabList.length) { //有追加的情况
          if (this.tab - 1 == this.tabList.length) { //最后一次追加
            return checkKey(key)
          } else {
            return true
          }
        } else {
          return checkKey(key)
        }
      } else {
        //非流程，全部不可用
        return true
      }
    },
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
          for (let key in mainRule) {
            if (enableList.indexOf(key) == -1) {
              delete this.mainRule[key]
            }
          }
          this.enableData = enableList
        }
      );
    },
    budgetTypeChange(val, departId, obj) {
      let find = this.treeData.find(item => item.id == departId)
      if (find && find.children && find.children.length) {
        let findType = find.children.find(el => el.expenseBudgetTypeTmplId == val)
        if (findType) {
          let name = findType.name
          obj.budgetType = name
        }
      }
    },
    hasUseMoney(val) {
      let budget = val?.budget || []
      let hasUseMoney = false
      for (var val of budget) {
        if (val.useMoney > 0) {
          hasUseMoney = true
          break
        }
      }
      return hasUseMoney
    },
    // departChange(idx) {
    //   this.departOptions.forEach(item => {
    //     item.hasSelect = false;
    //   });
    //   this.datas.forEach(item => {
    //     const departId = item.departId;
    //     const index = this.departOptions.findIndex(it => it.id == departId);
    //     if (index > -1) {
    //       this.$set(this.departOptions[index],'hasSelect',true)
    //     }
    //   });

    //   //清除归口
    //   if(idx!==undefined){
    //     this.datas[idx].budget = []
    //     this.addPlan(idx)
    //   }
    // },
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
    },
    addPlan(index) {
      const plan = deepClone(this.addPlanTemp);
      this.datas[index].budget.push(plan);
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
          if (res.data && res.data.flowNodeTemplate && res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
          }
          let mainRule = deepClone(this.mainRule)
          for (let key in mainRule) {
            if (enableList.indexOf(key) == -1) {
              delete this.mainRule[key]
            }
          }
          this.enableData = enableList
        }
      );
    },
    // getPermision() {
    //   this.$axios.post(
    //     Api.qualityManage.findApprovePermission,
    //     {
    //       data: {

    //       },
    //       nodeProxyId: this.flowNodeProxyId
    //     },
    //     (res) => {
    //       let enableList = []
    //       if (res.data && res.data.flowNodeFieldPowerTemplateList) {
    //         let tmpList = res.data.flowNodeFieldPowerTemplateList || []
    //         enableList = tmpList.map(item => {
    //           return item.formFieldTemplateEnglishName
    //         })
    //       }
    //       this.enableData = enableList
    //     }
    //   );
    // },
    getCompanyBudget() {
      let query = {
        id: this.id,
      }
      return this.$axios.post(
        Api.annualBudget.findBudgetById,
        {
          data: query,
        }
      );
    },
    initData(key, idx) {

      if (JSON.stringify(this.params) == '{}') {
        return false
      }
      let datas = []
      let budgetDetailsVos = []
      let params = {}
      // let obj = {}
      if (key == 'budgetDetailsVos') {
        this.currentBudget = params = this.params
        budgetDetailsVos = params.budgetDetailsVos
        //如果是有项目预算
        if (params.costBudgetVoList && params.costBudgetVoList.length) {
          //排序
          params.costBudgetVoList.sort((a, b) => { return a.sort - b.sort })
          params.costBudgetVoList.forEach(item => {
            let costBudgetId = item.id
            let costBudgetDetailsVos = item?.budgetDetailsVos || []
            costBudgetDetailsVos.forEach(el => {
              el.costBudgetId = costBudgetId
              budgetDetailsVos.push(el)
            })
          })
        }
        this.initBudget = budgetDetailsVos// = params.budgetDetailsVos
      } else {
        this.currentBudget = params = this.params.appendCostBudgetVos[idx]
        budgetDetailsVos = params.appendBudgetDetailsVos
      }
      this.bizId = params.id
      this.getFileByBizId(this.bizId)

      this.form = {
        budgetId: this.params.id,
        companyId: this.params.companyId,
        companyName: this.params.companyName,
        annual: this.params.budgetTime.substr(0, 4),
        money: math.multiply(params.money, 10000),
        remarks: params.remarks,
        type: params.type
      }
      for (let i = 0; budgetDetailsVos[i]; i++) {
        let departId = budgetDetailsVos[i].departmentId
        let index = datas.findIndex(item => item.departId == departId)
        let budgetTypeVo = budgetDetailsVos[i].budgetTypeVo
        let isRelateProj = budgetDetailsVos[i].projectId ? true : false
        let useMoney = budgetDetailsVos[i].useMoney || 0
        let tmp = {
          budgetDetailsId: budgetDetailsVos[i].budgetDetailsId, //|| budgetDetailsVos[i].id,
          budgetTypeId: budgetTypeVo.id,
          budgetType: budgetTypeVo.name,
          isRelateProj,
          relateProjName: budgetDetailsVos[i].projectName,
          relateProjId: budgetDetailsVos[i].projectId,
          budgetMoney: math.multiply(budgetDetailsVos[i].money, 10000),
          useMoney: math.multiply(useMoney, 10000),//budgetDetailsVos[i].useMoney || 0,
          appendMoney: 0,
          canDelete: false,
          disabled: true,
          templateId: budgetTypeVo.templateId,
          companyId: budgetTypeVo.companyId,
          departName: budgetTypeVo.departmentName,
          status: budgetTypeVo.status,
          remarks: budgetDetailsVos[i].remarks
        }
        if (budgetDetailsVos[i].costBudgetId) tmp.costBudgetId = budgetDetailsVos[i].costBudgetId
        if (index > -1) {
          datas[index].budget.push(tmp)
        } else {
          let obj = {
            planId: this.datas.length + 1,
            departName: this.getDepartNameById(budgetDetailsVos[i].departmentId) || budgetDetailsVos[i].budgetTypeVo.departmentName || budgetDetailsVos[i].projectName,
            departId: budgetDetailsVos[i].departmentId,
            activeNames: '1',
            disabled: true
          }
          obj.budget = []
          obj.budget.push(tmp)
          datas.push(obj)
        }
      }
      //集团部门预算
      // console.log('this.params',this.params)
      if (params.type == 6 || params.type == 4) {
        this.groupDepartId = params.projectId//find.departmentId
        this.groupDepartName = params.departmentName == '公司领导' ? '公司固定费用' : params.departmentName //find.budgetTypeVo.departmentName
        // if(params.budgetDetailsVos && params.budgetDetailsVos.length){
        //   let find = params.budgetDetailsVos.find(el=>el.budgetTypeVo.departmentName != '公司固定费用')
        //   if(find){

        //   }
        // }
      }
      this.datas = datas
      let appendCostBudgetVos = this.params?.appendCostBudgetVos || []
      this.tabList = appendCostBudgetVos.map((item, i) => {
        let obj = {
          label: `第${NUMCN[i]}次追加`,
          index: i
        }
        return obj
      })
    },
    radioChange(val) {
      let len, isShowLog = false
      if (val > 1) {
        let index = val - 2
        this.initData('appendCostBudgetVos', index)
        len = this.params?.appendCostBudgetVos?.length || 0
      } else {
        len = 0
        this.initData('budgetDetailsVos')
      }
      if (this.isFlow) { //在流程里面，需要处理是否显示流程日志的问题
        if (val - 1 == len) {
          isShowLog = true
        } else {
          isShowLog = false
        }
        this.$emit('showLog', isShowLog)
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
            let treeData = res.data[0] || {};
            this.originDepartData = {}
            if (treeData && treeData.childrenList) {
              this.originDepartData = treeData.childrenList
              this.getCompanyListOfOnDuty().then(resp => {
                if (resp.isSuccess) {
                  this.companyOption = resp.data
                  this.companyChange(id)
                  this.getProjectVosByCompanyId(id)
                } else {
                  //没有在任何公司任职
                }
              })
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getCompanyListOfOnDuty() {
      return this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {})
    },
    // projRadioChange(val, i, j) {
    //   if (val) { // 关联项目
    //     const companyId = this.form.companyId;
    //     if (!companyId) {
    //       this.datas[i].budget[j].isRelateProj = false;
    //       return this.$message.error('请先选择公司，再关联项目');
    //     }
    //     this.datas[i].budget[j].isRelateProj = true;
    //     // this.getProjectVosByCompanyId(companyId);
    //   } else { // 不关联
    //     this.datas[i].budget[j].relateProjId = '';
    //   }
    // },
    submit(st) {
      var status = 0
      if (st === undefined) status = 1
      // console.log('this.form', this.form)
      // console.log('this.tab', this.tab, this.tabList)
      // let
      if (this.tabList.length + 1 != this.tab) {
        this.tab = this.tabList.length + 1
        this.radioChange(this.tab)
      }
      // console.log('this.datas', this.datas)
      // return
      // console.log('this.params', this.params)
      let data = {
        companyId: this.form.companyId,
        projectId: '',
        money: this.form.money,
        remarks: this.form.remarks,
        enclosure: this.form.enclosure,
        budgetTime: `${this.form.annual}-01-01 00:00:00`,
        examineStatus: "0",
        status,
        type: this.form.type,
        id: this.currentBudget.id,
        budgetId: this.params.id,
        sort: 0,
        costBudgetVoList: []
      }
      if (this.form.type == 4 || this.form.type == 6) {
        data.projectId = this.groupDepartId
        data.departmentName = this.groupDepartName
      }
      let len = this.params?.appendCostBudgetVos?.length || 0
      if (len) data.sort = len
      let totalBueget = 0, arr = [], budgetDetailsVos = [], costBudgetVoList = []
      let childrenSort = 0, err = false
      this.datas.forEach(item => {
        const budget = item.budget;
        budget.forEach((el, j) => {
          // if (el.budgetMoney > 0) {
            const tmpObj = {};
            childrenSort += 1
            tmpObj.budgetDetailsId = el.budgetDetailsId || '';
            tmpObj.budgetTypeId = el.budgetTypeId || '';
            tmpObj.departmentId = item.departId;
            tmpObj.type = '1'
            tmpObj.projectId = el.relateProjId;
            tmpObj.money = el.budgetMoney || 0;
            tmpObj.remarks = el.remarks
            tmpObj.sort = childrenSort
            totalBueget = math.add(totalBueget, (tmpObj.money - 0))
            tmpObj.budgetTypeVo = {};
            const budgetTypeVo = {};
            budgetTypeVo.templateId = el.templateId;
            budgetTypeVo.name = budget[j].budgetType;
            if (budgetTypeVo.templateId == '') {
              this.$message.error('所有费用预算类型均为必填');
              err = true
              return false;
            }
            if (el.useMoney > 0) {
              if (el.useMoney > el.budgetMoney) {
                let departName = el.departName == '公司领导' ? '公司固定费用' : el.departName
                this.$message.error(`${departName || ''}归口[${el.budgetType || ''}]预算金额应该大于归口已使用金额`);
                err = true
                return false;
              }
            }
            // let idx = budget.findIndex((el, k) => k != j && el.budgetType == budgetTypeVo.name)
            // if (idx > -1) {
            //   this.$message.error('相同部门下不可有相同归口');
            //   return false
            // }
            budgetTypeVo.parentId = 1;
            budgetTypeVo.departmentId = item.departId;
            budgetTypeVo.type = '1';
            budgetTypeVo.annually = this.form.annual;
            budgetTypeVo.status = 0;// this.form.status
            budgetTypeVo.departmentName = el.departName || this.getDepartNameById(item.departId),
              budgetTypeVo.companyId = this.form.companyId
            budgetTypeVo.templateId = el.templateId
            budgetTypeVo.id = el.budgetTypeId || ''
            budgetTypeVo.status = el.status || 0
            tmpObj.budgetTypeVo = budgetTypeVo;


            if (this.isProjectBudget(item.departId)) {
              tmpObj.projectId = item.departId
              tmpObj.costBudgetId = el.costBudgetId
              budgetTypeVo.type = 3
              budgetTypeVo.projectId = tmpObj.projectId
              costBudgetVoList.push(deepClone(tmpObj))
            } else {
              budgetDetailsVos.push(deepClone(tmpObj));
            }

            // arr.push(deepClone(tmpObj));
          // }
        })
      })
      if (err) return false
      data.money = totalBueget
      // if (totalBueget.toFixed(6) != data.money.toFixed(6)) {
      //   this.$message.error('预算总金额和预算详情金额不符');
      //   let obj = {
      //     status: 'fail',
      //     val: this.busVal
      //   }
      //   this.$bus.$emit('submitBeforeHandleOk', obj);
      //   return false;
      // }
      if (this.editeType == 'editAppend') {
        data.appendBudgetDetailsVos = budgetDetailsVos //追加这里后面要改TODO
      } else {
        data.budgetDetailsVos = budgetDetailsVos
      }
      let budgetDetailsVosLen = budgetDetailsVos.length || 0
      if (costBudgetVoList && costBudgetVoList.length) { //如果有项目预算
        costBudgetVoList.forEach((el, index) => {
          let projectId = el.projectId
          el.type = 3
          let find = data.costBudgetVoList.find(it => it.projectId == projectId)
          if (find) {
            find.budgetDetailsVos.push(el)
          } else {
            data.costBudgetVoList.push({
              id: el.costBudgetId,
              projectId: el.projectId,
              type: '3',
              companyId: localstorageGet('companyId'),
              money: 0, //遍历汇总
              status: data.status, // 0草稿1提交
              examineStatus: data.examineStatus,
              budgetTime: data.budgetTime,
              sort: Number(budgetDetailsVosLen) + Number(index),
              budgetDetailsVos: [el]
            })
          }
        })
        //计算money
        data.costBudgetVoList.forEach(item => {
          item.money = item.budgetDetailsVos.reduce((prev, cur) => {
            return prev + cur.money
          }, 0)
        })
      } else {
        data.costBudgetVoList = []
      }
      data.companyBudgetVos = []
      this.postData(data)
    },
    postData(data) {
      // console.log('this.$refs.eleupload.getFileId();',this.$refs.eleupload.getFileId())
      const eleuploadFile = this.$refs.eleupload.getFileId();
      // let action = Api.annualBudget.initBudgetUpdate;
      let action = Api.annualBudget.costBudgetSaveTask;
      if (this.editeType == 'editAppend') {
        action = Api.annualBudget.costBudgetUpdate;
      }
      // console.log('this.editeType', this.editeType)
      // console.log('action', action)
      // return
      // if(this.isGroupMember && this.actionType == 'create'){
      //   data.type = 6
      //   data.projectId = this.params.projectId
      // }else{
      //   data.type = 1
      // }
      //转换金额
      let pData = deepClone(data)
      this.transeMoney(pData)
      this.$axios.post(action, { data: pData }).then(async res => {
        if (eleuploadFile.length) {
          // 绑定文件
          const relationId = data.id;
          const fileId = eleuploadFile[0];
          this.bindFileById(relationId, fileId).then(r => { });
        }
        let userName = localstorageGet('userName')
        if(this.createrId){
          let userInfo = await this.getUserInfo()
          if(userInfo)userName = userInfo.name
        }
        if (data.status == 1) {
          // let money = math.multiply(data.money, 10000)
          let money = data.money
          let obj = {
            status: 'success',
            val: this.busVal,
            total: money,
            id: data.id,
            name: `${this.form.annual}公司年度预算￥${money}元-${userName}`
          }
          if (!res.isSuccess) {
            obj.status = 'fail'
          }
          this.$bus.$emit('submitBeforeHandleOk', obj);
        } else {
          this.$bus.$emit('close')
        }
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
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
