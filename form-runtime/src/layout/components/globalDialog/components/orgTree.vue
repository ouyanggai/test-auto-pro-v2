<template>
  <el-dialog :visible="visible"
    :title="dialogTitle" width="50%"
    top="100px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
    <div class="dialog-container" id="testTab">
      <el-input placeholder="请输入名称" v-model="filterText" clearable ref="searchInput"></el-input>
      <el-tree :show-checkbox="(fielSelectType == 'multiPerson' || fielSelectType == 'multiDep')" :node-key="propData.nodeKey || 'idWithParentId'"
      :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10" :filter-node-method="filterNode" ref="companyTree"
      :check-strictly="!!propData.checkStrictly" @check="handleCheck">
        <span slot-scope="{node,data}">
          <template v-if="fielSelectType == 'company'">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 5" @input="()=>{radioChange(node)}"><span></span></el-radio>
            <span>{{ data.name }}</span>
            <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
          </template>

          <template v-if="fielSelectType == 'department'">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 2" @input="()=>{radioChange(node)}"><span></span></el-radio>
            <span>{{ data.name }}</span>
            <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
          </template>

          <template v-if="fielSelectType == 'duty'">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 4" @input="()=>{radioChange(node)}"><span></span></el-radio>
            <span>{{ data.name }}</span>
            <!-- <span style="color:#ccc;margin-left: 10px;">{{data.roleName}}</span> -->
          </template>

          <template v-if="fielSelectType == 'onlyCompany'">
            <el-radio v-model="chooseHeaderRadio" :label="data.id" @input="()=>{radioChange(node)}"><span></span></el-radio>
            <span>{{ data.name }}</span>
            <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
          </template>

          <template v-if="fielSelectType == 'multiPerson' || fielSelectType == 'multiDep'">
            <span>{{ data.name }}</span>
            <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
          </template>
        </span>
      </el-tree>
    </div>

    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="confirmTaskHeader">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import store from '@/store';
import { localstorageGet } from '@/utils/auth';

