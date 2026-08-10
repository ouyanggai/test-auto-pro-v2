<!--
 * @Descripttion:自定义上传组件
 * @Author: Calvin
 * @Date: 2021-07-06 10:31:06
-->

<template>
  <div>
    <div class="upload">
      <div
        class="upload-btn flex-box"
        v-if="showBtn"
      >
        <el-button
          round
          @click="handleClick"
          class="btn"
          :disabled='disabled'
          v-permission="permission.upload"
        >上传文件</el-button>
        <div class="note gray">{{note}}</div>
        <input
          type="file"
          ref="input"
          class="upload-input"
          :name="name"
          @change="handleChange"
          :multiple="multiple"
        >
      </div>
      <template v-if="uploadedList.length + uploadList.length > 0">
        <el-row>
          <el-col
            :span="4"
            class="upload-til"
          >
            <p>文件：（{{uploadedList.length}}个）</p>
          </el-col>
          <el-col :span="20">
            <ul class="upload-list clear">
              <li
                class="clear"
                v-for="(file) in uploadedList"
                :key="file.id"
              >
                <div class="file-left">
                  <img :src="getFileTypeIcon(file.type, file.name|| file.fileName)">
                </div>
                <div class="file-right">
                  <div class="file-info">
                    <p class="til">{{file.name || file.fileName}} ({{file.size || file.fileSize}}KB)</p>
                    <div class="btn-list clear">
                      <div
                        @click="handleRemove(file)"
                        v-permission="permission.delete"
                        v-if="showDelete"
                        class="del btn"
                      >删除</div>
                    </div>
                  </div>
                </div>
              </li>
              <li
                v-for="(file, index) in uploadList"
                :key="index"
              >
                <div class="file-left opacity50">
                  <img :src="getFileTypeIcon(file.type)">
                </div>
                <div class="file-right">
                  <div class="file-info">
                    <p class="til gray">{{file.name}} ({{file.size}}KB)</p>
                    <template v-if="file.status">
                      <div class="progress btn-list clear gray">
                        <div class="progressbar">
                          <el-progress
                            :percentage="file.uploadPercentage"
                            :show-text="false"
                          ></el-progress>
                        </div>
                        <div>{{file.speed}}KB/s</div>
                        <div>{{file.excessTime}}</div>
                        <div>
                          <div
                            @click="onAbort(file)"
                            class="del btn"
                          >删除</div>
                        </div>
                      </div>
                    </template>
                    <template v-else>
                      <div class="btn-list">
                        <div>
                          <span class="red">上传失败，请重新上传！</span>
                        </div>
                        <div
                          @click="onAbort(file)"
                          class="abo btn"
                        >放弃</div>
                        <div
                          @click="onReUpload(file)"
                          class="re btn"
                        >重新上传</div>
                      </div>
                    </template>
                  </div>
                </div>
              </li>
            </ul>
          </el-col>
        </el-row>
      </template>
    </div>
  </div>
</template>

<script>
import fetch from './fetch';
import { transferTime } from '@/utils';
import axios from 'axios';
const CancelToken = axios.CancelToken;
function noop() { }
const cancelFun = {};

