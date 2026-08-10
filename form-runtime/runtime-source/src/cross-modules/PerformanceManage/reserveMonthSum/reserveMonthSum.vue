<template>
  <div class="sumFlow">
    <div
      class='print'
      ref="print"
    >
      <h3 style="text-align:center;">后备英才月度绩效汇总表</h3>
      <span>考核时间：</span>
      <el-date-picker
        v-model="targetTime"
        type="month"
        ref="datepicker"
        placeholder="选择日期"
        format="yyyy年M月"
        value-format="yyyyM"
        :clearable="false"
        :disabled="this.actionType == 'examine' || this.actionType == 'preview' || this.isReInitiate"
        style="width:300px;margin-right:30px;"
        @change="changeDate"
      ></el-date-picker>
      <div
        style="margin-top:10px"
        class="attachFiles"
      >
        <eleupload
          :showOnly="actionType != 'create' && actionType != 'edit'"
          ref="eleupload"
          :size="200"
          :attachFile="attachFile"
        ></eleupload>
      </div>
      <table
        border="1"
        bordercolor="black"
        width="100%"
        cellspacing="0"
        cellpadding="0"
      >
        <thead>
          <tr>
            <th v-if="actionType != 'preview'"></th>
            <th style="width: 60px;">序号</th>
            <th style="width: 120px;">考核时间</th>
            <th style="min-width: 130px;">轮岗公司</th>
            <th>轮岗部门</th>
            <th>轮岗人员</th>
            <th>最终得分</th>
            <th>奖励分值</th>
            <th>扣罚分值</th>
            <th>岗效比例</th>
            <th style="max-width:200px;">调薪情况</th>
            <th>考核结果</th>
            <th>签字确认</th>
            <th class="no-print">查看</th>
          </tr>
        </thead>
        <tbody ref="tbody">
          <tr
            v-for="(i,index) in kpiList"
            :key="i.id"
            :style="{backgroundColor: i.id || 'rgba(255, 242, 204, 0.5)'}"
          >
            <td v-if="actionType != 'preview'"><i
                class="el-icon-rank sortable-move"
                style="font-size:18px;cursor:move;color:#1989fa"
              ></i></td>
            <td>{{ index+1 }}</td>
            <td>{{`${String(i.targetTime).substr(0,4)}年${String(i.targetTime).substr(4)}月`}}</td>
            <td style="min-width: 130px;">{{ i.companyName }}</td>
            <td>{{ i.depName }}</td>
            <td>{{ i.userName }}</td>
            <td>{{ i.totalScore }}</td>
            <td>{{ i.extraPointsValue }}</td>
            <td>{{ i.deductPointsValue }}</td>
            <td>{{(i.totalKpi * 100).toFixed(2) + '%'}}</td>
            <td
              :title="findDesc(i.kpiDynamicItemList,'change_salary')"
              style="max-width:200px;"
            >
              {{ findDesc(i.kpiDynamicItemList,'change_salary') || (i.id ? '-' : '') }}
            </td>
            <td style="max-width: 250px;">{{ i.comment }}</td>
            <td>{{ i.kpiGroupStatus == 'pass' ?  i.userName : ''}}</td>
            <td class="no-print"> <el-button
                type="text"
                @click="checkDetail(i)"
              >详情</el-button> </td>
          </tr>
          <tr v-if="!kpiList.length">
            <td
              colspan="30"
              style="height:70px;"
            >暂无数据</td>
          </tr>
          <tr>
            <td colspan="2">备注</td>
            <!-- <td colspan="10"><el-input type="textarea" v-model="remark" :disabled="this.actionType == 'examine' || this.actionType == 'preview'"></el-input></td> -->
            <td colspan="12"><el-input
                type="textarea"
                :autosize="{ minRows: 5,maxRows: 5 }"
                show-word-limit
                maxlength="500"
                v-model="remark"
              ></el-input></td>
          </tr>
        </tbody>
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
      <el-button
        type="primary"
        icon='el-icon-view'
        v-if="actionType == 'create'"
        @click="$parent.handleCheckFlow()"
      >查看流程</el-button>
      <!-- <el-button type="primary" @click="print">打 印</el-button> -->
      <el-button
        @click="handleClose"
        v-if="actionType == 'create'"
      >取 消</el-button>
      <el-button
        type="primary"
        @click="savePerfSum"
        v-if="actionType == 'create'"
      >提 交</el-button>
      <el-button type="primary no-print" style="margin:0 auto" @click="exportTableData()" v-if="actionType != 'create'">导出</el-button>
      <!--  || actionType == 'edit' -->
    </div>
    <enterprise-examine-dialog
      v-if="examineDialogVisible"
      :btnVisible="false"
      :isExamine="false"
      :isReInitiate="false"
      :flowId="flowId"
      :formId="formId"
      :flowNodeProxyId="flowNodeProxyId2"
      :jobTaskId="jobTaskId"
      :flowInstanceId="flowInstanceId2"
      :selectFlowType="selectFlowType2"
      :visible.sync="examineDialogVisible"
      :businessId="businessId"
      :companyId="companyId"
    />
  </div>
