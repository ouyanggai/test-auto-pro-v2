<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>创建公司年度预算</h2>
      <div class="inner-container">
        <el-card class="box-card" shadow="never">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row style="margin-bottom:30px" :gutter="20">
              <el-col :span="11">
                <el-form-item label="部门名称：" prop="groupDepartId" v-if="isGroupMember">
                  <!-- 集团人员做预算 -->
                  <el-select v-model="form.groupDepartId" placeholder="请选择"  :disbled="disableCompany"  @change="groupDepartChange" style="width: 100%;">
                    <el-option v-for="item in groupDepartOptions" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
                  <el-form-item label="公司名称：" prop="companyId" v-else>
                  <!-- 子公司人员 -->
                  <el-select v-model="form.companyId" placeholder="请选择"  :disbled="disableCompany" style="width: 100%;">
                    <el-option v-for="item in companyOption" :key="item.id" :label="item.name" :value="item.id">
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="预算年度：" prop="annual">
                <!-- <el-select v-model="form.annual" placeholder="请选择">
                    <el-option v-for="item in dateOption" :key="item.dictLabel" :label="item.dictValue"
                      :value="item.dictValue">
                    </el-option>
                    </el-select> -->
                  <!-- <el-date-picker v-model="form.annual" type="year" value-format="yyyy" @change="annualChange" style="width: 100%;"
                  :picker-options="pickerOptions"> -->
                  <el-date-picker v-model="form.annual" type="year" value-format="yyyy" @change="annualChange" style="width: 100%;"></el-date-picker>
                </el-form-item>
              </el-col>
              <el-col :span="7">
                <el-form-item label="预算金额(元)：" prop="money">
                  <el-input-number v-model="form.money" :min="0.00" :precision="2" :step="0.1" :controls="false" :disabled="true"  style="width: 100%;">
                  </el-input-number>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px">
              <el-col :span="24">
                <el-form-item label="预算金额分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" maxlength="5000">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row>
              <el-col :span="24">
                <eleupload ref="eleupload" :size="20" :multiple="false" :uploadLimit="1"></eleupload>
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
                    <el-select v-model="val.departId" placeholder="请选择" style="margin-right:10%;width: 210px;" @change="v=>departChange(index,v)">
                      <el-option v-for="item in departOptions" :key="item.id" :label="item.name" :value="item.id"
                      :disabled="datas.findIndex(it=>it.departId == item.id) > -1">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </div>
              </template>
              <div class="el-icon-close" @click="deleteDepartPlan(index)" v-if="datas.length>1">
              </div>
              <el-collapse v-model="val['activeNames']">
                <el-collapse-item name="1">
                  <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                    <el-table-column type="index" label="编号" width="54px">
                    </el-table-column>
                    <el-table-column prop="templateId" label="费用预算类型（一级）" class-name="budgetType" width="240px">
                      <template slot-scope="scope">
                        <el-select v-model="scope.row.templateId" placeholder="请选择" v-if="treeData.find(item=>item.id == val.departId)"
                          @change="vals=>budgetTypeChange(vals,val.departId,scope.row)" style="width: 200px;"
                          >
                        <!-- <el-select v-model="scope.row.templateId" placeholder="请选择" > -->
                          <el-option
                            v-for="item in treeData.find(item=>item.id == val.departId).children"
                            :key="item.expenseBudgetTypeTmplId"
                            :label="item.name"
                            :value="item.expenseBudgetTypeTmplId"
                            :disabled="val['budget'].findIndex(it=>it.templateId == item.expenseBudgetTypeTmplId) > -1"
                            >
                          </el-option>
                        </el-select>
                        <el-select v-model="scope.row.templateId" v-else>
                        </el-select>
                      </template>
                    </el-table-column>
                    <!-- <el-table-column prop="relateProjId" label="是否关联项目" width="340px">
                      <template slot-scope="scope">
                        <div>
                          <el-radio v-model="scope.row.isRelateProj" :label="true"
                            @change="v => radioChange(v, index, scope.$index)">是
                          </el-radio>
                          <el-radio v-model="scope.row.isRelateProj" :label="false"
                            @change="v => radioChange(v, index, scope.$index)">
                            否
                          </el-radio>
                          <el-select v-model="scope.row.relateProjId" placeholder="请选择" style="margin-right:10px;"
                            :disabled="!scope.row.isRelateProj">
                            <el-option v-for="item in projectOptions" :key="item.id" :label="item.name" :value="item.id">
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
                    <el-table-column prop="budgetMoney" label="预算金额(元)" width="160px">
                      <template slot-scope="scope">
                        <el-input-number v-model="scope.row.budgetMoney" :precision="2" :step="0.1"
                          :controls="false" @blur="calculateTotalBudget" v-focusSelect :min="scope.row.useMoney">
                        <!-- <el-input-number v-model="scope.row.budgetMoney" :precision="6" :step="0.1"
                          :controls="false" @blur="calculateTotalBudget" v-focusSelect > -->
                        </el-input-number>
                      </template>
                    </el-table-column>
                    <el-table-column prop="remarks" label="备注" >
                      <template slot-scope="scope">
                        <el-input type="textarea" autosize v-model="scope.row.remarks" maxlength="50"></el-input>
                      </template>
                    </el-table-column>
                    <el-table-column fixed="right" label="操作" width="80px">
                      <template slot-scope="scope">
                        <el-button @click="planDelete(index, scope.$index,scope.row,val)" type="text" size="small"><i
                            class="el-icon-delete-solid delete-icon" v-if="!scope.row.useMoney"></i></el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <i class="el-icon-circle-plus add-plan-icon" @click="addPlan(index)" ></i>
                </el-collapse-item>
              </el-collapse>
            </el-card>
          </el-form>

        </el-card>
      </div>
      <div class="footer-bt">
        <div class="footer-inner">
          <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
          <el-button @click="prevStepHandle" plain>取 消</el-button>
          <el-button type="primary" plain @click="submit(0)">保 存</el-button>
          <el-button type="primary" @click="submit(1)">提 交</el-button>
        </div>
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
import {
  localstorageGet
} from '@/utils/auth';
import Api from '@/api';
import mixin from './mixins/mixin';
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog';
import BranchChoose from '@/views/BudgetManage/CompanyBudget/components/branchChoose';
import BranchPralleChoose from '@/views/BudgetManage/CompanyBudget/components/branchPralleChoose';
import numFunc from '@/utils/number'; // 重写toFixed
import { deepClone } from '@/utils';
import math from '@/utils/math.js'
import moment from 'moment'
Number.prototype.toFixed = numFunc;
export default {
  name: 'AddAnnualBudgetComponent',
  mixins: [mixin],
  components: { eleupload, PersonSelectDialog, BranchChoose, BranchPralleChoose },
  props: ['annual','actionType'],
  inject: ['prevStepHandle', 'sumbitFlow', 'submitFlowFinal'],
  data() {
    return {
      form: {
        companyId: localstorageGet('companyId'),
        annual: this.annual, // new Date().getFullYear() + '',
        money: '',
        remarks: '',
        status: '', // 0草稿1提交
        examineStatus: '',
        groupDepartId:'',
        type: '1'
      },
      disableCompany: false,
      dateOption: [],
      datas: [
        {
          planId: 1,
          departName: '',
          departId: '',
          activeNames: '1',
          budget: [
            {
              budgetType: '',
              isRelateProj: false,
              relateProjName: '',
              relateProjId: '',
              budgetMoney: 0,
              status: 0,
              templateId: '',
              remarks: '',
              companyId: localstorageGet('companyId')
            }
          ]
        }
      ],
      companyOption: [{
        id: localstorageGet('companyId'),
        name: localstorageGet('companyName')
      }],
      departOptions: [],
      projectOptions: [],
      selectFlowType: 'company_annual_budget',

      nodeChooseVisible: false,
      nextNodeName: '',
      nextNodeProxyId: '',
      checkboxPersonGroup: [],

      flowId: '',
      pickerOptions:{
        disabledDate(v){
          let year = moment(v).format('YYYY')
          let current = moment().format('YYYY')
          return current < year
        }
      }
    };
  },
  created() {
    // this.getDepartmentList(this.form.companyId);
    // this.$bus.$off('company_annual_budget_before_handle')
    // this.$bus.$on('company_annual_budget_before_handle', val => {
    //   this.busVal = val
    //   this.submit()
    // });

    this.getBudgetTypeOfGroup();

    // this.getParentCompanyList()
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
    // await this.getFlowType()
    // this.companyOption = [{
    //   name:localstorageGet('companyId'),
    //   id:localstorageGet('companyName')
    // }]
  },
  watch: {
    // companyOption(val) {
    //   if (val.length) {
    //     const mainCompany = val.find(item => {
    //       return item.flag == 'mainDutyCompany';
    //     });
    //     if (mainCompany) {
    //       this.form.companyId = mainCompany.id;
    //       this.disableCompany = true;
    //       // this.companyChange(this.form.companyId);
    //     }
    //   }
    // }
  },
  mounted() {
  },
  methods: {
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
    budgetChange(val, departId) {
      const budgetTypeList = this.treeData.find(item => item.id == departId);
      if (budgetTypeList && budgetTypeList.children && budgetTypeList.children.length) {
        const list = budgetTypeList.children;
        const find = list.find(el => el.id == val);
        if (find)find.disabled = true;
      }
      // console.log('budgetList',budgetList)
      // console.log(val,departId)
    },
    getCompanyListOfOnDuty() {
      return this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {});
    },
    annualChange() {
      if(this.isGroupMember){
        if (this.form.groupDepartId && this.form.annual) {
          // console.log('this.form.groupDepartId',this.form.groupDepartId)
          this.getBuegetInfo(this.form.companyId, this.form.annual,this.form.groupDepartId);
        }
      }else{
        if (this.form.companyId && this.form.annual) {
          this.getBuegetInfo(this.form.companyId, this.form.annual);
        }
      }

    },
    async getBuegetInfo(companyId, annual,projectId) {
      const query = {
        companyId, // localstorageGet('companyId'),
        budgetTime: `${annual}-01-01 00:00:00`,
        type: 1
      };
      if(this.isGroupMember){
        query.stringList = [4,6]
        if(projectId)query.projectId = projectId
        delete query.type
      }
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
                //该集团部门没有预算,可能是子部门
                this.toEditPage(list);
                // console.log('没有预算')
                return
              }
            }
            // if (list.status == 0) {
            //   this.$confirm(`该公司在${this.form.annual}年度的预算有已保存的草稿，是否要载入数据`, '提示', {
            //     closeOnClickModal: false,
            //     confirmButtonText: '确定',
            //     cancelButtonText: '重新选择',
            //     type: 'warning'
            //   }).then(() => {
            //     this.toEditPage(list);
            //   }).catch(() => {
            //     // if (!this.disableCompany) {
            //     //   this.form.companyId = '';
            //     // }
            //     this.datas = []
            //     this.addDepartPlan()
            //     this.form.annual = '';
            //   });
            // } else
             if (list.status == 0 || (list.status == 1 && list.examineStatus == 2)) { // 该预算被驳回
              // this.toEditPage(list);
              this.$confirm(`该公司在${this.form.annual}年度的预算为草稿、驳回或撤销状态，请到待办页面选择编辑或者重新发起审批`, '提示', {
                closeOnClickModal: false,
                confirmButtonText: '确定',
                cancelButtonText: '重新选择',
                type: 'warning'
              }).then(() => {
                this.$parent.$parent.$parent.handleClose(true)
                this.$parent.$parent.$parent.$parent.activeName='dueout'
                // this.$router.push({path:'/groupApproveManage/index?tab=dueout'})
              }).catch(() => {
                this.datas = []
                this.addDepartPlan()
                this.form.annual = '';
              });
            } else if (list.status == 2) {
              // 系统生成的预算
              this.toEditPage(list);
            } else {
              this.$alert(`该公司在${this.form.annual}年度已做预算，请重新选择年份`, '提示').then(() => {
                // if (!this.disableCompany) {
                //   this.form.companyId = '';
                // }

                this.form.annual = '';
              }).catch(() => {
                if (!this.disableCompany) {
                  this.form.companyId = '';
                }

                this.form.annual = '';
              });
            }
          }
        }
        // console.log('res', res)
      });
    },
    toEditPage(list) { // 编辑，也就是再次发起审批
      this.$emit('changeComponent', { list, type: 'edit_company_annual_budget' });
    },
    //审核驳回，重新搜索流程数据，调用再次发起
    getInstanceId(otherBizId) {
      const flowInstanceBizRelevanceList = [{
        otherBiz: 'company_annual_budget',
        otherBizId:otherBizId
      }];
      // const otherBizIdList = this.list.map(item => {
      //   return item.id;
      // });
      // flowInstanceBizRelevanceList[0].otherBizIdList = otherBizIdList;
      const data = {
        useScope: 'invest',
        initiator: 'all',
        // auditWayList: this.sFlowTypeList,
        flowInstanceBizRelevanceList
      };
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.schedule.getFlowInstanceList, { data }).then(res => {
          if (res.isSuccess) {
            const data = res.data || [];
            console.log('data',data)
            // const subInstance = data.map(item => {
            //   return {
            //     otherBiz: 'subprocessInstanceId',
            //     otherBizId: item.id
            //   };
            // });
            // resolve(subInstance);
          }
        });
      });
    },
    radioChange(val, i, j) {
      if (val) { // 关联项目
        const companyId = this.form.companyId;
        if (!companyId) {
          this.datas[i].budget[j].isRelateProj = false;
          return this.$message.error('请先选择公司，再关联项目');
        }
        this.getProjectVosByCompanyId(companyId);
      } else { // 不关联
        this.datas[i].budget[j].relateProjId = '';
      }
    },
    // getGroupBudget(){
    //   const query = {
    //     companyId: localstorageGet('companyId'),
    //     budgetTime: `${this.form.annual}-01-01 00:00:00`,
    //     stringList: [4,6]
    //   };
    //   return new Promise((resolve,reject)=>{
    //     this.$axios.post(
    //     Api.annualBudget.budgetList,
    //     {
    //       data: query
    //     },
    //     res=>{
    //       resolve(res)
    //     }
    //     )
    //   })
    // },
    // async departChange(idx,val) {
    //   if(val && this.isGroupMember){
    //     let find = this.departOptions.find(item=>item.name == '公司固定费用')
    //     if(find){
    //       let res = await this.getGroupBudget()
    //       console.log('res',res)
    //       if(res.isSuccess){
    //         console.log('11',2)
    //         let list = res.data?.dataList || []
    //         console.log('11',val)
    //         let findBudget = list.find(item=>item.projectId == val)
    //         console.log('findBudget',findBudget)
    //         if(findBudget){
    //           console.log(1)
    //           this.datas[idx].departName= findBudget.departmentName
    //           console.log(2)
    //           this.datas[idx].departId= findBudget.projectId
    //           console.log(3)
    //           // const obj = {
    //           //   planId: this.datas.length + 1,
    //           //   departName: findBudget.departmentName,
    //           //   companyId: this.form.companyId,
    //           //   departId:findBudget.projectId,
    //           //   activeNames: '1',
    //           //   disabled: true,
    //           //   budget:[]
    //           // };
    //           this.datas[idx].budget = []
    //           console.log(4)
    //           let budgetDetailsVos = findBudget.budgetDetailsVos
    //           budgetDetailsVos.forEach(el=>{
    //             const tmp = {
    //               // id: appendBudgetDetailsVos[j].id,
    //               budgetId: el.budgetId,
    //               budgetDetailsId: el.id,
    //               budgetTypeId: el.budgetTypeId,
    //               budgetType: el.budgetTypeVo.name,
    //               templateId: el.budgetTypeVo.templateId,
    //               relateProjId: el.departmentId,
    //               budgetMoney: el.money || 0, // + appendMoneys,
    //               appendMoney: 0,
    //               isOrigin: true,
    //               status: el.budgetTypeVo.status,
    //               useMoney: el.useMoney || 0,
    //               isOrigin: false,
    //               remarks: el.remarks
    //               // canDelete: false,
    //               // disabled: true
    //             };
    //             this.datas[idx].budget.push(tmp)
    //           })
    //           // this.$set(this.datas,this.datas.length-1,obj)
    //           //this.datas.splice(this.datas.length-1,1,obj)
    //           console.log('this.datas',this.datas)
    //         }
    //       }
    //     }
    //   }
    //   this.departOptions.forEach(item => {
    //     item.hasSelect = false;
    //   });
    //   this.datas.forEach(item => {
    //     // this.departOptions[index].hasSelect = false
    //     const departId = item.departId;
    //     const index = this.departOptions.findIndex(it => it.id == departId);
    //     if (index > -1) {
    //       this.departOptions[index].hasSelect = true;
    //     }
    //   });

    //   // 清除归口
    //   if (idx !== undefined) {
    //     this.datas[idx].budget = [];
    //     this.addPlan(idx);
    //   }
    // },
    addPlan(index) {
      const plan = {
        budgetType: '',
        isRelateProj: false,
        relateProjName: '',
        relateProjId: '',
        budgetMoney: 0,
        templateId: ''
      };
      this.datas[index].budget.push(plan);
    },

    addDepartPlan() {
      const index = this.datas.length;
      const departPlan = {
        planId: index,
        departName: '',
        departId: '',
        activeNames: '1',
        budget: [
          {
            budgetType: '',
            isRelateProj: false,
            relateProjName: '',
            relateProjId: '',
            budgetMoney: 0,
            status: 0,
            remarks: ''
          }
        ]
      };
      this.datas.push(departPlan);
      this.$nextTick(() => {
        var flowContent = document.querySelector('.outer .container')
        flowContent.style.scrollBehavior = 'smooth'
        // let chidlHeight = flowContent.clientHeight
        flowContent.scrollTop = 10000
      })
    },

    /**
     *
     * @param {} status 提交状态0草稿1提交
     * @param {*} noNeedCheckFlow
     * 获取流程，即调用getFlowFindById方法的条件
     * 1非草稿提交预算 2有对应的流程id 3noNeedCheckFlow 参数为false*
     */
    async submit(status, noNeedCheckFlow) {
      let err = false;
      this.$refs.ruleForm.validate(res => {
        if (!res) {
          this.$message.error('有必填项未填')
          this.$parent.$parent.$parent.submitLoading = false
          err = true;
        }
      });
      if (err) return false;
      this.$refs.subForm.validate(res => {
        if (!res) {
          this.$message.error('有必填项未填')
          this.$parent.$parent.$parent.submitLoading = false
          err = true;
        }
      });
      if (err) return false;
      // console.log(this.datas)
      // console.log(3)
      // return
      if (!this.datas.length) {
        this.$parent.$parent.$parent.submitLoading = false
        return this.$message.error('预算不可为空');
      }
      this.form.status = status;
      this.form.processId = '';
      this.form.projectId = '';
      this.form.examineStatus = 0;
      const budgetDetailsVos = [],costBudgetVoList = []
      let totalBueget = 0;
      let childrenSort = 0;
      for (let i = 0; this.datas[i]; i++) {
        const budget = this.datas[i].budget;
        for (let j = 0; budget[j]; j++) {
          childrenSort += 1;
          const tmpObj = {};
          tmpObj.departmentId = this.datas[i].departId;
          tmpObj.type = this.form.type;
          tmpObj.projectId = budget[j].relateProjId;
          tmpObj.money = budget[j].budgetMoney;
          tmpObj.sort = childrenSort;
          // if (!tmpObj.money) {
          //   this.$message.error(`${budget[j].departName || ''}归口[${budget[j].budgetType || ''}]预算金额不可为0`);
          //   this.$parent.$parent.$parent.submitLoading = false
          //   return false;
          // }
          totalBueget = math.add(totalBueget , (tmpObj.money - 0));
          tmpObj.budgetTypeVo = {};
          const budgetTypeVo = {};
          budgetTypeVo.templateId = budget[j].templateId;
          if (budgetTypeVo.templateId == '') {
            this.$message.error('所有费用预算类型均为必填');
            this.$parent.$parent.$parent.submitLoading = false
            return false;
          }
          // const idx = budget.findIndex((el, k) => k != j && el.budgetType == budgetTypeVo.name);
          // if (idx > -1) {
          //   this.$message.error('相同部门下不可有相同归口');
          //   return false;
          // }
          budgetTypeVo.parentId = 1;
          budgetTypeVo.departmentId = this.datas[i].departId;
          budgetTypeVo.type = this.form.type;
          budgetTypeVo.annually = this.form.annual;
          budgetTypeVo.status = 0;// this.form.status
          budgetTypeVo.departmentName = this.getDepartNameById(this.datas[i].departId),
          budgetTypeVo.companyId = this.form.companyId;
          budgetTypeVo.name = budget[j].budgetType;
          tmpObj.budgetTypeVo = budgetTypeVo;
          tmpObj.templateId = budget[j].templateId;
          tmpObj.status = 0;
          tmpObj.remarks = budget[j].remarks;

          //companyBudgetVos 这个是项目的预算
          //判断这个部门id代表的是部门还是项目，如果是项目，需要给projectid赋值，并且push进companyBudgetVos字段
          if(this.isProjectBudget(this.datas[i].departId)){
            tmpObj.projectId = this.datas[i].departId
            budgetTypeVo.type = 3
            budgetTypeVo.projectId = tmpObj.projectId
            costBudgetVoList.push(tmpObj)
          }else{
            budgetDetailsVos.push(tmpObj);
          }
        }
      }
      this.form.budgetDetailsVos = budgetDetailsVos;
      this.form.costBudgetVoList = []
      this.form.budgetTime = `${this.form.annual}-01-01 00:00:00`;
      let budgetDetailsVosLen = budgetDetailsVos.length || 0
      if(costBudgetVoList && costBudgetVoList.length){ //如果有项目预算
        costBudgetVoList.forEach((el,index)=>{
          let projectId = el.projectId
          el.type = 3
          let find = this.form.costBudgetVoList.find(it=>it.projectId == projectId)
          if(find){
            find.budgetDetailsVos.push(el)
          }else{
            this.form.costBudgetVoList.push({
              projectId:el.projectId,
              type:'3',
              companyId: localstorageGet('companyId'),
              money:0, //遍历汇总
              status: this.form.status, // 0草稿1提交
              examineStatus: this.form.examineStatus,
              budgetTime: this.form.budgetTime,
              sort:Number(budgetDetailsVosLen) + Number(index),
              budgetDetailsVos:[el]
            })
          }
        })
        //计算money
        this.form.costBudgetVoList.forEach(item=>{
          item.money = item.budgetDetailsVos.reduce((prev,cur)=>{
            return math.add(prev,cur.money)
          },0)
        })
      }else{
        this.form.costBudgetVoList = []
      }
      // this.form.costBudgetVoList = costBudgetVoList;
      const fileId = this.$refs.eleupload.getFileId();
      this.form.enclosure = fileId[0] || '';
      this.form.money = totalBueget;
      // console.log('this.form',this.form)
      // return
      // if (totalBueget.toFixed(6) != this.form.money.toFixed(6)) {
      //   return this.$message.error('预算总金额和预算详情金额不符');
      // }
      // if (this.flowId && status == 1 && !noNeedCheckFlow) await this.getFlowFindById()
      if (status == 1) this.sumbitFlow();
      else {
        this.postData();
      }
      // if (!this.canSubmit && status == 1) return false
      // this.postData(this.form);
    },
    postData(status,batchCode) {
      var data = deepClone(this.form);
      //所有金额转成万元
      this.transeMoney(data)
      if(this.isGroupMember){
        data.type = 6
        data.projectId = this.form.groupDepartId
      }
      // if(id)data.id = id
      // this.$axios.post(Api.annualBudget.costBudgetSave, { data }, res => {
      this.$axios.post(Api.annualBudget.costBudgetSaveTask, { data,batchCode }, res => {
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
          this.$parent.$parent.$parent.submitLoading = false
        }
      }
      );
    },
    processReturnData(data, res) {
      let money = math.multiply(data.money,10000)
      if (data.status == 1) {
        // 提交审核
        // name:
        const name = `${data.annual}公司年度预算￥${money}元-${localstorageGet('userName')}`;
        this.submitFlowFinal(true, res.data.id, '', money, name);
      } else {
        // this.$router.push({ path: '/groupBudgetManage/companyBudget/index' });
        const name = `${data.annual}公司年度预算￥${money}元-${localstorageGet('userName')}`;
        this.submitFlowFinal(true, res.data.id, '', money, name,'draft');
      }
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
    }
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss">
  .budgetType .el-input__inner{
    width: initial !important;
  }
  .box-card .el-select{
    width: 100%;
  }
</style>
