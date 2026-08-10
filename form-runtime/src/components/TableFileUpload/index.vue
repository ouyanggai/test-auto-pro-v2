
<!--
 * @Descripttion: 暂时用于表格内文件上传
 * @Author: zhengzetao
 * @Date: 2022-08-27
-->

<template>
  <!-- <div style="display:flex;"> -->
  <div>
    <el-upload action="" :data="fileData" class="upload" :http-request="(response) => httpRequest(response, row)"
      :on-change="handleChange" :limit=uploadLimit
      :disabled="row.uploadFileList && row.uploadFileList.length >= uploadLimit || disabled" :multiple="multiple">
      <el-button type="primary" :disabled="row.uploadFileList && row.uploadFileList.length >= uploadLimit || disabled">
        上传附件<i class="el-icon-upload el-icon--right"></i>
      </el-button>
    </el-upload>

    <ul class="flex-box attach-ul">
      <li v-for="(val, index) in row.uploadFileList" class="attach-li" :key="index">
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
            <span>{{ val["name"] | shortName }}</span>
            <!-- <span v-if="val['size']">({{ val["size"] | fileSize }})</span> -->
            <!-- <span v-else></span> -->
          </el-link>
          <!-- <i v-if="!disabled" class="el-icon-close" @click.stop="handleRemove(val['id'], row)"></i> -->
          <i class="el-icon-view" @click="viewFile(val.absolutelyFileUrl)" style="color:#409eff;cursor: pointer;margin-left:5px;"></i>
          <i class="el-icon-download" @click="downLoad(val.absolutelyFileUrl,val['name'])" style="color:#409eff;cursor: pointer;margin-left:5px;"></i>
          <i v-if="!disabled" class="el-icon-close" @click.stop="handleRemove(val['id'], row)"></i>
        </div>
      </li>
    </ul>
  </div>
</template>

<script>
const icon_arr = ['png', 'jpg', 'jpeg', 'gif'];
import { deepClone,viewFile } from '@/utils';
export default {
  name: '',
  components: {},
  data() {
    return {
      action: 'web/file/api/file/uploadFile',
      fileData: {
        fileType: 'ordinaryFile'
      },
      setInt: ''
    };
  },
  props: {
    row: {
      type: Object,
      default() {
        return {};
      }
    },
    uploadLimit: {
      type: Number,
      default: 1
    },
    type: {
      type: String,
      default: 'add'
    },
    disabled: {
      type: Boolean,
      default: false
    },
    multiple: {
      type: Boolean,
      default: false
    }
  },
  computed: {},
  watch: {},
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
    },
    shortName(name) {
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
    }
  },
  created() { },
  mounted() { },
  methods: {
    viewFile(url) {
      viewFile(url)
    },
    httpRequest(response, row) {
      console.log(response);
      console.log(row);
      // return;
      const { file } = response;
      const uid = file.uid;
      const formData = new FormData();
      formData.append('file', file);
      formData.append('fname', file.name);
      formData.append('fileType', 'ordinaryFile');
      this.$axios.post(
        this.action,
        formData,
        res => {
          if (res.isSuccess) {
            const fileData = res.data;
            console.log('fileData', fileData);
            // return;
            const index = row.uploadFileList.findIndex(
              item => item.name == fileData.originFileName
            );
            if (index !== -1) {
              // list有
              row.uploadFileList[index].name = fileData.originFileName;
              row.uploadFileList[index].status = 'uploaded';
              row.uploadFileList[index].id = fileData.id;
              row.uploadFileList[index].absolutelyFileUrl =
                fileData.absolutelyFileUrl;
              row.uploadFileList[index].percentage = 100;
            } else {
              const obj = {
                name: fileData.fileName,
                status: 'uploaded',
                id: fileData.id,
                absolutelyFileUrl: fileData.absolutelyFileUrl,
                percentage: 100
              };
              row.uploadFileList.push(obj);
            }
          } else {
            const errorIndex = row.uploadFileList.findIndex(item => item.uid == uid);
            if (errorIndex !== -1) {
              row.uploadFileList.splice(errorIndex, 1);
            }
            this.$message.error('上传失败');
          }
        },
        true
      );
    },
    handleRemove(k, row) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          const index = row.uploadFileList.findIndex(item => item.id == k);
          if (index >= 0) {
            row.uploadFileList.splice(index, 1);
            // 删除接口等后端提供新的
            // this.$axios.post(
            //   Api.schedule.deleteAttachment,
            //   {
            //     data: {},
            //     ids: [k]
            //   },
            //   (res) => {
            //     if (res.isSuccess) {
            //       row.uploadFileList.splice(index, 1);
            //     } else {
            //       this.$message.error(res.message);
            //     }
            //   }
            // );
          }
        })
        .catch(() => { });
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
      this.row.uploadFileList.push(fileObj);
      if (this.setInt) clearInterval(this.setInt);
      this.setInt = setInterval(() => {
        this.changeProgress();
      }, 1000 / 16);
    },
    changeProgress() {
      // 伪装的上传进度
      let uploaded = 0;
      for (let i = 0; this.row.uploadFileList[i]; i++) {
        if (this.row.uploadFileList[i].status == 'uploading' && this.row.uploadFileList[i].percentage < 100) {
          const addPercent = this.getRandom(1, 8);
          let percent = this.row.uploadFileList[i].percentage;
          percent = percent + addPercent;
          percent = percent >= 95 ? 95 : percent;
          this.row.uploadFileList[i].percentage = percent;
          uploaded++;
        }
      }
      if (uploaded == 0) if (this.setInt) clearInterval(this.setInt);
    },
    getRandom(min, max) {
      return Math.floor(Math.random() * (max - min + 1)) + min;
    }
  }
};

</script>
<style lang='scss' scoped>
.upload {
  display: flex;
  align-items: center;
}

::v-deep .el-upload-list {
  display: none;
}

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
        margin-left:5px;
        line-height:normal;
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
</style>