</template>

<script>
import {
  localstorageGet, localstorageSet
} from '@/utils/auth';
import Api from '@/api';
import eleupload from '@/components/EleUpload';
import { Print as $print } from '@/utils/print.js';
import Sortable from 'sortablejs';
import enterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue';
export default {
  name: '',
  components: { eleupload, enterpriseExamineDialog },
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
  data() {
    return {
      postscriptList: [],
      logTableData: [],
      selectFlowType2: '',
      flowNodeProxyId2: '',
      flowInstanceId2: '',
      flowId: '',
      formId: '',
      businessId: '',
      jobTaskId: '',
      companyId: '',
      examineDialogVisible: false,
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
      }
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
        '/web/plan/api/kpiSummary/exportReserveKpiSummary',
        param,
        (res, originResponse) => {
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            // const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            link.download = '后备英才月度绩效汇总.xlsx';
            link.click();
            // document.body.removeChild(link);
            window.URL.revokeObjectURL(link.href);
            link.remove();
          }
        }, '', { responseType: 'blob' }
      );
    },
    setSortable() {
      // 获取表格row的父节点
      // const ele = this.$refs[dom].$el.querySelector('.el-table__body > tbody');

      var ele = this.$refs.tbody;
      console.log(ele, 'tbody-zxf');
      // 创建拖拽实例
      const that = this;
      this.dragObj = Sortable.create(ele, {
        animation: 150, // 动画
        handle: '.sortable-move', // 指定拖拽目标，点击此目标才可拖拽元素(此例中设置操作按钮拖拽)
        dragClass: 'dragClass', // 设置拖拽样式类名
        ghostClass: 'ghostClass', // 设置拖拽停靠样式类名
        chosenClass: 'chosenClass', // 设置选中样式类名
        onUpdate: function (event) {
          var newIndex = event.newIndex;
          var oldIndex = event.oldIndex;
          var $newItem = ele.children[newIndex];
          var $oldItem = ele.children[oldIndex];
          // 先还原sotable拖拽
          ele.removeChild($newItem);
          if (newIndex > oldIndex) {
            ele.insertBefore($newItem, $oldItem);
          } else {
            ele.insertBefore($newItem, $oldItem.nextSibling);
          }
          var item = that.kpiList.splice(oldIndex, 1);
          that.kpiList.splice(newIndex, 0, item[0]);
          const newArray = that.kpiList.slice(0);
          // 重新赋值来刷新视图 （这里我调的父组件的变量，你的可以换成当前组件的变量）
          that.kpiList = []; // 必须有此步骤，不然拖拽后回弹
          that.$nextTick(function () {
            that.kpiList = newArray; // 重新赋值，用新数据来刷新视图
          });
        }
      });
    },
    checkDetail(row) {
      this.$fm2.show("flowDetail", {
        data: {
          id: undefined, // 流程实例id
          flowInstanceBizRelevanceList: [
            {
              otherBiz: undefined, // 业务类型
              otherBizId: row.id // 业务id
            }
          ]
        }
      });
      // this.getInstanceId(row);
    },
    addAttach() {
      this.kpiList2.push(this.getEmptyKpi());
      const index = this.kpiList2.length - 1;
      this.$nextTick(() => {
        if (index > -1) {
          const attach = document.querySelector('.attach' + index);
          if (attach) attach.scrollIntoView({ behavior: 'smooth' });
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
    addPointRatio() {
      if (this.kpiList.length) {
        var addTotal = this.kpiList.reduce((prev, curr) => {
          return prev + ((curr?.extraPointsValue || curr?.rewardPonitsValue) ? 1 : 0);
        }, 0);
        return (addTotal / this.kpiList.length * 100).toFixed(2);
      }
      return '0.00';
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
          // company: {
          //   id: this.$store.state.user.companyId
          // },
          // department: {
          //   id: isCompany ? undefined : localstorageGet('userDepartmentId')
          // },
          assessmentCycle: 'month', // 考核范围，month月度绩效考核，年度考核暂时不考虑
          examineStatus: 'under_review', // 审核状态under_review审核中、finish已完成，页面状态以流程为准
          // applyScope: isCompany ? 'company' : 'department',
          remark: this.remark,
          targetTime: this.targetTime,
          kpiList: this.kpiList.map((item, index) => {
            return item.id ? ({ id: item.id, sort: index + 1 }) : false;
          }).filter(Boolean)
        }
      };
      this.$axios.post(
        '/web/plan/api/kpiSummary/saveReserveKpiSummary',
        params,
        (res) => {
          if (res.isSuccess) {
            var { data: { id } } = res;
            var str = String(this.targetTime);
            var name = `${str.substr(0, 4)}-${str.substr(4)}后备英才月度绩效汇总`;
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
        '/web/plan/api/kpiSummary/updateReserveKpiSummary',
        {
          data: {
            id,
            examineStatus: examineStatus || undefined,
            remark: this.remark,
            kpiList: this.kpiList.map((item, index) => {
              return item.id ? ({ id: item.id, sort: index + 1 }) : false;
            }).filter(Boolean)
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
    getInstanceId(row, otherBiz = 'monthly_perf_reserveTalent', taskStatus = 'view') {
      const data = {
        useScope: 'invest',
        // taskStatus:'waiting_send',
        // statusList:["await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end","draft"],//: 'waiting_send',
        initiator: 'all',
        // auditWayList: this.sFlowTypeList,
        flowInstanceBizRelevanceList: [{
          otherBiz,
          otherBizId: row.id
          // otherBiz: 'company',
          // otherBizId: '' // row.id
        }]
      };
      let api;
      if (taskStatus == 'edit') {
        data.taskStatus = 'waiting_send';
        api = Api.approveManage.getTaskList;
      } else {
        api = Api.schedule.getFlowInstanceList;
      }
      this.$axios.post(api, { data, size: 1, pagination: true, pages: 1 }).then(res => {
        if (res.isSuccess) {
          const data = res?.data || [];
          if (data.length) {
            var obj = data[0];
            console.log(obj, 'obj33');
            this.flowId = obj.flowProxyId;
            this.formId = obj.formProxyId;
            this.flowNodeProxyId2 = obj.flowNodeProxyId;
            // this.flowInstanceId2 = obj.flowInstanceId;
            this.flowInstanceId2 = obj.id;
            this.jobTaskId = obj.jobTaskId;
            this.selectFlowType2 = obj.auditWay;

            // const find = obj.flowInstanceBizRelevanceList[0];
            // this.businessId = find?.otherBizId || '';

            const find = obj.flowInstanceBizRelevanceList.find(item => item.otherBiz == obj.auditWay);
            this.businessId = find?.otherBizId || '';

            const company = obj.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
            this.companyId = company?.otherBizId || '';
            this.examineDialogVisible = true;
            this.$nextTick(() => {
              setTimeout(() => {
                var c = document.querySelector('body>.el-dialog__wrapper:last-of-type .examine-content>.right-side').childNodes;
                c[0].remove();
                c[0].remove();
              }, 2000);
            });
          } else {

          }
        }
      });
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
      // $print(this.$refs.print);
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
      // var isCompany = this.selectFlowType == 'monthly_perf_companySum';
      // var companyId = this.$store.state.user.companyId;
      // var departmentId = isCompany ? undefined : localstorageGet('userDepartmentId');
      // var applyScope = isCompany ? 'company' : 'department';
      this.getList2(date);
    },
    getReserveByid(id) {
      this.$axios.post(
        '/web/plan/api/kpiSummary/findReserveKpiSummaryById',
        { data: { id } },
        (res) => {
          if (res.isSuccess) {
            var { data: { remark, targetTime, kpiList } } = res;
            this.targetTime = `${targetTime}`;
            this.remark = remark;
            this.kpiList = kpiList || [];
            this.$nextTick(() => {
              this.setSortable();
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getList2(date) {
      const params = {
        data: {
          // company: { // 发起公司，必填
          //   id: companyId // '8fe922a5da21445a8a26aba74d0af5e1'
          // },
          // department: { // 发起部门，公司发起，可以不用填 '50d063d8a418491e9687ccfa7867f108'
          //   id: departmentId
          // },
          // applyScope: applyScope, // 发起范围，公司发起company，部门发起department
          manageType: 'work_and_manager_target', // 考核类型月度绩效为该值
          targetTime: date // 考核时间
        }
      };
      this.$axios.post(
        '/web/plan/api/reserveKpiGroup/getKpiGroupByTargetTime',
        params,
        (res) => {
          if (res.isSuccess) {
            this.kpiList = res.data || [];
            this.$nextTick(() => {
              this.setSortable();
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
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
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    kpiSumFindById(id) {
      this.$axios.post(
        Api.performance.findById,
        { data: { id } },
        (res) => {
          if (res.isSuccess) {
            var { data: { remark, targetTime, applyScope, company: { id: companyId }, department, supplements } } = res;
            this.targetTime = `${targetTime}`;
            this.remark = remark;
            console.log(department, 'department233');
            this.companyDeta = res.data.company;
            this.kpiList2 = supplements || [];
            if (this.actionType == 'create') {
              this.getList(this.targetTime, companyId, department?.id || undefined, applyScope);
            } else {
              this.kpiList = res.data.kpiList || [];
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
    }
  },
  mounted() {
    window.abb = this;
    if (!this.isReInitiate) {
      this.$refs.datepicker.focus();
    }
    console.log(this.bizId || null, 'bizId');
    console.log(this.flowProxyId, 'flowProxyId');
    console.log(this.actionType, 'actionType');
    console.log(this.selectFlowType, 'selectFlowType');
    console.log(this.isReInitiate, 'isReInitiate');
  },
  created() {
    // this.getDepartmentList();
    if (this.actionType == 'create') {
      // this.targetTime = new Date();
      // this.$nextTick(() => {
      //   var date = `${this.targetTime.getFullYear()}${this.targetTime.getMonth() + 1}`;
      //   this.targetTime = date;
      //   this.getListBefore(date);
      // });
    } else {
      var bizId = this.$route.query.id || this.bizId;
      this.getReserveByid(bizId);
      // this.kpiSumFindById(bizId);
      this.getAttachmentList(bizId);
    }
    // this.getCustomerTree();
    var busType = `${this.selectFlowType}_before_handle`;
    console.log(busType, 'busType');
    this.$bus.$off(busType);
    this.$bus.$on(busType, val => {
      console.log(val, 'status');
      console.log(this.isReInitiate, 'isReInitiate');
      if (this.isReInitiate) {
        this.updateData(undefined, () => { // val
          this.$bus.$emit('submitBeforeHandleOk', {
            status: 'success'
          });
        });
      } else {
        this.updateData(undefined, () => {
          this.$bus.$emit('submitBeforeHandleOk', {
            status: 'success'
          });
        });
      }
      return;
      if (this.status == 'not_submitted') {
        this.postData();
      } else {
        this.status = val;
        this.submitData('form').then(res => {
          const obj = {
            status: 'success'
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        }).catch(err => {
          const obj = {
            status: 'fail'
          };
          this.$bus.$emit('submitBeforeHandleOk', obj);
        });
      }
    });
  },
  computed: {},
  watch: {}

};

</script>
<style lang='scss' scoped>
// 拖拽
.dragClass {
  background: rgba(160, 207, 255, 0.5) !important;
}

// 停靠
.ghostClass {
  background: rgba(160, 207, 255, 0.5) !important;
}

// 选择
.chosenClass:hover > td {
  background: rgba(140, 197, 255, 0.5) !important;
}
.attack_table {
  background-color: #e8ecf3;
  margin-top: 10px;
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
@media print {
  @page {
    size: A4 landscape;
    // size: A3 landscape;
    // size: 297mm 420mm;
    // size: auto; //打印可以选择布局：横向，纵向
    // size: landscape;//横向
    // size: portrait;//纵向
    // margin: 23.5mm; //默认边距
    // paper-type: custom;
    // custom-paper-source: OMB-A;
  }
  .print {
    zoom: 0.7;
    .flow-log-container {
      display: block;
    }
    ::v-deep input {
      border: none;
    }
    ::v-deep textarea {
      border: none;
      resize: none;
      font-size: 14px !important;
      color: rgba(0, 0, 0, 0.847);
    }
    ::v-deep .el-input__count {
      display: none;
    }
    ::v-deep .attachFiles .el-upload {
      display: none;
    }
    ::v-deep .attach-ul .el-icon-view {
      display: none;
    }
    .colorTip {
      display: none;
    }
    .attack_button {
      display: none;
    }
  }
}
table {
  border: 0.5px solid #333333; // #cbcfd8
  // border-top: 0.5px solid #333333;
  // border-left: 0.5px solid #333333;
  margin-top: 10px;
}
table td,
table th {
  border: 0.5px solid #333333;
  // border-bottom: 0.5px solid #333333;
  // border-right: 0.5px solid #333333;
  text-align: center;
  padding: 5px;
  font-size: 13px;
  color: rgba(0, 0, 0, 0.847);
}
table .ellipsis {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.sumFlow {
  .colorTip {
    margin-right: 100px;
    .square {
      display: inline-block;
      height: 20px;
      width: 40px;
      background-color: rgba(255, 242, 204, 1);
      vertical-align: middle;
      border-radius: 5px;
      // border: 1px solid gray;
      margin-right: 10px;
    }
    .text {
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
