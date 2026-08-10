<!--
 * @Descripttion:
 * @version:
 * @Author: Ronin
 * @Date: 2023-05-25 17:01:13
 * @LastEditors: Ronin
<<<<<<< HEAD
<<<<<<< HEAD
 * @LastEditTime: 2023-06-15 09:44:55
=======
 * @LastEditTime: 2023-06-14 16:14:19
>>>>>>> feat-contract
=======
 * @LastEditTime: 2023-06-15 09:44:55
>>>>>>> dev
-->
<template>
  <div>
    <el-dialog title="上传" :visible="uploadDialogVisible" width="50%" :close-on-click-modal="false" center top="100px"
      @close='handleClose'>
      <el-form :model="ruleForm" ref="ruleForm" label-width="100px" class="demo-ruleForm">
        <el-form-item label="文件目录" prop="folderName">
          <selectLazyTree labelName="文件目录" :value.sync="ruleForm.folderId" :options="treeData"
          :loadNode="loadNode" :props="defaultProps" @getValue="getValue"></selectLazyTree>
        </el-form-item>
        <el-form-item label="上传文件" prop="fileUrl">
          <el-upload ref="upload" action="./" :auto-upload="false" :http-request="httpRequest" :on-change="handleChange"  :on-remove="handleRemove" class="upload" :file-list="fileList"
             :limit="uploadLimit"  v-if="showOnly === false" :multiple="true">
            <el-button type="primary" :disabled="this.fileList.length >= uploadLimit">上传附件<i
                class="el-icon-upload el-icon--right"></i>
            </el-button>
          </el-upload>
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="uploadDialogVisible = false">取 消</el-button>
        <el-button type="primary" @click="submitForm" :loading="loading">确 定</el-button>

      </span>
    </el-dialog>
  </div>
