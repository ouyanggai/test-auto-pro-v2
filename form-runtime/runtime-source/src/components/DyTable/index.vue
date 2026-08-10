<!--
 * @Descripttion:动态表格
 * @Author: Calvin
 * @Date: 2021-05-11 20:51:52
-->
<template>

  <div :class="{ 'dytable-view-container': true }">

    <!-- 自定义头部 搜索栏等 -->
    <div
      class="dytable-header mb-20"
      v-if="showHeader"
    >
      <slot name="header"></slot>
    </div>
    <div
      class="dytable-view-body"
      ref="dytableBody"
    >
      <el-table
        class="multipleTable-class"
        v-loading="loading"
        :max-height="maxTableHeight"
        :height="height"
        element-loading-spinner="aaa"
        :row-class-name="rowClassName"
        :span-method="spanMethod"
        ref="multipleTable"
        :data="list"
        :stripe="isStripe"
        :border="isShowBorder"
        :show-header="isShowHeader"
        :show-summary = "showSummary"
        :summary-method="summaryMethod"
        tooltip-effect="dark"
        highlight-current-row
        style="width: 100%"
        @select="handleSelect"
        @select-all="handleSelectAll"
        @row-click="rowClick"
        @selection-change="handleSelectionChange"
      >
        <el-table-column
          v-if="showSerial"
          label="序号"
          type="index"
          width="100"
        >
        </el-table-column>
        <el-table-column
          v-if="isDrag"
          width="55"
        >
          <i
            class="el-icon-rank sortable-move"
            style="font-size: 18px;cursor: pointer;"
          ></i>
        </el-table-column>
        <el-table-column
          v-if="showCheckBox"
          :selectable="selectable"
          type="selection"
          width="55"
        >
        </el-table-column>
        <el-table-column
          v-if="showRadio"
          width="55"
        >
          <template slot-scope="scope">
            <el-radio v-model="currentRowId" :label="scope.row.id"><i></i></el-radio>
          </template>
        </el-table-column>
        <template v-for="(value, key, index) in keys">
          <!-- :fixed="firstColumnFixed && index == 0" -->
          <el-table-column
            :fixed="value.ifFixed"
            :label="(typeof value === 'string') ? value : value.label"
            :sortable="value.sortable != undefined ? value.sortable : sortable"
            :prop="key"
            :class-name="value.className"
            :key="'col' + key"
            :width="value.width || ''"
            :show-overflow-tooltip="value.showTooltip"
            :align="value.align?value.align:'left'"
            :min-width="value.minWidth || ''"
            v-if="!value.noShowColumn"
          >
            <template slot-scope="scope">
              <dyitem
                :style="value.customStyle ? value.customStyle(scope) : ''"
                v-if="value.handle"
                :render-item="(createElement) => value.handle(scope, createElement)"
              />
              <!-- 需要文字换行和tooltip提示 -->
              <template v-if="value.toolTipContent">
                <el-tooltip placement="top">
                  <div
                    v-html="value.toolTipContent(scope)"
                    slot="content"
                  ></div>
                  <div v-html="value.toolTipContent(scope)"></div>
                </el-tooltip>
              </template>
              <div v-if="(!value.handle && !value.handleCustomToolTip)">{{ scope.row[key] }}</div>
            </template>
          </el-table-column>
        </template>
        <!-- 操作栏 -->
        <el-table-column
          v-if="actions.length"
          label="操作"
          :class="'op'"
          :width="actions[0].width || ''"
          :fixed="actions[0].actionFixed"
        >
          <template slot-scope="scope">
            <div class="action-list">
              <template v-for="(action, index) in actions">
                <template v-if="!action.noShowActionButton"> 
                  <template v-if="!action.handle">
                    <el-button
                      v-if="action.permission"
                      :key="'action' + index"
                      @click="e => { e.stopPropagation(); action.action(scope.row, e); }"
                      type="text"
                      size="small"
                      v-permission="action.permission"
                      style="text-align: left"
                    >{{ action.label }}</el-button>
                    <el-button
                      v-else
                      :key="'action' + index"
                      @click="e => { e.stopPropagation(); action.action(scope.row, e); }"
                      type="text"
                      size="small"
                      style="text-align: left"
                    >{{ action.label }}</el-button>
                  </template>
                  <template v-else>
                    <dyitem
                      v-if="action.permission"
                      v-permission="action.permission"
                      :key="'action' + index"
                      :render-item="(createElement) => actionHandle(action, scope, createElement)"
                    />
                    <dyitem
                      v-else
                      :key="'action' + index"
                      :render-item="(createElement) => actionHandle(action, scope, createElement)"
                    />
                  </template>
                </template>
              </template>
            </div>
          </template>
        </el-table-column>
        <!-- 无数据展示内容 -->
        <template slot="empty">
          <div
            class="dytable-view-empty"
            v-if="list.length <= 0 && !loading"
          >
            <p>{{ emptyText }}</p>
          </div>
        </template>
      </el-table>

    </div>
    <!-- 分页器 -->
    <div
      class="dytable-view-paging"
      v-show="list.length > 0 && showPagination"
    >
      <!-- :pager-count="p" -->
      <el-pagination
        v-if="isPagination"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        :page-size="pagination.size"
        background
        :current-page="pagination.pages"
        :layout="layoutPage"
        @size-change="handlePageSize"
        @current-change="handlePageCurrent"
        style="margin-top: 20px;text-align: right"
      >
      </el-pagination>
    </div>
  </div>
