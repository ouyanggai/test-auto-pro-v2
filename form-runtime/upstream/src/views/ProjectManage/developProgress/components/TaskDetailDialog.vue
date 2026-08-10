<!--
 * @Descripttion: 审核任务详情
 * @Author: zhengzetao
 * @Date: 2022-4-15
-->
<template>
  <el-dialog :visible="visible" :title="'查看任务'" width="80%" :close-on-click-modal="false" @close='handleClose'
    append-to-body>
    <div class="task-container">
      <div :class="'append'">
        <div style="padding: 10px;background: #fff;">
          <h3 style="margin-bottom: 10px">基本信息</h3>
          <div class="info-wrap">
            <div class="info-item">
              <div class="info-item-title">任务名称</div>
              <div>{{ basicInfoData ? basicInfoData.name : '' }}</div>
            </div>
            <div class="info-item">
              <div class="info-item-title">关联项目</div>
              <div>
                <el-select v-model="assoicaProject.id" placeholder="请选择项目" disabled style="width: 200px">
                  <el-option v-for="(item) in assoicaProjectList" :label="item.name" :value="item.id" :key="item.id">
                  </el-option>
                </el-select>
              </div>
            </div>
            <div class="info-item">
              <div class="info-item-title">完成状态</div>
              <div></div>
              <div>{{ basicInfoData.finishStatus | filtersFinishStatus }}</div>
            </div>

            <!-- 印章 -->
            <div style="margin-bottom: 20px;position: relative;">
              <div class="seal" :style="{ borderColor: getSealColor(basicInfoData.planExamineStatus) }">
                <div class="seal-son" :style="{ borderColor: getSealColor(basicInfoData.planExamineStatus) }">
                  <span class="status-text"
                    :class="[basicInfoData.planExamineStatus == 'pending' || basicInfoData.planExamineStatus == 'has_been_sent_withdraw' ? 'is-pending' : 'is-noPending']"
                    :style="{ color: getSealColor(basicInfoData.planExamineStatus) }">{{
                      basicInfoData.planExamineStatus |
                      filtersSealStatus
                    }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="info-wrap">
            <div class="info-item">
              <div class="info-item-title">下发人</div>
              <div>{{ basicInfoData && basicInfoData.user ? basicInfoData.user.name : '' }}</div>
            </div>
            <div class="info-item">
              <div class="info-item-title">下发时间</div>
              <div>{{ basicInfoData.createDate }}</div>
            </div>
          <div class="info-item">
            <div class="info-item-title">截止时间</div>
            <div>{{ basicInfoData.endTime }}</div>
            </div>
            <!-- <div class="info-item">
                                                    <div class="info-item-title">岗位</div>
                                                    <div>{{ basicInfoData && basicInfoData.user ? basicInfoData.user.dutyName : '' }}</div>
                                                  </div> -->
          </div>

          <div class="info-wrap">
            <div class="info-item">
              <div class="info-item-title">关联目标责任书</div>
              <div>
                <el-input placeholder="请选择目标责任书指标" v-model="kpiList.name" readonly :title="kpiList.name"
                  style="width:200px">
                  <i slot="suffix" class="el-input__icon el-icon-user suffix-icon"></i>
                </el-input>
              </div>
            </div>
            <div class="info-item">
              <div class="info-item-title">任务要求</div>
              <div v-html="basicInfoData.remark" class="info-desc">
              </div>
              <!-- <div>{{ basicInfoData.remark }}</div> -->
            </div>
            <div class="info-item">
              <div class="info-item-title">附件</div>
              <div>
                <p v-for="item in fileList" :key="item.id">{{ item.name }}
                  <i class="el-icon-download" @click="downloadFile(item)"
                    style="float: right;color:#409eff;cursor: pointer;margin-left: 10px;"></i>
                  <i class="el-icon-view" @click="viewFile(item.fileUrl)"
                    style="float: right;color:#409eff;cursor: pointer;"></i>
                </p>
              </div>
            </div>
          </div>
          <div v-if="assoicaProject.id" style="clear: both;">
            <h3 style="margin-bottom:10px"><i class="el-icon-paperclip"></i>关联业务</h3>
            <div v-if="planBusinessList.length > 0">
              <div v-for="planBusiness in planBusinessList" :key="planBusiness.businessId" style="margin-bottom: 5px;">
                <i class="el-icon-paperclip" style="line-height:30px"></i>
                <span style="margin-right:20px">{{ planBusiness.businessPlanName }}</span>
                <span v-if="planBusiness.previewUrl" style="margin:0 15px;color:#4c4c4c">
                  已上传：{{ planBusiness.fileName }}</span>
                <el-button v-if="planBusiness.previewUrl" size="mini" type="text"
                  @click="viewFile(planBusiness.previewUrl)">
                  预览
                </el-button>
              </div>
            </div>
            <div v-if="planBusinessList.length == 0">暂未关联业务</div>
          </div>
        </div>

      <div class="card-style">
        <el-tabs v-model="tabActiveName" @tab-click="handleClick">
          <el-tab-pane label="工作计划" name="plan">
            <!-- <dy-table
                :fetchData="getWorkFlowList"
                                                      :keys="projectDepartColKey"
                                                      :list="workFlowList"
                                                      :isShowBorder='true'
                                                      style="padding:0px;min-height:0px;"
                                                    ></dy-table> -->

              <el-table :data="workFlowList" border style="width: 100%;padding:0px;min-height:0px;">
                <el-table-column prop="name" label="工作项" width="150">
                </el-table-column>
                <el-table-column prop="stepCount" label="工作流">
                  <template slot-scope="scope">
                    <div class="custom-step-wrap">
                      <div class="step-item-wrap" v-for="step in scope.row.planItemStepList" :key="step.id">
                        <el-tooltip effect="light" :content="step.stepName + ':' + step.finishStatusName" placement="top"
                          style="cursor:pointer">
                          <div class="step-item" :style="{ backgroundColor: getColor(step.finishStatus, step.task) }">
                          </div>
                        </el-tooltip>
                        <div class="testClass" style="margin-right: 0px;">
                        </div>
                      </div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column prop="startTime" label="计划开始时间" width="130">
                </el-table-column>
                <el-table-column prop="endTime" label="计划结束时间" width="130">
                </el-table-column>

              </el-table>
            </el-tab-pane>

            <el-tab-pane label="子任务" name="subtask">
              <!-- height="calc(100vh - 62vh)" -->
              <el-table ref="planTable" :data="subTableData" lazy :load="load" style="width: 100%" stripe row-key="id"
                :tree-props="{ children: 'children', hasChildren: 'hasChildren' }">
                <el-table-column prop="name" label="任务名称" width="250" fixed>
                </el-table-column>
                <el-table-column prop="endTime" label="截止时间" width="200" align="center">
                </el-table-column>
                <el-table-column prop="planState" label="完成状态" align="center">
                  <template slot-scope="scope">
                    {{ scope.row.planState | newPlanState }}
                  </template>
                </el-table-column>
                <el-table-column label="负责人" width="200" align="center">
                  <template slot-scope="scope">
                    {{ scope.row.user ? scope.row.user.name : '' }}
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>

            <!-- 审核记录 -->
            <el-tab-pane label="审核记录" name="auditRecord">
              <div v-if="Object.keys(basicInfoData).length && basicInfoData.executeLogList.length">
                <div v-for="(item, index) in basicInfoData.executeLogList" :key="index">
                  <div v-if="item.executePhase == 'has_been_sent_withdraw'"
                    style="padding: 12px 10px;border-bottom: 12px solid #f2f2f2;">
                    <div>
                      {{ item.user.name + ' 于 ' + item.createDate + ' 终止此项工作' }}
                    </div>
                    <div>
                      {{ item.executeExplain }}
                    </div>
                  </div>

                  <div v-if="item.executePhase == 'submit'" style="padding: 12px 10px;border-bottom: 12px solid #f2f2f2;">
                    <div>
                      {{ item.user.name + ' 于 ' + item.createDate + ' 提交审核' }}
                    </div>
                    <div>
                      {{ item.executeExplain }}
                    </div>
                    <div v-if="item.executePhase == 'submit' && item.fileList && item.fileList.length">
                      <div v-for="(fItem, fIndex) in item.fileList" :key="fIndex">
                        <el-button type="text" @click="downLoadDocument(fItem.fileUrl, fItem.fileName)">
                        {{ fItem.fileName }}</el-button>
                      <i class="el-icon-view" @click="viewFile(fItem.fileUrl)"
                        style="color:#409eff;cursor: pointer;margin-left:10px"></i>
                      </div>
                      <!-- <el-button
                                                              type="text"
                                                              @click="downLoadDocument(item.fileUrl,item.fileName)"
                                                            >{{item.fileName}}</el-button> -->
                    </div>
                  </div>
                  <div v-if="item.executePhase == 'pass' || item.executePhase == 'not_pass'"
                    style="padding: 12px 10px;border-bottom: 12px solid #f2f2f2;">
                    <div>
                      {{
                        item.examiner.name + ' 于 ' + item.createDate + (item.executePhase == 'pass' ? ' 审核通过' : '
                                            审核不通过')}}
                    </div>
                    <div>
                      {{ item.executeExplain }}
                    </div>
                  </div>
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button type="primary" @click="handleClose()">关闭</el-button>
    </span>
  </el-dialog>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { viewFile } from '@/utils';
export default {
  name: '',
  components: { DyTable },
  data() {
    return {
      activeName: 'plan',
      tabActiveName: 'auditRecord',
      basicInfoData: {},
      selectManageId: '', // 一级目录Id
      selectManageContentId: '', // 二级目录Id
      kpiList: { // 目标责任书指标
        id: '',
        name: ''
      },
      assoicaProject: {
        id: '',
        name: ''
      },
      assoicaProjectList: [],
      // 工作计划
      workFlowList: [],
      projectDepartColKey: {
        name: '工作项',
        stepCount: '工作流',
        startTime: '计划开始时间',
        endTime: '计划结束时间'
      },
      // 子任务
      subTableData: [],
      saveTreeObj: {},
      // 审核
      auditForm: {
        // opinion: '1',
        content: ''
      },
      businessList: [
        {
          name: '手续文件',
          tag: 'prophase_procedures'
        },
        {
          name: '管理进度',
          tag: 'software_progress_plan'
        },
        {
          name: '季度工作计划',
          tag: 'work_plan'
        }
      ],
      planBusinessList: [],
      fileList: []
      // auditFormRules: {
      //   opinion: [
      //     { required: true, message: '请选择审核意见', trigger: 'change' }
      //   ]
      // }
    };
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    detailId: {
      type: String,
      default: ''
    }
  },
  computed: {},
  watch: {},
  created() {
  },
  mounted() {
    this.init();
  },
  methods: {
    viewFile(url) {
      viewFile(url)
    },
    downloadFile(data) {
      console.log(data, '9999999');
      this.downLoad(data.fileUrl, data.name);
    },
    downLoad(href, name) {
      const ele = document.createElement('a');
      ele.target = '_blank';
      fetch(href)
        .then(res => {
          return res.blob();
        })
        .then(blob => {
          const b = new Blob([blob]);
          const url = window.URL.createObjectURL(b);
          ele.href = url;
          ele.download = name;
          document.body.appendChild(ele);
          ele.click();
          document.body.removeChild(ele);
          window.URL.revokeObjectURL(url);
        });
    },
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      }).then(res => {
        if (res.isSuccess) {
          res.data.map(item => {
            this.fileList.push({ name: item.fileName, fileUrl: item.fileUrl, id: item.id });
          });
        }
      });
    },
    init() {
      this.auditForm = {
        // opinion: '1',
        content: ''
      };
      this.getAssociateProject();
      this.getDetailData();
      this.getWorkFlowList();
      this.getSubTaskData();
      this.getFileByBizId(this.detailId);
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    getColor(type, task) {
      let color = '';
      if (type == 'finish' || type == 'early_finish' || type == 'overtime_finish') {
        color = '#2FC25B';
      } else if (type == 'not_finish') {
        if (task) {
          if (task.taskStatus == 'waiting_send') {
            color = '#1890FF';
          } else if (task.taskStatus == 'pending' || task.taskStatus == 'has_been_sent_withdraw') {
            color = '#223273';
          }
        } else {
          color = '#ccc';
        }
      } else if (type == 'overtime_not_finish') {
        if (task) {
          if (task.taskStatus == 'waiting_send') {
            color = '#FACC14';
          } else if (task.taskStatus == 'pending' || task.taskStatus == 'has_been_sent_withdraw') {
            color = '#223273';
          }
        } else {
          color = '#ccc';
        }
      } else if (type == 'withdraw') {
        color = '#dc143c';
      }
      return color;
    },
    getSealColor(type) {
      let color = '';
      if (type == 'withdraw') {
        color = 'rgba(220,20,60,0.7)';
        // color = '#dc143c';
      } else if (type == 'has_been_sent_withdraw') {
        color = 'rgb(34, 50, 115,0.7)';
      } else if (type == 'pending') {
        color = 'rgb(34, 50, 115,0.7)';
        // color = '#223273';
      } else if (type == 'waiting_send') {
        color = 'rgb(24, 144, 255,0.7)';
        // color = '#1890FF';
      } else if (type == 'done') {
        color = 'rgba(47, 194, 91,0.7)';
        // color = '#2FC25B';
      }
      return color;
    },
    // updateAudit(formName) { // 提交审核 -- 我的全景图没有提交审核这个
    //   this.$refs[formName].validate((valid) => {
    //     if (valid) {
    //       this.postWorkDetail();
    //     } else {
    //       return false;
    //     }
    //   });
    // },
    postWorkDetail(type) {
      const param = {
        data: {
          id: this.detailId
        }
      };
      param.data.submitInfo = {
        executeExplain: this.auditForm.content,
        executePhase: type == 1 ? 'pass' : 'not_pass'
      };
      this.$axios.post(Api.taskManage.myTask.submitPlanTask, param, res => {
        if (res.isSuccess) {
          this.$message.success('审核成功！');
          this.$emit('checkInit');
          this.handleClose();
        } else {
          this.$message.error(res.message);
        }
      });
    },
    getWorkFlowList() {
      // 获取工作项表格数据
      this.$axios.post(
        Api.taskManage.myTask.getNewWorkPlanList,
        {
          data: {
            plan: {
              id: this.detailId
            }
          }
        },
        res => {
          if (res.isSuccess) {
            this.workFlowList = res.data ? res.data : [];
            this.workFlowList.forEach(x => {
              x.planItemStepList.forEach(y => {
                y.finishStatusName = this.finishStatus(y.finishStatus); // 获取状态名称
              });
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取子任务数据
    getSubTaskData(type, tree, resolve) {
      const param = {
        data: {
          id: this.detailId
        }
      };

      if (type == 'treeLoad') {
        param.data.id = this.saveTreeObj.tree.id;
      }
      this.$axios.post(
        Api.taskManage.myTask.getTaskDistributeList,
        param,
        res => {
          if (type == 'treeLoad') { // 树结构点击下一层级异步加载
            if (res.isSuccess) {
              if (res.data) {
                // eslint-disable-next-line no-return-assign
                res.data.map(x => x.hasChildren = true);
                this.saveTreeObj.resolve(res.data);
              } else {
                this.saveTreeObj.resolve([]);
              }
            }
          } else {
            if (res.isSuccess) {
              if (res.data) {
                // eslint-disable-next-line no-return-assign
                // res.data.map(x => x.isParent = !!isParent);
                // eslint-disable-next-line no-return-assign
                res.data.map(x => x.hasChildren = true);
                this.subTableData = res.data;
              } else {
                this.subTableData = [];
              }
            } else {
            }
          }
        }
      );
    },
    // 树结构点击异步加载子节点
    load(tree, treeNode, resolve) {
      this.saveTreeObj = {
        tree: tree,
        resolve: resolve
      };
      this.getSubTaskData('treeLoad', tree, resolve);
    },
    // // 关联项目
    getAssociateProject() {
      this.$axios.post(
        Api.taskManage.taskArrange.getAssocialProjectList,
        {
          data: {
            projectReqTypeEnum: 'PROJECT_COMPANY_ROLE'
          }
        },
        res => {
          if (res.isSuccess) {
            this.assoicaProjectList = res.data || [];
            this.assoicaProjectList.unshift({
              id: 0,
              name: '无'
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getDetailData() {
      const param = {
        data: {
          id: this.detailId
        }
      };
      if (this.$store.state.user.groupDepartment != 'group') { // 项目入口需要传项目id
        param.data.project = {
          id: this.$store.state.user.projectId
        };
      }
      this.$axios.post(
        Api.taskManage.myTask.getTaskListDetail,
        param,
        res => {
          if (res.isSuccess) {
            if (res.data.project) {
              this.assoicaProject = {
                id: res.data.project.id,
                name: res.data.project.name
              };
            } else {
              this.assoicaProject = {
                id: 0,
                name: '无'
              };
            }
            if (res.data.planBusinessList && res.data.planBusinessList.length) {
              this.planBusinessList = [];
              res.data.planBusinessList.forEach(planBusiness => {
                this.getBusinessPlanName(res.data, planBusiness);
              });
            }
            if (res.data.kpiGroup) {
              if (res.data.kpiGroup.manageType == 'work_target') {
                this.kpiList = {
                  id: res.data.kpiGroup.kpiList[0].id,
                  name: res.data.kpiGroup.kpiList[0].targetItemTwo
                };
              } else {
                this.kpiList = {
                  id: res.data.kpiGroup.kpiList[0].id,
                  name: res.data.kpiGroup.kpiList[0].content
                };
              }
              this.selectManageId = res.data.kpiGroup.id;
              this.selectManageContentId = res.data.kpiGroup.kpiList[0].id;
            }
            this.basicInfoData = res.data;
            this.getFlowStep();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getFlowStep() {
      // if (this.planDetail.task && Object.keys(this.planDetail.task).length) { // task字段不为空
      // this.stepList = this.planDetail.executeLogList;
      this.basicInfoData.executeLogList.forEach(async x => {
        const fileArr = await this.getAttachmentList(x.id);
        if (fileArr) {
          this.$set(x, 'fileList', fileArr);
          // this.$set(x, 'fileUrl', fileObj.fileUrl);
          // this.$set(x, 'fileName', fileObj.fileName);
        } else {
          this.$set(x, 'fileList', []);
          // this.$set(x, 'fileUrl', null);
          // this.$set(x, 'fileName', null);
        }
      });
      // }
    },
    // 开始下载前使用业务id去获取对应的文件数组
    getAttachmentList(id) {
      return new Promise((resolve, reject) => {
        let arr = null;
        this.$axios.post(
          Api.schedule.getAttachmentList,
          {
            data: {
              relationId: id
            }
            // fileType: 'ordinaryFile'
          },
          res => {
            if (res.isSuccess) {
              if (res.data.length) {
                arr = res.data.map(x => {
                  return {
                    fileUrl: x.fileUrl,
                    fileName: x.fileName
                  };
                });
                // arr = {
                //   fileUrl: res.data[0].fileUrl,
                //   fileName: res.data[0].fileName
                // };
              } else {
                arr = null;
                // this.$message.warning('此工作流未上传附件！');
              }
              resolve(arr);
            } else {
              // this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 获取手续业务的数据
    getProcedureDetail(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.projectDept.getProcedureDetail,
          {
            data: {
              id
            }
          },
          res => {
            if (res.isSuccess) {
              if (res.data.formalitiesSonApiVos) {
                resolve(res.data.formalitiesSonApiVos);
              }
            }
          }
        );
      });
    },
    // 获取拼接关联业务的各层级名称
    async getBusinessPlanName(detailData, planBusiness) {
      let tagName = '';
      this.businessList.map(item => {
        if (item.tag == planBusiness.planBusinessType) {
          tagName = item.name;
        }
      });
      const file = await this.getBusinessFile(planBusiness.businessId);
      if (planBusiness.planBusinessType == 'prophase_procedures') {
        // 获取手续文件层级名称
        const data = await this.getProcedureDetail(planBusiness.businessId);
        let targetType = '';
        data.map(type => {
          if (type.projectDocTypeTemplateApiVos && type.projectDocTypeTemplateApiVos.length) {
            type.projectDocTypeTemplateApiVos.map(x => {
              if (x.id == planBusiness.businessId) {
                targetType = type.name;
              }
            });
          }
        });
        this.businessPlanName = tagName + '/' + targetType + '/' + planBusiness.assembledName;
      } else if (planBusiness.planBusinessType == 'software_progress_plan') {
        this.businessPlanName = tagName + '/' + planBusiness.assembledName;
      }
      this.planBusinessList.push({
        businessId: planBusiness.businessId,
        planBusinessType: planBusiness.planBusinessType,
        businessPlanName: this.businessPlanName,
        previewUrl: file.url,
        fileName: file.fileName
      });
    },
    /* 查询业务绑定的文件--关联业务的文件 */
    getBusinessFile(id) {
      return new Promise((resolve, reject) => {
        // 查任务详情时如果有业务 查询业务关联的文件
        this.$axios.post(
          Api.schedule.getAttachmentList,
          {
            data: {
              relationId: id
            }
            // fileType: 'ordinaryFile'
          },
          (res) => {
            if (res.isSuccess) {
              if (res.data.length) {
                resolve({
                  url: res.data[0].fileUrl,
                  fileName: res.data[0].fileName
                });
              } else {
                resolve({
                  url: '',
                  fileName: ''
                });
              }
            }
          }
        );
      });
    },
    previewBusinessFile(url) {
      // 预览文件
      const ele = document.createElement('a');
      ele.target = '_blank';
      if (url) {
        ele.href = url;
        ele.style.display = 'none';
        document.body.appendChild(ele);
        ele.click();
        document.body.removeChild(ele);
      }
    },
    downLoadDocument(url, name = '下载文件') {
      if (!!window.ActiveXObject || 'ActiveXObject' in window) { // IE无法识别download属性，用户自行保存
        this.$message.error(
          '当前浏览器不支持点击下载，请手动保存，或者切换到Google Chrome浏览器进行下载'
        );
      } else {
        const x = new XMLHttpRequest();
        x.open('GET', url, true);
        x.responseType = 'blob';
        x.onload = function () {
          const url = window.URL.createObjectURL(x.response);
          const a = document.createElement('a');
          a.href = url;
          a.download = name;
          a.click();
          a.remove();
        };
        x.send();
      }
    },
    handleClick(tab) {

    },
    finishStatus(type) {
      if (type == 'finish') {
        return '完成';
      } else if (type == 'not_finish') {
        return '未完成';
      } else if (type == 'early_finish') {
        return '提前完成';
      } else if (type == 'overtime_finish') {
        return '超时完成';
      } else if (type == 'overtime_not_finish') {
        return '超时未完成';
      } else if (type == 'withdraw') {
        return '终止';
      } else {
        return '';
      }
    }
  },
  filters: {
    filtersFinishStatus: function (type) {
      if (type == 'finish') {
        return '完成';
      } else if (type == 'not_finish') {
        return '未完成';
      } else if (type == 'early_finish') {
        return '提前完成';
      } else if (type == 'overtime_finish') {
        return '超时完成';
      } else if (type == 'overtime_not_finish') {
        return '超时未完成';
      } else if (type == 'withdraw') {
        return '终止';
      } else {
        return '';
      }
    },
    filtersSealStatus: function (type) {
      let text = '';
      if (type == 'withdraw') {
        text = '已撤销';
      } else if (type == 'has_been_sent_withdraw') {
        text = '申请撤销';
      } else if (type == 'pending') {
        text = '申请完成';
      } else if (type == 'waiting_send') {
        text = '进行中';
      } else if (type == 'done') {
        text = '已通过';
      }
      return text;
    },
    newPlanState: (value) => {
      if (value == 'under_review') {
        return '进行中';
      } else if (value == 'end' || value == 'finish') {
        return '已完成';
      } else if (value == 'not_enabled' || value == 'enabled') {
        return '未完成';
      }
    }
  }
};
</script>
<style lang='scss' scoped>
::v-deep .el-dialog {
  margin: 0 auto;
}

.seal {
  width: 160px;
  height: 160px;
  border: solid 6px #b4b4b4;
  border-radius: 100%;
  background-color: rgba(255, 255, 255, 0.8);
  position: relative;
  display: flex;
  justify-content: center;
  align-items: center;
  position: absolute;
  right: 10px;
  top: -16px;
}

.seal-son {
  width: 145px;
  height: 145px;
  border: solid 2px #b4b4b4;
  border-radius: 100%;
  background-color: rgba(255, 255, 255, 0.8);
  position: relative;

  .status-text {
    position: absolute;
    top: 45px;
    text-align: center;
    // font-size: 34px;
    transform: rotate(-45deg);
    // right: 20px;
    font-weight: 900;

    &.is-pending {
      font-size: 30px;
      right: 16px;
    }

    &.is-noPending {
      font-size: 34px;
      right: 20px;
    }
  }
}

.task-container {
  // background: #fff;
  user-select: none;

  .append {
    height: 600px;
    overflow: auto;
  }

  .lower {
    height: 400px;
    overflow: auto;
  }

  // padding: 10px;
  .card-style {
    background: #fff;
    border-radius: 4px;
    margin-bottom: 20px;
    padding: 10px;
  }

  .info-wrap {
    display: flex;

    .info-item {
      margin-bottom: 20px;
      // margin-top: 10px;
      flex: 1;

      .info-item-title {
        font-weight: bold;
        margin-bottom: 10px;
      }

      .info-desc {
        max-height: 200px;
        overflow: auto;

        ::v-deep img {
          max-width: 100%;
        }
      }
    }
  }

  .check-wrap {
    padding: 20px;
    box-shadow: 0 2px 12px 0 #ccc;
    margin-top: 20px;
    margin-bottom: 20px;
  }

  .postCheckBtnWrap {
    position: fixed;
    left: 230px;
    bottom: 20px;
    z-index: 100;
    width: 100%;
    width: calc(100% - 250px) !important;
    padding: 20px;
    background: #fff;
    text-align: center;

    .postCheckBtn {
      width: 60%;
    }
  }
}

.custom-step-wrap {
  position: relative;
  display: flex;

  // justify-content: space-between;
  .step-item-wrap {
    display: flex;
    position: relative;
    flex-basis: 50%;

    .step-item {
      width: 12px;
      height: 12px;
      background: #2fc25b;
      border-radius: 50%;
      display: inline-block;
    }

    .testClass {
      height: 2px;
      background-color: #c0c4cc;
      position: absolute;
      width: calc(100% - 12px);
      left: 12px;
      top: 5px;
    }

    &:last-child {
      flex-basis: auto !important;
    }

    &:last-child .testClass {
      display: none;
    }
  }
}

.status-bar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 30px;
  position: absolute;
  right: 2px;
  top: 0px;

  .stutus-item {
    margin-left: 20px;
    font-size: 12px;

    i {
      display: inline-block;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      margin-right: 3px;
    }
  }
}

.hideSidebar .postCheckBtnWrap {
  width: calc(100% - 94px) !important;
  left: 74px;
}

::v-deep .el-dialog {
  top: -66px;
}

::v-deep .el-dialog__body {
  padding: 0 20px;
}

::v-deep .el-table__fixed::before,
.el-table__fixed-right::before {
  z-index: 0;
}
</style>
