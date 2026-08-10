<template>
  <div 
    class="split-panel" 
    :class="{ 'horizontal': direction === 'horizontal', 'vertical': direction === 'vertical' }"
    :style="panelStyle"
  >
    <!-- 左侧/上方面板 -->
    <div 
      class="panel panel-left" 
      :style="leftPanelStyle"
    >
      <slot name="left"></slot>
    </div>
    
    <!-- 分割线 -->
    <div 
      class="splitter" 
      :style="splitterStyle"
      @mousedown="startDrag"
      :class="{ dragging: isDragging }"
    ></div>
    
    <!-- 右侧/下方面板 -->
    <div 
      class="panel panel-right" 
      :style="rightPanelStyle"
    >
      <slot name="right"></slot>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SplitPanel',
  props: {
    // 分割方向：horizontal(水平) 或 vertical(垂直)
    direction: {
      type: String,
      default: 'vertical',
      validator: (value) => ['horizontal', 'vertical'].includes(value)
    },
    // 初始分割比例，左侧/上方面板占比
    initialRatio: {
      type: Number,
      default: 0.5,
      validator: (value) => value > 0 && value < 1
    },
    // 最小比例限制，防止面板过小
    minRatio: {
      type: Number,
      default: 0.1,
      validator: (value) => value >= 0 && value < 1
    },
    // 最大比例限制
    maxRatio: {
      type: Number,
      default: 0.9,
      validator: (value) => value > 0 && value <= 1
    },
    // 分割线宽度
    splitterSize: {
      type: Number,
      default: 4
    }
  },
  data() {
    return {
      // 当前左侧/上方面板占比
      currentRatio: this.initialRatio,
      // 是否正在拖拽
      isDragging: false,
      // 记录初始拖拽位置
      startPos: 0
    };
  },
  computed: {
    // 面板容器样式
    panelStyle() {
      return {
        width: '100%',
        height: '100%',
        display: 'flex'
      };
    },
    // 左侧/上方面板样式
    leftPanelStyle() {
      if (this.direction === 'vertical') {
        return {
          width: `${this.currentRatio * 100}%`,
          height: '100%',
          overflow: 'auto'
        };
      } else {
        return {
          width: '100%',
          height: `${this.currentRatio * 100}%`,
          overflow: 'auto'
        };
      }
    },
    // 右侧/下方面板样式
    rightPanelStyle() {
      if (this.direction === 'vertical') {
        return {
          width: `${(1 - this.currentRatio) * 100}%`,
          height: '100%',
          overflow: 'auto'
        };
      } else {
        return {
          width: '100%',
          height: `${(1 - this.currentRatio) * 100}%`,
          overflow: 'auto'
        };
      }
    },
    // 分割线样式
    splitterStyle() {
      const style = {
        backgroundColor: '#e0e0e0',
        userSelect: 'none',
        transition: 'background-color 0.2s'
      };
      
      if (this.direction === 'vertical') {
        style.width = `${this.splitterSize}px`;
        style.height = '100%';
        style.cursor = 'col-resize';
      } else {
        style.height = `${this.splitterSize}px`;
        style.width = '100%';
        style.cursor = 'row-resize';
      }
      
      // 拖拽时的样式变化
      if (this.isDragging) {
        style.backgroundColor = '#999';
      }
      
      return style;
    }
  },
  methods: {
    // 开始拖拽
    startDrag(e) {
      this.isDragging = true;
      this.startPos = this.direction === 'vertical' ? e.clientX : e.clientY;
      
      // 添加鼠标移动和释放事件监听
      document.addEventListener('mousemove', this.onDrag);
      document.addEventListener('mouseup', this.endDrag);
      
      // 防止拖动时选中文本
      e.preventDefault();
    },
    // 拖拽中
    onDrag(e) {
      if (!this.isDragging) return;
      
      // 获取容器尺寸
      const containerSize = this.direction === 'vertical' 
        ? this.$el.clientWidth 
        : this.$el.clientHeight;
      
      // 计算拖拽距离
      const dragDistance = (this.direction === 'vertical' ? e.clientX : e.clientY) - this.startPos;
      
      // 计算新的比例
      let newRatio = this.currentRatio + dragDistance / containerSize;
      
      // 限制在最小和最大比例之间
      newRatio = Math.max(this.minRatio, Math.min(this.maxRatio, newRatio));
      
      // 更新比例
      this.currentRatio = newRatio;
      this.startPos = this.direction === 'vertical' ? e.clientX : e.clientY;
      
      // 触发事件，通知父组件比例变化
      this.$emit('ratio-change', this.currentRatio);
    },
    // 结束拖拽
    endDrag() {
      this.isDragging = false;
      // 移除事件监听
      document.removeEventListener('mousemove', this.onDrag);
      document.removeEventListener('mouseup', this.endDrag);
    }
  },
  // 监听容器尺寸变化，调整比例
  watch: {
    '$el.clientWidth'(newVal, oldVal) {
      if (this.direction === 'vertical' && oldVal && newVal !== oldVal) {
        // 容器宽度变化时保持比例
        this.currentRatio = Math.max(
          this.minRatio, 
          Math.min(this.maxRatio, this.currentRatio)
        );
      }
    },
    '$el.clientHeight'(newVal, oldVal) {
      if (this.direction === 'horizontal' && oldVal && newVal !== oldVal) {
        // 容器高度变化时保持比例
        this.currentRatio = Math.max(
          this.minRatio, 
          Math.min(this.maxRatio, this.currentRatio)
        );
      }
    }
  }
};
</script>

<style scoped>
.split-panel {
  position: relative;
}

.panel {
  box-sizing: border-box;
}

.splitter {
  position: relative;
  /* 拖拽条悬停效果 */
}

.splitter:hover {
  background-color: #ccc;
}

/* 垂直分割 */
.vertical {
  flex-direction: row;
}

/* 水平分割 */
.horizontal {
  flex-direction: column;
}

/* 拖拽时的样式 */
.splitter.dragging {
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}
</style>
