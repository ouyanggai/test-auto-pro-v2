<template>
  <div class='app'>
    <div class="searchTop">
      考核时间：
      <el-date-picker
        v-model="targetTime"
        type="month"
        ref="datepicker"
        placeholder="选择日期"
        format="yyyy年M月"
        value-format="yyyyM"
        clearable
        style="width:200px;margin-right:30px;"
        ></el-date-picker>
       <!-- 公司/部门：
       <el-cascader
        v-model="targetScope"
        ref="elcascader"
        :props="selectProps"
        :options="treeData[0].childrenList"
        :expandTrigger="true"
        style="width: 300px"
        popper-class="monthPerfSum20240514"
        @change="changeScope"
      >
        <span slot-scope="{ node, data }" :title="data.name">
          <span v-if="data.companyType=='PLATFORM_COMPANY'" :companyType="data.companyType">{{data.name}}</span>
          <span v-else  :companyType="data.companyType">{{data.name}}</span>
        </span>
      </el-cascader> -->
      <el-button type="primary" @click="getList">查询</el-button>
      <el-button type="primary" icon="el-icon-setting" style="float:right;margin-right: 40px;" v-if="true" @click="handleSetSort">设置</el-button>
    </div>
    <dy-table
      :fetchData="getList"
      :actions="actions"
      :keys="colKey"
      :list="kpiSumList"
      :isPagination="true"
      :pagination="pagination"
      :max-height="height"
    ></dy-table>
    <el-dialog custom-class="monthlyPerfBlocSum" v-if="visible" title="" :fullscreen="true" :visible="visible" :append-to-body="true" :close-on-click-modal="false"  center
      @close='handleClose'>
      <div ref="print" class="print">
        <monthPerfSum :actionType="'preview'" :bizId="bizId"></monthPerfSum>
      </div>
    </el-dialog>
    <el-dialog v-if="sortVisible" title="调整顺序" :visible="sortVisible" :append-to-body="true" :close-on-click-modal="false"  center
      @close='handleSortClose' width="800px" v-dialogDraw top="10px">
        <div>
          <el-button type="primary" @click="addSortPersonVisible = true">添加人员</el-button>
          <el-button @click="sortDelete">删除人员</el-button>
          <dy-table maxTableHeight="700" :keys="colKey2" :fetchData="fetchSortData" :list="sortTableData" style="padding:0px;"
          :isDrag="true" ref="sortDytable" class="myDyTable" @rowClick="rowClick">
        </dy-table>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleSortClose">取 消</el-button>
        <el-button type="primary" @click="_=>{clickSetSort(false)}">保 存</el-button>
      </div>
    </el-dialog>
    <el-dialog v-if="addSortPersonVisible" title="选择人员" :visible="addSortPersonVisible" :append-to-body="true" :close-on-click-modal="false"
      @close='addSortPersonClose' width="50%" v-dialogDraw top="5px">
        <div style="max-height:730px;overflow-y:scroll;">
          <el-input placeholder="请输入人员名称" v-model="filterText" clearable></el-input>
          <el-tree :data="treeData" :props="defaultProps" :default-expand-all="true" :indent="10" node-key="id"
            :filter-node-method="filterNode" ref="companyTree">
            <span slot-scope="{node,data}" style="width:100%;">
              <el-radio v-model="chooseRadio" :label="data.id" :disabled="checkDisabledRadio(data.id)" v-if="data.type == 5"  style="width:100%;">
                <span>{{data.name}}</span>
                <span style="margin-left:10px;zoom:0.9">{{data.roleName}}</span>
              </el-radio>
              <span v-else>
                <span>{{data.name}}</span>
                <span style="margin-left:10px;zoom:0.9">{{data.roleName}}</span>
              </span>
            </span>
          </el-tree>
        </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="addSortPersonClose">取 消</el-button>
        <el-button type="primary" @click="addSortPersonConfirm">确 定</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import monthPerfSum from './monthPerfSum.vue';
