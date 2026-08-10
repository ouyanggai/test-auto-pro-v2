<template>
  <div class="infinite-scroll-table" style="height:100%;background-color: white;">
    <el-table ref="table" :data="tableList" border style="width: 100%" max-height="393" :cell-style="handleCellStyle" @cell-click="handleCellClick">
      <el-table-column prop="companyName" label="公司" v-if="tableType == '公司'" min-width="120"/>
      <el-table-column prop="depName" label="部门" v-if="tableType == '部门'" min-width="120"/>
      <el-table-column prop="level_a" label="A" align="center" min-width="70" class-name="rating-cell-custom">
        <template slot-scope="scope">
          <div @[eventName]="handleClick(scope.row, 'level_a')" :class="{ 'rating-num': !!scope.row.level_a }">
            {{ scope.row.level_a }}
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="level_b" label="B" align="center" min-width="70" class-name="rating-cell-custom">
        <template slot-scope="scope">
          <div @click="handleClick(scope.row, 'level_b')" :class="{ 'rating-num': !!scope.row.level_b }">
            {{ scope.row.level_b }}
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="level_c" label="C" align="center" min-width="70" class-name="rating-cell-custom">
        <template slot-scope="scope">
          <div @click="handleClick(scope.row, 'level_c')" :class="{ 'rating-num': !!scope.row.level_c }">
            {{ scope.row.level_c }}
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="level_d" label="D" align="center" min-width="70" class-name="rating-cell-custom">
        <template slot-scope="scope">
          <div @click="handleClick(scope.row, 'level_d')" :class="{ 'rating-num': !!scope.row.level_d }">
            {{ scope.row.level_d }}
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="level_e" label="E" align="center" min-width="70" class-name="rating-cell-custom">
        <template slot-scope="scope">
          <div @click="handleClick(scope.row, 'level_e')" :class="{ 'rating-num': !!scope.row.level_e }">
            {{ scope.row.level_e }}
          </div>
        </template>
      </el-table-column>
    </el-table>
    <!-- <div v-if="loading" class="loading-mask">
      <el-loading-spinner></el-loading-spinner>
      <p class="loading-text">加载中...</p>
    </div>
    <div v-if="noMoreData && !loading" class="no-more">
      没有更多数据了
    </div> -->
     <!-- <div class="rating-cell">
        <span class="rating-value rating-a">34</span>
      </div>
     <div class="rating-cell">
        <span class="rating-value rating-b">34</span>
      </div>
     <div class="rating-cell">
        <span class="rating-value rating-c">34</span>
      </div>
     <div class="rating-cell">
        <span class="rating-value rating-d">34</span>
      </div>
     <div class="rating-cell">
        <span class="rating-value rating-e">34</span>
      </div> -->
    <el-dialog center v-if="previewVisible" :visible.sync="previewVisible" append-to-body width="67%">
      <div class="dialog-container" style="min-height:200px;">
        <el-table :data="previewList" border max-height="550">
          <el-table-column prop="userName" label="姓名" align="center"/>
          <el-table-column prop="companyName" label="公司" align="center" min-width="120"></el-table-column>
          <el-table-column prop="depName" label="部门" align="center"/>
          <el-table-column prop="dutyName" label="岗位" align="center"></el-table-column>
          <el-table-column prop="name" label="考核时间" align="center">
            <template slot-scope="scope">
              {{ showPlanName(scope.row) }}
            </template>
          </el-table-column>
          <el-table-column prop="totalScore" label="分数" align="center"></el-table-column>
          <el-table-column prop="ratingLevel" label="评级" align="center">
            <template slot-scope="scope">
              {{ scope.row.ratingLevel == 'none' ? '未评级' : levelEnums[scope.row.ratingLevel] }}
            </template>
          </el-table-column>
           <el-table-column label="操作" align="center">
              <template slot-scope="scope">
                <el-button type="text" size="small" @click="viewPerformanceReview(scope.row)">详情</el-button>
              </template>
            </el-table-column>
        </el-table>
      </div>
      <div slot="footer" class="dialog-footer" style="text-align: center;">
        <el-button type="info" @click="previewVisible=false">关 闭</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
