<!-- 无表单流程统一发起组件 -->
<template>
  <div>
    <!-- 审批人选择 -->
    <PersonSelectDialog :visible.sync="nodeChooseVisible" v-if="nodeChooseVisible" @getSelectPerson="getSelectPerson" :isProject="true"
      :selection-prompt="isCarbonCopyNode ? '请选择要抄送的人' : ''" />

    <!-- 并行审批人自选 -->
    <el-dialog v-if="parallelChooseVisible" width="400px" title="并行节点自选" class="nodePerson"
      :visible="parallelChooseVisible" :before-close="handleCloseParallelChoose" :close-on-click-modal="false"
      append-to-body>
      <div v-for="chooseNode in parallelChooseNodes" :key="chooseNode.nextNodeTemplateId">
        <span style="line-height: 28px;">{{ chooseNode.nodeName }} <span>：</span> </span>
        <span v-for="audit in chooseNode.nodeAuditList" :key="audit.id"> {{ audit.name }} </span>
        <el-button type="text" size="mini" @click="chooseParallelNode(chooseNode)">
          {{ chooseNode.nodeAuditList.length ? (chooseNode.nodeType == 'carbon_copy' ? '重新选择抄送人' : '重新选择') : (chooseNode.nodeType == 'carbon_copy' ? '选择抄送人' : '选择人员') }}</el-button>
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
              @click="handleChooseBranchNode">{{ branch.nodeType == 'carbon_copy' ? '选择抄送人' : '选择审批人' }}</el-button>
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

    <extended-attribute-users
      :nextNodeProxyId="currentChooseNodeId"
      :visible.sync="extendedVisible"
      :personList="extendedPersonList"
      @confirmChooseExtendPerson="confirmChooseExtendPerson"
    />
  </div>
</template>

