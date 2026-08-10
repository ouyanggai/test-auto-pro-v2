<!-- 一个或多个相同步骤的步骤条列表组件 -->
<template>
  <div class="m-steps-area">
    <div v-if="stepsItem.length">
      <div
        class="m-steps"
        v-for="item in stepsItem"
        :key="item.projectDivision"
      >
        <span class="step-title">{{item.projectDivision}}:</span>
        <!--@click="onChange(n)"-->
        <div
          :class="['m-steps-item',
          { 'finished': item.designProcess > n,
            'process': item.designProcess == n && n != totalSteps,
            'last-process': item.designProcess == totalSteps && n == totalSteps,
            'middle-wait': item.designProcess < n && n != totalSteps,
            'last-wait': item.designProcess < n && n == totalSteps,
          }
        ]"
          v-for="n in totalSteps"
          :key="n"
        >
          <div class="m-steps-icon">
            <span
              class="u-icon"
              v-if="item.designProcess<=n"
            >{{ n }}</span>
            <span
              class="el-icon-check u-icon"
              v-else
            ></span>
          </div>
          <div class="m-steps-content">
            <div class="u-steps-title">{{ stepsLabel[n-1] || 'S ' + n }}</div>
            <!-- 步骤文字下面描述-->
            <!-- <div class="u-steps-description">{{ stepsDesc[n-1] }}</div> -->
          </div>
        </div>
      </div>
    </div>
    <div
      class="empty"
      v-else
    >
      暂无数据
    </div>
  </div>
