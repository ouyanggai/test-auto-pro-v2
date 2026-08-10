import Api from '@/api';
import {
  localstorageGet
} from '@/utils/auth';
import math from '@/utils/math.js'
const mixin = {
  created(){
    if(this.isGroupMember){
      this.form.companyId = localstorageGet('companyId')
    }
  },
  data() {
    return {
      mainRule: {
        companyId: [{ required: true, message: '请选择公司', trigger: 'change' }],
        groupDepartId: [{ required: true, message: '请选择部门', trigger: 'change' }],
        annual: [{ required: true, message: '请选择预算年度', trigger: 'change' }],
        money: [{ required: true, message: '请输入预算金额', trigger: 'blur' }, {
          pattern: /^(?!(0[0-9]{0,}$))[0-9]{1,}[.]{0,}[0-9]{0,}$/,
          message: '预算金额要大于0',
          trigger: 'blur'
        }]
      },
      subRule: {
        departId: [{ required: true, message: '请选择部门', trigger: 'change' }],
        budgetType: [{ required: true, message: '请输入预算类型', trigger: 'input' }]
      },
      // 流程弹出组件相关
      branchChooseVisible: false,
      manulNodeList: [],

      pralleNodeVisible: false,
      pralleNodeList: [],
      parlleNodePerson: {},
      canSubmit: true,
      treeData: [],
      isGroupMember:localstorageGet('companyId') == localstorageGet('topCompanyId') ? true:false, //是否是集团人员登录
      groupDepartOptions:[],
      // groupDepartId:''
    };
  },
  methods: {
    getGroupBudget(){
      if(!this.form.annual) return
      const query = {
        companyId: localstorageGet('companyId'),
        budgetTime: `${this.form.annual}-01-01 00:00:00`,
        stringList: [4,6]
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
        Api.annualBudget.budgetList,
        {
          data: query
        },
        res=>{
          resolve(res)
        }
        )
      })
    },
    async departChange(idx,val) {
      if(!this.form.annual){
        this.datas[idx].departId = ''
        this.datas[idx].departName = ''
        return this.$message.error('请先选择年度')
      }
      //如果是集团部门预算而且是选择公司固定预算，需要查询自动生成的数据
      if(val && this.isGroupMember){
        let find = this.departOptions.find(item=>item.name == '公司固定费用')
        if(find && find.id == val){
          let res = await this.getGroupBudget()
          if(res.isSuccess){
            let list = res.data?.dataList || []
            let findBudget = list.find(item=>item.projectId == val)
            if(findBudget){
              this.datas[idx].departName= findBudget.departmentName
              this.datas[idx].departId= findBudget.projectId
              this.datas[idx].budget = []
              let budgetDetailsVos = findBudget.budgetDetailsVos
              budgetDetailsVos.forEach(el=>{
                const tmp = {
                  // id: appendBudgetDetailsVos[j].id,
                  budgetId: el.budgetId,
                  budgetDetailsId: el.id,
                  budgetTypeId: el.budgetTypeId,
                  budgetType: el.budgetTypeVo.name,
                  templateId: el.budgetTypeVo.templateId,
                  relateProjId: el.departmentId,
                  budgetMoney: el.money || 0, // + appendMoneys,
                  appendMoney: 0,
                  isOrigin: true,
                  status: el.budgetTypeVo.status,
                  useMoney: el.useMoney || 0,
                  isOrigin: false,
                  remarks: el.remarks
                  // canDelete: false,
                  // disabled: true
                };
                this.datas[idx].budget.push(tmp)
              })
            }
          }
        }
      }else{
        // 清除归口
        if (idx !== undefined) {
          this.datas[idx].budget = [];
          this.addPlan(idx);
        }
      }
    },
    hasValue(departId,templateId){
      let find = this.treeData.find(item=>item.id == departId)
      if(find && find?.children){
        let children = find.children
        let someFind = children.some(item=>item.expenseBudgetTypeTmplId == templateId )
        if(someFind){
          return true
        }else{
          return false
        }
      }else{
        return false
      }
    },
    //集团人员登录后，切换
    groupDepartChange(val){
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos,departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo} = item;
        if(sysDepartmentVo.id == val || sysDepartmentVo.departmentName == '公司领导' || sysDepartmentVo.parentId == val){
          departOptions.push({
            id:sysDepartmentVo.id,
            name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
            hasSelect: false,
          })
        }
      })
      this.departOptions = departOptions
      if(this.isGroupMember){
        if (this.form.groupDepartId && this.form.annual) {
          this.getBuegetInfo(this.form.companyId, this.form.annual);
        }
      }else{
        if (this.form.companyId && this.form.annual) {
          this.getBuegetInfo(this.form.companyId, this.form.annual);
        }
      }
    },
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
    },
    calculateTotalBudget() {
      let total = 0;
      this.datas.forEach(item => {
        const budget = item.budget || [];
        let temp = budget.reduce((prev, cur) => {
          cur.budgetMoney = cur.budgetMoney || 0
          return math.add(prev , cur.budgetMoney || 0)
        }, 0);
        total = math.add(total,temp)
      });
      this.form.money = total;
    },
    // 获取当前预算模板
    getBudgetTypeOfGroup() {
      this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            const data = res.data || [];
            const find = data.find(item => item.companyVo.id == this.form.companyId);
            if (find) {
              this.centralizedApiVos = find.centralizedApiVos[0];
              this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
              this.generateTree();
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const treeData = [],departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo, budgetCentralizedVoList } = item;
        sysDepartmentVo.children = [];
        // this.$set(sysDepartmentVo,'children',[])
        if (budgetCentralizedVoList && budgetCentralizedVoList.length) {
          sysDepartmentVo.children = budgetCentralizedVoList;
        }
        treeData.push(sysDepartmentVo);
        departOptions.push({
          id:sysDepartmentVo.id,
          name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
          hasSelect: false,
          isSubDepart:item?.sysDepartmentVo?.parentId ? true:false,
          parentId:item?.sysDepartmentVo?.parentId || null
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item=>{
        let projectBudget = {
          id:item.projectVo.id,
          departmentName:item.projectVo.shortName || item.projectVo.name,
          companyId:this.activeName,
          isProject:true,
          children:[]
        }
        let projectBudgetCentralizedVoList = item?.projectBudgetCentralizedVoList || []
        projectBudget.children = projectBudgetCentralizedVoList
        treeData.push(projectBudget)
        departOptions.push({
          id:item.projectVo.id,
          name:item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject:true,
          isSubDepart:false,
          parentId:item?.sysDepartmentVo?.parentId || null
        })
      })
      this.treeData = treeData;
      this.departOptions = departOptions
      // return
      if(this.isGroupMember && this.actionType == 'create'){
        this.groupDepartOptions = this.departOptions.filter(item=>{
          return !item.isSubDepart
        })
        this.departOptions = []
        //编辑 审核
        if(this.form.groupDepartId){
          this.departOptions = departOptions.filter(item=>{
            return item.id == this.form.groupDepartId || item.name == '公司固定费用' || item.parentId == this.form.groupDepartId
          })
          this.departChange()
        }
      }
    },
    sumNums(values) {
      let val = 0;
      values.forEach(item => {
        const v = item || 0;
        val = math.add(val,v - 0);
        // val = val+(v - 0);
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
          let total = this.sumNums(values)
          total = this.numberCommas(total.toFixed(2))
          sums[index] = '￥' + (total)
        } else if (column.property === 'appendMoney') {
          const values = data.map(item => item[column.property]);
          appendTotal = this.sumNums(values);
          appendTotal = this.numberCommas(appendTotal)
          sums[index] = '￥' + appendTotal;
        } else {
          sums[index] = '';
        }
      });
      return sums;
    },
    planDelete(i, j,row,data) {
      console.log(row)
      console.log('data',data)
      console.log('departOptions',this.departOptions)
      // return
      //Api.annualBudget.budgetDetailsList
      let find = this.departOptions.find(item=>item.id == data.departId)
      this.$confirm('确认删除？', '提示').then(() => {
        if(row.budgetTypeId && find){
          let data = {
            budgetTypeId:row.budgetTypeId
          }
          if(find.isProject){
            data.projectId = data.departId
            data.type = 5
          }else{
            data.departmentId = data.departId
            data.type = 2
          }
          this.$axios.post(Api.annualBudget.budgetDetailsList,{data},res=>{
            if(res.isSuccess){
              let list = res?.data?.dataList || []
              if(list.length){
                return this.$message.error('当前归口不可删除')
              }else{
                this.datas[i].budget.splice(j, 1);
              }
            }else{
              this.datas[i].budget.splice(j, 1);
            }
          })
        }else{
          this.datas[i].budget.splice(j, 1);
        }
      }).catch(() => {});
      // this.departChange()
    },
    deleteDepartPlan(index) {
      if (this.datas[index].budget && this.datas[index].budget.length) {
        let hasUseMoney = false;
        for (const val of this.datas[index].budget) {
          if (val.useMoney && val.useMoney > 0) {
            hasUseMoney = true;
            break;
          }
        }
        if (hasUseMoney) {
          return this.$message.error('部门预算有已使用金额，不可删除');
        } else {
          this.$confirm('确认删除？', '提示').then(() => {
            this.datas.splice(index, 1);
            this.departChange();
          }).catch(() => {});
        }
      }
    },

    getDepartNameById(id) {
      const index = this.departOptions.findIndex(item => item.id == id);
      if (index > -1) {
        return this.departOptions[index].name;
      } else {
        return '';
      }
    },
    // 获取关联项目 TODO
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
            this.projectOptions = res.data;
          }
        }
      );
    },
    getCompanyById(id) {
      // console.log('this.originDepartData', this.originDepartData)
      const company = this.originDepartData.find(item => {
        return item.id == id;
      });
      return company;
    },
    // 切换公司，下属部门需要跟随切换,流程也需要切换
    async companyChange(val) {
      if (this.$options.name == 'AddAnnualBudgetComponent') {
        if (this.form.annual) {
          await this.getBuegetInfo(val, this.form.annual);
        }
        this.getDepartmentList(val);
        await this.getFlowType();
      } else if (this.$options.name == 'EditeBudget') {
        this.$confirm('切换公司，归口信息和预算详情将全部清空', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          // 清空详情数据
          this.datas = [];
          // this.toEditPage(list);
          this.getDepartmentList(val);
        });
      } else {
        this.getDepartmentList(val);
      }
    },
    async getParentCompanyList() { // 查询公司列表
      await this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id: localstorageGet('companyId') // 当前用的公司id
          }
        },
        res => {
          this.companyOption = res.data;
        }
      );
    },
    // getDepartmentListByKey(id) {
    //   const findCompany = this.originDepartData.find(item => item.id == id);
    //   const departOptions = [];
    //   var fn = (list) => {
    //     list.forEach(item => {
    //       if (item.type == 2) {
    //         if (item.name == '公司领导') item.name = '公司固定费用';
    //         departOptions.push({
    //           hasSelect: false,
    //           id: item.id,
    //           name: item.name
    //         });
    //         // if(item.childrenList && item.childrenList.length)fn(item.childrenList)
    //       }
    //     });
    //   };
    //   if (findCompany) {
    //     fn(findCompany.childrenList);
    //   }
    //   this.departOptions = departOptions;
    //   this.departChange();
    // },
    // 获取公司下的部门列表
    async getDepartmentList(id) {
      await this.$axios.post(
        Api.budgetManage.getCompanyDeptVoByCompanyId,
        {
          data: {
            id// : this.detailRow.companyId
          }
        },
        res => {
          if (res.isSuccess) {
            let list = [];
            if (res.data && res.data.departmentVos) {
              list = res.data.departmentVos.map(item => {
                if (item.departmentName == '公司领导') item.departmentName = '公司固定费用';
                return {
                  hasSelect: false,
                  id: item.id,
                  name: item.departmentName
                };
              });
            }
            let index = -1;
            list.forEach((item, i) => {
              if (item.name == '公司固定费用') index = i;
            });
            if (index > -1) {
              list.unshift(list.splice(index, 1)[0]);
            }
            this.departOptions = list;
            this.departChange();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    cancel() {
      if (this.type == 'detail') {
        this.$router.go(-1);
      } else {
        this.$confirm('确认取消?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$router.go(-1);
        });
      }
    },
    // 根据业务id获取文件
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
          data: {
            relationId: id
          }
        }).then(res => {
        if (res.isSuccess) {
          const list = res.data;
          const attachFile = list.map(item => {
            return item;
          });
          this.attachFile = attachFile;
          this.form.enclosure = attachFile[0]?.id || '';
        }
      });
    },
    async getFlowType() {
      const data = {
        typeId: '',
        flowName: '',
        nextNodeName: '',
        flowNodeType: '',
        nextNodeProxyId: '',
        flowStatus: 'enable',
        auditWay: this.selectFlowType,
        useScope: 'invest'
      };
      await this.$axios.post(
        Api.schedule.getFlowTemplateList,
        {
          data,
          formTemplateBizRelevanceList: [
            // {
            //   otherBiz: 'customerCode',
            //   otherBizId: this.$store.state.user.customerCode
            // },
            {
              otherBiz: 'company',
              otherBizId: this.form.companyId
            }
          ],
          platformCode: '999999',
          ignoreTemplateData: true,
          pagination: true,
          pages: 1,
          size: 9999
        },
        res => {
          if (res.isSuccess) {
            if (!res.data.length) {
              this.flowId = null;
              this.$message.error('暂无流程，可保存为草稿');
            } else {
              this.flowId = res.data[0].id; // 流程id
            }
          }
        }
      );
    },
    getSelectPerson(data) {
      if (this.isPrall) { // 并行选人员
        const tmp = data.checkboxPersonGroup.map(item => {
          return {
            name: item.name,
            id: item.id,
            nodeProxyId: this.currentParallNodeId
          };
        });
        this.$set(this.parlleNodePerson, this.currentParallNodeId, tmp);
      } else {
        this.checkboxPersonGroup = data.checkboxPersonGroup.map(item => {
          const obj = {
            name: item.name,
            id: item.id
          };
          if (this.needChooseSpecialNode) {
            obj.nextNodeTemplateId = this.nextNodeTemplateId;
          }
          return obj;
        });
        if (!this.needChooseSpecialNode) {
          this.canSubmit = true;
          this.submit(1, true); // 直接提交，不需要获取流程数据
        }
      }
    },
    // 判断是否审批自选
    checkIsOptional() {
      let flag = false;
      if (this.flowNodeType == 'run_node_choose') { // 是自选节点
        if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
          this.nodeChooseVisible = true;
          flag = true;
        }
      }
      return flag;
    },
    bindFileById(relationId, fileId) {
      const data = {
        relationId,
        fileId
      };
      return this.$axios.post(
        Api.schedule.saveAttachment,
        { data }
      );
    },
    submitFinal(id, total) {
      const param = {
        data: {
          flowProxyId: this.flowId,
          flowInstanceBizRelevanceList: [
            {
              otherBiz: this.selectFlowType,
              otherBizId: id // 保存后返回的id
            }
          ]
        },
        formDataMongoVo: {
          data: {
            money: total,
            initiatorRange: this.$store.state.user.userId
          }
        }
      };
      if (this.checkboxPersonGroup.length) {
        const nextAuditorList = [];
        this.checkboxPersonGroup.map(item => {
          nextAuditorList.push({
            bizId: item.id
          });
        });
        param.nextAuditorList = nextAuditorList;
      }
      if (this.isPrall) {
        const nextAuditorList = [];
        Object.keys(this.parlleNodePerson).forEach(it => {
          this.parlleNodePerson[it].forEach(item => {
            nextAuditorList.push({
              bizId: item.id,
              nodeProxyId: item.nodeProxyId
            });
          });
        });
        param.nextAuditorList = nextAuditorList;
      }

      if (this.needChooseSpecialNode) {
        param.fixedExecuteNodeId = this.branchChoose.nextNodeTemplateId;
      }

      return this.$axios.post(
        Api.schedule.saveFlowInstance,
        param
      );
    },
    async getFlowFindById() {
      await this.$axios.post(
        Api.schedule.flowTemplateFindById,
        {
          data: {
            id: this.flowId
          }
        },
        res => {
          if (res.isSuccess) {
            const data = res.data;
            this.needChooseSpecialNode = false;
            this.isPrall = false;
            if (data.flowNodeTemplate && data.flowNodeTemplate.childFlowNodeTemplate) {
              if (data.flowNodeTemplate.childFlowNodeTemplate.branchExecuteType == 'custom_choose') { // 手动分支
                this.needChooseSpecialNode = true;
                const childFlowNodeTemplate = data.flowNodeTemplate.childFlowNodeTemplate;
                this.manulNodeList = childFlowNodeTemplate.conditionNodes.map(item => {
                  return {
                    nextNodeTemplateId: item.nextNodeTemplateId,
                    nodeName: item?.childFlowNodeTemplate?.nodeName,
                    nodeType: item?.childFlowNodeTemplate?.type, // 为处理空节点
                    branchName: item.name,
                    auditType: item?.childFlowNodeTemplate?.flowNodeAuditConfig?.auditType
                  };
                });
                this.branchChooseVisible = true;
                this.canSubmit = false;
              } else {
                if (data.flowNodeTemplate.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') { // 选择人员
                  this.checkboxPersonGroup = [];
                  this.nodeChooseVisible = true;
                  this.canSubmit = false;
                } else {
                  if (data.flowNodeTemplate.childFlowNodeTemplate.type == 'parallel') { // 下一个并行节点，需要判断是否有自选的
                    const parallelNodes = data.flowNodeTemplate.childFlowNodeTemplate.parallelNodes;
                    const pralleNodeList = [];
                    parallelNodes.forEach(item => {
                      if (item.childFlowNodeTemplate && item.childFlowNodeTemplate.flowNodeAuditConfig && item.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') {
                        pralleNodeList.push({
                          nodeName: item.childFlowNodeTemplate.nodeName,
                          // nodeId: item.nodeTemplateId
                          nodeId: item.nextNodeTemplateId
                        });
                      }
                    });
                    if (pralleNodeList.length) { // 并行节点需要选人
                      this.canSubmit = false;
                      this.isPrall = true;
                      this.pralleNodeList = pralleNodeList;
                      this.pralleNodeVisible = Boolean(pralleNodeList.length);
                    } else { // 并行不需要选人，直接走掉
                      this.canSubmit = true;
                    }
                  } else {
                    this.canSubmit = true;
                  }
                }
              }
            }
          }
        }
      );
    },
    // failSubmitFlow(response, res) {
    //   if (response.data) {
    //     this.flowNodeType = 'run_node_choose';
    //     if (response.data.nodeName && response.data.id) {
    //       this.nextNodeProxyId = response.data.id;
    //       this.nextNodeName = response.data.nodeName;
    //     } else {
    //       this.noNeedSave = true //不需要再次提交数据，选择审批人后直接提交流程即可
    //     }
    //     this.nodeChooseVisible = true;
    //   } else {
    //     this.checkboxPersonGroup = [];
    //     //如果提交审核失败，调用更新接口，把预算状态改成草稿
    //     this.$message.error(response.message + '，预算将转成草稿状态，你可以再次提交审核');
    //     this.updateStatusWhenFlowFail(res.data)
    //   }
    // },
    failSubmitFlow(response, res) {
      // 如果提交审核失败，调用更新接口，把预算状态改成草稿
      this.$message.error(response.message + '，预算将转成草稿状态，你可以再次提交审核');
      this.updateStatusWhenFlowFail(res.data || res);
    },
    showSelectPerson(nextNodeTemplateId) {
      this.nextNodeTemplateId = nextNodeTemplateId;
      this.nodeChooseVisible = true;
    },
    saveBranchChooseNode(data) {
      if (data.auditType == 'run_node_choose') {
        if (!this.checkboxPersonGroup.length) {
          return this.$message.warning('该分支需选择审批人，请先选择');
        }
      }
      this.branchChoose = data;
      this.branchChooseVisible = false;
      this.canSubmit = true;
      this.submit('1', true);
    },
    clearCheckboxPersonGroup() {
      this.checkboxPersonGroup = [];
    },
    parlleChoosePerson(nodeId) {
      this.currentParallNodeId = nodeId;
      this.nodeChooseVisible = true;
    },
    parlleSubmit() {
      let hasChooseAll = true;
      this.pralleNodeList.forEach(item => {
        const nodeId = item.nodeId;
        if (!this.parlleNodePerson[nodeId]) {
          hasChooseAll = false;
        } else {
          if (this.parlleNodePerson[nodeId].length <= 0) {
            hasChooseAll = false;
          }
        }
      });
      if (!hasChooseAll) {
        return this.$message.warning('每个节点均需要选择审批人');
      }
      this.pralleNodeVisible = false;
      this.canSubmit = true;
      this.submit('1', true);
    },
    isProjectBudget(id){
      let isProject = false
      let find = this.departOptions.find(item=>item.id == id)
      if(find && find.isProject)isProject = true
      return isProject
    },
    numberCommas(x){
      if(x){
        var res = x.toString().replace(/\d+/, function (n) {
          return n.replace(/(\d)(?=(\d{3})+$)/g, function ($1) {
            return $1 + ",";
          });
        });
        return res;
      }else{
        return x
      }
    },
    //金额转成万元提交
    transeMoney(data){
      var divide = 10000
      data.money = math.divide(data.money,divide)
      let budgetDetailsVos = data?.budgetDetailsVos || []
      budgetDetailsVos.forEach(el=>{
        el.money = math.divide(el.money,divide)
      })
      let costBudgetVoList = data?.costBudgetVoList || []
      costBudgetVoList.forEach(el=>{
        el.money = math.divide(el.money,divide)
        let budgetDetailsVos = el?.budgetDetailsVos
        budgetDetailsVos.forEach(item=>{
          item.money = math.divide(item.money,divide)
        })
      })
    },
  }
};
export default mixin;
