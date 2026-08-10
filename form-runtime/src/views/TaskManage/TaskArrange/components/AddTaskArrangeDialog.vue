<!--
 * @Descripttion: 任务下发表单操作
 * @Author: zhengzetao
 * @Date: 2021-06-15
-->
<template>
  <div>
    <el-dialog :title="arrangeType == 'check' ? '查看任务' : arrangeType == 'add' ? '任务下发' : '任务更新'" :visible="visible"
      width="50%" :close-on-click-modal="false" top="100px" @close="handleClose">
      <el-form :model="form" :rules="formRules" ref="form" label-width="100px" label-position="top">
        <h3 style="margin-bottom: 10px">基本信息</h3>
        <div class="info-wrap">
          <div class="info-item">
            <el-form-item label="任务名称" prop="name">
              <!-- <el-input v-model.trim="form.name" :readonly="arrangeType == 'check'" placeholder="请输入任务名称" maxlength="50"
                show-word-limit></el-input> -->
                <el-input type="textarea" size="medium" :autosize="{ minRows: 1 }" v-model.trim="form.name" :readonly="arrangeType == 'check'"
                      placeholder="请输入任务名称" maxlength="50" show-word-limit ></el-input>
            </el-form-item>
          </div>
          <div class="info-item">
            <el-form-item label="是否关联项目">
              <el-select v-model="form.isAssociated" placeholder="请选择"
                @change="changeAssociatedProject"
                :disabled="arrangeType == 'check' || arrangeType == 'edit' || fromAssign != 0 || isProjectSpace">
                <el-option label="是" :value="true"></el-option>
                <el-option label="否" :value="false"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="info-item" v-if="form.isAssociated">
            <el-form-item label="关联项目:" prop="project.name">
              <el-input type="input" v-model="form.project.name" placeholder="请选择关联项目" readonly
                :disabled="fromAssign != 0 || isProjectSpace" @focus="openSelectProjectDialog"></el-input>
            </el-form-item>
          </div>
          <div class="info-item" v-if="arrangeType != 'edit' && arrangeType != 'check'">
            <el-form-item label="任务接收:" prop="receivePlanType">
              <el-radio-group v-model="form.receivePlanType" @change="changeGroup" size="small" :disabled="arrangeType == 'check'">
                <el-radio label="user" border>个人</el-radio>
                <el-radio label="user_group" border :disabled="fromAssign == 1">群组</el-radio>
              </el-radio-group>
            </el-form-item>
          </div>
          <div class="info-item" v-if="form.receivePlanType == 'user'">
            <el-form-item prop="user.name" label="任务负责人:">
              <el-input style="width: 50%" placeholder="请选择任务负责人" v-model="form.user.name" @focus="openIndicatorHeaderDialog(-1)" readonly>
                <i slot="suffix" class="el-input__icon el-icon-user suffix-icon"></i>
              </el-input>
            </el-form-item>
          </div>
          <div class="info-item" v-if="form.receivePlanType == 'user_group'">
            <el-form-item prop="userGroup.id" label="任务负责群组:">
              <el-select v-model="form.userGroup.id" placeholder="请选择" :disabled="arrangeType == 'check'">
                <el-option v-for="(item) in taskGroupList" :label="item.groupName" :value="item.id" :key="item.id">
                </el-option>
              </el-select>
            </el-form-item>
          </div>

          <div class="info-item">
            <el-form-item prop="endTime" label="截止时间:">
              <el-date-picker v-model="form.endTime" value-format="yyyy-MM-dd HH:mm" format="yyyy-MM-dd HH:mm"
                type="datetime" placeholder="选择截止时间" :editable='false' :readonly="arrangeType == 'check'"
                style="width: 200px;">
              </el-date-picker>
            </el-form-item>
          </div>
          <div class="info-item">
            <el-form-item prop="planType" label="工作记录需审批:">
              <div style="display:flex;">
                <div style="flex:1;margin-right:20px;">
                  <el-select v-model="form.planType" placeholder="请选择" :disabled="arrangeType == 'check'"
                    style="width: 100%">
                    <el-option v-for="(item) in needCheckList" :label="item.name" :value="item.id" :key="item.id">
                    </el-option>
                  </el-select>
                </div>
                <div style="flex:1;">
                  <el-tooltip content="如果选择需要审核，则任务接收人必须做工作计划！" placement="top" effect="light">
                    <i class="el-icon-warning-outline"
                      style="margin-left:5px;cursor:pointer;color: black;font-size: 16px;"></i>
                  </el-tooltip>
                </div>
              </div>
            </el-form-item>
          </div>
          <div class="info-item">
            <el-form-item prop="pointTab" label="优先级:">
              <el-select v-model="form.pointTab" placeholder="请选择" :disabled="arrangeType == 'check'">
                  <el-option v-for="(item) in pointTabList" :label="item.label" :value="item.value" :key="item.value">
                  </el-option>
                </el-select>
            </el-form-item>
          </div>
          <div class="info-item">
            <el-form-item prop="examiner.name" label="任务审核人:">
              <el-input style="width: 50%" placeholder="请选择任务审核人" v-model="form.examiner.name" @focus="openIndicatorHeaderDialog(-2)" readonly>
                <i slot="suffix" class="el-input__icon el-icon-user suffix-icon"></i>
              </el-input>
            </el-form-item>
          </div>
          <div class="info-item" v-if="form.receivePlanType != 'user_group'">
            <el-form-item label="目标责任书指标:">
              <el-button type="primary" @click="openWorkTargetDialog">关联目标责任书</el-button>
              <div v-if="form.kpiGroup.name" style="margin-top:10px;">
                <span class="style-common">关联目标责任书：</span>{{ form.kpiGroup.name }}</div>
              <div v-if="form.kpiGroup.resolveContent">
                <span class="style-common">关联指标：</span>{{ form.kpiGroup.resolveContent }}</div>
            </el-form-item>
          </div>
          <div class="info-item" style="width:100%;">
            <el-form-item prop="planType" label="上传附件:">
              <div style="display:flex;">
                <el-upload ref="upload" :action="pdfAction" :data="fileData" accept="" multiple
                  :before-upload="beforeAvatarUpload" :on-success="handleAvatarSuccess" :on-remove="handleRemove"
                  :before-remove="beforeRemove" :file-list="fileList">
                  <el-button size="small" type="primary">选择文件</el-button>
              </el-upload>
            </div>
          </el-form-item>
        </div>
          <!-- 关联业务、任务下发选择群组的话手续文件不能群组下发，管理进度可以群组下发。 -->
          <div v-if="form.project.id" style="clear: both;width:100%;">
            <h3 style="margin-bottom:10px"><i class="el-icon-paperclip"></i>关联业务</h3>
            <el-popover v-if="planBusinessList.length < 2 && arrangeType != 'check'" placement="right" width="150"
              trigger="hover">
              <div v-for="(business, index) in businessList" :key="index"
                @click="handleClickBusiness(business.tag, business.name)" class="businessItem">
                <el-button type="text" :disabled="(fromAssign == 0 && form.receivePlanType == 'user_group' && business.name == '手续文件') || (fromAssign == 2 && form.receivePlanType == 'user_group') || planBusinessList.some(x => x.planBusinessType == business.tag)">
                  {{ business.name }}</el-button>
              </div>
              <el-button slot="reference" type="primary" :disabled="arrangeType == 'check'">选择业务</el-button>
            </el-popover>
            <div v-if="planBusinessList.length > 0">
              <div v-for="(planBusiness, index) in planBusinessList" :key="planBusiness.businessId"
                style="margin-bottom: 5px">
                <i class="el-icon-paperclip"></i>
                <span style="margin-right:20px">{{ planBusiness.businessPlanName }}</span>
                <el-button
                  v-show="fromAssign == 0 || (fromAssign == 1 && planBusiness.planBusinessType == 'software_progress_plan') || (fromAssign == 2 && planBusiness.planBusinessType == 'prophase_procedures')"
                  v-if="arrangeType == 'add'" type="primary" size="mini"
                  @click="handleReChooseBusiness(planBusiness.planBusinessType)">重 选</el-button>
                <el-button
                  v-show="fromAssign == 0 || (fromAssign == 1 && planBusiness.planBusinessType == 'software_progress_plan') || (fromAssign == 2 && planBusiness.planBusinessType == 'prophase_procedures')"
                  v-if="arrangeType !== 'check'" type="danger" size="mini"
                  @click="handleCancelBusiness(index)">解除关联</el-button>
              </div>
            </div>
            <div v-if="planBusinessList.length == 0 && arrangeType == 'check'">暂未关联业务</div>
          </div>
          <div class="info-item" style="width:100%;">
            <el-form-item prop="remark" label="任务要求:">
              <!-- <div style="width:100%"> -->
              <!-- 富文本编辑器 -->
            <RichEditor ref="richEditorRef"></RichEditor>
          </el-form-item>
        </div>
      </div>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button @click="saveTempalte" type="success">保存模板</el-button>

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
    <WorkTargetDailog v-if="workTargetVisible" :visible.sync="workTargetVisible" :userId="form.user.id" :selectManageId="selectManageId"
      :selectManageContentId="selectManageContentId" :selectContentResolveId="selectContentResolveId" :selectResolveKpiId="selectResolveKpiId" @workTargetSelect="workTargetSelect" />
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
import { deepClone } from '@/utils/index';
import IndicatorHeaderDialog from './IndicatorHeaderDialog.vue';
import BusinessRelateDialog from './BusinessRelateDialog.vue';
import WorkTargetDailog from './WorkTargetDailog';
import WorkTemplateDialog from './WorkTemplateDialog';
import SelectProjectDialog from './SelectProjectDialog.vue';
import RichEditor from '@/components/RichEditor/index.vue';
import dayjs from 'dayjs';

