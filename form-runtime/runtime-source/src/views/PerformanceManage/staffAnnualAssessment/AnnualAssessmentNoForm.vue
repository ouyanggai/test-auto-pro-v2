<template>
  <div class="annual-assessment-noform">
      <div class="title">{{ detail.year }}年 年终绩效考核</div>
      <table class="assessment-table">
        <tbody>
          <tr class="no-border">
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
          </tr>
          <tr>
            <th class="label">姓名</th>
            <td class="value" colspan="4">{{ detail.userName || '-' }}</td>
            <th class="label">公司</th>
            <td class="value" colspan="6">{{ detail.companyName || '-' }}</td>
          </tr>
          <tr>
            <th class="label">部门</th>
            <td class="value" colspan="5">{{ detail.deptName || detail.departmentName || '-' }}</td>
            <th class="label">年度</th>
            <td class="value" colspan="5">{{ detail.year || '-' }}</td>
          </tr>
          <tr>
            <td class="section" colspan="12">季度考核分数</td>
          </tr>
          <tr>
            <th class="label" colspan="3">一季度</th>
            <th class="label" colspan="3">二季度</th>
            <th class="label" colspan="3">三季度</th>
            <th class="label" colspan="3">四季度</th>
          </tr>
          <tr>
            <td class="value score-value" colspan="3">{{ displayScore(detail.firstQuarter) }}</td>
            <td class="value score-value" colspan="3">{{ displayScore(detail.secondQuarter) }}</td>
            <td class="value score-value" colspan="3">{{ displayScore(detail.thirdQuarter) }}</td>
            <td class="value score-value" colspan="3">{{ displayScore(detail.fourQuarter) }}</td>
          </tr>
          <tr>
            <td class="section" colspan="12">年终考核分数</td>
          </tr>
          <tr>
            <th class="label" colspan="6">年终述职得分</th>
            <th class="label" colspan="6">年终得分</th>
          </tr>
          <tr>
            <td class="value score-value" colspan="6">
              {{ displayScore(detail.reviewScore) }}
            </td>
            <td class="value score-value" colspan="6">
              {{ displayScore(detail.finalScore) }}
            </td>
          </tr>
          <tr>
            <th class="label" colspan="2">本人确认</th>
            <td class="value opinion" colspan="10" style="text-align: center; padding: 10px; height: auto;">
              <div v-for="(log, index) in filteredLogData" :key="index" style="white-space: pre-wrap; margin-bottom: 5px;">
                {{ log.executeDesc }}
                <span style="font-weight: bold;">【{{ getStatusText(log.auditStatus) }}】</span>
                <span style="margin-left: 20px;">{{ log.executorName }} {{ log.createDate }}</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- 审批人选择 -->
      <PersonSelectDialog
        :visible.sync="nodeChooseVisible"
        v-if="nodeChooseVisible"
        @getSelectPerson="getSelectPerson"
        :isProject="false"
        :flowInstanceId="reInitiateFlowInstanceId"
        :nextNodeProxyId="nextNodeProxyId"
      />
    </div>
</template>

<script>
import { localstorageGet } from '@/utils/auth';
import Api from '@/api';
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog.vue';

