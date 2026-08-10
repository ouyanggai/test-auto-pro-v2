<!--
 * @Author: treasure
 * @Description: 新增节点
 * @FilePath: \workflowEngine\src\views\home\components\addNode.vue
-->
<template>
  <div class="add-node-con">
    <div class="add-node-btn add-node-arrow">
      <el-popover
        v-model="visible"
        placement="right"
        width="130"
        trigger="click"
      >
        <a
          class="add-node-popover-item approver"
          @click="addType(sort)"
        >
          <div class="item-wrapper">
            <i class="el-icon-user-solid icon-font" />
          </div>
          <p>审批人</p>
        </a>
        <el-button
          slot="reference"
          class="btn-plus"
          type="primary"
          icon="el-icon-plus"
          circle
        />
      </el-popover>
    </div>
  </div>
</template>
<script>
export default {
  name: '',
  components: {},
  props: {
    addNodeData: {
      type: Object,
      default: () => {
        return {};
      }
    },
    sort: {
      type: Number
    }
  },
  data() {
    return {
      visible: false,
      type: ''
    };
  },
  computed: {},
  watch: {},
  created() {
    this.type = this.$route.query.type;
  },
  mounted() { },
  methods: {
    // 新增
    addType(sort) {
      // if (this.type) {
      //   this.$message.error('编辑、查看不能新增流程节点');
      //   return;
      // }
      this.visible = false;
      const data = { nodeName: '审核人', flowNodeFieldPowerTemplateList: [] };
      data.sort = sort + 1;
      this.$emit('handleAddNode', data);
    }
  }
};
</script>
<style scoped lang='scss'>
.add-node-con {
  padding-bottom: 6px;
  .add-node-btn {
    position: relative;
    width: 240px;
    padding: 36px 0;
    display: flex;
    -webkit-box-pack: center;
    -ms-flex-pack: center;
    justify-content: center;
    .btn-plus:hover {
      transform: scale(1.3);
      box-shadow: 0 13px 27px 0 rgba(0, 0, 0, 0.1);
    }
    .btn-plus {
      position: relative;
      z-index: 3;
    }
  }

  .add-node-btn:after {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 1;
    margin: auto;
    width: 2px;
    height: 100%;
    box-sizing: border-box;
    background-color: #cacaca;
  }
  .add-node-arrow:before {
    content: "";
    position: absolute;
    bottom: -10px;
    left: 50%;
    -webkit-transform: translateX(-50%);
    transform: translateX(-50%);
    width: 0;
    height: 4px;
    border-style: solid;
    border-width: 8px 6px 4px;
    border-color: #cacaca transparent transparent;
    // background: #f5f5f7;
  }
}

.add-node-popover-item {
  text-align: center;
  .item-wrapper {
    text-align: center;
    .icon-font {
      font-size: 36px;
      width: 50px;
      height: 50px;
      line-height: 50px;
      border-radius: 50%;
      border: 1px solid #ccc;
      color: #fa9602;
    }
  }
  &:hover {
    .icon-font {
      color: #fff;
      background-color: #3296fa;
    }
  }
}
</style>
