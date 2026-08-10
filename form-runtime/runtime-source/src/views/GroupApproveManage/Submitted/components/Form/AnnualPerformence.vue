<template>
  <div>
    <h2>年度绩效</h2>
    <workTarget ref="workTarget"></workTarget>
    <div class="footer-bt">
      <div class="footer-inner">
        <el-button @click="prevStepHandle" plain>取 消</el-button>
        <el-button type="primary" plain @click="submit(0)">保 存</el-button>
        <el-button type="primary" @click="submit(1)">提 交</el-button>
      </div>
    </div>
  </div>

</template>

<script>
import workTarget from '@/views/PerformanceManage/TargetBook/WorkTarget/index'
export default {
  name:'AnnualPerformence',
  components: {
    workTarget
  },
  inject: ['prevStepHandle', 'sumbitFlow','submitFlowFinal'],
  props: [],
  data() {
    return {
    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {},
  methods: {
    submit(state){
      this.sumbitFlow()
      // this.$refs.workTarget.submit('form')
    },
    postData(){
      //提交业务
      this.$refs.workTarget.submit('form').then(res=>{
      }).catch(err=>{})

    }
  },
};
</script>
<style lang="scss" scoped>
h2 {
  text-align: center;
}
.footer-bt {
  text-align: center;
  background: transparent;
  width: 100%;
  position: absolute;
  left: 0;
  bottom: 10px;
  z-index: 1000;
  padding: 15px 10px;

  .footer-inner {

    background: #fff;
  }
}
</style>