export default {
  name: 'StaffAnnualAssessmentNoForm',
  inject: ['submitFlowFinal', 'flowDialog'],
  components: { PersonSelectDialog },
  props: {
    bizId: { type: String, default: '' },
    flowProxyId: { type: String, default: '' },
    flowInstanceId: { type: String, default: '' },
    flowNodeProxyId: { type: String, default: '' },
    actionType: { type: String, default: '' },
    isExamine: { type: Boolean, default: false },
    logTableData: { type: Array, default: () => [] }
  },
  data() {
    return {
      rawData: {},
      detail: {
        userName: '',
        companyName: '',
        deptName: '',
        year: new Date().getFullYear().toString(),
        firstQuarter: '-',
        secondQuarter: '-',
        thirdQuarter: '-',
        fourQuarter: '-',
        reportScore: '-',
        finalScore: '-',
        opinion: ''
      },
      nodeChooseVisible: false,
      reInitiateFlowInstanceId: '',
      nextNodeProxyId: ''
    };
  },
  computed: {
    filteredLogData() {
      // 过滤掉第一条数据（发起人记录）
      if (!this.logTableData || this.logTableData.length <= 1) {
        return [];
      }
      return this.logTableData.slice(1);
    }
  },
  created() {
    const busType = 'staff_annual_assessment_before_handle';
    if (this.$bus && this.$bus.$off && this.$bus.$on) {
      this.$bus.$off(busType);
      this.$bus.$on(busType, (method) => {
        // 调用 postData 方法提交业务数据
        this.postData(method);
      });
    }

    // 监听审批意见更新事件
    if (this.$bus && this.$bus.$off && this.$bus.$on) {
      this.$bus.$off('updateOpinion');
      this.$bus.$on('updateOpinion', (opinion) => {
        // 更新 rawData 和 detail 中的审批意见
        if (this.rawData) {
          this.rawData.opinion = opinion;
        }
        if (this.detail) {
          this.detail.opinion = opinion;
        }
      });
    }
  },
  mounted() {
    // 如果有 bizId（已有流程），通过接口获取数据
    if (this.bizId) {
      this.fetchData();
    } else {
      // 如果没有 bizId（新建流程），尝试从 flowDialog 获取初始数据
      this.$nextTick(() => {
        if (this.flowDialog && this.flowDialog.tempFormData) {
            this.rawData = { ...this.flowDialog.tempFormData };
            this.detail = { ...this.flowDialog.tempFormData };
          }
      });
    }
  },
  methods: {
    fetchData() {
      if (!this.bizId) {
        return;
      }
      this.$axios.post(
        '/web/plan/api/annualPerformance/findById',
        { data: { id: this.bizId } },
        res => {
          if (res.isSuccess && res.data) {
            this.rawData = res.data;
            this.detail = res.data;
          } else {
            this.$message.error(res.message || '获取数据失败');
          }
        }
      );
    },
    getValues() {
      return { ...this.rawData };
    },
    // 设置审批意见
    setOpinion(opinion) {
      if (this.rawData) {
        this.rawData.opinion = opinion;
      }
      if (this.detail) {
        this.detail.opinion = opinion;
      }
    },
    displayScore(v) {
      return v === null || v === undefined ? '-' : v;
    },
    getStatusText(status) {
      const statusMap = {
        'pass': '同意',
        'no_pass': '驳回',
        'withdraw': '撤销',
        'retrieve': '取回',
        'transfer': '移交',
        'roll_back_the_previous_level': '回退上一节点'
      };
      return statusMap[status] || status || '同意';
    },
    // 提交业务数据
    postData(method, id, callback, opinion, needReSubmit) {
      if (needReSubmit) {
        const annualAssessmentId = this.rawData?.annualAssessmentId;
        if (!annualAssessmentId) {
           this.$message.error('未找到annualAssessmentId，无法重新发起');
           this.$parent.$parent.$parent.submitLoading = false;
           return;
        }
        this.executeReFlow(annualAssessmentId, opinion);
        return;
      }

      this.saveBusinessData(opinion).then(resData => {
         // 保存成功后
         if (callback) {
           callback(null, resData);
         } else if (this.bizId) {
            // ... existing logic ...
            const year = this.rawData.year || '';
            const userName = this.rawData.userName || '';
            const name = `${year}年年终绩效考核-${userName}`;
            this.$bus.$emit("submitBeforeHandleOk", {
              status: "success",
              id: resData,
              name: name
            });
         } else {
            if (method === "pass") {
               // ...
            } else {
               // Reuse saveParams logic? saveBusinessData constructs params internally.
               // processReturnData needs params and res.
               // We might need to adjust processReturnData or pass necessary info.
               // Actually processReturnData uses 'data' (saveParams) to get year/userName.
               // We can reconstruct it or return it from saveBusinessData.
               // For simplicity, let's keep saveParams construction inside saveBusinessData
               // and maybe pass a flag or handle it.
               // processReturnData mainly needs year and userName which are in this.rawData.
               this.processReturnData(this.rawData, { data: resData }, method, id);
            }
         }
      }).catch(err => {
         this.$parent.$parent.$parent.submitLoading = false;
         if (callback) callback(err, null);
      });
    },

    // 抽取保存业务数据逻辑
    saveBusinessData(opinion) {
      return new Promise((resolve, reject) => {
          // 检查 rawData 是否存在必要的数据
          if (!this.rawData || !this.rawData.year) {
            const error = '数据不完整，请刷新页面重试';
            this.$message.error(error);
            reject(error);
            return;
          }

          // 获取审批意见
          let approveMessage = opinion !== undefined ? opinion : (this.detail.opinion || this.rawData.opinion || '');

          const saveParams = {
            data: {
              userId: this.rawData.userId,
              year: this.rawData.year,
              firstQuarter: this.rawData.firstQuarter,
              secondQuarter: this.rawData.secondQuarter,
              thirdQuarter: this.rawData.thirdQuarter,
              fourQuarter: this.rawData.fourQuarter,
              customerCode: this.rawData.customerCode || localstorageGet('customerCode') || '54e991e06b9b4f55b1747ee71fd0d79a',
              companyName: this.rawData.companyName || '',
              deptName: this.rawData.deptName || this.rawData.departmentName || '',
              userName: this.rawData.userName || '',
              assessmentScore: this.formatScore(this.rawData.reportScore) || '0',
              finalScore: this.formatScore(this.rawData.finalScore) || '0',
              opinion: approveMessage
            }
          };

          if(this.bizId){
            saveParams.data.id = this.bizId;
          }

          this.$axios.post(
              '/web/plan/api/annualPerformance/save',
              saveParams,
              res => {
                if (res.isSuccess && res.data) {
                  resolve(res.data);
                } else {
                  const error = res.message || '保存数据失败';
                  this.$message.error(error);
                  reject(error);
                }
              }
          );
      });
    },

    async executeReFlow(annualAssessmentId, opinion) {
        const checkParams = {
          data: {
            useScope: 'invest',
            auditWayList: ['staff_annual_assessment'],
            statusList: ['await_sent', 'run', 'withdraw', 'termination', 'abandon', 'rejected', 'end'],
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'staff_annual_assessment',
                otherBizId: annualAssessmentId
              }
            ]
          },
          pagination: false
        };

        const existingFlows = await new Promise(resolve => {
          this.$axios.post(Api.schedule.getFlowInstanceList, checkParams, res => {
            if (res.isSuccess) resolve(res.data || []);
            else resolve([]);
          });
        });

        if (existingFlows.length > 0) {
          const flowInstanceId = existingFlows[0].id;
          this.reInitiateFlowInstanceId = flowInstanceId;

          // 1. 先检查是否需要选人 (不保存业务数据)
          const checkParam = {
             data: {
               id: flowInstanceId,
               checkPermissions: 'first'
             }
          };

          this.$axios.post(Api.schedule.saveFlowInstanceAgain, checkParam, async reRes => {
              if (!reRes.isSuccess && reRes.data && reRes.data.errorType === 'run_node_choose') {
                   // 需要选人
                   this.nextNodeProxyId = reRes.data.node.id;
                   this.nodeChooseVisible = true;
                   // 停止，等待用户选人
              } else {
                   // 不需要选人，保存业务数据并提交
                   try {
                       await this.saveBusinessData(opinion);
                       this.submitReFlow(flowInstanceId);
                   } catch (e) {
                       this.$parent.$parent.$parent.submitLoading = false;
                   }
              }
          });

        } else {
           this.$message.warning('未找到可重新发起的流程实例');
           this.$parent.$parent.$parent.submitLoading = false;
        }
    },

    // 提交重新发起流程
    submitReFlow(flowInstanceId, nextAuditorList) {
          const param = {
              data: { id: flowInstanceId },
              formDataMongoVo: {
                data: {}
              },
              nextAuditorList: nextAuditorList
          };

          this.$axios.post(Api.schedule.saveFlowInstanceAgain, param, reRes => {
              if (reRes.isSuccess) {
                this.$message.success('流程已重新发起');
                // 停止加载状态
                this.$parent.$parent.$parent.submitLoading = false;
                this.$bus.$emit("submitBeforeHandleOk", {
                  status: "success"
                });
                if (this.nodeChooseVisible) {
                    this.nodeChooseVisible = false;
                }
              } else {
                 // 理论上这里不应该再报 run_node_choose，因为前面已经检查过了
                 if (reRes.data && reRes.data.errorType === 'run_node_choose') {
                    this.nextNodeProxyId = reRes.data.node.id;
                    this.nodeChooseVisible = true;
                 } else {
                    this.$message.error('重新发起流程失败: ' + reRes.message);
                    this.$parent.$parent.$parent.submitLoading = false;
                 }
              }
            });
    },

    // 处理返回数据并提交流程
    processReturnData(data, res, method, id) {
      // 兼容 rawData 和 saveParams 结构
      const year = data.year || (data.data && data.data.year) || '';
      const userName = data.userName || (data.data && data.data.userName) || '';
      const name = `${year}年年终绩效考核-${userName}`;
      const bizId = res.data?.id || res.data;
      if (typeof this.submitFlowFinal === 'function') {
        try {
          console.log("提交流程")
          console.log('%c [ name ]-253', 'font-size:13px; background:pink; color:#bf2c9f;', name)
          this.submitFlowFinal(false, bizId, 'staff_annual_assessment', 0, name);

        } catch (error) {
          this.$message.error('提交流程失败：' + error.message);
        }
      } else {
        this.$message.error('提交流程失败，submitFlowFinal 方法未找到');
      }
    },

    // 格式化分数
    formatScore(score) {
      if (score === null || score === undefined || score === '') {
        return null;
      }
      const num = parseFloat(score);
      return isNaN(num) ? null : num.toFixed(2);
    },
    // 处理人员选择回调
    getSelectPerson(data) {
      // data.checkboxPersonGroup 是选中的人员列表
      // 参考 Flow.vue 的处理逻辑
      if (!data || !data.checkboxPersonGroup || data.checkboxPersonGroup.length === 0) {
        this.$message.warning('请至少选择一位审批人');
        return;
      }

      const nextAuditorList = [];
      data.checkboxPersonGroup.forEach(item => {
        nextAuditorList.push({
          name: item.name,
          bizId: item.id,
          auditDetailType: "personnel",
          nodeProxyId: item.nodeProxyId || this.nextNodeProxyId
        });
      });

      // 选完人后，先保存业务数据，再提交流程
      this.saveBusinessData().then(() => {
          this.submitReFlow(this.reInitiateFlowInstanceId, nextAuditorList);
      }).catch(err => {
          this.$parent.$parent.$parent.submitLoading = false;
      });
    }
  }
};
</script>

