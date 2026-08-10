<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-27 16:41:51
-->
<!-- 集团下共有项目和公司下私有项目选择 -->

<template>
  <chooseComProject v-model="currentInfoObj" :disabled="disabled ? true : false"><!-- v-model动态绑定id -->
    <template v-slot="scope">
      <el-input readonly v-model="scope.viewName" style="width:100%" :disabled="disabled ? true : false"></el-input> <!-- v-model动态回显项目名称 -->
    </template>
  </chooseComProject>
  <!-- <el-select v-model="dataModel" :disabled="disabled">
    <el-option v-for="(val, index) in options" :key="index" :label="val.value" :value="val.value">
    </el-option>
  </el-select> -->
</template>

<script>
import chooseComProject from './chooseComProject';
/* eslint-disable */
export default {
  name: 'CustomeSelectProject',
  components: {chooseComProject},
  props: {
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
      currentInfoObj: this.value,
      // projectId: this.value,
      // modifyData:true,
      // dataModel: this.value,
    };
  },
  created() { },
  mounted() { },
  watch: {
    value(val) {
      console.log('val123',val)
      this.currentInfoObj = val
      // this.projectId = val
    },
    currentInfoObj(val){
      console.log('=======1',val)
      this.$emit('input', val)
      // this.$emit('input', val)
    },
  },
  computed: {},
  methods: {},
};
</script>
<style lang="scss" scoped></style>