import Sortable from 'sortablejs';
import { Print as $print } from '@/utils/print.js';
import {
  localstorageGet, localstorageSet
} from '@/utils/auth';
import Api from '@/api';
export default {
  name: '',
  components: { DyTable, monthPerfSum },
  data () {
    return {
      selectRow: null,
      treeData: [],
      chooseRadio: '',
      filterText: '',
      addSortPersonVisible: false,
      defaultProps: {
        // children: 'children',
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      dragObj: null,
      sortTableData: [],
      kpiSumList: [],
      targetTime: '',
      targetScope: '',
      selectProps: {
        value: 'id',
        label: 'name',
        children: 'childrenList',
        checkStrictly: true,
        emitPath: false
      },
      visible: false,
      sortVisible: false,
      height: '650',
      val: '',
      activeRow: null,
      bizId: null,
      pagination: {
        total: 0,
        pages: 1,
        current: 1,
        size: 10
      },
      colKey2: {
        sort: {
          label: '序号',
          width: '100',
          handle: (scope, createElement) => {
            return <span>{scope.row.sortIndex + 1}</span>;
          }
        },
        name: '姓名',
        department: {
          label: '部门',
          handle: (scope, createElement) => {
            return <span>{scope.row.department && scope.row.department.departmentName}</span>;
          }
        },
        duty: {
          label: '岗位',
          handle: (scope, createElement) => {
            return <span>{scope.row.duty && scope.row.duty.dutyName}</span>;
          }
        }
      },
      colKey: {
        targetTime: {
          label: '考核时间',
          handle: (scope, createElement) => {
            const targetTime = String(scope.row.targetTime);
            let targetTitle = '';
            if (targetTime) {
              const year = targetTime.substr(0, 4);
              const month = targetTime.substr(4);
              targetTitle = `${year}年${month}月`;
            }
            return <span>{targetTitle}</span>;
          }
        },
        company: {
          label: '公司/部门',
          handle: (scope, createElement) => {
            return <span>{scope.row.company?.name} {scope.row.user?.department}</span>;
          }
        },
        initiator: {
          label: '发起人',
          handle: (scope, createElement) => {
            return <span>{scope.row.user?.name}</span>;
          }
        },
        createDate: '发起时间',
        examineStatus: {
          label: '状态',
          handle: (scope, createElement) => {
            var statusObj = { not_submitted: '未提交', under_review: '审核中', finish: '已完成', rejected: '已驳回', pass: '已通过' };
            return createElement('span', statusObj[scope.row.examineStatus]);
          }
        }
      },
      actions: [
        {
          label: '详情',
          // permission: '/approveManage/backlog/check111122',
          width: 250,
          action: (row) => {
            this.activeRow = row;
            this.bizId = row.id;
            this.viewDetail(row);
          }
        }
      ]
    };
  },
  provide() {
    return {
      prevStepHandle: this.handleClose,
      activeRow: this.activeRow
    };
  },
  mounted() {
    window.abb = this;
  },
  beforeDestroy() {
    if (this.dragObj) {
      this.dragObj.destroy();
      this.dragObj = null;
    }
  },
  methods: {
    checkDisabledRadio(val) {
      return this.sortTableData.some(i => {
        return i.id == val;
      });
    },
    addSortPersonConfirm() {
      if (!this.chooseRadio) {
        this.$message.error('请选择添加人员');
        return;
      }
      this.sortTableData.push({ id: this.chooseRadio, duty: {}, department: {}});
      this.clickSetSort(true, _ => {
        this.fetchSortData(() => {
          const wrapper = document.querySelector('.myDyTable .dytable-view-body .el-table__body-wrapper.is-scrolling-none');
          wrapper.style.scrollBehavior = 'smooth';
          wrapper.scrollTop = 5000;
          wrapper.style.scrollBehavior = 'unset';
        });
      });
      this.addSortPersonVisible = false;
      this.chooseRadio = '';
      this.selectRow = null;
    },
    rowClick({ row, event, column }) {
      this.selectRow = row;
      console.log({ row, event, column });
    },
    async getCompanyTree() { // 获取公司部门架构数据
      var { data } = await this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            // flag: this.$store.state.user.projectRelationType == 'private_project' ? 5 : 3,
            flag: 5,
            id: localstorageGet('companyId') // 公司id
            // buildMainDept: true
            // sonId: this.proMainDeptId // 项目部id
          }
        },
        _ => _
      );
      var isCompany = this.$route.path.includes('monthlyPerfCompanySum');
      if (isCompany) {
        this.treeData = data;
        return;
      }
      var id = localstorageGet('userDepartmentId');
      var treearr = [];
      var func = (j) => {
        var find = j.filter(i => {
          if (i.id == id) {
            return true;
          } else if (i.childrenList && i.childrenList.length && i.childrenList[0].type != '5') {
            return func(i.childrenList);
          };
        });
        console.log(find, 'find');
        if (find.length > 0) {
          treearr.unshift(find);
          return true;
        }
      };
      func(data);
      treearr.forEach((i, idx) => {
        if (treearr[idx + 1]) {
          treearr[idx][0].childrenList = treearr[idx + 1];
        }
      });
      this.treeData = data;
      // console.log(treearr, 'treearr');
      // console.log(data, 'data22');
    },
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    sortDelete(row) {
      if (!this.selectRow) {
        this.$message.error('请选择删除人员');
        return;
      }
      this.$confirm('确定删除选中人员?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          var index = this.sortTableData.findIndex(i => i.id == this.selectRow.id);
          if (index == -1) return;
          this.sortTableData.splice(index, 1);
          this.selectRow = null;
          this.clickSetSort(true, _ => {
            this.fetchSortData();
          });
        })
        .catch(() => { });
    },
    sortablePersonInfoMethods(dom, dataArray) {
      // 获取表格row的父节点
      const ele = this.$refs[dom].$el.querySelector('.el-table__body > tbody');
      // 创建拖拽实例
      const that = this;
      const wrapper = document.querySelector('.myDyTable .dytable-view-body .el-table__body-wrapper.is-scrolling-none');
      this.dragObj = Sortable.create(ele, {
        animation: 150, // 动画
        handle: '.sortable-move', // 指定拖拽目标，点击此目标才可拖拽元素(此例中设置操作按钮拖拽)
        dragClass: 'dragClass', // 设置拖拽样式类名
        ghostClass: 'ghostClass', // 设置拖拽停靠样式类名
        chosenClass: 'chosenClass', // 设置选中样式类名
        onUpdate: function (event) {
          var newIndex = event.newIndex;
          var oldIndex = event.oldIndex;
          var $newItem = ele.children[newIndex];
          var $oldItem = ele.children[oldIndex];
          // 先还原sotable拖拽
          ele.removeChild($newItem);
          if (newIndex > oldIndex) {
            ele.insertBefore($newItem, $oldItem);
          } else {
            ele.insertBefore($newItem, $oldItem.nextSibling);
          }
          var item = that.sortTableData.splice(oldIndex, 1);
          that.sortTableData.splice(newIndex, 0, item[0]);
          const newArray = that.sortTableData.slice(0);
          // 重新赋值来刷新视图 （这里我调的父组件的变量，你的可以换成当前组件的变量）
          var topHeight = wrapper.scrollTop;
          that.sortTableData = []; // 必须有此步骤，不然拖拽后回弹
          that.$nextTick(function () {
            that.sortTableData = newArray; // 重新赋值，用新数据来刷新视图
            queueMicrotask(() => {
              this.selectRow = null;
              wrapper.scrollTop = topHeight;
            });
          });
        }
      });
    },
    clickSetSort(visible = false, cb) {
      var isCompany = this.$route.path.includes('monthlyPerfCompanySum');
      const params = {
        data: {
          company: {
            id: this.$store.state.user.companyId
          },
          department: {
            id: isCompany ? undefined : localstorageGet('userDepartmentId')
          },
          // assessmentCycle: 'month',
          applyScope: isCompany ? 'company' : 'department',
          users: this.sortTableData.map((item, index) => {
            return { id: item.id, sort: index + 1 };
          })
        }

      };
      this.$axios.post(
        Api.performance.KpiSummaryUserSortSetUpdate,
        params,
        (res) => {
          if (res.isSuccess) {
            this.sortVisible = visible;
            cb && cb();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    fetchSortData(cb) {
      var isCompany = this.$route.path.includes('monthlyPerfCompanySum');
      const params = {
        data: {
          company: {
            id: this.$store.state.user.companyId
          },
          department: {
            id: isCompany ? undefined : localstorageGet('userDepartmentId')
          },
          assessmentCycle: 'month',
          applyScope: isCompany ? 'company' : 'department',
          targetTime: this.targetTime
        }

      };
      this.$axios.post(
        Api.performance.KpiSummaryUserSortSetList,
        params,
        (res) => {
          if (res.isSuccess) {
            console.log(res, 'succ');
            const data = res.data.users || [];
            this.sortTableData = data.map((item, index) => {
              item.sortIndex = index;
              return item;
            });
            this.$nextTick(() => {
              cb && cb();
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleSetSort() {
      this.selectRow = null;
      this.sortVisible = true;
      this.$nextTick(() => {
        this.sortablePersonInfoMethods('sortDytable');
      });
    },
    handleSortClose() {
      this.sortVisible = false;
    },
    addSortPersonClose() {
      this.addSortPersonVisible = false;
      this.chooseRadio = '';
    },
    getList() {
      var isCompany = this.$route.path.includes('monthlyPerfCompanySum');
      const params = {
        data: {
          company: {
            id: this.$store.state.user.companyId
          },
          department: {
            id: isCompany ? undefined : localstorageGet('userDepartmentId')
          },
          assessmentCycle: 'month',
          applyScope: isCompany ? 'company' : 'department',
          targetTime: this.targetTime
        },
        pagination: true,
        size: this.pagination.size,
        current: this.pagination.pages

      };
      this.$axios.post(
        Api.performance.kpiSummaryList,
        params,
        (res) => {
          if (res.isSuccess) {
            this.kpiSumList = res.data || [];
            this.pagination.total = res.total || 0;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    changeScope() {
      this.$refs.elcascader.dropDownVisible = false;
    },
    getCustomerTree() { // 客户组织架构
      const params = {
        data: {
          clienteleId: this.$store.state.user.customerCode // 查客户组织架构，带用户id
        }
      };
      this.$axios.post(
        '/web/user/api/clienteleCompany/findCompany',
        params,
        (res) => {
          if (res.isSuccess) {
            this.treeData = res.data;
            if (res?.data?.length > 0) {
              // this.selectId = res.data[0].id;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    print() {
      $print(this.$refs.print);
    },
    viewDetail(row) {
      console.log(row, 'row');
      // this.visible = true;
      this.$fm.show("flowDetail", {
        data: {
          id: undefined, // 流程实例id
          flowInstanceBizRelevanceList: [{
            otherBiz: undefined, // 业务类型
            otherBizId: row.id // 业务id
          }]
        }
      });
    },
    handleClose() {
      this.visible = false;
    }
  },
  created() {
    this.getCompanyTree();
  },
  computed: {},
  watch: {
    filterText(val) {
      this.$refs.companyTree.filter(val);
    }

  }
};

</script>
<style>
.monthlyPerfBlocSum .el-dialog__body{
  min-height: 95vh !important;
}
</style>
<style lang='scss' scoped>
::v-deep .el-radio{
  margin-right: 0;
}
.app{
  background-color: white;
  height: 100%;
  .searchTop{
    padding-top: 20px;
    padding-left: 20px;
  }
}
</style>
