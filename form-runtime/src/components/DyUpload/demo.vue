<!--
 * @Descripttion:动态上传组件示例
 * @Author: Calvin
 * @Date: 2021-07-06 10:39:42
-->
<template>
  <div class="container">
    <DyUpload
      :action="'/file/uploadFile'"
      :fileList="fileList"
      :onSuccess="onUploadSuccess"
      :onRemove="onFileRemove"
    />
  </div>
</template>

<script>
import DyUpload from '@/components/DyUpload';
export default {
  name: '',
  components: { DyUpload },
  data() {
    return {
      fileList: [
        {
          fileName: '文件1'
        }
      ]
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() { },
  methods: {
    onUploadSuccess(res) {
      this.fileList.push(res.data);
    },
    // 删除功能：代码根据自己业务逻辑自定义编写
    onFileRemove(file) {
      this.fileList.findIndex((item, index) => {
        return item.fileName === file.fileName && this.fileList.splice(index, 1);
      });
      this.$message({
        message: '删除成功',
        type: 'success'
      });
    }
  }
};
</script>

<style scoped lang="scss">
</style>
