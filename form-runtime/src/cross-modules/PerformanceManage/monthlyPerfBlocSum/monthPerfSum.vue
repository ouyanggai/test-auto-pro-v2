<template>
  <div class="sumFlow">
    <div class='print' ref="print" >
      <h2 style="text-align:center;padding:5px">{{ company.name }}</h2>
      <h3 style="text-align:center;">
        <span v-if="this.actionType == 'examine' || this.actionType == 'preview'">{{ targetTime.substr(0,4)+'年'+targetTime.substr(4,2)+'月'}}</span>
        <el-date-picker
          v-else
          v-model="targetTime"
          type="month"
          ref="datepicker"
          placeholder="考核时间"
          format="yyyy年M月"
          value-format="yyyyM"
          :clearable="false"
          :disabled="this.actionType == 'examine' || this.actionType == 'preview'"
          style="width:130px"
          @change="changeDate"
          ></el-date-picker>
        {{isCompany || isCompanyDetail ? '公司' : '部门'}}月度绩效汇总表</h3>
      <!-- <span>考核时间：</span>
      <el-date-picker
        v-model="targetTime"
        type="month"
        ref="datepicker"
        placeholder="考核时间"
        format="yyyy年M月"
        value-format="yyyyM"
        :clearable="false"
        :disabled="this.actionType == 'examine' || this.actionType == 'preview'"
        style="width:300px;margin-right:30px;"
        @change="changeDate"
        ></el-date-picker> -->
        <span style="padding:10px 0 5px 0;float: right;font-size:15px">
          <span style="margin-right:110px;vertical-align: middle;">加分比例：{{ addPointRatio }}%</span>
          <span class="colorTip"><span class="square"></span><span class="text">未提交月度绩效</span></span>
          <span>{{isCompany || isCompanyDetail ? '公司' : '部门'}}人员总数：{{companyPersonNumber}}人</span>
          <span style="margin:0 30px">已提月度绩效：{{submitted()}}人</span>
        </span>
      <!-- 公司/部门：
      <el-cascader
        v-model="targetScope"
        ref="elcascader"
        :props="selectProps"
        :options="treeData[0].childrenList"
        :expandTrigger="true"
        style="width: 300px"
        popper-class="monthPerfSum20240514"
        @change="changeScope"
      >
        <span slot-scope="{ node, data }" :title="data.name">
          <span v-if="data.companyType=='PLATFORM_COMPANY'" :companyType="data.companyType">{{data.name}}</span>
          <span v-else  :companyType="data.companyType">{{data.name}}</span>
        </span>
      </el-cascader> -->
      <div style="margin-top:10px" class="attachFiles">
        <eleupload :showOnly="actionType != 'create' && actionType != 'edit'" ref="eleupload" :size="20" :attachFile="attachFile" :showFullName="true"></eleupload>
      </div>
      <table border="1" bordercolor="black" width="100%" cellspacing="0" cellpadding="0" class="main_table">
            <tr>
                <td style="width: 60px;">序号</td>
                <td style="width: 120px;">考核时间</td>
                <!-- <td style="width: 200px;">公司</td> -->
                <td>部门</td>
                <td>姓名</td>
                <td>最终得分</td>
                <!-- <td>加分值</td>
                <td>扣分值</td> -->
                <td>奖励分值</td>
                <td>扣罚分值</td>
                <td>岗效比例</td>
                <td>调薪情况</td>
                <td>考核结果</td>
                <!-- <td>请假信息</td> -->
                <td>签字确认</td>
                <th class="no-print">查看</th>
            </tr>
            <tr v-for="(i,index) in kpiList" :key="i.id" :style="{backgroundColor: i.id || 'rgba(255, 242, 204, 0.5)'}">
                <td>{{ index+1 }}</td>
                <td>{{`${String(i.targetTime).substr(0,4)}年${String(i.targetTime).substr(4)}月`}}</td>
                <!-- <td>{{ i.companyName }}</td> -->
                <td>{{ i.depName }}</td>
                <td>{{ i.userName }}</td>
                <td>{{ i.totalScore }}</td>
                <!-- <td>{{ i.extraPointsValue || (i.id ? '-' : '') }}</td>
                <td>{{ i.deductPointsValue || (i.id ? '-' : '') }}</td> -->
                <td>{{ i.rewardPonitsValue || (i.id ? '-' : '') }}</td>
                <td>{{ i.punishPonitsValue || (i.id ? '-' : '') }}</td>
                <td>{{(i.totalKpi * 100).toFixed(2) + '%'}}</td>
                <td :title="findDesc(i.kpiDynamicItemList,'change_salary')" style="max-width:200px;" class="editIcon_td">
                  {{ findDesc(i.kpiDynamicItemList,'change_salary') || (i.id ? '-' : '') }}
                  <i v-if="i.id && permissionSet.has('chang_salary')" class="el-icon-edit-outline editIcon no-print" title="修改" @click="editIconClick(i.kpiDynamicItemList,'change_salary', i)"></i>
                </td>
                <td style="max-width: 250px;" class="editIcon_td">
                  <span v-if="findDesc(i.kpiDynamicItemList,'ask_for_leave') != ''">
                    {{ findDesc(i.kpiDynamicItemList,'ask_for_leave') || '' }}
                  </span>
                  <span v-if="findDesc(i.kpiDynamicItemList,'ask_for_leave') == ''">
                    {{ `${math.multiply(i.totalKpi, 100).toFixed(2) + '%'}岗效工资+基本工资+餐费20元*${i.workDays||0}` }}
                  </span>
                  <i v-if="i.id && permissionSet.has('kpi_result')" class="el-icon-edit-outline editIcon no-print" title="修改" @click="editIconClick(i.kpiDynamicItemList,'ask_for_leave', i)"></i>
                </td>
                <!-- <td style="max-width:300px" :title="findDesc(i.kpiDynamicItemList,'ask_for_leave')">
                  {{ findDesc(i.kpiDynamicItemList,'ask_for_leave') || (i.id ? '-' : '') }}
                </td> -->
                <td>{{ i.kpiGroupStatus == 'pass' ?  i.userName : ''}}</td>
                <td class="no-print"><el-button v-if="i.id" type="text" @click="checkDetail(i)">详情</el-button></td>
            </tr>
            <tr v-if="!kpiList.length">
              <td colspan="30" style="height:70px;">暂无数据</td>
            </tr>
            <tr>
                <td colspan="2">备注</td>
                <!-- <td colspan="10"><el-input type="textarea" v-model="remark" :disabled="this.actionType == 'examine' || this.actionType == 'preview'"></el-input></td> -->
                <td colspan="12"><el-input type="textarea" :autosize="{ minRows: 5,maxRows: 5 }" show-word-limit maxlength="500" v-model="remark"></el-input></td>
            </tr>
            <!-- <tr>
                <td colspan="12">审签意见</td>
            </tr>
            <tr>
                <td colspan="2">子公司总经理</td>
                <td colspan="4"></td>
                <td colspan="2.5">集团人力资源部</td>
                <td colspan="4"></td>
            </tr>
            <tr>
                <td colspan="2">集团人力资源分管领导</td>
                <td colspan="4"></td>
                <td colspan="2">集团总经理</td>
                <td colspan="4"></td>
            </tr> -->
      </table>
      <div style="margin:10px" class="attack_button" v-if="actionType != 'preview'"> <!-- v-if="actionType == 'create'" -->
        <!-- <el-button type="primary" @click="kpiList2.push(getEmptyKpi())">新增附加项</el-button> -->
        <el-button type="primary" @click="addAttach">新增附加项</el-button>
        <el-button @click="_=>{importKpiList()}">导入最近一次数据</el-button>
      </div>
      <div class="attack_table" v-if="kpiList2.length">附加表</div>
      <table border="1" bordercolor="black" width="100%" cellspacing="0" cellpadding="0" style="margin-top:0" v-if="kpiList2.length">
            <tr>
                <td style="width: 60px;">序号</td>
                <td style="width: 120px;">考核时间</td>
                <!-- <td style="width: 200px;">公司</td> -->
                <td>部门</td>
                <td>姓名</td>
                <td style="width: 65px;">最终得分</td>
                <td style="width: 65px;">奖励分值</td>
                <td style="width: 65px;">扣罚分值</td>
                <td style="width: 65px;">岗效比例</td>
                <td>调薪情况</td>
                <td>考核结果</td>
                <!-- <td>签字确认</td> -->
                <td v-if="actionType != 'preview'" class="no-print">操作</td>
            </tr>
            <tr v-for="(i,index) in kpiList2" :key="i.id" :class="'attach'+index">
                <td>{{ index+1 }}</td>
                <td>{{`${targetTime.substr(0, 4)}年${targetTime.substr(4)}月`}}</td>
                <!-- <td>{{ i.companyName }}</td> -->
                <td>
                  <el-select v-model="i.depName" :disabled="false">
                    <el-option v-for="(val, index) in departmentList" :key="index" :label="val.departmentName" :value="val.departmentName"></el-option>
                  </el-select>
                </td>
                <td><el-input v-model="i.userName"></el-input></td>
                <td><el-input v-model="i.totalScore" @input="val=>{numConvert(i,'totalScore',val)}"></el-input></td>
                <td><el-input v-model="i.rewardPonitsValue" @input="val=>{numConvert(i,'rewardPonitsValue',val)}"></el-input></td>
                <td><el-input v-model="i.punishPonitsValue" @input="val=>{numConvert(i,'punishPonitsValue',val)}"></el-input></td>
                <td>{{i.totalKpi}}</td>
                <td>
                  <pre class="comment-print">{{ i.changeSalary }}</pre>
                  <el-input class="no-print" type='textarea'  :autosize="{ minRows: 3,maxRows: 3 }" v-model="i.changeSalary"></el-input>
                </td>
                <!-- <td>
                  {{ `${i.totalKpi || 0}%岗效工资+基本工资+餐费20元*`}}
                  <input v-model="i.workDays" style="line-height:25px;width:40px;border: 1px solid #dcdfe6;border-radius: 5px;" placeholder="天数"/>
                  <input v-model="i.comment" style="line-height:25px;min-width:60px;margin-left:5px;border: 1px solid #dcdfe6;border-radius: 5px;" placeholder="备注"/>
                </td> -->
                <td>
                  <pre class="comment-print">{{ i.comment }}</pre>
                  <el-input class="no-print" style="min-width:160px;" type='textarea' v-model="i.comment" :autosize="{ minRows: 3,maxRows: 3 }" show-word-limit maxlength="500" />
                </td>
                <!-- <td><el-input v-model="i.userName"></el-input></td> -->
                <td class="no-print" v-if="actionType != 'preview'"><i class="el-icon-delete" style="color:#1989fa;font-size:17px;cursor:pointer;" title="删除"
                  @click="_=>{kpiList2.splice(index,1)}"></i></td>
            </tr>
            <tr v-if="!kpiList2.length">
              <td colspan="30" style="height:70px;">暂无数据</td>
            </tr>
      </table>
      <div class="flow-log-container" v-if="postscriptList.length">
        <div direction="vertical" style="color: 000;margin-top:10px;font-size:12px;">
          <div style="background:rgb(140,140,140);">发起人附言</div>
          <div v-for="(val, index) in postscriptList" :key="index"
            style="padding:6px 10px;margin:5px 0;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
            <div style="display:flex;">
              <div style="margin-right:5px;width:80px;">{{ val.replyName || val.sendName }}</div>
              <div style="margin-right:30px;">{{ val.createDate }} </div>
            </div>
            <div style="margin-left:5px;width:100%;">{{val.text}}</div>
            <span style="margin-left:10px;color: #47a1fb;" v-if="val.relationFileDataVos && val.relationFileDataVos.length>0"><span style="margin-left:5px" :key="file.id" v-for="file in val.relationFileDataVos">{{ file.fileName }}</span></span>

            <div v-if="val.children.length" style="margin-left:10px;border: 1px solid #ccc;padding: 4px;margin: 5px 0px;" class="script-item-child">
              <div v-for="( childItem, childIndex) in val.children" :key="childItem.id">
                <div class="item-info-child">
                  <span style="margin-right:30px;">{{ childItem.replyName || childItem.sendName }}</span>
                  <span class="item-info-date">{{ childItem.createDate }}</span>
                </div>
                <div style="text-indent: 1rem;">{{ childItem.text }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="flow-log-container">
        <div direction="vertical" style="color: 000;margin-top:10px;font-size:12px;">
          <div style="background:rgb(140,140,140);">流程日志</div>
          <div v-for="(val, index) in logTableData" :key="index"
            style="display: flex;padding:6px 10px;margin:5px 0;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
            <div style="margin-right:5px;width:80px;">{{ val.executorName }}</div>
            <div style="margin-right:5px;width:80px;">{{ val.auditStatus }} </div>
            <!-- <div style="margin-right:30px;">
              {{ val.auditStatus }}
            </div> -->
            <div style="margin-right:30px;">
              {{ val.createDate }}
            </div>
            <div>
              {{val.executeDesc}}
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="botton-group">
          <el-button type="primary" icon='el-icon-view' v-if="actionType == 'create'" @click="$parent.handleCheckFlow()">查看流程</el-button>
          <!-- <el-button type="primary" @click="print">打 印</el-button> -->
          <el-button @click="handleClose" v-if="actionType == 'create'">取 消</el-button>
          <el-button type="primary" @click="savePerfSum" v-if="actionType == 'create'">提 交</el-button>
          <el-button type="primary no-print" style="margin:0 auto" @click="exportTableData()" v-if="actionType != 'create'">导出</el-button>
          <!--  || actionType == 'edit' -->
    </div>
    <examine-dialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="isReInitiate2" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId2"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId2" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId2"
      :initiatorId="initiatorId" @success="()=>{}" :actionType="actionType2" :flowType="flowType" :flowName="flowName"/>
      <el-dialog
      v-dialogDraw
      v-if="editRowVisible"
      :title="editRow.userName + '-' + editRow.title"
      :visible="editRowVisible"
      width="500px"
      :modal="false"
      append-to-body
      @close="editRowVisible = false"
      :close-on-click-modal="false"
    >
      <el-form
        ref="form1"
        :model="editRow"
        :rules="formRules"
        label-width="50px"
      >
        <el-form-item label="" prop="desc">
          <el-input
            class="my_input"
            v-model="editRow.desc"
            style="width: 350px"
          ></el-input>
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="_=>{editRowVisible = false}">取 消</el-button>
        <el-button type="primary" @click="confirmClick">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import {
  localstorageGet, localstorageSet
} from '@/utils/auth';
import Api from '@/api';
import eleupload from '@/components/EleUpload';
import { Print as $print } from '@/utils/print.js';
import math from '@/utils/math.js'
// import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
const ExamineDialog = () => import('@/views/GroupApproveManage/components/ExamineDialog.vue');
export default {
  name: 'MonthPerfSum',
  components: { eleupload, ExamineDialog },
  props: {
    bizId: { // 流程绑定的业务id，提交绩效汇总数据返回的id
      type: String,
      default: ''
    },
    selectFlowType: { // 流程类型
      type: String,
      default: ''
    },
    actionType: { // 动作类型：发起 编辑 查看等
      type: String,
      default: 'create'
    },
    flowNodeProxyId: { // 流程节点id
      type: String,
      default: ''
    },
    flowProxyId: { // 流程id
      type: String,
      default: ''
    },
    flowInstanceId: { // 流程实例id
      type: String,
      default: ''
    },
    isReInitiate: { // 是否重新发起
      type: Boolean,
      default: false
    }
  },
  data () {
    return {
      permissionSet: new Set([]),
      formRules: {
        desc: [{ required: true, trigger: "blur", message: "不能为空" }],
      },
      math,
      ExpensesClaimFormVisible:false,
      flowType: 'monthly_perf',
      operaType:'',
      flowId:'',
      flowInstanceId2:'',
      flowNodeType:'',
      showFlowLog: true,
      parallelNodeChooseList:'',
      manualChooseNodes:'',
      formId:'',
      searchFlowType:'',
      flowNodeProxyId2:'',
      noFormFlowInstanceId:'',
      flowProxyId2:'',
      initiatorId:'',
      actionType2:'',
      flowName:'',
      isExamine:false,
      isReInitiate2:false,
      selectFlowType2: '',

      attachFile: [],
      remark: '',
      isCompany: this.selectFlowType == 'monthly_perf_companySum',
      isCompanyDetail: this.$route.path.includes('monthlyPerfCompanySum'),
      companyDeta: {},
      departmentData: {},
      departmentList: [],
      kpiList: [],
      kpiList2: [],
      targetTime: '',
      targetScope: '',
      val: '',
      status: '',
      treeData: [],
      selectProps: {
        value: 'id',
        label: 'name',
        children: 'childrenList',
        checkStrictly: true,
        emitPath: false
      },
      postscriptList: [],
      logTableData: [],
      company: { id: this.$store.state.user.companyId, name: this.$store.state.user.companyName },
      companyPersonNumber: 0,
      addPointRatio: 0,
      editRow: {},
      editRowVisible: false
    };
  },
  inject: {
    prevStepHandle: { value: 'prevStepHandle', default: null },
    sumbitFlow: { value: 'sumbitFlow', default: null },
    submitFlowFinal: { value: 'submitFlowFinal', default: null },
    activeRow: { value: 'activeRow', default: null }
  },
  methods: {
    exportTableData() {
      var bizId = this.$route.query.id || this.bizId;
      const param = {
        data: { id: bizId },
        // pagination: false
      };
      this.$axios.post(
        '/web/plan/api/kpiSummary/export',
        param,
        (res, originResponse) => {
          if (originResponse.headers['content-disposition']) {
            var queryFileName = decodeURI(originResponse.headers['content-disposition'].split(';')[1].split('filename=')[1], 'UTF-8');
          }
          var fileName = queryFileName || '月度绩效汇总.xlsx';
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            // const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            link.download = fileName;
            link.click();
            // document.body.removeChild(link);
            window.URL.revokeObjectURL(link.href);
            link.remove();
          }
        }, '', { responseType: 'blob' }
      );
    },
    confirmClick() {
      this.$refs.form1.validate((valid) => {
        if (valid) {
          this.$axios.post(
            '/web/plan/api/kpiGroup/updateKpiDynamicItem',
            {
              data: this.editRow
            },
            (res) => {
              if (res.isSuccess) {
                this.$message.success('修改成功');
                this.editRowVisible = false;
                var bizId = this.$route.query.id || this.bizId;
                if (bizId) {
                  this.kpiSumFindById(bizId);
                } else {
                  this.getListBefore(this.targetTime);
                }
                // this.getTableData();
              }
            }
          );
        } else {
          console.log('error submit!!');
          return false;
        }
      });
    },
    editIconClick(data, type, row) {
      var fi = data?.find(i => i.kpiDynamicType == type);
      this.editRow = {
        id: fi.id,
        userName: row.userName,
        desc: fi.desc || (type == 'ask_for_leave' ? `${math.multiply(row.totalKpi, 100).toFixed(2) + '%'}岗效工资+基本工资+餐费20元*${row.workDays || 0}` : ''),
        title: ({ change_salary: '调薪情况', ask_for_leave: '考核结果' })[type]
      };
      console.log(this.editRow, 'editRow');
      this.editRowVisible = true;
    },
    getPersonNumber() {
      var isCompany = this.isCompany || this.isCompanyDetail;
      const params = {
        data: {
          company: {
            id: this.$store.state.user.companyId
          },
          department: {
            id: isCompany ? undefined : localstorageGet('userDepartmentId')
          },
          assessmentCycle: 'month',
          applyScope: isCompany ? 'company' : 'department',
          targetTime: this.targetTime
        }

      };
      this.$axios.post(
        Api.performance.KpiSummaryUserSortSetList,
        params,
        (res) => {
          if (res.isSuccess) {
            const data = res.data.users || [];
            this.companyPersonNumber = data.length;
            this.addPointRatioCompute();
          }
        }
      );
    },
    previewHandle(row, type) {
      if (type == 'monthly_perf') { // 月度绩效
        this.selectFlowType2 = row.auditWay;
        this.formExist = row.formExist;
        this.operaType = 'check';
        this.actionType2 = 'preview';
        this.isExamine = false;
        this.isReInitiate2 = false;
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.flowId = find.otherBizId;
        }
        this.flowInstanceId2 = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.searchFlowType = row.auditWay;
        this.flowProxyId2 = row.flowProxyId
        this.ExpensesClaimFormVisible = true;
      }
    },
    checkDetail(row, type = 'monthly_perf') {
      this.getInstanceId(row.id, type).then(data => {
        if (data) {
          this.previewHandle(data, type);
        } else {
          this.$message.error('流程已删除，请删除本条月度绩效后重新发起月度流程');
        }
      });
    },
    getInstanceId(id, type, taskStatus) {
      let otherBiz = type ? type :'monthly_perf'
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
    addAttach() {
      this.kpiList2.push(this.getEmptyKpi());
      const index = this.kpiList2.length - 1;
      this.$nextTick(() => {
        if (index > -1) {
          const attach = document.querySelector('.attach' + index);
          if (attach)attach.scrollIntoView({ behavior: 'smooth' });
        }
      });
    },
    numConvert(item, key, val) {
      item[key] = val.replace(/[^0-9.]/g, '');
      if (val.length == 1 && val == '.') {
        item[key] = '';
      }
      key == 'totalScore' && this.debounceGetVal(item);
    },
    getEmptyKpi() {
      return {
        targetTime: this.targetTime,
        companyName: this.companyDeta.name || localstorageGet('companyName'),
        depName: this.departmentData?.departmentName,
        userName: '',
        totalScore: '',
        rewardPonitsValue: '',
        punishPonitsValue: '',
        totalKpi: '',
        subsidy: '20',
        changeSalary: '',
        workDays: '',
        dutyName: '',
        comment: '',
        manageType: 'work_and_manager_target',
        assessmentCycle: 'month',
        setTime: null
      };
    },
    addPointRatioCompute() {
      if (this.kpiList.length) {
        var addTotal = this.kpiList.reduce((prev, curr) => {
          return prev + ((curr?.extraPointsValue || curr?.rewardPonitsValue) ? 1 : 0);
        }, 0);
        this.addPointRatio = (addTotal / this.companyPersonNumber * 100).toFixed(2);
        return;
      }
      this.addPointRatio = '0.00';
    },
    submitted() {
      return this.kpiList.reduce((prev, curr) => prev + (curr?.id ? 1 : 0), 0);
    },
    getAttachmentList(id) { // 1、根据业务id获取附件文件回显
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
              fileUrl: item.fileUrl,
              absolutelyFileUrl: item.fileUrl
            };
          });
          this.attachFile = attachFile;
        }
      });
    },
    bindBatchFileByIds(relationId) { // 多个文件绑定业务id
      const fileIds = this.$refs.eleupload.getFileId();
      const data = {
        relationId,
        fileIds
      };
      return this.$axios.post(
        Api.budgetManage.saveBatchFile,
        { data }
      );
    },
    findDesc(list, val) {
      return list?.find(i => i.kpiDynamicType == val)?.desc || '';
    },
    changeScope() {
      this.$refs.elcascader.dropDownVisible = false;
    },
    getCustomerTree() { // 客户组织架构
      const params = {
        data: {
          clienteleId: this.$store.state.user.customerCode // 查客户组织架构，带用户id
        }
      };
      this.$axios.post(
        '/web/user/api/clienteleCompany/findCompany',
        params,
        (res) => {
          if (res.isSuccess) {
            this.treeData = res.data;
            if (res?.data?.length > 0) {
              // this.selectId = res.data[0].id;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    submit(status) {
      console.log('保存草稿');
      this.updateData(undefined, () => {
        this.prevStepHandle();
      });
      // this.updateData(status == 0 ? 'not_submitted' : status, () => {
      //   this.prevStepHandle();
      // });
      return;
      if (status == 0) {
        this.status = 'not_submitted';
        this.submitData('form', 'nocheck').then(() => {
          this.$message.success('操作成功');
          this.prevStepHandle();
        });
      } else {
        this.status = status;
        if (status == 'under_review') {
          this.sumbitFlow('submit');
        } else {
          this.sumbitFlow('draft');
        }
      }
      // 提交之前的校验,之后自动调用 postData方法提交业务和绑定流程
    },
    postData(status, batchCode) { // 调用汇总接口，返回业务id再提交流程和绑定
      // var id, name;
      // if (this.bizId) {
      //   id = this.bizId;
      //   name = 'monthPerfSumName2';
      // } else {
      //   // this.saveperfData();
      //   id = 'monthPerfSumId2';
      //   name = 'monthPerfSumName2';
      // }
      var isCompany = this.selectFlowType == 'monthly_perf_companySum';
      const params = {
        batchCode,
        data: {
          company: {
            id: this.$store.state.user.companyId
          },
          department: {
            id: isCompany ? undefined : localstorageGet('userDepartmentId')
          },
          assessmentCycle: 'month', // 考核范围，month月度绩效考核，年度考核暂时不考虑
          examineStatus: 'under_review', // 审核状态under_review审核中、finish已完成，页面状态以流程为准
          applyScope: isCompany ? 'company' : 'department',
          remark: this.remark,
          targetTime: this.targetTime,
          kpiList: this.kpiList.map(i => {
            return i.id ? ({ id: i.id }) : false;
          }).filter(Boolean),
          supplements: this.kpiList2
        }
      };
      // if(id)params.data.id = id
      this.$axios.post(
        Api.performance.kpiSummarySave,
        params,
        (res) => {
          if (res.isSuccess) {
            var { data: { id }} = res;
            var str = String(this.targetTime);
            var name = `${str.substr(0, 4)}-${str.substr(4)}${isCompany ? '公司' : '部门'}月度绩效汇总-${this.company.name}`;
            console.log(id, name, 'id-name');
            id && this.submitFlowFinal(true, id, '', '', name);
            id && this.bindBatchFileByIds(id);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    updateData(examineStatus, callback) {
      var id = this.$route.query.id || this.bizId;
      this.$axios.post(
        Api.performance.kpiSummaryUpdate,
        {
          data: {
            id,
            examineStatus: examineStatus || undefined,
            remark: this.remark,
            kpiList: this.kpiList.map(i => {
              return i.id ? ({ id: i.id }) : false;
            }).filter(Boolean),
            supplements: this.kpiList2
          }
        },
        (res) => {
          if (res.isSuccess) {
            this.bindBatchFileByIds(id);
            callback();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    savePerfSum() {
      this.sumbitFlow('submit');
    },
    handleClose() {
      this.prevStepHandle();
    },
    async printPage() {
      await Promise.all([this.fetchLogData(), this.getPostScriptList()]);
      var printInst = $print(this.$refs.print, {}, () => {
        console.log(printInst, 'printInst-弹窗关闭');
      });
    },
    getPostScriptList() {
      return this.$axios.post(
        Api.approveManage.getPostScriptList,
        {
          data: {
            flowInstanceId: this.flowInstanceId
          }
        },
        (res) => {
          if (res.isSuccess) {
            this.postscriptList = this.generateTree(res.data);
            // this.postscriptList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree(flatArray) {
      // 创建一个映射，用于存储每个节点的引用
      const nodeMap = {};
      // 创建一个数组，用于存储树的根节点
      const tree = [];

      // 遍历扁平数组，初始化每个节点
      flatArray.forEach(item => {
        nodeMap[item.id] = { ...item, children: [] };
      });

      // 再次遍历扁平数组，构建树结构
      flatArray.forEach(item => {
        const node = nodeMap[item.id];
        if (item.pid === null) {
          // 如果没有父节点，则为根节点
          tree.push(node);
        } else {
          // 如果有父节点，则将当前节点添加到父节点的子节点数组中
          const parentNode = nodeMap[item.pid];
          if (parentNode) {
            node.isReplay = true;
            parentNode.children.push(node);
          }
        }
      });
      return tree;
    },
    fetchLogData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.approveManage.findRecord,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          res => {
            if (res.isSuccess) {
              this.logTableData = this.filterWithdraw(res.data);
              this.logTableData.forEach(item => {
                this.translateStatus(item);
              });
              resolve()
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    filterWithdraw(data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    // 已建任务和流程日志操作状态字符转换
    translateStatus(obj) {
      let chnStatus;
      if (obj.auditStatus) {
        switch (obj.auditStatus) {
          case 'pass':
            chnStatus = '通过';
            break;
          case 'no_pass':
            chnStatus = '驳回';
            break;
          case 'withdraw':
            chnStatus = '撤销';
            break;
          case 'retrieve':
            chnStatus = '取回';
            break;
          case 'transfer':
            chnStatus = '移交';
            break;
          case 'roll_back_the_previous_level':
            chnStatus = '回退上一节点';
            break;
          default:
            chnStatus = '';
            break;
        }
      } else if (obj.flowStatus) {
        switch (obj.flowStatus) {
          case 'await_sent':
            chnStatus = '待发';
            break;

          case 'run':
            chnStatus = '运行中';
            break;

          case 'withdraw':
            chnStatus = '撤销';
            break;

          case 'termination':
            chnStatus = '终止';
            break;

          case 'rejected':
            chnStatus = '驳回';
            break;

          case 'end':
            chnStatus = '完结';
            break;

          default:
            chnStatus = '';
            break;
        };
      }
      obj.auditStatus = chnStatus;
    },
    changeDate(date) {
      this.getListBefore(date);
    },
    debounceGetVal(item) {
      if (this.setTime) clearTimeout(this.setTime);
      this.setTime = setTimeout(() => {
        this.calculateKpi(item);
      }, 300);
    },
    calculateKpi(item) {
      item.totalKpi = '0.00';
      var totalScore = item.totalScore.trim();
      if (!totalScore) return;
      this.$axios.post(
        Api.performance.calculateKpi,
        {
          data: {
            assessmentCycle: 'month',
            totalScore
          }
        },
        (res) => {
          if (res.isSuccess) {
            if (res.data) {
              item.totalKpi = (res.data.totalKpi * 100).toFixed(2);
            }
          }
        }
      );
    },
    importKpiList() {
      var isCompany = this.selectFlowType == 'monthly_perf_companySum';
      var companyId = this.$store.state.user.companyId;
      var departmentId = isCompany ? undefined : localstorageGet('userDepartmentId');
      // var applyScope = isCompany ? 'company' : 'department';
      this.$axios.post(
        Api.performance.getLastKpiSummary,
        {
          data: {
            company: {
              id: companyId
            },
            department: {
              id: departmentId
            },
            // applyScope: applyScope,
            // manageType: 'work_and_manager_target',
            targetTime: this.targetTime
          }
        },
        (res) => {
          if (res.isSuccess) {
            if (res.data?.supplements) {
              // var data = [res.data.supplements || false].filter(Boolean);
              // data.map(i => {
              //   i.id = Date.now();
              // });
              this.kpiList2 = res.data?.supplements || [];
            }
          }
        }
      );
    },
    getListBefore(date) {
      var isCompany = this.selectFlowType == 'monthly_perf_companySum';
      var companyId = this.$store.state.user.companyId;
      var departmentId = isCompany ? undefined : localstorageGet('userDepartmentId');
      var applyScope = isCompany ? 'company' : 'department';
      this.getList(date, companyId, departmentId, applyScope);
    },
    getList(date, companyId, departmentId, applyScope) {
      const params = {
        data: {
          company: { // 发起公司，必填
            id: companyId // '8fe922a5da21445a8a26aba74d0af5e1'
          },
          department: { // 发起部门，公司发起，可以不用填 '50d063d8a418491e9687ccfa7867f108'
            id: departmentId
          },
          applyScope: applyScope, // 发起范围，公司发起company，部门发起department
          manageType: 'work_and_manager_target', // 考核类型月度绩效为该值
          targetTime: date // 考核时间
        }
      };
      this.$axios.post(
        Api.performance.getKpiGroupByTargetTime,
        params,
        (res) => {
          if (res.isSuccess) {
            this.kpiList = res.data || [];
            this.addPointRatioCompute();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    kpiSumFindById(id) {
      this.$axios.post(
        Api.performance.findById,
        { data: { id }},
        (res) => {
          if (res.isSuccess) {
            var { data: { remark, targetTime, applyScope, company: { id: companyId }, department, supplements }} = res;
            this.targetTime = `${targetTime}`;
            this.remark = remark;
            console.log(department, 'department233');
            this.companyDeta = res.data.company;
            this.kpiList2 = supplements || [];
            if (this.actionType == 'create') {
              this.getList(this.targetTime, companyId, department?.id || undefined, applyScope);
            } else {
              this.company.name = res.data?.company?.name || '';
              this.company.id = res.data?.company?.id || '';
              this.kpiList = res.data.kpiList || [];
              this.addPointRatioCompute();
            }
            this.getDepartmentList();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getDepartmentList() {
      this.$axios.post(
        Api.frameworkInfo.getUperDepartmentList,
        {
          data: {
            relationId: this.companyDeta.id || localstorageGet('companyId'), // 公司id
            type: 'company'
          }
        },
        res => {
          if (res.isSuccess) {
            var list = [];
            const fn = (source) => {
              source.forEach((el) => {
                list.push(el);
                (el.sysDepartmentVos && el.sysDepartmentVos.length > 0) && fn(el.sysDepartmentVos);
              });
            };
            fn(res.data);
            this.departmentList = list;
            this.departmentData = list.find(i => i.id == localstorageGet('userDepartmentId'));
          }
        }
      );
    },
    getEditPermision() {
      console.log(this.getEditPermision, 'getEditPermision')
      this.$axios.post(
        Api.qualityManage.findApprovePermission,
        {
          data: {},
          nodeProxyId: this.flowNodeProxyId
        },
        (res) => {
          let enableList = [];
          if (res.data && res.data?.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
            this.permissionSet = new Set(enableList);
          } else {
            this.permissionSet = new Set([]);
          }
        }
      );
    },
    getInitPermision() {
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      this.$axios.post(
        url,
        {
          data: {
            id: this.flowProxyId // 流程id
          }
        },
        (res) => {
          var enableList = [];
          if (res.data && res.data.flowNodeTemplate && res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList) {
            const tmpList = res.data.flowNodeTemplate.flowNodeFieldPowerTemplateList || [];
            enableList = tmpList.map(item => {
              return item.formFieldTemplateEnglishName;
            });
            this.permissionSet = new Set(enableList);
          } else {
            this.permissionSet = new Set([]);
          }
        }
      );
    },
  },
  mounted() {
    this.getPersonNumber();
    if (this.actionType == 'create') {
      this.$refs?.datepicker?.focus();
    };
    console.log(this.bizId || null, 'bizId');
    console.log(this.flowProxyId, 'flowProxyId');
    console.log(this.actionType, 'actionType');
    console.log(this.selectFlowType, 'selectFlowType');
  },
  created() {
    if (this.actionType == 'examine') {
      this.getEditPermision();
    } else if (this.actionType == 'create' || this.actionType == 'edit') {
      this.getInitPermision();
    } else if (this.actionType == 'preview') {
      this.permissionSet = new Set([]);
    }
    this.getDepartmentList();
    if (this.actionType == 'create') {
      // this.targetTime = new Date();
      // this.$nextTick(() => {
      //   var date = `${this.targetTime.getFullYear()}${this.targetTime.getMonth() + 1}`;
      //   this.targetTime = date;
      //   this.getListBefore(date);
      // });
    } else {
      var bizId = this.$route.query.id || this.bizId;
      this.kpiSumFindById(bizId);
      this.getAttachmentList(bizId);
    }
    // this.getCustomerTree();
    var busType = `${this.selectFlowType}_before_handle`;
    console.log(busType, 'busType');
    this.$bus.$off(busType);
    this.$bus.$on(busType, (val, cb) => {
      var isCompany = this.selectFlowType == 'monthly_perf_companySum';
      var str = String(this.targetTime);
      this.updateData(undefined, () => {
        this.$bus.$emit('submitBeforeHandleOk', {
          status: 'success',
          name: `${str.substr(0, 4)}-${str.substr(4)}${isCompany ? '公司' : '部门'}月度绩效汇总-${this.company.name}`
        });
      });

      // if (this.isReInitiate) {
      //   this.updateData(undefined, () => { // val
      //     this.$bus.$emit('submitBeforeHandleOk', {
      //       status: 'success',
      //       name: `${str.substr(0, 4)}-${str.substr(4)}${isCompany ? '公司' : '部门'}月度绩效汇总-${this.$store.state.user.companyName}`
      //     });
      //   });
      // } else {
      //   this.updateData(undefined, () => {
      //     this.$bus.$emit('submitBeforeHandleOk', {
      //       status: 'success',
      //       name: `${str.substr(0, 4)}-${str.substr(4)}${isCompany ? '公司' : '部门'}月度绩效汇总-${this.$store.state.user.companyName}`
      //     });
      //   });
      // }

      // if (this.status == 'not_submitted') {
      //   this.postData();
      // } else {
      //   this.status = val;
      //   this.submitData('form').then(res => {
      //     const obj = {
      //       status: 'success'
      //     };
      //     this.$bus.$emit('submitBeforeHandleOk', obj);
      //   }).catch(err => {
      //     const obj = {
      //       status: 'fail'
      //     };
      //     this.$bus.$emit('submitBeforeHandleOk', obj);
      //   });
      // }
    });
  },
  computed: {},
  watch: {}

};

</script>
<style lang='scss' scoped>
::v-deep .el-dialog--center .el-dialog__body {
    padding: 25px 25px 30px !important;
}
.main_table{
  .editIcon_td{
    position: relative;
    .editIcon {
      color: #1989fa;
      font-size: 14px;
      cursor: pointer;
      position: absolute;
      top: 5px;
      right: 5px;
      &:hover {
        color: blue;
      }
    }
  }

}
.attack_table{
  background-color:#e8ecf3;
  margin-top:10px;
  text-align: center;
  border-top: 2px solid black;
  border-left: 2px solid black;
  border-right: 2px solid black;
  line-height: 35px;
  font-weight: 700;
  color: black;
}
.flow-log-container {
  display: none;
}
::v-deep  .el-date-editor input {
  font-weight: bold;
  padding-right: 0px;
  padding-left: 30px;
  text-align: center;
  font-size: medium;
}
.comment-print {
  display: none;
}
@media print {
  @page {
    // size: A4 landscape;
    // size: A3 landscape;
    // size: 297mm 420mm;
    // size: auto; //打印可以选择布局：横向，纵向
    // size: landscape;//横向
    size: portrait;//纵向
    // margin: 23.5mm; //默认边距
    // paper-type: custom;
    // custom-paper-source: OMB-A;
  }
  ::v-deep .flow-log-container {
    margin: 0 auto;
    overflow: initial;
    display: block;
    .flowWrap {
      margin-top: 0px !important;
      padding: 0px;
    }
  }
  .print{
    zoom: 0.7;
    ::v-deep input{
      border: none;
    }
    ::v-deep textarea{
      border: none;
      resize: none;
      font-size: 14px !important;
      color: rgba(0, 0, 0, 0.847);
    }
    ::v-deep .el-input__count{
      display: none;
    }
    ::v-deep .attachFiles .el-upload{
      display: none;
    }
    ::v-deep .attach-ul .el-icon-view{
      display: none;
    }
    .colorTip{
      display: none;
    }
    .attack_button{
      display: none;
    }
    .comment-print {
      display: block;
    }
  }
}
table{
  border: 0.5px solid #333333; // #cbcfd8
  // border-top: 0.5px solid #333333;
  // border-left: 0.5px solid #333333;
  margin-top:10px;
}
table td,
table th {
    border: 0.5px solid #333333;
    // border-bottom: 0.5px solid #333333;
    // border-right: 0.5px solid #333333;
    text-align: center;
    padding:5px;
    font-size: 13px;
    color: rgba(0, 0, 0, 0.847);
}
table .ellipsis{
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.sumFlow{
  .colorTip{
    margin-right: 100px;
    .square{
      display: inline-block;
      height: 20px;
      width: 40px;
      background-color: rgba(255, 242, 204, 1);
      vertical-align: middle;
      border-radius: 5px;
      // border: 1px solid gray;
      margin-right: 10px;
    }
    .text{
      vertical-align: middle;
    }
  }
  // .botton-group{
  //   text-align: center;
  //   padding: 5px;
  // }
  .botton-group {
  text-align: center;
  background: transparent;
  width: 100%;
  position: absolute;
  left: 0;
  bottom: 10px;
  z-index: 2000;
  //pointer-events: none;
  .footer-inner {
    background: #fff;
    padding: 25px 0;
    pointer-events: all;
    padding-top: 0;
  }
}
}
</style>
<style lang='scss'>
.monthPerfSum20240514 .el-cascader-menu__wrap {
    height: 100%;
    // .el-cascader-node:has([companytype='PLATFORM_COMPANY'])>.el-radio{
    //   // display:none;
    // }
}
</style>
