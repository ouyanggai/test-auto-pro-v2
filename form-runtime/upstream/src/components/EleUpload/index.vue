<template>
  <div style="display:flex;">
    <el-upload ref="upload" action="" :http-request="httpRequest" :on-change="handleChange" class="upload"
      :disabled="this.fileList.length >= uploadLimit" :limit=uploadLimit v-if="showOnly === false" :multiple="multiple"
      :accept="accept"
      >
      <el-button type="primary" :disabled="this.fileList.length >= uploadLimit">{{ title }}<i
          class="el-icon-upload el-icon--right"></i>
      </el-button>
    </el-upload>
    <ul class="flex-box attach-ul">
      <li v-for="(val, index) in fileList" class="attach-li" :key="index">
        <div class="attach-div">
          <transition name="el-fade-in-linear">
            <div class="progress-div" v-show="val['status'] == 'uploading'">
              <el-progress style="margin-top:3px;" :percentage="val['percentage']" :text-inside="true"
                :stroke-width="17"></el-progress>
            </div>
          </transition>
          <!-- <el-link @click="downLoad(val['absolutelyFileUrl'], val['name'])"> -->
          <el-link @click="viewFile(val.absolutelyFileUrl)">
            <i :class="val['name'] | fileIcon"></i>
            <span>{{ shortName(val["name"]) }}</span>
            <!-- <span v-if="val['size']">({{ val["size"] | fileSize }})</span>
            <span v-else></span> -->
          </el-link>
          <i class="el-icon-view" @click="viewFile(val.absolutelyFileUrl)" style="color:#409eff;cursor: pointer;margin-left:5px;"></i>
          <i class="el-icon-download" @click="downLoad(val.absolutelyFileUrl,val['name'])" style="color:#409eff;cursor: pointer;margin-left:5px;"></i>
          <i class="el-icon-close" @click.stop="handleRemove(val['id'])" v-if="showOnly === false"></i>
        </div>
      </li>
    </ul>
  </div>
</template>

