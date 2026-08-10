<template>
  <el-dialog
    :title="groupType=='add' ? '新增分组' : '编辑分组'"
    :visible.sync="dialogVisible"
    width="900px"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    @close="handleClose"
  >
    <el-form :model="form" :rules="rules" ref="groupForm" label-width="80px">
      <el-form-item label="分组名称" prop="name">
        <el-input v-model="form.name" placeholder="请输入分组名称"></el-input>
      </el-form-item>
      <el-form-item label="类型" prop="type">
        <el-select v-model="form.type" placeholder="请选择类型">
          <el-option label="指定人员" value="target_user"></el-option>
          <el-option label="指定岗级别" value="duty_level"></el-option>
        </el-select>
      </el-form-item>
      <!-- 分组等级字段暂时隐藏 -->
      <!-- <el-form-item label="分组等级" prop="structureLevel">
        <el-select v-model="form.structureLevel" placeholder="请选择级别">
          <el-option label="高管级别" value="executive_level"></el-option>
          <el-option label="普通管理者级别" value="ordinary_admin_level"></el-option>
          <el-option label="一般员工级别" value="ordinary_level"></el-option>
        </el-select>
      </el-form-item> -->
      <el-form-item v-if="form.type == 'target_user'" label="关联设置" prop="users">
        <div
          class="el-input el-input--suffix"
          @click="openPersonDialog"
          style="width: 100%; cursor: pointer;"
        >
          <div class="el-input__inner" style="min-height: 32px; text-align: left; padding: 0 8px; height: auto;">
            <el-tag
              v-for="item in form.users"
              :key="item.id"
              size="small"
              style="margin: 2px 4px;"
            >
              {{ item.name }}
            </el-tag>
            <span v-if="!form.users.length" style="color: #C0C4CC;">请选择人员</span>
          </div>
          <span class="el-input__suffix">
            <span class="el-input__suffix-inner">
              <i class="el-select__caret el-input__icon el-icon-arrow-up"></i>
            </span>
          </span>
        </div>
      </el-form-item>
      <el-form-item v-else label="关联设置" prop="grades">
        <div
          class="el-input el-input--suffix"
          @click="openGradeDialog"
          style="width: 100%; cursor: pointer;"
        >
          <div class="el-input__inner" style="min-height: 32px; text-align: left; padding: 0 8px; height: auto;">
            <el-tag
              v-for="item in form.grades"
              :key="item.id"
              size="small"
              style="margin: 2px 4px;"
            >
              {{ item.name }}
            </el-tag>
            <span v-if="!form.grades.length" style="color: #C0C4CC;">请选择定岗级别</span>
          </div>
          <span class="el-input__suffix">
            <span class="el-input__suffix-inner">
              <i class="el-select__caret el-input__icon el-icon-arrow-up"></i>
            </span>
          </span>
        </div>
      </el-form-item>
    </el-form>
    <span slot="footer" class="dialog-footer">
      <el-button @click="handleCancel">取 消</el-button>
      <el-button type="primary" @click="handleSubmit">{{ groupType === 'add' ? '新 增' : '确 定' }}</el-button>
    </span>

    <!-- 人员选择弹窗 -->
    <person-dialog
      :visible.sync="personDialogVisible"
      :selected-users="form.users"
      @confirm="handlePersonSelect"
    />

    <!-- 岗位选择弹窗 -->
    <grade-dialog
      :visible.sync="gradeDialogVisible"
      :selected-grade="form.grades"
      :company-id="companyId"
      @confirm="handleGradeSelect"
    />
  </el-dialog>
</template>

