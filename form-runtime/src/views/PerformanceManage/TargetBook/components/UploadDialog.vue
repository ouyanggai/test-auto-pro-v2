
<template>
  <el-dialog
    class="upload-container"
    title="导入数据"
    :visible="visible"
    :close-on-click-modal="false"
    append-to-body
    top="100px"
    @close='handleClose'
  >
    <el-upload
      class="upload-demo"
      drag
      accept=".xlsx,.xls"
      :limit="1"
      action="123"
      :show-file-list="false"
      :auto-upload="false"
      :file-list="fileList"
      :before-upload="beforeUpload"
      :on-change="onChange"
      :on-exceed="onExceed"
      multiple
    >
      <i class="el-icon-upload"></i>
      <div class="el-upload__text">将文件拖到此处，或<em>点击上传</em></div>
      <div
        class="el-upload__tip"
        slot="tip"
      >*上传文件只能为excel文件，且为xlsx,xls格式</div>
    </el-upload>
    <el-radio-group v-model="radio" v-if="this.assessmentCycle == 'month'">
      <el-radio label="generic_month_kpi_template">通用模板</el-radio>
      <el-radio label="complex_month_kpi_template">复杂模板(带合并单元格)</el-radio>
    </el-radio-group>
    <div style="height:200px;">
      <el-table
        :data="fileList"
        style="width: 100%"
      >
        <el-table-column
          prop="name"
          label="文件名"
        >
        </el-table-column>
        <el-table-column
          prop="size"
          label="大小"
          :formatter="formatFileSize"
        >
        </el-table-column>
        <el-table-column label="状态">
          <template
            slot-scope="scope"
            v-if="uploadVisible"
          >
            <span v-if="false">{{scope}}</span>
            <el-progress :percentage="progress"></el-progress>
          </template>
        </el-table-column>
        <el-table-column
          fixed="right"
          label="操作"
          align="center"
          width="100"
        >
          <template slot-scope="scope">
            <el-button
              @click="handleClick(scope.row)"
              type="text"
              size="small"
            >删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div
      slot="footer"
      class="dialog-footer"
    >
      <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="submitUpload"
      >上 传</el-button>
    </div>
  </el-dialog>
</template>

<script>
export default {
  name: 'UploadFile',
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    assessmentCycle:{
      type:String,
      default:'year'
    }
  },
  data() {
    return {
      radio: 'generic_month_kpi_template',
      uploadVisible: false,
      urlAction: `/web/plan/api/kpi/uploadKpiTemplate`, // 默认
      fileList: [],
      progress: 0
    };
  },
  created() {

  },
  methods: {
    handleClose() {
      this.clear();
      this.$emit('update:visible', false);
    },
    formatFileSize(row) {
      let temp;
      if (row.size < 1024) {
        return row.size + 'B';
      } else if (row.size < (1024 * 1024)) {
        temp = row.size / 1024;
        temp = temp.toFixed(2);
        return temp + 'KB';
      } else if (row.size < (1024 * 1024 * 1024)) {
        temp = row.size / (1024 * 1024);
        temp = temp.toFixed(2); return temp + 'MB';
      } else {
        temp = row.size / (1024 * 1024 * 1024);
        temp = temp.toFixed(2);
        return temp + 'GB';
      }
    },
    beforeUpload(file) {
      console.log(file, 'file');
    },
    onChange(file, fileList) {
      this.fileList = fileList;
    },
    onExceed(files, fileList) {
      this.$message.error('最多一次同时上传1个文件');
    },
    submitUpload() {
      if (!this.fileList.length) {
        this.$message.error('请选择上传文件');
        return;
      }
      if (this.fileList.length > 1) {
        this.$message.error('最多一次同时上传1个文件');
        return;
      }
      this.uploadVisible = true;
      const formData = new FormData();
      const sid = this.$store.state.user.token;
      this.fileList.map(item => {
        formData.append('file', item.raw);
        formData.append('sid', sid);
        formData.append('assessmentCycle', this.assessmentCycle);
        if (this.assessmentCycle == 'month') {
          formData.append('uploadTemplateType', this.radio);
        } else if (this.assessmentCycle == 'year') {
          formData.append('uploadTemplateType', 'generic_year_kpi_template');
        }
      });
      const config = {
        onUploadProgress: progressEvent => {
          this.progress = ((progressEvent.loaded / progressEvent.total) * 100) | 0;
        }
      };
      this.$axios.post(this.urlAction, formData, res => {
        this.uploadVisible = false;
        if (res.isSuccess) {
          const { data } = res;
          this.$message.success('操作成功');
          this.$emit('success', data);
          this.handleClose();
        } else {
          this.$message.warning(res.message);
        }
      }, null, config
      );
    },
    handleClick(row) {
      this.fileList = this.fileList.filter(e => { return e.uid !== row.uid; });
    },
    clear() {
      this.progress = 0;
      this.fileList = [];
    }
  }
};

</script>

<style lang="scss">
.upload-container {
  .upload-demo {
    .el-upload__tip {
      text-align: left;
      margin-bottom: 20px;
    }
  }
}
</style>
