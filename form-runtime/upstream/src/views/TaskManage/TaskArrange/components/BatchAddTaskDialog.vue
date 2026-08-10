<!--
 * @Descripttion: 任务下发表单操作
 * @Author: zhengzetao
 * @Date: 2021-06-15
-->
<template>
  <div>
    <el-dialog :title="arrangeType == 'check' ? '查看任务' : arrangeType == 'add' ? '批量新建' : '任务更新'" :visible="visible"
      width="90%" :close-on-click-modal="false" top="100px" @close="handleClose">
      <el-form :model="form" :rules="formRules" ref="form" label-width="100px" label-position="top">
        <div>
          <h3 style="margin-bottom: 10px">基本信息</h3>
          <div class="info-item">
            <el-form-item label="是否关联项目">
              <el-select v-model="form.isAssociated" placeholder="请选择" style="width: 100%"
                @change="changeAssociatedProject"
                :disabled="arrangeType == 'check' || arrangeType == 'edit' || isProjectSpace">
                <el-option label="是" :value="true"></el-option>
                <el-option label="否" :value="false"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="info-item" v-if="form.isAssociated">
            <el-form-item label="关联项目:" prop="project.name">
            <el-input type="input" v-model="form.project.name" placeholder="请选择关联项目" readonly :disabled="isProjectSpace"
              @focus="openSelectProjectDialog"></el-input>
          </el-form-item>
        </div>
        <!-- <div class="info-item">
            <el-form-item label="上传附件:">
              <el-upload ref="upload" :action="pdfAction" :data="fileData" accept="" multiple
                :before-upload="beforeAvatarUpload" :on-success="handleAvatarSuccess" :on-remove="handleRemove"
                        :before-remove="beforeRemove" :file-list="fileList">
              <el-button size="small" type="primary">选择文件</el-button>
            </el-upload>
          </el-form-item>
        </div> -->
        <!-- <div class="info-wrap">
            <div class="info-item">
              <el-form-item label="关联项目">
                <el-select
                  v-model="form.project.id"
                  placeholder="请选择项目"
                  :disabled=" arrangeType == 'check' || (arrangeType == 'edit'&&!projectCanUpdate) || $store.state.user.groupDepartment != 'group'"
                  @change="selectAssociateProject"
                  style="width: 100%"
                >
                  <el-option
                    v-for="(item) in assoicaProject"
                    :label="item.name"
                    :value="item.id"
                    :key="item.id"
                  ></el-option>
                </el-select>
              </el-form-item>
                              </div>
            <div class="info-item">

            </div>
          </div> -->
        <!-- <div class="info-wrap">
            <div class="info-item">
              <el-form-item
                label="任务名称"
                prop="name"
              >
                <el-input
                  v-model="form.name"
                  :readonly="arrangeType == 'check'"
                  placeholder="请输入任务名称"
                ></el-input>
              </el-form-item>
            </div>
            <div class="info-item">
              <el-form-item label="关联项目">
                <el-select
                  v-model="form.project.id"
                  placeholder="请选择项目"
                  :disabled=" arrangeType == 'check' || (arrangeType == 'edit'&&!projectCanUpdate) || $store.state.user.groupDepartment != 'group'"
                  @change="selectAssociateProject"
                  style="width: 100%"
                >
                  <el-option
                    v-for="(item) in assoicaProject"
                    :label="item.name"
                    :value="item.id"
                    :key="item.id"
                                    ></el-option>
                </el-select>
              </el-form-item>
            </div>
          </div> -->
        <!-- <div class="info-wrap">
            <div class="info-item">
              <el-form-item
                prop="user.name"
                label="任务负责人:"
              >
                <el-input
                  placeholder="请选择任务负责人"
                  v-model="form.user.name"
                  @focus="openIndicatorHeaderDialog(-1)"
                  readonly
                >
                  <i
                    slot="suffix"
                    class="el-input__icon el-icon-user suffix-icon"
                  ></i>
                </el-input>
              </el-form-item>
            </div>
            <div class="info-item">
              <el-form-item
                prop="remark"
                label="任务要求:"
              >
                <el-input
                  :disabled="arrangeType == 'check'"
                                    placeholder="请输入任务要求"
                  v-model="form.remark"
                >
                </el-input>
              </el-form-item> -->
        <!-- <el-form-item label="负责人岗位:">
                <el-select
                  v-model="form.user.dutyId"
                  placeholder="请选择负责人岗位"
                  :disabled="arrangeType == 'check'"
                  clearable
                  style="width:100%;"
                >
                  <el-option
                    v-for="item in chargerDutyList"
                    :label="item.workContent + '-' + item.dutyName"
                                      :value="item.id"
                    :key="item.id"
                                    ></el-option>
                </el-select>
              </el-form-item> -->
        <!-- </div>
          </div> -->
        <!-- <div class="info-wrap">
            <div class="info-item">
              <el-form-item
                prop="endTime"
                label="截止时间:"
              >
                <el-date-picker
                  v-model="form.endTime"
                  value-format="yyyy-MM-dd HH:mm"
                  format="yyyy-MM-dd HH:mm"
                  type="datetime"
                  placeholder="选择截止时间"
                  :editable='false'
                  :readonly="arrangeType == 'check'"
                  style="width: 200px;"
                >
                </el-date-picker>
              </el-form-item>
                              </div>
                              <div class="info-item">

                              </div>
                            </div> -->
        </div>
        <div style="clear:both;">
          <div class="flex-box  flex-between">
            <h3 style="margin-bottom: 10px">任务</h3>
            <i class="el-icon-circle-plus suffix-icon" style="color: #409eff" v-if="arrangeType == 'add'"
              @click="addWorkTemplate"><span style="padding-left: 6px">调用任务模板</span></i>
          </div>
          <el-form-item label="" prop="indicateName">
            <el-table :data="form.childrenWrokPlan" border style="width: 100%">
              <el-table-column label="任务名称">
                <template slot-scope="scope">
                  <el-form-item :prop="'childrenWrokPlan.' + scope.$index + '.name'"
                    :rules='formRules.indicatorTaskListName'>
                    <i v-if="arrangeType == 'add' || (arrangeType == 'edit' && !scope.row.id)"
                      style="color:red;margin-right:5px" class="el-icon-remove suffix-icon"
                      @click="deleteTaskIndicator(scope.$index)"></i>
                    <el-input type="textarea" size="medium" :autosize="{ minRows: 2 }" v-model.trim="scope.row.name"
                      placeholder="请输入任务名称" maxlength="50" show-word-limit style="width:90%;"></el-input>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="任务要求">
                <template slot-scope="scope">
                  <el-form-item :prop="'childrenWrokPlan.' + scope.$index + '.remark'"
                    :rules='formRules.indicatorTaskListSandard'>
                    <el-input type="textarea" size="medium" :autosize="{ minRows: 2 }" v-model.trim="scope.row.remark"
                      :readonly="arrangeType == 'check'" placeholder="请输入任务要求" maxlength="200" show-word-limit>
                    </el-input>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="负责人">
                <template slot-scope="scope">
                  <el-form-item :prop="'childrenWrokPlan.' + scope.$index + '.user.name'"
                    :rules='formRules.indicatorTaskListTaskHeaderName'>
                    <el-input type="input" v-model="scope.row.user.name" placeholder="请选择负责人" readonly style="width:92%"
                      @focus="openIndicatorHeaderDialog(scope.$index, scope.row, -1)"></el-input>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="审核人">
                <template slot-scope="scope">
                  <el-form-item :prop="'childrenWrokPlan.' + scope.$index + '.examiner.name'"
                    :rules='formRules.indicatorTaskListTaskHeaderName'>
                    <el-input type="input" v-model="scope.row.examiner.name" placeholder="请选择负责人" readonly style="width:92%"
                      @focus="openIndicatorHeaderDialog(scope.$index, scope.row, -2)"></el-input>
                  </el-form-item>
                </template>
              </el-table-column>

              <el-table-column label="工作记录需审批">
                <template slot="header">
                  <span>工作记录需审批</span>
                  <el-tooltip content="如果选择需要审核，则任务接收人必须做工作计划！" placement="top" effect="light">
                    <i class="el-icon-warning-outline"
                      style="margin-left:5px;cursor:pointer;color: black;font-size: 16px;"></i>
                  </el-tooltip>
                </template>
                <template slot-scope="scope">
                  <el-form-item>
                    <el-select v-model="scope.row.planType" placeholder="请选择" :disabled="arrangeType == 'check'"
                      style="width: 100%">
                      <el-option v-for="(item) in needCheckList" :label="item.name" :value="item.id" :key="item.id">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </template>
              </el-table-column>
              <el-table-column label="截止时间" width="180">
                <template slot-scope="scope">
                  <el-form-item :prop="'childrenWrokPlan.' + scope.$index + '.endTime'"
                    :rules='formRules.indicatorTaskListFinishTime'>
                    <el-date-picker v-model="scope.row.endTime" value-format="yyyy-MM-dd HH:mm" format="yyyy-MM-dd HH:mm"
                      type="datetime" placeholder="选择截止时间" :editable='false' :readonly="arrangeType == 'check'"
                      style="width: 160px;">
                    </el-date-picker>
                  </el-form-item>
                </template>
              </el-table-column>
              <!-- 关联项目必填 -->
              <el-table-column label="关联业务" width="320" v-if="form.project.id">
                <template slot-scope="scope">
                  <el-form-item>
                    <el-popover v-if="scope.row.planBusinessList.length < 2" placement="right" width="150"
                      trigger="hover">
                      <div v-for="(business, index) in businessList" :key="index"
                        @click="handleClickBusiness(business.tag, business.name, scope.$index)" class="businessItem">
                        <el-button type="text"
                          :disabled="scope.row.planBusinessList.some(x => x.planBusinessType == business.tag)">
                          {{ business.name }}</el-button>
                      </div>
                      <el-button slot="reference" type="text" :disabled="arrangeType == 'check'">选择业务</el-button>
                    </el-popover>
                    <div v-if="scope.row.planBusinessList.length > 0">
                      <div v-for="(planBusiness, index) in scope.row.planBusinessList" :key="planBusiness.businessId">
                        <div style="margin-right:20px"><i class="el-icon-paperclip"></i>{{ planBusiness.businessPlanName
                        }}
                        </div>
                        <el-button v-if="arrangeType !== 'check'" type="primary"
                          @click="handleReChooseBusiness(scope.$index, planBusiness.planBusinessType)">重 选</el-button>
                        <el-button v-if="arrangeType !== 'check'" type="danger"
                          @click="handleCancelBusiness(scope.$index, index)">解除关联</el-button>
                      </div>
                    </div>
                  </el-form-item>
                </template>
              </el-table-column>
            </el-table>
          </el-form-item>
          <i class="el-icon-circle-plus suffix-icon" style="color: #409eff" v-if="arrangeType != 'check'"
            @click="addDistributeTask"><span style="padding-left: 6px">添加任务</span></i>
        </div>

      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" v-if="arrangeType != 'check'" @click="submitForm('form')">提 交</el-button>
      </span>
    </el-dialog>

    <!-- 任务负责人：选择任务负责人 -->
    <IndicatorHeaderDialog :visible.sync="indicatorHeaderVisible" v-if="indicatorHeaderVisible"
      :associateId="form.project.id" :taskHeaderId="taskHeaderId" :projectRelationType="projectRelationType"
      @selectHeader="selectHeader" />
    <!-- @getHeaderDutyList="getHeaderDutyList" -->

    <!-- 选择项目弹窗 -->
    <SelectProjectDialog v-if="selectProjectVisible" :visible.sync="selectProjectVisible" :associateId="form.project.id"
      :assoicaProject="assoicaProject" @selectProject="selectProject" />

    <!-- 目标责任书指标 -->
    <WorkTargetDailog v-if="workTargetVisible" :visible.sync="workTargetVisible" :selectManageId="selectManageId"
      :selectManageContentId="selectManageContentId" @workTargetSelect="workTargetSelect" />
    <!--任务模板  -->
    <WorkTemplateDialog v-if="workTemplateVisible" :visible.sync="workTemplateVisible"
      @workTemplateSelect="workTemplateSelect" />
    <!-- 关联业务 -->
    <BusinessRelateDialog :title="businessTagName" :businessTag="businessTag" :visible.sync="businessDialogVisible"
      :projectId="form.project.id" @getBusinessData="getBusinessData" />
  </div>
