<!--
 * @Descripttion: 公司预算金额调剂申请单
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->

<template>
  <div class='outer'>
    <div class="container">
      <h2 style="text-align:center;margin-bottom:20px">公司预算金额调剂申请单</h2>
      <el-form :model="infoForm" label-width="100px" label-position="right">
        <!-- 预算基本信息 -->
        <el-card class="box-card" style="margin-top:30px">
          <div style="margin-bottom:20px;">
            <h3 style="display:inline-block">预算基本信息</h3>
          </div>
          <div v-if="showFlagList.flag1">
            <el-form ref="ruleForm" :model="infoForm" label-width="120px" :rules='mainRule' :disabled="isDisable">
              <el-row :gutter="20">
                <el-col :span="12">
                  <el-form-item label="公司名称" prop="companyId">
                    <el-select v-model="infoForm.companyId" placeholder="选择公司名称" style="width:100%;"
                      @change="getBudgetByCondition">
                      <el-option v-for="item in companyList" :key="item.id" :label="item.name" :value="item.id">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="预算年度" prop="annual">
                    <el-date-picker v-model="infoForm.annual" format="yyyy 年" value-format="yyyy" type="year"
                      placeholder="选择年" style="width:100%;" @change="getBudgetByCondition">
                    </el-date-picker>
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="20">
                <el-col :span="12">
                  <el-form-item label="预算总额" prop="money">
                    <div style="display: flex;">
                      <el-input-number v-model="infoForm.money" :min="0.00" :precision="6" :step="0.1" :controls="false"
                        style="width:100%;flex:1;" :disabled="operaType == ''">
                      </el-input-number>
                      <div style="position: absolute;right: 12px;">
                        万
                      </div>
                    </div>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="调剂金额分析" prop="remarks">
                    <el-input type="textarea" :rows="3" v-model="infoForm.remarks" placeholder="请输入调剂金额分析"></el-input>
                  </el-form-item>
                </el-col>
              </el-row>
              <!-- <el-row v-if="!isDisable"> -->
              <el-row>
                <el-col :span="24">
                  <eleupload ref="eleupload" v-if="(operaType == 'exam' || operaType == 'check')" :showOnly="true"
                    :attachFile="attachFile">
                  </eleupload>
                  <eleupload ref="eleupload" v-else></eleupload>
                </el-col>
              </el-row>
            </el-form>
          </div>
        </el-card>

        <!-- 预算详情 -->
        <el-card class="box-card" style="margin-top:30px">
          <div style="margin-bottom:20px;">
            <h3 style="display:inline-block">预算详情</h3>
            <span class="show-flag-list" @click="showFlagList.flag2 = !showFlagList.flag2">
              {{ showFlagList.flag2 ? '收起' : '展开' }}
              <i class="el-icon-d-arrow-left" :class="{ 'show-content': showFlagList.flag2 }"></i>
            </span>
          </div>

          <div class="abc" v-if="showFlagList.flag2">
            <el-card v-for="(item, index) in infoForm.budgetDetail" :key="index" class="box-card budgetDetail"
              shadow="hover" style="margin-bottom:10px">
              <div slot="header" class="clearfix">
                <el-select v-model="item.attribute" placeholder="请选择" style="margin-right:10%;" :disabled="true">
                  <el-option v-for="p in attributeList" :key="p.value" :label="p.label" :value="p.value">
                  </el-option>
                </el-select>
              </div>
              <div>
                <el-table :data="item.detailTableData" border :summary-method="getBudgetDetailSummaries" show-summary
                  style="width: 100%">
                  <el-table-column type="index" label="编号"> </el-table-column>
                  <el-table-column label="费用预算类型（一级）" width="180px">
                    <template slot-scope="scope">
                      <el-form-item label-width="0px">
                        <el-input v-model="scope.row.name" :disabled="scope.row.isOriginal" placeholder="请输入费用类型">
                        </el-input>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="是否关联项目" width="280px">
                    <template slot-scope="scope">
                      <el-form-item label-width="0px">
                        <div style="display: flex;">
                          <el-radio :label="true" v-model="scope.row.isProject" :disabled="scope.row.isOriginal">是
                          </el-radio>
                          <el-radio :label="false" v-model="scope.row.isProject" :disabled="scope.row.isOriginal">否
                          </el-radio>
                          <div style="flex:1;">
                            <el-select v-model="scope.row.projectId"
                              :disabled="scope.row.isOriginal || scope.row.isProject == 0" placeholder="请选择">
                              <el-option :key="index" v-for="(val, index) in projectList" :label="val.name"
                                :value="val.id"></el-option>
                              <!-- <el-option label="项目2" :value="false"></el-option> -->
                            </el-select>
                          </div>
                        </div>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="预算总额" prop="money" width="180px">
                    <template slot-scope="scope">
                      <el-form-item label-width="0px">
                        <div style="display: flex;">
                          <el-input-number v-model="scope.row.money" :min="0.00" :precision="6" :step="0.1"
                            :controls="false" :disabled="true" style="width:100%;flex:1;">
                          </el-input-number>
                          <div style="width:40px;padding-left:10px;">
                            万
                          </div>
                        </div>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="剩余预算" prop="budgetRemain" width="180px">
                    <template slot-scope="scope">
                      <el-form-item label-width="0px">
                        <div style="display: flex;">
                          <el-input-number v-model="scope.row.budgetRemain" :min="0.00" :precision="6" :step="0.1"
                            :controls="false" :disabled="true" style="width:100%;flex:1;">
                          </el-input-number>
                          <div style="width:40px;padding-left:10px;">
                            万
                          </div>
                        </div>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="调剂金额" prop="adjustNumber" width="180px">
                    <template slot-scope="scope">
                      <el-form-item label-width="0px">
                        <div style="display: flex;">
                          <el-input-number v-model="scope.row.adjustNumber" :min="0.00" :precision="6" :step="0.1"
                            :controls="false" style="width:100%;flex:1;" :disabled="isDisable">
                          </el-input-number>
                          <div style="width:40px;padding-left:10px;">
                            万
                          </div>
                        </div>
                      </el-form-item>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" fixed="right" width="80px">
                    <template slot-scope="scope">
                      <i class="el-icon-delete-solid delete-class" title="删除"
                        :class="{ 'not-allow': scope.row.isOriginal }"
                        @click="deleteExpendDetail(index, scope.$index, scope.row)"></i>
                    </template>
                  </el-table-column>
                </el-table>
                <el-button type="primary" icon="el-icon-plus" @click="addDetailTableData(index)"
                  style="margin-left: 20px;margin-top: 20px;" circle v-if="!isDisable"></el-button>
              </div>
            </el-card>
          </div>
        </el-card>

      </el-form>
    </div>

  </div>
