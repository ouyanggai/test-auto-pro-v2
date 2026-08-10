<!-- 集团财务无表单流程页面 -->

<template>
  <div class="outer">
    <div class="container">
      <h2>集团各公司年度预算</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row style="margin-bottom:30px">
              <el-col :span="12">
                <el-form-item label="预算年度：" prop="annual">
                  <el-date-picker v-model="form.annual" type="year" value-format="yyyy" @change="annualChange"
                    :disabled="isExamine" style="width: 120px;" :picker-options="pickerOptions">
                  </el-date-picker>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="预算填报人：" prop="fillName">
                  <el-input v-model="form.fillName" :disabled="true"></el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px">
              <el-col :span="16">
                <el-form-item label="预算分析：">
                  <el-input type="textarea" :autosize="{ minRows: 4, maxRows: 9 }" placeholder="请输入内容"
                    v-model="form.remarks" maxlength="5000" show-limit :disabled="isExamine">
                  </el-input>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row>
              <el-col :span="24">
                <eleupload ref="eleupload" :size="20" :multiple="false" :showOnly="true" :attachFile="attachFile"
                  v-if="isExamine"></eleupload>
                <eleupload ref="eleupload" :size="20" :uploadLimit="1" :multiple="false" :attachFile="attachFile" v-else></eleupload>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
        <el-card class="box-card" style="margin-top: 10px;">
          <el-tabs v-model="activeName" type="card">
            <el-tab-pane v-for="(val, index) in companyOption" :name="val.id" :key="val.id">
              <span slot="label">
                <span>{{ val.name }}</span>
              </span>
            </el-tab-pane>
          </el-tabs>

          <div style="padding: 10px 0;display: flex;align-items: center;" v-if="totalMoney">
            <div>
              预算总额：<span class="money">{{ totalMoney }}</span>（万元）
            </div>
            <template v-if="!isGroup">
              <div style="display: flex;align-items: center;margin-left: 15px;" v-if="params.remarks">
                <svg-icon icon-class="sub-remark"></svg-icon> <span class="link-span"
                  @click="showRemarks(params.remarks || '暂无数据')">预算金额分析</span>
              </div>
              <div style="display: flex;align-items: center;margin-left: 15px;" v-if="subAttachFile.length">
                <eleupload :attachFile="subAttachFile" :showOnly="true"></eleupload>
              </div>
            </template>
          </div>
          <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;"
            shadow="never" :class="{ 'nopadding': val['activeNames'] == 0 }">
            <template slot="header">
              <div class="card-title" @click.stop.prevent="() => { }">
                <el-select v-model="val.departName" placeholder="请选择" style="margin-right:20px;" :disabled="true">
                  <el-option v-for="item in []" :key="item.id" :label="item.name" :value="item.id"
                    :disabled="item.hasSelect">
                  </el-option>
                </el-select>
              </div>
            </template>
            <span v-if="isGroup">
              <span v-if="val.topRemark && val.departName !='公司固定费用'" class="link-span top-remark" @click="showRemarks(val.topRemark || '暂无数据')">预算金额分析</span>
              <div style="position: absolute;top:15px;left:295px; display: flex;align-items: center;margin-left: 15px;" v-if="val.subAttachFile && val.subAttachFile.length">
                <eleupload :attachFile="val.subAttachFile" :showOnly="true"></eleupload>
              </div>
            </span>
            <!-- <span v-if="index== 0 && isGroup">
              <span v-if="params.remarks && val.departName !='公司固定费用'" class="link-span top-remark" @click="showRemarks(params.remarks || '暂无数据')">预算金额分析</span>
              <div style="position: absolute;top:15px;left:295px; display: flex;align-items: center;margin-left: 15px;" v-if="subAttachFile.length">
                <eleupload ref="eleupload" :attachFile="subAttachFile" :showOnly="true"></eleupload>
              </div>
            </span> -->
            <span class="subSum">合计：<span class="money inline-block">{{ calcuSum(val) }}</span> （万元）</span>
            <span class='open-tips' @click="openCollapse(val, index)">展开</span>
            <!-- <el-collapse v-model="val['activeNames']"> -->
            <el-collapse v-model="collapseIndex">
              <el-collapse-item :name="index" style="padding:none;">
                <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                  <el-table-column type="index" label="编号" width="65px">
                  </el-table-column>
                  <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType">

                    <template slot-scope="scope">
                      <!-- <el-form-item :prop="'datas.' + index + '.departId'" :rules="subRule.departId"> -->
                      <el-input v-model="scope.row.budgetType" maxlength="20" :disabled="true"></el-input>
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
                  <el-table-column prop="useMoney" label="已使用金额(万)">
                    <template slot-scope="scope">
                      <el-input v-model="scope.row.useMoney" maxlength="20" :disabled="true"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column prop="budgetMoney" label="预算金额(万)" width="160px">

                    <template slot-scope="scope">
                      <el-input-number v-model="scope.row.budgetMoney" :min="0" :precision="6" :step="0.1"
                        :controls="false" :disabled="true"></el-input-number>
                    </template>
                  </el-table-column>
                  <el-table-column prop="remarks" label="备注">
                    <template slot-scope="scope">
                      <el-input v-model="scope.row.remarks" maxlength="50" :disabled="true"></el-input>
                    </template>
                  </el-table-column>
                </el-table>
              </el-collapse-item>
            </el-collapse>
          </el-card>
        </el-card>
        <!-- <div class="footer-bt" v-if="!operaType">
          <div class="footer-inner">
            <el-button type="primary" icon='el-icon-view' @click="$parent.handleCheckFlow()">查看流程</el-button>
            <el-button @click="prevStepHandle" plain>取 消</el-button>
            <el-button type="primary" @click="submit">提 交</el-button>
          </div>
        </div> -->
      </div>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import eleupload from '@/components/EleUpload';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils';
