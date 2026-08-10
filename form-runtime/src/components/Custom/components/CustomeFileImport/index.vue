<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-09-12
-->
<template>
  <el-upload
    action="#"
    accept=".xlsx"
    :limit="1"
    :file-list="fileList"
    :show-file-list="false"
    :http-request="uploadHttpRequest"
    style="margin:0px 5px;"
  >
    <el-button type="primary" :disabled="disabled">导入模板</el-button>
  </el-upload>
</template>

<script>
import { viewFileUrl } from '@/config/env.js';
import { localstorageGet } from '@/utils/auth';

/* eslint-disable */
export default {
  name: 'CustomeFileView',
  components: {},
  props: {
    // value: {
    //   type: Object,
    //   default: function(){
    //     return {}
    //   }
    // },
    value: {
      type: String,
      default: ''
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      fileList:[],
      // fileDataObj:{},
    };
  },
  watch:{
    value(val) {
      console.log('val123--',val)
      // this.init(val)
    },
  },
  created() {
    
  },
  mounted() { },
  computed: {},
  methods: {
    uploadHttpRequest(param) {
      console.log('uploadHttpRequest')
      const sid = localstorageGet('token') || '';
      const formData = new FormData();
      formData.append('multipartFile', param.file);
      formData.append('sid', sid);
      this.$axios.post('/web/measuring/api/contractInvoicing/uploadFile', formData, res => {
        this.fileList = [];
        if (res.isSuccess) {
          this.$emit('input', JSON.stringify(res.data))
          this.$message.success('导入成功');
        } else {
          this.$message.warning(res.message);
        }
      }, {
        headers: { 'Content-Type': 'multipart/form-data', sid: sid }
      });
    },
    init(val){
      // let file = JSON.parse(JSON.stringify(val));
      // let fileObj = Object.keys(file).length > 0 ? file : {name:'',url:''}
      // this.fileDataObj = Object.assign({},this.fileDataObj,fileObj)
      // console.log('this.fileDataObj',this.fileDataObj)
    },
  },
};
</script>
<style lang="scss" scoped>
  // .file-wrap {
  //   margin:0 15px;
  //   color:#1989FA;
  //   padding: 4px 20px;
  //   .file-icon {
  //     color:#409eff;
  //     cursor: pointer;
  //     margin-left: 16px;
  //   }
  // }
  // .no-allow {
  //   background-color: #f5f7fa;
  //   border-color: #e4e7ed;
  //   color: #c0c4cc;
  //   cursor: not-allowed;
  //   .file-icon {
  //     color: #c0c4cc;
  //   }
  // }
</style>