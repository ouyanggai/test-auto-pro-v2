<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>公司年度预算编辑</h2>
      <div class="inner-container">
        <el-card class="box-card" shadow="never">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row :gutter="20" style="margin-bottom:30px">
              <el-col :span="11">
                <el-form-item :label="params.type == 4 || params.type == 6 ?'部门名称：': '公司名称：'">
                  <!-- <el-input v-model="form.companyName"></el-input> -->
                  <el-input v-model="groupDepartName" :disabled="true" v-if="params.type == 4 || params.type == 6" style="width: 100%;"></el-input>
                  <el-select v-model="form.companyId" placeholder="请选择" style="margin-right:6%;width: 100%;" :disabled="true" v-else>
                    <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="预算年度：">
                  <!-- <el-input v-model="form.annual"></el-input> -->
                  <el-date-picker v-model="form.annual" type="year" value-format="yyyy"  style="width: 100%;" :disabled="true">
                  </el-date-picker>
                </el-form-item>
              </el-col>
              <el-col :span="7">
                <el-form-item label="预算金额(元)：" prop="money">
                  <!-- <el-input v-model="form.money"></el-input> -->
                  <el-input-number v-model="form.money" :min="0.00" :precision="2" :step="0.1" :controls="false" :disabled="true" style="width: 100%;">
                  </el-input-number>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20" style="margin-bottom:30px">
              <el-col :span="24">
                <el-form-item label="预算金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" maxlength="5000">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="20">
              <el-col :span="24">
                <eleupload ref="eleupload" :attachFile="attachFile" @clearAllFile="clearAllFile" :uploadLimit="1"></eleupload>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
        <el-card class="box-card" style="margin-top:30px" shadow="never">
          <div>
            <h3 style="display:inline-block;margin-bottom:20px;">
              预算详情
            </h3>
            <el-button style="margin-left:30px" type="primary" icon="el-icon-plus" @click="addDepartPlan">新增</el-button>
          </div>
          <el-form :model="{ datas }" :rules="subRule" ref="subForm">
            <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;"
              shadow="never">
              <template slot="header">
                <div class="card-title" @click.stop.prevent="() => { }">
                  <el-form-item :prop="'datas.' + index + '.departId'" :rules="subRule.departId">
                    <el-select v-model="val.departId" placeholder="请选择" style="margin-right:10%;width: 240px;" @change="v=>departChange(index,v)"
                      :disabled="val.isOrigin || hasUseMoney(val)">
                      <el-option v-for="(item,i) in departOptions" :key="i" :label="item.name" :value="item.id"
                        :disabled="datas.findIndex(it=>it.departId == item.id) > -1">
                      </el-option>
                      <el-option v-if="val.departId && !departOptions.some(item=>item.id == val.departId )" :label="val.departName" :value="val.departId"></el-option>
                    </el-select>
                  </el-form-item>
                </div>
              </template>
              <div class="el-icon-close" @click="deleteDepartPlan(index)" v-if="!val['isOrigin']">
              </div>
              <el-collapse v-model="val['activeNames']">
                <el-collapse-item name="1">
                  <el-table :data="val['budget']" :show-summary="true" sum-text="合计(元)" :summary-method="summary"
                    border>
                    <el-table-column type="index" label="编号" width="65px">
                    </el-table-column>
                    <el-table-column prop="templateId" label="费用预算类型（一级）" class-name="budgetType"  width="240px">
                      <template slot-scope="scope">
                        <el-select v-model="scope.row.templateId" placeholder="请选择" style="width: 180px;"
                          v-if="treeData.find(item=>item.id == val.departId)" @change="vals=>budgetTypeChange(vals,val.departId,scope.row)">
                          <el-option
                            v-for="item in treeData.find(item=>item.id == val.departId)['children']"
                            :key="item.expenseBudgetTypeTmplId"
                            :label="item.name"
                            :value="item.expenseBudgetTypeTmplId"
                            :disabled="val['budget'].findIndex(it=>it.templateId == item.expenseBudgetTypeTmplId) > -1"
                            >
                          </el-option>
                          <el-option v-if="!hasValue(val.departId,scope.row.templateId)" :label="scope.row.budgetType" :value="scope.row.templateId"></el-option>
                        </el-select>
                        <el-select  v-model="scope.row.templateId" v-else>
                          <el-option :label="scope.row.budgetType" :value="scope.row.templateId"></el-option>
                        </el-select>
                      </template>
                    </el-table-column>
                    <!-- <el-table-column prop="relateProjId" label="是否关联项目" width="320px">
                      <template slot-scope="scope">
                        <div>
                          <el-radio v-model="scope.row.isRelateProj" :label="true"
                            @change="v => radioChange(v, index, scope.$index)" :disabled="scope.row.isOrigin">是
                          </el-radio>
                          <el-radio v-model="scope.row.isRelateProj" :label="false"
                            @change="v => radioChange(v, index, scope.$index)" :disabled="scope.row.isOrigin">
                            否
                          </el-radio>
                          <el-select v-model="scope.row.relateProjId" placeholder="请选择" style="margin-right:10%;"
                            :disabled="!scope.row.isRelateProj || scope.row.isOrigin">
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
                    <el-table-column prop="appendMoney" label="本次追加(元)" width="160" v-if="editeType == 'append'">
                      <template slot-scope="scope">
                        <el-input-number v-model="scope.row.appendMoney" :precision="2" :step="0.1" :controls="false" v-focusSelect>
                        </el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="budgetMoney" label="预算总额(元)" width="160" v-if="editeType == 'append'"
                      :disabled="true">
                      <template slot-scope="scope">
                        <el-input-number :value="(scope.row.budgetMoney - 0)" :disabled="true" :precision="2"
                          :step="0.1" :controls="false"></el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="budgetMoney" label="预算金额(元)" width="160" v-else-if="editeType == 'init'">
                      <template slot-scope="scope">
                        <el-input-number v-model="scope.row.budgetMoney" :disabled="scope.row.isOrigin" :precision="2"
                          :step="0.1" :controls="false"  @blur="calculateTotalBudget" v-focusSelect :min="scope.row.useMoney">
                        </el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="remarks" label="备注" >
                      <template slot-scope="scope">
                        <el-input v-model="scope.row.remarks" maxlength="50"></el-input>
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="80px" fixed="right">
                      <template slot-scope="scope">
                        <span v-if="!scope.row.useMoney">
                          <el-button @click="planDelete(index, scope.$index,scope.row,val)" type="text" size="small"
                            v-if="scope.row.canDelete || paramsInfo.examineStatus == 0"><i class="el-icon-delete-solid delete-icon"></i></el-button>
                        </span>
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
import math from '@/utils/math.js'
const NUMCN = [
  '一', '二', '三', '四', '五', '六', '七', '八', '九', '十'
];
export default {
  name: 'EditeBudget',
  mixins: [mixin],
  props: ['paramsInfo', 'type','createrId'],
  components: { eleupload, PersonSelectDialog, BranchChoose, BranchPralleChoose },
  data() {
    return {
      tab: '1',
      form: {
        type: '1',
        groupDepartId:''
      },
      dateOption: [],
      datas: [
      ],
      params:{},
      companyOption: [],
      departOptions: [],
      addPlanTemp: {
        budgetDetailsId: '',
        budgetTypeId: '',
        budgetType: '',
        templateId: '',
        companyId: localstorageGet('companyId'),
        isRelateProj: false,
        relateProjName: '',
        relateProjId: '',
        budgetMoney: 0,
        appendMoney: 0,
        status: 0,
        canDelete: true,
        remarks: ''
      },
      tabList: [],
      editeType: '',
      // selectFlowType: 'edit_company_annual_budget',
      attachFile: [],

      nodeChooseVisible: false,
      checkboxPersonGroup: [],
      flowId: '',
      groupDepartName:''
    };
  },
  created() {
    this.initSelection();
    this.getBudgetTypeOfGroup();
  },
  inject: ['prevStepHandle', 'sumbitFlow', 'submitFlowFinal'],
  mounted() {
  },
  methods: {
    hasUseMoney(val) {
      const budget = val?.budget || [];
      let hasUseMoney = false;
      for (var val of budget) {
        if (val.useMoney > 0) {
          hasUseMoney = true;
          break;
        }
      }
      return hasUseMoney;
    },
    budgetTypeChange(val, departId, obj) {
      const find = this.treeData.find(item => item.id == departId);
      if (find && find.children && find.children.length) {
        const findType = find.children.find(el => el.expenseBudgetTypeTmplId == val);
        if (findType) {
          const name = findType.name;
          obj.budgetType = name;
        }
      }
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
          if (data && data.dataList && data.dataList.length) {
            var list = data.dataList[0];
            if(this.isGroupMember){
              let find = data.dataList.find(item=>item.budgetDetailsVos.find(el=>el.departmentId == this.form.groupDepartId))
              if(find)list = find
              else{
                //该集团部门没有预算
                return
              }
            }
            if (list.status == 0) {
              this.$confirm(`该公司在${this.form.annual}年度的预算有已保存的草稿，是否要载入数据`, '提示', {
                closeOnClickModal: false,
                confirmButtonText: '确定',
                cancelButtonText: '重新选择',
                type: 'warning'
              }).then(() => {
                this.toEditPage(list);
              }).catch(() => {
                // if (!this.disableCompany) {
                //   this.form.companyId = '';
                // }
                this.datas = []
                this.addDepartPlan()
                this.form.annual = '';
              });
            } else if (list.status == 1 && list.examineStatus == 2) { // 该预算被驳回
              // this.toEditPage(list);
              this.$confirm(`该公司在${this.form.annual}年度的预算被驳回，是否重新编辑`, '提示', {
                closeOnClickModal: false,
                confirmButtonText: '确定',
                cancelButtonText: '重新选择',
                type: 'warning'
              }).then(() => {
                this.toEditPage(list);
              }).catch(() => {
                // if (!this.disableCompany) {
                //   this.form.companyId = '';
                // }
                this.datas = []
                this.addDepartPlan()
                this.form.annual = '';
              });
            } else if (list.status == 2) {
              // 系统生成的预算
              this.toEditPage(list);
            } else {
              this.$alert(`该公司在${this.form.annual}年度已有预算，请重新选择公司或者年份`, '提示').then(() => {
                // if (!this.disableCompany) {
                //   this.form.companyId = '';
                // }
                this.form.annual = '';
              }).catch(() => {
                if (!this.disableCompany) {
                  this.form.companyId = '';
                }
                this.datas = []
                this.addDepartPlan()
                this.form.annual = '';
              });
            }
          } else {
            this.toEditPage({ annual: this.form.annual });
          }
        }
        // console.log('res', res)
      });
    },
    async initSelection() { // 初始化下拉数据
      await this.getParentCompanyList();
      // this.getCompanyListOfOnDuty().then(res => {
      //   if (res.isSuccess) {
      //     let dutyCompanyOption = res.data;
      //     let companyOptions = []
      //     dutyCompanyOption.forEach(item => {
      //       let dutyCompanyId = item.id
      //       let index = this.companyOption.findIndex(el => el.id == dutyCompanyId)
      //       if (index > -1) {
      //         companyOptions.push(deepClone(this.companyOption[index]))
      //       }
      //     })
      //     this.companyOption = companyOptions
      //   } else {
      //     this.$message.error('人员暂无公司岗位')
      //   }
      // })
      this.getCompanyTree(this.paramsInfo.companyId);

      this.initData();
    },
    getCompanyListOfOnDuty() {
      return this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {});
    },
    async initData(list) {
      let params;
      if (list) {
        params = this.params = deepClone(list);
      } else {
        params = this.params = deepClone(this.paramsInfo);
      }
      params.examineStatus = 0;
      let formData;
      if (!params.appendCostBudgetVos || (params.appendCostBudgetVos && params.appendCostBudgetVos.length == 0)) {
        // 没有追加预算，当前预算就是不通过的那一条，取他的外围数据作为form
        this.formData = formData = params;
        // this.editeType 当前编辑的预算类型 初始init/追加append
        this.editeType = 'init';
        // this.selectFlowType = 'edit_company_annual_budget'
      } else {
        // 有追加
        // 获取追加的对象
        const appendCostBudgetVos = params?.appendCostBudgetVos || [];
        let nopassIndex = appendCostBudgetVos.findIndex(el => el.examineStatus == 2);
        if (nopassIndex == -1) {
          // 没有审批驳回，那就是草稿状态，再次进入页面做审核 最后一条就是当前需要编辑的预算
          nopassIndex = appendCostBudgetVos.length - 1;
        }
        // console.log('nopassIndex', nopassIndex)
        const currentAppendCostBudgetVos = appendCostBudgetVos[nopassIndex];
        this.formData = formData = currentAppendCostBudgetVos;
        this.editeType = 'append';
        // this.selectFlowType = 'edit_add_annual_budget'
      }
      this.bizId = formData.id;
      this.getFileByBizId(this.bizId);
      this.form = {
        budgetId: this.params.id,
        companyId: this.params.companyId,
        companyName: this.params.companyName,
        annual: this.params.budgetTime.substr(0, 4),
        money: math.multiply(formData.money,10000),
        remarks: formData.remarks,
        type:formData.type
      };
      const datas = [];
      // let obj = {}
      const budgetDetailsVos = this.params.budgetDetailsVos;
      if(params.costBudgetVoList && params.costBudgetVoList.length){
          //排序
          params.costBudgetVoList.sort((a, b) => {return a.sort - b.sort})
          params.costBudgetVoList.forEach(item=>{
            let costBudgetId = item.id
            let costBudgetDetailsVos = item?.budgetDetailsVos || []
            costBudgetDetailsVos.forEach(el=>{
              el.costBudgetId = costBudgetId
              budgetDetailsVos.push(el)
            })
          })
        }
      for (let i = 0; budgetDetailsVos[i]; i++) {
        const departId = budgetDetailsVos[i].departmentId;
        const index = datas.findIndex(item => item.departId == departId);
        const budgetTypeVo = budgetDetailsVos[i].budgetTypeVo;
        const isRelateProj = !!budgetDetailsVos[i].projectId;
        let budgetMoney = budgetDetailsVos[i].money || 0
        let useMoney = budgetDetailsVos[i].useMoney || 0
        const tmp = {
          // id: budgetDetailsVos[i].id,
          budgetId: budgetDetailsVos[i].budgetId,
          budgetDetailsId: budgetDetailsVos[i].id,
          budgetTypeId: budgetTypeVo.id,
          budgetType: budgetTypeVo.name,
          templateId: budgetTypeVo.templateId,
          companyId: budgetTypeVo.companyId,
          departName: budgetTypeVo.departmentName,
          status: budgetTypeVo.status,
          isRelateProj,
          relateProjName: budgetDetailsVos[i].projectName,
          relateProjId: budgetDetailsVos[i].projectId,
          budgetMoney:  math.multiply(budgetMoney,10000),//budgetDetailsVos[i].money || 0, // + appendMoneys,
          appendMoney: 0,
          useMoney: math.multiply(useMoney,10000),
          isOrigin: false,
          remarks: budgetDetailsVos[i].remarks
          // canDelete: true,
          // disabled: false
        };
        if(budgetDetailsVos[i].costBudgetId)tmp.costBudgetId = budgetDetailsVos[i].costBudgetId
        if (this.params.appendCostBudgetVos && this.params.appendCostBudgetVos.length) {
          tmp.isOrigin = true;
        }
        if (index > -1) {
          datas[index].budget.push(tmp);
        } else {
          const obj = {
            planId: datas.length + 1,
            departName: this.getDepartNameById(budgetDetailsVos[i].departmentId) ||  budgetDetailsVos[i].budgetTypeVo.departmentName ||  budgetDetailsVos[i].projectName,
            // departName: this.getDepartNameById(budgetDetailsVos[i].departmentId),
            departId: budgetDetailsVos[i].departmentId,
            activeNames: '1',
            isOrigin: false
          };
          if (this.params.appendCostBudgetVos && this.params.appendCostBudgetVos.length) {
            obj.isOrigin = true;
          }
          obj.budget = [];
          obj.budget.push(tmp);
          datas.push(obj);
        }
      }
      // console.log('data',datas)
      // 追加的预算
      if (this.params.appendCostBudgetVos && this.params.appendCostBudgetVos.length > 0) {
        const appendCostBudgetVosArr = this.params.appendCostBudgetVos || [];
        const len = appendCostBudgetVosArr.length - 1;
        for (let i = len; appendCostBudgetVosArr[i]; i--) {
          const appendBudgetDetailsVos = appendCostBudgetVosArr[i].appendBudgetDetailsVos || {};
          for (let j = 0; appendBudgetDetailsVos[j]; j++) {
            const budgetTypeId = appendBudgetDetailsVos[j].budgetTypeId;
            const obj = this.findBudgetById(datas, budgetTypeId);
            if (obj.has) {
              const appends = appendBudgetDetailsVos[j].money;
              if (appendCostBudgetVosArr[i].examineStatus == 2 || appendCostBudgetVosArr[i].status == 0) {
                datas[obj.x].budget[obj.y].appendMoney = appends;
              } else {
                datas[obj.x].budget[obj.y].budgetMoney = math.add(datas[obj.x].budget[obj.y].budgetMoney, (appends - 0))
              }
            } else {
              const isRelateProj = !!appendBudgetDetailsVos[j].projectId;
              const tmp = {
                // id: appendBudgetDetailsVos[j].id,
                budgetId: appendBudgetDetailsVos[j].budgetId,
                budgetDetailsId: appendBudgetDetailsVos[j].id,
                budgetTypeId: appendBudgetDetailsVos[j].budgetTypeId,
                budgetType: appendBudgetDetailsVos[j].budgetTypeVo.name,
                templateId: appendBudgetDetailsVos[j].budgetTypeVo.templateId,
                isRelateProj,
                relateProjName: appendBudgetDetailsVos[j].projectName,
                relateProjId: appendBudgetDetailsVos[j].projectId,
                budgetMoney: appendBudgetDetailsVos[j].money || 0, // + appendMoneys,
                appendMoney: 0,
                isOrigin: true,
                status: appendBudgetDetailsVos[j].status
                // canDelete: false,
                // disabled: true
              };
              const departId = appendBudgetDetailsVos[j].budgetTypeVo.departmentId;
              const idx = datas.findIndex(el => el.departId == departId);
              if (appendCostBudgetVosArr[i].examineStatus == 2 || appendCostBudgetVosArr[i].status == 0) {
                tmp.appendMoney = tmp.budgetMoney;
                tmp.budgetMoney = 0;
                tmp.isOrigin = false;
              }
              if (idx > -1) {
                datas[idx].budget.push(tmp);
              } else {
                const obj = {
                  planId: datas.length + 1,
                  departName: this.getDepartNameById(departId),
                  companyId: this.form.companyId,
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
      }

      //如果是集团部门预算
      if(params.type == 6 || params.type == 4){
        this.groupDepartId = params.projectId
        this.groupDepartName = params.departmentName == '公司领导'?'公司固定费用':params.departmentName //find.budgetTypeVo.departmentName
        this.params.projectId = this.groupDepartId
        this.params.departmentName = this.groupDepartName
      }
      this.datas = datas;
      // await this.getFlowType()
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
            flag: 2,
            id: localstorageGet('companyId') // 公司id
          }
        },
        async res => {
          if (res.isSuccess) {
            const treeData = res.data[0] || {};
            this.originDepartData = {};
            if (treeData && treeData.childrenList) {
              this.originDepartData = treeData.childrenList;
              // console.log('this.originDepartData->', this.originDepartData)
              // this.companyChange(id);
              // console.log('id',id)
              // this.getProjectVosByCompanyId(id);
              this.departChange();
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },

    async getParentCompanyList() { // 查询公司列表
      await this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id: localstorageGet('companyId') // 当前用的公司id
          }
        },
        res => {
          this.companyOption = res.data;
        }
      );
    },
    toEditPage(list) { // 转到新建
      this.$emit('changeComponent', { list, type: 'company_annual_budget' });
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
      this.$nextTick(() => {
        var flowContent = document.querySelector('.outer .container')
        flowContent.style.scrollBehavior = 'smooth'
        // let chidlHeight = flowContent.clientHeight
        flowContent.scrollTop = 10000
      })
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
        if(!res){
          this.$message.error('有必填项未填')
          this.$parent.$parent.$parent.submitLoading = false
          err = true;
        }
      });
      if (err) return false;
      this.$refs.subForm.validate(res => {
        if(!res){
          this.$message.error('有必填项未填')
          this.$parent.$parent.$parent.submitLoading = false
          err = true;
        }
      });
      if (err) return false;
      if (!this.datas.length) {
        this.$parent.$parent.$parent.submitLoading = false
        return this.$message.error('预算不可为空');
      }
      this.params.status = status;
      this.params.type = this.form.type;
      let editAppend;
      let processDataRes;
      if (this.editeType == 'append') {
        //追加的后面做，有问题 TODO
        editAppend = this.params.appendCostBudgetVos[this.params.appendCostBudgetVos.length - 1];
        editAppend.appendBudgetDetailsVos = [];
        processDataRes = this.processPostData();
        if (!processDataRes) return false;
        editAppend.appendBudgetDetailsVos = processDataRes;
      } else {
        editAppend = this.params;
        processDataRes = this.processPostData();
        if (!processDataRes) return false;
        const {budgetDetailsVos,costBudgetVoList} = processDataRes
        // if (!processDataRes) return false;
        editAppend.budgetDetailsVos = budgetDetailsVos;
        editAppend.costBudgetVoList = []
        let budgetDetailsVosLen = budgetDetailsVos.length || 0
        if(costBudgetVoList && costBudgetVoList.length){ //如果有项目预算
          costBudgetVoList.forEach((el,index)=>{
            let projectId = el.projectId
            el.type = 3
            let find = editAppend.costBudgetVoList.find(it=>it.projectId == projectId)
            if(find){
              find.budgetDetailsVos.push(el)
            }else{
              editAppend.costBudgetVoList.push({
                id:el.costBudgetId,
                projectId:el.projectId,
                type:'3',
                companyId: localstorageGet('companyId'),
                money:0, //遍历汇总
                status: this.params.status, // 0草稿1提交
                examineStatus: this.params.examineStatus,
                budgetTime: this.params.budgetTime,
                sort:Number(budgetDetailsVosLen) + Number(index),
                budgetDetailsVos:[el]
              })
            }
          })
          //计算money
          editAppend.costBudgetVoList.forEach(item=>{
            item.money = item.budgetDetailsVos.reduce((prev,cur)=>{
              return math.add(prev,cur.money)
            },0)
          })
        }else{
          editAppend.costBudgetVoList = []
        }
      }
      const fileId = this.$refs.eleupload.getFileId();
      this.params.enclosure = fileId[0] || '';
      this.params.money = this.form.money;
      this.params.remarks = this.form.remarks;
      // if (this.flowId && status == 1 && !noNeedCheckFlow) await this.getFlowFindById()
      // if (!this.canSubmit && status == 1) return false
      if (status == 1) this.sumbitFlow();
      else {
        this.postData(status);
      }
      // this.handlePost(status);
    },
    // handlePost(status) {
    postData(status,batchCode) {
      if (this.editeType == 'init') {
        // console.log('this.params',this.params)
        // return
        this.handlePost(this.params,batchCode);
      } else {
        this.formData.examineStatus = 0;
        this.formData.status = status;
        const data = this.formData;// appendCostBudgetVos[len - 1]
        // if(id)data.id = id
        this.handlePost(data,batchCode);
      }
    },
    processPostData() {
      let totalBueget = 0;
      const arr = [],budgetDetailsVos = [],costBudgetVoList=[]
      let childrenSort = 0;
      for (let i = 0; this.datas[i]; i++) {
        const budget = this.datas[i].budget;
        for (let j = 0; budget[j]; j++) {
          const tmpObj = {};
          if (budget[j].useMoney > 0) {
            if(budget[j].useMoney > budget[j].budgetMoney){
              let departName = budget[j].departName == '公司领导' ? '公司固定费用':budget[j].departName
              this.$message.error(`${departName || ''}归口[${budget[j].budgetType || ''}]预算金额应该大于归口已使用金额`);
              return false;
            }
          }
          // if (this.editeType == 'init') {
          //   if (budget[j].budgetMoney == 0) continue;
          //   tmpObj.money = budget[j].budgetMoney;
          // } else {
          //   if (budget[j].appendMoney == 0) continue;
          //   tmpObj.money = budget[j].appendMoney;
          // }
          childrenSort += 1;
          // budgetDetailsId: budgetDetailsVos[i].id,
          if (this.editeType == 'init') tmpObj.budgetId = budget[j].budgetId;
          tmpObj.budgetDetailsId = budget[j].budgetDetailsId || '';
          tmpObj.budgetTypeId = budget[j].budgetTypeId || '';
          tmpObj.departmentId = this.datas[i].departId;
          tmpObj.type = this.params.type;
          tmpObj.projectId = budget[j].relateProjId;
          tmpObj.sort = childrenSort;
          tmpObj.remarks = budget[j].remarks;
          tmpObj.money = budget[j].budgetMoney || 0
          // if (!tmpObj['money']) {
          //   this.$message.error('预算金额不可为0')
          //   return false
          // }
          totalBueget = math.add(totalBueget , (tmpObj.money - 0));
          tmpObj.budgetTypeVo = {};
          const budgetTypeVo = {};
          budgetTypeVo.name = budget[j].budgetType;
          budgetTypeVo.templateId = budget[j].templateId;
          if (budgetTypeVo.templateId == '') {
            this.$message.error('所有费用预算类型均为必填');
            return false;
          }
          // const idx = budget.findIndex((el, k) => k != j && el.budgetType == budgetTypeVo.name);
          // if (idx > -1) {
          //   this.$message.error('相同部门下不可有相同归口');
          //   return false;
          // }
          budgetTypeVo.parentId = 1;
          budgetTypeVo.departmentId = this.datas[i].departId;
          budgetTypeVo.type = this.params.type;
          budgetTypeVo.annually = this.form.annual;
          budgetTypeVo.status = 0;// this.form.status
          budgetTypeVo.departmentName = this.getDepartNameById(this.datas[i].departId) || budget[j].departName,
          budgetTypeVo.companyId = this.form.companyId;
          budgetTypeVo.templateId = budget[j].templateId;
          budgetTypeVo.id = budget[j].budgetTypeId || '';
          budgetTypeVo.status = budget[j].status || 0;
          tmpObj.budgetTypeVo = budgetTypeVo;
          // arr.push(tmpObj);
          if(this.isProjectBudget(this.datas[i].departId)){
            tmpObj.projectId = this.datas[i].departId
            tmpObj.costBudgetId = budget[j].costBudgetId
            budgetTypeVo.type = 3
            budgetTypeVo.projectId = tmpObj.projectId
            costBudgetVoList.push(deepClone(tmpObj))
          }else{
            budgetDetailsVos.push(deepClone(tmpObj));
          }
        }
      }
      // console.log(totalBueget, this.form.money)
      this.form.money = totalBueget;
      // if (totalBueget.toFixed(6) != this.form.money.toFixed(6)) {
      //   this.$message.error('预算总金额和预算详情金额不符');
      //   return false;
      // }
      return {budgetDetailsVos,costBudgetVoList};
    },
    // postData(data) {
    handlePost(data,batchCode) {
      // if(this.isGroupMember){
      //   data.type = 6
      //   data.projectId = this.params.projectId
      // }
      // let action = Api.annualBudget.initBudgetUpdate;
      let action = Api.annualBudget.costBudgetSaveTask; //新的保存接口
      if (this.editeType == 'append') {
        action = Api.annualBudget.costBudgetUpdate;
      }
      //转换金额
      this.transeMoney(data)
      this.$axios.post(action, { data,batchCode}, res => {
        if (res.isSuccess) {
          if (data.enclosure) {
            this.bindFileById(data.id, data.enclosure).then(() => {
              this.processReturnData(data);
            });
          } else {
            this.processReturnData(data);
          }
        } else {
          this.$parent.$parent.$parent.submitLoading = false
          this.$message.error(res.message);
        }
      });
    },
    async processReturnData(data) {
      let money = math.multiply(data.money,10000)
      let userName = localstorageGet('userName')
      if(this.createrId){
        let userInfo = await this.getUserInfo()
        if(userInfo)userName = userInfo.name
      }
      if (data.status == 1) {
        // 提交审核
        const name = `${this.form.annual}公司年度预算￥${money}元-${userName}`;
        this.submitFlowFinal(true, data.id, '', money, name);
      } else {
        // this.$router.push({ path: '/groupBudgetManage/companyBudget/index' });
        const name = `${this.form.annual}公司年度预算￥${money}元-${userName}`;
        this.submitFlowFinal(true, data.id, '', money, name,'draft');
      }
    },
    updateStatusWhenFlowFail(data) { // 流程发起失败后，修改业务状态为草稿
      data.status = 0;
      let action = Api.annualBudget.initBudgetUpdate;
      if (this.editeType == 'append') {
        action = Api.annualBudget.costBudgetUpdate;
      }
      this.$axios.post(action, { data }, res => {
        if (res.isSuccess) {
          this.processReturnData(data);
        } else {
          this.$message.error(res.message);
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
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
    }
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss">
  .budgetType .el-input__inner{
    width: initial !important;
  }
</style>