<style lang="scss" scoped>
.annual-assessment-noform {
  width: 70%;
  min-width: 800px;
  margin: 0 auto;
  color: #000;
  min-height: 900px;
  padding: 20px 0;
}
.title {
  font-size: 32px;
  font-weight: 800;
  text-align: center;
  margin: 20px 0 30px;
  color: #000;
  letter-spacing: 1px;
}
.assessment-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}
.assessment-table th,
.assessment-table td {
  border: 1px solid #000;
  padding: 12px 10px;
  text-align: center;
  font-size: 16px;
  height: 50px;
}
.assessment-table .label {
  width: 20%;
  color: #000;
  font-size: 18px;
}
.assessment-table .value {
  width: 30%;
  color: #000;
  font-size: 18px;
}
.assessment-table .score-value {
  font-size: 20px;
  font-family: Arial, sans-serif;
}
.assessment-table .section {
  background: #fff;
  padding: 15px;
  color: #000;
  font-size: 20px;
}
.assessment-table .sign {
  height: 200px;
  vertical-align: top;
  font-size: 18px;
}
.assessment-table .opinion {
  height: 200px;
  text-align: center;
  vertical-align: middle;
  padding: 15px;
}
.no-border td {
  height: 1px;
  padding: 0;
  line-height: 0;
  font-size: 0;
  border: 1px solid #000;
  border-top: none;
  visibility: hidden;
}
</style>
