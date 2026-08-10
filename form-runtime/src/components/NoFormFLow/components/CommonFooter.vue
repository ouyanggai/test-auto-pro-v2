<template>
  <div class="footer-bt" >
    <div class="footer-inner" v-if="isReInitiate">
      <el-button type="primary" @click="reSubmit('save')">保存草稿</el-button>
      <el-button type="primary" @click="reSubmit('submit')">提交</el-button>
    </div>
    <div class="footer-inner" v-else>
      <el-button @click="cancel" plain>取 消</el-button>
      <el-button type="primary" plain @click="submit(0)">保 存</el-button>
      <el-button type="primary" @click="submit(1)">提 交</el-button>
    </div>
  </div>
</template>

<script>
export default {
  name:'CommonFooter',
  components: {},
  props: ['isReInitiate'],
  data() {
    return {
    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {},
  methods: {
    cancel(){
      var cp = this.$parent.$parent.$parent.$parent.$parent
      cp.prevStepHandle()
      // console.log(cp)
    },
    submit(status){
      this.$emit('submit',status)
    },
    reSubmit(type){
      this.$emit('reSubmit',type)
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