</template>
<script>
export default {
  name: 'Steps',
  props: {
    stepsLabel: { // 每个步骤的数组 ['步骤1', '步骤2', '步骤3'],
      type: Array,
      default: () => {
        return [];
      }
    },
    stepsItem: { // 步骤条数组对象
      type: Array,
      default: () => {
        return [];
      }
    },
    stepsDesc: { // 步骤description数组
      type: Array,
      default: () => {
        return [];
      }
    },
    totalSteps: { // 总的步骤数
      type: Number,
      default: 4
    }
    // currentStep: { // 当前选中的步骤
    //   type: Number,
    //   default: 1
    // }
  },
  data() {
    return {
      // 若当前选中步骤超过总步骤数，则默认选择步骤1
      // progress: this.currentStep > this.totalSteps ? 1 : this.currentStep
    };
  },
  methods: {
    // onChange(index) { // 点击切换选择步骤
    // console.log('index:', index)
    // if (this.progress !== index) {
    //   this.progress = index
    //   this.$emit('change', index)
    // }
    // }
  }
};
</script>
<style lang="scss" scoped>
$steps-color: #1890ff;
.m-steps-area {
  width: 100%;
  margin: 0px auto;
  height: 220px;
  overflow: auto;
  position: relative;
  .m-steps {
    padding: 15px 0 0;
    display: flex;
    .step-title {
      flex: 0.9;
      text-align: right;
      line-height: 28px;
      font-size: 14px;
      font-weight: bold;
      margin-right: 20px;
    }
    .m-steps-item {
      display: inline-block;
      flex: 1; // 弹性盒模型对象的子元素都有相同的长度，且忽略它们内部的内容
      flex-wrap: nowrap;
      white-space: nowrap;
      overflow: hidden;
      font-size: 14px;
      line-height: 28px;
      .m-steps-icon {
        display: inline-block;
        margin-right: 8px;
        width: 28px;
        height: 28px;
        border-radius: 50%;
        text-align: center;
      }
      .m-steps-content {
        display: inline-block;
        vertical-align: top;
        padding-right: 16px;
        .u-steps-title {
          position: relative;
          display: inline-block;
          padding-right: 16px;
        }
        .u-steps-description {
          font-size: 14px;
          max-width: 140px;
        }
      }
    }
    .finished {
      margin-right: 16px;
      //cursor: pointer;
      // &:hover {
      //   .m-steps-content {
      //     .u-steps-title {
      //       //color: $steps-color;
      //     }
      //     .u-steps-description {
      //       //color: $steps-color;
      //     }
      //   }
      // }
      .m-steps-icon {
        background: #fff;
        border: 1px solid rgba(0, 0, 0, 0.25);
        border-color: $steps-color;
        .u-icon {
          color: $steps-color;
        }
      }
      .m-steps-content {
        color: rgba(0, 0, 0, 0.65);
        .u-steps-title {
          color: rgba(0, 0, 0, 0.65);
          &:after {
            background: $steps-color;
            position: absolute;
            top: 16px;
            left: 100%;
            display: block;
            width: 9999px;
            height: 1px;
            content: "";
          }
        }
        .u-steps-description {
          color: rgba(0, 0, 0, 0.45);
        }
      }
    }
    .process {
      margin-right: 16px;
      .m-steps-icon {
        background: $steps-color;
        border: 1px solid rgba(0, 0, 0, 0.25);
        border-color: $steps-color;
        .u-icon {
          color: #fff;
        }
      }
      .m-steps-content {
        color: rgba(0, 0, 0, 0.65);
        .u-steps-title {
          font-weight: 600;
          color: rgba(0, 0, 0, 0.85);
          &:after {
            background: #e8e8e8;
            position: absolute;
            top: 16px;
            left: 100%;
            display: block;
            width: 9999px;
            height: 1px;
            content: "";
          }
        }
        .u-steps-description {
          color: rgba(0, 0, 0, 0.65);
        }
      }
    }
    .last-process {
      margin-right: 0;
      .m-steps-icon {
        background: $steps-color;
        border: 1px solid rgba(0, 0, 0, 0.25);
        border-color: $steps-color;
        .u-icon {
          color: #fff;
        }
      }
      .m-steps-content {
        color: rgba(0, 0, 0, 0.65);
        .u-steps-title {
          font-weight: 600;
          color: rgba(0, 0, 0, 0.85);
        }
        .u-steps-description {
          color: rgba(0, 0, 0, 0.65);
        }
      }
    }
    .middle-wait {
      margin-right: 16px;
      //cursor: pointer;
      // &:hover {
      //   .m-steps-icon {
      //     //border: 1px solid $steps-color;
      //     .u-icon {
      //       // color: $steps-color;
      //     }
      //   }
      //   .m-steps-content {
      //     .u-steps-title {
      //       // color: $steps-color;
      //     }
      //     .u-steps-description {
      //       // color: $steps-color;
      //     }
      //   }
      // }
      .m-steps-icon {
        background: #fff;
        border: 1px solid rgba(0, 0, 0, 0.25);
        .u-icon {
          color: rgba(0, 0, 0, 0.25);
        }
      }
      .m-steps-content {
        color: rgba(0, 0, 0, 0.65);
        .u-steps-title {
          color: rgba(0, 0, 0, 0.45);
          &:after {
            background: #e8e8e8;
            position: absolute;
            top: 16px;
            left: 100%;
            display: block;
            width: 9999px;
            height: 1px;
            content: "";
          }
        }
        .u-steps-description {
          color: rgba(0, 0, 0, 0.45);
        }
      }
    }
    .last-wait {
      margin-right: 0;
      // cursor: pointer;
      // &:hover {
      //   .m-steps-icon {
      //     //border: 1px solid $steps-color;
      //     .u-icon {
      //       //  color: $steps-color;
      //     }
      //   }
      //   .m-steps-content {
      //     .u-steps-title {
      //       //@debug  color: $steps-color;
      //     }
      //     .u-steps-description {
      //       //  color: $steps-color;
      //     }
      //   }
      // }
      .m-steps-icon {
        background: #fff;
        border: 1px solid rgba(0, 0, 0, 0.25);
        .u-icon {
          color: rgba(0, 0, 0, 0.25);
        }
      }
      .m-steps-content {
        color: rgba(0, 0, 0, 0.65);
        .u-steps-title {
          color: rgba(0, 0, 0, 0.45);
        }
        .u-steps-description {
          color: rgba(0, 0, 0, 0.45);
        }
      }
    }
  }
  .empty {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: #909399;
    font-size: 14px;
    font-weight: 300;
  }
}
</style>
