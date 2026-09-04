<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-20 15:35:38
-->
<template>
   <!-- :class="{'no-allow':disabled}" -->
  <div class="file-wrap">
    <span>{{ fileDataObj.name }}</span>
    <span v-if="fileDataObj.name">
      <el-tooltip content="下载">
        <i class="el-icon-download file-icon" v-if="fileDataObj.url" @click="downloadFile"></i>
      </el-tooltip>
      <el-tooltip content="预览">
        <i class="el-icon-view file-icon" v-if="fileDataObj.url" @click="viewFile(fileDataObj.url)"></i>
      </el-tooltip>
    </span>
    <span v-else>
      暂无文件
    </span>
  </div>
</template>

<script>
//
import { localstorageGet } from '@/utils/auth';
import { viewFile } from '@/utils';
import { parseJsonObject } from '@/utils/parse-value';

/* eslint-disable */
export default {
  name: 'CustomeFileView',
  components: {},
  props: {
    value: {
      type: Object,
      default: function(){
        return {}
      }
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      // dataModel: this.value,
      fileDataObj:{},
      test:true
    };
  },
  watch:{
    value(val) {
      console.log('这是文件组件val123--',val)
      this.init(val)
      // this.projectId = val
    },
    // fileDataObj:{
    //   handler:function(val){
    //     console.log(val,111)
    //   },
    //   deep:true
    // }
  },
  created() {

  },
  mounted() { },
  // watch: {
  //   value(val) {
  //     this.dataModel = val
  //   },
  //   dataModel(val) {
  //     this.$emit('input', val)
  //   }
  // },
  computed: {},
  methods: {
    init(val){
      let file = parseJsonObject(val);
      let fileObj = Object.keys(file).length > 0 ? file : {name:'',url:''}
      this.fileDataObj = Object.assign({},this.fileDataObj,fileObj)
      console.log('表单文件组件-this.fileDataObj',this.fileDataObj)
    },
    // 下载文件
    downloadFile() {
      let needSlice = this.fileDataObj.url.lastIndexOf('.')
      let fileSuffix = this.fileDataObj.url.slice(needSlice)
      console.log('下载文件',this.fileDataObj.name+fileSuffix)
      this.downLoad(this.fileDataObj.url, this.fileDataObj.name+fileSuffix);
    },
    // 预览
    viewFile(url) {
      viewFile(url)
    },
    downLoad(href, name) {
      const ele = document.createElement('a');
      ele.target = '_blank';
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
    },
  },
};
</script>
<style lang="scss" scoped>
  .file-wrap {
    margin:0 15px;
    color:#1989FA;
    padding: 4px 20px;
    .file-icon {
      color:#409eff;
      cursor: pointer;
      margin-left: 16px;
    }
  }
  .no-allow {
    background-color: #f5f7fa;
    border-color: #e4e7ed;
    color: #c0c4cc;
    cursor: not-allowed;
    .file-icon {
      color: #c0c4cc;
    }
  }
</style>
