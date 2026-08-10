/*
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-11-21 16:41:11
 */
import Api from '@/api';
const mixin = {
  data(){
    return {
      // examineDialogVisible: true,
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
      btnVisible:false,
    }
  },
  methods:{
    // 查看流程详情(本身列表就是流程)
    async previewHandle(row, type){
      console.log('previewHandle-查看流程详情',row)
      // console.log('window.$vue',window.$vue)
      if (window.$vue.testInstance) {
        // console.log('window.$vue.testInstance',window.$vue.testInstance.$parent.$attrs.index)
        // 应对流程公共组件使用this.$fm.show打开多个弹窗实例无法打开的问题。先关闭上一个，再打开新的。
        if (window.$vue.testInstance.$parent.$attrs.index == 2) {
          window.$vue.testInstance.handleClose();
          window.$vue.testInstance = undefined;
        }
        
        setTimeout(() => {
          this.$fm2.show("flowDetail", {
            data: {
              id: row.flowInstanceId || row.id, // 流程实例id
            }
          });
        });
      } else {
        this.$fm2.show("flowDetail", {
          data: {
            id: row.flowInstanceId || row.id, // 流程实例id
          }
        });
      }
      
      // this.currentRowFlowData = row;
      // this.selectFlowType = row.auditWay;
      // this.isExamine = false;
      // this.isReInitiate = false;
      // this.flowId = row.flowProxyId;
      // this.formId = row.formProxyId;
      // this.flowNodeProxyId = row.currentNodeProxyId;
      // this.flowInstanceId = row.id;
      // this.jobTaskId = row.jobTaskId;
      // this.examineDialogVisible = true;
      // const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
      // this.businessId = find?.otherBizId || '';
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