</template>

<script>
import { capitalMoney, deepClone } from '@/utils';
import eleupload from '@/components/EleUpload';
import Api from '@/api';
import {
  localstorageGet
} from '@/utils/auth';
import numFunc from '@/utils/number'  //重写toFixed
Number.prototype.toFixed = numFunc
export default {
  name: 'CompanyAmountAdjustForm',
  components: { eleupload },
  props: {
    operaType: {
      type: String,
      default: ''
    },
    id: {
      type: String,
      default: ''
    },
  },
  data() {
    return {
      mainRule: {
        companyId: [{ required: true, message: '请选择公司', trigger: 'change' }],
        annual: [{ required: true, message: '请选择预算年度', trigger: 'change' }],
        money: [{ required: true, message: '请输入预算金额', trigger: 'blur' }]
      },
      value: '',
      textarea2: '',
      planShow: false,
      attributeList: [],
      showFlagList: {
        flag1: true,
        flag2: true,
        flag3: true,
        flag4: true
      },
      infoForm: {
        companyId: '',
        companyName: '',
        annual: '',
        money: '',
        remarks: '',
        budgetDetail: [
          {
            attribute: '',
            detailTableData: [ // 初始带出的数据不能被修改
              {
                isOriginal: true,
                name: '',
                money: 0,
                budgetRemain: 0,
                adjustNumber: 0,
                isProject: 0,
                projectId: ''
              }
            ]
          }
        ]
      },
      companyList: [
      ],
      projectList: [
      ],
      temp: {
        name: '',
        money: 0,
        budgetRemain: 0,
        adjustNumber: 0,
        isProject: false,
        projectId: ''
      },
      submitType: '1',
      attachFile: []
    };
  },
  computed: {
    isDisable() {
      if (this.operaType == 'check' || this.operaType == 'exam' || this.operaType == 'edit') {
        // if (this.type == 'exam') {
        return true;
      } else {
        return false;
      }
    }
  },
  watch: {},
  async created() {
    if (this.operaType == 'check' || this.operaType == 'edit') { // 审核 || 查看
      this.adjustId = this.id; // TODO 这里先写死，后序由url传入
      this.getFileByBizId(this.id)
      // if (this.$route.query.adjustId) {
      //   this.adjustId = this.$route.query.adjustId;
      // }
      await this.getBudgetAdjustDetail();
    } else {
      this.infoForm.companyId = localstorageGet('companyId')
    }
    // return
    this.getCompanyList(this.infoForm.companyId);
  },
  mounted() {
  },
  methods: {
    async getBudgetAdjustDetail() {
      await this.$axios.post(
        Api.annualBudget.getBudgetAdjustDetail,
        {
          data: {
            adjustId: this.adjustId
            // id: this.adjustId,
          }
        },
        res => {
          let dataList = res.data.dataList;
          if (dataList.length) {
            let info = dataList[0];
            this.infoForm.companyId = info.companyId
            this.processData(info);
          }
          // console.log('dd')
          // this.companyList = res.data;
          // this.infoForm.companyId = id
          // this.getDepartByCompanyId(id)
          // this.getProjectVosByCompanyId(id)
          // this.getCompanyBudget()
        }
      );
    },
    getCompanyList(id) { // 查询公司列表
      this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id // 当前用的公司id
          }
        },
        res => {
          this.companyList = res.data;
          let index = this.companyList.findIndex(el => el.flag == 'mainDutyCompany')
          if (index > -1) {
            this.infoForm.companyId = this.companyList[index].id
          }

          this.getDepartByCompanyId(this.infoForm.companyId);
          this.getProjectVosByCompanyId(this.infoForm.companyId);
          // this.getCompanyBudget()
        }
      );
    },
    getDepartByCompanyId(id) { // 获取公司部门架构数据
      this.$axios.post(
        Api.annualBudget.getDepartByCompanyId,
        {
          data: { id }
        },
        res => {
          if (res.isSuccess) {
            let data = res.data;
            if (data && data.departmentVos) {
              data.departmentVos.forEach(item => {
                if (item.departmentName == '公司领导') item.departmentName = '公司固定费用'
              })
              this.attributeList = data.departmentVos.map(item => {
                return {
                  value: item.id,
                  label: item.departmentName
                };
              });
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取关联项目
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
            // console.log('res.data', res.data)
            this.projectList = res.data.map(item => {
              return {
                id: item.id,
                name: item.name
              };
            });
          }
        }
      );
    },
    getBudgetByCondition() {
      if (this.infoForm.companyId && this.infoForm.annual) {
        let query = {
          type: 1,
          budgetTime: `${this.infoForm.annual}-01-01 00:00:00`,
          companyId: this.infoForm.companyId
        };
        this.$axios.post(
          Api.annualBudget.budgetList,
          {
            data: query
          },
          res => {
            if (res.isSuccess) {
              let dataList = res.data.dataList;
              if (dataList.length) {
                let info = dataList[0];
                //如果examineStatus ！=1 ，而且没有追加的预算，则表明这条预算
                if (info.examineStatus != 1) {
                  return this.$message.error('未审核完成的预算不可调剂')
                }
                this.processData(info);
                this.getDepartByCompanyId(info.companyId);
                this.getProjectVosByCompanyId(info.companyId);
              } else {
                this.$message.error('所选公司所选年度暂没有预算');
              }
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }
    },
    processData(data) {
      this.infoForm.id = data.id || '';
      this.infoForm.remarks = data.remarks;
      this.infoForm.money = data.money;
      this.infoForm.budgetDetail = [];
      let budgetDetailsVos = data.budgetDetailsVos;
      // budgetDetailsVos
      budgetDetailsVos.forEach(item => {
        let departmentId = item.departmentId;

        let obj = this.createObj(item);
        let index = this.infoForm.budgetDetail.findIndex(it => it.departmentId == departmentId);
        if (index > -1) {
          this.infoForm.budgetDetail[index].detailTableData.push(obj);
        } else {
          let budgetDetailObj = {
            departmentId,
            attribute: departmentId,
            detailTableData: []
          };
          budgetDetailObj.detailTableData.push(obj);
          this.infoForm.budgetDetail.push(budgetDetailObj);
        }
      });
      // 如果有追加，需要再查一次
      let appendCostBudgetVos = data.appendCostBudgetVos;
      if (appendCostBudgetVos.length) {
        appendCostBudgetVos.forEach(item => {
          if (item.examineStatus == 1) {
            let appendBudgetDetailsVos = item.appendBudgetDetailsVos || [];
            this.infoForm.money += (item.money - 0);
            appendBudgetDetailsVos.forEach(it => {
              let departmentId = it.departmentId;
              let idx = this.infoForm.budgetDetail.findIndex(el => el.departmentId == departmentId);
              if (idx > -1) { // 有这个部门
                let budgetTypeId = it.budgetTypeId;
                let lidx = this.infoForm.budgetDetail[idx].detailTableData.findIndex(el => el.budgetTypeId == budgetTypeId);
                if (lidx > -1) { // 有这个归口
                  let money = it.money || 0;
                  let useMoney = it.useMoney || 0;
                  let budgetRemain = money - useMoney;
                  this.infoForm.budgetDetail[idx].detailTableData[lidx].money += (it.money - 0);
                  this.infoForm.budgetDetail[idx].detailTableData[lidx].budgetRemain += budgetRemain;
                } else { // 没有这个归口，新建
                  let obj = this.createObj(it);
                  this.infoForm.budgetDetail[idx].detailTableData.push(obj);
                }
              } else { // 没有部门，新建部门，新建归口
                let budgetDetailObj = {
                  departmentId,
                  attribute: departmentId,
                  detailTableData: []
                };
                let obj = this.createObj(it);
                budgetDetailObj.detailTableData.push(obj);
                this.infoForm.budgetDetail.push(budgetDetailObj);
              }
            });
          }
        });
      }
      // 如果是查看或者审核，需要回显，遍历budgetAdjustVo对象，获取上次调剂填写的信息
      if (this.operaType == 'exam' || this.operaType == 'check' || this.operaType == 'edit') {
        this.infoForm.annual = data.budgetTime.substr(0, 4);
        this.infoForm.companyId = data.companyId;
        let budgetAdjustVoArr = data.budgetAdjustVo;
        if (budgetAdjustVoArr.length) {
          let budgetAdjustVo = budgetAdjustVoArr[0];
          this.infoForm.remarks = budgetAdjustVo.remarks;
          let adjustMoneyVos = budgetAdjustVo.adjustMoneyVos;
          adjustMoneyVos.forEach(item => {
            let budgetDetailsId = item.budgetDetailsId;
            let adjustNumber = item.money;
            // let x, y,adjustNumber
            for (let i = 0; this.infoForm.budgetDetail[i]; i++) {
              let infoFormDetailTableData = this.infoForm.budgetDetail[i].detailTableData;
              let idx = infoFormDetailTableData.findIndex(it => it.budgetDetailsId == budgetDetailsId);
              if (idx > -1) {
                // x = i, y = idx,adjustNumber=infoFormDetailTableData[idx].
                this.infoForm.budgetDetail[i].detailTableData[idx].adjustNumber = adjustNumber;
                break;
              } else {
                //通过budgetDetailsId找归口
                let budgetDetailsId = item.budgetDetailsId
                let checkObj = this.checkByBudgetDetailsId(budgetDetailsId)
                if (checkObj.has) {
                  //adjustNumber = item.budgetDetailsVo.money + (item.surplus - 0)
                  // let money = it.money || 0;
                  // let useMoney = it.useMoney || 0;
                  // let budgetRemain = money - useMoney;
                  this.infoForm.budgetDetail[checkObj.x].detailTableData[checkObj.y].adjustNumber += (item.money - 0);
                  // this.infoForm.budgetDetail[idx].detailTableData[lidx].budgetRemain += budgetRemain;
                } else {
                  let departmentId = item.budgetDetailsVo.budgetTypeVo.departmentId
                  let idx = this.infoForm.budgetDetail.findIndex(el => el.departmentId == departmentId);
                  if (idx > -1) { //有部门
                    this.infoForm.budgetDetail[idx].detailTableData.push(this.createAdjustObj(item))
                  } else {
                    let budgetDetailObj = {
                      departmentId,
                      attribute: departmentId,
                      detailTableData: []
                    };
                    let obj = this.createAdjustObj(item)//this.createObj(it);
                    budgetDetailObj.detailTableData.push(obj);
                    this.infoForm.budgetDetail.push(budgetDetailObj);
                  }
                }
              }
            }
          });
        }
      }
    },
    checkByBudgetDetailsId(budgetDetailsId) {
      let has = false, x, y

      for (let i = 0; this.infoForm.budgetDetail[i]; i++) {
        let detailTableData = this.infoForm.budgetDetail[i].detailTableData
        for (let j = 0; detailTableData[j]; j++) {
          if (detailTableData[j].budgetDetailsId == budgetDetailsId) {
            // return {
            has = true
            x = i
            y = j
            break
            // }
          }
        }
      }
      return {
        has, x, y
      }
    },
    createAdjustObj(item) {
      let departmentId = item.budgetDetailsVo.budgetTypeVo.departmentId
      let useMoney = item.budgetDetailsVo.useMoney || 0;
      let budgetRemain = item.money - useMoney;
      let obj = {
        isOriginal: true,
        budgetId: item.budgetDetailsVo.budgetId,
        budgetTypeId: item.budgetDetailsVo.budgetTypeId,
        budgetDetailsId: item.budgetDetailsId,
        departmentId,
        money: item.money,
        isProject: !!item.budgetDetailsVo.projectId,
        budgetRemain,
        adjustNumber: item.budgetDetailsVo.money + (item.surplus - 0),//0, // item.money,
        projectId: item.budgetDetailsVo.projectId,
        name: item.budgetDetailsVo.budgetTypeVo.name,
        useMoney,
        budgetTypeVoId: item.budgetDetailsVo.budgetTypeVo.id,
        budgetDetailsVosId: item.budgetDetailsVo.id
      }
      return obj
    },
    createObj(item) {
      let isProject = !!item.projectId;
      let departmentId = item.departmentId;
      let useMoney = item.useMoney || 0;
      let budgetRemain = item.money - useMoney;
      let obj = {
        isOriginal: true,
        budgetId: item.budgetId,
        budgetTypeId: item.budgetTypeId,
        budgetDetailsId: item.id,
        departmentId,
        money: item.money,
        isProject,
        budgetRemain,
        adjustNumber: 0, // item.money,
        projectId: item.projectId,
        name: item.budgetTypeVo.name,
        useMoney,
        budgetTypeVoId: item.budgetTypeVo.id,
        budgetDetailsVosId: item.id
      };
      return obj;
    },
    // 预算详情表格操作
    addDetailTableData(index) {
      this.infoForm.budgetDetail[index].detailTableData.push(deepClone(this.temp));
    },
    deleteExpendDetail(cindex, index, row) {
      console.log(cindex, index);
      if (row.isOriginal) {
        return;
      }
      this.infoForm.budgetDetail[cindex].detailTableData.splice(index, 1);
    },
    getBudgetDetailSummaries(param) { // 预算详情表格合计
      // console.log(param);

      let { columns, data } = param;
      let sums = [];
      columns.forEach((column, index) => {
        // if (index === 2) {
        //   sums[index] = '合计：';
        //   return;
        // }
        let values = data.map(item => Number(item[column.property]));
        if (!values.every(value => isNaN(value))) {
          sums[index] = values.reduce((prev, curr) => {
            let value = Number(curr);
            if (!isNaN(value)) {
              return prev + curr;
            } else {
              return prev;
            }
          }, 0);
          sums[index] = sums[index].toFixed(6) //+ ' 元';
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },
    //根据业务id获取文件
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id,
        }
      }).then(res => {
        if (res.isSuccess) {
          let list = res.data
          let attachFile = list.map(item => {
            return {
              id: item.id,
              fileName: item.fileName,
              fileUrl: item.fileUrl
            }
          })
          this.attachFile = attachFile
        }
      })
    },
    submit() {
      // console.log('budgetDetail', this.infoForm.budgetDetail)
      // return
      let con = true
      this.$refs.ruleForm.validate(res => {
        if (!res) {
          con = false
        }
      })
      if (!con) return false
      let fileId = this.$refs.eleupload.getFileId();
      let data = {
        // id: this.infoForm.id,

        companyId: this.infoForm.companyId,
        budgetId: this.infoForm.id,
        enclosure: fileId[0] || '',
        budgetTime: `${this.infoForm.annual}-01-01 00:00:00`,
        budgetDetailsVos: [],
        remarks: this.infoForm.remarks,
        examineStatus: 0
      };
      let budgetDetail = this.infoForm.budgetDetail;
      let total = 0
      this.infoForm.budgetDetail.forEach(item => {
        let detailTableData = item.detailTableData
        detailTableData.forEach(el => {
          total += (el.adjustNumber - 0)
        })
      })
      if (total.toFixed(6) != this.infoForm.money.toFixed(6)) {
        return this.$message.error('调剂金额总和不等于预算总额')
      }
      budgetDetail.forEach(item => {
        let departmentId = item.departmentId;
        let detailTableData = item.detailTableData;
        detailTableData.forEach(el => {
          let adjustNumber = el.adjustNumber || 0;
          let useMoney = el.useMoney || 0;
          let surplus = adjustNumber - useMoney;
          let increaseOrReduce = adjustNumber - el.money; // > 0 ? 'increase' : 'reduce'
          let obj = {
            departmentId,
            projectId: el.projectId,
            money: el.money,
            budgetId: this.infoForm.id,
            type: this.submitType,
            id: el.budgetDetailsVosId || '',
            budgetTypeVo: {
              name: el.name,
              parentId: '1',
              departmentId,
              annually: this.infoForm.annual,
              status: 0,
              type: this.submitType,
              id: el.budgetTypeVoId || ''
            },
            // adjustMoneyVos: [{
            adjustMoneyVo: [{
              money: el.adjustNumber,
              increaseOrReduce: increaseOrReduce + '',
              surplus: surplus + '',
              budgetAdjustId: '', // this.infoForm.id,
              budgetDetailsId: ''// el.budgetDetailsId || ''
            }]
          };
          data.budgetDetailsVos.push(obj);
        });
      });
      // console.log('data', data);
      return this.postData(data);
    },
    postData(data) {
      return this.$axios.post(Api.annualBudget.budgetAdjust, { data })
      // this.$axios.post(Api.annualBudget.budgetAdjust, { data }, res => {
      //   if (res.isSuccess) {
      //     this.$message.success('操作成功');
      //   } else {
      //     this.$message.error(res.message);
      //   }
      // }
      // );
    }
  }
};

</script>
<style lang='scss' scoped>
.outer {
  padding: 10px;
  overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;
  // width: 950px;
  margin: 0 auto;
  text-align: center;

  // .top {
  //   margin: 40px 0 0 40px;
  // }
  .container {
    display: inline-block;
    min-width: 1080px;
    max-width: 1235px;
    margin: auto;
    text-align: initial;
  }
}

.show-flag-list {
  font-size: 16px;
  float: right;
  cursor: pointer;
  transition: all 0.3s ease;

  i {
    transition: all 0.3s ease;
  }
}

.show-content {
  transform: rotate(-90deg);
}

.delete-class {
  color: #1989FA;
  font-size: 16px;
  cursor: pointer;
}

.not-allow {
  cursor: not-allowed;
}

#budgetTypeTable,
#expendDetailTable,
#valueAddedTaxTable {
  ::v-deep .el-form-item--mini.el-form-item {
    margin-bottom: 0px;
  }
}

.budgetDetail {
  ::v-deep {
    .el-card__header {
      padding: 10px 20px;
      background: rgb(250, 250, 250);
    }

    .el-radio {
      margin-right: 12px;
      margin-top: 8px;
    }
  }
}
</style>

