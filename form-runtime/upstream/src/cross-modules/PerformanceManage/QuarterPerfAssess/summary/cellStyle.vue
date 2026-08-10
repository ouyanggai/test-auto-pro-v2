<template>
  <div class="score-table-container">
    <el-table
      :data="tableData"
      border
      style="width: 100%"
      :cell-style="handleCellStyle"
    >
      <el-table-column
        prop="name"
        label="姓名"
        align="center"
      />
      <el-table-column
        prop="department"
        label="部门"
        align="center"
      />
      <el-table-column
        prop="A"
        label="A"
        align="center"
      />
      <el-table-column
        prop="B"
        label="B"
        align="center"
      />
      <el-table-column
        prop="C"
        label="C"
        align="center"
      />
      <el-table-column
        prop="D"
        label="D"
        align="center"
      />
      <el-table-column
        prop="E"
        label="E"
        align="center"
      />
    </el-table>
  </div>
</template>

<script>
export default {
  name: "ScoreTable",
  data() {
    return {
      tableData: [
        { name: "招聘专员A", department: "人力资源部", A: 4.5, B: "", C: "", D: "", E: "" },
        { name: "人力资源专员A", department: "人力资源部", A: "", B: "", C: 3.6, D: "", E: "" },
        { name: "人力资源专员B", department: "人力资源部", A: 4.1, B: "", C: "", D: "", E: "" },
        { name: "行政专员C", department: "行政中心", A: "", B: 4, C: "", D: "", E: "" },
        { name: "行政专员D", department: "行政中心", A: "", B: "", C: "", D: 3.5, E: "" },
        { name: "后勤专员E", department: "后勤中心", A: "", B: 4, C: "", D: "", E: "" },
        { name: "后勤专员F", department: "后勤中心", A: "", B: "", C: "", D: 3.2, E: "" },
        { name: "企划专员G", department: "企划中心", A: "", B: "", C: 3.7, D: "", E: "" },
        { name: "企划专员H", department: "企划中心", A: "", B: "", C: "", D: "", E: 1.2 },
        { name: "法务专员I", department: "合规部", A: "", B: 4, C: "", D: "", E: "" },
      ],
    };
  },
  methods: {
    /** 自定义单元格样式：根据列和值设置背景色 */
    handleCellStyle({ row, column }) {
      console.log({row, column}, 'row, column')
      const prop = column.property; // 当前列的字段（如"A"、"B"等）
      const cellValue = row[prop]; // 当前单元格的值
      console.log(prop, cellValue, 'prop, cellValue');

      if (!cellValue) return {}; // 空值不设置样式

      // 按列和值匹配背景色
      if (prop === "A" && [4.5, 4.1].includes(cellValue)) {
        return { backgroundColor: "#007bff", color: "#fff" }; // 深蓝色
      } else if (prop === "B" && cellValue === 4) {
        return { backgroundColor: "#5bc0de", color: "#fff" }; // 浅蓝色
      } else if (prop === "C" && [3.6, 3.7].includes(cellValue)) {
        return { backgroundColor: "#5cb85c", color: "#fff" }; // 绿色
      } else if (prop === "D" && [3.5, 3.2].includes(cellValue)) {
        return { backgroundColor: "#f0ad4e", color: "#fff" }; // 橙色
      } else if (prop === "E" && cellValue === 1.2) {
        return { backgroundColor: "#d9534f", color: "#fff" }; // 红色
      }

      return {};
    },
  },
};
</script>

<style scoped>
.score-table-container {
  padding: 20px;
}
</style>