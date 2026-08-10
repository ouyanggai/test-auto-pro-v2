<!--
 * @Descripttion: 发票信息列表组件
 * @Author: liufuze
-->
<template>
  <div style="margin-top:10px">
    <div>
      <el-upload ref="upload" action="" :http-request="httpRequest" :on-change="handleChange" class="upload"
        :multiple="true">
        <el-button type="primary">上传发票<i class="el-icon-upload el-icon--right"></i>
        </el-button>
      </el-upload>
      <div style="margin-top: 15px;">
        <span class="warning"><i class="el-icon-warning-outline"></i> 该发票信息为第三方接口解析结果，请务必再次核对发票信息与附件的一致性、准确性，谢谢！</span>
      </div>
      <div class="tab-div">
        <el-radio-group size="medium" v-model="billType">
          <el-radio-button v-for="item in tabList" :value="item.id" :label="item.id">
            <el-badge :value="item.num" class="item">{{ item.name }}</el-badge>
          </el-radio-button>
        </el-radio-group>
        <div>

        </div>
      </div>

    </div>
  </div>
</template>
<script>
import { deepClone } from '@/utils';
export default {
  name: 'BillTable',
  props: [],
  data() {
    return {
      fileList: [],
      billType: 1,
      tabList: [
        { id: 1, name: '火车票', num: 1 },
        { id: 2, name: '汽车票', num: 2 },
        { id: 3, name: '全店票', num: 6 },
        { id: 4, name: '火车票', num: 5 },
        { id: 5, name: '出租车票', num: 9 },
      ]
    }
  },
  created() {
  },
  methods: {
    httpRequest(param) {
      const { file } = param;
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
          } else {
            this.$message.error('上传失败');
            this.$refs.upload.clearFiles();
          }
        },
        true
      );
    },
    handleChange() {

    }
  }
}
</script>
<style scoped>
::v-deep .el-tabs__nav-wrap,
::v-deep .el-tabs__nav-scroll {
  overflow: visible;
}

.tab-div {
  overflow: hidden;
  margin-top: 15px;
  padding: 15px 0;
}

.warning {
  color: #fff;
  background: chocolate;
  padding: 5px 10px;
  border-radius: 4px;
}

::v-deep .el-badge__content.is-fixed {
  top: -10px;
}
</style>
