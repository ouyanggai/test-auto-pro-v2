<template>
  <div>
    <!-- <el-dialog :visible.sync="detailVisible" title="费用明细" :modal-append-to-body="true" :close-on-click-modal="false"
      @closed="closed" @close="closeDialog" :before-close="closeDialog" width="800px"> -->
    <div>
      <div class="top">
        <el-select v-model="query.depart" placeholder="请选择部门" style="margin-right:10px">
          <el-option v-for="item in depart" :key="item.value" :label="item.label" :value="item.value">
          </el-option>
        </el-select>
        <el-select v-model="query.costType" placeholder="请选择预算类型" style="margin-right:10px">
          <el-option v-for="item in costType" :key="item.value" :label="item.label" :value="item.value">
          </el-option>
        </el-select>
        <el-select v-model="query.date" placeholder="日期" style="margin-right:10px">
          <el-option v-for="item in dateList" :key="item.value" :label="item.label" :value="item.value">
          </el-option>
        </el-select>
      </div>
      <div class="content" style="padding:15px 0;">
        <el-table :data="tableData" :show-summary="true">
          <el-table-column type="index" width="50"> </el-table-column>
          <el-table-column prop="budget" label="金额">
          </el-table-column>
          <el-table-column prop="name" label="姓名"> </el-table-column>
          <el-table-column prop="createTime" label="创建时间">
          </el-table-column>
          <el-table-column prop="depart" label="部门"> </el-table-column>
          <el-table-column prop="budgetType" label="费用预算类型">
          </el-table-column>
          <el-table-column prop="costType" label="费用类型">
          </el-table-column>
          <el-table-column fixed="right" label="操作" width="100">
            <template slot-scope="scope">
              <el-button @click="handleClick(scope.row)" type="text" size="small">详细</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <div slot="footer" class="dialog-footer" style="text-align:right;">
        <el-button @click="closeDialog" type="primary">关 闭</el-button>
      </div>
    </div>
    <!-- </el-dialog> -->
  </div>
</template>

<script>
export default {
  name: "detailedCost",
  props: ["detailVisible"],
  data() {
    return {
      query: { depart: "", costType: "", date: "" },
      depart: [
        { label: "开发部", value: 1 },
        { label: "设计部", value: 2 }
      ],
      costType: [
        { label: "交通预算", value: 1 },
        { label: "住宿预算", value: 2 }
      ],
      dateList: [
        { label: "2022-05", value: 1 },
        { label: "2022-06", value: 2 }
      ],
      tableData: [
        {
          id: 1,
          budget: 1000.0,
          name: "刘玄德",
          createTime: "2022-06",
          depart: "开发部",
          budgetType: "营销部业务费",
          costType: "餐费"
        },
        {
          id: 2,
          budget: 1300.0,
          name: "曹孟德",
          createTime: "2022-07",
          depart: "营销部",
          budgetType: "推广费用",
          costType: "打车费"
        }
      ]
    };
  },
  methods: {
    closed() { },
    closeDialog() {
      this.$emit("update:detailVisible", false);
    }
  }
};
</script>

<style>
</style>
