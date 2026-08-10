<!--
 * @Author: junshao
 * @Date: 2023-03-08 10:32:32
 * @LastEditors: Please set LastEditors
 * @LastEditTime: 2025-07-31 11:00:17
 * @Description: 发起人附言
-->
<template>
  <div class="postscript-divWrap">
    <div class="flow-wrap">
      <div class="postscript-container">
        <div class="header">
          <span class="header-title">发起人附言</span>
            <!-- <el-switch
            style="display: inline-block;margin-left: 20px;"
              v-model="isPostMessage"
              active-text="消息推送"
              inactive-text="">
            </el-switch> -->
          <el-button v-if="isInitiator && !handleReadStatus" type="text" @click="clickAddPostscript">新增附言</el-button>
          <el-upload v-if="isAddPostscript" class="myLanguage" style="position: relative;" action="#" multiple :limit="10" :on-exceed="onExceed"
            :http-request="param => uploadSectionFile(param)" :on-remove="removeFile" :on-change="fileChange"
            :file-list="attachmentFileList" :show-file-list="true" list-type="text" :on-preview="handlePreview">
            <el-button icon="el-icon-link" round >附件</el-button>
          </el-upload>
        </div>
        <div class="postscript-content" v-if="isAddPostscript">
          <el-input type="textarea" maxlength="500" show-word-limit v-model.trim="postscriptMessage"></el-input>
          <div class="postscript-btns">
            <div>
              <el-button type="primary" round @click="handleSubmit">提交</el-button>
              <el-button round @click="cancelPostscript">取消</el-button>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="flow-wrap" v-loading="loading">
      <div  v-if="postscriptList && postscriptList.length > 0" class="postscript-list">
        <p>附言记录</p>
        <div v-for="( item, index) in postscriptList" :key="item.id" class="script-item">
          <div>
            <p class="item-info">
              <span>{{ item.replyName || item.sendName }}</span>
              <span class="item-info-date">{{ item.createDate }}</span>
              <el-button type="text" class="item-info-reply" @click="handleReply(index)">回复</el-button>
            </p>
            <p style="text-indent: 1rem;">{{ item.text }}</p>
            <p v-if="item.relationFileDataVos && item.relationFileDataVos.length>0" style="text-indent: 1rem;">
              <span>附件：</span>
              <span v-for="file in item.relationFileDataVos" :key="file.id" class="file-item">
                {{ file.originFileName }}
                <i class="el-icon-view fileClick-icon" @click.stop="viewFile(file)"></i>
                <i class="el-icon-download fileClick-icon" @click.stop="downloadFileItem(file)"></i>
              </span>
            </p>
            <div v-if="item.children.length" class="script-item-child">
              <div v-for="( childItem, childIndex) in item.children" :key="childItem.id">
                <p class="item-info-child">
                  <span>{{ childItem.replyName || childItem.sendName }}</span>
                  <span class="item-info-date">{{ childItem.createDate }}</span>
                </p>
                <p style="text-indent: 1rem;">{{ childItem.text }}</p>
              </div>
            </div>
          </div>
          <div v-if="isShowReplyInput && activeReplyIndex == index">
            <el-input type="textarea" v-model.trim="replyMessage"
            maxlength="500" show-word-limit :autosize="{ minRows: 1}"
            :placeholder="'回复'+ (item.replyName || item.sendName)" style="width: 85%;margin-right: 10px;"></el-input>
            <el-button type="text" @click="handleReplySubmit(item.id)">提交</el-button>
            <el-button type="text" @click="handleCancelReply">取消</el-button>
          </div>
        </div>
      </div>
      <!-- <div v-else class="postscript-list">暂无附言</div> -->
    </div>

    <!-- onlyOffice插件预览 -->
    <ViewWordFile :excelVisible.sync="excelVisible" :wordFileUrl="wordFileUrl" v-if="excelVisible"></ViewWordFile>
  </div>
</template>

