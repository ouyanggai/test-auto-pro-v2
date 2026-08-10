
<!--
 * @Descripttion: 添加节点
 * @Author: zhengzetao
 * @Date: 2021-03-18
-->
<template>
  <div class="add-node-btn-box">
    <div class="add-node-btn">
      <el-popover v-model="visible" placement="right-start">
        <div class="add-node-popover-body">
          <!-- <a class="add-node-popover-item condition" @click="addType(4)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-share" />
              </span>
            </div>
            <p>条件分支</p>
          </a> -->
          <a class="add-node-popover-item parallel" style="flex: 0.5;" @click="addType(5)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-operation" />
              </span>
            </div>
            <p>并行节点</p>
          </a>
          <!-- <a class="add-node-popover-item manual" @click="addType(6)">
            <div class=" item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-promotion" />
              </span>
            </div>
            <p>手动分支</p>
          </a> -->
          <a class="add-node-popover-item approver" style="flex: 0.5;" @click="addType(1)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-user-solid" />
              </span>
            </div>
            <p>审批人</p>
          </a>
          <a class="add-node-popover-item synergy-node" style="flex: 0.5;" @click="addType(2)">
            <div class=" item-wrapper">
              <span class="iconfont">
                <i class="el-icon-edit-outline" />
              </span>
            </div>
            <p>协同</p>
          </a>
          <!-- <a class="add-node-popover-item empty-node" @click="addType(3)">
            <div class=" item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-flag" />
              </span>
            </div>
            <p>空节点</p>
          </a> -->
        </div>
        <button v-show="editType != 3" slot="reference" class="btn" type="button">
          <span class="iconfont"><i class="el-icon-plus" /></span>
        </button>
      </el-popover>
    </div>
  </div>
</template>
<script>
export default {
  props: ['childNodeP', 'editType'],

  data() {
    return {
      visible: false
    };
  },
  methods: {
    addType(type) {
      this.visible = false;
      var nodes = this.getChildNodeParams(type);
      this.$emit('update:childNodeP', nodes);
    },
    getChildNodeParams(type) {
      let data = {};
      switch (type) {
        case 1:
          // 审核人
          data = {
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
            childFlowNodeTemplate: this.childNodeP,
            nodeUserList: [],
            flowNodeAuditConfig: {
              auditType: null,
              type: 'scramble',
              flowNodeDetailConfigList: []
            },
            flowNodeFieldPowerTemplateList: []
          };
          break;
        case 2:
          // 协同
          data = {
            nodeName: '协同',
            error: false,
            type: 'synergy',
            settype: 1,
            selectMode: 0,
            selectRange: 0,
            directorLevel: 1,
            examineMode: 1,
            noHanderAction: 1,
            examineEndDirectorLevel: 0,
            childFlowNodeTemplate: this.childNodeP,
            nodeUserList: [],
            flowNodeAuditConfig: {
              auditType: null,
              type: 'scramble',
              flowNodeDetailConfigList: []
            },
            flowNodeFieldPowerTemplateList: []
          };
          break;
        case 3:
          // 空节点
          data = {
            nodeName: '空节点',
            error: false,
            type: 'empty',
            settype: 1,
            selectMode: 0,
            selectRange: 0,
            directorLevel: 1,
            examineMode: 1,
            noHanderAction: 1,
            examineEndDirectorLevel: 0,
            childFlowNodeTemplate: this.childNodeP,
            nodeUserList: [],
            flowNodeAuditConfig: {
              auditType: null,
              type: 'scramble',
              flowNodeDetailConfigList: []
            },
            flowNodeFieldPowerTemplateList: []
          };
          break;
        case 4:
          // 条件分支
          data = {
            nodeName: '路由',
            type: 'condition',
            branchExecuteType: '',
            childFlowNodeTemplate: this.childNodeP,
            conditionNodes: [{
              nodeName: '条件1',
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
              flowNodeFieldPowerTemplateList: []
              // childFlowNodeTemplate:
            }, {
              nodeName: '条件2',
              error: false,
              type: 'common',
              strategyType: 'change',
              sort: 2,
              conditionList: [],
              nodeUserList: [],
              flowNodeAuditConfig: {
                auditType: null,
                flowNodeDetailConfigList: []
              },
              childFlowNodeTemplate: null
            }]
          };
          break;
        case 5:
          // 并行节点
          data = {
            nodeName: '并行',
            type: 'parallel',
            childFlowNodeTemplate: this.childNodeP,
            parallelNodes: [
              {
                nodeName: '节点1',
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
                  nodeName: '审核人-节点1',
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
                    type: 'scramble',
                    flowNodeDetailConfigList: []
                  },
                  flowNodeFieldPowerTemplateList: []
                }
              },
              {
                nodeName: '节点2',
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
                  nodeName: '审核人-节点2',
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
                    type: 'scramble',
                    flowNodeDetailConfigList: []
                  },
                  flowNodeFieldPowerTemplateList: []
                }
              }
            ]
          };
          break;
        case 6:
          // 手动分支
          data = {
            nodeName: '分支',
            type: 'condition',
            branchExecuteType: 'custom_choose',
            childFlowNodeTemplate: this.childNodeP,
            conditionNodes: [{
              nodeName: '分支1',
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
                  type: 'scramble',
                  flowNodeDetailConfigList: []
                },
                flowNodeFieldPowerTemplateList: []
              }
            }, {
              nodeName: '分支2',
              error: false,
              type: 'common',
              strategyType: 'change',
              sort: 2,
              conditionList: [],
              nodeUserList: [],
              flowNodeAuditConfig: {
                auditType: null,
                flowNodeDetailConfigList: []
              },
              flowNodeFieldPowerTemplateList: [],
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
                  type: 'scramble',
                  flowNodeDetailConfigList: []
                },
                flowNodeFieldPowerTemplateList: []
              }
            }]
          };
          break;
      }
      return data;
    }
  }
};
</script>
<style lang='scss' scoped>
// @import "~@/assets/styles/override-element-ui.scss";
@import "~@/assets/styles/workflow.scss";
</style>