</template>
<script>
import Dyitem from './dyitem';
import { handlePageSize, handlePageCurrent } from './pageAction';

export default {
  components: {
    Dyitem
  },

  props: {
    layoutPage: {
      type: String,
      default: 'total, sizes, prev, pager, next'
    },
    maxTableHeight: {
      type: [String, Number],
      default: '1000'
    },

    height : {
      type: [String,Number],
      default: null
    },

    loading: {
      type: Boolean,
      default: false
    },
    /**
    * 是否固定列
    * @param {Boolean}
    * @example
    */
    ifFixed: {
      type: [Boolean, String],
      default: false
    },
    /**
    * table是否展示序号
    * @param {Boolean}
    * @example
    */
    showSerial: {
      type: Boolean,
      default: false
    },
    /**
    * table是否添加多选框列
    * @param {Boolean}
    * @example
    */
    showCheckBox: {
      type: Boolean,
      default: false
    },
    showRadio:{
      type:Boolean,
      default:false
    },
    /**
    * table是否可以行拖拽
    * @param {Boolean}
    * @example
    */
    isDrag: {
      type: Boolean,
      default: false
    },
    /**
    * 是否展示头部 slot组件
    * @param {Boolean}
    * @example 可自行加入搜索框选项
    */
    showHeader: {
      type: Boolean,
      default: false
    },
    /**
    * table 数据
    * @param {Array}
    * @example 可自行加入搜索框选项
    */
    list: Array,
    /**
    * 是否添加分页器
    * @param {Boolean}
    * @example
    */

    isPagination: {
      type: Boolean,
      default: false
    },
    pagination: Object,
    // p: {
    //   type: [String, Number],
    //   default: 3
    // },
    /**
     * actions:自定义操作
     */
    actions: {
      type: Array,
      default: () => []
    },
    /**
    * 是否展示表格边框
    * @param {Boolean}
    */
    isShowBorder: {
      type: Boolean,
      default: false
    },
    /**
    * 是否展示表格头
    * @param {Boolean}
    */
    isShowHeader: {
      type: Boolean,
      default: true
    },
    /**
    * 是否固定操作列
    * @param {Boolean，String}
    */
    actionFixed: {
      type: [Boolean, String],
      default: false
    },
    /**
    * 是否固定第一列
    * @param {Boolean，String}
    */
    firstColumnFixed: {
      type: [Boolean, String],
      default: false
    },
    /**
    * 是否带斑马纹表格
    * @param {Boolean}
    */
    isStripe: {
      type: Boolean,
      default: false
    },
    /**
    * 是否展示分页器
    * @param {Boolean}
    */
    showPagination: {
      type: Boolean,
      default: true
    },
    /**
    * 是否显示排序
    * @param {Boolean}
    */
    sortable: {
      type: Boolean,
      default: false
    },
    /**
  * 排序方法
  * @param {Function}
  */
    changeSort: {
      type: Function,
      default: null
    },
    /**
  * 排序方法
  * @param {Function}
  */
    spanMethod: {
      type: Function,
      default: null
    },
    /**
    * table表头
    * @param {Object}
    * @example
    * keys个格式有两种
     * 1. 普通形式
     * {
     *   name: '名称',
     *   age: '年龄'
     * }
     * 2. 可以自定义的形式
     * {
     *   name: '名称',
     *   age: '年龄'
     *   dance: {
     *     label: '跳舞',
     *     handle: function(scope, createElement) {
     *        return createElement('h1', 'hello, world');
     *     }
     *   }
     * }
     *  @param {Object} label 表头名称
     *  @param {Boolean} sortable 是否展示排序
     *  @param {String} className 自定义class
     *  @param {String} width 自定义宽度
     *  @param {String} align 自定义表头对齐方式
     *  @param {Function} handle 自定义rander函数
    */
    keys: {
      type: Object,
      default: () => ({})
    },
    /**
    * 数据请求方法
    * @param {Function}
    */
    fetchData: {
      type: Function,
      default: null
    },
    /**
    * 无数据时展示文案
    * @param {String}
    */
    emptyText: {
      type: String,
      default: '暂无数据'
    },
    defaultFirstRowChecked: {
      type: Boolean,
      default: false
    },
    showSummary:{
      type:Boolean,
      default:false
    },
    summaryMethod:{
      type:Function,
      default:null
    },
    rowClassName: Function // 行的 className 的回调方法
  },
  watch: {
    list(val, oldval) {
      if (this.defaultFirstRowChecked) {
        this.$refs.multipleTable.setCurrentRow(val[0]);
      }
    }
    // isDrag(val) {
    //   console.log(val);
    //   this.showDragButton = true;
    // }
  },
  data() {
    return {
      // 当前行是否被选中 默认未选中
      isCurrentRowChecked: false,
      // 是否能够关联构建（如果本身意见关联）--绑定按钮的disabled属性
      // disabled = !isAbleToConnect
      isAbleToConnect: false,
      // 当前选中列数据
      tableActiveRow: null,
      datas: [],
      selectDatas: [],
      currentRowId:''
      // showDragButton: false

    };
  },
  created() {
    this.fetchData();
  },

  mounted() {
    // this.$nextTick(function(){
    //   console.log(this.list)
    // });
    // console.log(this.defaultFirstRowChecked)
    // this.$refs.multipleTable.setCurrentRow(this.list[0])
  },
  updated() {
  },
  methods: {
    doLayout(){
      if (this.$refs.multipleTable && this.$refs.multipleTable.doLayout){
        this.$refs.multipleTable.doLayout();
      }
    },
    selectable(row, index){
      if(row.selectable === undefined)return true
      else{
        return row.selectable
      }
    },
    handleSizeChange(size) {
      // 切换每页显示的数量
      this.pagination.limit = size;
      this.fetchData();
    },
    handleIndexChange(current) {
      // 切换页码
      this.pagination.page = current;
      this.fetchData();
    },

    handleSelectionChange(val) {
      this.selectDatas = val;
      this.$emit('selectDataEvent', this.selectDatas);
    },
    handleClearSelection(val) {
      this.$refs.multipleTable.clearSelection();
    },
    handleToggleRowSelection(row) {
      this.$refs.multipleTable.toggleRowSelection(row,true);
    },

    handleSelect(select, row) {
      this.$emit('handleSelect', row);
    },
    handleSelectAll(selection){
      this.$emit('handleSelectAll', selection);
    },
    handleClick(row) {
      // console.log(row);
    },
    handlePageSize,

    handlePageCurrent,
    rowClick(row, event, column) {
      // 判断两个对象是否相等
      if (JSON.stringify(this.tableActiveRow) === JSON.stringify(row)) {
        this.isCurrentRowChecked = !this.isCurrentRowChecked;
      } else {
        this.tableActiveRow = row;
        this.isCurrentRowChecked = true;
      }
      if (this.isCurrentRowChecked) {
        this.isAbleToConnect = true;
      } else {
        this.isAbleToConnect = false;
      }
      this.$emit('tableRowClick', { row, isAbleToConnect: this.isAbleToConnect });
      this.$emit('rowClick', { row, event, column });
      if(this.showRadio){
        this.currentRowId = row.id
      }
    },

    actionHandle(action, scope, createElement) {
      return action.handle(scope, createElement, this);
    }
  }
};
</script>

