<!-- 无表单流程统一发起组件 -->
<template>
  <div>
    <!-- 审批人选择 -->
    <PersonSelectDialog :visible.sync="nodeChooseVisible" v-if="nodeChooseVisible" @getSelectPerson="getSelectPerson" :isProject="true" />

    <!-- 并行审批人自选 -->
    <el-dialog v-if="parallelChooseVisible" width="400px" title="并行节点自选" class="nodePerson"
      :visible="parallelChooseVisible" :before-close="handleCloseParallelChoose" :close-on-click-modal="false"
      append-to-body>
      <div v-for="chooseNode in parallelChooseNodes" :key="chooseNode.nextNodeTemplateId">
        <span style="line-height: 28px;">{{ chooseNode.nodeName }} <span>：</span> </span>
        <span v-for="audit in chooseNode.nodeAuditList" :key="audit.id"> {{ audit.name }} </span>
        <el-button type="text" size="mini" @click="chooseParallelNode(chooseNode)">
          {{ chooseNode.nodeAuditList.length ? '重新选择' : '选择人员' }}</el-button>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleSaveParallelChooseNode">提 交</el-button>
      </span>
    </el-dialog>

    <!-- 手动分支选择 -->
    <el-dialog v-if="branchChooseVisible" width="500px" title="选择流程分支" class="nodePerson" :visible="branchChooseVisible"
      :before-close="handleCloseBranchChoose" :close-on-click-modal="false" append-to-body>
      <el-radio-group v-model="chooseBranchNode" @change="handleChangeChooseBranch">
        <el-radio v-for="(branch, index) in manualChooseNodes" :key="index" :label="branch" class="radio-choose-item">
          <span>{{ branch.branchName }}-{{ branch.nodeName }}<span v-if="branch.auditType == 'run_node_choose'">：</span></span>
          <span v-if="branch.auditType == 'run_node_choose'">
            <span v-for="(audit, index) in chooseBranchNodeList" :key="index"> {{ audit.name }} </span>
            <el-button type="primary" :disabled="branch.nextNodeTemplateId != chooseBranchNode.nextNodeTemplateId"
              @click="handleChooseBranchNode">选择审批人</el-button>
          </span>
          <span v-if="branch.auditType == 'initiator'">由发起人审批</span>
          <span v-if="branch.auditType == 'assign'">由流程配置人员审批</span>
          <span v-if="branch.nodeType == 'empty'">空节点</span>
        </el-radio>
      </el-radio-group>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleSaveBranchChooseNode">提 交</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api'
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog.vue';
export default {
  name:'Flow',
  components: {PersonSelectDialog},
  props: ['flowId','flowNodeType','nextNodeProxyId','flowInstanceId','formId'],//flowId---->流程id ,flowNodeType-->下一节点的类型
  data() {
    return {
      nodeChooseVisible:false,

      parallelChooseNodes: [], // 并行节点发起后选择审批人自选
      parallelNextNodeTemplateId: '',

      //手动选择分支
      chooseBranchNode: {},
      chooseBranchNodeList: [],

      parallelChooseVisible:false,

      branchChooseVisible:false,
      checkboxPersonGroup:[], //选择的人员

      nextAuditorList:[],
      isReInitiate:false,
      isNoFlow:true,
      isDraft:false

    };
  },
  created() {
  },
  mounted() {},
  watch: {},
  computed: {},
  provide() {
    return { jusgeCustomChoose:()=>{}};
  },
  methods: {
    checkFlowPermission() {
      const param = {
        data: {
          flowProxyId: this.flowId,
          checkPermissions: 'first'
        }
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.saveFlowInstance,
          param,
          (res) => {
            if (res.isSuccess) {
              // this.jusgeCustomChoose();
              resolve(true)
            } else {
              resolve(false)
            }
          }
        );
      })
    },
    // 判断无表单流程的下一节点是否并行或手动
    async getFlowFindById() {
      await this.$axios.post(
        Api.schedule.flowTemplateFindById,
        {
          data: {
            id: this.flowId
          }
        },
        (res) => {
          if (res.isSuccess) {
            if (res.data.flowNodeTemplate.childFlowNodeTemplate.type == 'parallel') {
              // 处理发起人后并行节点审批人自选
              const parallelChooseNodes = [];
              let hasChoose = false;
              res.data.flowNodeTemplate.childFlowNodeTemplate.parallelNodes.forEach(parallelNode => {
                if (parallelNode.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') {
                  hasChoose = true;
                  parallelChooseNodes.push(
                    {
                      nodeName: parallelNode.childFlowNodeTemplate.nodeName,
                      nextNodeTemplateId: parallelNode.nextNodeTemplateId,
                      nodeAuditList: []
                    }
                  );
                }
              });
              if (hasChoose) {
                this.parallelChooseNodes = parallelChooseNodes
                // this.$emit('update:parallelChooseNodes', parallelChooseNodes);
              }
            }
            if (res.data.flowNodeTemplate.childFlowNodeTemplate.branchExecuteType == 'custom_choose') {
              // 处理发起人后的手动分支选择
              const manualChooseNodes = [];
              res.data.flowNodeTemplate.childFlowNodeTemplate.conditionNodes.forEach(branch => {
                manualChooseNodes.push(
                  {
                    nextNodeTemplateId: branch.nextNodeTemplateId,
                    nodeName: branch.childFlowNodeTemplate.nodeName,
                    nodeType: branch.childFlowNodeTemplate.type, // 为处理手动分支
                    branchName: branch.name,
                    auditType: branch.childFlowNodeTemplate.flowNodeAuditConfig.auditType
                  }
                );
              });
              this.manualChooseNodes = manualChooseNodes
              // this.$emit('update:manualChooseNodes', manualChooseNodes);
            }
          }
        }
      );
    },
    //isNoFlow true是无表单  false有表单
    reSubmit(isDraft,isNoFlow,bizId,componentName){
      this.isReInitiate = true
      this.beforeHandle(isDraft,isNoFlow,bizId,componentName)
    },
    async beforeHandle(isDraft,isNoFlow,bizId,componentName){
      // console.log(isDraft,isNoFlow,bizId,componentName)
      // console.log('this.flowNodeType',this.flowNodeType)
      // return
      this.isDraft = isDraft
      this.isNoFlow = isNoFlow || false
      this.bizId = bizId
      this.componentName = componentName
      if(this.isDraft){
        this.submitFinal(true);
      }else{
        if (this.flowNodeType == 'run_node_choose') { // 自选节点
          if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
            this.nodeChooseVisible = true;
            return
          }
          this.submitFinal(true);
        }else{ //判断其他节点
          await this.getFlowFindById()
          if (this.parallelChooseNodes && this.parallelChooseNodes.length) {
          // 并行审批人自选
            this.parallelChooseVisible = true;
            return false;
          } else if (this.manualChooseNodes && this.manualChooseNodes.length) {
            this.branchChooseVisible = true;
            // 手动分支选择分支
          }else{
            this.submitFinal(true);
          }
        }
      }
    },
    getSelectPerson(data) {
      if (this.parallelNextNodeTemplateId || (!this.parallelNextNodeTemplateId && this.parallelChooseNodes && this.parallelChooseNodes.length)) {
        // 并行分支-自选-有表单+无表单
        if (data.checkboxPersonGroup && data.checkboxPersonGroup.length > 0) {
          this.parallelChooseNodes.forEach(item => {
            if (item.nextNodeTemplateId == this.parallelNextNodeTemplateId) {
              item.nodeAuditList = JSON.parse(JSON.stringify(data.checkboxPersonGroup));
            }
          });
        } else {
          this.$message.warning('至少选择一位审批人');
          return false;
        }
        this.parallelNextNodeTemplateId = '';
        this.checkboxRersonGroup = [];
      } else if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        // 手动分支-自选
        if (data.checkboxPersonGroup && data.checkboxPersonGroup.length > 0) {
          this.chooseBranchNodeList = JSON.parse(JSON.stringify(data.checkboxPersonGroup));
          data.checkboxPersonGroup.forEach(item => {
            this.nextAuditorList.push(
              {
                bizId: item.id,
                nodeProxyId: this.chooseBranchNode.nextNodeTemplateId
              }
            );
          });
          this.nodeChooseVisible = false;
          // this.submitFinal(true);
        } else {
          this.$message.warning('至少选择一位审批人');
          return false;
        }
      } else {
        // 普通审批人自选
        this.checkboxPersonGroup = data.checkboxPersonGroup.map(x => x);
        this.nextAuditorList = [];
        this.checkboxPersonGroup.map(item => {
          this.nextAuditorList.push({
            bizId: item.id
          });
        });
        if(!this.checkboxPersonGroup.length){
          this.$message.warning('至少选择一位审批人');
          return
        }
        this.submitFinal(true);
      }
    },
    handleCloseParallelChoose() {
      this.parallelChooseVisible = false;
    },
    // 并行节点自选
    chooseParallelNode(node) {
      this.parallelNextNodeTemplateId = node.nextNodeTemplateId;
      this.nodeChooseVisible = true;
    },
    // 发起人-并行节点自选审批人保存后提交
    async handleSaveParallelChooseNode() {
      this.nextAuditorList = [];
      let superVisorId = '';
      let managerId = '';
      const isHasSuperVisor = this.parallelChooseNodes.some(node => node.auditType == 'department_supervisor');
      const isHasViceManager = this.parallelChooseNodes.some(node => node.auditType == 'branched_passage_manager');
      if (isHasSuperVisor) {
        superVisorId = await this.getSuperVisorOrLeaderId('department_supervisor');
      }
      if (isHasViceManager) {
        managerId = await this.getSuperVisorOrLeaderId('branched_passage_manager');
      }
      let hasNoChoose = false;// 是否有未选择审批人的节点
      this.parallelChooseNodes.forEach(item => {
        if (item.auditType == 'run_node_choose') {
          if (!item.nodeAuditList.length) {
            hasNoChoose = true;
          }
          item.nodeAuditList.forEach(auditNode => {
            this.nextAuditorList.push(
              {
                bizId: auditNode.id,
                nodeProxyId: item.nextNodeTemplateId
              }
            );
          });
        } else if (item.auditType == 'department_supervisor' || item.auditType == 'branched_passage_manager') {
          // 为并行中的主管和副总添加审批人节点参数传到提交
          this.nextAuditorList.push(
            {
              bizId: item.auditType == 'department_supervisor' ? superVisorId : managerId,
              nodeProxyId: item.nextNodeTemplateId
            }
          );
        } else {
          if (!item.nodeAuditList.length) {
            hasNoChoose = true;
          }
        }
      });
      if (hasNoChoose) {
        this.$message.warning('当前并行分支下有节点审批人未指定');
        this.parallelChooseVisible = true;
        return false;
      } else {
        this.parallelChooseVisible = false;
        // 无表单流程
        this.submitFinal(true);
      }
    },
    handleCloseBranchChoose() {
      this.chooseBranchNode = {};
      this.branchChooseVisible = false;
    },
    handleChangeChooseBranch(branch) {
      if (branch.auditType != 'run_node_choose') {
        this.chooseBranchNodeList = [];
      }
    },
    // 手动选择分支
    handleChooseBranchNode() {
      this.nodeChooseVisible = true;
    },
    // 发起人-手动选择分支-自选审批人后保存提交
    handleSaveBranchChooseNode() {
      if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        if (this.chooseBranchNode.auditType == 'run_node_choose') {
          // 自选节点
          if (!this.nextAuditorList.length) {
            this.$message.warning('该分支需选择审批人，请先选择');
          } else {
            // if (this.selectFlowType == 'enterprise') {
            //   // 表单流程
            //   this.enterpriseHandleSubmit(true);
            // } else {
            this.submitFinal(true);
            // }
          }
        } else if (this.chooseBranchNode.auditType == 'department_supervisor' || this.chooseBranchNode.auditType == 'branched_passage_manager') {
        // 节点-发起人部门主管或者分管副总
          this.getSuperVisorOrLeader(this.chooseBranchNode.auditType, this.chooseBranchNode.nextNodeTemplateId);
        } else {
          this.submitFinal(false);
        }
        this.nodeChooseVisible = false;
      } else {
        this.$message.warning('请选择流程分支');
      }
    },
    // 最终的提起流程审批接口--无表单流程
    submitFinal(customChooseFlag, id) {
      if(!this.isNoFlow){ //有表单流程再次提交
        this.submitFinalForFlow(true)
        return
      }
      const param = {
        data: {
          flowProxyId: this.flowId,
          flowInstanceBizRelevanceList: [
            {
              otherBiz: 'project',
              otherBizId: this.$store.state.user.projectId,
            },
            {
              otherBiz: this.componentName,//this.selectFlowType,
              otherBizId: this.bizId,// 保存后返回的id
            }
          ],
        },
        formDataMongoVo: {
          data: {
            // money,
            initiatorRange: this.$store.state.user.userId // 提审的时候每张表单都传一下发起人范围，条件判断有用
          }
        }
      };
      if (customChooseFlag) { // 自选审批人的节点
        param.nextAuditorList = this.nextAuditorList;
      }
      if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
      }
      if (this.parallelChooseNodes && this.parallelChooseNodes.length) { // 并行多分支
        var nextAuditorList = [];
        this.parallelChooseNodes.forEach(item => {
          var nodeProxyId = item.nextNodeTemplateId;
          var personList = item.nodeAuditList;
          personList.forEach(el => {
            var bizId = el.id;
            nextAuditorList.push({
              nodeProxyId, bizId
            });
          });
        });
        param.nextAuditorList = nextAuditorList;
      }
      if(this.isDraft)param.data.status = 'draft'
      var api = Api.schedule.saveFlowInstance
      if(this.isReInitiate){
        api = Api.schedule.saveFlowInstanceAgain
        delete param.data.flowProxyId
        param.data.id = this.flowInstanceId
        if(this.isDraft)api = Api.schedule.saveFlowInstance
      }
      // console.log('this.isReInitiate',this.isReInitiate)
      // return
      // id: this.flowInstanceId,
      this.$axios.post(
        api,
        param,
        (res) => {
          if (res.isSuccess) {
            this.$message.success('提交成功！');
            this.$bus.$emit('success')
          } else {
            if (res.data) {
              // this.flowNodeType = 'run_node_choose';
              this.nodeChooseVisible = true;
            } else {
              this.checkboxPersonGroup = [];
              this.$message.error(res.message);
            }
          }
        }
      );
    },
    submitFinalForFlow(flag){
      this.$parent.$refs.generateForm.getData().then(value => {
        // console.log('value',value)
        // return
        const param = {
          data: {
            id: this.flowInstanceId,
            formProxyId: this.formId,
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'project',
                otherBizId: this.$store.state.user.projectId,
              }
            ],
          },
          formDataMongoVo: {
            data: value
          }
        };
        if (flag) {
          param.nextAuditorList = this.nextAuditorList;
        }
        if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
          param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
        }
        if(this.isDraft)param.data.status = 'draft'
        this.$axios.post(
          Api.schedule.saveFlowInstanceAgain,
          param,
          res => {
            if (res.isSuccess) {
              this.$message.success('提交成功！');
              this.$bus.$emit('success')
              // this.chooseBranchNode = {};
              // this.chooseBranchNodeList = [];
              // this.handleCloseParallelChoose();
              // this.handleCloseBranchChoose();
              // this.$emit('resetParallelNodeChooseList', []);
              // this.$message.success('提交成功！');
              // this.closeCheck();
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }).catch(e => {
        console.log(e);
      });
    },
    getSuperVisorOrLeader(nodeAuditType, nextNodeId) {
      // 获取发起人部门主管或分管副总id--重新发起或者提交审批
      const url = nodeAuditType == 'department_supervisor' ? Api.schedule.getSupervisor : Api.schedule.getDeptLeader;
      this.$axios.post(
        url,
        {
          data: {
            id: localstorageGet('userId') // 发起人id
          }
        },
        res => {
          if (res.isSuccess) {
            var id = res?.data?.id || ''
            if(id){
              const nextAuditor = {
                bizId: id
              };
              if (nextNodeId) {
                // 手动分支-选择主管或副总类型节点
                nextAuditor.nodeProxyId = nextNodeId;
              }
              this.nextAuditorList = [nextAuditor];
              // this.enterpriseHandleSubmit(true);
              if (this.selectFlowType == 'enterprise') {
                // 表单流程
                this.enterpriseHandleSubmit(true);
              } else {
                // 无表单流程
                this.submitFinal(true);
              }
            }else{ //如果没有主管，降级，让用户自选审批节点
              if(this.nextAuditorList.length){
                if (this.selectFlowType == 'enterprise') {
                  // 表单流程
                  this.enterpriseHandleSubmit(true);
                } else {
                  // 无表单流程
                  this.submitFinal(true);
                }
              }else{
                this.nodeChooseVisible = true;
              }
              // if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
              //   this.nodeChooseVisible = true;

              // }else{
              //   if (this.selectFlowType == 'enterprise') {
              //     // 表单流程
              //     this.enterpriseHandleSubmit(true);
              //   } else {
              //     // 无表单流程
              //     this.submitFinal(true);
              //   }
              // }
            }
          }
        }
      );
    },
    getSuperVisorOrLeaderId(nodeAuditType) {
      // 并行分支获取副总和主管的id作为下一审批人传参
      return new Promise((resolve, reject) => {
        const url = nodeAuditType == 'department_supervisor' ? Api.schedule.getSupervisor : Api.schedule.getDeptLeader;
        this.$axios.post(
          url,
          {
            data: {
              id: localstorageGet('userId') // 发起人id
            }
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data.id);
            }
          }
        );
      });
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