import mixin from './mixins/mixin';
import numFunc from '@/utils/number'; // 重写toFixed
import math from '@/utils/math.js'
import moment from 'moment'
export default {
  name: 'GroupFinance',
  components: {
    eleupload
  },
  mixins: [mixin],
  props: ['operaType', 'id', 'param', 'flowNodeProxyId', 'showType', 'isExamine'],
  inject: ['prevStepHandle', 'sumbitFlow', 'submitFlowFinal'],
  data() {
    return {
      activeName: '',
      companyOption: [],
      originCompanyOption: [],
      form: {
        annual: new Date().getFullYear() + '',
        fillName: localstorageGet('userName'),
        remarks: ''
      },
      mainRule: {
        annual: [{
          required: true,
          trigger: 'change',
          message: '请选择年份'
        }],
        fillName: [{
          required: true,
          trigger: 'blur',
          message: '请输入填报人'
        }]
      },
      query: {
        companyId: '', // localstorageGet('companyId'),
        companyIds: [],
        // annual: '', // new Date().getFullYear(),
        budgetTime: '',
        // type: 1
        stringList: [1, 6]
      },
      datas: [],
      originList: [],
      list: [],
      totalMoney: '',
      attachFile: [],
      subAttachFile: [],
      collapseIndex: [],
      isGroup:false, //标签是否是集团
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
    this.$bus.$off('group_finance_before_handle');
    this.$bus.$on('group_finance_before_handle', val => {
      this.busVal = val;
      // this.submit();
      this.postData();
    });
    if (this.id) {
      this.getFileByBizId(this.id,'attachFile');
      this.getParentCompanyList().then(respnse => {
        this.getCompanyBudget().then(res => {
          if (res.isSuccess) {
            const data = res.data.dataList[0];
            this.form = {
              id: data.id,
              annual: data.budgetTime.substr(0, 4),
              fillName: data.fillName,
              remarks: data.remarks,
              enclosure: data.enclosure
            };
            const groupBudgetList = data.groupBudgetList;

            this.query.budgetTime = `${this.form.annual}-01-01 00:00:00`;
            this.query.companyId = '';
            this.query.companyIds = groupBudgetList.map(item => {
              return item.companyId;
            });
            this.getAllCompanyBudget();
          }
        });
      });
    } else {
      this.getParentCompanyList().then(res => {
        this.query.budgetTime = '';
        this.query.annual = this.form.annual;
        if (this.query.annual) {
          this.query.budgetTime = `${this.form.annual}-01-01 00:00:00`;
        }
        // this.query.companyId = val
        if (!this.query.companyIds) {
          this.query.companyIds = this.originCompanyOption.map(item => {
            return item.id;
          });
        }
        this.getAllCompanyBudget();
      });
    }
  },
  mounted() { },
  computed: {
    isFlow() {
      // return false
      if (this.operaType) {
        return true;
      } else {
        return false;
      }
    }
  },
  watch: {
    activeName(val) {
      this.collapseIndex = []
      if (val) {
        if(val == localstorageGet('topCompanyId'))this.isGroup = true
        else this.isGroup = false
        const find = this.list.find(item => item.companyId == val);
        if (find) {
          if(this.isGroup){
            let groupList = this.list.filter(item=>item.companyId == val && item.type == 6)
            let totalMoney = 0
            if(groupList.length){
              let mainGroupData = deepClone(groupList[0])
              groupList.forEach((item,index)=>{
                let topId = item.id
                let topRemark = item.remarks
                let budgetDetailsVos = item.budgetDetailsVos || []
                totalMoney = math.add(totalMoney, Number(item.money))
                budgetDetailsVos.forEach(el=>{
                  el.topId = topId
                  el.topRemark = topRemark
                  if(index >=1)mainGroupData.budgetDetailsVos.push(el)
                })
              })
              this.params = deepClone(mainGroupData)
              this.params.money = totalMoney
              this.initData('budgetDetailsVos');
            }
          }else{
            this.params = deepClone(find);
            this.initData('budgetDetailsVos');
            this.getFileByBizId(this.params.id);
          }
        } else {
          this.datas = [];
          this.totalMoney = 0;
          this.subAttachFile = '';
        }
      } else {
        this.datas = [];
        this.totalMoney = 0;
        this.subAttachFile = '';
      }
    }
  },
  methods: {
    openCollapse(row, index) {
      let idx = this.collapseIndex.indexOf(index)
      if (idx > -1) {
        this.collapseIndex.splice(idx, 1)
      } else {
        this.collapseIndex.push(index)
      }
    },
    showRemarks(remarks) {
      this.$alert(`<div class='alert-remarks' style="max-height:70vh;overflow:auto;">${remarks}</div>`, '预算金额分析', {
        dangerouslyUseHTMLString: true
      });
    },
    // 根据业务id获取文件
    getFileByBizId(id,key) {
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
              id: item.fileId,
              fileName: item.fileName,
              fileUrl: item.fileUrl
            };
          });
          if(key)this[key] = attachFile
          else{
            this.subAttachFile = attachFile;
          }
        }
      });
    },
    calcuSum(val) {
      if (val.budget && val.budget.length) {
        return val.budget.reduce((prev, cur) => {
          return math.add(prev, cur.budgetMoney, cur.appendMoney);
        }, 0);
      } else {
        return 0;
      }
    },
    getCompanyBudget() {
      const query = {
        id: this.id
      };
      return this.$axios.post(
        Api.annualBudget.findBudgetById,
        {
          data: query
        }
      );
    },
    initData(key, idx) {
      if (JSON.stringify(this.params) == '{}') {
        return false;
      }


      const datas = [];
      let budgetDetailsVos = [];
      let params = {}
      // let obj = {}
      this.totalMoney = this.params.money;
      if (key == 'budgetDetailsVos') {
        this.currentBudget = params = this.params;
        this.initBudget = budgetDetailsVos = params.budgetDetailsVos;
      } else {
        this.currentBudget = params = this.params.appendCostBudgetVos[idx];
        budgetDetailsVos = params.appendBudgetDetailsVos;
      }
      //如果有项目预算
      if (params.costBudgetVoList && params.costBudgetVoList.length) {
        params.costBudgetVoList.forEach(item => {
          let costBudgetId = item.id
          let costBudgetDetailsVos = item?.budgetDetailsVos || []
          costBudgetDetailsVos.forEach(el => {
            el.costBudgetId = costBudgetId
            budgetDetailsVos.push(el)
          })
        })
      }
      // this.bizId = params.id
      // this.getFileByBizId(this.bizId)

      // this.form = {
      //   budgetId: this.params.id,
      //   companyId: this.params.companyId,
      //   companyName: this.params.companyName,
      //   annual: this.params.budgetTime.substr(0, 4),
      //   money: params.money,
      //   remarks: params.remarks,
      // }
      for (let i = 0; budgetDetailsVos[i]; i++) {
        const departId = budgetDetailsVos[i].departmentId;
        const index = datas.findIndex(item => item.departId == departId);
        const budgetTypeVo = budgetDetailsVos[i].budgetTypeVo;
        const isRelateProj = !!budgetDetailsVos[i].projectId;
        const tmp = {
          budgetDetailsId: budgetDetailsVos[i].budgetDetailsId, // || budgetDetailsVos[i].id,
          budgetTypeId: budgetTypeVo.id,
          budgetType: budgetTypeVo.name,
          useMoney:budgetDetailsVos[i].useMoney,
          isRelateProj,
          relateProjName: budgetDetailsVos[i].projectName,
          relateProjId: budgetDetailsVos[i].projectId,
          budgetMoney: budgetDetailsVos[i].money,
          appendMoney: 0,
          remarks: budgetDetailsVos[i].remarks,
          canDelete: false,
          disabled: true
        };
        if (index > -1) {
          datas[index].budget.push(tmp);
        } else {
          const obj = {
            departName: this.getDepartNameById(budgetDetailsVos[i].departmentId,budgetDetailsVos[i].budgetTypeVo.departmentName),
            departId: budgetDetailsVos[i].departmentId,
            activeNames: '0',
            disabled: true,
          };
          if(budgetDetailsVos[i].topRemark)obj.topRemark = budgetDetailsVos[i].topRemark
          if(budgetDetailsVos[i].topId)obj.topId = budgetDetailsVos[i].topId
          obj.budget = [];
          obj.budget.push(tmp);
          datas.push(obj);
        }
      }
      this.datas = datas;
      //如果是集团预算，则需要遍历每一个部门，获取部门提的预算有没有绑定文件
      if(this.isGroup){
      //遍历datas,如果有topId,通过topId去获取绑定的文件
      this.datas.forEach(item=>{
        if(item.topId && item.departName !='公司固定费用'){
          this.$axios.post(
            Api.schedule.getAttachmentList, {
            data: {
              relationId: item.topId
            }
            },res=>{
              if(res.isSuccess){
                const list = res.data;
                if(list && list.length){
                  // item.subAttachFile = []
                  this.$set(item,'subAttachFile',[])
                  list.forEach(el => {
                    item.subAttachFile.push({
                      id: el.fileId,
                      fileName: el.fileName,
                      fileUrl: el.fileUrl
                    })
                });
                }

              }
            })
        }
      })
      }
      const appendCostBudgetVos = this.params.appendCostBudgetVos || [];
      this.tabList = appendCostBudgetVos.map((item, i) => {
        const obj = {
          label: `第${NUMCN[i]}次追加`,
          index: i
        };
        return obj;
      });
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        const v = item || 0;
        // val += v - 0;
        val = math.add(val,Number(v));
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
        if (column.property === 'budgetMoney') {
          const values = data.map(item => item[column.property]);
          // sums[index] = "￥" + ((this.sumNums(values) - 0) + appendTotal - 0);
          sums[index] = '￥' + ((this.sumNums(values) - 0)).toFixed(6);
        } else if (column.property === 'appendMoney') {
          const values = data.map(item => item[column.property]);
          appendTotal = this.sumNums(values);
          sums[index] = '￥' + appendTotal.toFixed(6);
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },
    annualChange() {
      this.activeName = '';
      if (this.form.annual) {
        this.query.budgetTime = `${this.form.annual}-01-01 00:00:00`;
        //获取当前年度集团预算的详情
        this.getAllCompanyBudget()
      }else{
        this.datas = []
      }


    },
    // 获取当前预算模板
    getBudgetTypeOfGroup() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.budgetManage.getBudgetCentralizedOfGroup,
          {},
          res => {
            if (res.isSuccess) {
              const data = res.data || [];
              this.budgetTypeGroup = data
              resolve()
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
          id: sysDepartmentVo.id,
          name: sysDepartmentVo.departmentName == '公司领导' ? '公司固定费用' : sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item => {
        departOptions.push({
          id: item.projectVo.id,
          name: item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject: true
        })
      })
      // this.treeData = treeData;
      // this.departOptions = departOptions
      // this.getAllCompanyBudget();
      return departOptions
    },
    getGroupDetail() {
      let query = {
        companyId: '',//localstorageGet('companyId'),
        companyIds: [],
        annual: this.form.annual, // new Date().getFullYear(),
        budgetTime: `${this.form.annual}-01-01 00:00:00`,
        type: 4
        // stringList:[1,6]
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.annualBudget.budgetList,
          { data: query },
          res => {
            let list = res.data?.dataList || []
            if(list.length){
              resolve(list[0])
            }else{
              resolve({})
            }
          })
      })
    },
    async getAllCompanyBudget() {
      const query = deepClone(this.query);
      if(!this.id){ //新建的时候
        let currentBudget = await this.getGroupDetail()
        if(currentBudget.status == 1){ //审核通过的年份不能再次被选中做预算
          this.$alert(`${this.form.annual}集团年度预算已上报，如需操作请使用【${currentBudget.fillName}】账号进行`, '提示').then(() => {
            this.form.annual = '';
          }).catch(() => {
            this.form.annual = '';
          });
          return
        }
      }
      // let groupBudgetList = []
      // if(currentBudget && currentBudget.groupBudgetList){
      //   groupBudgetList = currentBudget.groupBudgetList
      // }
      // query.companyIds = groupBudgetList.map(item=>item.companyId)
      // // console.log('外面完成',currentBudgetList)
      // // return
      delete query.companyId;
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query,
          pagination: true,
          pages: 1,
          size: 20
        },
        async res => {
          if (res.isSuccess) {
            const data = res.data || {};
            this.originList = data.dataList || [];
            // return
            const list = deepClone(this.originList);
            list.filter(item => {
              return (item.examineStatus!=2 &&(item.status == 1 || item.status == 2));
            });
            this.list = list;
            await this.getBudgetTypeOfGroup()
            const companyOption = [];
            this.originCompanyOption.forEach(item => {
              const id = item.id;
              const index = this.originList.findIndex(el => el.companyId == id);
              if (index > -1) {
                const status = this.originList[index].status;
                const examineStatus = this.originList[index].examineStatus;
                //通过公司id获取归口模板里面的部门和归口信息
                const find = this.budgetTypeGroup.find(item => item.companyVo.id == id);
                let temp = {},childrenList = []
                if (find) {
                  this.centralizedApiVos = find.centralizedApiVos[0];
                  this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
                  childrenList = this.generateDepartOption()
                }
                if (status > 0) {
                  temp = {
                    childrenList,
                    status,
                    examineStatus,
                    name:item.name,
                    id:item.id,
                  }
                  companyOption.push(temp);
                }
              }
            });
            this.companyOption = companyOption;
            if (companyOption.length) this.activeName = companyOption[0].id;
            else {
              this.datas = [];
            }
            //
            // if (list.length < this.companyOption.length) {
            //   this.genWaningMsg(list, this.companyOption)
            // }
          }
        }
      );
    },
    getParentCompanyList() { // 查询公司列表
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.frameworkInfo.getCompanyFrameworkData,
          {
            data: {
              id: localstorageGet('companyId'), // 当前用的公司id
              flag: 2
            }
          },
          res => {
            var arr = [];
            var fn = (list) => {
              list.forEach(item => {
                if (item.type == 1) {
                  arr.push({
                    id: item.id,
                    name: item.name,
                    type: item.type,
                    childrenList: item.childrenList
                  });
                  if (item.childrenList && item.childrenList.length) {
                    fn(item.childrenList);
                  }
                }
              });
            };
            fn(res.data);
            this.originCompanyOption = arr;
            resolve();
          }
        );
      });
    },
    getDepartNameById(id,budgetDepartName) {
      const index = this.companyOption.findIndex(item => item.id == this.activeName);
      let departmentName = ''
      if (index > -1) {
        const department = this.companyOption[index].childrenList;

        let find = department.find(item => item.id == id)
        if(find){
          departmentName = find.name;
        }else{
          departmentName = budgetDepartName
        }
        if (departmentName == '公司领导') departmentName = '公司固定费用';
        return departmentName;
      } else {
        return budgetDepartName;
      }
    },
    checkSubmitAll(list, companyOption) {
      const noBudgetArr = []; let isAll = true;
      //集团的部门
      let group = companyOption.find(item=>item.id == localstorageGet('topCompanyId'))
      let groupChildren = group?.childrenList || []
      let groupDepartment = groupChildren.filter(item=>item.type == 2)
      groupDepartment.forEach(item=>{
        const id = item.id;
        let name = item.name == '公司领导'?'公司固定费用':item.name
        const index = list.findIndex(el => el.projectId == id);
        if (index == -1) {
          isAll = false;
          noBudgetArr.push(`集团${name}:未提交部门预算`);
        } else {
          const status = list[index].status; const examineStatus = list[index].examineStatus;
          if (status == 2) {
            noBudgetArr.push(`集团${name}:未提交部门预算`);
          } else {
            if (examineStatus == 2) { noBudgetArr.push(`集团${name}:预算被驳回`); }
          }
        }
      })
      //集团的子公司
      companyOption.forEach(item => {
        const id = item.id;
        const index = list.findIndex(el => el.companyId == id);
        if (index == -1) {
          isAll = false;
          noBudgetArr.push(`${item.name}:未提交公司预算`);
        } else {
          const status = list[index].status; const examineStatus = list[index].examineStatus;
          if (status == 2) {
            noBudgetArr.push(`${item.name}:未提交公司预算`);
          } else {
            if (examineStatus == 2) { noBudgetArr.push(`${item.name}:预算被驳回`); }
          }
        }
      });
      if (isAll) {
        return { isAll };
      } else {
        const msg = noBudgetArr.join('<br/>');
        // this.$alert(msg, '请注意', { dangerouslyUseHTMLString: true, center: true, confirmButtonText: '关闭' })
        return { isAll, msg };
      }
    },
    // submit() {
    postData(status,batchCode) {
      if (this.isExamine && this.operaType == 'edit') {
        const data = deepClone(this.form);
        const relationId = data.id;
        this.processReturnData(data, relationId);
      } else {
        this.$refs.ruleForm.validate(res => {
          if (res) {
            // 业务提交 TODO
            const checkRes = this.checkSubmitAll(this.list, this.originCompanyOption);
            if (checkRes.isAll) {
              this.postBudget(batchCode);
            } else {
              if(!this.hasTip){
                this.$confirm(checkRes.msg, '请注意', {
                  closeOnClickModal: false,
                  // angerouslyUseHTMLString: true,
                  center: true,
                  // distinguishCancelAndClose: true,
                  confirmButtonText: '继续提交预算',
                  cancelButtonText: '暂不提交',
                  dangerouslyUseHTMLString: true, // 使用HTML片段
                  type: 'warning'

                }).then(() => {
                  this.hasTip = true
                  this.postBudget(batchCode);
                }).catch(() => {
                  this.hasTip = false
                  this.$parent.$parent.$parent.submitLoading = false
                });
              }else{
                this.hasTip = true
                this.postBudget(batchCode);
              }

            }
          }
        });
      }
    },
    postBudget(batchCode) {
      const data = deepClone(this.form);
      data.budgetTime = `${data.annual}-01-01 00:00:00`;
      data.status = '1';
      data.projectId = '';
      data.examineStatus = 0;
      data.type = '4';

      data.groupBudgetList = this.list.map(item => {
        return {
          companyId: item.companyId,
          type:item.type
        };
      });
      data.money = this.list.reduce((prev, item) => {
        return math.add(prev , item.money);
      }, 0);
      const fileId = this.$refs.eleupload.getFileId();
      data.enclosure = fileId[0] || '';
      data.companyId = localstorageGet('topCompanyId');
      // return
      let action = Api.annualBudget.costBudgetSave;
      if (data.id) {
        action = Api.annualBudget.initBudgetUpdate;
      }
      // console.log('data',data)
      // return
      this.$axios.post(action, { data,batchCode }, res => {
        if (res.isSuccess) {
          let relationId = '';
          if (data.id) relationId = data.id;
          else relationId = res.data.id;
          if(!this.form.enclosure){ //没有，现在有，绑定文件
            if(data.enclosure){
              // 绑定文件
              const fileId = data.enclosure;
              // data.id = res.data.id
              this.bindFileById(relationId, fileId).then(r => {
                this.processReturnData(data, relationId);
              });
            }else{
              this.processReturnData(data, relationId);
            }
          }else{ //之前有
            if (data.enclosure && this.form.enclosure != fileId[0]) {
              // 绑定文件
              const fileId = data.enclosure;
              // data.id = res.data.id
              this.bindFileById(relationId, fileId).then(r => {
                this.processReturnData(data, relationId);
              });
            } else {
              this.processReturnData(data, relationId);
            }
          }
        } else {
          this.$parent.$parent.$parent.submitLoading = false
          this.$message.error(res.message);
        }
      });
      // }
    },
    //relationId主业务的id
    processReturnData(data, relationId) {
      this.getInstanceId(relationId).then(subInstance => {
        if (!data.id) {
          const submitObj = {
            id: relationId, // 业务id
            money: data.money,
            name: `${data.annual}集团各公司年度预算-${data.money}万-${data.fillName}`,
            subInstance
          };
          // this.sumbitFlow('submit', submitObj);
          this.submitFlowFinal(true, submitObj.id, '', submitObj.money, submitObj.name);
        } else {
          const obj = {
            status: 'success',
            // val: this.busVal,
            total: data.money,
            id: relationId
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        }
      });
    },
    // 获取流程实例id
    getInstanceId(relationId) {
      const flowInstanceBizRelevanceList = [{
        otherBiz: 'company_annual_budget'
      }];
      const otherBizIdList = this.list.map(item => {
        return item.id;
      });
      flowInstanceBizRelevanceList[0].otherBizIdList = otherBizIdList;
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
            const subInstance = data.map(item => {
              return {
                otherBiz: 'subprocessInstanceId',
                otherBizId: item.id
              };
            });
            resolve(subInstance);
          }
        });
      });
    }
  }
};
</script>

<style lang="scss" scoped src="./style/style.scss"></style>

<style lang="scss" scoped>
::v-deep .el-badge__content.is-fixed.is-dot {
  top: 20px;
  right: -5px;
}

::v-deep .nopadding .el-card__body {
  padding: 0 !important;
}

.subSum {
  position: absolute;
  right: 90px;
  top: 12px;
  color: #6b6b6b;
}
.top-remark{
  position: absolute;
  left: 215px;
  top: 15px;
}
.open-tips {
  position: absolute;
  right: 27px;
  top: 14px;
  color: #409EFF;
  cursor: pointer;
}

.money {
  color: #409EFF;
  font-size: 16px;
}

.inline-block {
  display: inline-block;
  width: 130px;
  text-align: right;
}

.link-span {
  color: #409EFF;
  cursor: pointer;
  margin-left: 5px;
}

::v-deep .attach-ul .attach-li {
  margin-top: 0px !important;
}

::v-deep .el-collapse-item .el-icon-arrow-right:before {
  color: #409EFF;
}
::v-deep .el-table--border{
  border-left: none;
  border-top: none;
}
::v-deep .el-table--border::after{
  display: none;
}
::v-deep .el-select--mini{
  width: initial;
}
</style>