<script>
import Api from '@/api'
import PersonSelectDialog from '@/views/GroupApproveManage/components/PersonSelectDialog.vue';
import extendedAttributeUsers from '@/components/extendedAttributeUsers.vue';
export default {
  name:'Flow',
  components: {PersonSelectDialog, extendedAttributeUsers},
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
      isDraft:false,
      currentChooseNodeId: '',
      isCarbonCopyNode: false,
      extendedVisible: false,
      extendedPersonList: []

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
                      nodeType: parallelNode.childFlowNodeTemplate.type,
                      auditType: parallelNode.childFlowNodeTemplate.flowNodeAuditConfig.auditType,
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
          const parallelNode = this.parallelChooseNodes.find(item => item.nextNodeTemplateId == this.parallelNextNodeTemplateId);
          this.$message.warning(parallelNode && parallelNode.nodeType == 'carbon_copy' ? '请选择要抄送的人' : '至少选择一位审批人');
          return false;
        }
        this.parallelNextNodeTemplateId = '';
        this.checkboxRersonGroup = [];
      } else if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        // 手动分支-自选
        if (data.checkboxPersonGroup && data.checkboxPersonGroup.length > 0) {
          this.chooseBranchNodeList = JSON.parse(JSON.stringify(data.checkboxPersonGroup));
          const nodeProxyId = this.chooseBranchNode.nextNodeTemplateId;
          this.nextAuditorList = this.nextAuditorList.filter(item => item.nodeProxyId != nodeProxyId);
          data.checkboxPersonGroup.forEach(item => {
            this.nextAuditorList.push(
              {
                bizId: item.id,
                nodeProxyId,
                auditDetailType: 'personnel'
              }
            );
          });
          this.nodeChooseVisible = false;
          // this.submitFinal(true);
        } else {
          this.$message.warning(this.chooseBranchNode.nodeType == 'carbon_copy' ? '请选择要抄送的人' : '至少选择一位审批人');
          return false;
        }
      } else {
        // 普通审批人自选
        this.checkboxPersonGroup = data.checkboxPersonGroup.map(x => x);
        if (!this.checkboxPersonGroup.length) {
          this.$message.warning(this.isCarbonCopyNode ? '请选择要抄送的人' : '至少选择一位审批人');
          return;
        }
        const nodeProxyId = this.currentChooseNodeId || this.nextNodeProxyId;
        this.nextAuditorList = this.nextAuditorList.filter(item => item.nodeProxyId != nodeProxyId);
        this.checkboxPersonGroup.map(item => {
          this.nextAuditorList.push({
            bizId: item.id,
            nodeProxyId,
            auditDetailType: 'personnel'
          });
        });
        this.submitFinal(true);
      }
    },
    handleCloseParallelChoose() {
      this.parallelChooseVisible = false;
    },
    // 并行节点自选
    chooseParallelNode(node) {
      this.parallelNextNodeTemplateId = node.nextNodeTemplateId;
      this.currentChooseNodeId = node.nextNodeTemplateId;
      this.isCarbonCopyNode = node.nodeType == 'carbon_copy';
      this.nodeChooseVisible = true;
    },
    // 发起人-并行节点自选审批人保存后提交
    async handleSaveParallelChooseNode() {
      const parallelNodeIds = this.parallelChooseNodes.map(item => item.nextNodeTemplateId);
      this.nextAuditorList = this.nextAuditorList.filter(item => parallelNodeIds.indexOf(item.nodeProxyId) === -1);
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
      const missingNode = this.parallelChooseNodes.find(item => item.auditType == 'run_node_choose' &&
        !(item.nodeAuditList || []).length);
      if (missingNode) {
        return this.$message.warning(missingNode.nodeType == 'carbon_copy' ? '请选择要抄送的人' : '该分支需选择审批人，请先选择');
      }
      this.parallelChooseNodes.forEach(item => {
        if (item.auditType == 'run_node_choose') {
          item.nodeAuditList.forEach(auditNode => {
            this.nextAuditorList.push(
              {
                bizId: auditNode.id,
                nodeProxyId: item.nextNodeTemplateId,
                auditDetailType: 'personnel'
              }
            );
          });
        } else if (item.auditType == 'department_supervisor' || item.auditType == 'branched_passage_manager') {
          // 为并行中的主管和副总添加审批人节点参数传到提交
          this.nextAuditorList.push(
            {
              bizId: item.auditType == 'department_supervisor' ? superVisorId : managerId,
              nodeProxyId: item.nextNodeTemplateId,
              auditDetailType: 'personnel'
            }
          );
        }
      });
      this.parallelChooseVisible = false;
      // 无表单流程
      this.submitFinal(true);
    },
    handleCloseBranchChoose() {
      this.chooseBranchNode = {};
      this.branchChooseVisible = false;
    },
    handleChangeChooseBranch(branch) {
      this.isCarbonCopyNode = branch.nodeType == 'carbon_copy';
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
          const hasSelectedApprover = this.nextAuditorList.some(item =>
            item.nodeProxyId == this.chooseBranchNode.nextNodeTemplateId && item.bizId
          );
          if (!hasSelectedApprover) {
            this.$message.warning(this.chooseBranchNode.nodeType == 'carbon_copy' ? '请选择要抄送的人' : '该分支需选择审批人，请先选择');
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
          formProxyId: this.formId,
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
      if (!this.isDraft && this.nextAuditorList.length) {
        param.nextAuditorList = this.nextAuditorList.map(item => ({
          ...item,
          auditDetailType: item.auditDetailType || 'personnel'
        }));
      }
      if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
        param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
      }
      if (this.parallelChooseNodes && this.parallelChooseNodes.length) { // 并行多分支
        const parallelNodeIds = this.parallelChooseNodes.map(item => item.nextNodeTemplateId);
        var nextAuditorList = this.nextAuditorList.filter(item => parallelNodeIds.indexOf(item.nodeProxyId) === -1);
        this.parallelChooseNodes.forEach(item => {
          var nodeProxyId = item.nextNodeTemplateId;
          var personList = item.nodeAuditList;
          personList.forEach(el => {
            var bizId = el.id;
            nextAuditorList.push({
              nodeProxyId,
              bizId,
              auditDetailType: 'personnel'
            });
          });
        });
        param.nextAuditorList = nextAuditorList;
      }
      if (this.isDraft) {
        param.data.status = 'draft';
      } else {
        param.data.status = 'run';
      }
      let api = this.isDraft ? Api.schedule.saveFlowInstance : Api.newWorkflowCenter.flowSubmit;
      if (this.isReInitiate) {
        param.data.id = this.flowInstanceId;
        api = this.isDraft ? Api.schedule.saveFlowInstance : Api.newWorkflowCenter.flowReSubmit;
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
          }
        }
      ).catch(err => {
        if (!this.isDraft) this.flowErrorHandle(err);
      });
    },
    submitFinalForFlow(flag){
      this.$parent.$refs.generateForm.getData().then(value => {
        // console.log('value',value)
        // return
        const param = {
          data: {
            id: this.flowInstanceId,
            flowProxyId: this.flowId,
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
          param.nextAuditorList = this.nextAuditorList.map(item => ({
            ...item,
            auditDetailType: item.auditDetailType || 'personnel'
          }));
        }
        if (this.chooseBranchNode && this.chooseBranchNode.nextNodeTemplateId) {
          param.fixedExecuteNodeId = this.chooseBranchNode.nextNodeTemplateId;
        }
        param.data.status = this.isDraft ? 'draft' : 'run';
        this.$axios.post(
          this.isDraft ? Api.schedule.saveFlowInstance : Api.newWorkflowCenter.flowReSubmit,
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
            }
          }
        ).catch(err => {
          if (!this.isDraft) this.flowErrorHandle(err);
        });
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
                bizId: id,
                auditDetailType: 'personnel'
              };
              if (nextNodeId) {
                // 手动分支-选择主管或副总类型节点
                nextAuditor.nodeProxyId = nextNodeId;
              }
              if (nextNodeId) {
                this.nextAuditorList = this.nextAuditorList.filter(item => item.nodeProxyId != nextNodeId);
              }
              this.nextAuditorList.push(nextAuditor);
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
        ).catch(err => {
          if (!this.isDraft) this.flowErrorHandle(err);
        });
    },
    confirmChooseExtendPerson(person) {
      const nodeProxyId = person.nodeProxyId || this.currentChooseNodeId;
      this.nextAuditorList = this.nextAuditorList.filter(item => item.nodeProxyId != nodeProxyId);
      this.nextAuditorList.push({
        nodeProxyId,
        bizId: person.id,
        name: person.name,
        auditDetailType: 'personnel'
      });
      this.extendedVisible = false;
      this.submitFinal(true);
    },
    flowErrorHandle(res) {
      const data = res && res.data;
      if (data && data.errorType === 'extended_attribute_users') {
        this.extendedPersonList = data.fullUserInfoList || [];
        this.currentChooseNodeId = data.nodeProxy && data.nodeProxy.id || '';
        this.extendedVisible = true;
        return;
      }
      if (data && data.errorType === 'run_node_choose') {
        this.isCarbonCopyNode = data.node && data.node.type === 'carbon_copy';
        this.currentChooseNodeId = data.node && data.node.id || '';
        this.nextNodeProxyId = this.currentChooseNodeId;
        this.nodeChooseVisible = true;
        return;
      }
      if (data && data.errorType === 'custom_choose') {
        this.manualChooseNodes = (data.branchNodes || []).map((node, index) => ({
          nextNodeTemplateId: node.id,
          nodeName: node.nodeName,
          nodeType: node.type,
          branchName: node.nodeName || `分支${index + 1}`,
          auditType: node.flowNodeAuditConfig && node.flowNodeAuditConfig.auditType
        }));
        this.branchChooseVisible = this.manualChooseNodes.length > 0;
        return;
      }
      if (data && data.errorType === 'parallel_choose') {
        this.parallelChooseNodes = (data.branchNodes || [])
          .filter(node => node.flowNodeAuditConfig && node.flowNodeAuditConfig.auditType === 'run_node_choose')
          .map(node => ({
            nodeName: node.nodeName,
            nextNodeTemplateId: node.flowNodeAuditConfig.nodeTemplateId || node.id,
            nodeType: node.type,
            auditType: node.flowNodeAuditConfig.auditType,
            nodeAuditList: []
          }));
        this.parallelChooseVisible = this.parallelChooseNodes.length > 0;
        if (!this.parallelChooseVisible) this.$message.error(res.message || '流程节点人员配置错误');
        return;
      }
      this.$message.error(res && (res.message || res.errorMsg) || '流程提交失败');
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
