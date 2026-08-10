<!--
 * @Descripttion: 图表外框
 * @Author: zhengzetao
 * @Date: 2022-06-27 10:48:51
-->
<template>
  <div>
    <dv-border-box-1 class="lr_titles" v-if="typeNum && typeNum == 1">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-1>
    <dv-border-box-7 class="lr_titles" v-if="typeNum && typeNum == 7">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-7>
    <dv-border-box-8 class="lr_titles" v-if="typeNum && typeNum == 8">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-8>
    <dv-border-box-9 class="lr_titles" v-if="typeNum && typeNum == 9">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-9>
    <dv-border-box-11 class="lr_titles" v-if="typeNum && typeNum == 11">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-11>
    <dv-border-box-12 class="lr_titles" v-if="typeNum && typeNum == 12">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-12>

    <dv-border-box-13 class="lr_titles" v-if="typeNum == 0">
      <div class="item_title" v-if="title !== ''">
        <div class="zuo"></div>
        <span class="title-inner"> &nbsp;&nbsp;{{ title }}&nbsp;&nbsp; </span>
        <div class="you"></div>
      </div>
      <div :class="title !== '' ? 'item_title_content' : 'item_title_content_def'">
        <slot></slot>
      </div>
    </dv-border-box-13>
  </div>

</template>

<script>
export default {
  data() {
    return {};
  },
  props: {
    title: {
      type: String,
      default: () => ''
    },
    typeNum: {
      type: Number,
      default: 0
    }
  },
  created() { },

  mounted() { },
  methods: {}
};
</script>
<style lang='scss' scoped>
$item-title-height: 38px;
$item_title_content-height: calc(100% - 38px);

.lr_titles {
  box-sizing: border-box;
  cursor: default;

  ::v-deep .border-box-content {
    box-sizing: border-box;
    padding: 6px 16px 0px;
  }

  .item_title {
    height: $item-title-height;
    line-height: $item-title-height;
    width: 100%;
    color: #31abe3;
    text-align: center;
    // background: linear-gradient(to right, transparent, #0f0756, transparent);
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;

    .zuo,
    .you {
      width: 58px;
      height: 14px;
      background-image: url("../../assets/images/bigData/titles/zuo.png");
    }

    .you {
      transform: rotate(180deg);
    }

    .title-inner {
      font-weight: 900;
      padding-top: 4px;
      letter-spacing: 2px;
      background: linear-gradient(92deg,
          #0072ff 0%,
          #00eaff 48.8525390625%,
          #01aaff 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
  }

  .item_title_content {
    height: $item_title_content-height;
  }

  .item_title_content_def {
    width: 100%;
    height: 100%;
  }
}
</style>
