<!--
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-04-29 11:22:57
-->
<template>
  <el-breadcrumb class="app-breadcrumb" separator="/">
    <transition-group name="breadcrumb">
      <el-breadcrumb-item v-for="(item, index) in levelList" :key="item.path">
        <span v-if="item.redirect === 'noRedirect' || index == levelList.length - 1" class="no-redirect">
          {{ item.meta.saasTitle || item.meta.title }}</span>
        <a v-else @click.prevent="handleLink(item)">{{ item.meta.saasTitle || item.meta.title }}</a>
      </el-breadcrumb-item>
    </transition-group>
  </el-breadcrumb>
</template>

<script>
import pathToRegexp from 'path-to-regexp';

export default {
  data() {
    return {
      levelList: null
    };
  },
  watch: {
    $route() {
      this.getBreadcrumb();
    }
  },
  created() {
    this.getBreadcrumb();
  },
  methods: {
    getBreadcrumb() {
      let matched = this.$route.matched.filter(item => item.meta && item.meta.title);
      console.log(matched, 'matched++', this.$route.matched);
      const first = matched[0];
      if (!this.isDashboard(first)) {
        matched = [].concat(matched);
      }
      this.levelList = matched.filter(item => item.meta && item.meta.title && item.meta.breadcrumb !== false);
      const length = this.levelList.length;
      if (this.levelList[length - 1].meta && this.levelList[length - 1].meta.pathAll) {
        this.levelList = [...this.levelList.slice(0, length - 1), ...this.levelList[length - 1].meta.pathAll, ...this.levelList.slice(length - 1)];
      }
    },
    isDashboard(route) {
      const name = route && route.name;
      if (!name) {
        return false;
      }
      return true;
    },
    pathCompile(path) {
      const { params } = this.$route;
      var toPath = pathToRegexp.compile(path);
      return toPath(params);
    },
    handleLink(item) {
      const { redirect, path } = item;
      if (redirect) {
        this.$router.push(redirect);
        return;
      }
      this.$router.push(this.pathCompile(path));
    }
  }
};
</script>

<style lang="scss" scoped>
.app-breadcrumb.el-breadcrumb {
  // display: inline-block;
  font-size: 13px;
  line-height: 50px;
  // 固定 50px 以配合 .app-main 的 margin-top: 100px（50 顶部栏 + 50 面包屑）
  height: 50px;
  box-sizing: border-box;
  padding-left: 20px;
  background: #f7f9fc;
  border-bottom: 1px solid #e8ecf2;

  a,
  .no-redirect {
    color: #8a94a6;
    transition: color 0.2s;
  }

  // 面包屑底是浅灰，青色对比度不足，这里用系统主色
  a:hover {
    color: #409EFF;
  }

  .no-redirect {
    cursor: text;
  }

  // 末级即当前页（模板渲染为 .no-redirect），加深加粗与可跳转项区分。
  // 这里刻意不用 ::v-deep —— 末级元素在本组件模板内，带 scope 属性，直接选中即可。
  .el-breadcrumb__item:last-child .no-redirect {
    color: #1f2d3d;
    font-weight: 600;
  }

  ::v-deep .el-breadcrumb__separator {
    color: #c0c6d0;
  }
}
</style>
