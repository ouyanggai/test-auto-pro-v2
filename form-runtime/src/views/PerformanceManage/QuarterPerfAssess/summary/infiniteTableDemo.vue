<template>
  <div class="infinite-scroll-table" style="height:100%;background-color: white;">
    <el-table ref="table" :data="tableData" border style="width: 100%" height="300">
      <el-table-column
        prop="id"
        label="ID"
        width="100"
        align="center"
      ></el-table-column>
      <el-table-column
        prop="name"
        label="名称"
        width="200"
      ></el-table-column>
      <el-table-column
        prop="date"
        label="日期"
        width="180"
        align="center"
      ></el-table-column>
      <el-table-column
        prop="address"
        label="地址"
      ></el-table-column>
    </el-table>
    <!-- <div v-if="loading" class="loading-mask">
      <el-loading-spinner></el-loading-spinner>
      <p class="loading-text">加载中...</p>
    </div>
    <div v-if="noMoreData && !loading" class="no-more">
      没有更多数据了
    </div> -->
  </div>
</template>

<script>
export default {
  data() {
    return {
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
    // 初始化加载第一页数据
    this.loadData();
    // 监听表格滚动事件
    this.initScrollListener();
  },
  beforeDestroy() {
    // 移除滚动事件监听，避免内存泄漏
    const tableBody = this.$refs.table.bodyWrapper;
    tableBody.removeEventListener('scroll', this.handleScroll);
  },
  methods: {
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
<style scoped>
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
</style>
