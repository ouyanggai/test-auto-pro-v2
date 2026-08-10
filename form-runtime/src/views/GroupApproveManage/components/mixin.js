import Api from '@/api';
const mixin = {
  methods: {
    setTracking(){
      // flowInstanceId
      let tracking = this.outTracking
      let msg = tracking ? '取消跟踪此流程？' : '确认跟踪此流程？'
      let targetTrackingStatus = tracking ? false : true
      this.$confirm(msg,'提示').then(()=>{
        let data = {
          data:{
            id:this.flowInstanceId
          },
          tracking:targetTrackingStatus
        }
        this.$axios.post(Api.schedule.flowTracking,data,res=>{
          if(res.isSuccess){
            this.$message.success('操作成功')
            this.$emit('update:outTracking',targetTrackingStatus)
            // this.fetchData();
            this.$emit('updateList');
          }else{
            this.$message.error(res.message)
          }
        })
      }).catch(()=>{})
    },
    // 获取表单字段值
    getFormDataById(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id//: this.flowInstanceId
            },
            // excludeFieldNames: ['auto_audit_info_']
          },
          (res) => {
            if (res.isSuccess) {
              // console.log('res.data',res.data)
              resolve(res?.data?.data || {});
            }
          },
          '',
          {noErrMsg:false,noLoading:true}
        );
      });
    },
    clickRollBack () {
      const flowInstanceId = this.flowInstanceId; const jobTaskId = this.jobTaskId;
      this.$confirm('确定要继续吗？', '您将回退流程至上一审批节点', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.schedule.rollBackNode,
          {
            data: {
              id: flowInstanceId,
              jobTaskId: jobTaskId,
              withdrawDesc: this.approveForm.approveMessage
            }
          },
          (res) => {
            if (res.isSuccess) {
              this.$message.success('已回退到上一审批节点');
              this.nodeChooseVisible = false;
              this.$emit('postExamine');
            } else { }
          }
        );
      }).catch(() => { });
    },
    filterWithdraw (data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    fetchLogData () {
      console.log('获取流程日志')
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
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 已建任务和流程日志操作状态字符转换
    translateStatus (obj) {
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
      obj.status = obj.auditStatus
      obj.auditStatus = chnStatus;
    }
  }
};
export default mixin;