<script>
import { viewFile,arrayToTree } from '@/utils';
import Api from '@/api';
import ViewWordFile from './ViewWordFile.vue';
export default {
  name: '',
  components: {ViewWordFile},
  data() {
    return {
      loading:false,
      isAddPostscript: false,
      isPostMessage: false,
      postscriptMessage: '',
      replyMessage: '',
      isInitiator: false,
      isShowReplyInput: false,
      activeReplyIndex: 0,
      newPostscriptId: '',
      postscriptList: [],
      attachmentFileList: [],
      wordFileUrl:'',
      excelVisible:false
    };
  },
  computed: {},
  props: {
    flowInstanceId: {
      type: String,
      default: ''
    },
    handleReadStatus: {
      type: Boolean,
      default: false
    },
  },
  watch: {
  },
  created() { },
  mounted() {
    this.isInitiator = this.$attrs.isInitiator;
    this.getPostScriptList();
  },
  methods: {
    viewFile(row) {
      console.log('viewFile',row)
      if (!row.fileUrl) return;
      
      var fileSuffix = row.fileUrl.substring(row.fileUrl.lastIndexOf(".") + 1);
      console.log('fileSuffix',fileSuffix)
      // 因为后端文件预览插件的bug，有的word文档会出现格式错乱问题，所以用onlyoffice预览word文件
      if (fileSuffix == 'docx' || fileSuffix == 'doc') {
        this.excelVisible = true;
        this.wordFileUrl = row.fileUrl;
        // this.openView(sid,url);
      } else {
        viewFile(row.fileUrl)
      }
    },
    handlePreview(target){
      if(target){
        let file = target.raw
        let fileBlob = new Blob([file],{type:`${file.type};charset=utf-8`});
        const url = URL.createObjectURL(fileBlob)
        const ele = document.createElement('a');
        ele.target = '_blank';
        ele.href = url;
        ele.style.display = 'none';
        document.body.appendChild(ele);
        ele.click();
        document.body.removeChild(ele);
      }
    },
    /* 随机id生成 */
    uuidGenerator() {
      const originStr = 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx';
      const originChar = '0123456789abcdef';
      const len = originChar.length;
      return originStr.replace(/x/g, function (match) {
        return originChar.charAt(Math.floor(Math.random() * len));
      });
    },
    clickAddPostscript() {
      this.isAddPostscript = true;
    },
    cancelPostscript() {
      this.isAddPostscript = false;
    },
    uploadFile(param) {
      // 上传文件
      const formData = new FormData();
      this.attachmentFileList.map(item => {
        formData.append('file', item.raw);
      });
      formData.append('fileType', 'ordinaryFile');
      this.$axios.post(
        Api.user.uploadFileList,
        formData,
        (res) => {
          if (res.isSuccess && res.data) {
            const fileIds = [];
            res.data.map(file => {
              fileIds.push(file.id);
            });
            this.relateBusiness(fileIds);
          } else {
            this.$message.error('上传失败');
          }
        }
      );
    },
    uploadSectionFile(param) {
      // console.log(param);
    },
    fileChange(file, fileList) {
      this.attachmentFileList = fileList;
    },
    removeFile(file, fileList) {
      this.attachmentFileList = fileList;
    },
    onExceed() {
      this.$message.warning('上传文件个数超出限制');
    },
    relateBusiness(fileIds) {
      const data = {
        data: {
          relationId: this.newPostscriptId,
          fileIds
        }
      };
      this.$axios.post(
        Api.budgetManage.saveBatchFile,
        data,
        (res) => {
          if (res.isSuccess) {
            this.handleSubmitPostscript(fileIds);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleSubmit() {
      this.newPostscriptId = this.uuidGenerator();
      if (this.attachmentFileList.length > 0) {
        // 有关联文件
        this.uploadFile();
      } else {
        // 未关联文件
        this.handleSubmitPostscript();
      }
    },
    handleSubmitPostscript(fileIds) {
      if (this.postscriptMessage == '') {
        this.$message.warning('附言信息不能为空！');
        return false;
      }
      const data = {
        text: this.postscriptMessage,
        id: this.newPostscriptId,
        flowInstanceId: this.flowInstanceId
      };
      if (fileIds && fileIds.length > 0) {
        const flowInstanceAttachmentList = [];
        fileIds.forEach(item => {
          flowInstanceAttachmentList.push({ attachmentId: item });
        });
        data.flowInstanceAttachmentList = flowInstanceAttachmentList;
      }
      this.$axios.post(
        Api.approveManage.savePostScript,
        { data },
        (res) => {
          if (res.isSuccess) {
            this.$message.success('提交成功');
            this.reset();
            this.getPostScriptList();
          } else {
            this.$message.error(res.message);
            this.reset();
          }
        }
      );
    },
    getPostScriptList() {
      this.$axios.post(
        Api.approveManage.getPostScriptList,
        {
          data: {
            flowInstanceId: this.flowInstanceId
          }
        },
        (res) => {
          if (res.isSuccess) {
            this.postscriptList = this.generateTree(res.data);
            // this.postscriptList = res.data;
            console.log('this.postscriptList',this.postscriptList)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree(flatArray) {
      // 创建一个映射，用于存储每个节点的引用
      const nodeMap = {};
      // 创建一个数组，用于存储树的根节点
      const tree = [];

      // 遍历扁平数组，初始化每个节点
      flatArray.forEach(item => {
        nodeMap[item.id] = { ...item, children: [] };
      });

      // 再次遍历扁平数组，构建树结构
      flatArray.forEach(item => {
        const node = nodeMap[item.id];
        if (item.pid === null) {
          // 如果没有父节点，则为根节点
          tree.push(node);
        } else {
          // 如果有父节点，则将当前节点添加到父节点的子节点数组中
          const parentNode = nodeMap[item.pid];
          if (parentNode) {
            node.isReplay = true;
            parentNode.children.push(node);
          }
        }
      });
      return tree;
    },
    reset() {
      this.postscriptMessage = '';
      this.isAddPostscript = false;
      this.isPostMessage = false;
      this.attachmentFileList = [];
    },
    handleReply(index) {
      this.activeReplyIndex = index;
      this.isShowReplyInput = true;
    },
    handleCancelReply() {
      this.replyMessage = '';
      this.isShowReplyInput = false;
    },
    handleReplySubmit(msgId) {
      if (this.replyMessage == '') {
        this.$message.warning('回复内容不能为空！');
        return false;
      }
      const data = {
        text: this.replyMessage,
        pid: msgId,
        flowInstanceId: this.flowInstanceId
      };
      this.$axios.post(
        Api.approveManage.savePostScript,
        { data },
        (res) => {
          if (res.isSuccess) {
            this.handleCancelReply();
            this.getPostScriptList();
          } else {
            this.$message.error(res.message);
            this.handleCancelReply();
          }
        }
      );
    },
    downloadFileItem(file) {
      if (!!window.ActiveXObject || 'ActiveXObject' in window) { // IE无法识别download属性，用户自行保存
        this.$message.error(
          '当前浏览器不支持点击下载，请手动保存，或者切换到Google Chrome浏览器进行下载'
        );
      } else {
        // this.loading = true
        const x = new XMLHttpRequest();
        x.open('GET', file.fileUrl, true);
        x.responseType = 'blob';
        var _this = this
        x.onload = function () {
          const url = window.URL.createObjectURL(x.response);
          const a = document.createElement('a');
          a.href = url;
          a.download = file.originFileName;
          a.click();
          a.remove();
          _this.loading = false
        };
        x.onerror=()=>{
          _this.loading = false
          // this.loading = false
        };
        x.send();
      }
    }
  }
};

</script>
<style lang='scss' scoped>
.flow-wrap {
  margin: 0 auto;
  width: 950px;
  padding: 10px;

  .postscript-container {
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  }

  .header {
    background-color: #fff7da;
    border-top: 1px solid #daa134;
    padding: 10px 20px;

    .header-title {
      margin-right: 20px;
    }
  }

  .postscript-content {
    padding: 10px 20px;
    position:relative;

    .postscript-btns {
      display: flex;
      justify-content: space-between;
      margin-top: 8px;
    }
  }

  .postscript-list {
    max-height: 160px;
    overflow-y: auto;
    padding: 10px 8px;
    // padding: 10px 20px;
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

    .script-item {
      margin: 2px 0;
      padding: 5px;

      .item-info {
        position: relative;
        color: #8c8c8c;

        .item-info-date {
          margin-left: 15px;
        }

        .item-info-reply {
          display: none;
          position: absolute;
          right: 0px;
          top: -3px;
          // right: 15px;
        }
      }
      .file-item {
        font-size: 12px;
        color: #1989fa;
        margin-right: 12px;
        cursor: default;
        // cursor: pointer;
      }
      .fileClick-icon {
        // margin-right: 7px;
        // color: #909399;
        // height: 100%;
        // line-height: inherit;
        // color:#409eff;
        text-indent: 8px;
        &:hover{
          // color: #409eff;
          cursor: pointer;
        }
      }
    }
    .script-item-child {
      margin: 2px 0;
      padding: 4px;
      margin-left: 8px;
      margin-bottom: 5px;
      background: aliceblue;
      border: 1px solid #ccc;
      .item-info-child {
        position: relative;
        color: #8c8c8c;
        .item-info-date {
          margin-left: 15px;
        }
      }
    }

    .script-item:hover {
      background-color: #f5f5f6;
      border-radius: 8px;

      .item-info-reply {
        display: inline-block;
      }
    }
  }
}
// ::v-deep .el-upload-list{
//   position:absolute;
// }
::v-deep {
  .myLanguage .el-upload-list__item-name {
    color: #606266;
    display: block;
    margin-right: 0px;
    padding-left: 4px;
    transition: color 0.3s;
    white-space: normal;
  }
}
@media screen and (min-width: 1300px) {
  .flow-wrap {
    width: 80%;
  }
}</style>
