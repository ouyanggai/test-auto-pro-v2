<template>
  <div class="queryModule">
    <span>费用预算类型：</span>
    <el-select v-model="value" placeholder="请选择">
      <el-option
        v-for="item in options"
        :key="item.value"
        :label="item.label"
        :value="item.value">
      </el-option>
    </el-select>
    <span style="margin-left: 20px;">统计时间：</span>
    <el-date-picker
      clearable
      v-model="value1"
      type="daterange"
      range-separator="至"
      start-placeholder="开始日期"
      end-placeholder="结束日期">
    </el-date-picker>
    <el-button icon="el-icon-search" type="primary" style="margin-left: 20px;">查询</el-button>
    <div style="margin: 20px 0;font-size: 16px;font-weight: bold;">
      <span>项目名称：</span>
      <span>深圳润世华总部大楼全过程建设工程</span>
      <span style="margin-left: 40px;">单位：</span>
      <span>万元</span>
    </div>
  </div>
</template>

<script>
  export default {
  name: "queryModule",
  props: {

  },
  data() {
    return {
      options: [{
        value: '选项1',
        label: '黄金糕'
      }, {
        value: '选项2',
        label: '双皮奶'
      }, {
        value: '选项3',
        label: '蚵仔煎'
      }, {
        value: '选项4',
        label: '龙须面'
      }, {
        value: '选项5',
        label: '北京烤鸭'
      }],
      value: '',
      value1:''
    }
  },
  watch: {

  },
  computed: {

  },
  methods: {

  },
  created() {

  },
  mounted() {

  },
  updated() {

  },
  destroyed() {

  }
}
</script>

<style lang="scss" scoped>
.queryModule{

}
</style>