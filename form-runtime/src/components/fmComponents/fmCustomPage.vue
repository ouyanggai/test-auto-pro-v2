<template>
  <!-- <div> -->
  <component
    v-if="allComponents.has(type)"
    v-model="modelValue"
    :is="type"
    :ref="type"
    :propData="propData"/>
    <!-- {{ modelValue }}
    </div> -->
</template>
<!-- <fm-custom-page v-model="dataModel" type="demo" :propData="{d:23,t:this}"></fm-custom-page> -->
<script>
import components from './components.js';
export default {
  name: 'fmCustomPage',
  props: ['type', 'propData', 'value'],
  components,
  data () {
    return {
      allComponents: new Set(Object.keys(components))
    };
  },
  computed: {
    modelValue: {
      get() {
        return this.value;
      },
      set(val) {
        console.log(val, 'modelValue-set');
        this.$emit('input', val);
      }
    }
  },
  watch: {
    // modelValue(value) {
    //   console.log(value, 'fmcustomepage-watch-this.value');
    // }
  },
  created() {},
  mounted() {
    // console.log(this.type, this.propData, 'type--propData380');
    // console.log(this.$refs, 'this.$refs380');
  },
  methods: {

  }
};

</script>
<style lang='scss' scoped>
.fmCostomPage233{
  // color: red;
}
</style>