</template>
<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import selectLazyTree from '../selectLazyTree';
export default {
  name: 'UploadForLazyTree',
  components: { selectLazyTree },
  props: {
    uploadDialogVisible: {
      type: Boolean,
      default: false
    },
    uploadLimit: {
      type: Number,
      default: 10
   },
    showOnly: {
      type: Boolean,
      default: false
    },
    attachFile: {
      type: Array,
      default() {
        return [];
      }
    },
    businessId: { // 业务id
      type: String,
      default: ''
    }
  },
  data() {
    return {
      ruleForm: {
        folderName: '',
        folderId: '',
        fileUrl: ''
      },
      treeData: [],
      defaultProps: {
        children: 'children',
        label: 'fileName',
        value: 'id'
      },
      action: 'web/file/api/file/uploadFile',
      fileList: [],
      setInt: '',
      successFileNUmber: 0,
      loading:false
    };
  },
  mounted() {
    console.log(this.$route, '++++');
    // const companyId = localstorageGet('currentCompanyId')
    //   ? localstorageGet('currentCompanyId')
    //   : localstorageGet('companyId');
    // this.$axios.post(
    //   Api.ArchiveManage.getFileDataByProject,
    //   {
    //     data: {
    //       companyVo: {
    //         id: companyId
    //       },
    //       project: {
    //         id: this.$store.state.user.projectId
    //       },
    //       parentId: ''
    //     },
    //     sid: localstorageGet('token')
    //   },
    //   res => {
    //     if (res.isSuccess) {
    //       this.treeData = res.data ? res.data : [];
    //     } else {
    //       this.$message.error(res.message);
    //     }
    //   });
  },
  methods: {
    loadNode(node, resolve) {
      console.log(node, resolve);
      const companyId = localstorageGet('currentCompanyId')
        ? localstorageGet('currentCompanyId')
        : localstorageGet('companyId');
      this.$axios.post(
        Api.ArchiveManage.getFileDataByProject,
        {
          data: {
            companyVo: {
              id: companyId
            },
            project: {
              id: this.$route.query.projectId || this.$store.state.user.projectId
            },
            parentId: node.data.id
          },
          sid: localstorageGet('token')
        },
        res => {
          if (res.isSuccess) {
            let data = res.data ? res.data : [];
            if (data.length == 0) {
              node.isLeaf = false;
              node.expanded = false;
            }
            data = data.filter(item => item.fileType == 'folder');
            resolve(data);
          } else {
            resolve([]);
          }
        }
      );
    },
    submitForm() {
      this.loading = true
      // this.
      console.log(this.fileList,'fileList+++++++')
      if(this.fileList.length==0){
        this.loading = false
        this.$message.error('请选择需要上传的文件！')
        return
      }
      const file = this.fileList[0].raw;
      const formData = new FormData();
      var nameArr = file.name.split('.');
      var extendName = nameArr[nameArr.length - 1];
      nameArr.splice(nameArr.length - 1, 1);
      var fname = nameArr.join('.') + new Date().getTime().toString().substr(0, 10) + '.' + extendName;
      formData.append('file', file);
      formData.append('fname', fname);
      if (this.ruleForm.folderId) {
        formData.append('parentId', this.ruleForm.folderId);
      }
      formData.append('fileType', 'ordinaryFile');
      this.$axios.post(
        this.action,
        formData,
        res => {
          if (res.isSuccess) {
            this.relateBusinessFile(res.data.id);
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
            } else {
              const obj = {
                name: fileData.fileName,
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
            this.loading = false
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
    getValue(data) {
      console.log(data, '9998888----');
    },
    handleClose() {
      this.$emit('update:uploadDialogVisible', false);
    },
    httpRequest(param) {
      console.log(this.fileList, '++++');
      // if (!this.ruleForm.folderId) {
      //   // this.$message.error('请选择文件目录后在上传文件');
      //   return;
      // }
      const { file } = param;
      const uid = file.uid;
      const formData = new FormData();
      var nameArr = file.name.split('.');
      var extendName = nameArr[nameArr.length - 1];
      nameArr.splice(nameArr.length - 1, 1);
      var fname = nameArr.join('.') + new Date().getTime().toString().substr(0, 10) + '.' + extendName;
      formData.append('file', file);
      formData.append('fname', fname);
      if (this.ruleForm.folderId) {
        formData.append('parentId', this.ruleForm.folderId);
      }
      formData.append('fileType', 'ordinaryFile');
      this.$axios.post(
        this.action,
        formData,
        res => {
          if (res.isSuccess) {
            this.relateBusinessFile(res.data.id);
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
            } else {
              const obj = {
                name: fileData.fileName,
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
    // handleRemove(k) {
    //   this.$confirm('确认要删除吗?', '提示', {
    //     confirmButtonText: '确定',
    //     cancelButtonText: '取消',
    //     type: 'warning'
    //   })
    //     .then(() => {
    //       const index = this.fileList.findIndex(item => item.id == k);
    //       if (index >= 0) {
    //         var targetFile = this.fileList[index];
    //         this.deleteFile(targetFile, index);
    //         if (!this.fileList.length) {
    //           this.$refs.upload.clearFiles();
    //         }
    //       }
    //     })
    //     .catch(() => { });
    // },
    deleteFile(file, index) {
      var data = {
        id: file.id,
        fileType: file.fileType,
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
          } else {
            this.$message.success('操作失败');
          }
        });
    },
    downLoad(href, name) {
      const lastIndex = name.lastIndexOf('.');
      const extendName = name.substr(lastIndex + 1);
      const ele = document.createElement('a');
      ele.target = '_blank';
      if (extendName == 'pdf' || icon_arr.indexOf(extendName) !== -1) {
        // 预览
        ele.href = href;
        ele.style.display = 'none';
        document.body.appendChild(ele);
        ele.click();
        document.body.removeChild(ele);
      } else {
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
      }
    },
    handleChange(fileObj) {
      console.log(fileObj,'fileObj++')
      this.fileList = [fileObj]
      // if (!this.ruleForm.folderId) {
      //   this.fileList = [];
      //   this.$message.error('请选择文件目录后在上传文件');
      //   return;
      // }
      // fileObj.percentage = 0;
      // fileObj.status = 'uploading';
      // this.fileList.push(fileObj);
      // if (this.setInt) clearInterval(this.setInt);
      // this.setInt = setInterval(() => {
      //   this.changeProgress();
      // }, 1000 / 16);
    },
    handleRemove(file, fileList){
      console.log(file, fileList)
      this.fileList = fileList
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
        }else{
          this.fileList[i].percentage = percent;
        }
      }
      if (uploaded == 0) if (this.setInt) clearInterval(this.setInt);
    },
    getRandom(min, max) {
      return Math.floor(Math.random() * (max - min + 1)) + min;
    },
    relateBusinessFile(fileId) {
      // 文件关联业务
      this.$axios.post(
        Api.schedule.saveAttachment,
        {
          data: {
            relationId: this.businessId,
            fileId
          }
        },
        (res) => {
          if (res.isSuccess) {
            this.loading = false
            this.successFileNUmber++;
            this.$message.success('上传成功');
            if (this.successFileNUmber == this.fileList.length) {
              this.fileList = []
              this.ruleForm.folderId = ''
              this.successFileNUmber = 0
              this.$emit('update:uploadDialogVisible', false);
              this.$emit('updateFile');
            }
          } else {
            this.loading = false
            this.$message.error('上传失败');
          }
        }
      );
    }
  }
};
</script>
