<template>
  <div class='globalDialog' style="width: 0;height: 0;overflow: hidden;">
    <component
    v-if="fm.visible"
    :visible.sync="fm.visible"
    :is="fm.currentComponent"
    :ref="fm.currentComponent"
    :propData="fm.propData"></component>
  </div>
</template>

<script>
// 使用示例：
// this.$fm.show('orgTree', {type: 'duty'}).then(dialog=>{  // show('弹窗名','props数据')打开对应弹窗
//     dialog.$on('confirmed',(res)=>{ // confirmed弹窗点确定后回调, res弹窗选择的数据
//         console.log(res,'res');
//     })
// })
import Vue from 'vue';
import components from '@/layout/components/globalDialog/components.js';
import Fm from '@/layout/components/globalDialog/Fm.js';
export default {
  name: '',
  components,
  data () {
    return {
      fm: new Fm(this)
    };
  },
  mounted() {},
  methods: {},
  computed: {},
  watch: {},
  created() {
    var str = this.$attrs.index || 1;
    str = str == 1 ? '' : str;
    Vue.prototype['$fm' + str] = this.fm;
  }

};
</script>
<!-- <style lang='scss' scoped>
.globalDialog{
  width: 0;
  height: 0;
  overflow: hidden;
}
</style> -->
