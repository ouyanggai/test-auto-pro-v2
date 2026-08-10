import Api from '@/api';
const mixin = {
  methods: {
    generateTableData(data) {
      console.log(data,'88888')
      this.pagination.total = data.length
      var current = this.pagination.pages
      var size = this.pagination.size
      var start = (current - 1) * size
      var end = start + size
      return data.slice(start, end)
    },
    pageChange(current) {
      this.pagination.pages = current
      this.tableData = this.generateTableData(this.wholeData)
    },
    sizeChange(size) {
      this.pagination.pages = 1
      this.pagination.size = size
      this.tableData = this.generateTableData(this.wholeData)
    },
    formType() {
      return this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.typeList,
        {
          data: {
            name: '',
            useScope: 'invest'
          },
          pages: 1,
          size: 99999
        }
      );
    },
    //获取流程
    getFlow(ids) {
      const param = {
        data: {
          typeIds: ids,
          useScope: 'invest',
        },
        showMe: true, // 只能看到配置了自己为发起人的流程
        ignoreFormTemplateBizRelevanceData: true,
        platformCode: '999999', // 如果使用实施管理平台的流程需要加这个字段
        ignoreTemplateData: true,
        pagination: true,
        pages: 1,
        size: 9999
      };
      return this.$axios.post(
        Api.schedule.getFlowTemplateList,
        param
      );
    },
    // 获取流程实例id
    getInstanceId(id, type,taskStatus) {
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
    //打开弹窗
    clickReInitiate(row) {
      console.log(row,'row')
      this.selectFlowType = row.auditWay;

      // 这里的接口和已提的接口不一样字段也不一样---参考审批功能
      this.parallelNodeChooseList = [];
      this.manualChooseNodes = [];
      if (row.nextNodeType == 'parallel') {
        // 下一节点为并行,取出其中需要自选的节点
        row.nextAuditNodeList.map(x => {
          if (x.flowNodeAuditConfig?.auditType == 'run_node_choose') {
            // 取出其中的审批人自选节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          } else if (x.flowNodeAuditConfig?.auditType == 'department_supervisor' || x.flowNodeAuditConfig?.auditType == 'branched_passage_manager') {
            // 取出其中的主管审批和副总审批的类型节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          }
        });
      }
      if (row.auditPassLogicFlag && row.branchExecuteType == 'custom_choose' && row.nextAuditNodeList.length > 1) {
        // 下一节点为手动分支
        row.nextAuditNodeList.map((x, index) => {
          this.manualChooseNodes.push({
            nextNodeTemplateId: x.id,
            nodeName: x.nodeName,
            nodeType: x.type, // 为处理空节点
            branchName: '分支' + (index + 1),
            auditType: x.flowNodeAuditConfig.auditType
          });
        });
      }
      this.flowName = row.flowName;
      this.formExist = row.formExist;
      this.isExamine = false
      this.isReInitiate = true
      // 固定页面
      this.operaType = 'reEdit';
      this.actionType = 'edit';
      this.initiatorId = row.initiatorId;
      this.flowNodeProxyId = row.flowNodeProxyId;
      this.flowNodeType = row.flowNextNodeAuditType || 'run_node_choose';
      // this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId;
      if (row.flowInstanceBizRelevanceList.length == 1) {
        this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
      } else {
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.flowId = find.otherBizId;
      }
      this.flowProxyId = row.flowProxyId;
      this.formId = row.formProxyId;
      this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
      this.noFormFlowInstanceId = row.flowInstanceId;
      this.searchFlowType = row.auditWay;
      if (row.auditWay == 'annual_perf'|| row.auditWay == 'year_kpi_work_target') {
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
        if (find) { // 管理指标
          this.flowType = 'manageTarget';
        } else { // 工作指标
          this.flowType = 'workTarget';
        }
      }
      this.ExpensesClaimFormVisible = true;
    },
    previewHandle(row){
      this.selectFlowType = row.auditWay;
      this.formExist = row.formExist;
      this.operaType = 'check';
      this.actionType = 'preview';
      if (row.auditWay == 'annual_perf'|| row.auditWay == 'year_kpi_work_target') {
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
        if (find) { // 管理指标
          this.flowType = 'manageTarget';
        } else { // 工作指标
          this.flowType = 'workTarget';
        }
      }
      this.isExamine = false;
      this.isReInitiate =false
      console.log(1,row)
      if (row.flowInstanceBizRelevanceList.length == 1) {
        this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
      } else {
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.flowId = find.otherBizId;
      }
      console.log(2)
      this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
      this.searchFlowType = row.auditWay;
      this.flowProxyId = row.flowProxyId
      this.ExpensesClaimFormVisible = true;
    },
    deleteBiz(obj, auditWay) {
      const typeName = auditWay;
      const index = obj.findIndex(item => item.otherBiz == typeName);
      if (index > -1) {
        obj[index].otherBizId;
        const id = obj[index].otherBizId;
        if(typeName == 'annual_perf' || typeName == 'monthly_perf'){
          this.deletePerf(id).then(()=>{this.fetchData()})
        }
      }
    },
    deletePerf(id) {
      return this.$axios.post(
        Api.performance.deleteWorkTarget,
        { data: { id: id }});
    },
    handleDel(row,type) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.getInstanceId(row.id,type,'edit').then(res=>{
          if(res?.flowInstanceBizRelevanceList?.length){
            let flowInstanceId = res.flowInstanceId
            this.$axios.post(Api.approveManage.taskFlowDelete,{data:{},ids:[flowInstanceId]},result=>{
              if (result.isSuccess) {
                this.$message.success('删除成功');
                if (res.flowInstanceBizRelevanceList.length) this.deleteBiz(res.flowInstanceBizRelevanceList, res.auditWay);
              }else{
                this.$message.error(result.message);
              }
            })
          }else{
            this.deletePerf(row.id).then(()=>{this.fetchData()})
          }
      }
        );
      }).catch(() => { });
    },
    addTarget() {
      this.assessmentCycle = ''
      console.log(777777)
      // 获取目标责任书的流程id,多个的话取最新的,自己排序
      this.isExamine = false
      this.formType().then(res => {
        if (res.isSuccess) {
          const data = res.data;
          const find = data.filter(item => item.auditWay == 'annual_perf');
          if (find && find.length) {
            const ids = find.map(item => item.id);
            this.getFlow(ids).then(resp => {
              if (resp.isSuccess) {
                const list = resp.data;
                if (list.length) {
                  // 获取
                  if (list.length > 1) {
                    // this.flowList = list;
                    this.flowList = list.filter(item=>!item.flowName.includes('（年终）')&&!item.flowName.includes('（半年）'))
                    this.visible = true;
                  } else {
                    // const params = JSON.stringify(list[0]);
                    this.toFlowPage(list[0]);
                  }
                } else {
                  this.$message.error('暂无目标责任书流程,请先添加');
                }
              } else {
                this.$message.error('暂无目标责任书流程,请先添加');
              }
            });
          } else {
            this.$message.error('暂无目标责任书流程,请先添加');
          }
        } else {
          this.$message.error('暂无目标责任书流程,请先添加');
        }
      });
    },
    // 对目标责任书发起流程
    handlExame(row,type) {
      this.assessmentCycle = type
      this.assessmentCycleType = row.assessmentCycle
      this.formType().then(res => {
        if (res.isSuccess) {
          const data = res.data;
          let find
          if(type){
            find = data.filter(item => item.auditWay == 'year_kpi_work_target');
          }else{
            find = data.filter(item => item.auditWay == 'annual_perf');
          }
          console.log(find,'find+++')
          // const find = data.filter(item => item.auditWay == 'annual_perf');
          if (find && find.length) {
            const ids = find.map(item => item.id);
            this.getFlow(ids).then(resp => {
              if (resp.isSuccess) {
                const list = resp.data;
                if (list.length) {
                  // 获取
                  // if(list.length > 1){
                  if(type=='half_year'){
                    this.flowList = list.filter(item=>item.flowName.includes('（半年）'))
                  }else if(type=='year'){
                    this.flowList = list.filter(item=>item.flowName.includes('（年终）'))
                  }else{
                    console.log(888888888)
                    this.flowList = list.filter(item=>!item.flowName.includes('（年终）')&&!item.flowName.includes('（半年）'))
                  }
                  // this.flowList = list;
                  this.visible = true;
                  this.isExamine = true;
                  this.exameBizId = row.id;
                  // }else{
                  // let params = JSON.stringify(list[0])
                  // this.toFlowPage(params)
                  // }
                } else {
                  this.$message.error('暂无年度考核流程,请先添加');
                }
              } else {
                this.$message.error('暂无年度考核流程,请先添加');
              }
            });
          } else {
            this.$message.error('暂无年度考核流程,请先添加');
          }
        } else {
          this.$message.error('暂无年度考核流程,请先添加');
        }
      });
    },
  }
}
export default mixin
