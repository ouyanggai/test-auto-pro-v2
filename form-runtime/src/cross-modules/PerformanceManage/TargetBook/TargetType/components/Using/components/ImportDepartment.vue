<!--
 * @Descripttion:
 * @Author: zhengzetao
 * @Date: 2021-06
-->
<template>
  <div class="container">
    <el-dialog
      :title="title"
      :visible="visible"
      :close-on-click-modal="false"
      center
      width="400px"
      top='20vh'
      @close='handleClose'
    >
      <el-form
        :model="form"
        :rules='rules'
        ref="form"
        label-width="100px"
      >
        <el-form-item
          label="导入文件"
          prop="file"
        >
          <div style="display: flex;align-items: center;">
            <div style="display:inline-block;">
              <el-upload
                action="#"
                accept=".xlsx"
                :limit="1"
                :http-request="uploadHttpRequest"
                :on-success="handleAvatarSuccess"
                :on-remove="removeFile"
                :file-list="fileList"
                :on-change="fileChange"
              >
                <el-button type="primary">上传文件</el-button>
              </el-upload>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <div
        slot="footer"
        class="dialog-footer"
      >
        <el-button
          type="primary"
          :loading="uploadLoading"
          @click="submit('form')"
        >提 交</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import { localstorageGet } from '@/utils/auth';

export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    title: {
      type: String
    }
  },
  data() {
    return {
      postUrl: '',
      form: {
        file: ''
      },
      fileList: [
      ],
      rules: {
        file: [
          { required: true, message: '请上传文件', trigger: 'change' }
        ]
      },
      uploadLoading: false
    };
  },
  computed: {},
  watch: {
  },
  created() { },
  mounted() {
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    submit(formName) {
      this.$refs[formName].validate(valid => {
        if (valid) {
          const sid = localstorageGet('token') || '';
          const formData = new FormData();
          this.uploadLoading = true;
          formData.append('file', this.form.file);
          formData.append('departmentId', this.departmentId);
          formData.append('sid', sid);
        } else {
          return false;
        }
      });
    },
    pdfAction: function () {
      // 因为action参数是必填项，我们使用二次确认进行文件上传时，直接填上传文件的url会因为没有参数导致api报404，所以这里将action设置为一个返回为空的方法就行，避免抛错
      return '';
    },
    uploadHttpRequest(param) {
      this.form.file = param.file;
      this.$refs.form.validateField('file', res => {
        if (res) {
          return true;
        } else {
          return false;
        }
      });
    },
    // 文件发生改变
    fileChange(file, fileList) {
      this.fileList.push({
        name: file.name,
        url: ''
      });
      if (fileList.length > 0) {
        this.fileList = [fileList[fileList.length - 1]]; // 展示最后一次选择的文件
      }
    },
    removeFile(file, fileList) {
      if (this.uploadLoading) return;
      this.fileList = [];
      this.form.file = '';
    },

    // 文件上传
    handleAvatarSuccess(resp, file) {
    }
  }
};
</script>

<style scoped lang="scss">
</style>