export default {
  name: '',
  components: { IndicatorHeaderDialog, WorkTargetDailog, WorkTemplateDialog, BusinessRelateDialog, SelectProjectDialog, RichEditor },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    arrangeType: {
      type: String,
      default: ''
    },
    detailId: {
      type: String,
      default: ''
    },
    dataFromAssigned: { // 指派任务的数据
      type: Object,
      default: () => {
        return {};
      }
    },
    fromAssign: {
      type: Number,
      default: 0 // 1表示直接下达手续文件; 2表示直接下达开发计划
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
      businessDialogVisible: false,
      businessTag: '',
      businessTagName: '',
      businessPlanName: '',
      businessPlanId: '',
      selectManageId: '', // 一级目录Id
      selectManageContentId: '', // 二级目录Id
      selectContentResolveId: '', // 三级目录
      selectResolveKpiId: '', // 四级目录
      indicatorHeaderIdx: -1,
      taskHeaderId: '',
      maxEndTime: null,
      minEndTime: null,
      projectCanUpdate: true, // 关联项目是否可编辑
      form: {
        name: '',
        isAssociated: false,
        project: {
          id: '',
          name: ''
        },
        receivePlanType: 'user', // userGroup
        userGroup: {
          id: ''
        },
        user: {
          name: '',
          id: ''
          // dutyId: '' // 岗位id
        },
        // kpiList: [{ // 目标责任书指标
        //   id: '',
        //   name: ''
        // }],
        kpiGroup: {
          name: '',
          resolveContent: ''
        },
        endTime: '',
        remark: '', // 任务要求
        planType: 'target',
        pointTab: 'ordinary', // 优先级
        examiner: { // 任务审核人
          id: '',
          name: ''
        }
        // childrenWrokPlan: [
        // {
        //   name: '',
        //   planType: '',
        //   endTime: '',
        //   remark: '',
        //   user: {
        //     name: '',
        //     id: ''
        //   }
        // }
        // ]
      },
      assoicaProject: [
      ],
      chargerDutyList: [],
      businessList: [
        {
          name: '手续文件',
          tag: 'prophase_procedures'
        },
        {
          name: '管理进度',
          tag: 'software_progress_plan'
        },
        // {
        //   name: '季度工作计划',
        //   tag: 'work_plan'
        // }
      ],
      planBusinessList: [],
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
      projectRelationType: '',
      formRules: {
        name: [{ required: true, message: '请填写任务名称', trigger: 'blur' }],
        'project.name': [{ required: true, message: '请选择关联项目', trigger: 'none' }],
        'user.name': [{ required: true, message: '请选择任务负责人', trigger: 'none' }],
        'examiner.name': [{ required: true, message: '请选择任务审核人', trigger: 'none' }],
        'userGroup.id': [{ required: true, message: '请选择任务负责群组', trigger: 'change' }],
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
        pointTab: [
          {
            required: true,
            trigger: 'change',
            message: '请选择优先级'
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
      fileList: [
      ],
      flieId: [],
      pointTabList: [
        { label: '普通', value: 'ordinary' },
        { label: '较高级', value: 'urgent' },
        { label: '紧急', value: 'milestone' }
      ],
      taskGroupList: [
        // {
        //   label:'',
        //   value:''
        // }
      ]
    };
  },
  computed: {
    pdfAction() {
      const sid = this.$store.state.user.token;
      return `${baseUrl}/web/file/api/file/uploadFile?sid=${sid}&platformCode=200001`;
    }
  },
  watch: {
    fromAssign: {
      // 处理手续办理指派任务
      handler(newVal) {
        const data = deepClone(this.dataFromAssigned);
        console.log('data', data);
        if (newVal != 0 && data != '{}') {
          this.form = data.form;
          this.form.kpiGroup = {
            name: '',
            resolveContent: ''
          };
          this.form.examiner = {
            id: '',
            name: ''
          };
          this.form.receivePlanType = 'user';
          this.$set(this.form, 'userGroup', { id: '' });
        }
        if (newVal === 1) {
          if (data != '{}') {
            this.planBusinessList.push(
              {
                businessId: data.planBusiness.businessId,
                planBusinessType: 'prophase_procedures',
                businessPlanName: `手续文件/${data.planBusiness.processName}/${data.planBusiness.businessType}`
              }
            );
            this.businessPlanId = data.planBusiness.businessId;
            this.businessPlanName = `手续文件/${data.planBusiness.processName}/${data.planBusiness.businessType}`;
            this.businessTag = 'prophase_procedures';
            this.businessTagName = '手续文件';
          }
        } else if (newVal === 2) {
          if (data != '{}') {
            this.planBusinessList.push(
              {
                businessId: data.planBusiness.businessId,
                planBusinessType: 'software_progress_plan',
                businessPlanName: `管理进度/${data.planBusiness.businessType}`
              }
            );
            this.businessPlanId = data.planBusiness.businessId;
            this.businessPlanName = `管理进度/${data.planBusiness.businessType}`;
            this.businessTag = 'software_progress_plan';
            this.businessTagName = '管理进度';
          }
        }
        console.log('fromAssign');
      },
      immediate: true
    }
    // dataFromAssigned: {
    //   handler(newVal, oldVal) {

    //   },
    //   immediate: true,
    //   deep: true
    // }
  },
  created() { },
  mounted() {
    this.getAssociateProject();
    this.getUserGroupList();
    // 编辑或查看
    console.log(this.arrangeType, 'this.arrangeType');
    if (this.arrangeType == 'check' || this.arrangeType == 'edit') {
      this.getTargetTypePlanDetail();
      this.getFileByBizId(this.detailId);
    } else {
      if (this.arrangeType == 'add') {
        // 新增时设置默认审核人
        this.form.examiner = {
          id: localstorageGet('userId') || '',
          name: localstorageGet('userName') || ''
        };
      }
      if (localstorageGet('groupDepartment') != 'group') { // 如果在项目里面，关联项目只显示这个项目，不能选择其他
        this.form.project.id = this.$store.state.user.projectId;
      }
    }
  },
  methods: {
    changeGroup() {
      // eslint-disable-next-line dot-notation
      this.$refs['form'].clearValidate();
      this.form.userGroup = {
        id: ''
      };
      this.form.user = {
        name: '',
        id: ''
      };

      // 只要是群组，就不能关联手续文件，手续文件任务只能指派一个人做
      if (this.form.receivePlanType == 'user_group') {
        const valIndex = this.planBusinessList.findIndex(x => x.businessPlanName.indexOf('手续文件') > -1);
        console.log('valIndex', valIndex);
        if (valIndex > -1) {
          this.handleCancelBusiness(valIndex);
        }
      }
    },
    // 群组列表
    getUserGroupList() {
      const data = {
        ownerId: this.$store.state.user.userId,
        enableType: 'enable'
      };
      this.$axios.post(
        Api.taskManage.taskArrange.getUserGroupList,
        {
          data
        },
        res => {
          if (res.isSuccess) {
            this.taskGroupList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
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
          res.data.map(item => {
            this.flieId.push(item.fileId);
            this.fileList.push({ name: item.fileName, fileUrl: item.fileUrl, fileId: item.fileId });
          });
        }
      });
    },
    beforeRemove(file, fileList) {
      if (!this.detailId) return true;
      const id = file.response ? file.response.data.id : file.fileId;
      return new Promise((resolve, reject) => {
        this.$confirm('是否确定解除关联?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$axios.post(
            Api.budgetManage.deleteBatchFile,
            {
              data: {
                relationId: this.detailId,
                fileIds: [id]
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
      console.log(file, 'file');
      this.fileList = fileList;
      const id = file.response ? file.response.data.id : file.fileId;
      this.flieId = this.flieId.filter(e => e != id);
    },
    // 文件上传
    handleAvatarSuccess(res, file) {
      console.log(res, file, '++++');
      if (res.code == 'RESP200') {
        this.flieId.push(res.data.id);
        if (this.detailId) {
          this.$axios.post(
            Api.schedule.saveAttachment,
            {
              data: {
                relationId: this.detailId,
                fileId: res.data.id
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
        }
      } else {
        this.$message.error(`文件上传失败,请重新上传`);
      }
    },
    beforeAvatarUpload(file) {
      console.log(file, '3333');
      // if (!/\.(xlsx|xls|XLSX|XLS)$/.test(file.name)) {
      //   this.$message.error(
      //     '上传文件只能为excel文件，且为xlsx,xls格式'
      //   );
      //   return false;
      // }
    },
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
    },
    openSelectProjectDialog(index) {
      if (this.arrangeType == 'check' || this.arrangeType == 'edit') {
        return;
      }
      this.selectProjectVisible = true;
    },
    getTargetTypePlanDetail() {
      console.log('getTargetTypePlanDetail');
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
              name: data.name,
              project: {
                id: data.project ? data.project.id : '',
                name: data.project ? data.project.name : ''
              },
              user: {
                name: data.user.name,
                id: data.user.id
                // dutyId: data.user.dutyId // 岗位id
              },
              receivePlanType: 'user', // userGroup
              userGroup: {
                id: ''
              },
              // kpiList: [
              //   {
              //     id: data.kpiGroup?.keyPerformanceIndicatorsList[0].id,
              //     name: data.kpiGroup?.manageType == 'work_target' ? data.kpiGroup?.keyPerformanceIndicatorsList[0].targetItemTwo : data.kpiGroup?.keyPerformanceIndicatorsList[0].content
              //   }
              // ],
              kpiGroup: {
                id: data.kpiGroup?.id,
                // performanceId:data.kpiGroup?.keyPerformanceIndicatorsList[0]?.id,
                name: data.kpiGroup?.keyPerformanceIndicatorsList[0].targetItemTwo,
                resolveContent: data.kpiGroup?.keyPerformanceIndicatorsList[0].kpiSplitItems ? data.kpiGroup.keyPerformanceIndicatorsList[0].kpiSplitItems[0].content : null
              },
              endTime: data.endTime,
              planType: data.planType,
              remark: data.remark,
              pointTab: data.pointTab,
              examiner: { // 任务审核人
                id: data.examiner && data.examiner.id ? data.examiner.id : '',
                name: data.examiner && data.examiner.name ? data.examiner.name : ''
              }
              // childrenWrokPlan: data.childrenWrokPlan || []
            };
            this.$refs.richEditorRef.contentHtml = data.remark;
            this.form.isAssociated = !!(this.form.project && this.form.project.id);
            if (data.planBusinessList && data.planBusinessList.length) {
              this.planBusinessList = [];
              data.planBusinessList.forEach(planBusiness => {
                this.getBusinessPlanName(data, planBusiness);
              });
            } else {
              this.planBusinessList = [];
            }
            if (data.project) {
              this.projectCanUpdate = data.project.canUpdate;
            } else {
              this.projectCanUpdate = true;
            }
            this.selectManageId = data.kpiGroup?.id;
            this.selectManageContentId = data.kpiGroup?.keyPerformanceIndicatorsList[0].id;
            const groupL = data.kpiGroup.keyPerformanceIndicatorsList[0];
            if (groupL) {
              if (groupL.kpiSplitItems[0]) {
                // if (groupL['kpiSplitItems'][0]['kpiSplitType'] == 'quarterly') {
                this.selectContentResolveId = groupL.kpiSplitItems[0].id;
                this.selectResolveKpiId = groupL.kpiSplitItems[0].kpiSplitItemWeights ? groupL.kpiSplitItems[0].kpiSplitItemWeights[0].id : groupL.kpiSplitItems[0].id;
                console.log('this.selectContentResolveId', this.selectContentResolveId);
                // }
              }
            }
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
    // 选择关联项目时重新选择任务负责人
    selectAssociateProject(val) {
      this.form.user = {
        id: '',
        name: ''
        // dutyId: ''
      };
      // 把子任务负责人清空
      // if (this.form.childrenWrokPlan.length) {
      //   this.form.childrenWrokPlan.forEach(item => {
      //     item.user.id = '';
      //     item.user.name = '';
      //   });
      // }
    },
    // 任务负责人选择赋值
    selectHeader(data) {
      if (this.indicatorHeaderIdx >= 0) {
        this.$set(this.form.childrenWrokPlan[this.indicatorHeaderIdx], 'user', {
          name: data.name,
          id: data.id
        });
      } else if (this.indicatorHeaderIdx == -1) {
        // 设置任务负责人
        this.form.user = {
          name: data.name,
          id: data.id
          // dutyId: ''
        };

        console.log(123, this.form.kpiGroup);
        if (this.form.kpiGroup.name && this.form.kpiGroup.id) {
          console.log(123);
          // let KpiGroupId = this.form.kpiGroup.id // 目标责任书保存后，如果要修改，这个目标责任书id要保留原来的
          // let KpiPerformanceId = this.form.kpiGroup.performanceId // 目标责任书保存后，如果要修改，这个绩效指标id要保留原来的
          this.form.kpiGroup = {
            // id:KpiGroupId,
            name: '',
            resolveContent: ''
            // keyPerformanceIndicatorsList:[{id:KpiPerformanceId}]
          };
        }
        // console.log('this.form.kpiGroup',this.form)
        // return;
      } else if (this.indicatorHeaderIdx == -2) {
        // 设置审核人
        this.form.examiner = {
          name: data.name,
          id: data.id
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
    openIndicatorHeaderDialog(index, row) {
      if (this.arrangeType == 'check') {
        return;
      }
      // 判断是否表格点击
      if (index >= 0) {
        this.indicatorHeaderIdx = index;
        this.taskHeaderId = row.user.id;
      } else if (index == -1) {
        this.indicatorHeaderIdx = index;
        this.taskHeaderId = this.form.user.id;
      } else if (index == -2) {
        this.indicatorHeaderIdx = index;
        this.taskHeaderId = this.form.examiner.id;
      }

      if (this.form.project.id) {
        this.projectRelationType = this.assoicaProject.find(x => x.id == this.form.project.id).relationType;
      }
      this.indicatorHeaderVisible = true;
    },
    // 点击目标责任书指标
    openWorkTargetDialog() {
      if (this.arrangeType == 'check') return;
      if (this.form.user.id == '') {
        this.$message.warning('请先选择任务负责人');
        return;
      };
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
    submitForm(formName) {
      const content = this.$refs.richEditorRef.contentHtml;
      // richEditorRef在没有输入值时，还会返回空行标签，修复bug
      this.form.remark = content.indexOf('<p><br></p>') == 0 && content.length == 11 ? '' : this.$refs.richEditorRef.contentHtml;

      this.$refs[formName].validate((valid) => {
        if (valid) {
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
      console.log('saveTask', this.arrangeType);
      const url = this.arrangeType == 'edit' ? Api.taskManage.taskArrange.updateTask : Api.taskManage.taskArrange.saveTask;
      this.form.endTime = dayjs(this.form.endTime).format('YYYY-MM-DD HH:mm:ss');
      const params = {
        data: this.form
      };
      // 删除目标责任书指标
      if (this.arrangeType == 'edit') {
        params.data.id = this.detailId;
      }
      if (this.form.project.id) { // 有项目ID才需要传这个字段
        params.data.project = {
          id: this.form.project.id
        };
        if (this.planBusinessList.length) {
          const planBusinessList = [];
          this.planBusinessList.forEach(planBusiness => {
            planBusinessList.push({
              businessId: planBusiness.businessId,
              planBusinessType: planBusiness.planBusinessType
            });
          });
          params.data.planBusinessList = planBusinessList;
        }
      } else {
        params.data.project = {
          id: ''
        };
      }
      console.log('params', params);
      // return
      this.$axios.post(
        url, params,
        res => {
          console.log('res', res);
          if (res.isSuccess) {
            if (res.data) {
              console.log('提交成功！');
              res.data.forEach(item => {
                this.relevanceSubmit(item.id);
              });
              // this.relevanceSubmit(res.data.id);
            }
            this.$message.success('提交成功！');
            this.$emit('addIndicatorEvent');
          } else {
            this.$message.error(res.message);
          }
          this.$emit('update:visible', false); // 关闭弹窗
        }
      );
    },
    relevanceSubmit(flowId) { // 上传的文件关联业务id
      console.log('relevanceSubmit');
      console.log('this.flieId', this.flieId);
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
    addDistributeTask() {
      this.form.childrenWrokPlan.push(
        {
          name: '',
          endTime: '',
          planType: 'target',
          remark: '',
          user: {
            name: '',
            id: ''
          }
        }
      );
    },
    deleteTaskIndicator(index) {
      this.form.childrenWrokPlan.splice(index, 1);
    },
    handleClose(formName = 'form') {
      this.$emit('update:visible', false);
    },
    saveTempalte() {
      console.log(this.form, '+++++++', this.$refs.richEditorRef.contentHtml);
      const reg = /(<\/?.*?>)/gi;
      const content = this.$refs.richEditorRef.contentHtml.replace(reg, '');
      if (!this.form.name && !content) {
        this.$message.error('任务名称和任务要求必填！');
      }
      this.$axios.post(
        '/web/plan/api/planTemplate/save',
        {
          data: {
            name: this.form.name,
            templateItemList: [
              { name: this.form.name, standard: content }
            ]
          }
        },
        res => {
          if (res.isSuccess) {
            this.$message.success('模板保存成功');
          }
        }
      );
    },
    workTargetSelect(data) {
      console.log('workTargetSelect1', data);
      console.log('this.form', this.form);
      if (data.selectDatas.length) {
        this.selectManageId = data.manageId;
        this.selectManageContentId = data.selectDatas[0].id;
        // this.form.kpiList[0].id = data.selectDatas[0].id;
        this.form.kpiGroup = data.newSelectDatas;
        this.form.kpiGroup.name = data.selectDatas[0].targetItemTwo;
        this.form.kpiGroup.resolveContent = data.newSelectDatas.keyPerformanceIndicatorsList[0].kpiSplitItems ? data.newSelectDatas.keyPerformanceIndicatorsList[0].kpiSplitItems[0].content : null;
        // this.form.kpiGroup.performanceId = data.newSelectDatas.keyPerformanceIndicatorsList[0]?.id
        // console.log('workTargetSelect', data);
        // console.log('this.form', this.form);
        // if (data.manageType == 'work_target') {
        //   this.form.kpiList[0].name = data.selectDatas[0].targetItemTwo;
        // } else {
        //   this.form.kpiList[0].name = data.selectDatas[0].content;
        // }
      }
    },
    workTemplateSelect(data) {
      const selectWorkTemplateDatas = data.map(item => {
        return {
          name: item.name,
          endTime: '',
          remark: item.standard,
          user: {
            id: '',
            name: ''
          }
        };
      });
      this.form.childrenWrokPlan.unshift(...selectWorkTemplateDatas);
    },
    addWorkTemplate() {
      this.workTemplateVisible = true;
    },
    // 选择关联业务的类型
    handleClickBusiness(tag, name) {
      this.businessTag = tag;
      if (tag) {
        this.businessDialogVisible = true;
        this.businessTagName = name;
      }
    },
    // 重选
    handleReChooseBusiness(tag) {
      this.businessTag = tag;
      this.businessTagName = tag == 'software_progress_plan' ? '管理进度' : tag == 'prophase_procedures' ? '手续文件' : tag == 'work_plan' ? '季度工作计划' : '';
      this.businessDialogVisible = true;
    },
    // 解除关联
    handleCancelBusiness(index) {
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
                planBusinessList: [
                  {
                    businessId: this.planBusinessList[index].businessId
                  }
                ],
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
          this.businessTag = '';
          this.businessTagName = '';
          this.businessPlanName = '';
          this.businessPlanId = '';
        }).catch(() => {
        });
      } else if (this.arrangeType == 'add') {
        this.planBusinessList.splice(index, 1);
      }
    },
    // 获取关联业务的数据
    getBusinessData(planName, planId) {
      this.planBusinessList.forEach((planBusiness, index) => {
        if (planBusiness.planBusinessType == this.businessTag) {
          this.planBusinessList.splice(index, 1);
        }
      });
      this.planBusinessList.push(
        {
          planBusinessType: this.businessTag,
          businessId: planId,
          businessPlanName: this.businessTagName + '/' + planName
        }
      );
      // this.businessPlanName = this.businessTagName + '/' + planName;
      // this.businessPlanId = planId;
      this.businessDialogVisible = false;
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
      console.log(this.businessPlanName);
      this.planBusinessList.push({
        businessId: planBusiness.businessId,
        planBusinessType: planBusiness.planBusinessType,
        businessPlanName: this.businessPlanName
      });
    }
  }
};
</script>
<style lang='scss' scoped>
.info-wrap {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  max-height: 580px;
  overflow-y: auto;
  padding: 0 10px;

  .info-item {
    margin: 10px 0;
    // padding: 0px 10px;
    // flex: 1;
    width: 48%;
    // float: left;

    // &:nth-child(odd) {
    //   margin-left: 20px;
    // }

    .info-item-title {
      font-weight: bold;
      margin-bottom: 10px;
    }
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

.style-common {
    color: #fff;
    border-radius: 16px;
    background: rgb(47, 194, 91);
    text-align: center;
    padding: 3px 2px 2px 8px;
    margin-right: 6px;
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

::v-deep .el-textarea .el-input__count {
  line-height: 16px;
}

// ::v-deep .el-input .el-input__count {
//   display: inline-block;
//   align-items: inherit;
//   // line-height: 58px !important;
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
