<!--
 * @Descripttion: 预算预览
 * @Author: liufuze
-->
<template>
  <el-dialog :visible.sync="previewVisible" :append-to-body="true" width="80%" :before-close="closeTemp">
    <!-- <cardTable></cardTable> -->
    <!-- <div style="overflow: hidden;">
                              <el-button type="primary" @click="closeTemp"> 返回 <i class=" el-icon-d-arrow-left"></i></el-button>
                            </div> -->
    <div style="height: 45vh;overflow: auto;margin-top: 15px;">
      <el-card v-for="(val, index) in datas" :key="index" style="margin-bottom:20px;position:relative;" shadow="hover">
        <template slot="header">
          <div class="card-title">
            <el-input v-model="val.departId" :disabled="true"></el-input>
          </div>
        </template>
        <el-collapse v-model="val['activeNames']">
          <el-collapse-item name="1">
            <el-table :data="val['budget']" :show-summary="true" border>
              <el-table-column type="index" label="编号" width="65px">
              </el-table-column>
              <el-table-column prop="budgetType" label="费用预算类型（一级）" class-name="budgetType">
                <template slot-scope="scope">
                  <el-input v-model="scope.row.budgetType" :disabled="true"></el-input>
                </template>
              </el-table-column>
              <el-table-column prop="relateProjId" label="是否关联项目" width="340px">
                <template slot-scope="scope">
                  <div>
                    <el-input v-model="scope.row.relateProjId" :disabled="true"></el-input>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="budgetMoney" label="预算金额(万)" width="160px">
                <template slot-scope="scope">
                  <el-input-number v-model="scope.row.budgetMoney" :min="0" :precision="6" :step="0.1" :controls="false"
                    :disabled="true"></el-input-number>
                </template>
              </el-table-column>
            </el-table>
          </el-collapse-item>
        </el-collapse>
      </el-card>
    </div>
  </el-dialog>
</template>
<script>
export default {
  name: 'BudgetInfoDialog',
  props: ['previewVisible', 'datas'],
  data() {
    return {
    }
  },
  methods: {
    closeTemp() {
      this.$emit("update:previewVisible", false)
    },
  }

}
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
