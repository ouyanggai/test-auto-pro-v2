<!--
 * @Descripttion:集团人员和部门选择弹窗-用于表单里面打开的弹窗。（挪到src/component文件夹下，支持共用）（这里暂时不用）
 * @Author: zhengzetao
 * @Date: 2022-3-30
-->
<template>
  <el-dialog :visible="visible"
    :title="fielSelectType == 'company' ? '选择人员' : fielSelectType == 'department' ? '选择部门' : '选择岗位'" width="50%"
    top="100px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog"
    @close='handleClose'>
    <el-tabs v-model='activeName' @tab-click="tabClick" type="card" class="dialog-container" id="testTab">
      <el-input placeholder="请输入人员名称" v-model="filterText" clearable>
      </el-input>
      <el-tab-pane name="first" label="公司架构">
        <el-tree :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10" :filter-node-method="filterNode" ref="companyTree">
          <span slot-scope="{node,data}">
            <template v-if="fielSelectType == 'company'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 5" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
            </template>

            <template v-if="fielSelectType == 'department'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 2 || data.type == 1 && node.level != 1" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
            </template>

            <template v-if="fielSelectType == 'duty'">
              <el-radio v-model="chooseHeaderRadio" :label="data.id" v-if="data.type == 4" @input="()=>{radioChange(node)}"><span></span></el-radio>
              <span>{{ data.name }}</span>
              <!-- <span style="color:#ccc;margin-left: 10px;">{{data.roleName}}</span> -->
            </template>

          </span>

        </el-tree>
      </el-tab-pane>
    </el-tabs>

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
    fielSelectType: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
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
      }
    };
  },
  computed: {},
  watch: {
    filterText(val) {
      if (this.$refs.companyTree) {
        this.$refs.companyTree.filter(val);
      }
      // if (this.$refs.projectTree) {
      //   this.$refs.projectTree.filter(val);
      // }
      // if (this.$refs.associateTree) {
      //   this.$refs.associateTree.filter(val);
      // }
    }
  },
  created() { },
  mounted() {
    this.init();
  },
  methods: {
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    radioChange(node) {
      var func = (node) => {
        if (node.parent.data.type == '1') {
          return node.parent.data.name;
        } else {
          return func(node.parent);
        }
      };
      this.selectRadio = func(node);
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
            flag: this.fielSelectType == 'duty' ? 4 : 3,
            id: localstorageGet('companyId') // 公司id
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    confirmTaskHeader() {
      const choseObj = this.getChidlren(this.chooseHeaderRadio, this.treeData);
      const choseObj2 = this.getChidlren(this.chooseHeaderRadio, this.treeData2);
      const choseObj3 = this.getChidlren(this.chooseHeaderRadio, this.treeData3);
      // console.log('choseObj', choseObj);
      // console.log('choseObj2', choseObj2);
      // console.log(choseObj3);
      if (choseObj || (choseObj2 || choseObj3)) {
        this.$emit('selectHeader', choseObj || (choseObj2 || choseObj3), this.selectRadio);
        // this.$emit('getHeaderDutyList');
      }
      this.handleClose();
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
