<template>
  <div class="printTable">
    <el-table :data="printTableData" header-row-class-name="table-header" style="margin-top: 10px;">
      <el-table-column v-for="(el, index) in printTableConfig" :key="index" :prop="el.prop" :label="el.label"
        :width="el.width" align="center">
        <template slot-scope="scope">
          {{ scope.row[el.prop] }}
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
export default {
  name: 'PrintTable',
  props: {
    printTableConfig: {
      type: Array,
      default() {
        return [];
      }
    },
    printTableData: {
      type: Array,
      default() {
        return [];
      }
    }
  },
  data() {
    return {

    };
  }
};
</script>

<style lang="scss" scoped>
.printTable {}
</style>