export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    propData: {
      type: Object,
      default: _ => {}
    }
    // fielSelectType: {
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      fielSelectType: '',
      filterText: '',
      selectRadio: null,
      treeData: [],
      activeName: 'first',
      chooseHeaderRadio: '',
      defaultProps: {
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      splitFilterText: [],
      visibleCheckedPersonLength: 0
    };
  },
  computed: {
    dialogTitle() {
      var type = this.fielSelectType;
      var title = (type == 'company' || type == 'multiPerson') ? '选择人员' : (type == 'department' || type == 'multiDep') ? '选择部门' : type == 'onlyCompany' ? '选择公司' : '选择岗位';
      if (type == 'multiPerson' && this.visibleCheckedPersonLength > 0) {
        title += `（已选中${this.visibleCheckedPersonLength}人）`;
      }
      return title;
    }
  },
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.splitFilterText = val.trim().split(/\s+|[\s,，、\s]+/).filter(Boolean);
        this.$refs.companyTree.filter(val);
        this.handleCheck();
      }
    }
  },
  created() {
    this.fielSelectType = this.propData.type;
  },
  mounted() {
    this.init();
    setTimeout(() => {
      if (this.$refs.searchInput) {
        this.$refs.searchInput.focus();
      }
    }, 100);
  },
  methods: {
    handleCheck(data, checkList) {
      if (this.fielSelectType == 'multiPerson') {
        const checkNode = this.getVisibleCheckedNodes();
        var personList = checkNode.filter(x => x.type == 5);
        this.visibleCheckedPersonLength = personList.length;
      }
    },
    filterNode(value, data) {
      if (!value) return true;
      return this.splitFilterText.some(word => data.name.indexOf(word) !== -1);
      // return data.name.indexOf(value) !== -1;
    },
    radioChange(node) {
      console.log(node, 'node233');
      var func = (node) => {
        if (node?.data?.type == '1' || node?.data?.type == null) {
          return node?.data;
        } else if (node?.parent?.data?.type == '1') {
          return node?.parent?.data; // .name
        } else {
          return func(node?.parent);
        }
      };
      this.selectRadio = func(node);
      console.log(this.selectRadio, 'this.selectRadio');
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    tabClick() {

    },
    init() {
      this.getCompanyTree();
    },
    hasPermission(href) { // 除了html中可以用v-permission控制节点隐藏，代码中需要用到的权限判断用这个方法
      const btnPermissionList = store.getters.btnPermissionList;
      const flag = btnPermissionList.some(item => {
        return item.href == href;
      });
      return flag; // 有权限返回true
    },
    getCompanyTree() { // 获取公司部门架构数据
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            flag: (_ => {
              // return 5 查询当前公司
              if (this.fielSelectType == 'department' || this.fielSelectType == 'multiDep') return 2;
              if (this.fielSelectType == 'onlyCompany') return 7;
              if (this.fielSelectType == 'duty') return 4;
              if (this.fielSelectType == 'company') return 3;
              return 3;
              // return this.fielSelectType == 'duty' ? 4 : 3;
            })(),
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData = this.filterTree(res.data);
            this.$nextTick(_ => { this.reviewSelectValue(); });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    reviewSelectValue() {
      var selectValue = this.propData.selectValue;
      if ((this.fielSelectType == 'multiPerson' || this.fielSelectType == 'multiDep') && selectValue && selectValue.length) {
        this.$refs.companyTree.setCheckedNodes(selectValue);
        // var arr = selectValue.map(i => i.id);
        // console.log(arr, 'arr');
        // this.$refs.companyTree.setCheckedKeys(arr);
        this.$nextTick(_ => { this.handleCheck(); });
      }
    },
    filterTree(data, type) {
      var filterId = this.propData.filterId;
      if (!filterId) {
        this.setUniqueIdWithParentId(data);
        return data;
      }
      function fuc(tree) {
        return tree.filter((i) => {
          if (i.id == filterId) {
            return true;
          } else {
            if (i.childrenList && i.childrenList.length) {
              var l = fuc(i.childrenList);
              i.childrenList = l;
              return !!l.length;
            };
          }
        });
      }
      var result = fuc(data);
      this.setUniqueIdWithParentId(result);
      return result;
    },
    setUniqueIdWithParentId(data) {
      function fuc(tree) {
        return tree.forEach((i) => {
          i.idWithParentId = i.id + i.parentId;
          if (i.childrenList && i.childrenList.length) {
            fuc(i.childrenList);
          };
        });
      }
      fuc(data);
    },
    confirmTaskHeader() {
      if (this.fielSelectType == 'multiPerson') {
        // const checkNode = this.$refs.companyTree.getCheckedNodes();
        const checkNode = this.getVisibleCheckedNodes();
        var personList = checkNode.filter(x => x.type == 5);
        console.log(personList, 'personList');
        this.$emit('confirmed', personList);
      } else if (this.fielSelectType == 'multiDep') {
        const checkNode = this.$refs.companyTree.getCheckedNodes();
        var depList = checkNode.filter(x => x.type == 2);
        this.$emit('confirmed', depList);
      } else {
        const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
        this.$emit('confirmed', choseObj);
      }
      this.handleClose();

      // const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
      // const choseObj2 = this.getChidlren(this.chooseHeaderRadio, this.treeData2);
      // const choseObj3 = this.getChidlren(this.chooseHeaderRadio, this.treeData3);
      // if (choseObj || (choseObj2 || choseObj3)) {
      //   // this.$emit('selectHeader', choseObj || (choseObj2 || choseObj3), this.selectRadio);
      //   // this.$emit('getHeaderDutyList');
      //   this.$emit('confirmed', choseObj || (choseObj2 || choseObj3));
      // }
      // this.handleClose();
    },
    getChidlren(id, data) {
      var hasFound = false; // 表示是否有找到id值
      var result = null;
      function fn(data) {
        if (Array.isArray(data) && !hasFound) { // 判断是否是数组并且没有的情况下，
          data.forEach(item => {
            if (item.id === id) { // 数据循环每个子项，并且判断子项下边是否有id值
              result = item; // 返回的结果等于每一项
              hasFound = true; // 并且找到id值
            } else if (item.childrenList) {
              fn(item.childrenList); // 递归调用下边的子项
            }
          });
        }
      }
      fn(data);
      if (result) {
        result.mapProMainDeptId = data[0].id;
      }
      return result;
    },
    getVisibleCheckedNodes(refName = 'companyTree') {
      const checkedNodes = this.$refs[refName].getCheckedNodes();
      return checkedNodes.filter(data => {
        const node = this.$refs[refName].store.getNode(data);
        return node.visible;
      });
    }
  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  .dialog-container {
    // height: 600px;
    height: 48vh;
    overflow-y: auto;
  }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}
</style>
