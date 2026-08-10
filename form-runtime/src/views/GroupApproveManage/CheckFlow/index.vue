<!--
 * @Author: junshao
 * @Date: 2023-03-23 15:41:24
 * @LastEditors: oygdeMac-mini-4.local
 * @LastEditTime: 2026-03-13 16:07:32
 * @Description: 查看流程进度--从实施平台流程设计拷贝过来
-->
<template>
  <div>
    <!-- 审批节点 -->
    <div class="node-wrap" v-if="nodeConfig.type == 'start' || nodeConfig.type == 'common'">
      <div class="node-wrap-box" :class="(nodeConfig.type == 'start' ? 'start-node ' : '')">
        <div>
            <div class="title"
              :class="{ 'titActive': nodeConfig.type == 'start', 'titActive': nodeConfig.type == 'common' }" style="position: relative;">
              <span class="iconfont" v-show="nodeConfig.type == 'common'">
                <i class="el-icon-user-solid"></i>
              </span>
              <span class="editable-title">{{ nodeConfig.nodeName }}</span>
              <!-- <el-tooltip class="item" effect="dark" content="当前节点" placement="right" :manual="true" :value="true" v-if="nodeConfig.id == nextNodeProxyId">
                <span></span>
              </el-tooltip> -->
              <el-tag v-if="checkShouldSkip(nodeConfig)" type="info" effect="dark" style="position: absolute;top:2px;right:5px">跳过</el-tag>
              <el-tag v-else-if="nodeConfig.type == 'start' && !isDraft" :type="'success'" effect="dark" style="position: absolute;top:2px;right:5px">已发送</el-tag>
              <el-tag v-else-if="judgeIsAduited(nodeConfig) == 'pass'" :type="'success'" effect="dark" style="position: absolute;top:2px;right:5px">已审批</el-tag>
              <el-tag v-else-if="judgeIsAduited(nodeConfig) == 'no_pass'" type="info" effect="dark" style="position: absolute;top:2px;right:5px">已驳回</el-tag>
            </div>
          <div slot="reference" class="content" style="cursor: default;" @click="setPerson(1)">
            <div class="text">
              <el-popover placement="right" width="400" trigger="click">
                <div class="node-config-detail">
                    <span>节点名称：</span><span>{{ nodeForm.nodeName }}</span>
                </div>
                <div class="node-config-detail">
                  <span>节点类型：</span>
                  <span v-if="branchType == 'parallel'">并行节点</span>
                  <span v-else-if="branchType == 'custom_choose'">分支节点</span>
                  <span v-else-if="branchType == 'condition'">条件节点</span>
                  <span v-else>普通节点</span>
                </div>
                <div class="node-config-detail" v-if="nodeConfig.type != 'start'">
                  <span>审批方式：</span>
                  <span>{{ nodeForm.type == 'countersign' ? '会签' : '竞签' }}</span>
                </div>
                <div class="node-config-detail" v-if="nodeConfig.type != 'start' && nodeForm.type == 'countersign'">
                  <span>会签人数：</span>
                  <span>{{ nodeForm.countersignNum == -1 ? '所有' : nodeForm.countersignNum }}人</span>
                </div>
                <div class="node-config-detail">
                  <span v-if="nodeConfig.type == 'start'">发起人类型：</span>
                  <span v-else>审批人类型：</span>
                  <span v-if="nodeForm.auditType == 'assign'">选择人员</span>
                  <span v-if="nodeForm.auditType == 'initiator'">发起人自己</span>
                  <span v-if="nodeForm.auditType == 'run_node_choose'">审批人自选</span>
                  <span v-if="nodeForm.auditType == 'role'">选择角色</span>
                  <span v-if="nodeForm.auditType == 'department_supervisor'">发起人部门主管</span>
                  <span v-if="nodeForm.auditType == 'branched_passage_manager'">发起人分管副总</span>
                  <span v-if="nodeForm.auditType == 'company'">项目指定人员</span>
                  <span v-if="nodeForm.auditType == 'company_id'">指定公司</span>
                  <span v-if="nodeForm.auditType == 'department'">指定部门</span>
                  <span v-if="nodeForm.auditType == 'position'">指定岗位</span>
                  <span v-if="nodeForm.auditType == 'level'">指定岗级</span>
                  <span v-if="nodeForm.auditType == 'extendedAttribute'">扩展属性</span>
                  <span v-if="nodeForm.auditType == 'form_person'">指定表单人员</span>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'initiator' || nodeForm.auditType == 'run_node_choose'
                 || nodeForm.auditType == 'assign' || nodeForm.auditType == 'company' || nodeForm.auditType == 'department_supervisor'
                || nodeForm.auditType == 'branched_passage_manager' || nodeForm.auditType == 'form_person'"
                  style="max-height: 250px;overflow: auto;display: flex;">
                  <div>人员：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.personTagList" size="mini" :key="tag.id" :closable="editType != 3"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'company_id'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>公司：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.flowNodeDetailConfigList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'department'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>部门：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.flowNodeDetailConfigList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'level'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>岗级：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.levelNameList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                  <!-- <div style="width:260px">
                    <el-tag size="mini" :disable-transitions="true">
                      {{ nodeForm.auditConditionName }}
                    </el-tag>
                  </div> -->
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'position'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>岗位：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.personTagList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.delay" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>处理期限：</div>
                  <div style="width:260px">
                    {{ nodeForm.delay }}{{ timeUnitFormatter[nodeForm.unit] }}
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.delay && getDeadlineTypeText(nodeForm.deadlineType)">
                  处理期限计算规则：{{ getDeadlineTypeText(nodeForm.deadlineType) }}
                </div>
                <div class="node-config-detail" v-if="nodeForm.isSkip" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>匹配不到人：</div>
                  <div style="width:260px">
                    自动跳过该节点
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'extendedAttribute'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>属性人员：</div>
                  <div style="width:260px">
                    {{ nodeForm.selectAttr.name }}
                    (<template v-if="nodeForm.selectAttr.selectAttrUser.length">
                      <el-tag v-for="tag in nodeForm.selectAttr.selectAttrUser" size="mini" :key="tag.id"
                        :disable-transitions="true">
                        {{ tag.name }}
                      </el-tag>
                    </template>
                    <el-tag v-else size="mini" :disable-transitions="true">未设置</el-tag>
                    )
                    <!-- (<el-tag>{{ nodeForm.selectAttr.selectAttrUser ? nodeForm.selectAttr.selectAttrUser : '未设置' }}</el-tag>) -->
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.checkedRole && nodeForm.checkedRole.id"
                  style="max-height: 250px;overflow: auto;display: flex;">
                  <div style="margin-top: 15px;">审批人员：</div>
                  <el-collapse v-model="activeRoleNames" style="width: 280px;">
                    <el-collapse-item :title="nodeForm.checkedRole.name" :name="1">
                      <el-tag v-for="user in roleUserList" :key="user.id">{{ user.userVo.name }}</el-tag>
                    </el-collapse-item>
                  </el-collapse>
                </div>
                <span slot="reference" style="cursor: pointer;">查看节点配置</span>
              </el-popover>
            </div>
          </div>
        </div>
      </div>
      <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
    </div>
    <!-- 协同--目前和普通审批节点功能一样-考虑未来协同功能改变-单独写一份 -->
    <div class="node-wrap" v-if="nodeConfig.type == 'synergy'">
      <div class="node-wrap-box">
        <div>
          <div class="title synergy-node">
            <span class="iconfont">
              <i class="el-icon-user-solid"></i>
            </span>
            <span class="editable-title">{{ nodeConfig.nodeName }}</span>
            <!-- <el-tooltip class="item" effect="dark" content="当前节点" placement="right" :manual="true" :value="true" v-if="nodeConfig.id == nextNodeProxyId">
                <span></span>
              </el-tooltip> -->
            <i class="anticon anticon-close close" v-if="nodeConfig.type != 'start' && editType != '3'"
              @click="delNode()"></i>
            <el-tag v-if="checkShouldSkip(nodeConfig)" type="info" effect="dark" style="position: absolute;top:2px;right:5px">跳过</el-tag>
            <el-tag v-else-if="judgeIsAduited(nodeConfig) == 'pass'" :type="'success'" effect="dark" style="position: absolute;top:2px;right:5px">已审批</el-tag>
            <el-tag v-else-if="judgeIsAduited(nodeConfig) == 'no_pass'" type="info" effect="dark" style="position: absolute;top:2px;right:5px">已驳回</el-tag>
          </div>
          <div class="content" @click="setPerson(1)" style="cursor:default;padding:0;">
            <div class="text">
              <el-popover placement="right" width="400" trigger="click" >
                <div class="node-config-detail">
                  <span>节点名称：</span><span>{{ nodeForm.nodeName }}</span>
                </div>
                <div class="node-config-detail" v-if="nodeConfig.type != 'start'">
                  <span>审批方式：</span>
                  <span>{{ nodeForm.type == 'countersign' ? '会签' : '竞签' }}</span>
                </div>
                <div class="node-config-detail" v-if="nodeConfig.type != 'start' && nodeForm.type == 'countersign'">
                  <span>会签人数：</span>
                  <span>{{ nodeForm.countersignNum == -1 ? '所有' : nodeForm.countersignNum }}人</span>
                  <!-- <span>{{ nodeForm.countersignNum }}人</span> -->
                </div>
                <div class="node-config-detail">
                  <span>审批人类型：</span>
                  <span v-if="nodeForm.auditType == 'assign'">选择人员</span>
                  <span v-if="nodeForm.auditType == 'initiator'">发起人自己</span>
                  <span v-if="nodeForm.auditType == 'run_node_choose'">审批人自选</span>
                  <span v-if="nodeForm.auditType == 'role'">选择角色</span>
                  <span v-if="nodeForm.auditType == 'department_supervisor'">发起人部门主管</span>
                  <span v-if="nodeForm.auditType == 'branched_passage_manager'">发起人分管副总</span>
                  <span v-if="nodeForm.auditType == 'company'">项目指定人员</span>
                  <span v-if="nodeForm.auditType == 'position'">指定岗位</span>
                  <span v-if="nodeForm.auditType == 'level'">指定岗级</span>
                  <span v-if="nodeForm.auditType == 'extendedAttribute'">扩展属性</span>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'initiator' || nodeForm.auditType == 'run_node_choose' || nodeForm.auditType == 'assign' || nodeForm.auditType == 'company'"
                  style="max-height: 250px;overflow: auto;display: flex;">
                  <div>审批人员：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.personTagList" :key="tag.id" size="medium" :closable="editType != 3"
                      :disable-transitions="false">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'level'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>岗级：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.levelNameList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                  <!-- <div style="width:260px">
                    <el-tag size="mini" :disable-transitions="true">
                      {{ nodeForm.auditConditionName }}
                    </el-tag>
                  </div> -->
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'position'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>岗位：</div>
                  <div style="width:260px">
                    <el-tag v-for="tag in nodeForm.personTagList" size="mini" :key="tag.id"
                      :disable-transitions="true">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
                 <!-- v-if="nodeForm.delay" -->
                <div class="node-config-detail" v-if="nodeForm.delay" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>处理期限：</div>
                  <div style="width:260px">
                    {{ nodeForm.delay }}{{ timeUnitFormatter[nodeForm.unit] }}
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.delay && getDeadlineTypeText(nodeForm.deadlineType)">
                  处理期限计算规则：{{ getDeadlineTypeText(nodeForm.deadlineType) }}
                </div>
                <div class="node-config-detail" v-if="nodeForm.auditType == 'extendedAttribute'" style="max-height: 250px;overflow: auto;display: flex;">
                  <div>属性人员：</div>
                  <div style="width:260px">
                    {{ nodeForm.selectAttr.name }}
                    (<template v-if="nodeForm.selectAttr.selectAttrUser.length">
                      <el-tag v-for="tag in nodeForm.selectAttr.selectAttrUser" size="mini" :key="tag.id"
                        :disable-transitions="true">
                        {{ tag.name }}
                      </el-tag>
                    </template>
                    <el-tag v-else size="mini" :disable-transitions="true">未设置</el-tag>
                    )
                    <!-- (<el-tag>{{ nodeForm.selectAttr.selectAttrUser ? nodeForm.selectAttr.selectAttrUser : '未设置' }}</el-tag>) -->
                  </div>
                </div>
                <div class="node-config-detail" v-if="nodeForm.checkedRole && nodeForm.checkedRole.id"
                  style="max-height: 250px;overflow: auto;display: flex;">
                  <div style="margin-top: 15px;">审批人员：</div>
                  <el-collapse v-model="activeRoleNames" style="width: 280px;">
                    <el-collapse-item :title="nodeForm.checkedRole.name" :name="1">
                      <el-tag v-for="user in roleUserList" :key="user.id">{{ user.userVo.name }}</el-tag>
                    </el-collapse-item>
                  </el-collapse>
                </div>
                <div slot="reference" style="cursor: pointer;padding: 16px;padding-right: 30px;">查看节点配置</div>
              </el-popover>
            </div>
            <i class="anticon anticon-right arrow"></i>
          </div>
        </div>
      </div>
      <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
    </div>
    <!-- 条件分支 -->
    <div class="branch-wrap condition-branch"
      v-if="nodeConfig.type == 'condition' && nodeConfig.branchExecuteType != 'custom_choose'">
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- nodeConfig.conditionNodes：条件节点数据列表 -->
          <div class="col-box" v-for="(item, index) in nodeConfig.conditionNodes" :key="index">
            <div class="condition-node">
              <div class="condition-node-box">
                <div class="auto-judge">
                  <div class="title-wrapper">
                    <template v-if="item.type">
                      <span class="editable-title" v-if="!isInputList[index]">
                        {{ item.nodeName }}
                      </span>
                    </template>
                    <template v-else>
                      <span class="editable-title">
                        {{ item.name }}
                      </span>
                    </template>
                    <!-- <el-tooltip class="item" effect="dark" content="当前节点" placement="right" :manual="true" :value="true" v-if="nodeConfig.id == nextNodeProxyId">
                      <span></span>
                    </el-tooltip> -->
                  </div>
                  <el-popover placement="bottom" width="400" trigger="click" style="background-color: #ddd;">
                    <el-form ref="conditionSelectForm" :model="conditionSelectForm" label-width="80px"
                      :rules="conditionSelectFormRules">
                      <el-form-item label="条件名称" prop="conditionName">
                        <el-input :readOnly='editType == 3' v-model="conditionSelectForm.conditionName" maxlength="15"
                          placeholder="请输入条件名称" autocomplete="off"></el-input>
                      </el-form-item>

                      <el-form-item label="条件值" v-if="conditionSelectForm.conditionSelectList.length">
                        <div
                          v-for="(item, index) in conditionSelectForm.conditionSelectList"
                          :key="index"
                          style="margin-bottom:10px;"
                        >
                          <div style="display:flex;margin-bottom:10px;">
                            <el-select v-model="item.judge" :disabled='editType == 3' placeholder="请选择条件" style="width:200px;margin-right:5px;">
                              <el-option label="大于等于" value="gte" />
                              <el-option label="小于等于" value="lte" />
                              <el-option label="大于" value="gt" />
                              <el-option label="小于" value="lt" />
                              <el-option label="等于" value="eq" />
                              <el-option label="不等于" value="neq" />
                              <el-option
                                label="包含"
                                value="contains"
                              />
                            </el-select>

                            <div
                              v-if="item.btype == 'person'"
                              class="condition-personBox"
                            >
                              <div>
                                <el-tag
                                  v-for="(tag, tagIndex) in item.personList"
                                  :key="tag.id"
                                  size="small"
                                  :disable-transitions="false"
                                >
                                  {{ tag.name }}
                                </el-tag>
                              </div>
                            </div>
                            <el-input v-else :readOnly='editType == 3' v-model="item.bvalue" style="width:200px;" />
                          </div>
                          <div v-if="conditionSelectForm.conditionSelectList.length > 1 && index !== conditionSelectForm.conditionSelectList.length - 1">
                            <el-select :disabled='editType == 3' v-model="item.conditionType" placeholder="请选择其他条件" style="width:182px;">
                              <el-option label="或" value="or" />
                              <el-option label="且" value="and" />
                            </el-select>
                          </div>
                        </div>
                      </el-form-item>
                      <!-- <template >
                        <div v-for="(item, index) in conditionSelectForm.conditionSelectList" :key="index">
                          <el-form-item label="条件值" :prop="`conditionSelectList.${index}.bvalue`" :rules="bRules">
                            <el-input :readOnly='editType == 3' v-model="item.bvalue" maxlength="15"
                              placeholder="请输入条件字段值" autocomplete="off"></el-input>
                          </el-form-item>
                        </div>
                      </template> -->
                    </el-form>
                    <div slot="reference" class="content" @click="setPerson(item.sort)">查看条件</div>
                  </el-popover>
                </div>
                <addNode :childNodeP.sync="item.childFlowNodeTemplate" :editType.sync="editType"></addNode>
              </div>
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate" :isDraft="isDraft"
              :nodeConfig.sync="item.childFlowNodeTemplate" :branchType="'condition'" :fields.sync="fields"
              :editType.sync="editType"  :auditRecordNodeList="auditRecordNodeList" :nextNodeProxyId="nextNodeProxyId"
              :initiatorId="initiatorId" :flowInstanceId="flowInstanceId" :nodeConfigUser="nodeConfigUser" :formData="formData" :companyId="companyId"></NodeWrap>
            <div class="top-left-cover-line" v-if="index == 0"></div>
            <div class="bottom-left-cover-line" v-if="index == 0"></div>
            <div class="top-right-cover-line" v-if="index == nodeConfig.conditionNodes.length - 1"></div>
            <div class="bottom-right-cover-line" v-if="index == nodeConfig.conditionNodes.length - 1"></div>
          </div>
        </div>
        <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
      </div>
    </div>
    <!-- 并行节点 -->
    <div class="branch-wrap parallel-branch" v-if="nodeConfig.type == 'parallel'">
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- <button class="add-branch" v-show="editType != 3" @click="addParallelNode">添加并行</button> -->
          <!-- nodeConfig.parallelNodes：并行节点数据列表 -->
          <div class="col-box" v-for="(item, index) in nodeConfig.parallelNodes" :key="index">
            <div class="condition-node">
              <div class="condition-node-box">
              </div>
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate" :isDraft="isDraft"
              :nodeConfig.sync="item.childFlowNodeTemplate" :parallelTop.sync="nodeConfig" :jsonData.sync="jsonData"
              :fields.sync="fields" :editType.sync="editType" :branchType="'parallel'" :auditRecordNodeList="auditRecordNodeList"
              :nextNodeProxyId="nextNodeProxyId" :formData="formData" :flowInstanceId="flowInstanceId"
              :initiatorId="initiatorId" :nodeConfigUser="nodeConfigUser" :companyId="companyId"
              >
            </NodeWrap>
            <div class="top-left-cover-line" v-if="index == 0"></div>
            <div class="bottom-left-cover-line" v-if="index == 0"></div>
            <div class="top-right-cover-line" v-if="index == nodeConfig.parallelNodes.length - 1"></div>
            <div class="bottom-right-cover-line" v-if="index == nodeConfig.parallelNodes.length - 1"></div>
          </div>
        </div>
        <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
      </div>
    </div>
    <!-- 手动分支 -->
    <div class="branch-wrap manual-branch" v-if="nodeConfig.branchExecuteType == 'custom_choose'">
      <div class="branch-box-wrap">
        <div class="branch-box">
          <!-- nodeConfig.conditionNodes：条件节点数据列表 -->
          <div class="col-box" v-for="(item, index) in nodeConfig.conditionNodes" :key="index">
            <div class="condition-node">
              <div class="condition-node-box">
                <addNode :childNodeP.sync="item.childFlowNodeTemplate" :editType.sync="editType"></addNode>
              </div>
            </div>
            <!-- childFlowNodeTemplate：用于节点的层层递归数据传递 -->
            <NodeWrap v-if="item.childFlowNodeTemplate && item.childFlowNodeTemplate" :isDraft="isDraft"
              :nodeConfig.sync="item.childFlowNodeTemplate" :branchTop.sync="nodeConfig" :jsonData.sync="jsonData"
              :fields.sync="fields" :editType.sync="editType" :branchType="'custom_choose'" :auditRecordNodeList="auditRecordNodeList"
              :nextNodeProxyId="nextNodeProxyId" :formData="formData" :flowInstanceId="flowInstanceId"
              :initiatorId="initiatorId" :nodeConfigUser="nodeConfigUser" :companyId="companyId"
              >
            </NodeWrap>
            <div class="top-left-cover-line" v-if="index == 0"></div>
            <div class="bottom-left-cover-line" v-if="index == 0"></div>
          <div class="top-right-cover-line" v-if="index == nodeConfig.conditionNodes.length - 1"></div>
            <div class="bottom-right-cover-line" v-if="index == nodeConfig.conditionNodes.length - 1"></div>
          </div>
        </div>
        <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
      </div>
      <!-- <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate"
                  :editType.sync="editType"></addNode> -->
    </div>
    <!-- 空节点 -->
    <div class="node-wrap" v-if="nodeConfig.type == 'empty'">
      <div class=" node-wrap-box" :class="nodeConfig.type == 'start' ? 'start-node ' : ''">
        <div>
          <div class="title empty-node">
            <span class="iconfont">
              <i class="el-icon-user-solid"></i>
            </span>
            <span class="editable-title">{{
              nodeConfig.nodeName
            }}</span>
          </div>
          <div class="content" style="cursor: default;">
            空节点
          </div>
        </div>
      </div>
      <addNode :childNodeP.sync="nodeConfig.childFlowNodeTemplate" :editType.sync="editType"></addNode>
    </div>
    <NodeWrap v-if="nodeConfig.childFlowNodeTemplate && nodeConfig.childFlowNodeTemplate" :isDraft="isDraft"
      :nodeConfig.sync="nodeConfig.childFlowNodeTemplate" :jsonData.sync="jsonData" :fields.sync="fields"
      :editType.sync="editType"  :auditRecordNodeList="auditRecordNodeList" :nextNodeProxyId="nextNodeProxyId"
      :initiatorId="initiatorId" :flowInstanceId="flowInstanceId" :nodeConfigUser="nodeConfigUser" :formData="formData" :companyId="companyId">
    </NodeWrap>
  </div>
