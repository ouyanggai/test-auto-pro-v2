<template>
  <!-- :multiple="isCheck" -->
  <!-- collapse-tags -->
  <el-select v-model="valueTitle" :clearable="clearable" @clear="clearHandle" @visible-change="visibleChange" filterable
    :placeholder="'请选择' + `${labelName}`" :disabled="disabled" :filter-method="search" :style="{ width: inputWidth }"
    ref="selectInput" @focus="handleTreeList">
    <el-option :value="valueTitle" :label="valueTitle" @click.stop.prevent="">
      <div style="padding: 2px 10px" @click.stop.prevent v-if="isShowFilter">
        <el-input v-model="filterText" :placeholder="searchPlaceholder"></el-input>
      </div>
      <el-tree accordion style="max-height: 300px;overflow: auto;" ref="selectTreeCon" :data="options" :props="props"
        :show-checkbox="isCheck" :node-key="props.value" :expand-on-click-node="false" :filter-node-method="filterNode"
        :default-expanded-keys="defaultExpandedKey" @node-click="handleNodeClick" @check-change="checkChange">
      </el-tree>
      <!-- :check-strictly="true" 父子没有关联 :node-key="props.value" -->
    </el-option>
  </el-select>
</template>

<script>
export default {
  name: 'ElTreeSelect',
  // props: ['options', 'props', 'value'],
  props: {
    options: {
      type: Array, // 必须是树形结构的对象数组
      default: () => {
        return [];
      }
    },
    props: {
      type: Object,
      default: () => {
        return {
          value: 'id', // ID字段名
          label: 'title', // 显示名称
          children: 'children' // 子级字段名
        };
      }
    },
    labelName: { // 提示文本
      type: String,
      default: '请选择'
    },
    value: {
      type: [String, Number, Array],
      default: ''
    },
    clearable: {
      type: Boolean,
      default: true

    },
    isLevel: {
      // 是否选择最后一级
      type: Boolean,
      default: true
    },
    disabled: {
      // 是否禁用
      type: Boolean,
      default: () => {
        return false;
      }
    },
    isCheck: { // 下拉树显示勾选框,默认false (或者是默认为true,点节点只能选一个,点复选框则可以选多个)
      type: Boolean,
      default: false
    },
    inputWidth: { // 下拉树显示勾选框,默认false (或者是默认为true,点节点只能选一个,点复选框则可以选多个)
      type: String,
      default: '100%'
    },
    isShowFilter: { // 暂时只考虑到非懒加载的情况
      // 是否可以搜索true
      type: Boolean,
      default: false
    },
    searchPlaceholder: { // 未选择时的提示语
      type: String,
      default: '请输入搜索数据'
    },
    isShowChildTitle: { // 是否显示子级名称
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      valueId: this.value, // 初始值
      valueTitle: '',
      defaultExpandedKey: [],
      filterText: '',
      arr: []
    };
  },
  mounted() {
    this.init();
  },
  watch: {
    filterText(val) {
      this.$refs.selectTreeCon.filter(val);
    },
    value(val, oldVal) {
      console.log(val, oldVal)
      if (val !== oldVal) {
        this.valueId = val;
        this.setValue(this.valueId);
        // console.log(this.valueId)
        if (!val) {
          this.clearHandle();
        }
      }
    },
    options(val) {
      if (val.length > 0) {
        this.init();
      }
    }
  },
  methods: {
    visibleChange(val) {
      if (val) {
        this.filterText = null;
      }
    },
    // 设置值
    setValue(val) {
      // console.log(this.options)
      console.log(this.$refs.selectTreeCon.getNode(val),val,'55555')
      if (val) {
        this.valueTitle =''
        if(Array.isArray(val)){
          val.forEach((item,index)=>{
            // this.$refs.selectTreeCon.setCurrentKey(item); // 设置默认选中
            this.valueTitle += (index!=0?',':'')+(this.$refs.selectTreeCon.getNode(item).data[
            'fullName'
            ]||this.$refs.selectTreeCon.getNode(item).data[
           this.props.label
            ]) 
          })
          // this.$refs.selectTreeCon.setCurrentKey(val); // 设置默认选中
          this.defaultExpandedKey = val; // 设置默认展开
        }else{
          this.valueTitle = this.$refs.selectTreeCon.getNode(val).data[
            this.props.label
          ]; // 初始化显示
          this.$refs.selectTreeCon.setCurrentKey(val); // 设置默认选中
          this.defaultExpandedKey = [val]; // 设置默认展开
        }
      } else {
        this.valueTitle = '';
      }
    },
    // 初始化值
    init(/* rest */) {
      // console.log(this.options)
      if (this.valueId) {
        // console.log('tree???',this.$refs.selectTreeCon.getNode(this.valueId))
        this.valueTitle = (this.$refs.selectTreeCon.getNode(this.valueId) || { data: {} }).data[
          this.props.label
        ]; // 初始化显示
        // console.log('tree', this.valueTitle)
        this.$refs.selectTreeCon.setCurrentKey(this.valueId); // 设置默认选中
        this.defaultExpandedKey = [this.valueId]; // 设置默认展开
      } else {
        this.valueTitle = '';
      }
    },
    // 筛选树
    filterNode(value, data) {
      if (!value) {
        return true;
      } else {
        return data[this.props.label].indexOf(value) !== -1;
      }
    },
    // 切换选项
    handleNodeClick(node) {
      if (!this.isCheck) { // 在不使用勾选时生效
        if (this.isLevel) {
          if (
            !node[this.props.children] ||
            node[this.props.children].length === 0
          ) {
            this.valueTitle = node[this.props.label];
            this.valueId = node[this.props.value];
            this.$emit('getValue', node);
            this.$emit('update:value', this.valueId);
            this.$refs.selectInput.blur();
          }
        } else {
          this.valueTitle = node[this.props.label];
          this.valueId = node[this.props.value];
          this.$emit('getValue', node);
          this.$emit('update:value', this.valueId);
          this.$refs.selectInput.blur();
        }
      }
    },
    // 筛选树
    search(val) {
      this.filterText = val || null;
    },
    // 清除选中
    clearHandle() {
      this.search();
      this.$emit('clear');
      this.$refs.selectTreeCon.setCurrentKey(null); // 设置默认选中
      if (this.isCheck) {
        this.$refs.selectTreeCon.setCheckedKeys([]); // 如果有勾选框，则清理掉被勾选中的数据
        this.valueId = [];
      } else {
        this.valueId = null;
      }
      this.$emit('getValue', {});
      this.$emit('update:value', '');
      this.valueTitle = '';
      this.arr = [];
      // this.defaultExpandedKey = []
    },
    checkChange(node, currentBol) {
      // console.log('是否显示勾选框----------')
      // console.log(this.isCheck)
      // console.log('当前节点----------------')
      // console.log(node)
      // console.log('当前节点的勾选状态')
      console.log(currentBol,'currentBol+++')
      if (this.isCheck) {
        if (this.arr.length === 0) { // 当点选和勾选切换时清空内容
          this.valueTitle = '';
        }
        if (currentBol) {
          this.valueTitle += node.name + ', ';
          this.getChildrenId(node);
        } else {
          this.valueTitle = this.valueTitle.replace(node.name + ', ', '');
          console.log(node,'55555555554444444')
          this.removeChildrenId(node);
        }
        // var v = new Set(this.arr) // 数组去重
        // this.valueId = JSON.parse(JSON.stringify([...v]))
        // node[this.props.value] = this.valueId.toString() // 转成普通字符串

        // node[this.props.value] = JSON.stringify(this.valueId) // 转成json字符串
        // console.log(this.$refs.selectTreeCon.getCheckedKeys())
        this.$emit('getValue', node);
        console.log(this.$refs.selectTreeCon.getCheckedKeys(),'this.$refs.selectTreeCon.getCheckedKeys()')
        this.$emit('update:value', this.$refs.selectTreeCon.getCheckedKeys());
        this.$emit('setValue', this.$refs.selectTreeCon.getCheckedNodes());
        this.$refs.selectTreeCon.getCheckedNodes().map((item,index)=>{
          this.valueTitle += (index!=0?',':'')+item['fullName']||item[this.props.label] 
        })
        // this.$emit('update:value', this.valueId)
        // this.$refs.selectInput.blur() // 收回下拉框
      }
    },
    getChildrenId(node) { // 递归求出所有的末级分类
      if (node[this.props.children] !== undefined && node[this.props.children].length !== 0) {
        node[this.props.children].forEach(item => {
          if (item[this.props.children] === undefined) { // item.children.length === 0
            this.arr.push(item.bizid);
            console.log(item,'item.name')
            this.valueTitle += item.name + ', '
          } else {
            this.getChildrenId(item);
          }
        });
      } else {
        this.arr.push(node.bizid); // 放到数组中
        // this.valueTitle += node.name + ', '
        console.log(node,'node.name')
      }
    },
    removeChildrenId(node) { // 递归移除没选中的末级分类
      if (node[this.props.children] !== undefined && node[this.props.children].length !== 0) {
        node[this.props.children].forEach(item => {
          if (item[this.props.children] === undefined) { // 递归移除所有子
            const idx = this.arr.indexOf(item.bizid);
            if (idx >= 0) {
              this.arr.splice(idx, 1);
              this.valueTitle = this.valueTitle.replace(item.name + ', ', '');
            }
          } else {
            this.removeChildrenId(item);
          }
        });
      } else {
        const idx = this.arr.indexOf(node.bizid); // 移除自己
        if (idx >= 0) {
          this.arr.splice(idx, 1);
          this.valueTitle = this.valueTitle.replace(node.name + ', ', '');
        }
      }
    },
    handleTreeList() {
      if (this.options.length === 0) {
        this.$emit('treeData');
      }
    }
  }
};
</script>
<style scoped>
.el-scrollbar .el-scrollbar__view .el-select-dropdown__item {
  height: auto;
  padding: 0;
}

.el-select-dropdown__item.selected {
  font-weight: normal;
}

ul li>>>.el-tree .el-tree-node__content {
  height: 26px;
}

.el-tree-node__label {
  font-weight: normal;
}

.el-tree>>>.is-current .el-tree-node__label {
  color: #409eff;
  font-weight: 700;
}

.el-tree>>>.is-current .el-tree-node__children .el-tree-node__label {
  color: #606266;
  font-weight: normal;
}
</style>
