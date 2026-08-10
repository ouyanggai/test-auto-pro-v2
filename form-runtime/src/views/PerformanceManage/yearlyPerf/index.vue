<template>
  <div class="qualificationManagement">
    <el-tabs v-model="activeName" @tab-click="handleClick">
      <el-tab-pane label="全年考核结果" name="year" >
        <year v-if="activeName=='year'" :key="activeName" :searchType="activeName" :kpiScope="'my_company'"></year>
      </el-tab-pane>
      <el-tab-pane label="半年及年终考核结果" name="halfYearAndYear" >
        <year v-if="activeName=='halfYearAndYear'" :mergeRow="false" :key="activeName" :searchType="activeName" :kpiScope="'my_company'"></year>
      </el-tab-pane >
    </el-tabs>
  </div>
</template>

<script>
import year from './year.vue';
export default {
  name: '',
  components: { year },
  data() {
    return {
      activeName: 'year'
    };
  },
  methods: {
    handleClick(tab, event) {
      console.log(tab, event);
    }
  },
  computed: {},
  watch: {},
  created() {},
  mounted() {}
};
</script>
  <style lang='scss' scoped>
.qualificationManagement {
  height: 100%;
  background-color: white;
  padding: 15px;
}
</style>