<script>
import { deepClone,viewFile } from '@/utils';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
const icon_arr = ['png', 'jpg', 'jpeg', 'gif'];
export default {
  name: 'EleUpload',
  data() {
    return {
      action: 'web/file/api/file/uploadFile',
      fileList: [],
      setInt: ''
    };
  },
  props: {
    title: {
      type: String,
      default: '上传附件'
    },
    uploadLimit: {
      type: Number,
      default: 10000
    },
    showOnly: {
      type: Boolean,
      default: false
    },
    showFullName: {
      type: Boolean,
      default: false
    },
    attachFile: {
      type: Array,
      default() {
        return [];
      }
    },
    size: {
      type: String | Number,
      default: ''
    },
    multiple: {
      type: Boolean,
      default: true
    },
    fileType: {
      type: String,
      default: 'ordinaryFile'
    },
    accept:{
      type:String,
      default:''
    }
  },
  watch: {
    attachFile: {
      handler(list) {
        if (list) {
          const attachList = deepClone(this.attachFile);
          this.fileList = attachList.map(item => {
            return {
              name: item.originFileName || item.fileName,
              status: 'uploaded',
              id: item.id,
              absolutelyFileUrl: item.fileUrl,
              fileUrl: item.fileUrl,
              fileType: item.fileType,
              fileId:item.fileId,
              percentage: 100,
            };
          });
        }
      },
      immediate: true,
      deep: true
    }
  },
  filters: {
    fileSize(size) {
      if (size) {
        return Math.ceil(size / 1024) + 'k';
      } else {
        return '';
      }
    },
    fileIcon(name) {
      const lastIndex = name.lastIndexOf('.');
      const extendName = name.substr(lastIndex + 1);
      let icon = 'el-icon-s-order';
      if (icon_arr.indexOf(extendName) !== -1) icon = 'el-icon-picture';
      return icon;
    }
  },
  methods: {
    shortName(name) {
      if (this.showFullName) return name;
      const lastIndex = name.lastIndexOf('.');
      const extendName = name.substr(lastIndex + 1);
      const nameWithoutExd = name.substring(0, lastIndex);
      let shortName = nameWithoutExd + '.' + extendName;
      if (nameWithoutExd.length >= 25) {
        const first = nameWithoutExd.substr(0, 12);
        const last = nameWithoutExd.substr(-12);
        shortName = first + '...' + last + '.' + extendName;
      }
      return shortName;
    },
    viewFile(url) {
      viewFile(url)
    },
    httpRequest(param) {
      const { file } = param;
      if (this.size && !isNaN(Number(this.size))) {
        const size = file.size;
        if (size > 1024 * 1024 * this.size) {
          this.fileList.splice(this.fileList.length - 1, 1);
          this.$message.error(`上传文件大小不能超过 ${this.size}MB`);
          return false;
        }
      }
      const uid = file.uid;
      const formData = new FormData();
      var nameArr = file.name.split('.');
      var extendName = nameArr[nameArr.length - 1];
      nameArr.splice(nameArr.length - 1, 1);
      var fname = nameArr.join('.') + new Date().getTime().toString().substr(0, 10) + '.' + extendName;
      formData.append('file', file);
      formData.append('fname', fname);
      formData.append('fileType', this.fileType);
      this.$axios.post(
        this.action,
        formData,
        res => {
          if (res.isSuccess) {
            const fileData = res.data;
            const index = this.fileList.findIndex(
              item => item.name == fileData.originFileName
            );
            if (index > -1) {
              // list有
              this.fileList[index].id = fileData.id;
              this.fileList[index].status = 'uploaded';
              this.fileList[index].name = fileData.originFileName;
              this.fileList[index].fileUrl = fileData.fileUrl;
              this.fileList[index].fileType = fileData.fileType;
              this.fileList[index].percentage = 100;
              this.fileList[index].absolutelyFileUrl = fileData.absolutelyFileUrl;
              this.fileList[index].fileId = fileData.fileId
            } else {
              const obj = {
                name: fileData.originFileName || fileData.fileName,
                status: 'uploaded',
                id: fileData.id,
                absolutelyFileUrl: fileData.absolutelyFileUrl,
                fileUrl: fileData.fileUrl,
                fileType: fileData.fileType,
                percentage: 100
              };
              this.fileList.push(obj);
            }
          } else {
            const errorIndex = this.fileList.findIndex(item => item.uid == uid);
            if (errorIndex !== -1) {
              this.fileList.splice(errorIndex, 1);
            }
            this.$message.error('上传失败');
            this.$refs.upload.clearFiles();
          }
        },
        true
      );
    },
    handleRemove(k) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          const index = this.fileList.findIndex(item => item.id == k);
          if (index >= 0) {
            var targetFile = this.fileList[index];
            this.deleteFile(targetFile, index);
            // if (!this.fileList.length) {
            //   this.$refs.upload.clearFiles();
            // }
          }
          this.$refs.upload.clearFiles();
        })
        .catch(() => { });
    },
    deleteFile(file, index) {
      console.log('file',file)
      var data = {
        id: file.fileId || file.id,
        fileType: 'ordinaryFile',
        fileUrl: file.fileUrl
      };
      this.$axios.post(
        Api.filesManage.deleteFile,
        {
          data
        },
        res => {
          if (res.isSuccess) {
            this.fileList.splice(index, 1);
            this.$message.success('操作成功');
            this.$emit('deleteComplete');
            this.$emit('change',this.fileList)
          } else {
            // this.$message.success('操作失败')
          }
        });
    },
    downLoad(href, name) {
      const lastIndex = name.lastIndexOf('.');
      const extendName = name.substr(lastIndex + 1);
      const ele = document.createElement('a');
      ele.target = '_blank';
      // if (extendName == 'pdf' || icon_arr.indexOf(extendName) !== -1) {
      //   // 预览
      //   ele.href = href;
      //   ele.style.display = 'none';
      //   document.body.appendChild(ele);
      //   ele.click();
      //   document.body.removeChild(ele);
      // } else {
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
      // }
    },
    handleChange(fileObj) {
      fileObj.percentage = 0;
      fileObj.status = 'uploading';
      this.fileList.push(fileObj);
      if (this.setInt) clearInterval(this.setInt);
      this.setInt = setInterval(() => {
        this.changeProgress();
      }, 1000 / 16);
      this.$emit('change',this.fileList)
    },
    changeProgress() {
      // 伪装的上传进度
      let uploaded = 0;
      // console.log('this.fileList', this.fileList)
      for (let i = 0; this.fileList[i]; i++) {
        if (this.fileList[i].status == 'uploading' && this.fileList[i].percentage < 100) {
          const addPercent = this.getRandom(1, 8);
          let percent = this.fileList[i].percentage;
          percent = percent + addPercent;
          percent = percent >= 95 ? 95 : percent;
          this.fileList[i].percentage = percent;
          uploaded++;
        }
      }
      if (uploaded == 0) if (this.setInt) clearInterval(this.setInt);
    },
    getRandom(min, max) {
      return Math.floor(Math.random() * (max - min + 1)) + min;
    },
    getFileId() {
      const arr = this.fileList.map(item => {
        return item.fileId || item.id;
      });
      return arr;
    },
    showCloseIcon(index) {
      this.$set(this.fileList[index], 'isShowUploadIcon', true);
    },
    hideCloseIcon(index) {
      this.fileList[index].isShowUploadIcon = false;
    }
  }
};
</script>

<style lang="scss" scoped>

.attach-ul {
  margin-left: 5px;
  flex-wrap: wrap;

  .attach-li {
    margin-right: 5px;
    margin-top: 5px;
    background: rgb(230, 238, 247);

    .attach-div {
      position: relative;
      padding: 1px 5px;
      background: transparent;
      display: flex;
      align-items: center;

      i {
        color: rgb(55, 126, 240);
      }

      i.el-icon-close:hover {
        background: #377ef0;
        color: #fff;
      }
    }
  }
}

.progress-div {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  background: transparent;
  z-index: 2;
}

.upload-close {
  position: absolute;
  top: 50%;
  right: 10px;
  transform: translateY(-50%);
  z-index: 3;
}

.title {
  width: 95px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.upload {
  display: flex;
  align-items: center;
}
::v-deep .el-upload-list.el-upload-list--text{
  display: none !important;
}
</style>
