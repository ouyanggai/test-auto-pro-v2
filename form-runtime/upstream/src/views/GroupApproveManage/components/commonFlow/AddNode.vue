
<!--
 * @Descripttion: 添加节点
 * @Author: zhengzetao
 * @Date: 2021-03-18
-->
<template>
  <div class="add-node-btn-box">
    <div class="add-node-btn" :style="{visibility: isVisible ? 'visible' : 'hidden'}">
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
          <a class="add-node-popover-item parallel" @click="addType(5)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-operation" />
              </span>
            </div>
            <p>并行节点</p>
          </a>
          <a class="add-node-popover-item parallel" @click="addType(7)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-operation" />
              </span>
            </div>
            <p>与下一节点并行</p>
          </a>
          <!-- <a class="add-node-popover-item manual" @click="addType(6)">
            <div class=" item-wrapper">
              <span class="iconfont">
                <i class="el-icon-s-promotion" />
              </span>
            </div>
            <p>手动分支</p>
          </a> -->
          <a class="add-node-popover-item approver" @click="addType(1)">
            <div class="item-wrapper">
              <span class="iconfont">
                <i class="el-icon-user-solid" />
              </span>
            </div>
            <p>审批人</p>
          </a>
          <a class="add-node-popover-item synergy-node" @click="addType(2)">
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
  props: ['childNodeP', 'editType','isVisible','nodeConfig'],

  data() {
    return {
      visible: false
    };
  },
  mounted() {
    // console.log('nodeConfig-addNode',this.nodeConfig)
  },
  methods: {
    addType(type) {
      this.visible = false;
      console.log('type',type)
      console.log('type==7',type == 7)
      if (type == 7) { // 与下一节点并行按钮
        var nodes = this.getChildNodeParams(type);
        console.log('nodes',nodes)
        if (nodes) {
          this.$emit('update:childNodeP', nodes);
        } else {
          this.$message.warning('无法与下一节点并行')
        }
      } else {
        console.log('直接加签')
        var nodes = this.getChildNodeParams(type);
        console.log('nodes',nodes)
        this.$emit('update:childNodeP', nodes);
      }
    },
    getChildNodeParams(type) {
      // 前端自定义新加了一个字段currentDelete，用来控制加签的节点是否可以删除和配置选项
      let data = {};
      switch (type) {
        case 1:
          console.log('this.nodeConfig1',this.nodeConfig)
          console.log('this.childNodeP1',this.childNodeP)
          // 审核人
          data = {
            currentDelete:true,
            styleType:true,
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
            flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
            tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
            // flowNodeFieldPowerTemplateList: []
          };
          break;
        case 2:
          // 协同
          data = {
            currentDelete:true,
            styleType:true,
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
            flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
            tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
            // flowNodeFieldPowerTemplateList: []
          };
          break;
        case 7:
          // 与下一节点并行
          console.log('this.nodeConfig',this.nodeConfig)
          console.log('this.childNodeP',this.childNodeP)
          if (!this.childNodeP){ // 最后一个节点不能与下一节点并行
            data = null;
            break;
          }
          // console.log('下一节点人员配置',this.childNodeP.flowNodeAuditConfig.flowNodeDetailConfigList)
          if (this.childNodeP.parallelNodes) { // 如果下一个节点是并行节点
            let index = this.childNodeP.parallelNodes.length + 1;
            this.childNodeP.parallelNodes.push({
              nodeName: '节点'+index,
              error: false,
              type: 'common',
              strategyType: 'change',
              sort: Number(index),
              conditionList: [],
              nodeUserList: [],
              flowNodeAuditConfig: {
                auditType: null,
                flowNodeDetailConfigList: []
              },
              flowNodeFieldPowerTemplateList: [],
              childFlowNodeTemplate: {
                currentDelete:true,
                styleType:true,
                nodeName: '审核人-节点'+index,
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
                flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
                tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
              }
            })
            data = this.childNodeP;
          } else if (this.childNodeP.branchNodes && (this.childNodeP.nodeName == '分支' || this.childNodeP.nodeName == '路由')){
            console.log('进来手动分支和路由')
             // 如果下一节点是条件分支或手动分支，不能点击与下一节点并行的按钮
            data = null;
          } else { // 单个审核节点
            data = {
              currentDelete:true,
              styleType:true,
              nodeName: '并行',
              type: 'parallel',
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
                    currentDelete:this.childNodeP.currentDelete || false,
                    styleType:this.childNodeP.styleType || false,
                    nodeName: this.childNodeP.nodeName,
                    // nodeName: '审核人-节点1',
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
                    isSkip: this.childNodeP.isSkip, // 是否跳过
                    flowNodeAuditConfig: this.childNodeP.flowNodeAuditConfig,
                    flowNodeFieldPowerTemplateList: this.childNodeP.flowNodeFieldPowerTemplateList,
                    tempStorageFieldConfig:this.childNodeP.flowNodeFieldPowerTemplateList
                  }
                },
                {
                  nodeName: '节点2',
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
                    currentDelete:true,
                    styleType:true,
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
                    flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
                    tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
                  }
                }
              ],
              childFlowNodeTemplate: this.childNodeP?.childFlowNodeTemplate || null,
            };
          }

          break;
        case 3:
          // 空节点
          data = {
            currentDelete:true,
            styleType:true,
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
            flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
            tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
            // flowNodeFieldPowerTemplateList: []
          };
          break;
        case 4:
          // 条件分支
          data = {
            currentDelete:true,
            styleType:true,
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
            currentDelete:true,
            styleType:true,
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
                  currentDelete:true,
                  styleType:true,
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
                  flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
                  tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
                  // flowNodeFieldPowerTemplateList: []
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
                  currentDelete:true,
                  styleType:true,
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
                  flowNodeFieldPowerTemplateList: this.nodeConfig.flowNodeFieldPowerTemplateList,
                  tempStorageFieldConfig:this.nodeConfig.flowNodeFieldPowerTemplateList
                  // flowNodeFieldPowerTemplateList: []
                }
              }
            ]
          };
          break;
        case 6:
          // 手动分支
          data = {
            currentDelete:true,
            styleType:true,
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
