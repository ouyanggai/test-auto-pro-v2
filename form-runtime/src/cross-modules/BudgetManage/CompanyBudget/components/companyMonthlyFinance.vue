<!-- 集团财务无表单流程页面 -->

<template>
  <div class="outer">
    <div class="container">
      <h2>公司月度预算</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <el-form ref="ruleForm" :model="form" label-width="120px" :rules='mainRule'>
            <el-row style="margin-bottom:30px">
              <el-col :span="12">
                <el-form-item label="公司名称：" prop="companyId">
                  <el-input v-model="form.companyName" :disabled="true">
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="月份：" prop="budgetTime">
                  <!-- <template v-if="isExamine || operaType == 'check'">
                    {{ form.budgetTime.substr(0,7) }}
                  </template> -->
                    <el-date-picker :disabled="isExamine || operaType == 'check'" v-model="form.budgetTime" type="month" placeholder="选择月" value-format="yyyy-MM-01 00:00:00 " @change="monthChange" >
                    </el-date-picker>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row style="margin-bottom:30px">
              <el-col :span="12">
                <el-form-item label="预算金额(元)：" >
                  <el-input v-model="totalMoney" :disabled="true">
                  </el-input>
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
        <el-card class="box-card" style="margin-top: 10px;" >
          <div v-if="budgetList.length">
            <el-card v-for="(val, index) in budgetList" :key="index" style="margin-bottom:20px;position:relative;"
            shadow="never" :class="{ 'nopadding': val['activeNames'] == 0 }" >
            <template slot="header">
              <div class="card-title" @click.stop.prevent="() => { }">
                <span class="title">{{ val.name }}</span>
                <span style="margin-left: 5px;color: #409EFF;" v-if="val.status == 0">({{ val.statusName }})</span>
                <span style="margin-left: 5px;color: #409EFF;" v-if="val.status == 1">({{ val.statusName }})</span>
                <span style="margin-left: 5px;color: #F56C6C;" v-if="val.status == -1">(未提交)</span>
              </div>
            </template>
            <el-collapse v-model="collapseIndex">
              <el-collapse-item style="padding:none;" :name="index">
                <el-table :data="val['budget']" :show-summary="true" :summary-method="summary" border>
                  <el-table-column type="index" label="编号" width="65px">
                  </el-table-column>
                  <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType">
                    <template slot-scope="scope">
                      {{ scope.row.budgetTypeVo.name }}
                    </template>
                  </el-table-column>
                  <!-- <el-table-column prop="useMoney" label="已使用金额(元)">
                    <template slot-scope="scope">
                      ￥{{ scope.row.useMoney || 0 }}
                    </template>
                  </el-table-column> -->
                  <el-table-column prop="money" label="预算金额(元)" width="160px">
                    <template slot-scope="scope">
                      ￥{{ scope.row.money }}
                    </template>
                  </el-table-column>
                  <el-table-column prop="remarks" label="备注">
                    <template slot-scope="scope">
                      {{ scope.row.remarks }}
                    </template>
                  </el-table-column>
                </el-table>
              </el-collapse-item>
            </el-collapse>
          </el-card>
          </div>
          <el-empty description="暂无数据" v-else></el-empty>
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
import math from '@/utils/math.js'
import moment from 'moment'
export default {
  name: 'CompanyMonthlyFinance',
  components: {
    eleupload
  },
  mixins: [mixin],
  props: ['operaType', 'id', 'param', 'flowNodeProxyId', 'showType', 'isExamine'],
  inject: ['prevStepHandle', 'sumbitFlow', 'submitFlowFinal'],
  data() {
    return {
      companyOption: [],
      originCompanyOption: [],
      companyName:localstorageGet('companyName'),
      form: {
        remarks: '',
        budgetTime:'',
        enclosure:'',
        companyId:localstorageGet('companyId'),
        companyName:localstorageGet('companyName'),
        money:''
      },
      mainRule: {
        budgetTime: [{
          required: true,
          trigger: 'blur',
          message: '请选择时间'
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
      },
      departmentList:[],
      budgetList:[],
    };
  },
  created() {
    if (this.id) {
      this.getFileByBizId(this.id,'attachFile');
      this.getMainBudget().then(async res=>{
        if (res.isSuccess) {
          const data = res.data.dataList[0];
          this.form = {
            id: data.id,
            budgetTime: `${data.budgetTime.substr(0, 7)}-01 00:00:00`,
            remarks: data.remarks,
            enclosure: data.enclosure,
            companyId:data.companyId,
            companyName:data.companyName
          }
          //获取当前月度各部门预算
          await this.getDepartmentList()
          this.getCompanyBudget()
        }
      })
    } else {
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
    },
    totalMoney(){
      var total = 0
      if(this.budgetList.length){
        this.budgetList.forEach(item=>{
          let budget = item?.budget || []
          budget.forEach(el=>{
            total = math.add(el.money,total)
          })
        })
        return total
      }else{
        return 0
      }
    }
  },
  methods: {
    getCompanyMonthBudget() {
      return new Promise((resolve,reject)=>{
        const query = {
          budgetTime: this.form.budgetTime,
          companyId: this.form.companyId,
          type: 7 // 公司的月度预算汇总
        };
        return this.$axios.post(
          Api.annualBudget.budgetList,
          {
            data: query
          },
          res=>{
            resolve(res.data?.dataList || [])
          }
        );
      })
    },
    getDepartmentList() {
      return new Promise((resolve, reject) => {
        let data = {
          data:{
            annually:this.form.budgetTime.substr(0,4),//new Date().getFullYear(),
            companyId:this.form.companyId
          }
        }
        this.$axios.post(Api.budgetManage.getBudgetList,data,res=>{
          let list = res?.data?.dataList || []
          let departmentList = []

          list.forEach(item=>{
            let departmentId = item.departmentId
            if(departmentList.findIndex(el=>el.id == departmentId) == -1){
              departmentList.push({
                id:departmentId,
                name:item.departmentName == '公司领导' ? '公司固定费用' : item.departmentName,
                type:item.type,
              })
            }
          })
          this.departmentList = departmentList
          resolve()
        })
      })
    },
    transformName(type){
      return {
        1:'(公司归口)',
        2:'(月度归口)',
        3:'(项目归口)',
      }[type]
    },
    async monthChange(){
      //判断当月是否又预算，如果有，就不再显示，给出提示
      let list = await this.getCompanyMonthBudget()
      if(!list.length){
        await this.getDepartmentList()
        this.getCompanyBudget()
      }else{
        //提示
        let data = list[0]
        if(data.status == 1){
          this.$alert(`该公司在${this.form.budgetTime.substr(0,7)}已存在预算，状态为【审核中】，不能重新发起。`, '提示').then(()=>{
            this.budgetList = []
            this.form.budgetTime = ''
          })
        }
        if(data.status == 0){
          this.$alert(`该公司在${this.form.budgetTime.substr(0,7)}预算已存在草稿，请到待办页面进行编辑。`, '提示').then(()=>{
            this.budgetList = []
            this.form.budgetTime = ''
          })
        }
      }
    },
    getMainBudget() {
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
    getCompanyBudget() {
      let query = {
        companyId: this.form.companyId,
        projectId: '',
        budgetTime: this.form.budgetTime,
        stringList:[2,5]
      }
      this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query,
          pagination:true,
          grouping:true,
          detailed:true,
        },
        res => {
          if (res.isSuccess) {
            const data = res.data?.dataList || [];
            this.createCompanyMonthData(data)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    createCompanyMonthData(data){
      let budgetList = deepClone(this.departmentList),collapseIndex = []
      budgetList.forEach((item,index)=>{
        let id = item.id
        item.status = -1 //没有提交的部门
        collapseIndex.push(index)
        data.forEach(it=>{
          let budgetDetailMap = it.budgetDetailMap
          if(budgetDetailMap[id]){
            item.budget = budgetDetailMap[id]
            item.status = it.status,
            item.otherBizId = it.id,
            item.examineStatus = it.examineStatus
            item.statusName = this.examineStatusSn(it.status,it.examineStatus)
            item.budget.forEach(el=>{
              el.useMoney = math.multiply(el.useMoney,10000)
              el.money = math.multiply(el.money,10000)
            })
          }
        })
      })
      this.budgetList = budgetList
      this.collapseIndex = collapseIndex
    },
    examineStatusSn(status, examineStatus) {
      if (status == 0) {
        return '草稿';
      } else if(status == 1) {
        const CN = { 0: '审核中', 1: '审核通过', 2: '审核驳回' };
        return CN[examineStatus];
      }else if(status == 2){
        return '系统生成'
      }
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
        if (column.property === 'money') {
          const values = data.map(item => item[column.property]);
          // sums[index] = "￥" + ((this.sumNums(values) - 0) + appendTotal - 0);
          sums[index] = '￥' + Number(((this.sumNums(values) - 0)).toFixed(6));
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
    postData(status,batchCode) {
      // if (this.isExamine && this.operaType == 'edit') {
      //   const data = deepClone(this.form);
      //   const relationId = data.id;
      //   this.processReturnData(data, relationId);
      // } else {
      // return new Promise((resolve,reject)=>{
        this.$refs.ruleForm.validate(res=>{
          if(res){
            this.postBudget(status,batchCode);
          }else{
            resolve(false)
          }
        })
      // })
    },
    submit(){
      return new Promise((resolve,reject)=>{
        this.$refs.ruleForm.validate(res=>{
          if(res){
            let submitNum = 0
            this.budgetList.forEach(el=>{
              submitNum += el.budget?.length || 0
            })
            if(submitNum <= 0){
              this.$message.error('需要至少提交一个部门的月度预算')
              return
            }
            let data = {
              companyId:this.form.companyId,
              budgetTime: this.form.budgetTime,//`${data.annual}-01-01 00:00:00`;
              money:this.totalMoney,
              status :'1',
              projectId : '',
              examineStatus : 0,
              type :'7',
              remarks: this.form.remarks,
            }
            if(this.form.id)data.id = this.form.id
            const fileId = this.$refs.eleupload.getFileId();
            data.enclosure = fileId[0] || '';

            let action = Api.annualBudget.costBudgetSave;
            if (data.id) {
              action = Api.annualBudget.initBudgetUpdate;
            }
            this.$axios.post(action, { data }, res => {
              if (res.isSuccess) {
                let relationId = '';
                if (data.id) relationId = data.id;
                else relationId = res.data.id;
                const obj = {
                  status: 'success',
                  // val: this.busVal,
                  total: this.totalMoney,
                  id: relationId
                };
                if(data.enclosure){
                  // 绑定文件
                  const fileId = data.enclosure;
                  // data.id = res.data.id
                  this.bindFileById(relationId, fileId).then(r => {
                    // resolve(obj)
                    this.processReturnDataWithoutFlow(data,relationId,resolve)
                  });
                }else{
                  // this.processReturnData(data, relationId);
                  this.processReturnDataWithoutFlow(data,relationId,resolve)
                }
              } else {
                this.$message.error(res.message);
              }
            }).catch((e)=>{

            })

          }else{
            resolve(false)
          }
        })
      })
    },
    postBudget(status,batchCode) {
      let submitNum = 0
      this.budgetList.forEach(el=>{
        submitNum += el.budget?.length || 0
      })
      if(submitNum <= 0){
        this.$message.error('需要至少提交一个部门的月度预算')
        return
      }
      let data = {
        companyId:this.form.companyId,
        budgetTime: this.form.budgetTime,//`${data.annual}-01-01 00:00:00`;
        money:this.totalMoney,
        status : status == 'draft' ? '0': '1',
        projectId : '',
        examineStatus : 0,
        type :'7',
        remarks: this.form.remarks,
      }
      if(this.form.id)data.id = this.form.id
      const fileId = this.$refs.eleupload.getFileId();
      data.enclosure = fileId[0] || '';

      let action = Api.annualBudget.costBudgetSave;
      if (data.id) {
        action = Api.annualBudget.initBudgetUpdate;
      }
      this.$axios.post(action, { data,batchCode }, res => {
        if (res.isSuccess) {
          let relationId = '';
          if (data.id) relationId = data.id;
          else relationId = res.data.id;
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
        } else {
          this.$message.error(res.message);
        }
      }).catch((e)=>{

      })
    },
    //不提交流程
    processReturnDataWithoutFlow(data,relationId,resolve){
      this.getInstanceId(relationId).then(subInstance => {
        let money = math.multiply(data.money,1)
        if (!data.id) {
          let month = this.form.budgetTime.substr(0,7)
          const submitObj = {
            id: relationId, // 业务id
            money,//: data.money,
            name: `${month}公司月度预算/${this.companyName}/${money}元`,
            subInstance
          };
          resolve(submitObj)
          // this.sumbitFlow('submit', submitObj);
        } else {
          const obj = {
            status: 'success',
            // val: this.busVal,
            total: money,
            id: relationId
          };
          resolve(obj)
          // let name = `${month}公司月度预算/${this.companyName}/${money}元`
          // this.submitFlowFinal(true, relationId, '', money, name);
        }
      }).catch((e)=>{
        console.log(e)
        resolve(false)
      })
    },
    //relationId主业务的id
    processReturnData(data, relationId) {
      this.getInstanceId(relationId).then(subInstance => {
        let money = math.multiply(data.money,1)
        if (!data.id) {
          let month = this.form.budgetTime.substr(0,7)
          const submitObj = {
            id: relationId, // 业务id
            money,//: data.money,
            name: `${month}公司月度预算/${this.companyName}/${money}元`,
            subInstance
          };
          // resolve(submitObj)
          this.submitFlowFinal(true, submitObj.id, '', money, submitObj.name);
          // this.sumbitFlow('submit', submitObj);
        } else {
          const obj = {
            status: 'success',
            // val: this.busVal,
            total: money,
            id: relationId
          };
          let name = `${month}公司月度预算/${this.companyName}/${money}元`
          this.submitFlowFinal(true, relationId, '', money, name);
        }
      }).catch((e)=>{
        console.log(e)
        resolve(false)
      })
    },
    // 获取流程实例id
    getInstanceId() {
      const flowInstanceBizRelevanceList = [{
        otherBiz: 'depart_monthly_budget'
      }];
      let otherBizIdList = []
      this.budgetList.forEach(item => {
        if(item.otherBizId)otherBizIdList.push(item.otherBizId)
      });
      flowInstanceBizRelevanceList[0].otherBizIdList = otherBizIdList;
      const data = {
        useScope: 'invest',
        initiator: 'all',
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
.card-title{
  align-items: center;
  .title{
    font-weight: 600;
  }
}
::v-deep .el-form-item__label{
  font-weight: 600 !important;
  color: #303133;
}
</style>