export default {
  props: {
    tableType: {
      type: String,
      default: '公司'
    },
    tableList: {
      type: Array,
      default: _ => []
    }
  },
  data() {
    return {
      eventName: 'click',
      eventName2: 'none',
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' },
      levelEnums: { 'level_a': 'A级', 'level_b': 'B级', 'level_c': 'C级', 'level_d': 'D级', 'level_e': 'E级' },
      previewVisible: false,
      previewList: [],
      timer: null,
      tableData: [],       // 表格数据
      page: 1,             // 当前页码
      pageSize: 10,        // 每页条数
      total: 0,            // 总条数
      loading: false,      // 加载状态
      noMoreData: false,   // 是否没有更多数据
      isScrolling: false   // 防止滚动事件频繁触发
    };
  },
  mounted() {
    setTimeout(() => {
      console.log(this.tableList, 'tableList - 传入的数据');
    }, 1000);
    // // 初始化加载第一页数据
    // this.loadData();
    // // 监听表格滚动事件
    // this.initScrollListener();
  },
  beforeDestroy() {
    // 移除滚动事件监听，避免内存泄漏
    // const tableBody = this.$refs.table.bodyWrapper;
    // tableBody.removeEventListener('scroll', this.handleScroll);
  },
  methods: {
    handleCellClick(row, column, cell, event) {
      console.log('cell clicked', row, column, cell, event);
      this.handleClick(row, column.property);
    },
    viewPerformanceReview(row) {
      this.$fm.show("flowDetail", { data: { flowInstanceBizRelevanceList: [{ otherBizId: row.id }] }});
    },
    showPlanName({targetTime}) {
      targetTime += '';
      console.log(targetTime, 'targetTime');
      const year = targetTime.substr(0, 4);
      const month = targetTime.substr(4, 2);
      return year + '年' + this.quarterEnums[month] + '季度';
    },
    handleClick(row, level) {
      if (row[level]) {
        this.previewList = row[level + '_kpi'] || [];
        this.previewVisible = true;
        console.log(row, 'row');
      }
    },
    handleCellStyle ({ row, column }) {
      console.log({ row, column }, 'row, column');
      const prop = column.property;
      const cellValue = row[prop];
      console.log(prop, cellValue, 'prop, cellValue');
      if (!cellValue) return {}; // 空值不设置样式
      if (prop === "level_a" && cellValue) {
        return { backgroundColor: "#007bff", color: "#fff" }; // 深蓝色
      } else if (prop === "level_b" && cellValue) {
        return { backgroundColor: "#5bc0de", color: "#fff" }; // 浅蓝色
      } else if (prop === "level_c" && cellValue) {
        return { backgroundColor: "#5cb85c", color: "#fff" }; // 绿色
      } else if (prop === "level_d" && cellValue) {
        return { backgroundColor: "#f0ad4e", color: "#fff" }; // 橙色
      } else if (prop === "level_e" && cellValue) {
        return { backgroundColor: "#d9534f", color: "#fff" }; // 红色
      }
      return {};
    },
    // 初始化滚动监听
    initScrollListener() {
      // 获取表格的滚动容器
      const tableBody = this.$refs.table.bodyWrapper;
      // 添加滚动事件监听
      tableBody.addEventListener('scroll', this.handleScroll);
    },
    // 滚动事件处理函数
    handleScroll() {
      // 防止频繁触发
      if (this.isScrolling) return;
      this.isScrolling = true;
      // 使用setTimeout防抖
      if(this.timer) clearTimeout(this.timer);
      this.timer = setTimeout(() => {
        const tableBody = this.$refs.table.bodyWrapper;
        const { scrollTop, scrollHeight, clientHeight } = tableBody;
        // 判断是否滚动到了底部（距离底部10px以内）
        if (scrollTop + clientHeight >= scrollHeight - 10) {
          this.loadMoreData();
        }
        this.isScrolling = false;
      }, 100);
    },
    // 加载数据
    loadData() {
      console.log('请求接口')
      // 显示加载状态
      this.loading = true;
      // 模拟API请求
      // 实际项目中替换为真实的接口调用
      setTimeout(() => {
        // 生成模拟数据
        const newData = Array.from({ length: this.pageSize }, (_, index) => ({
          id: (this.page - 1) * this.pageSize + index + 1,
          name: `项目名称 ${(this.page - 1) * this.pageSize + index + 1}`,
          date: new Date(2023, 0, 1 + Math.floor(Math.random() * 365)).toLocaleDateString(),
          address: `这是一段地址信息，用于测试表格内容的显示效果 ${index + 1}`
        }));
        // 如果是第一页，直接赋值；否则追加数据
        if (this.page === 1) {
          this.tableData = newData;
        } else {
          this.tableData = [...this.tableData, ...newData];
        }
        // 模拟总条数，实际项目中从接口获取
        this.total = 56;
        // 隐藏加载状态
        this.loading = false;
        // 判断是否还有更多数据
        this.noMoreData = this.tableData.length >= this.total;
      }, 800);
    },
    // 加载更多数据
    loadMoreData() {
      console.log('加载更多数据');
      // 如果正在加载中，或者没有更多数据，则不执行
      if (this.loading || this.noMoreData) return;
      // 页码加1
      this.page++;
      // 加载下一页数据
      this.loadData();
    }
  }
};
</script>
<style scoped lang="scss">
.el-table {
  /* margin-top: 30px; */
  box-shadow: 0 1px 4px rgba(0,0,0,0.08);
}
.rating-num {
  cursor: pointer;
}
.loading-mask {
  text-align: center;
  padding: 15px;
  color: #666;
}

.loading-text {
  margin-top: 10px;
  font-size: 14px;
}

.no-more {
  text-align: center;
  padding: 15px;
  color: #999;
  font-size: 14px;
}


    .rating-cell {
      /* display: flex;
      justify-content: center;
      align-items: center;
      height: 100%; */
      display: inline-block;
      width: 150px;
    }
    .rating-value {
      display: inline-block;
      padding: 5px 10px;
      border-radius: 4px;
      color: white;
      font-weight: bold;
      min-width: 50px;
      text-align: center;
    }
    .rating-a {
      background-color: #67C23A; /* 绿色 - 最高分 */
    }
    .rating-b {
      background-color: #409EFF; /* 蓝色 - 高分 */
    }
    .rating-c {
      background-color: #E6A23C; /* 橙色 - 中等分 */
    }
    .rating-d {
      background-color: #F56C6C; /* 红色 - 低分 */
    }
    .rating-e {
      background-color: #909399; /* 灰色 - 最低分 */
    }
::v-deep .rating-cell-custom:has(.rating-num) {
  padding: 0;
  cursor: pointer;
  &:hover {
      transform: scale(0.99);
      filter: opacity(0.8);
  }
}
</style>