<script>
import PersonDialog from './personDialog.vue';
import GradeDialog from './gradeDialog.vue';
import Api from '@/api';
export default {
  name: 'GroupSetDialog',
  components: {
    PersonDialog,
    GradeDialog
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    companyId: {
      type: String,
      default: ''
    },
    groupData: {
      type: Object,
      default: () => {}
    },
    groupType: {
      type: String,
      default: 'add'
    }
  },
  data() {
    return {
      dialogVisible: this.visible,
      personDialogVisible: false,
      gradeDialogVisible: false,
      form: {
        id: '',
        name: '',
        type: 'target_user',
        // structureLevel: '',
        users: [],
        grades: []
      },
      rules: {
        name: [
          { required: true, message: '请输入分组名称', trigger: 'blur' },
          { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }
        ],
        type: [
          { required: true, message: '请选择类型', trigger: 'change' }
        ],
        // structureLevel: [
        //   { required: true, message: '请选择级别', trigger: 'change' }
        // ],
        users: [
          { required: true, message: '请选择人员', trigger: 'change' }
        ],
        grades: [
          { required: true, message: '请选择岗级', trigger: 'change' }
        ]
      }
    };
  },
  watch: {
    visible(newVal) {
      this.dialogVisible = newVal;
    },
    'form.type': {
      handler(newVal) {
        // 类型变更时，清空对应的选择项
        if (newVal === 'target_user') {
          this.form.grades = [];
        } else if (newVal === 'duty_level') {
          this.form.users = [];
        }
        // 触发表单校验
        this.$nextTick(() => {
          if (this.$refs.groupForm) {
            this.$refs.groupForm.validateField('users');
          }
        });
      }
    },
    groupData: {
      handler(newVal) {
        console.log(newVal, 'newVal');
        if (this.groupType == 'add') {
          this.form = {
            id: '',
            name: '',
            type: 'target_user',
            // structureLevel: '',
            users: [],
            grades: []
          };
        } else {
          this.form.id = this.groupData.id;
          this.form.name = this.groupData.name;
          this.form.type = this.groupData.planGroupType;
          // this.form.structureLevel = this.groupData.structureLevel;
          this.form.users = (this.groupData.planGroupType == 'target_user' && this.groupData.users && this.groupData.users.length) ? this.groupData.users.map(item => {
            return {
              id: item.id,
              name: item.name
            };
          }) : [];
          this.form.grades = (this.groupData.planGroupType == 'duty_level' && this.groupData.dutyLevels && this.groupData.dutyLevels.length) ? this.groupData.dutyLevels.map(item => {
            return {
              id: item.id,
              name: item.name,
              checked: true
            };
          }) : [];
        }
        // 表单初始化后，重置校验状态
        this.$nextTick(() => {
          if (this.$refs.groupForm) {
            this.$refs.groupForm.clearValidate();
          }
        });
      },
      deep: true,
      immediate: true
    }
  },
  methods: {
    handleClose() {
      // 重置表单校验状态
      if (this.$refs.groupForm) {
        this.$refs.groupForm.clearValidate();
      }
      // 通知父组件关闭弹窗
      this.$emit('update:visible', false);
    },
    handleCancel() {
      // 重置表单
      if (this.$refs.groupForm) {
        this.$refs.groupForm.resetFields();
      }
      this.handleClose();
    },
    handleSubmit() {
      // 先检查公司ID是否存在
      if (!this.companyId) {
        this.$message.error('公司ID不能为空');
        return;
      }

      // 表单校验
      this.$refs.groupForm.validate((valid) => {
        if (valid) {
          // 根据类型检查关联项是否已选择
          let hasRelation = false;
          if (this.form.type === 'target_user') {
            hasRelation = this.form.users && this.form.users.length > 0;
          } else if (this.form.type === 'duty_level') {
            hasRelation = this.form.grades && this.form.grades.length > 0;
          }

          if (!hasRelation) {
            this.$message.error(this.form.type === 'target_user' ? '请选择关联人员' : '请选择关联岗级');
            return false;
          }

          // 提交表单
          this.$axios.post(
            this.groupType == 'add' ? Api.newPerformance.planUserGroupSave : Api.newPerformance.planUserGroupUpdate,
            {
              data: {
                id: this.groupType == 'add' ? '' : this.form.id,
                name: this.form.name,
                planGroupType: this.form.type,
                // structureLevel: this.form.structureLevel,
                company: {
                  id: this.companyId
                },
                relationIds: this.form.type == 'target_user' ? this.form.users.map(item => item.id) : this.form.grades.map(item => item.id)
              }
            },
            res => {
              console.log(res, 'res');
              if (res.isSuccess) {
                this.$message.success(this.groupType == 'add' ? '分组创建成功！' : '分组编辑成功！');
                this.$emit('submit', true);
                this.handleClose();
              } else {
                this.$message.error(res.message);
              }
            }
          );
        } else {
          // 校验失败时滚动到第一个错误字段
          const firstErrorField = this.$refs.groupForm.fields.find(field => field.validateState === 'error');
          if (firstErrorField) {
            firstErrorField.$el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          }
          return false;
        }
      });
    },
    openPersonDialog() {
      this.personDialogVisible = true;
    },
    openGradeDialog() {
      this.gradeDialogVisible = true;
    },
    handlePersonSelect(selectedUsers) {
      console.log(selectedUsers, 'selectedUsers');
      this.form.users = selectedUsers;
      // 触发表单校验
      this.$nextTick(() => {
        this.$refs.groupForm.validateField('users');
      });
    },
    handleGradeSelect(selectedGrade) {
      console.log(selectedGrade, 'selectedGrade');
      this.form.grades = selectedGrade;
      // 触发表单校验
      this.$nextTick(() => {
        this.$refs.groupForm.validateField('users');
      });
    }
  }
};
</script>

<style lang='scss' scoped>
.el-tag {
  margin-right: 4px;
}
</style>
