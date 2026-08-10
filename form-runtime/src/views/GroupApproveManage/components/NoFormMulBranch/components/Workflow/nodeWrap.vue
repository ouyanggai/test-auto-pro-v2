<!--
 * @Descripttion: 多分支流程
 * @Author: zhengzetao
 * @Date: 2021-03-18
 数据字段解释：
 childFlowNodeTemplate：包含下一节点的信息

conditionNodes：如果是审批节点为null，如果是条件节点，里面就是包含条件节点的信息
conditionNodes.conditionList：存放判断条件（金额的判断会用到fieldaName，bvalue，conditionType，judge）

parallelNodes: 并行节点

flowNodeAuditConfig：审批节点才有的参数设置
flowNodeAuditConfig.flowNodeDetailConfigList：存放公司（bizId公司id）、人员、角色等信息

flowNodeFieldPowerTemplateLis：流程节点选择表单字段的数组，目的控制字段权限（formFieldTemplateEnglishName：字段名）(fieldPower: edit编辑  only_read只读)

type：condition（条件），start（发起人），common（审批节点），end（结束节点），error（错误节点）
-->

<template>
  <div>
    <!-- 审核人 -->
    <div
      v-if="nodeConfig.type == 'start' || nodeConfig.type == 'common'"
      class="node-wrap"
    >
      <div
        class="node-wrap-box"
        :class="nodeConfig.type == 'start' ? 'start-node ' : ''"
      >
        <!-- :class="(nodeConfig.type=='start'?'start-node ':'')+(isTried&&nodeConfig.error?'active error':'')" -->
        <div>
          <div
            class="title"
            :class="{
              titActive: nodeConfig.type == 'start',
              titActive: nodeConfig.type == 'common'
            }"
          >
            <!-- :class="{'titActive':nodeConfig.type =='start','titActive':nodeConfig.type =='common','titActiveEnd':nodeConfig.type =='end'}" -->
            <span v-show="nodeConfig.type == 'common'" class="iconfont">
              <i class="el-icon-user-solid" />
            </span>
            <span class="editable-title" @click="clickEvent()">{{
              nodeConfig.nodeName
            }}</span>
            <i
              v-if="nodeConfig.type != 'start' && nodeConfig.currentDelete"
              class="anticon anticon-close close"
              @click="delNode()"
            />
          </div>
          <div class="content" @click="setPerson(1)">
            <!-- @click="handleNode(1)" -->
            <div class="text">
              <span>定义节点配置<span class="jiaqian" v-if="nodeConfig.styleType">加签</span></span>
              <span
                v-if="(nodeConfig.flowNodeAuditConfig && nodeConfig.flowNodeAuditConfig.flowNodeDetailConfigList && nodeConfig.flowNodeAuditConfig.flowNodeDetailConfigList.length) ||
                  (nodeConfig.flowNodeAuditConfig.auditType && nodeConfig.flowNodeAuditConfig.auditType != 'assign')"
                class="complete"
              ><i class="el-icon-connection" /></span>
            </div>
            <i class="anticon anticon-right arrow" />
          </div>
        </div>
      </div>
      <addNode
        :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
        :node-config.sync="nodeConfig"
        :edit-type.sync="editType"
        :is-visible.sync="nodeConfig.id == flowNodeProxyId"
      />
    </div>
    <!-- 协同--目前和普通审批节点功能一样-考虑未来协同功能改变-单独写一份 -->
    <div v-if="nodeConfig.type == 'synergy'" class="node-wrap">
      <div class="node-wrap-box">
        <div>
          <div class="title synergy-node">
            <span class="iconfont">
              <i class="el-icon-user-solid" />
            </span>
            <span
              class="editable-title"
              @click="clickEvent()"
            >{{ nodeConfig.nodeName }}</span>
            <i
              v-if="nodeConfig.type != 'start' && nodeConfig.currentDelete"
              class="anticon anticon-close close"
              @click="delNode()"
            />
          </div>
          <div class="content" @click="setPerson(1)">
            <div class="text">
              <span>定义节点配置<span class="jiaqian" v-if="nodeConfig.styleType">加签</span></span>
              <span
                v-if="(nodeConfig.flowNodeAuditConfig && nodeConfig.flowNodeAuditConfig.flowNodeDetailConfigList && nodeConfig.flowNodeAuditConfig.flowNodeDetailConfigList.length) || (nodeConfig.flowNodeAuditConfig.auditType && nodeConfig.flowNodeAuditConfig.auditType != 'assign')"
                class="complete"
              ><i class="el-icon-connection" /></span>
            </div>
            <i class="anticon anticon-right arrow" />
          </div>
        </div>
      </div>
      <addNode
        :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
        :node-config.sync="nodeConfig"
        :edit-type.sync="editType"
        :is-visible.sync="nodeConfig.id == flowNodeProxyId"
      />
    </div>
    <!-- 条件分支 -->
    <div
      v-if="nodeConfig.type == 'condition' &&
        nodeConfig.branchExecuteType != 'custom_choose'
      "
      class="branch-wrap condition-branch"
    >
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- <button v-show="editType != 3" class="add-branch" @click="addTerm">
            添加条件
          </button> -->
          <!-- nodeConfig.conditionNodes：条件节点数据列表 -->
          <div
            v-for="(item, index) in nodeConfig.conditionNodes"
            :key="index"
            class="col-box"
          >
            <div class="condition-node">
              <div class="condition-node-box">
                <div class="auto-judge">
                  <div class="title-wrapper">
                    <template v-if="item.type">
                      <span v-if="!isInputList[index]" class="editable-title">
                        {{ item.nodeName }}
                      </span>
                    </template>
                    <template v-else>
                      <span class="editable-title">
                        {{ item.name }}
                      </span>
                    </template>
                    <i
                      v-if="nodeConfig.currentDelete"
                      class="anticon anticon-close close"
                      @click="delTerm(index)"
                    />
                  </div>
                  <div class="content" @click="setPerson(item.sort)">
                    设置条件
                  </div>
                </div>
                <addNode
                  :child-node-p.sync="item.childFlowNodeTemplate"
                  :edit-type.sync="editType"
                  :is-visible.sync="nodeConfig.id == flowNodeProxyId"
                />
              </div>
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap
              v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate"
              :node-config.sync="item.childFlowNodeTemplate"
              :json-data.sync="jsonData"
              :fields.sync="fields"
              :edit-type.sync="editType"
              :company-person-list.sync="companyPersonList"
              :field-tree-list.sync="fieldTreeList"
              :step1audit-way.sync="step1AuditWay"
              :props-role-list.sync="propsRoleList"
              :attr-list="attrList"
              :flow-node-proxy-id.sync="flowNodeProxyId"
            />
            <div v-if="index == 0" class="top-left-cover-line" />
            <div v-if="index == 0" class="bottom-left-cover-line" />
            <div
              v-if="index == nodeConfig.conditionNodes.length - 1"
              class="top-right-cover-line"
            />
            <div
              v-if="index == nodeConfig.conditionNodes.length - 1"
              class="bottom-right-cover-line"
            />
          </div>
        </div>
        <addNode
          :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
          :node-config.sync="nodeConfig"
          :edit-type.sync="editType"
          :is-visible.sync="nodeConfig.id == flowNodeProxyId"
        />
      </div>
    </div>
    <!-- 并行节点 -->
    <div
      v-if="nodeConfig.type == 'parallel'"
      class="branch-wrap parallel-branch"
    >
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- <button
            v-show="editType != 3"
            class="add-branch"
            @click="addParallelNode"
          >
            添加并行
          </button> -->
          <!-- nodeConfig.parallelNodes：并行节点数据列表 -->
          <div
            v-for="(item, index) in nodeConfig.parallelNodes"
            :key="index"
            class="col-box"
          >
            <div class="condition-node">
              <div class="condition-node-box" />
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap
              v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate"
              :node-config.sync="item.childFlowNodeTemplate"
              :parallel-top.sync="nodeConfig"
              :json-data.sync="jsonData"
              :fields.sync="fields"
              :edit-type.sync="editType"
              :company-person-list.sync="companyPersonList"
              :field-tree-list.sync="fieldTreeList"
              :step1audit-way.sync="step1AuditWay"
              :props-role-list.sync="propsRoleList"
              :attr-list="attrList"
              :flow-node-proxy-id.sync="flowNodeProxyId"
              @panetrateNodeConfig="panetrateNodeConfig"
            />
            <div v-if="index == 0" class="top-left-cover-line" />
            <div v-if="index == 0" class="bottom-left-cover-line" />
            <div
              v-if="index == nodeConfig.parallelNodes.length - 1"
              class="top-right-cover-line"
            />
            <div
              v-if="index == nodeConfig.parallelNodes.length - 1"
              class="bottom-right-cover-line"
            />
          </div>
        </div>
        <addNode
          :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
          :node-config.sync="nodeConfig"
          :edit-type.sync="editType"
          :is-visible.sync="nodeConfig.id == flowNodeProxyId"
        />
      </div>
    </div>
    <!-- 手动分支 -->
    <div
      v-if="nodeConfig.branchExecuteType == 'custom_choose'"
      class="branch-wrap manual-branch"
    >
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- <button v-show="editType != 3" class="add-branch" @click="addBranch">
            添加分支
          </button> -->
          <!-- nodeConfig.conditionNodes：条件节点数据列表 -->
          <div
            v-for="(item, index) in nodeConfig.conditionNodes"
            :key="index"
            class="col-box"
          >
            <div class="condition-node">
              <div class="condition-node-box">
                <addNode
                  v-show="item.childFlowNodeTemplate.type != 'empty'"
                  :child-node-p.sync="item.childFlowNodeTemplate"
                  :edit-type.sync="editType"
                  :is-visible.sync="nodeConfig.id == flowNodeProxyId"
                />
              </div>
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap
              v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate"
              :node-config.sync="item.childFlowNodeTemplate"
              :branch-top.sync="nodeConfig"
              :json-data.sync="jsonData"
              :fields.sync="fields"
              :edit-type.sync="editType"
              :company-person-list.sync="companyPersonList"
              :field-tree-list.sync="fieldTreeList"
              :step1audit-way.sync="step1AuditWay"
              :props-role-list.sync="propsRoleList"
              :attr-list="attrList"
              :flow-node-proxy-id.sync="flowNodeProxyId"
              @panetrateNodeConfig="panetrateNodeConfig"
            />
            <div v-if="index == 0" class="top-left-cover-line" />
            <div v-if="index == 0" class="bottom-left-cover-line" />
            <div
              v-if="index == nodeConfig.conditionNodes.length - 1"
              class="top-right-cover-line"
            />
            <div
              v-if="index == nodeConfig.conditionNodes.length - 1"
              class="bottom-right-cover-line"
            />
          </div>
        </div>
        <addNode
          :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
          :node-config.sync="nodeConfig"
          :edit-type.sync="editType"
          :is-visible.sync="nodeConfig.id == flowNodeProxyId"
        />
      </div>
    </div>
    <!-- 空节点 -->
    <div v-if="nodeConfig.type == 'empty'" class="node-wrap">
      <div
        class=" node-wrap-box"
        :class="nodeConfig.type == 'start' ? 'start-node ' : ''"
      >
        <div>
          <div class="title empty-node">
            <span v-show="nodeConfig.type == 'common'" class="iconfont">
              <i class="el-icon-user-solid" />
            </span>
            <span class="editable-title">{{
              nodeConfig.nodeName
            }}</span>
            <i
              v-if="nodeConfig.type != 'start' && nodeConfig.currentDelete"
              class="anticon anticon-close close"
              @click="delEmptyNode()"
            />
          </div>
          <div class="content" style="cursor: default;">
            空节点
          </div>
        </div>
      </div>
      <addNode
        :child-node-p.sync="nodeConfig.childFlowNodeTemplate"
        :node-config.sync="nodeConfig"
        :edit-type.sync="editType"
        :is-visible.sync="nodeConfig.id == flowNodeProxyId"
      />
    </div>

    <NodeWrap
      v-if="nodeConfig.childFlowNodeTemplate && nodeConfig.childFlowNodeTemplate"
      :node-config.sync="nodeConfig.childFlowNodeTemplate"
      :json-data.sync="jsonData"
      :fields.sync="fields"
      :edit-type.sync="editType"
      :company-person-list.sync="companyPersonList"
      :field-tree-list.sync="fieldTreeList"
      :step1audit-way.sync="step1AuditWay"
      :props-role-list.sync="propsRoleList"
      :attr-list="attrList"
      :flow-node-proxy-id.sync="flowNodeProxyId"
    />

    <!-- 审批节点配置弹窗 -->
    <el-drawer
      :append-to-body="true"
      title="详情"
      :visible.sync="dialogVisible"
      direction="rtl"
      class="condition_copyer"
      size="550px"
      :wrapper-closable="false"
      :destroy-on-close="true"
      :show-close="false"
    >
      <div class="demo-drawer__content">
        <div class="condition_content drawer_content">
          <div class="approver_content">
            <el-form ref="nodeForm" :model="nodeForm" :rules="nodeFormRules">
              <el-form-item
                label="节点名称："
                :label-width="formLabelWidth"
                prop="nodeName"
              >
                <el-input
                  v-model="nodeForm.nodeName"
                  :disabled="editType == 3"
                  maxlength="50"
                  placeholder="请输入节点名称"
                  autocomplete="off"
                />
              </el-form-item>
              <el-form-item
                v-if="nodeConfig.type != 'start'"
                label="审批方式："
                :label-width="formLabelWidth"
              >
                <el-radio-group v-model="nodeForm.type" class="clear">
                  <el-radio :label="'scramble'">竞签</el-radio>
                  <el-radio :label="'countersign'">会签</el-radio>
                </el-radio-group>
              </el-form-item>
              <el-form-item
                v-if="nodeConfig.type != 'start' && nodeForm.type == 'countersign'"
                label="会签人数："
                :label-width="formLabelWidth"
              >
                <div style="display:flex;">
                  <el-input-number v-if="nodeForm.allCountersignNum != '1'" v-model="nodeForm.countersignNum" :disabled="nodeForm.allCountersignNum == '1'" :min="2" step-strictly />
                  <el-checkbox v-if="nodeForm.auditType == 'run_node_choose'" v-model="nodeForm.allCountersignNum" true-label="1" false-label="''" style="margin-left:10px;">所有人会签</el-checkbox>
                </div>
              </el-form-item>
              <!-- 无表单的有何权限功能 -->
              <el-form-item label="有何权限：" :label-width="formLabelWidth">
                <el-radio-group v-model="nodeForm.copyNodePower" class="clear">
                  <el-radio :label="'copyNodePower1'">同当前节点</el-radio>
                  <el-radio :label="'copyNodePower2'">只读</el-radio>
                </el-radio-group>
                <!-- <el-button
                  type="primary"
                  size="mini"
                  @click="selectPermission('normalNode')"
                >
                  {{ editType == 3 ? "查看权限" : "选择权限" }}
                </el-button> -->
              </el-form-item>
              <el-form-item label="审批人设置：" :label-width="formLabelWidth">
                <el-radio-group v-model="nodeForm.auditType" class="clear" @change="changeAllCounterNum">
                  <el-radio :label="'assign'">选择人员</el-radio>
                  <!-- <el-radio
                    v-if="nodeForm.type == 'scramble' && nodeConfig.type != 'start'"
                    :label="'initiator'"
                  >发起人自己</el-radio>
                  <el-radio
                    v-if="(nodeForm.type == 'scramble' || nodeForm.type == 'countersign') && nodeConfig.type != 'start'"
                    :label="'run_node_choose'"
                  >审批人自选</el-radio>
                  <el-radio :label="'role'">选择角色</el-radio>
                  <el-radio :label="'company_id'" v-if="nodeConfig.type=='start'">选择公司</el-radio>
                  <el-radio :label="'department'" v-if="nodeConfig.type=='start'">选择部门</el-radio>
                  <el-radio
                    v-if="nodeForm.type == 'scramble' && nodeConfig.type != 'start'"
                    :label="'position'"
                  >选择岗位</el-radio>
                  <el-radio
                    v-if="nodeForm.type == 'scramble' && nodeConfig.type != 'start'"
                    :label="'level'"
                  >选择岗级</el-radio>
                  <el-radio
                    v-if="nodeForm.type == 'scramble' && nodeConfig.type != 'start'"
                    :label="'extendedAttribute'"
                  >扩展属性</el-radio> -->
                </el-radio-group>
              </el-form-item>
              <template v-if="nodeForm.auditType=='role' || nodeForm.auditType=='position' || nodeForm.auditType=='level' || nodeForm.auditType=='extendedAttribute'">
                <el-form-item label="匹配不到人：" :label-width="formLabelWidth" v-if="nodeConfig.type != 'start'">
                  <el-checkbox v-model="nodeConfig.isSkip" >自动跳过该节点</el-checkbox>
                </el-form-item>
              </template>

              <el-form-item
                v-if="nodeForm.auditType == 'department'"
                label="选择部门："
                prop="conditionName"
                :label-width="formLabelWidth"
              >
                <el-button type="primary" icon="el-icon-plus" :disabled="editType == 3" @click="designatePerson">添加部门
                </el-button>
                <!-- <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.personTagList.length" @click="clearSelectPerson('personTagList')">清空人员</el-link> -->
                <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.departmentTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closeDepartmentTag(tag)"
                  >
                    {{ tag.name }}
                  </el-tag>
                </div>
              </el-form-item>
              <el-form-item
                v-if="nodeForm.auditType == 'company_id'"
                label="选择公司："
                prop="conditionName"
                :label-width="formLabelWidth"
              >
                <el-button type="primary" icon="el-icon-plus" :disabled="editType == 3" @click="designatePerson">添加公司
                </el-button>
                <!-- <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.personTagList.length" @click="clearSelectPerson('personTagList')">清空人员</el-link> -->
                <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.companyTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closeCompanyTag(tag)"
                  >
                    {{ tag.name }}
                  </el-tag>
                </div>
              </el-form-item>
              <el-form-item
                v-if="nodeForm.auditType == 'assign'"
                label="指定人员："
                prop="conditionName"
                :label-width="formLabelWidth"
              >
                <el-button
                  type="primary"
                  size="mini"
                  icon="el-icon-plus"
                  :disabled="editType == 3"
                  @click="designatePerson"
                >添加人员<span v-if="nodeForm.personTagList.length">({{nodeForm.personTagList.length}})</span>
                </el-button>
                <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.personTagList.length" @click="clearSelectPerson('personTagList')">清空人员</el-link>
                <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.personTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closePersonTag(tag)"
                  >
                    {{ tag.name }}
                  </el-tag>
                </div>
              </el-form-item>

              <el-form-item
                v-if="nodeForm.auditType == 'run_node_choose'"
                label="审批人自选范围："
                prop="personRangeTagList"
                :label-width="formLabelWidth"
              >
                <el-button type="primary" icon="el-icon-plus" :disabled="editType == 3" @click="selectRange">选择范围</el-button>
                <!-- <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.personTagList.length" @click="clearSelectPerson('personTagList')">清空人员</el-link> -->
                <div v-if="nodeForm.rangeType == 'position' || nodeForm.rangeType == 'role'" style="max-height: 250px;overflow: auto;">
                  <el-collapse v-for="duty in nodeForm.personRangeTagList" :key="duty.id">
                    <el-collapse-item v-if="nodeForm.rangeType == 'position'" :title="'岗位：'+ duty.departName+ ' / ' + duty.name" :name="1">
                      <el-tag v-for="user in duty.userList" :key="user.id">{{ user.name }}</el-tag>
                    </el-collapse-item>
                    <el-collapse-item v-else-if="nodeForm.rangeType == 'role'" :title="'角色：'+ duty.name" :name="1">
                      <el-tag v-for="user in duty.roleUserList" :key="user.id">{{ user.userVo.name }}</el-tag>
                    </el-collapse-item>
                  </el-collapse>
                </div>
                <div v-else style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.personRangeTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closeRangePersonTag(tag)"
                  >
                    <span v-if="nodeForm.rangeType == 'personnel' || nodeForm.rangeType == 'company'">{{ tag.name }}</span>
                    <span v-else-if="nodeForm.rangeType == 'department'">{{ tag.firstCompanyObj.name + ' / ' + tag.name }}</span>
                  </el-tag>
                </div>
              </el-form-item>

              <el-form-item
                v-if="nodeForm.auditType == 'role'"
                label="选择角色："
                prop="role"
                :label-width="formLabelWidth"
              >
                <el-button
                  type="primary"
                  size="mini"
                  icon="el-icon-plus"
                  :disabled="editType == 3"
                  @click="addFlowRole"
                >添加角色
                </el-button>
                <div style="max-height: 250px;overflow: auto;">
                  <el-collapse
                    v-if="nodeForm.checkedRole && nodeForm.checkedRole.id"
                    v-model="activeRoleNames"
                  >
                    <el-collapse-item
                      :title="nodeForm.checkedRole.name"
                      :name="1"
                    >
                      <el-tag
                        v-for="user in roleUserList"
                        :key="user.id"
                      >{{ user.userVo.name }}</el-tag>
                    </el-collapse-item>
                  </el-collapse>
                </div>
              </el-form-item>
              <!-- <el-form-item
                label="处理期限："
                prop="auditCondition"
                :label-width="formLabelWidth"
              >
                <div style="display:flex;">
                  <el-input-number v-if="nodeForm.unit" v-model="nodeForm.delay" step-strictly />
                  <el-select v-model="nodeForm.unit" style="width:90px;" placeholder="请选择">
                    <el-option
                      v-for="item in timeUnitOptions"
                      :key="item.value"
                      :label="item.label"
                      :value="item.value">
                    </el-option>
                  </el-select>
                </div>
              </el-form-item> -->
              <el-form-item
                v-if="nodeForm.auditType == 'level'"
                label="选择岗级："
                prop="auditCondition"
                :label-width="formLabelWidth"
              >
                <el-button
                  type="primary"
                  icon="el-icon-plus"
                  :disabled="editType == 3"
                  @click="designateJobTitle"
                >添加岗级
                </el-button>
                <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    v-if="nodeForm.auditCondition"
                    @close="closeJobTitleTag()"
                  >
                    {{ nodeForm.auditConditionName }}
                  </el-tag>
                </div>
                <!-- <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.jobTitleTagList.length" @click="clearSelectPerson('jobTitleTagList')">清空岗位</el-link> -->
                <!-- <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.jobTitleTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closeJobTitleTag(tag)"
                  >
                    {{ tag.name }}
                  </el-tag>
                </div> -->
              </el-form-item>
              <el-form-item
                v-if="nodeForm.auditType == 'position'"
                label="选择岗位："
                prop="jobTitleTagList"
                :label-width="formLabelWidth"
              >
                <el-button
                  type="primary"
                  icon="el-icon-plus"
                  :disabled="editType == 3"
                  @click="designateJob"
                >添加岗位
                </el-button>
                <el-link :underline="false" type="primary" style="margin-left: 15px;" v-if="nodeForm.jobTitleTagList.length" @click="clearSelectPerson('jobTitleTagList')">清空岗位</el-link>
                <div style="max-height: 250px;overflow: auto;margin-top: 5px;">
                  <el-tag
                    v-for="tag in nodeForm.jobTitleTagList"
                    :key="tag.id"
                    size="medium"
                    :closable="editType != 3"
                    :disable-transitions="false"
                    @close="closeJobTag(tag)"
                  >
                    {{ tag.name }}
                  </el-tag>
                </div>
              </el-form-item>
              <el-form-item
                v-if="nodeForm.auditType == 'extendedAttribute'"
                label="选择属性："
                prop="selectAttr"
                :label-width="formLabelWidth"
              >
                <el-select v-model="nodeForm.selectAttr.id" clearable :disabled="editType == 3" placeholder="请选择属性" class="search-select">
                  <el-option v-for="item in attrList" :key="item.id" :label="item.name" :value="item.id">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-form>
          </div>
        </div>
        <div class="demo-drawer__footer clear">
          <el-button
            type="primary"
            @click="saveTreeData"
          >{{ editType == 3 ? "关 闭" : "保 存" }}</el-button>
        </div>
      </div>
    </el-drawer>
    <!-- 条件设置弹窗 -->
    <el-drawer
      :append-to-body="true"
      title="条件设置"
      :visible.sync="conditionDrawer"
      direction="rtl"
      class="condition_copyer"
      size="550px"
      :wrapper-closable="false"
      :destroy-on-close="true"
      :show-close="false"
    >
      <div class="demo-drawer__content">
        <div class="condition_content drawer_content">
          <div class="approver_content">
            <!-- <el-button type="primary" @click="selectPermission('conditionNode')">{{ editType == 3 ? '查看' : '定义' }}条件字段
                          </el-button> -->
            <el-form
              ref="conditionSelectForm"
              :model="conditionSelectForm"
              label-width="80px"
              :rules="conditionSelectFormRules"
            >
              <el-form-item label="条件名称" prop="conditionName">
                <el-input
                  v-model="conditionSelectForm.conditionName"
                  :disabled="editType == 3"
                  maxlength="15"
                  placeholder="请输入条件名称"
                  autocomplete="off"
                />
              </el-form-item>
              <!-- v-if="conditionSelectForm.conditionSelectList.length" -->
              <el-form-item label="条件字段">
                <div
                  v-for="(item,
                          index) in conditionSelectForm.conditionSelectList"
                  :key="index"
                  style="position:relative;margin-bottom:10px;"
                >
                  <div class="condition-wrap">
                    <!-- <el-button type="primary" @click="selectPermission('conditionNode')"
                                        style="width:120px;margin-right:5px;">{{ editType == 3 ? '查看' : '定义'}}条件字段</el-button> -->
                    <div style="display: flex;">
                      <el-input
                        v-model="item.valueName"
                        placeholder="请选择条件字段"
                        suffix-icon="el-icon-date"
                        readonly
                        style="width:204px;margin-right:5px;margin-bottom:2px;"
                        @focus="selectPermission('conditionNode', index)"
                      />
                      <el-select
                        v-if="item.valueType && item.valueType != 'NUMBER'"
                        v-model="item.btype"
                        placeholder="请选择类型"
                        style="width:180px;"
                        @change="val => changeBtype(val, index)"
                      >
                        <el-option label="自定义" :value="null" />
                        <el-option label="人员" value="person" />
                      </el-select>
                    </div>
                    <div style="display: flex;">
                      <el-select
                        v-if="item.valueType == 'NUMBER' || item.btype"
                        v-model="item.judge"
                        placeholder="请选择条件"
                        style="width:120px;margin-right:5px;"
                      >
                        <el-option
                          label="大于"
                          value="gt"
                          :disabled="item.btype == 'person'"
                        />
                        <el-option
                          label="小于"
                          value="lt"
                          :disabled="item.btype == 'person'"
                        />
                        <el-option
                          label="等于"
                          value="eq"
                          :disabled="item.btype == 'person'"
                        />
                        <el-option
                          label="大于等于"
                          value="gte"
                          :disabled="item.btype == 'person'"
                        />
                        <el-option
                          label="小于等于"
                          value="lte"
                          :disabled="item.btype == 'person'"
                        />
                        <el-option
                          label="包含"
                          value="contains"
                          :disabled="item.btype != 'person'"
                        />
                      </el-select>
                      <template v-if="item.valueName">
                        <!-- 人员选择 -->
                        <!-- @click="designatePerson(index)" -->
                        <div
                          v-if="item.btype == 'person'"
                          class="condition-personBox"
                          @click="designatePerson(index)"
                        >
                          <div>
                            <el-tag
                              v-for="(tag, tagIndex) in item.personList"
                              :key="tag.id"
                              size="small"
                              :closable="editType != 3"
                              :disable-transitions="false"
                              @close="
                                closeConditionPersonTag(item, tag, tagIndex, index)"
                            >
                              {{ tag.name }}
                            </el-tag>
                          </div>
                          <div class="person-iconBox">
                            <i
                              class="el-icon-s-custom"
                              @click="designatePerson(index)"
                            />
                          </div>
                        </div>
                        <!-- 是或否等一些字符输入 -->
                        <el-input
                          v-else
                          v-model="item.bvalue"
                          placeholder="请填写条件内容"
                          style="width:264px;"
                        >
                          <template
                            v-if="item.valueType && item.valueType == 'NUMBER'"
                            slot="append"
                          />
                        </el-input>
                      </template>
                    </div>
                    <i
                      class="el-icon-delete delete-condition"
                      @click="delConditionItem(index)"
                    />
                    <!-- v-if="conditionSelectForm.conditionSelectList.length > 1" -->
                  </div>

                  <div
                    v-if="conditionSelectForm.conditionSelectList.length > 1 &&
                      index !==
                      conditionSelectForm.conditionSelectList.length - 1
                    "
                    style="margin-top:8px;"
                  >
                    <el-select
                      v-model="item.conditionType"
                      placeholder="请选择其他条件"
                      style="width:182px;"
                    >
                      <el-option label="或" value="or" />
                      <el-option label="且" value="and" />
                    </el-select>
                  </div>
                </div>
                <i
                  class="el-icon-circle-plus-outline"
                  style="font-size:18px;cursor: pointer;color:#1989FA"
                  @click="addConditionItem"
                />
              </el-form-item>
              <!-- <template v-if="Object.keys(conditionCurFieldObj).length && conditionCurFieldObj.valueType == 'NUMBER'">
                                  </template>
                                  <template v-if="Object.keys(conditionCurFieldObj).length && conditionCurFieldObj.valueType != 'NUMBER'">
                <div v-for="(item, index) in conditionSelectForm.conditionSelectList" :key="index">
                  <el-form-item label="条件字段" :prop="`conditionSelectList.${index}.bvalue`" :rules="bRules">
                    <el-input :disabled='editType == 3' v-model="item.bvalue" maxlength="15" placeholder="请输入条件字段值"
                      autocomplete="off"></el-input>
                  </el-form-item>
                </div>
              </template> -->
            </el-form>
          </div>
        </div>
        <div class="demo-drawer__footer clear">
          <el-button v-if="editType != 3" type="primary" @click="saveCondition">
            确 定</el-button>
          <el-button @click="conditionDrawer = false">取 消</el-button>
        </div>
      </div>
    </el-drawer>
    <!-- 无表单选择节点 -->
    <FieldSelectDialog
      v-if="fieldSelectDialog"
      :visible.sync="fieldSelectDialog"
      :field-check-list="nodeForm.fieldCheckList"
      :node-type="nodeType"
      :edit-type="editType"
      @select="handleFieldSelect"
      @checkConditionField="checkConditionField"
    />

    <!-- 选择人员或者公司或者部门 -->
    <PersonSelectDialog v-if="personSelectDialog" :flag="nodeForm.auditType == 'company_id' ? '7' : nodeForm.auditType == 'department' ? '2' : '3'"
     :visible.sync="personSelectDialog" :startDefaultCheckedKeys="startDefaultCheckedKeys" @select="handleExaminerSelect" />

    <!-- 添加角色 -->
    <SelectRole
      v-if="flowRoleSelectVisible"
      :flow-role-select-visible.sync="flowRoleSelectVisible"
      @handleSelectRole="handleSelectRole"
    />
    <!-- 选择岗级 -->
    <JobTitleSelectDialog
      v-if="JobTitleSelectDialog"
      :visible.sync="JobTitleSelectDialog"
      :auditCondition="nodeForm.auditCondition"
      @select="handleJobTitleSelect"
    />

    <!-- 审批人自选范围 -->
    <SelectRangeDialog
      v-if="selectRangeDialog"
      :visible.sync="selectRangeDialog"
      :rangeActiveType="nodeForm.rangeType"
      :defaultCheckedKeys="defaultCheckedKeys"
      @handleSelectRange="handleSelectRange"
    >
    </SelectRangeDialog>

    <!-- 选择岗级 -->
    <JobPosition
      v-if="JobTitleDialog"
      :visible.sync="JobTitleDialog"
      @select="handleJobSelect"
    >
    </JobPosition>

  </div>