</template>

<script>

import Api from '@/api';
import { baseUrl } from '@/config/env';
import { localstorageGet } from '@/utils/auth';
import IndicatorHeaderDialog from './IndicatorHeaderDialog.vue';
import BusinessRelateDialog from './BusinessRelateDialog.vue';
import WorkTargetDailog from './WorkTargetDailog';
import WorkTemplateDialog from './WorkTemplateDialog';
import SelectProjectDialog from './SelectProjectDialog.vue';

export default {
  name: '',
  components: { IndicatorHeaderDialog, WorkTargetDailog, WorkTemplateDialog, BusinessRelateDialog, SelectProjectDialog },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    arrangeType: {
      type: String,
      default: 'add'
    },
    detailId: {
      type: String,
      default: ''
    }
  },

  data() {
    const checkEndTime = (rule, value, callback) => {
      if (!value) {
        return callback(new Error('选择截止时间'));
      } else {
        if (this.arrangeType == 'edit') {
          const endTime = new Date(value).getTime();
          const maxEndTime = new Date(this.maxEndTime).getTime();
          const minEndTime = new Date(this.minEndTime).getTime();
          if (maxEndTime) {
            if (endTime - maxEndTime > 0) {
              return callback(new Error(`任务截止时间不能大于父任务截止时间：${this.maxEndTime}`));
            }
          }
          if (minEndTime) {
            if (endTime - minEndTime < 0) {
              return callback(new Error(`任务截止时间不能小于任务截止最小时间：${this.minEndTime}`));
            }
          }
          callback();
        }
        callback();
      }
    };
    const checkChildEndTime = (rule, value, callback) => {
      if (!value) {
        return callback(new Error('选择截止时间'));
      } else {
        const fatherEndTime = new Date(this.form.endTime).getTime();
        const endTime = new Date(value).getTime();
        if (fatherEndTime) {
          if (endTime - fatherEndTime > 0) {
            callback(new Error('子任务截止时间不能大于任务截止时间'));
          }
        }
        callback();
      }
    };
    return {
      isProjectSpace: !!localstorageGet('projectId'),
      indicatorHeaderVisible: false,
      workTargetVisible: false,
      workTemplateVisible: false,
      selectManageId: '', // 一级目录Id
      selectManageContentId: '', // 二级目录Id
      indicatorHeaderIdx: -1,
      taskHeaderId: '',
      maxEndTime: null,
      minEndTime: null,
      projectCanUpdate: true, // 关联项目是否可编辑
      form: {
        // name: '',
        isAssociated: false,
        project: {
          id: '',
          name: ''
        },
        // user: {
        //   name: '',
        //   id: ''
        //   // dutyId: '' // 岗位id
        // },
        // endTime: '',
        // remark: '', // 任务要求
        childrenWrokPlan: [
          // {
          //   name: '',
          //   endTime: '',
          //   remark: '',
          // planType: '',
          //   user: {
          //     name: '',
          //     id: ''
          //   }
          // }
        ]
      },
      personType: -1, // -1 负责人 -2 审核人
      assoicaProject: [
      ],
      chargerDutyList: [],
      needCheckList: [
        {
          id: 'target',
          name: '不审核'
        },
        {
          id: 'target_examine',
          name: '待审核'
        }
      ],
      selectTaskIndex: 0,
      businessDialogVisible: false,
      businessTag: '',
      businessTagName: '',
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
      projectRelationType: '',
      formRules: {
        name: [{ required: true, message: '请填写任务名称', trigger: 'blur' }],
        'project.name': [{ required: true, message: '请选择关联项目', trigger: 'none' }],
        'user.name': [{ required: true, message: '请选择任务负责人', trigger: 'none' }],
        'examiner.name': [{ required: true, message: '请选择任务审核人', trigger: 'none' }],
        // 'kpiList[0].name': [{ required: true, message: '请选择目标责任书指标', trigger: 'none' }],
        // 'user.dutyId': [{ required: true, message: '请选择负责人岗位', trigger: 'blur' }],
        endTime: [{ required: true, validator: checkEndTime, trigger: 'change' }],
        remark: [{ required: true, message: '请填写任务要求', trigger: 'blur' }],
        indicatorTaskListName: [
          {
            required: true,
            trigger: 'blur',
            message: '请输入任务名称'
          }
        ],
        indicatorTaskListSandard: [
          {
            required: true,
            trigger: 'blur',
            message: '请输入任务要求'
          }
        ],
        indicatorTaskListTaskHeaderName: [{ required: true, message: '请选择负责人', trigger: 'none' }],
        indicatorTaskListFinishTime: [
          {
            validator: checkChildEndTime,
            required: true,
            trigger: 'change'
          }
        ]
      },
      selectProjectVisible: false,
      fileData: {
        fileType: 'ordinaryFile'
      },
      fileList: [],
      flieId: []
    };
  },
  computed: {
    pdfAction() {
      const sid = this.$store.state.user.token;
      return `${baseUrl}/web/file/api/file/uploadFile?sid=${sid}&platformCode=200001`;
    }
  },
  watch: {
  },
  created() { },
  mounted() {
    this.getAssociateProject();
    // 编辑或查看
    if (this.arrangeType == 'check' || this.arrangeType == 'edit') {
      this.getTargetTypePlanDetail();
    } else {
      if (localstorageGet('groupDepartment') != 'group') { // 如果在项目里面，关联项目只显示这个项目，不能选择其他
        this.form.project.id = this.$store.state.user.projectId;
      }
    }
  },
  methods: {
    // 根据业务id获取文件
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
          data: {
            relationId: id
          }
        }).then(res => {
        if (res.isSuccess) {
          res.data.map(item => {
            this.flieId.push(item.id);
            this.fileList.push({ name: item.fileName, fileUrl: item.fileUrl, id: item.id });
          });
        }
      });
    },
    beforeRemove(file, fileList) {
      if (!this.detailId) return true;
      const id = file.response ? file.response.data.id : file.id;
      return new Promise((resolve, reject) => {
        this.$confirm('是否确定解除关联?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$axios.post(
            Api.taskManage.taskArrange.cancelBusiness,
            {
              data: {
                id: id
              }
            },
            res => {
              if (res.isSuccess) {
                this.$message.success('解除关联成功');
                resolve();
              } else {
                reject(res.message);
              }
            }
          );
        }).catch((err) => {
          reject(err);
        });
      });
    },
    handleRemove(file, fileList) {
      // console.log(file, 'file');
      this.fileList = fileList;
      const id = file.response ? file.response.data.id : file.id;
      this.flieId = this.flieId.filter(e => e != id);
    },
    // 文件上传
    handleAvatarSuccess(res, file) {
      if (res.code == 'RESP200') {
        this.flieId.push(res.data.id || this.detailId);
      } else {
        this.$message.error(`文件上传失败,请重新上传`);
      }
    },
    beforeAvatarUpload(file) { },
    changeAssociatedProject() {
      this.form.project = {
        id: '',
        name: ''
      };
      this.form.user = {
        id: '',
        name: ''
        // dutyId: ''
      };
      // 把子任务负责人清空
      if (this.form.childrenWrokPlan.length) {
        this.form.childrenWrokPlan.forEach(item => {
          item.user.id = '';
          item.user.name = '';
        });
      }
    },
    selectProject(data) {
      // this.form.indicatorName = data[0].name;
      // this.form.indicatorId = data[0].id;
      this.form.project = {
        id: data[0].id,
        name: data[0].name
        // dutyId: ''
      };
      this.form.user = {
        id: '',
        name: ''
        // dutyId: ''
      };
      // 把子任务负责人清空
      if (this.form.childrenWrokPlan.length) {
        this.form.childrenWrokPlan.forEach(item => {
          item.user.id = '';
          item.user.name = '';
        });
      }
    },
    openSelectProjectDialog(index) {
      if (this.arrangeType == 'check' || this.arrangeType == 'edit') {
        return;
      }
      this.selectProjectVisible = true;
    },
    getTargetTypePlanDetail() {
      this.$axios.post(
        Api.taskManage.taskArrange.targetTypePlanDetail,
        {
          data: {
            id: this.detailId
          }
        },
        res => {
          if (res.isSuccess) {
            const data = res.data;
            this.form = {
              // name: data.name,
              project: {
                id: data.project?.id || 0
              },
              // user: {
              //   name: data.user.name,
              //   id: data.user.id
              //   // dutyId: data.user.dutyId // 岗位id
              // },
              // endTime: data.endTime,
              // remark: data.remark,
              childrenWrokPlan: data.childrenWrokPlan || []
            };
            if (data.project) {
              this.projectCanUpdate = data.project.canUpdate;
            } else {
              this.projectCanUpdate = true;
            }
            this.selectManageId = data.kpiGroup?.id;
            this.selectManageContentId = data.kpiGroup?.keyPerformanceIndicatorsList[0].id;
            this.maxEndTime = data.maxEndTime;
            this.minEndTime = data.minEndTime;
            // 重新请求负责人岗位
            // this.getHeaderDutyList();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 选择关联项目时重新选择任务负责人，同时要判断任务指标的指标类型是否错误（不能同时选择项目下和公司下的指标）
    selectAssociateProject(val) {
      this.form.user = {
        id: '',
        name: ''
        // dutyId: ''
      };
      // this.chargerDutyList = [];
      // 把子任务负责人清空
      if (this.form.childrenWrokPlan.length) {
        this.form.childrenWrokPlan.forEach(item => {
          item.user.id = '';
          item.user.name = '';
        });
      }
    },
    // 任务负责人选择赋值
    selectHeader(data) {
      if (this.indicatorHeaderIdx >= 0) {
        if (this.personType == -1) {
          this.$set(this.form.childrenWrokPlan[this.indicatorHeaderIdx], 'user', {
            name: data.name,
            id: data.id
          });
        } else if (this.personType == -2) {
          this.$set(this.form.childrenWrokPlan[this.indicatorHeaderIdx], 'examiner', {
            name: data.name,
            id: data.id
          });
        }
      } else {
        this.form.user = {
          name: data.name,
          id: data.id
          // dutyId: ''
        };
      }
    },
    // getHeaderDutyList() {
    //   this.form.project.id ? this.getProjectHeaderDutyList() : this.getCompanyHeaderDutyList();
    // },
    // 查询负责人的所有岗位(项目和相关方)
    getProjectHeaderDutyList() {
      if (this.form.user.id) {
        const data = {
          selectedUserId: this.form.user.id,
          projectId: this.form.project.id
        };
        this.$axios.post(
          Api.taskManage.taskArrange.getProjectHeaderDutyList,
          {
            data
          },
          res => {
            if (res.isSuccess) {
              this.chargerDutyList = res.data;
            } else {
              this.$message.error(res.message);
            }
          }
        );
      } else {
        return false;
      }
    },
    getCompanyHeaderDutyList() { // 查询负责人的所有岗位（平台）
      if (this.form.user.id) {
        const data = {
          id: this.form.user.id,
          companyId: '',
          flag: 'company'
        };
        this.$axios.post(
          Api.taskManage.taskArrange.getCompanyHeaderDutyList,
          {
            data
          },
          res => {
            if (res.isSuccess) {
              this.chargerDutyList = res.data;
            } else {
              this.$message.error(res.message);
            }
          }
        );
      } else {
        return false;
      }
    },
    // 选择责任人/负责人
    openIndicatorHeaderDialog(index, row, type) {
      if (this.arrangeType == 'check') {
        return;
      }
      // 判断是否表格点击
      if (index >= 0) {
        this.indicatorHeaderIdx = index;
        if (type == -1) {
          this.taskHeaderId = row.user.id;
          this.personType = -1;
        } else if (type == -2) {
          this.taskHeaderId = row.examiner.id;
          this.personType = -2;
        }
      } else {
        this.indicatorHeaderIdx = -1;
        this.taskHeaderId = this.form.user.id;
      }
      if (this.form.project.id) {
        this.projectRelationType = this.assoicaProject.find(x => x.id == this.form.project.id).relationType;
      }
      this.indicatorHeaderVisible = true;
    },
    // 点击目标责任书指标
    openWorkTargetDialog() {
      if (this.arrangeType == 'check') return;
      this.workTargetVisible = true;
    },
    // 关联项目
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
            this.assoicaProject = res.data || [];
            if (this.isProjectSpace) {
              this.form.isAssociated = true;
              this.form.project.id = localstorageGet('projectId');
              this.form.project.name = localstorageGet('projectName');
            }
            // this.assoicaProject.unshift({
            //   id: 0,
            //   name: '无'
            // });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // submitForm(formName) {
    //   this.$refs[formName].validate((valid) => {
    //     if (valid) {
    //       this.saveTask(formName);
    //     } else {
    //       return false;
    //     }
    //   });
    // },
    submitForm(formName) {
      // console.log('222submitForm22',this.form.childrenWrokPlan)
      console.log('submitForm');
      this.$refs[formName].validate((valid) => {
        console.log('valid', valid);
        if (valid) {
          if (!this.form.childrenWrokPlan.length) {
            this.$message.warning('请添加任务！');
            return;
          }
          if (this.arrangeType == 'add') {
            this.$confirm('关联项目确认后不可重新修改，是否确认提交?', '提示', {
              closeOnClickModal: false,
              confirmButtonText: '确定',
              cancelButtonText: '取消',
              type: 'warning'
            }).then(() => {
              this.saveTask(formName);
            }).catch(() => {
            });
          } else {
            this.saveTask(formName);
          }
        } else {
          return false;
        }
      });
    },
    saveTask() {
      // console.log('this.arrangeType',this.arrangeType)
      const url = this.arrangeType == 'edit' ? Api.taskManage.taskArrange.updateTask : Api.taskManage.taskArrange.addSubTask;
      // const params = {
      //   data: this.form
      // };
      // if (this.arrangeType == 'edit') {
      //   params.data.id = this.detailId;
      // }
      // return;

      let params = {};
      if (this.arrangeType == 'add') {
        const planList = this.form.childrenWrokPlan.map(item => {
          if (item.planBusinessList.length > 0) {
            return item;
          } else {
            return {
              name: item.name,
              endTime: item.endTime,
              planType: item.planType,
              remark: item.remark,
              user: item.user,
              examiner: item.examiner
            };
          }
        });
        params = {
          data: {
            childrenWrokPlan: planList
          }
        };
        // params.data.id = this.taskPid;
      } else {
        params = {
          data: this.form
        };
        // params.data.id = this.detailId;
      }

      if (this.form.project.id) { // 有项目ID才需要传这个字段
        params.data.project = {
          id: this.form.project.id
        };
      } else {
        params.data.project = null;
      }
      this.$axios.post(
        url, params,
        res => {
          if (res.isSuccess) {
            this.$message.success('提交成功！');
            // this.relevanceSubmit(res.data.id);
            this.$emit('addIndicatorEvent');
          } else {
            this.$message.error(res.message);
          }
          this.$emit('update:visible', false); // 关闭弹窗
        }
      );
    },
    relevanceSubmit(flowId) { // 上传的文件关联业务id
      this.flieId.forEach(x => {
        this.$axios.post(
          Api.schedule.saveAttachment,
          {
            data: {
              relationId: flowId,
              fileId: x
              // fileId: this.fileId
            }
          },
          res => {
            this.loading = false;
            if (res.isSuccess) {
              // this.$message.success('关联附件成功');
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 添加项
    addDistributeTask() {
      this.form.childrenWrokPlan.push(
        {
          name: '',
          endTime: '',
          remark: '',
          planType: 'target',
          planBusinessList: [],
          user: {
            name: '',
            id: ''
          },
          examiner: {
            id: localstorageGet('userId') || '',
            name: localstorageGet('userName') || ''
          }
        }
      );
    },
    // 删除项
    deleteTaskIndicator(index) {
      this.form.childrenWrokPlan.splice(index, 1);
    },
    handleClose(formName = 'form') {
      this.$emit('update:visible', false);
    },
    workTargetSelect(data) {
      if (data.selectDatas.length) {
        this.selectManageId = data.manageId;
        this.selectManageContentId = data.selectDatas[0].id;
        this.form.kpiList[0].id = data.selectDatas[0].id;
        if (data.manageType == 'work_target') {
          this.form.kpiList[0].name = data.selectDatas[0].targetItemTwo;
        } else {
          this.form.kpiList[0].name = data.selectDatas[0].content;
        }
      }
    },
    workTemplateSelect(data) {
      console.log('workTemplateSelect');
      const selectWorkTemplateDatas = data.map(item => {
        return {
          name: item.name,
          endTime: '',
          planType: 'target',
          remark: item.standard,
          user: {
            id: '',
            name: ''
          },
          planBusinessList: []
        };
      });
      // console.log('selectWorkTemplateDatas', selectWorkTemplateDatas);
      this.form.childrenWrokPlan.unshift(...selectWorkTemplateDatas);
    },
    addWorkTemplate() {
      this.workTemplateVisible = true;
    },
    // 选择关联业务的类型
    handleClickBusiness(tag, name, index) {
      this.selectTaskIndex = index;
      // 组件的tag和name 传给关联业务的弹窗
      this.businessTag = tag;
      this.businessTagName = name;
      if (tag) {
        this.businessDialogVisible = true;
      }
    },
    // 重选
    handleReChooseBusiness(index, tag) {
      this.businessTag = tag;
      this.businessTagName = tag == 'software_progress_plan' ? '管理进度' : tag == 'prophase_procedures' ? '手续文件' : tag == 'work_plan' ? '季度工作计划' : '';
      this.businessDialogVisible = true;
    },
    // 解除关联
    handleCancelBusiness(scopeIndex, index) {
      if (this.arrangeType == 'edit') {
        this.$confirm('是否确定解除关联?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$axios.post(
            Api.taskManage.taskArrange.cancelBusiness,
            {
              data: {
                id: this.detailId
              }
            },
            res => {
              if (res.isSuccess) {
                this.$message.success('解除关联成功');
                this.getTargetTypePlanDetail();
              }
            }
          );
        }).catch(() => {
        });
      } else if (this.arrangeType == 'add') {
        this.form.childrenWrokPlan[scopeIndex].planBusinessList.splice(index, 1);
      }
      // this.form.childrenWrokPlan[scopeIndex].businessTagName = '';
      // this.form.childrenWrokPlan[scopeIndex].businessPlanName = '';
      // this.form.childrenWrokPlan[scopeIndex].planBusiness.businessId = '';
      // this.form.childrenWrokPlan[scopeIndex].planBusiness.planBusinessType = '';
    },
    // 获取关联业务的数据
    getBusinessData(planName, planId) {
      const repeat = this.form.childrenWrokPlan.forEach(item => {
        if (item.planBusinessList && item.planBusinessList.length) {
          return item.planBusinessList.some(x => x.businessId == planId);
        } else {
          return false;
        }
        // return item.planBusiness.businessId == planId;
      });
      // 前期手续业务不能被重复关联 开发计划可以
      if (repeat && this.businessTag == 'prophase_procedures') {
        this.$message.warning('此业务已被选择，请重选');
      } else {
        this.form.childrenWrokPlan[this.selectTaskIndex].planBusinessList.forEach((planBusiness, index) => {
          if (planBusiness.planBusinessType == this.businessTag) {
            this.form.childrenWrokPlan[this.selectTaskIndex].planBusinessList.splice(index, 1);
          }
        });
        this.form.childrenWrokPlan[this.selectTaskIndex].planBusinessList.push(
          {
            planBusinessType: this.businessTag,
            businessId: planId,
            businessPlanName: this.businessTagName + '/' + planName
          }
        );
        // this.form.childrenWrokPlan[this.selectTaskIndex].businessPlanName = this.form.childrenWrokPlan[this.selectTaskIndex].businessTagName + '/' + planName;
        // this.form.childrenWrokPlan[this.selectTaskIndex].planBusiness.businessId = planId;
        // console.log(this.form.childrenWrokPlan[this.selectTaskIndex].planBusinessList);
        this.businessDialogVisible = false;
      }
    },
    // 获取手续业务的数据
    getProcedureDataList(id) {
      this.$axios.post(
        Api.developProgress.getProcedureData,
        {
          data: {
            projectApiVo: {
              id
            },
            formalitiesDeal: false
          }
        },
        res => {
          if (res.isSuccess) {
            if (res.data.formalitiesMap) {
              return res.data.formalitiesMap;
            }
          }
        }
      );
    }
  }
};
</script>
<style lang='scss' scoped>
// .info-wrap {
//   display: flex;
//   justify-content: space-between;
//   .info-item {
//     margin-bottom: 20px;
//     padding: 0px 10px;
//     flex: 1;
//     .info-item-title {
//       font-weight: bold;
//       margin-bottom: 10px;
//     }
//   }
// }
.info-item {
  margin-bottom: 20px;
  width: 48%;
  float: left;

  &:nth-child(odd) {
    margin-left: 20px;
  }

  .info-item-title {
    font-weight: bold;
    margin-bottom: 10px;
  }
}

.businessItem {
  cursor: pointer;
  text-align: center;
  line-height: 32px;
}

.businessItem:hover {
  color: #4c90ff;
}

.task-distribute-tab {
  margin-top: 4px;
  display: flex;
}

.suffix-icon {
  cursor: pointer;
}

::v-deep .el-form-item__label {
  font-weight: normal;
}

::v-deep .el-table__row:hover>td {
  background-color: #ffffff !important;
}

::v-deep .el-table__row--striped:hover>td {
  background-color: #fafafa !important;
}

// ::v-deep .el-textarea .el-input__count {
//   line-height: 16px;
// }
::v-deep .el-textarea .el-input__count {
  line-height: 13px;
  background: #fff;
  height: 13px;
  width: 52px;
  padding: 0px 2px;
  bottom: 2px;
  right: 10px;
}

::v-deep .el-textarea .el-textarea__inner {
  padding-bottom: 20px;
}
</style>