export default {
  name: 'Upload',
  props: {
    permission: {
      type: Object,
      default() {
        return {
          delete: '',
          upload: '',
          preview: '',
          download: ''
        };
      }
    },
    /**
     * 上传文件地址
     */
    action: {
      type: String,
      required: true
    },
    /**
     * 禁用上传按钮
     */
    disabled: {
      type: Boolean,
      default: false
    },
    /**
     * 上传文件大小限制
     */
    maxSize: {
      type: Number,
      default: 400
    },
    name: {
      type: String,
      default: 'file'
    },
    /**
     * 提示信息
     */
    note: {
      type: String,
      default: '只能上传PDF、Word、Excel、图片格式（jpg、png、jpeg），且不可超过400MB'
    },
    /**
     * 上传文件个数限制
     */
    limit: {
      type: Number,
      default: 1
    },
    /**
     * 是否展示上传文件按钮
     */
    showBtn: {
      type: Boolean,
      default: true
    },
    /**
      * 删除按钮控制
      */
    showDelete: {
      type: Boolean,
      default: true
    },
    /**
     * 上传文件之前的钩子
     */
    beforeUpload: Function,
    /**
     * 自定义请求头
     */
    headers: {
      type: Object,
      default() {
        return {};
      }
    },
    /**
     * 文件列表
     */
    fileList: {
      type: Array,
      default() {
        return [];
      }
    },
    /**
     * @name: 限制上传文件类型
     * @msg: :fileType="['image', 'word']"
     * @param {Array}
     */
    fileType: {
      type: Array,
      default() {
        return ['image', 'word', 'excel', 'pdf'];
      }
    },
    /**
     * @name: 是否支持多文件上传
     * @msg:  :multiple=true
     * @param {Boolean}
     */
    multiple: {
      type: Boolean,
      default: false
    },
    /**
     * 上传成功回调
     */
    onSuccess: {
      type: Function,
      default: noop
    },
    /**
     * 上传进度回调
     */
    onProgress: {
      type: Function,
      default: noop
    },
    /**
     * 上传失败回调
     */
    onError: {
      type: Function,
      default: noop
    },
    /**
     * 超过文件数量回调
     */
    onExceed: {
      type: Function,
      default: noop
    },
    /**
     * 删除文件回调
     */
    onRemove: {
      type: Function,
      default: noop
    }
  },

  data() {
    return {
      reqs: {},
      uploadedList: this.fileList,
      uploadList: [
        /*          {
           id: 34324324,
           name: '我是图片的名字名字名字.jpe',
           uploadPercentage: 0,
           size: 2344,
           speed: 3213,
           loaded: 444,
           status: true,
           excessTime: '00:00:00',
           type: 'image/jpeg'
           }*/
      ],
      removeFile: {}
    };
  },
  watch: {
    fileList(list) {
      this.uploadedList = list;
    }
  },
  methods: {
    getFileTypeIcon(type, name) {
      const fileTypeIcon = {
        word: require('@/assets/images/icons_doctype_word.svg'),
        image: require('@/assets/images/icons_doctype_image.svg'),
        pdf: require('@/assets/images/icons_doctype_pdf.svg'),
        excel: require('@/assets/images/icons_doctype_excel.svg'),
        other: require('@/assets/images/icons_doctype_attachment.svg')
      };
      if (!fileTypeIcon[type]) return fileTypeIcon[this.getFileType(name)];
      return fileTypeIcon[type];
    },
    getFileType(file) {
      const suf = file.slice(file.lastIndexOf('.')).toLowerCase();
      if (/\.(doc|docx)$/.test(suf)) {
        return 'word';
      } else if (/\.(xls|xlsx)$/.test(suf)) {
        return 'excel';
      } else if (/\.(pdf)$/.test(suf)) {
        return 'pdf';
      } else if (/\.(png|jpg|jpeg)$/.test(suf)) {
        return 'image';
      } else {
        return 'other';
      }
    },
    handleClick() {
      this.$refs.input.value = null;
      this.$refs.input.click();
    },
    handleChange(ev) {
      const files = ev.target.files;
      if (!files) return;
      this.uploadFiles(files);
    },
    uploadFiles(files) {
      if (this.limit && this.uploadedList.length + files.length > this.limit) {
        this.handleExceed && this.handleExceed(files, this.uploadedList);
        return;
      }
      let postFiles = Array.prototype.slice.call(files);
      if (!this.multiple) {
        postFiles = postFiles.slice(0, 1);
      }
      if (postFiles.length === 0) {
        return;
      }
      postFiles.forEach((rawFile, i) => {
        // 开始遍历上传
        // 设置一个uid
        rawFile.id = Date.now() + i;
        rawFile.currentTime = Date.now();
        const fileSize = Math.round(rawFile.size / 1024);
        if (fileSize / 1024 > this.maxSize) {
          this.$message({
            type: 'error',
            message: `文件大小不能超过${this.maxSize}M`
          });
          return false;
        }
        const fileType = this.getFileType(rawFile.name);
        if (this.fileType && this.fileType.indexOf(fileType) === -1) {
          this.$message({
            type: 'error',
            message: `文件格式不对`
          });
          return false;
        }
        rawFile.fileType = fileType;
        rawFile.fileSize = fileSize;
        this.addUploadList(rawFile);
        this.upload(rawFile);
      });
    },
    addUploadList(rawFile) {
      this.uploadList.push({
        id: rawFile.id,
        name: rawFile.name,
        type: rawFile.fileType,
        uploadPercentage: 0,
        size: rawFile.fileSize,
        speed: 0,
        loaded: 0,
        status: true,
        excessTime: '99:59:59',
        file: rawFile
      });
      console.log('uploadList', this.uploadList);
    },
    upload(rawFile) {
      this.$refs.input.value = null;
      if (!this.beforeUpload) {
        return this.post(rawFile);
      }
      const before = this.beforeUpload(rawFile);
      if (before !== false) {
        this.post(rawFile);
      }
    },
    post(rawFile) {
      const { id } = rawFile;
      const options = {
        headers: this.headers,
        file: rawFile,
        data: {
          ...this.data
        },
        filename: this.name,
        action: this.action,
        onProgress: e => {
          this.handleProgress(e, rawFile);
        },
        onSuccess: res => {
          // 判断是否上传成功
          if (res.isSuccess) {
            this.handleSuccess(res, rawFile);
            delete this.reqs[id];
          } else {
            this.handleError(res, rawFile);
          }
        },
        onError: err => {
          this.handleError(err, rawFile);
        }
      };
      const req = this.httpRequest(options);
      this.reqs[id] = req;
      if (req && req.then) {
        req.then(options.onSuccess).catch(options.onError);
      }
    },
    httpRequest(option) {
      const formData = new FormData();
      if (option.data) {
        Object.keys(option.data).forEach(key => {
          formData.append(key, option.data[key]);
        });
      }
      formData.append(option.filename, option.file, option.file.name);
      formData.append('platformCode', '200001');
      return fetch({
        method: 'post',
        url: option.action,
        headers: option.headers,
        data: formData,
        timeout: 0,
        cancelToken: new CancelToken(function executor(c) {
          // executor 函数接收一个 cancel 函数作为参数
          cancelFun[option.file.id] = c;
        }),
        onUploadProgress: option.onProgress
      });
    },
    handleProgress(e, rawFile) {
      this.onProgress(e, rawFile);
      this.uploadList.forEach((item, i) => {
        if (item.id === rawFile.id) {
          const uploadPercentage = Math.round(e.loaded / e.total * 100);
          const s = (e.loaded - item.loaded) / 1024 / ((Date.now() - rawFile.currentTime) / 1000);

          item.loaded = e.loaded;
          rawFile.currentTime = Date.now();
          item.speed = Math.round(s);
          item.uploadPercentage = uploadPercentage;
          const excessTime = item.size * (100 - uploadPercentage) / item.speed / 100;
          item.excessTime = transferTime(excessTime);
          if (uploadPercentage === 100) {
            item.speed = 0;
            item.excessTime = '00:00:00';
          }
        }
      });
    },
    // 上传完成后在上传列表中删除
    handleSuccess(res, rawFile) {
      this.uploadList.forEach((item, i, arr) => {
        if (rawFile.id === item.id) {
          arr.splice(i, 1);
        }
      });

      // 添加到上传成功列表
      // this.uploadedList.push(res.data);
      this.onSuccess(res, rawFile, this.uploadedList);
    },
    handleError(err, rawFile) {
      this.onError(err, rawFile);
      this.uploadList.forEach((item, i) => {
        if (rawFile.id === item.id) {
          item.status = false;
        }
      });
    },
    handleExceed(files, fileList) {
      this.onExceed(files, fileList);
      this.$alert(
        `当前限制选择 ${this.limit} 个文件，本次选择了 ${files.length} 个文件，共选择了 ${files.length +
        fileList.length} 个文件`
      );
    },
    onReUpload(rawFile) {
      rawFile.status = true;
      rawFile.uploadPercentage = 0;
      rawFile.speed = 0;
      rawFile.loaded = 0;
      rawFile.excessTime = '99:59:59';
      this.upload(rawFile.file);
    },
    onAbort(rawFile) {
      this.uploadList.forEach((item, i, arr) => {
        if (rawFile.id === item.id) {
          arr.splice(i, 1);
          cancelFun[item.id]('已取消该文件的上传');
        }
      });
    },
    deleteSubmit() {
      this.onRemove(this.removeFile);
    },
    handleRemove(file) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.removeFile = file;
        this.deleteSubmit();
      }).catch(() => {
      });
    }
  },
  components: {
  }
};
</script>