</template>
<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import addNode from '../../../commonFlow/AddNode.vue';
import PersonSelectDialog from '../../../commonFlow/PersonSelectDialog';
import SelectRole from '../../../commonFlow/SelectRole.vue';
import JobTitleSelectDialog from '../../../commonFlow/JobTitleSelectDialog';
import JobPosition from '../../../commonFlow/JobPosition.vue';
import SelectRangeDialog from '../../../commonFlow/selectRangeDialog.vue';
import FieldSelectDialog from '../../../commonFlow/FieldSelectDialog';
import { deepClone,getObjById } from '@/utils';
import mixin from '../../../commonFlow/mixin.js';

export default {
  name: 'NodeWrap',
  components: {
    addNode,
    PersonSelectDialog,
    SelectRole,
    FieldSelectDialog,
    JobTitleSelectDialog,
    JobPosition,
    SelectRangeDialog
  },
  props: [
    'nodeConfig',
    'jsonData',
    'fields',
    'editType',
    'companyPersonList',
    'fieldTreeList',
    'step1AuditWay',
    'parallelTop',
    'branchTop',
    'propsRoleList',
    'attrList',
    'flowNodeProxyId'
  ],
  data() {
    var validateAttr = (rule, value, callback) => {
      // console.log('value',value)
      if (value.id == '') {
        callback(new Error('请选择属性'));
      } else {
        callback();
      }
    };
    return {
      // 我的
      formLabelWidth: '130px',
      nodeForm: {
        nodeName: '', // 节点名称
        nodeDesc: '', // 节点描述
        nodeFieldPower: [], // 字段权限
        editData: {},
        auditType: 'assign',
        copyNodePower:'copyNodePower1',
        type: 'scramble',
        countersignNum: 1,
        allCountersignNum: '',
        delay: null,
        unit: null,
        checkedRole: {},
        personTagList: [],
        companyTagList:[], //公司列表
        departmentTagList:[], // 部门列表
        fieldCheckList: [],
        jobTitleTagList: [],
        selectAttr: { // 选择属性
          name: '',
          id: '',
        },
        // 审批人自选范围
        rangeType:'',
        personRangeTagList:[]
      },
      JobTitleSelectDialog: false,
      dialogVisible: false,
      companyList: [], // 权限列表
      conditionSelectForm: {
        conditionSelectList: [
        ],
        conditionName: ''
      },
      selectConditionKey: '',
      nodeWrapTitle: '定义节点字段',
      nodeFormRules: {
        nodeName: [
          { required: true, message: '请输入节点名称', trigger: 'blur' }
        ],
        jobTitleTagList: [{ required: true, message: '请选择', trigger: 'blur' }],
        selectAttr: [
          { validator: validateAttr, trigger: 'change' }
        ],
        // 'selectAttr.id': [{ required: true, message: '请选择属性', trigger: 'change' }],
      },
      conditionSelectFormRules: {
        conditionName: [
          { required: true, message: '请输入条件名称', trigger: 'blur' }
        ]
        // bvalue: [{ required: true, message: '请输入条件字段值', trigger: 'blur' }]
      },
      bRules: [
        { required: true, message: '请输入条件字段值', trigger: 'blur' }
      ],
      conditionCurFieldObj: {},
      id: '',
      checkAll: false,
      checkboxGroup: [],
      checkboxData: [],
      checkboxRersonGroup: [],
      // dialogVisibleNodeAttr: false,
      dialogVisibleFlowRole: false,
      dialogVisibleFlowRerson: false,
      editData: {},
      isClick: false,
      flowNodeAuditConfig: {
        auditType: 'assign', // 暂时默认
        flowNodeDetailConfigList: []
      },
      sort: 0,
      disabledCellType: 0,
      conditionRadio: 0,
      placeholderList: ['发起人', '审核人', '抄送人'],
      isInputList: [],
      isInput: false,
      approverDrawer: false,
      roles: [],
      conditionDrawer: false,
      conditionVisible: false,
      conditionConfig: {},
      conditionsConfig: {
        conditionNodes: []
      },
      conditions: [],
      conditionList: [],
      conditionType: '',
      personSelectDialog: false,
      fieldSelectDialog: false,
      flowRoleSelectVisible: false,
      nodeType: '',
      focusFieldIndex: '',
      activeRoleNames: [],
      roleUserList: [],
      JobTitleDialog:false,

      //属性选择
      // attrList:[],

      // 审批人自选范围
      defaultCheckedKeys:[],
      selectRangeDialog:false,
      allCompanyInfoList:[],
      rangeMapList:{
        "company":{
          name:'公司',
          flag:'7',
          type:'1',
        },
        "department":{
          name:'部门',
          flag:'2',
          type:'2',
        },
        "personnel":{
          name:'人员',
          flag:'3',
          type:'5',
        },
        "position":{
          name:'岗位',
          flag:'4',
        },
        "role":{
          name:'角色',
          flag:'',
        },
      },
      startDefaultCheckedKeys:[],
      timeUnitOptions:[
        {
          value: null,
          label:'无',
        },
        {
          value:'MINUTES',
          label:'分钟',
        },
        {
          value:'HOURS',
          label:'小时',
        },
        {
          value:'DAYS',
          label:'天',
        },
      ],
    };
  },
  mixins:[mixin],
  created() { },
  mounted() {
    // this.getAttrList();
    // this.getRoleList(); // 用于审批人范围选择回显
  },
  methods: {
    changeAllCounterNum(val){
      // this.nodeForm.allCountersignNum = (val == 'run_node_choose') ? '1' : '';
      if (val !== 'run_node_choose') {
        this.nodeForm.allCountersignNum = '';
      }
    },
    // 选择范围弹窗
    selectRange(){
      this.selectRangeDialog = true;
    },
    // 选择范围弹窗选择数据后进行处理
    handleSelectRange(data){
      console.log('handleSelectRange',data)
      const obj = {};
      this.nodeForm.rangeType = data.rangeType;
      // || data.rangeType == 'position'
      if (data.rangeType == 'personnel') { // 原来岗位逻辑放这里，后面发现相同岗位名称会被去重，放到下面的逻辑判断
        // 去重
        const newArr = data.checkData.reduce((item, next) => {
          obj[next.name] ? '' : obj[next.name] = true && item.push(next);
          return item;
        }, []);
        this.nodeForm.personRangeTagList = newArr;
      } else if (data.rangeType == 'department' || data.rangeType == 'company' || data.rangeType == 'role' || data.rangeType == 'position') {
        this.nodeForm.personRangeTagList = data?.checkData || [];
      }
      console.log('this.nodeForm.personRangeTagList',this.nodeForm.personRangeTagList)
      this.defaultCheckedKeys = this.nodeForm.personRangeTagList.map(x=> x.id);
    },
    // 删除审批节点指定人员的人员
    closeRangePersonTag(tag,tag2) {
      this.nodeForm.personRangeTagList.splice(
        this.nodeForm.personRangeTagList.indexOf(tag),
        1
      );
      // console.log('closeRangePersonTag1',this.nodeForm.personRangeTagList)
      // console.log('tag',tag)
      // console.log('tag2',tag2)
      // if (this.nodeForm.rangeType == 'position') {

      // } else if (this.nodeForm.rangeType == 'role') {
      //   let roleArr = this.nodeForm.personRangeTagList.find(x=>x.id == tag.id);
      //   roleArr.roleUserList.splice(roleArr.roleUserList.indexOf(tag2),1);
      //   console.log('roleArr',roleArr)
      //   // this.nodeForm.personRangeTagList.roleUserList.splice(
      //   //   this.nodeForm.personRangeTagList.roleUserList.indexOf(tag.userVo),
      //   //   1
      //   // );
      // } else {
      //   this.nodeForm.personRangeTagList.splice(
      //     this.nodeForm.personRangeTagList.indexOf(tag),
      //     1
      //   );
      // }
      console.log('closeRangePersonTag2',this.nodeForm.personRangeTagList)
      this.defaultCheckedKeys = this.nodeForm.personRangeTagList.map(x=> x.id);
    },
    // 获取属性
    // getAttrList() {
    //   this.$axios.post(
    //     '/web/user/api/expandAttr/list',
    //     {
    //       data: {
    //         name:'',
    //         expandAttrType:null,
    //         enableType:"enable"
    //       },
    //       // pagination: true,
    //       // current: this.pagination.pages,
    //       // size: this.pagination.size
    //     },
    //     res => {
    //       if (res.isSuccess) {
    //         this.attrList = res.data || [];
    //         // this.pagination.total = res.total;
    //       }
    //     }
    //   );
    // },
    clearSelectPerson(key) {
      this.$confirm('确认清空所有选择', '确认').then(() => {
        this.nodeForm[key] = [];
      }).catch();
    },
    closeJobTitleTag() {
      // this.nodeForm.jobTitleTagList.splice(this.nodeForm.jobTitleTagList.indexOf(tag), 1);
      this.nodeForm.auditCondition = ''
      this.nodeForm.auditConditionName = ''
    },
    closeJobTag(tag) {
      this.nodeForm.jobTitleTagList.splice(this.nodeForm.jobTitleTagList.indexOf(tag), 1);
    },
    // 条件字段类型
    changeBtype(val, index) {
      const conditionNodes = JSON.parse(
        JSON.stringify(this.nodeConfig.conditionNodes)
      );
      const conNode = conditionNodes[this.sort].conditionList;
      this.focusFieldIndex = index;
      if (val == '') {
        conNode[index].personList = [];
      } else if (val == 'person') {
        conNode[index].judge = 'contains';
      }
      conNode[index].bvalue = '';
      this.conditionSelectForm.conditionSelectList = conNode;
      this.nodeConfig.conditionNodes = conditionNodes;
    },
    // 打开指定人员弹窗
    designatePerson(index) {
      console.log('nodeForm.departmentTagList',this.nodeForm.departmentTagList)
      const auditTypeCopy = this.nodeForm.auditType || '';
      console.log('auditTypeCopy ',auditTypeCopy )
      if (auditTypeCopy == 'assign') { // 人员
        this.startDefaultCheckedKeys = this.nodeForm.personTagList.map(x=>(x?.bizId||x.id));
      } else if (auditTypeCopy == 'company_id') { // 公司
        this.startDefaultCheckedKeys = this.nodeForm.companyTagList.map(x=>(x?.bizId||x.id));
      } else if (auditTypeCopy == 'department') { // 部门
        this.startDefaultCheckedKeys = this.nodeForm.departmentTagList.map(x=>(x?.bizId||x.id));
      }

      this.focusFieldIndex = index;
      this.personSelectDialog = true;
    },
    // 打开添加角色弹窗
    addFlowRole() {
      this.flowRoleSelectVisible = true;
    },
    // 打开选择岗位弹窗
    designateJob(index) {
      // this.focusFieldIndex = index;
      this.JobTitleDialog = true;
    },
    // 打开选择岗级弹窗
    designateJobTitle(index) {
      // this.focusFieldIndex = index;
      this.JobTitleSelectDialog = true;
    },
    // 岗位选择弹窗select之后调用,显示选择的岗位
    handleJobSelect(data){
        const obj = {};
        const newData = this.nodeForm.jobTitleTagList.concat(data);
        const newArr = newData.reduce((item, next) => { // 去重
          obj[next.name] ? '' : obj[next.name] = true && item.push(next);
          return item;
        }, []);
        this.nodeForm.jobTitleTagList = newArr
    },
    handleJobTitleSelect(data) {
      // console.log('data',data)
      // const obj = {};
      // const newData = this.nodeForm.jobTitleTagList.concat(data);
      // const newArr = newData.reduce((item, next) => { // 去重
      //   obj[next.name] ? '' : obj[next.name] = true && item.push(next);
      //   return item;
      // }, []);
      this.nodeForm.auditCondition = data.id
      this.nodeForm.auditConditionName = data.name
      // this.nodeForm.jobTitleTagList = [data];
    },
    // 打开权限字段选择弹窗（也可以是定义条件字段）
    selectPermission(nodeType, index) {
      this.nodeType = nodeType;
      if (nodeType == 'normalNode') {
        // 普通节点
        this.nodeForm.fieldCheckList = [];
        if (this.nodeConfig.flowNodeFieldPowerTemplateList) {
          this.nodeForm.fieldCheckList = this.nodeConfig.flowNodeFieldPowerTemplateList.map(
            x => x.formFieldTemplateEnglishName
          );
        }
      } else if (nodeType == 'conditionNode') {
        // 条件节点
        this.nodeForm.fieldCheckList = [];
        this.focusFieldIndex = index;
        const str = this.nodeConfig.conditionNodes[this.sort].conditionList[
          index
        ].fieldaName;
        this.nodeForm.fieldCheckList = str ? [str] : [];
      }

      this.fieldSelectDialog = true;
    },
    // 删除审批节点指定人员的人员
    closePersonTag(tag) {
      this.nodeForm.personTagList.splice(
        this.nodeForm.personTagList.indexOf(tag),
        1
      );
    },
    //删除指定部门
    closeDepartmentTag(tag) {
      this.nodeForm.departmentTagList.splice(this.nodeForm.departmentTagList.indexOf(tag), 1);
    },
    //删除指定公司
    closeCompanyTag(tag) {
      this.nodeForm.companyTagList.splice(this.nodeForm.companyTagList.indexOf(tag), 1);
    },
    // 删除条件节点选择的人员
    closeConditionPersonTag(item, tag, tagIndex, index) {
      const conditionNodes = JSON.parse(
        JSON.stringify(this.nodeConfig.conditionNodes)
      );
      const conNode = conditionNodes[this.sort].conditionList;

      conNode[index].personList.splice(item.personList.indexOf(tag), 1);
      conNode[index].bvalue = conNode[index].personList
        .map(x => x.id)
        .join(',');
      this.conditionSelectForm.conditionSelectList = conNode;
      this.nodeConfig.conditionNodes = conditionNodes;
    },
    // 无表单-选择字段权限点击确认（审批节点）
    handleFieldSelect(data) {
      const flowNodeFieldPowerTemplateList = [];
      data.forEach(x => {
        const obj = {
          fieldPower: 'edit',
          formFieldTemplateEnglishName: x
        };
        flowNodeFieldPowerTemplateList.push(obj);
      });

      this.nodeConfig.flowNodeFieldPowerTemplateList = flowNodeFieldPowerTemplateList;
    },
    // 无表单-选择条件字段（条件节点）
    checkConditionField(data) {
      // const conditionList = [];
      const conditionNodes = JSON.parse(
        JSON.stringify(this.nodeConfig.conditionNodes)
      );
      let conNode = conditionNodes[this.sort].conditionList;
      if (Object.keys(data).length) {
        const fieldObj = this.fieldTreeList.find(
          item => item.dictValue == data.dictValue
        );
        // 切换字段时先清空一些字段的原先值
        conNode[this.focusFieldIndex].bvalue = '';
        conNode[this.focusFieldIndex].btype = null;
        conNode[this.focusFieldIndex].judge = null;
        conNode[this.focusFieldIndex].personList = [];
        conNode[this.focusFieldIndex].fieldaName = fieldObj.dictValue;
        conNode[this.focusFieldIndex].valueType = fieldObj.valueType;
        conNode[this.focusFieldIndex].valueName = fieldObj.dictLabel;
      } else {
        // 没有选择字段
        conNode = [];
      }
      this.conditionSelectForm.conditionSelectList = conNode;
      this.nodeConfig.conditionNodes = conditionNodes;
    },
    // 无表单-选择负责人
    handleExaminerSelect(data) {
      const obj = {};
      if (this.nodeConfig.type == 'condition') {
        const conNode = this.nodeConfig.conditionNodes[this.sort].conditionList[
          this.focusFieldIndex
        ];
        if(conNode.personList === undefined)conNode.personList = []
        const personList = conNode.personList.concat(data);
        // 去重
        const newArr = personList.reduce((item, next) => {
          obj[next.name] ? '' : (obj[next.name] = true && item.push(next));
          return item;
        }, []);
        conNode.personList = newArr;
        conNode.bvalue = personList.map(x => x.id).join(',');
      }else {
        if(this.nodeForm.auditType  == 'company_id'){
          // const newData = this.nodeForm.companyTagList.concat(data);
          // 去重
          // const newArr = newData.reduce((item, next) => {
            // obj[next.name] ? '' : obj[next.name] = true && item.push(next);
            // return item;
          // }, []);
          this.nodeForm.companyTagList = data;
        } else if (this.nodeForm.auditType  == 'department') {
          this.nodeForm.departmentTagList = data;
        } else{
          const newData = this.nodeForm.personTagList.concat(data);
          // 去重
          const newArr = newData.reduce((item, next) => {
            obj[next.name] ? '' : obj[next.name] = true && item.push(next);
            return item;
          }, []);
          this.nodeForm.personTagList = newArr;
        }
      }
    },
    // 选择角色-确定
    handleSelectRole(radioRole) {
      this.nodeForm.checkedRole = radioRole;
      this.getRoleUser();
    },
    // 获取选中角色下的用户
    getRoleUser() {
      this.$axios.post(
        Api.roleManage.getRoleUserList,
        {
          data: {
            flowRoleId: this.nodeForm.checkedRole.id
          },
          pagination: false
        },
        res => {
          if (res.isSuccess) {
            this.roleUserList = res.data || [];
          }
        }
      );
    },
    // 添加条件配置项
    addConditionItem() {
      const conditionSelectList = this.conditionSelectForm.conditionSelectList;

      conditionSelectList.push({
        btype: null,
        bvalue: '',
        judge: null,
        fieldaName: this.selectConditionKey,
        conditionType: null,
        sort: null
      });
      // conditionSelectList[conditionSelectList.length - 1].conditionType = null;
      // conditionSelectList[conditionSelectList.length - 2].conditionType = '';
    },
    // 添加条件节点
    addTerm() {
      const len = this.nodeConfig.conditionNodes.length + 1;
      this.nodeConfig.conditionNodes.push({
        nodeName: '条件' + len,
        type: 3,
        sort: len,
        strategyType: 'change',
        conditionList: [],
        nodeUserList: [],
        flowNodeAuditConfig: {
          auditType: null,
          flowNodeDetailConfigList: []
        },
        childFlowNodeTemplate: null
      });
      this.$emit('update:nodeConfig', this.nodeConfig);
    },
    // 删除条件配置项
    delConditionItem(index) {
      this.conditionSelectForm.conditionSelectList.splice(index, 1);
    },
    // 删除条件节点
    delTerm(index) {
      this.$confirm('确认要删除该条件节点及该节点下的所有其他节点?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          this.nodeConfig.conditionNodes.splice(index, 1);
          this.nodeConfig.conditionNodes.map((x,index)=>x.sort = (Number(index)+1));
          // for (var i = 0; i < this.nodeConfig.conditionNodes.length; i++) {
          //   this.nodeConfig.conditionNodes[i].error = this.conditionStr(this.nodeConfig.conditionNodes[i], i) == '请设置条件' && i != this.nodeConfig.conditionNodes.length - 1;
          // }
          this.$emit('update:nodeConfig', this.nodeConfig);
          if (this.nodeConfig.conditionNodes.length == 1) {
            if (this.nodeConfig.childFlowNodeTemplate) {
              if (this.nodeConfig.conditionNodes[0].childFlowNodeTemplate) {
                this.reData(
                  this.nodeConfig.conditionNodes[0].childFlowNodeTemplate,
                  this.nodeConfig.childFlowNodeTemplate
                );
              } else {
                this.nodeConfig.conditionNodes[0].childFlowNodeTemplate = this.nodeConfig.childFlowNodeTemplate;
              }
            }
            if(this.nodeConfig.conditionNodes[0].childFlowNodeTemplate && this.nodeConfig.conditionNodes[0].childFlowNodeTemplate.strategyId !== undefined)this.nodeConfig.conditionNodes[0].childFlowNodeTemplate.strategyId = null
            this.$emit(
              'update:nodeConfig',
              this.nodeConfig.conditionNodes[0].childFlowNodeTemplate
            );
          }
        })
        .catch(() => { });
    },
    reData(data, addData) {
      if (!data.childFlowNodeTemplate) {
        data.childFlowNodeTemplate = addData;
      } else {
        this.reData(data.childFlowNodeTemplate, addData);
      }
    },
    // 删除审批节点
    delNode() {
      this.$confirm('是否确认要删除该审批节点?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          if (this.parallelTop && this.parallelTop.type == 'parallel') {
            // 并行节点删除
            this.delParallelNode();
          } else if (
            this.branchTop &&
            this.branchTop.branchExecuteType == 'custom_choose'
          ) {
            // 手动分支删除
            // this.changeBranchToEmpty();
            this.delBranchNode();
          } else {
            // 普通节点删除
            this.$emit(
              'update:nodeConfig',
              this.nodeConfig.childFlowNodeTemplate
            );
          }
        })
        .catch(() => { });
    },
    // 删除并行节点
    delParallelNode() {
      if(this.nodeConfig.childFlowNodeTemplate && this.nodeConfig.childFlowNodeTemplate.strategyId !== undefined)this.nodeConfig.childFlowNodeTemplate.strategyId = null
      this.$emit('update:nodeConfig', this.nodeConfig.childFlowNodeTemplate);
      const parallelNodesCopy = JSON.parse(JSON.stringify(this.parallelTop));
      parallelNodesCopy.parallelNodes.map((parallel, index) => {
        if (!parallel.childFlowNodeTemplate) {
          parallelNodesCopy.parallelNodes.splice(index, 1);
        }
      });
      if (parallelNodesCopy.parallelNodes.length == 1) {
        if (parallelNodesCopy.childFlowNodeTemplate) {
          if (parallelNodesCopy.parallelNodes[0].childFlowNodeTemplate) {
            this.reData(
              parallelNodesCopy.parallelNodes[0].childFlowNodeTemplate,
              parallelNodesCopy.childFlowNodeTemplate
            );
          } else {
            parallelNodesCopy.parallelNodes[0].childFlowNodeTemplate =
              parallelNodesCopy.childFlowNodeTemplate;
          }
        }
        this.$emit(
          'panetrateNodeConfig',
          parallelNodesCopy.parallelNodes[0].childFlowNodeTemplate
        );
      } else {
        this.$emit('panetrateNodeConfig', parallelNodesCopy);
      }
    },
    // 删除并行节点和手动分支--穿透属性
    panetrateNodeConfig(nodesCopy) {
      this.$emit('update:nodeConfig', nodesCopy);
    },
    // 添加并行节点
    addParallelNode() {
      const len = this.nodeConfig.parallelNodes.length + 1;
      this.nodeConfig.parallelNodes.push({
        nodeName: '节点' + len,
        error: false,
        type: 'common',
        strategyType: 'change',
        sort: 1,
        conditionList: [],
        nodeUserList: [],
        flowNodeAuditConfig: {
          auditType: null,
          flowNodeDetailConfigList: []
        },
        flowNodeFieldPowerTemplateList: [],
        childFlowNodeTemplate: {
          nodeName: '审核人-节点' + len,
          error: false,
          type: 'common',
          settype: 1,
          selectMode: 0,
          selectRange: 0,
          directorLevel: 1,
          examineMode: 1,
          noHanderAction: 1,
          examineEndDirectorLevel: 0,
          childFlowNodeTemplate: null,
          nodeUserList: [],
          flowNodeAuditConfig: {
            auditType: null,
            flowNodeDetailConfigList: []
          },
          flowNodeFieldPowerTemplateList: []
        }
      });
      this.$emit('update:nodeConfig', this.nodeConfig);
    },
    // 添加手动分支
    addBranch() {
      const len = this.nodeConfig.conditionNodes.length + 1;
      this.nodeConfig.conditionNodes.push({
        nodeName: '分支' + len,
        type: 3,
        sort: len,
        strategyType: 'change',
        conditionList: [],
        nodeUserList: [],
        flowNodeAuditConfig: {
          auditType: null,
          flowNodeDetailConfigList: []
        },
        childFlowNodeTemplate: {
          nodeName: '审核人',
          error: false,
          type: 'common',
          settype: 1,
          selectMode: 0,
          selectRange: 0,
          directorLevel: 1,
          examineMode: 1,
          noHanderAction: 1,
          examineEndDirectorLevel: 0,
          childFlowNodeTemplate: null,
          nodeUserList: [],
          flowNodeAuditConfig: {
            auditType: null,
            flowNodeDetailConfigList: []
          },
          flowNodeFieldPowerTemplateList: []
        }
      });
      this.$emit('update:nodeConfig', this.nodeConfig);
    },
    // 改分支节点为空
    changeBranchToEmpty() {
      if (this.nodeConfig.childFlowNodeTemplate) {
        this.delBranchNode();
      } else {
        const emptyNodeConfig = this.nodeConfig;
        emptyNodeConfig.nodeName = '空节点';
        emptyNodeConfig.type = 'empty';
        this.$emit('update:nodeConfig', emptyNodeConfig);
      }
    },
    // 删除空节点
    delEmptyNode() {
      if (this.parallelTop) { // 删除并行下的空节点
        this.delParallelNode();
      } else if (this.branchTop) { // 删除手动分支下的空节点
        this.delBranchNode();
      } else { // 删除普通空节点
        if(this.nodeConfig.childFlowNodeTemplate && this.nodeConfig.childFlowNodeTemplate.strategyId !== undefined)this.nodeConfig.childFlowNodeTemplate.strategyId = null
        this.$emit('update:nodeConfig', this.nodeConfig.childFlowNodeTemplate);
      }
    },
    // 删除手动分支
    delBranchNode() {
      // this.$emit('update:nodeConfig', null);
      if(this.nodeConfig.childFlowNodeTemplate && this.nodeConfig.childFlowNodeTemplate.strategyId !== undefined)this.nodeConfig.childFlowNodeTemplate.strategyId = null
      this.$emit('update:nodeConfig', this.nodeConfig.childFlowNodeTemplate);
      const branchNodeCopy = JSON.parse(JSON.stringify(this.branchTop));
      branchNodeCopy.conditionNodes.map((branch, index) => {
        if (!branch.childFlowNodeTemplate) {
          branchNodeCopy.conditionNodes.splice(index, 1);
        }
      });
      if (branchNodeCopy.conditionNodes.length == 1) {
        if (branchNodeCopy.childFlowNodeTemplate) {
          if (branchNodeCopy.conditionNodes[0].childFlowNodeTemplate) {
            this.reData(
              branchNodeCopy.conditionNodes[0].childFlowNodeTemplate,
              branchNodeCopy.childFlowNodeTemplate
            );
          } else {
            branchNodeCopy.conditionNodes[0].childFlowNodeTemplate =
              branchNodeCopy.childFlowNodeTemplate;
          }
        }
        this.$emit(
          'panetrateNodeConfig',
          branchNodeCopy.conditionNodes[0].childFlowNodeTemplate
        );
      } else {
        this.$emit('panetrateNodeConfig', branchNodeCopy);
      }
    },
    // 用于选择范围的数据回显
    async rangeShowData(rangeList) { // 获取公司部门架构数据
      console.log('rangeShowData',this.nodeForm.rangeType)
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: this.rangeMapList[this.nodeForm.rangeType]['flag'],
            id: localstorageGet('companyId') // 公司id
          }
        },
        async res => {
          if (res.isSuccess) {
            let data = res.data;
            let personList = [];
            if (this.nodeForm.rangeType == 'personnel' || this.nodeForm.rangeType == 'company') {
              rangeList.forEach(item=>{
                let currentTarget = getObjById(data,item.bizId)
                // console.log('currentTarget',currentTarget)
                personList.push({
                  id:item.bizId,
                  name:currentTarget.name,
                })
              })
            } else if(this.nodeForm.rangeType == 'department') {
              rangeList.forEach(item=>{
                let allAncestorsList = this.getCheckTag(data, item.bizId);
                // console.log('allAncestorsList',allAncestorsList)
                let firstCompanyObj = allAncestorsList.find(x=>x.type == '1');
                let currentTarget = getObjById(data,item.bizId)
                personList.push({
                  id:item.bizId,
                  name:currentTarget.name,
                  firstCompanyObj:firstCompanyObj
                })
              })
            } else if (this.nodeForm.rangeType == 'position'){
              for (var i=0;i<rangeList.length;i++){
                let item = rangeList[i];
                let result = await this.getUserListByDutyId(item.bizId);
                // console.log('result',result)
                let currentTarget = getObjById(data,item.bizId)
                let currentTarget2 = getObjById(data,currentTarget.parentId)
                personList.push({
                  id:item.bizId,
                  name:currentTarget.name,
                  departName:currentTarget2.name,
                  userList:result
                })
              }
              // rangeList.forEach(async item=>{
              //   let result = await this.getUserListByDutyId(item.bizId);
              //   console.log('result',result)
              //   let currentTarget = getObjById(data,item.bizId)
              //   personList.push({
              //     id:item.bizId,
              //     name:currentTarget.name,
              //     userList:result
              //   })
              // })
            } else if (this.nodeForm.rangeType == 'role'){
              rangeList.forEach(async item=>{
                // let currentTarget = this.roleList.find(x=>x.id == item.bizId)
                let currentTarget = this.propsRoleList.find(x=>x.id == item.bizId)
                personList.push({
                  id:item.bizId,
                  name:currentTarget.name,
                  roleUserList:currentTarget.roleUserList
                })
              })
            }
            console.log('personList',personList)
            this.nodeForm.personRangeTagList = deepClone(personList);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 点击节点打开配置弹窗，条件节点和审批节点
    setPerson(sort) {
      // 数据回显处理
      const { type, conditionNodes } = this.nodeConfig;
      const { flowNodeFieldPowerTemplateList } = this.nodeConfig;
      console.log('conditionNodes',conditionNodes)
      console.log('sort',sort)

      flowNodeFieldPowerTemplateList &&
        flowNodeFieldPowerTemplateList.forEach(item => {
          this.$set(this.editData, item.formFieldTemplateEnglishName, true);
        });
      this.checkboxData = [];
      this.checkboxData.length = 0;
      if (type == 'condition') {
        // 条件字段数据
        // this.conditionSelectForm.conditionSelectList = [];
        // if (sort < 1) {
        //   this.sort = sort;
        // } else {
        //   this.sort = sort - 1;
        // }
        // this.conditionDrawer = true;
        // conditionNodes[this.sort].conditionList.forEach(item => {
        //   const list = [];
        //   const fieldTreeObj = this.fieldTreeList.find(
        //     x => x.dictValue == item.fieldaName
        //   ); // 在字段列表中查询出来的当前字段属性对象
        //   item.valueType = fieldTreeObj && fieldTreeObj.valueType;
        //   item.valueName = fieldTreeObj && fieldTreeObj.dictLabel;
        //   if (item.btype && item.btype == 'person') {
        //     item.bvalue.split(',').forEach(val => {
        //       const personInfoItem = this.companyPersonList.find(
        //         x => x?.id == val
        //       );
        //       list.push(personInfoItem);
        //     });
        //     item.personList = list.filter(Boolean);
        //   }
        // });
        // this.conditionSelectForm.conditionSelectList =
        //   conditionNodes[this.sort].conditionList;
        // this.conditionSelectForm.conditionName =
        //   conditionNodes[this.sort].nodeName || conditionNodes[this.sort].name;
      } else {
        console.log('this.nodeConfig',this.nodeConfig)
        if (!this.nodeConfig.currentDelete) {
          return;
        }

        // 节点数据
        this.flowNodeAuditConfig = this.nodeConfig.flowNodeAuditConfig;
        const auditTypeCopy = this.flowNodeAuditConfig.auditType || '';
        const list = [];
        if(auditTypeCopy == 'run_node_choose'){
          console.log('进来审批人自选')
          // 审批人自选范围的回显
          let rangeList = this.flowNodeAuditConfig?.nodeAuditScopeList || [];
          console.log('rangeList',rangeList)
          this.defaultCheckedKeys = rangeList.map(x => x.bizId);
          this.nodeForm.rangeType = rangeList.length ? rangeList[0]['type'] : 'company';
          this.rangeShowData(rangeList);
        } else if (auditTypeCopy == 'assign') {
          console.log('指定人员-人员列表回显')
          console.log('this.flowNodeAuditConfig.flowNodeDetailConfigList',this.flowNodeAuditConfig.flowNodeDetailConfigList)
          console.log('this.companyPersonList',this.companyPersonList)
          
          // 指定人员-人员列表回显
          this.flowNodeAuditConfig.flowNodeDetailConfigList.forEach(item => {
            const personInfoItem = this.companyPersonList.find(
              x => x.id == item.bizId
            );
            if (personInfoItem) {
              list.push({
                bizId: item.bizId,
                id: item.bizId,
                name: personInfoItem.name
              });
            }
          });

          this.startDefaultCheckedKeys = list.map(x=>x.bizId);
          this.nodeForm.personTagList = list;
        }else if(auditTypeCopy == 'company_id'){ // 指定公司回显
          this.flowNodeAuditConfig.flowNodeDetailConfigList.forEach(item => {
            list.push({
              bizId: item.bizId,
              id: item.id,
              name: item.name
            });
          });
          this.startDefaultCheckedKeys = list.map(x=>x.bizId);
          this.nodeForm.companyTagList = list;
        } else if(auditTypeCopy == 'department'){ // 指定部门回显
          console.log(1224,this.flowNodeAuditConfig)
          this.flowNodeAuditConfig.flowNodeDetailConfigList.forEach(item => {
            list.push({
              bizId: item.bizId,
              id: item.id,
              name: item.name
            });
          });

          this.startDefaultCheckedKeys = list.map(x=>x.bizId);
          console.log('部门-list',this.startDefaultCheckedKeys)
          this.nodeForm.departmentTagList = list;
        } else if (auditTypeCopy == 'role') {
          // 选择角色-角色人员回显--目前只选择一种审批角色
          if (this.flowNodeAuditConfig?.flowNodeDetailConfigList[0]?.bizId) {
            this.setRole(this.flowNodeAuditConfig.flowNodeDetailConfigList[0].bizId || '');
          }
        } else if (auditTypeCopy == 'level') {
          // this.nodeForm.jobTitleTagList = this.flowNodeAuditConfig.flowNodeDetailConfigList;
          this.nodeForm.auditCondition = this.flowNodeAuditConfig.auditCondition;
          this.setJobTitleName(this.nodeForm.auditCondition);// 根据职级列表回显岗位名称
        } else if (auditTypeCopy == 'position') {
          this.nodeForm.jobTitleTagList = this.flowNodeAuditConfig.flowNodeDetailConfigList;
          this.setJobName(this.nodeForm.jobTitleTagList);// 根据岗位列表回显岗位名称
        } else if (auditTypeCopy == 'extendedAttribute') {
          // console.log(22222222,this.flowNodeAuditConfig.flowNodeDetailConfigList)
          this.nodeForm.selectAttr.id = this.flowNodeAuditConfig.flowNodeDetailConfigList[0]?.bizId
          // this.nodeForm.selectAttr = this.flowNodeAuditConfig.flowNodeDetailConfigList;
        }
        this.nodeForm.auditType = this.flowNodeAuditConfig.auditType || 'assign';
        this.nodeForm.type = this.flowNodeAuditConfig.type || 'scramble';
        this.nodeForm.countersignNum = this.flowNodeAuditConfig.countersignNum || 2;
        this.nodeForm.delay = this.nodeConfig.delay || null;
        this.nodeForm.unit = this.nodeConfig.unit || null;

        if (this.nodeForm.countersignNum == -1) {
          this.nodeForm.allCountersignNum = '1';
        }
        this.dialogVisible = true;
        this.nodeForm.nodeName = this.nodeConfig.nodeName || '';
      }
    },
    // 审批节点配置保存
    saveTreeData() {
      console.log('saveTreeData11')

      this.$refs.nodeForm.validate(valid => {
        if (valid) {
          this.nodeConfig.nodeName = this.nodeForm.nodeName;
          const list = [];
          const rangeList = [];
          let countersignNumOver = false;
          /* 处理审批类型不同---指定人员/选择角色 */
          if (this.nodeForm.auditType == 'assign') {
            if (this.nodeForm.personTagList.length) {
              this.nodeForm.personTagList &&
              this.nodeForm.personTagList.map(item => {
                const obj = {};
                obj.bizId = item.id;
                obj.id = item.id;
                obj.name = item.name;
                obj.auditDetailType = 'personnel';
                list.push(obj);
              });
            } else {
              this.$message.warning('未选择人员');
              return false;
            }

            countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > list.length : false;
          } else if (this.nodeForm.auditType == 'company_id') {
            this.nodeForm.companyTagList &&
              this.nodeForm.companyTagList.map(item => {
                const obj = {};
                obj.bizId = item.bizId || item.id;
                // obj.id = item.id;
                obj.name = item.name;
                obj.auditDetailType = 'company';
                list.push(obj);
              });
              countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > list.length : false;
          } else if (this.nodeForm.auditType == 'department'){
            console.log('save-this.nodeForm.departmentTagList',this.nodeForm.departmentTagList)
            this.nodeForm.departmentTagList &&
              this.nodeForm.departmentTagList.map(item => {
                const obj = {};
                obj.bizId = item.bizId || item.id;
                // obj.id = item.id;
                obj.name = item.name;
                obj.auditDetailType = 'department';
                list.push(obj);
              });
              countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > list.length : false;
          }  else if (this.nodeForm.auditType == 'role') {
            if (this.nodeForm.checkedRole && this.nodeForm.checkedRole.id) {
              list.push({ bizId: this.nodeForm.checkedRole.id });
            } else {
              this.$message.warning('未添加角色');
              return false;
            }
            countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > this.roleUserList.length : false;
          } else if (this.nodeForm.auditType == 'level') {
            if(this.nodeForm.auditCondition){
              this.flowNodeAuditConfig.auditCondition = this.nodeForm.auditCondition
            }else{
              this.$message.warning('未选择岗位');
              return false;
            }

          }else if(this.nodeForm.auditType == 'position'){
            this.nodeForm.jobTitleTagList &&
              this.nodeForm.jobTitleTagList.map(item => {
                const obj = {};
                obj.bizId = item.bizId || item.id;
                obj.id = item.id;
                obj.name = item.name;
                obj.auditDetailType = 'position';
                list.push(obj);
              });
            countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > list.length : false;
          } else if(this.nodeForm.auditType == 'extendedAttribute'){ // 属性
            let attrObj = this.attrList.find(x=>x.id == this.nodeForm.selectAttr.id);
            let obj = {
              bizId:attrObj.id, // 第一次添加bizId和id一样，后面编辑可能不一样
              id:attrObj.id,
              name:attrObj.name,
              auditDetailType: 'extendedAttribute'
            }
            list.push(obj);
            countersignNumOver = this.nodeForm?.type == 'countersign' ? this.nodeForm.countersignNum > list.length : false;
          } else if(this.nodeForm.auditType == 'run_node_choose'){ // 审批人自选
            console.log('saveTree-this.personRangeTagList',this.nodeForm.personRangeTagList)
            console.log('saveTree-this.rangeType',this.nodeForm.rangeType)
            this.nodeForm.personRangeTagList.forEach(item =>{
              const obj = {};
              obj.bizId = item.bizId || item.id;
              obj.type = this.nodeForm.rangeType;
              rangeList.push(obj);
            })
          }

          if (countersignNumOver) {
            this.$message.warning('会签人数不能大于可审批人数');
            return false;
          }

          this.nodeConfig.unit = this.nodeForm.unit;
          this.nodeConfig.delay = this.nodeForm.delay;
          this.flowNodeAuditConfig.auditType = this.nodeForm.auditType;
          this.flowNodeAuditConfig.type = this.nodeForm.type;
          this.flowNodeAuditConfig.countersignNum = this.nodeForm.allCountersignNum == '1'  ? -1 : this.nodeForm.countersignNum;
          // this.flowNodeAuditConfig.countersignNum = this.nodeForm.countersignNum;
          this.flowNodeAuditConfig.flowNodeDetailConfigList = list;
          this.flowNodeAuditConfig.nodeAuditScopeList = rangeList;
          this.nodeConfig.flowNodeAuditConfig = this.flowNodeAuditConfig;

          if (this.nodeForm.copyNodePower == 'copyNodePower1') {
            console.log('this.nodeConfig-保存配置',this.nodeConfig)
            this.nodeConfig.flowNodeFieldPowerTemplateList = JSON.parse(JSON.stringify(this.nodeConfig.tempStorageFieldConfig));
          } else if (this.nodeForm.copyNodePower == 'copyNodePower2') {
            this.nodeConfig.flowNodeFieldPowerTemplateList = [];
          }
          // console.log('nodeForm',this.nodeForm)
          console.log('this.nodeConfig',this.nodeConfig)

          this.activeRoleNames = [];
          this.dialogVisible = false;
        }
      });
    },
    // 输入框事件
    clickEvent(index) {
      if (index || index === 0) {
        this.$set(this.isInputList, index, true);
      } else {
        this.isInput = true;
      }
    },
    // 条件节点配置后的保存
    saveCondition() {
      // 最后一条条件分支不设置条件的话，conditionList可以为空数组
      this.$refs.conditionSelectForm.validate(valid => {
        if (valid) {
          const conditionNodes = this.nodeConfig.conditionNodes;
          const conditionSelectList = this.conditionSelectForm.conditionSelectList;
          if (conditionSelectList.length) {
            const len = conditionSelectList.length - 1;
            conditionSelectList[len].conditionType = null;
          }
          this.$set(
            conditionNodes[this.sort],
            'nodeName',
            this.conditionSelectForm.conditionName
          );
          this.$set(
            conditionNodes[this.sort],
            'name',
            this.conditionSelectForm.conditionName
          );

          // 保存后处理数据格式和校验
          // 校验,最后一条分支可以不加校验。后端逻辑是前面条件不通，会默认走最后一条。
          let flag = false;
          for (var i = 0; i < conditionSelectList.length; i++) {
            const listItem = conditionSelectList[i];
            if (!listItem.fieldaName) {
              this.$message.error(`请选择条件字段！`);
              flag = true;
              break;
            }
            if (listItem.valueType == 'NUMBER') {
              if (Object.is(Number(listItem.bvalue), NaN)) {
                this.$message.error(`条件内容请填写数字！`);
                flag = true;
                break;
              }
            } else if (listItem.valueType == 'STRING') {
              if (!listItem.btype) {
                // 条件类型是自定义时
                listItem.judge = 'eq';
                if (!listItem.bvalue) {
                  this.$message.error(`请填写条件内容！`);
                  flag = true;
                  break;
                }
                if (!Object.is(Number(listItem.bvalue), NaN)) {
                  this.$message.error(`条件内容请填写非数字！`);
                  flag = true;
                  break;
                }
              }
            }

            const isNoBvalue = !listItem.bvalue;
            const isNoJudge = !listItem.judge;
            // const isNoTypeList = conditionSelectList.map(x => !x.conditionType).filter(x => x); // 要求最后一个conditionType为nul，因为后端传参要求
            const isNoTypeList =
              i == conditionSelectList.length - 1
                ? false
                : !listItem.conditionType; // 最后一个conditionType为nul，其他的不能为空
            if (isNoBvalue || isNoJudge || isNoTypeList) {
              this.$message.error(`请完成判断条件的选择或输入！`);
              flag = true;
              break;
            }
          }
          if (flag) {
            return;
          }
          conditionNodes[this.sort].conditionList = conditionSelectList;
          // 每个条件节点内给个sort排序
          conditionNodes[this.sort].conditionList.forEach((x, index) => {
            x.sort = index;
          });
          this.conditionDrawer = false;
        }
      });
    },
    // 角色人员回显
    setRole(configRoleId) {
      this.$axios.post(
        Api.roleManage.getRoleList,
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            scope: 'invest'
          }
        },
        res => {
          if (res.isSuccess) {
            const data = res.data;
            data.forEach(role => {
              if (role.id == configRoleId) {
                this.nodeForm.checkedRole = role;
                this.getRoleUser();
              }
            });
          }
        }
      );
    },
    // 查询岗位名称用于回显name
    setJobName(jobList) {
      let id = localstorageGet('companyId');
      if (this.$route.query?.relative) {
        id = localstorageGet('formDetailCompany');
      }
      this.$axios.post(
        Api.taskManage.taskArrange.getJobTitleTree,
        {
          data: {
            flag: 4,
            id, // 公司id
            customerCode: localstorageGet('customerCode') ?? undefined // customerCode
          }
        },
        res => {
          if (res.isSuccess) {
            var allDuty = [];
            var treeData = res.data;
            var func = (i) => {
              i.forEach(j => {
                if (j.type == '4') {
                  allDuty.push(j);
                }
                j.childrenList && func(j.childrenList);
              });
            };
            func(treeData);
            jobList.forEach(job => {
              var name = allDuty.find(duty => duty.id == job.bizId)?.name;
              this.$set(job, 'name', name);
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    setJobTitleName(auditCondition) {
      this.$axios.post(
        Api.templateLibrary.dutyLevel,
        {
          data: { enableType: 'enable'}
        },
        res => {
          if (res.isSuccess) {
            let list = res?.data || [];
            let find = list.find(item=>item.id == auditCondition)
            // this.nodeForm.auditConditionName = find.name
            this.$set(this.nodeForm,'auditConditionName',find.name)
            // this.defaultFirstLevelId = [res.data[0].id];
          } else {
            // this.$message.error(res.message);
          }
        }
      );
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
            var id = res?.data?.id || '';
            if (id) {
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
            } else { // 如果没有主管，降级，让用户自选审批节点
              if (!this.checkboxPersonGroup || !this.checkboxPersonGroup.length) {
                this.nodeChooseVisible = true;
              } else {
                // if (this.selectFlowType == 'enterprise') {
                //   // 表单流程
                //   this.enterpriseHandleSubmit(true);
                // } else {
                //   // 无表单流程
                //   this.submitFinal(true);
                // }
              }
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
  }
};
</script>
<style lang="scss" scoped>
@import "~@/assets/styles/override-element-ui.scss";
@import "~@/assets/styles/workflow.scss";

.jiaqian{
  color: #FF422E;
  font-weight: bold;
  display: inline-block;
  padding: 2px 8px;
  background: #FFC1BB;
  border-radius: 4px;
  font-size: 12px;
  margin-left: 4px;
}
.checkbox-con {
  min-height: 400px;

  p {
    margin-top: 15px;
  }
}

.check-con {
  padding-bottom: 20px;

  > span {
    margin-right: 10px;
    margin-bottom: 10px;
  }
}

.scroll-bar {
  max-height: 600px;
  overflow: auto;

  &::-webkit-scrollbar-track-piece {
    background: #d3dce6;
  }

  &::-webkit-scrollbar {
    width: 9px;
  }

  &::-webkit-scrollbar-thumb {
    background: #99a9bf;
    border-radius: 20px;
  }
}

.titActiveStart {
  background-color: rgb(87, 106, 149) !important;
}

.titActive {
  background-color: rgb(255, 148, 62) !important;
}

.title.empty-node {
  background-color: #8a8c8e !important;
}

.title.synergy-node {
  background-color: #74905d !important;
}

.condition-branch .titActive,
.manual-branch .condition-branch .titActive,
.parallel-branch .condition-branch .titActive {
  background-color: #15bc83 !important;
}

.parallel-branch .titActive,
.condition-branch .parallel-branch .titActive,
.manual-branch .parallel-branch .titActive {
  background-color: #3296fa !important;
}

.manual-branch .titActive,
.parallel-branch .manual-branch .titActive,
.condition-branch .manual-branch .titActive {
  background-color: #9544f1 !important;
}

.titActiveEnd {
  background-color: rgb(50, 150, 250) !important;
}

.error_tip {
  position: absolute;
  top: 0px;
  right: 0px;
  transform: translate(150%, 0px);
  font-size: 24px;
}

.add-node-popover-body {
  display: flex;
}

.promoter_content {
  padding: 0 20px;
}

.el-form-item--mini.el-form-item {
  margin: 10px 0;
}

.condition_radio {
  padding: 10px 30px;

  ::v-deep .el-radio {
    display: block;
    margin-bottom: 16px;

    .el-radio__label {
      font-size: 14px !important;
    }
  }
}

.promoter_content p {
  padding: 18px 0;
  font-size: 14px;
  line-height: 20px;
  color: #000000;
}

.promoter_person .el-dialog__body {
  padding: 10px 20px 14px 20px;
}

.person_body {
  border: 1px solid #f5f5f5;
  height: 500px;
}

.person_tree {
  padding: 10px 12px 0 8px;
  width: 280px;
  height: 100%;
  border-right: 1px solid #f5f5f5;
}

.person_tree input {
  padding-left: 22px;
  width: 210px;
  height: 30px;
  font-size: 12px;
  border-radius: 2px;
  border: 1px solid #d5dadf;
  // background: url(~@/assets/images/list_search.png) no-repeat 10px center;
  background-size: 14px 14px;
  margin-bottom: 14px;
}

.tree_nav span {
  display: inline-block;
  padding-right: 10px;
  margin-right: 5px;
  max-width: 6em;
  color: #38adff;
  font-size: 12px;
  cursor: pointer;
  // background: url(~@/assets/images/jiaojiao.png) no-repeat right center;
}

.tree_nav span:last-of-type {
  background: none;
}

.person_tree ul,
.has_selected ul {
  height: 420px;
  overflow-y: auto;
}

.person_tree li {
  padding: 5px 0;
}

.person_tree li i {
  float: right;
  padding-left: 24px;
  padding-right: 10px;
  color: #3195f8;
  font-size: 12px;
  cursor: pointer;
  background: url(~@/assets/images/next_level_active.png) no-repeat 10px center;
  border-left: 1px solid rgb(238, 238, 238);
}

.person_tree li a.active + i {
  color: rgb(197, 197, 197);
  background-image: url(~@/assets/images/next_level.png);
  pointer-events: none;
}

.person_tree img {
  width: 14px;
  vertical-align: middle;
  margin-right: 5px;
}

.has_selected {
  width: 276px;
  height: 100%;
  font-size: 12px;
}

.has_selected ul {
  height: 460px;
}

.has_selected p {
  padding-left: 19px;
  padding-right: 20px;
  line-height: 37px;
  border-bottom: 1px solid #f2f2f2;
}

.has_selected p a {
  float: right;
}

.has_selected ul li {
  margin: 11px 26px 13px 19px;
  line-height: 17px;
}

.has_selected li span {
  vertical-align: middle;
}

.has_selected li img:first-of-type {
  width: 14px;
  vertical-align: middle;
  margin-right: 5px;
}

.has_selected li img:last-of-type {
  float: right;
  margin-top: 2px;
  width: 14px;
}

el-radio-group {
  padding: 20px 0;
}

.approver_content {
  padding: 20px;
  border-bottom: 1px solid #f2f2f2;
}

.el-radio {
  line-height: 28px;
  margin-right: 15px;
}

.approver_some .el-radio,
.approver_self_select .el-radio {
  width: 27%;
  margin-bottom: 20px;
}

.copyer_content .el-checkbox {
  margin-bottom: 20px;
}

.el-checkbox__label {
  font-size: 12px;
}

.approver_manager p:first-of-type,
.approver_some p {
  line-height: 19px;
  font-size: 14px;
  margin-bottom: 14px;
}

.approver_manager p {
  line-height: 32px;
}

.approver_manager select {
  width: 420px;
  height: 32px;
  background: rgba(255, 255, 255, 1);
  border-radius: 4px;
  border: 1px solid rgba(217, 217, 217, 1);
}

.approver_manager p.tip {
  margin: 10px 0 22px 0;
  font-size: 12px;
  line-height: 16px;
  color: #f8642d;
}

.approver_self {
  padding: 28px 20px;
}

.selected_list {
  margin-bottom: 20px;
  line-height: 30px;
}

.selected_list span {
  margin-right: 10px;
  padding: 3px 6px 3px 9px;
  line-height: 12px;
  white-space: nowrap;
  border-radius: 2px;
  border: 1px solid rgba(220, 220, 220, 1);
}

.selected_list img {
  margin-left: 5px;
  width: 7px;
  height: 7px;
  cursor: pointer;
}

.approver_self_select h3 {
  margin: 5px 0 20px;
  font-size: 14px;
  font-weight: bold;
  line-height: 19px;
}

.condition_copyer .el-drawer__body .priority_level {
  position: absolute;
  top: 11px;
  right: 30px;
  width: 100px;
  height: 32px;
  background: rgba(255, 255, 255, 1);
  border-radius: 4px;
  border: 1px solid rgba(217, 217, 217, 1);
}

.condition_content p.tip {
  margin: 20px 0;
  width: 510px;
  text-indent: 17px;
  line-height: 45px;
  background: rgba(241, 249, 255, 1);
  border: 1px solid rgba(64, 163, 247, 1);
  color: #46a6fe;
  font-size: 14px;
}

.condition_content ul {
  max-height: 500px;
  overflow-y: scroll;
  margin-bottom: 20px;
}

.condition_content li > span {
  float: left;
  margin-right: 8px;
  width: 70px;
  line-height: 32px;
  text-align: right;
}

.condition_content li > div {
  display: inline-block;
  width: 370px;
}

.condition_content li:not(:last-child) > div > p {
  margin-bottom: 20px;
}

.condition_content li > div > p:not(:last-child) {
  margin-bottom: 10px;
}

.condition_content li > a {
  float: right;
  margin-right: 10px;
  margin-top: 7px;
}

.condition_content li select,
.condition_content li input {
  width: 100%;
  height: 32px;
  background: rgba(255, 255, 255, 1);
  border-radius: 4px;
  border: 1px solid rgba(217, 217, 217, 1);
}

.condition_content li select + input {
  width: 260px;
}

.condition_content li select {
  margin-right: 10px;
  width: 100px;
}

.condition_content li p.selected_list {
  padding-left: 10px;
  border-radius: 4px;
  min-height: 32px;
  border: 1px solid rgba(217, 217, 217, 1);
}

.condition_content li p.check_box {
  line-height: 32px;
}

.condition_list .el-dialog__body {
  padding: 16px 26px;
}

.condition_list p {
  color: #666666;
  margin-bottom: 10px;
}

.condition_list p.check_box {
  margin-bottom: 0;
  line-height: 36px;
}

.delete-condition {
  font-size: 18px;
  cursor: pointer;
  color: red;
  position: absolute;
  right: -24px;
  top: 12px;
}

.condition-wrap {
  margin-bottom: 3px;
  border: 3px dashed#ebeef5;
  padding: 6px;
  /* box-shadow: 0 2px 12px 0 rgb(0 0 0 / 10%); */
  border-radius: 5px;

  .condition-personBox {
    position: relative;
    display: flex;
    flex: 1;
    justify-content: space-between;
    min-height: 20px;
    max-height: 250px;
    min-width: 258px;
    overflow: auto;
    border-radius: 4px;
    padding: 0 5px;
    border: 1px solid #dcdfe6;

    &:hover {
      cursor: pointer;

      .person-iconBox {
        color: #1989fa;
      }
    }

    .person-iconBox {
      width: 20px;
      // display: flex;
      // flex-direction: column;
      // justify-content: center;
      position: fixed;
      right: 48px;
      // top: 20%;
      background: rgba(255, 255, 255, 0.6);

      i {
        font-size: 18px;
        cursor: pointer;
      }
    }
  }
}

::v-deep .el-tag {
  margin-right: 5px;
  margin-bottom: 2px;
}

::v-deep .el-drawer__header button.el-drawer__close-btn {
  display: none;
}

::v-deep .el-drawer__header {
  margin-bottom: 0;
  padding: 14px 0 14px 20px;
  /* border-bottom: 1px solid #f2f2f2; */
  color: #323232;
  font-size: 16px;
}
</style>
