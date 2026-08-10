<template>
  <div class="borrowMoneyManage">
    <el-tabs v-model="activeName" @tab-click="handleClick">
      <el-tab-pane label="公司借款管理" name="company">
      </el-tab-pane>
      <el-tab-pane label="人员借款管理" name="person" >

      </el-tab-pane>
      <company-borrow :activeName="activeName" :key="index"></company-borrow>
    </el-tabs>
  </div>
</template>

<script>
import CompanyBorrow from './components/companyBorrow.vue'
export default {
  name: '',
  components: { CompanyBorrow },
  data() {
    return {
      activeName: 'company',
      index: 0
    };
  },
  methods: {
    handleClick(tab, event) {
      console.log(tab, event);
      this.index++;
    },
  },
  computed: {},
  watch: {},
  created() {},
  mounted() {},
};
</script>
<style lang='scss' scoped>
.borrowMoneyManage {
  height: 100%;
  background-color: white;
  padding: 15px;
}
</style>