<style lang="scss" scoped>
$link-color: #0763cd;
.gray {
  color: #a8aab7;
}
.red {
  color: #ff0009;
}
.opacity50 {
  opacity: 0.4;
}
.clear:before,
.clear:after {
  content: " ";
  display: table;
  clear: both;
}
.clear {
  *zoom: 1;
}
.upload {
  font-size: 12px;
  .upload-til {
    max-width: 100px;
    p {
      padding-left: 20px;
      line-height: 18px;
      background: url("~@/assets/images/icons_attach.svg") no-repeat left center;
    }
  }
}
.upload-input {
  display: none;
}
.upload-btn {
  margin-bottom: 10px;
  button.is-round {
    padding: 7px 16px;
    color: $link-color;
    float: left;
    span {
      font-size: 12px !important;
    }
  }
  .note {
    float: left;
    line-height: 30px;
    padding-left: 10px;
  }
}
.upload-list {
  margin-bottom: -15px;
  li {
    width: 50%;
    float: left;
    padding-left: 15px;
    box-sizing: border-box;
    margin-bottom: 20px;
    .file-left {
      float: left;
      width: 44px;
      margin-right: 18px;
      img {
        width: 44px;
      }
    }
    .file-right {
      float: left;
    }
  }
}
.file-info {
  .til {
    padding-bottom: 10px;
    line-height: 20px;
  }
  .progress {
    .progressbar {
      width: 130px;
      padding-top: 9px;
      padding-right: 10px;
    }
  }
  .btn-list {
    > div {
      height: 25px;
      line-height: 25px;
      cursor: pointer;
      float: left;
      padding-right: 20px;
      box-sizing: border-box;
    }
    .btn {
      padding-left: 20px;
      color: $link-color;
    }
    .del {
      background: url("~@/assets/images/icons_delete.svg") no-repeat left 2px;
    }
    .abo {
      background: url("~@/assets/images/icons_remove_link.svg") no-repeat left
        center;
    }
    .re {
      background: url("~@/assets/images/icons_offline_cached_re.svg") no-repeat
        left center;
    }
  }
}
</style>
