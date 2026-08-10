<template>
  <el-select v-model="dataModel" :disabled="disabled">
    <el-option v-for="(val, index) in options" :key="index" :label="val.value" :value="val.value">
    </el-option>
  </el-select>
</template>

<script>
/* eslint-disable */
export default {
  name: 'CustomeWeather',
  components: {},
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
      dataModel: this.value,
      options: [
        {
          value: '晴朗'
        },
        {
          value: '多云'
        },
        {
          value: '多云转晴'
        },
        {
          value: '雷阵雨转晴'
        },
        {
          value: '小雨'
        },
        {
          value: '中雨'
        },
        {
          value: '大雨'
        },
        {
          value: '小雪'
        },
        {
          value: '中雪'
        },
        {
          value: '大雪'
        },
        {
          value: '暴雪'
        },
        {
          value: '雨夹雪'
        },
        {
          value: '夜晚'
        },
        {
          value: '冰雹'
        },
        {
          value: '沙尘暴'
        },
        {
          value: '雾霾'
        },
        {
          value: '雨加冰雹'
        },
        {
          value: '雷阵雨'
        },
        {
          value: '中雨转晴'
        },
        {
          value: '夜晚多云'
        },
        {
          value: '霜降'
        }
      ],
    };
  },
  created() { },
  mounted() { },
  watch: {
    value(val) {
      this.dataModel = val
    },
    dataModel(val) {
      this.$emit('input', val)
    }
  },
  computed: {},
  methods: {},
};
</script>
<style lang="scss" scoped></style>