<!--  -->
<template>
  <div class='app'>
    <year :kpiScope="'personal'"></year>
  </div>
</template>

<script>
import year from '../yearlyPerf/year.vue';
export default {
  name: '',
  components: { year },
  data () {
    return {
    };
  },
  computed: {},
  watch: {},
  created() {},
  mounted() {},
  methods: {}
}

</script>
<style lang='scss' scoped>
.app{
  height: 100%;
  background-color: white;
  padding: 20px 15px 20px 15px;
}
</style>