</template>
<script>
import Api from '@/api';
import addNode from './AddNode.vue';
export default {
  name: 'NodeWrap',
  // props: ['nodeConfig', 'jsonData', 'fields', 'companyPersonList', 'editType', 'parallelTop', 'branchTop', 'branchType', 'auditRecordNodeList', 'isDraft'],
  props: ['nodeConfig', 'jsonData', 'fields', 'editType', 'parallelTop', 'branchTop', 'branchType', 'auditRecordNodeList',
  'isDraft','nextNodeProxyId','initiatorId','flowInstanceId','nodeConfigUser','propAllUserList','formData','companyId','dialogVisible'],
  // 'isTried'
  components: {
    addNode
    // PersonSelectDialog,
    // SelectRole
  },
  data() {
    return {
      // 我的
      formLabelWidth: '130px',
      nodeForm: {
        nodeName: '', // 节点名称
        nodeDesc: '', // 节点描述
        nodeFieldPower: [], // 字段权限
        editData: {},
        auditType: 'assign', // 现在类型分为公司和发起人自己
        type: 'scramble',
        countersignNum: 1,
        deadlineType: '',
        checkedRole: {},
        executeCompanyIds: [], // 公司权限
        personTagList: [],
        selectAttr: {
          name: '',
          id: '',
          selectAttrUser: [],
        },
      },
      // dialogVisible: false,
      companyList: [], // 权限列表
      conditionSelectForm: {
        conditionSelectList: [],
        conditionName: ''
      },
      selectConditionKey: '',
      nodeWrapTitle: '定义节点字段',
      nodeFormRules: {
        nodeName: [{ required: true, message: '请输入节点名称', trigger: 'blur' }],
        executeCompanyIds: [{ required: true, message: '请选择谁有权限', trigger: 'change' }]
      },
      conditionSelectFormRules: {
        conditionName: [{ required: true, message: '请输入条件名称', trigger: 'blur' }]
        // bvalue: [{ required: true, message: '请输入条件字段值', trigger: 'blur' }]
      },
      bRules: [{ required: true, message: '请输入条件字段值', trigger: 'blur' }],
      conditionCurFieldObj: {},
      id: '',
      checkAll: false,
      checkboxGroup: [],
      radioRole: '',
      checkboxData: [],
      checkboxRersonGroup: [],
      dialogVisibleNodeAttr: false,
      dialogVisibleFlowRole: false,
      dialogVisibleFlowRerson: false,
      editData: {},
      isClick: false,
      flowNodeAuditConfig: {
        auditType: 'assign',
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
      flowRoleSelectVisible: false,
      activeRoleNames: [],
      roleUserList: [],
      companyPersonList:[],

      allUserList:[],
      timeUnitFormatter:{
        MINUTES:'分钟',
        HOURS:'小时',
        DAYS:'天',
      },
      deadlineTypeFormatter: {
        work_day: '按工作日计算',
        natural_day: '按自然日计算',
      },
    };
  },
  // },
  async created() {

    // if (!this.allUserList.length) {
    //   this.allUserList = await this.getCompanyPersonList(this.$store.state.user.companyId);
    // }
  },
  mounted() {
    console.log('initiatorId-查看流程', this.initiatorId)
    console.log('this.companyId-查看流程',this.companyId)
    // console.log('this.companyId-查看流程',this.companyId)
  },
  methods: {
    getDeadlineTypeText(deadlineType) {
      return this.deadlineTypeFormatter[deadlineType] || '';
    },
    // 获取选中角色下的用户
    getRoleUser() {
      this.$axios.post(
        Api.schedule.getRoleUserList,
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
    // 点击节点打开配置弹窗，条件节点和审批节点
    async setPerson(sort) {
      console.log('setPerson')
      console.log('nodeForm',this.nodeForm)
      // 数据回显处理
      const { type, conditionNodes,id } = this.nodeConfig;
      const { flowNodeFieldPowerTemplateList } = this.nodeConfig;
      flowNodeFieldPowerTemplateList && flowNodeFieldPowerTemplateList.forEach(item => {
        this.$set(this.editData, item.formFieldTemplateEnglishName, true);
      });
      this.checkboxData = [];
      this.checkboxData.length = 0;
      if (type == 'condition') {
        // 条件字段数据
        this.conditionSelectForm.conditionSelectList = [];
        if (sort < 1) {
          this.sort = sort;
        } else {
          this.sort = sort - 1;
        }
        this.conditionDrawer = true;

        let allUserList = await this.getCompanyPersonList(this.$store.state.user.companyId);
        conditionNodes[this.sort].conditionList.forEach(item => {
          const list = [];
          // const fieldTreeObj = this.fieldTreeList.find(
          //   x => x.dictValue == item.fieldaName
          // ); // 在字段列表中查询出来的当前字段属性对象
          // item.valueType = fieldTreeObj && fieldTreeObj.valueType;
          // item.valueName = fieldTreeObj && fieldTreeObj.dictLabel;
          if (item.btype && item.btype == 'person') {
            item.bvalue.split(',').forEach(val => {
              const personInfoItem = allUserList.find(
                x => x?.id == val
              );
              list.push(personInfoItem);
            });
            item.personList = list.filter(Boolean);
          }
        });
        console.log('conditionNodes[this.sort].conditionList',conditionNodes[this.sort].conditionList)
        // return;

        this.conditionSelectForm.conditionName = conditionNodes[this.sort].nodeName || conditionNodes[this.sort].name;
        this.conditionSelectForm.conditionSelectList = conditionNodes[this.sort].conditionList;
        this.selectConditionKey = (this.conditionSelectForm.conditionSelectList[0] && this.conditionSelectForm.conditionSelectList[0].fieldaName) || '';
        // this.conditionCurFieldObj = this.fields.find(item => item.englishName == this.selectConditionKey) || {};
      } else {
        console.log('nodeForm111',this.nodeForm);
        // 节点数据
        this.flowNodeAuditConfig = this.nodeConfig.flowNodeAuditConfig;
        console.log('this.flowNodeAuditConfig.auditType',this.flowNodeAuditConfig.auditType)
        const auditTypeCopy = this.flowNodeAuditConfig.auditType || '';
        console.log(111111,this.flowNodeAuditConfig)
        var list = [];
        if (auditTypeCopy == 'assign' || auditTypeCopy == 'company' /**项目指定的人员*/) {
          //获取公司列表
          let promiseArr = []
          this.flowNodeAuditConfig.flowNodeDetailConfigList.forEach(el=>{
            if(el.auditDetailType == 'company'){
              promiseArr.push(this.getCompanyPersonList(el.bizId))
            }
          })
          if(!promiseArr.length){
            promiseArr.push(this.getCompanyPersonList(this.$store.state.user.companyId))
          }
          let companyPersonList = []
          Promise.all(promiseArr).then(res=>{
            res.forEach(item=>{
              companyPersonList = companyPersonList.concat(item)
            })
            // 指定人员-人员列表回显
            this.flowNodeAuditConfig.flowNodeDetailConfigList.forEach(item => {
              if(auditTypeCopy == 'assign' || (auditTypeCopy == 'company' && item.auditDetailType != "company")){
                const personInfoItem = companyPersonList.find(
                  x => x.id == item.bizId
                );
                if (personInfoItem) {
                  list.push({
                    bizId: item.bizId,
                    id: item.bizId,
                    name: personInfoItem.name
                  });
                }
              }
            });
          })

        } else if (auditTypeCopy == 'role') {
          // 选择角色-角色人员回显--目前只选择一种审批角色
          if (this.flowNodeAuditConfig?.flowNodeDetailConfigList[0]?.bizId) {
            this.setRole(this.flowNodeAuditConfig.flowNodeDetailConfigList[0].bizId || '');
          }
        }else if(auditTypeCopy == 'department_supervisor' || auditTypeCopy == 'branched_passage_manager'){
          if(this.initiatorId){
            this.getSuperVisorOrLeader(auditTypeCopy).then(res=>{
              let data = res?.data || {}
              list.push({
                bizId: data.id,
                id: data.id,
                name: data.name
              })
            })
          }
        }else if(auditTypeCopy == 'company_id'){
          //增加公司得选项
          this.$set(this.nodeForm,'flowNodeDetailConfigList',this.flowNodeAuditConfig.flowNodeDetailConfigList)
        }else if(auditTypeCopy == 'level'){
          this.setJobTitleName(this.flowNodeAuditConfig.flowNodeDetailConfigList)
          // this.setJobTitleName(this.flowNodeAuditConfig.auditCondition)
        }else if(auditTypeCopy == 'position'){
          list = this.flowNodeAuditConfig.flowNodeDetailConfigList.map(item=>{
            return {
              id:item.id,
              name:item.name
            }
          })
          // this.setJobTitleName(this.flowNodeAuditConfig.auditCondition)
        } else if(auditTypeCopy == 'extendedAttribute'){
          console.log('扩展属性')
          console.log('this.companyId',this.companyId)
          console.log(2,this.flowNodeAuditConfig.flowNodeDetailConfigList)

          this.nodeForm.selectAttr.id = this.flowNodeAuditConfig.flowNodeDetailConfigList[0]['bizId']
          this.nodeForm.selectAttr.name = this.flowNodeAuditConfig.flowNodeDetailConfigList[0]['name']
          this.setAttrPersonName(this.nodeForm.selectAttr)
          console.log('this.nodeForm-扩展属性',this.nodeForm)
        } else if(auditTypeCopy == 'department'){
          console.log('选择部门')
          console.log(2,this.flowNodeAuditConfig.flowNodeDetailConfigList)
          this.$set(this.nodeForm,'flowNodeDetailConfigList',this.flowNodeAuditConfig.flowNodeDetailConfigList)
          // let userList = await this.getDepartPersonList();
          // console.log('userList',userList)
        } else if(auditTypeCopy == 'form_person'){ // 后面等后端接口优化后，还要添加范围人员未选择情况的人员显示
          console.log('指定人员')
          console.log(1,this.formData)
          console.log(2,this.flowNodeAuditConfig)
          let realField = this.flowNodeAuditConfig.formPersonFields.replace('__formPersonId','');
          let realFieldValue;
          // return;
          // let realFieldValue = this.formData[realField] ? JSON.parse(this.formData[realField]) : {id: '', name: '未指定人员'};
          if (this.formData[realField] || this.formData[this.flowNodeAuditConfig.formPersonFields]) {
            console.log(11111111,this.flowNodeAuditConfig.formPersonFields)
            if (Array.isArray(this.formData[this.flowNodeAuditConfig.formPersonFields])) {
              realFieldValue = JSON.parse(this.formData[realField])?.flowList || JSON.parse(this.formData[realField]);
              list = realFieldValue.map(x=> {
                return{
                  id: x.id, name: x.name
                }
              })
            } else {
              realFieldValue = JSON.parse(this.formData[realField]);
              list = [{
                id: realFieldValue.id,
                name: realFieldValue.name
              }];
            }
          } else {
            realFieldValue = {id: '', name: '未指定人员'};
          }
          // console.log(3,realField)
          // console.log(4, realFieldValue)
          // list = [{
          //   id: realFieldValue.id,
          //   name: realFieldValue.name
          // }];


          // let nodeObj = this.nodeConfigUser[id]
          // if(nodeObj){
          //   let userList = nodeObj.userList || []
          //   list = userList.map(item=>{
          //     return {
          //       id: item.phone,
          //       name: item.name
          //     }
          //   })
          // }
        } else if(auditTypeCopy == 'initiator'){
          let nodeObj = this.nodeConfigUser[id]
          if(nodeObj){
            let userList = nodeObj.userList || []
            list = userList.map(item=>{
              return {
                id: item.phone,
                name: item.name
              }
            })
          }
        } else if (auditTypeCopy == 'run_node_choose') {
          console.log('审批范围')
          console.log('this.flowNodeAuditConfig.auditType',this.flowNodeAuditConfig.auditType)
          console.log(1,this.flowNodeAuditConfig)
          // console.log(1,this.flowNodeAuditConfig.nodeAuditScopeList)
          console.log(2,this.nodeConfigUser[id])
          console.log(3,this.nodeConfig)
          // return;
          if (this.nodeConfigUser[id]) { // 当前审核人直接显示名称
            let nodeObj = this.nodeConfigUser[id]
            if(nodeObj){
              let userList = nodeObj.userList || []
              list = userList.map(item=>{
                return {
                  id: item.phone,
                  name: item.name
                }
              })
            }
          } else { // 不是当前审核人显示范围内的所有人员---后面等后端接口优化后，还要添加范围人员未选择情况的人员显示
            if (this.flowNodeAuditConfig.nodeAuditScopeList && this.flowNodeAuditConfig.nodeAuditScopeList.length) {
              let rangType = this.flowNodeAuditConfig.nodeAuditScopeList[0]['type'];
              let ids = this.flowNodeAuditConfig.nodeAuditScopeList.map(x=>x.bizId);
              if (rangType == 'personnel') { // 这个代码好像没必要25.1.10
                this.allUserList = await this.getCompanyPersonList(this.$store.state.user.companyId);
                for (var i=0;i<ids.length;i++){
                  let item = ids[i];
                  let result = this.allUserList.find(x=>x.id == item);
                  list.push(result);
                }
              } else {
                const typeMapList = {
                  'position': 'DUTY',
                  'company': 'COMPANY',
                  'department': 'DEPT',
                  'role': 'ROLE',
                }
                list = await this.getUserListByDepartId(ids,typeMapList[rangType]);
              }
            } else {
              this.$set(this.nodeForm,'auditType',this.flowNodeAuditConfig.auditType)
              let result = await this.findAllAuditConfig(this.nodeConfig.id);
              console.log('result',result)
              let obj = {};
              if (result) {
                let nameArr = result[this.nodeConfig.id]['flowNodeDetailConfigList'];
                list = nameArr.map(x=>{
                  return {
                    id: x.id,
                    name: x.name
                  }
                });
                console.log('nameArr',nameArr)
                console.log('list',list)
              }

            }
          }
        }
        console.log('this.nodeConfig111',this.nodeConfig)
        this.nodeForm.personTagList = list;
        this.nodeForm.auditType = this.flowNodeAuditConfig.auditType || '';
        this.nodeForm.type = this.flowNodeAuditConfig.type || '';
        this.nodeForm.countersignNum = this.flowNodeAuditConfig.countersignNum || 1;
        // this.dialogVisible = true;
        this.nodeForm.nodeName = this.nodeConfig.nodeName || '';
        this.nodeForm.delay = this.nodeConfig.delay || '';
        this.nodeForm.unit = this.nodeConfig.unit || '';
        this.nodeForm.deadlineType = this.nodeConfig.deadlineType || '';
        this.nodeForm.isSkip = this.nodeConfig.isSkip;
        console.log('this.nodeForm33445556',this.nodeForm)
      }
    },
    findAllAuditConfig(nodeId) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
         Api.approveManage.findAllAuditConfig, {
          data: {
            id: this.flowInstanceId
          },
          nodeIds: [nodeId]
        },
        res => {
          if (res.isSuccess) {
            resolve(res.data)
          } else {
          }
        })
      })
    },
    // 根据部门id查询用户列表
    getUserListByDepartId(ids,type) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          '/web/user/api/user/getUserVosByBizIds', {
          data: {
            queryTypeEnum: type,
            bizIds: ids
          }
        },
        res => {
          if (res.isSuccess) {
            resolve(res.data)
          } else {
          }
        })
      })
    },
    // 获取属性下的人员
    setAttrPersonName(selectAttr) {
      this.$axios.post(
        '/web/user/api/expandAttr/findUsersByExpandAttr',
        // '/web/user/api/expandAttr/findUserByExpandAttr',
        {
          data: {
            'id': selectAttr.id, // 扩展属性id
            'workflowCreatorCompanyId': this.companyId, // 公司id
            'workflowCreatorId': this.initiatorId // 流程发起者id
          }
        },
        res => {
          if (res.isSuccess) {
            this.$set(selectAttr,'selectAttrUser',res?.data || [])
            // this.$set(selectAttr,'selectAttrUser',res.data && res.data[0]['name'])
            console.log('this.nodeForm-扩展属性2',this.nodeForm)
          } else {
            // this.$message.error(res.message);
          }
        }
      );
    },
    setJobTitleName(flowNodeDetailConfigList) {
      console.log('setJobTitleName', flowNodeDetailConfigList);
      this.$axios.post(
        Api.postInfo.dutyLevel,
        {
          data: {}
        },
        res => {
          if (res.isSuccess) {
            let list = res?.data || [];
            console.log('list',list)
            // let find = list.find(item=>item.id == auditCondition)
            // console.log('find',find)
            let arr = [];
            flowNodeDetailConfigList.forEach(item=>{
              let find = list.find(x=>x.id == item.bizId)
              if(find){
                arr.push({
                  name:find.name,
                  id:find.id,
                })
              }
            })
            this.$set(this.nodeForm,'levelNameList',arr)
            // this.$set(this.nodeForm,'auditConditionName',find.name)

            // this.nodeForm.auditConditionName = find.name
            // this.defaultFirstLevelId = [res.data[0].id];
          } else {
            // this.$message.error(res.message);
          }
        }
      );
    },
    //获取发起人主管或者副总
    getSuperVisorOrLeader(nodeAuditType) {
      const url =
        nodeAuditType == "department_supervisor"
          ? Api.schedule.getSupervisor
          : Api.schedule.getDeptLeader;
      return this.$axios.post(
        url,
        {
          data: {
            id: this.initiatorId, // 发起人id
          },
        }
        // ,
        // (res) => {
        //   if (res.isSuccess) {
        //     var id = res?.data?.id || "";
        //     console.log('res',res)
        //   }
        // }
      );
    },
    getDepartPersonList(departIds) {
      return new Promise((resolve,reject)=>{
        const param = {
          data: {
            queryTypeEnum: "DEPT",
            bizIds: departIds
          },
        };
        this.$axios.post(
          Api.user.getUserVosByBizIds,
          param,
          res => {
            this.loading = false;
            if (res.isSuccess) {
              resolve(res.data)
            } else {
              this.$message.error(res.message);
              reject()
            }
          }
        );
      })
    },
    getCompanyPersonList(companyId) {
      return new Promise((resolve,reject)=>{
        const param = {
          data: {
            companyId
          },
          pagination: true,
          pages: 1,
          size: 1000
        };
        this.$axios.post(
          Api.user.findByCompanyIdUserList,
          param,
          res => {
            this.loading = false;
            if (res.isSuccess) {
              resolve(res.data.dataList)
            } else {
              this.$message.error(res.message);
              reject()
            }
          }
        );
      })
    },
    // 角色人员回显
    setRole(configRoleId) {
      this.$axios.post(
        Api.schedule.getRoleList,
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            scope: 'invest'
          },
          platformCode: '999999'
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
    getLatestBatchNo(list) {
      if (!Array.isArray(list) || !list.length) {
        return '';
      }
      let latest = list[0];
      for (let i = 1; i < list.length; i++) {
        const item = list[i];
        if (item.createDate && (!latest.createDate || item.createDate > latest.createDate)) {
          latest = item;
        }
      }
      return latest.batchNo || '';
    },
    getEligibleCount(nodeCfg) {
      if (!nodeCfg) {
        return 0;
      }
      const nodeId = nodeCfg.id || '';
      if (nodeId && this.nodeConfigUser && this.nodeConfigUser[nodeId] && Array.isArray(this.nodeConfigUser[nodeId].userList)) {
        const list = this.nodeConfigUser[nodeId].userList || [];
        if (list.length) {
          return list.length;
        }
      }
      if (!nodeCfg.flowNodeAuditConfig) {
        return 0;
      }
      const ac = nodeCfg.flowNodeAuditConfig;
      const t = ac.auditType || '';
      const list = Array.isArray(ac.flowNodeDetailConfigList) ? ac.flowNodeDetailConfigList : [];
      if (list.length) {
        return list.length;
      }
      if (t === 'initiator' || t === 'department_supervisor' || t === 'branched_passage_manager' || t === 'role' || t === 'form_person' || t === 'extendedAttribute') {
        return 1;
      }
      return 0;
    },
    judgeIsAduited(nodeConfigOrId) {
      if (this.isDraft) {
        return false;
      }
      let nodeId = '';
      if (typeof nodeConfigOrId === 'string') {
        nodeId = nodeConfigOrId;
      } else if (nodeConfigOrId) {
        nodeId = (this.flowInstanceId && nodeConfigOrId.id) || (nodeConfigOrId.flowNodeAuditConfig && nodeConfigOrId.flowNodeAuditConfig.nodeTemplateId) || nodeConfigOrId.id || '';
      }
      if (!nodeId) {
        return false;
      }
      const latestBatchNo = this.getLatestBatchNo(this.auditRecordNodeList);
      const recordList = latestBatchNo ? this.auditRecordNodeList.filter(item => item.batchNo === latestBatchNo) : this.auditRecordNodeList;
      const nodeRecords = recordList.filter(x => x.flowNodeProxyId == nodeId);
      const hasReject = nodeRecords.some(x => x.auditStatus === 'no_pass');
      if (hasReject) {
        return 'no_pass';
      }
      const nodeCfgObj = typeof nodeConfigOrId === 'object' ? nodeConfigOrId : null;
      const isCountersign = !!(nodeCfgObj && nodeCfgObj.flowNodeAuditConfig && nodeCfgObj.flowNodeAuditConfig.type === 'countersign');
      if (isCountersign) {
        const passCount = nodeRecords.filter(x => x.auditStatus === 'pass').length;
        const ac = nodeCfgObj.flowNodeAuditConfig || {};
        const required = ac.countersignNum === -1 ? this.getEligibleCount(nodeCfgObj) : (ac.countersignNum || 1);
        if (required <= 0) {
          return false;
        }
        if (passCount >= required) {
          return 'pass';
        }
        return false;
      }
      let lastRecord = null;
      for (let i = 0; i < nodeRecords.length; i++) {
        const item = nodeRecords[i];
        if (!lastRecord || (item.createDate && item.createDate > lastRecord.createDate)) {
          lastRecord = item;
        }
      }
      if (lastRecord) {
        return lastRecord.auditStatus;
      }
      return false;
    },
    // 检查节点是否应该显示"自动跳过"
  // 当节点有isSkip=true，且该节点没有审批记录，且后续节点已通过时，显示自动跳过
  checkShouldSkip(nodeConfig) {
    // 如果是草稿状态不判断
    if (this.isDraft) {
      return false;
    }
    // 检查当前节点是否设置了isSkip（表示可以跳过）
    if (!nodeConfig || !nodeConfig.isSkip) {
      return false;
    }
    // 关键：检查当前节点是否有审批记录
    // 如果当前节点有审批记录（已审批或已驳回），说明没有跳过，直接返回false
    const currentAuditStatus = this.judgeIsAduited(nodeConfig);
    if (currentAuditStatus === 'pass' || currentAuditStatus === 'no_pass') {
      return false;
    }
    // 检查后续节点（childFlowNodeTemplate）是否已通过
    const childNode = nodeConfig.childFlowNodeTemplate;
    if (!childNode) {
      return false;
    }
    // 递归检查后续节点是否已通过（处理连续跳过的情况）
    const checkChildPassed = (node) => {
      if (!node) return false;
      // 如果子节点也是可跳过节点
      if (node.isSkip) {
        // 检查这个节点是否有审批记录
        const nodeAuditStatus = this.judgeIsAduited(node);
        // 如果这个可跳过节点有审批记录，说明没跳过
        if (nodeAuditStatus === 'pass' || nodeAuditStatus === 'no_pass') {
          return false;
        }
        // 继续检查其子节点
        return checkChildPassed(node.childFlowNodeTemplate);
      }
      // 检查子节点是否已通过审批
      const auditStatus = this.judgeIsAduited(node);
      return auditStatus === 'pass';
    };
    return checkChildPassed(childNode);
  }
  },

};
</script>
<style lang='scss' scoped>
@import "~@/assets/styles/override-element-ui.scss";
@import "~@/assets/styles/workflow.scss";

.checkbox-con {
  min-height: 400px;

  p {
    margin-top: 15px;
  }
}

.check-con {
  padding-bottom: 20px;

  >span {
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

.condition_content .el-button,
.copyer_content .el-button,
.approver_self_select .el-button,
.promoter_content .el-button,
.approver_content .el-button {
  // margin-bottom: 20px;
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

.el-radio {
  line-height: 28px;
  margin-right: 15px;
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

.person_tree li a.active+i {
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

.condition_content,
.copyer_content,
.approver_self_select,
.approver_manager,
.approver_some {
  // padding: 20px 20px 0;
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

.condition_content li>span {
  float: left;
  margin-right: 8px;
  width: 70px;
  line-height: 32px;
  text-align: right;
}

.condition_content li>div {
  display: inline-block;
  width: 370px;
}

.condition_content li:not(:last-child)>div>p {
  margin-bottom: 20px;
}

.condition_content li>div>p:not(:last-child) {
  margin-bottom: 10px;
}

.condition_content li>a {
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

.condition_content li select+input {
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

::v-deep .el-collapse-item__header {
  width: 282px;
}

.node-config-detail {
  margin: 5px;
}

</style>
