<template>
  <div class="scroll-container" ref="container"
    :style="containerStyle"
    @mousedown="startDrag"
    @wheel.prevent="handleWheel"
    @mousemove="handleDrag"
    @mouseup="endDrag"
    @mouseleave="endDrag">
      <div class="content" ref="content" :style="contentStyle">
        <slot></slot> <!-- 内容插槽 -->
      </div>
    <span class="zoomTip">Ctrl+鼠标滚轮缩放</span>
    <span class="sizeIcon">
      <i class="el-icon-remove-outline" @click="clickScale($event, 100)" title="鼠标滚轮↓缩小"></i>
      <span class="font">{{ parseInt(scale*100) }}%</span>
      <i class="el-icon-circle-plus-outline" @click="clickScale($event, -100)" title="鼠标滚轮↑放大"></i>
    </span>
  </div>
</template>
<script>
export default {
  name: 'DragZoomScroll',
  props: {
    initialScale: { // 初始缩放比例
      type: Number,
      default: 1
    }
  },
  data() {
    return {
      isDragging: false, // 拖拽状态
      startX: 0, // 拖拽起始X坐标
      startY: 0, // 拖拽起始Y坐标
      translateX: 0, // X轴位移
      translateY: 0, // Y轴位移
      scale: this.initialScale, // 当前缩放比例
      lastDragPosition: { x: 0, y: 0 } // 上次拖拽位置
    };
  },
  computed: {
    containerStyle() { // 容器样式（控制指针形状）
      return {
        cursor: this.isDragging ? 'grabbing' : 'grab'
      };
    },
    contentStyle() { // 内容区域样式（变换动画）
      return {
        transform: `
          translate(${this.translateX}px, ${this.translateY}px)
          scale(${this.scale})
        `,
        transition: 'transform 0.3s ease-out'
      };
    }
  },
  methods: {
    // 计算边界限制
    getBoundLimits() {
      const container = this.$refs.container;
      const content = this.$refs.content;
      if (!container || !content) return { minX: 0, maxX: 0, minY: 0, maxY: 0 };

      const containerWidth = container.clientWidth;
      const containerHeight = container.clientHeight;
      // 使用 scrollWidth/scrollHeight 获取内容原始尺寸，不受 transform 影响
      // scrollWidth/scrollHeight 包含溢出的内容尺寸
      let contentWidth = content.scrollWidth;
      let contentHeight = content.scrollHeight;

      // 如果 scrollWidth/scrollHeight 为 0（内容还没渲染或未溢出），使用 offsetWidth/offsetHeight
      if (contentWidth === 0) contentWidth = content.offsetWidth;
      if (contentHeight === 0) contentHeight = content.offsetHeight;

      // 应用缩放后的内容尺寸
      const scaledWidth = contentWidth * this.scale;
      const scaledHeight = contentHeight * this.scale;

      // 计算 X 轴边界：内容宽 < 容器宽时居中，否则限制在 [containerWidth-scaledWidth, 0]
      let minX, maxX;
      if (scaledWidth <= containerWidth) {
        minX = (containerWidth - scaledWidth) / 2;
        maxX = minX;
      } else {
        minX = containerWidth - scaledWidth;
        maxX = 0;
      }

      // 计算 Y 轴边界：内容高 < 容器高时居中，否则限制在 [containerHeight-scaledHeight, 0]
      let minY, maxY;
      if (scaledHeight <= containerHeight) {
        minY = (containerHeight - scaledHeight) / 2;
        maxY = minY;
      } else {
        minY = containerHeight - scaledHeight;
        maxY = 0;
      }

      return { minX, maxX, minY, maxY };
    },
    // 限制值在边界内
    clampTranslate(value, min, max) {
      return Math.min(Math.max(value, min), max);
    },
    clickScale(e, deltaY) {
      const delta = deltaY > 0 ? 0.8 : 1.2;
      const newScale = Math.min(Math.max(0.3, this.scale * delta), 3);
      const rect = this.$refs.container.getBoundingClientRect();
      const offsetX = window.innerWidth / 2 - rect.left;
      const offsetY = window.innerHeight / 2 - rect.top;

      // 先计算新的位移，再限制边界
      const newTranslateX = offsetX - (offsetX - this.translateX) * (newScale / this.scale);
      const newTranslateY = offsetY - (offsetY - this.translateY) * (newScale / this.scale);

      // 临时更新 scale 以便 getBoundLimits 计算正确的边界
      this.scale = newScale;

      const limits = this.getBoundLimits();
      this.translateX = this.clampTranslate(newTranslateX, limits.minX, limits.maxX);
      this.translateY = this.clampTranslate(newTranslateY, limits.minY, limits.maxY);
    },
    startDrag(e) { // 开始拖拽
      this.isDragging = true;
      this.startX = e.clientX - this.translateX;
      this.startY = e.clientY - this.translateY;
      this.lastDragPosition = { x: this.translateX, y: this.translateY };
    },
    handleDrag(e) { // 处理拖拽
      if (!this.isDragging) return;
      const deltaX = e.clientX - this.startX;
      const deltaY = e.clientY - this.startY;

      // 应用边界限制
      const limits = this.getBoundLimits();
      this.translateX = this.clampTranslate(deltaX, limits.minX, limits.maxX);
      this.translateY = this.clampTranslate(deltaY, limits.minY, limits.maxY);
    },
    endDrag() { // 结束拖拽
      this.isDragging = false;
    },
    handleWheel(e) { // 处理滚轮缩放
      if (e.ctrlKey || e.shiftKey || e.altKey) {
        e.preventDefault();
        const delta = e.deltaY > 0 ? 0.9 : 1.1;
        const newScale = Math.min(Math.max(0.3, this.scale * delta), 3);

        const rect = this.$refs.container.getBoundingClientRect();
        const offsetX = e.clientX - rect.left;
        const offsetY = e.clientY - rect.top;

        const newTranslateX = offsetX - (offsetX - this.translateX) * (newScale / this.scale);
        const newTranslateY = offsetY - (offsetY - this.translateY) * (newScale / this.scale);

        this.scale = newScale;

        const limits = this.getBoundLimits();
        this.translateX = this.clampTranslate(newTranslateX, limits.minX, limits.maxX);
        this.translateY = this.clampTranslate(newTranslateY, limits.minY, limits.maxY);
      } else {
        e.preventDefault();
        const limits = this.getBoundLimits();
        const newTranslateY = this.translateY - e.deltaY;
        this.translateY = this.clampTranslate(newTranslateY, limits.minY, limits.maxY);
      }
      document.body.click();
    }
  }
};
</script>
<style scoped lang="scss">
.scroll-container {
  width: 100%;
  height: 100%;
  overflow: hidden;
  position: relative;
  user-select: none;
  display: flex;
  justify-content: center;
}
.zoomTip{
  opacity: 0.8;
  font-size: 16px;
  line-height: 24px;
  position: absolute;
  top: 2%;
  left: 3%;
}
.sizeIcon{
  opacity: 0.8;
  font-size: 24px;
  position: absolute;
  top: 2%;
  right: 3%;
  .font{
    font-size: 16px;
    vertical-align: middle;
  }
  i {
    vertical-align: middle;
    cursor: pointer;
    margin-right: 4px;
    &:hover {
      color: #191f25;
    }
  }
}
.content {
  width: 100%;
  height: 100%;
  position: absolute;
  transform-origin: 0 0; /* 缩放基点 */
  will-change: transform; /* 优化动画性能 */
  user-select: none; /* 防止选中内容 */
  display: flex;
  justify-content: center;
}
/* 隐藏原生滚动条 */
.scroll-container::-webkit-scrollbar {
  width: 0;
  height: 0;
  background: transparent;
}
</style>