<style lang="scss">
// .multipleTable-class{

// }
.dytable-view-container {
  position: relative;
  min-height: 200px;
  // margin-top: 20px;
  padding: 20px;
  box-sizing: border-box;

  .el-table--mini td,
  .el-table--mini th {
    padding: 8px 0;
  }

  .dytable-view-body {
    flex-grow: 2;
    overflow-y: hidden;
  }

  .el-table .cell div,
  .el-table .cell span {
    text-overflow: ellipsis !important;
    overflow: hidden !important;
    white-space: nowrap !important;
  }

  .el-table--border::after,
  .el-table--group::after,
  .el-table::before {
    background-color: transparent;
  }

  .dytable-view-paging {
    margin-top: 20px;
  }

  .el-loading-mask {
    background-color: rgba(255, 255, 255, 1) !important;
  }

  .dytable-view-empty {
    img {
      margin-top: 50px;
      width: 100px;
    }

    p {
      line-height: normal;
    }
  }

  .el-table .danger-row {
    background: #fff1f0;
  }

  .el-table .success-row {
    background: #f6ffed;
  }

  .el-table .warning-row {
    background: #fffbe6;
  }

  // 任务完成背景色
  .el-table .bg-nostart {
    // 未开始
    background: #ccc;
  }

  .el-table .bg-running {
    // 进行中
    background: #1890ff;
  }

  .el-table .bg-overtime {
    // 已超时
    background: #facc14;
  }

  .el-table .bg-willchecked {
    // 待审核
    background: #223273;
  }

  .el-table .bg-finished {
    // 已完成
    background: #2fc25b;
  }

  // 已终止
  .el-table .bg-withdraw {
    background: #dc143c;
  }

  // 拖拽
  .dragClass {
    background: rgba(160, 207, 255, 0.5) !important;
  }

  // 停靠
  .ghostClass {
    background: rgba(160, 207, 255, 0.5) !important;
  }

  // 选择
  .chosenClass:hover > td {
    background: rgba(140, 197, 255, 0.5) !important;
  }
  .el-table__cell .el-link{
    width: 100%;
    justify-content:left;
  }
}
</style>
