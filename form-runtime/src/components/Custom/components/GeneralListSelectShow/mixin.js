/*
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2025-01-02 17:05:30
 */
import Api from '@/api';
const mixin = {
  data(){
    return {
      examineDialogVisible: false,
      checkViewFlowDetailVisible: false,
      flowId: '', // 绑定的业务id
      flowInstanceId: '', // 流程实例id
      formId: '',
      flowNodeProxyId: '',
      jobTaskId: '',
      isExamine: false,
      isReInitiate: false,
      selectFlowType:'',
      initiatorId: '', // 发起人id
      companyId: '',
      businessId: '',
      currentRowFlowData:{},
    }
  },
  methods:{
    // 查看流程详情(本身列表就是流程)
    async previewHandle(row, type) {
      console.log('previewHandle-查看流程详情', row);
      try {
        const flowDetail = await this.getFlowDetail(row.id);
        if (flowDetail && flowDetail.length) {
          this.currentRowFlowData = row;
          this.selectFlowType = row.auditWay;
          this.isExamine = false;
          this.isReInitiate = false;
          this.flowId = row.flowProxyId;
          this.formId = row.formProxyId;
          this.flowNodeProxyId = row.currentNodeProxyId;
          this.flowInstanceId = row.id;
          this.jobTaskId = row.jobTaskId;
          this.examineDialogVisible = true;
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.businessId = find?.otherBizId || '';
        } else {
          this.$message.warning('该流程不存在，无法查看流程详情');
          return;
        }
      } catch (error) {
        this.$message.warning('该流程不存在，无法查看流程详情');
      }
    },
    // 判断流程是否存在
    getFlowDetail(id) {
      const data = {
        useScope: 'invest',
        initiator: 'all',
        id: id
      };
      const api = Api.schedule.getFlowInstanceList;
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, size: 1, pagination: true, pages: 1 }).then(res => {
          console.log('getFlowDetail-查看流程详情', res);
          if (res.isSuccess) {
            const data = res?.data || [];
            resolve(data);
          } else {
            reject(new Error('流程不存在'));
          }
        });
      });
    },

    // 查看流程节点
    async handleCheckFlow() {
      let row = this.currentRowFlowData;
      this.selectFlowType = row.auditWay;
      this.flowId = row.flowProxyId;
      this.flowInstanceId = row.id;
      this.initiatorId = row.createrId;
      this.checkViewFlowDetailVisible = true;
    },
  }
}

export default mixin;
