<template>
  <el-dialog
    :visible.sync="visible"
    fullscreen
    custom-class="announcement-dialog"
    :close-on-click-modal="false"
    :show-close="false" @close="handleClose"
  >
    <div class="dialog-header">
      <el-date-picker
        v-model="filters.year"
        type="year"
        placeholder="年份"
        value-format="yyyy"
        :disabled="type=='edit'"
        @change="changeItem"
        size="medium"
        style="width: 130px;">
      </el-date-picker>
      <span class="dialog-title">年</span>
      <el-select v-model="filters.quarter" :disabled="type=='edit'" placeholder="季度" class="header-select"
        size="medium" style="width: 140px;" @change="changeItem">
        <el-option
          v-for="item in quarterOptions"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </el-select>
      <span class="dialog-title">目标计划表公示汇总表</span>
      <el-button class="close-btn" icon="el-icon-close" @click="handleClose" circle></el-button>
    </div>
    <div class="dialog-filter">
      <el-form :inline="true" size="small">
        <el-form-item label="公司">
          <el-select v-model="filters.company" placeholder="请选择" style="width: 320px;" @change="changeCompany" :disabled="type=='edit'">
            <el-option v-for="item in companyOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="考核组">
          <el-select v-model="filters.group" placeholder="请选择" @change="changeItem" :disabled="type=='edit'">
            <el-option v-for="item in groupOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="dialog-stat" v-if="type=='add'">
        <el-link v-if="stat.unSubmit!=''||stat.unSubmit==0" type="warning" style="margin-left: 20px;">未提交：{{ stat.unSubmit }}人</el-link>
        <el-link v-if="stat.submitted!=''||stat.submitted==0" type="success" style="margin-left: 20px;">已提交：{{ stat.submitted }}人</el-link>
        <el-link v-if="stat.total!=''||stat.total==0" type="primary" style="margin-left: 20px;">总人数：{{ stat.total }}人</el-link>
      </div>
      <div class="dialog-stat" v-else>
        <el-link v-if="stat.total!=''||stat.total==0" type="primary" style="margin-left: 20px;">公示总人数：{{ stat.total }}人</el-link>
      </div>
    </div>
    <div class="dialog-table-wrapper">
      <el-table :data="tableData" :row-class-name="rowClassName" id="announcement-table" border style="width: 100%;" :height="'calc(100vh - 242px)'" size="mini" :row-key="row=>row.user.id||row.name" :key="tableKey">
        <el-table-column label="排序" align="center" width="80" v-if="type=='add'">
          <template>
            <!-- <el-tooltip class="item" effect="dark" content="长按拖动排序" placement="top"> -->
                <el-button type="info" class="handle_drop" plain circle size="mini">
                    <i class="el-icon-rank" style="font-size: 14px;"></i>
                </el-button>
            <!-- </el-tooltip> -->
          </template>
        </el-table-column>
        <el-table-column prop="name" label="姓名" min-width="100" >
          <template slot-scope="scope">
            <span>{{ scope.row.user&&scope.row.user.name ? scope.row.user.name : ''}}</span>
          </template>
        </el-table-column>
        <el-table-column prop="companyName" label="公司" min-width="200" />
        <el-table-column prop="depName" label="部门" min-width="120" />
        <el-table-column prop="dutyName" label="岗位" min-width="120" />
        <el-table-column prop="noticeStatus" label="公示状态" min-width="100" align="center">
          <template slot-scope="scope">
            <el-tag v-if="scope.row.noticeStatus" :type="noticeStatusOption[scope.row.noticeStatus].type">{{ noticeStatusOption[scope.row.noticeStatus].name || '' }}</el-tag>
            <span v-else>{{ scope.row.noticeStatus }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="kpiGroupStatus" label="审核状态" min-width="100" align="center">
          <template slot-scope="scope">
            <el-tag v-if="scope.row.kpiGroupStatus" :type="kpiGroupStatusOption[scope.row.kpiGroupStatus].type">{{ kpiGroupStatusOption[scope.row.kpiGroupStatus].name || '' }}</el-tag>
            <span v-else>{{ scope.row.kpiGroupStatus }}</span>
          </template>
        </el-table-column>
          <el-table-column label="操作" :width="type=='add'?200:150" align="center">
            <template slot-scope="scope">
              <div style="display: flex;justify-content:space-around">
                <el-link v-if = "scope.row.id" :underline="false" type="primary" @click="onView(scope.row)" >查看</el-link>
                <el-link v-else :underline="false" type="danger" >未上传计划</el-link>
                <el-button v-if="type=='add'" type="danger" @click="onDelete(scope.row,scope.$index)" >删除</el-button>
              </div>
            </template>
        </el-table-column>
      </el-table>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="handleClose">关闭</el-button>
      <el-button v-if="type=='add'" type="primary" @click="handleConfirm">确认</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import Sortable from 'sortablejs';
export default {
  name: 'AnnouncementDialog',
  props: {
    visible: {
      type: Boolean,
      required: true
    },
    type: {
      type: String,
      required: true,
      default: ''
    },
    id: {
      type: String,
      default: ''
    },
    companyOptions: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      tableKey: Date.now(), // 强制刷新标识
      sorTable: null, // 拖拽实例
      filters: {
        company: '',
        group: '',
        year: '',
        quarter: ''
      },
      // companyOptions: [],
      groupOptions: [],
      quarterOptions: [
        { label: '一季度', value: '1' },
        { label: '二季度', value: '2' },
        { label: '三季度', value: '3' },
        { label: '四季度', value: '4' }
      ],
      stat: {
        unSubmit: '',
        submitted: '',
        total: ''
      },
      tableData: [
        // { id: '1', name: '润小华', company: '', depName: '', dutyName: '', status: '', auditStatus: '', view: '' }
      ],
      // 审核状态
      kpiGroupStatusOption: {
        not_submitted: {
          type: 'warning',
          name: '草稿'
        },
        under_review: {
          type: 'primary',
          name: '审核中'
        },
        rejected: {
          type: 'danger',
          name: '驳回'
        },
        pass: {
          type: 'success',
          name: '已通过'
        },
        finish: {
          type: 'success',
          name: '已通过'
        }
      },
      // 公示状态
      noticeStatusOption: {
        not_started: {
          type: 'warning',
          name: '未公示'
        },
        in_progress: {
          type: 'primary',
          name: '公示中'
        },
        end: {
          type: 'success',
          name: '公示结束'
        }
      }
    };
  },
  watch: {
    visible(val) {
      console.log(val, 'val');
      if (val) {
        if (this.type == 'add') {
          this.addInitial();
        } else {
          this.editInitial();
        }
      }
    }
  },
  methods: {
    // 新增初始化
    addInitial() {
      this.stat = {
        unSubmit: '',
        submitted: '',
        total: ''
      };
      this.tableData = [];
      this.filters.company = '';
      this.filters.group = '';
      this.filters.year = new Date().getFullYear().toString();
      this.filters.quarter = Math.ceil((new Date().getMonth() + 1) / 3).toString();
      if (this.companyOptions && this.companyOptions.length == 1) {
        this.filters.company = this.companyOptions[0].id;
        this.getGroupList();
      }
      this.$nextTick(() => {
        this.rowDrop();
      });
    },
    // 编辑初始化
    editInitial() {
      this.$axios.post(Api.newPerformance.workPlanPromulgateById,
        { data: { id: this.id }},
        res => {
          this.filters = {
            company: '',
            group: '',
            year: '',
            quarter: ''
          };
          this.tableData = [];
          this.stat = {
            unSubmit: '',
            submitted: '',
            total: ''
          };
          if (res.isSuccess) {
            console.log(res, res);
            const targetTime = res.data.targetTime.toString();
            this.filters = {
              company: res.data.companyName,
              group: res.data.planUserGroupName,
              year: targetTime.slice(0, (targetTime.length - 1)),
              quarter: targetTime.slice((targetTime.length - 1), (targetTime.length))
            };
            this.getGroupList(this.filters.group);
            if (res.data.workPlanGroups && res.data.workPlanGroups.length) {
              this.tableData = res.data.workPlanGroups.map(item => {
                return {
                  ...item,
                  user: {
                    ...item.user,
                    name: item.user.name ? item.user.name : item.userName
                  }
                };
              });
              this.stat.total = res.data.workPlanGroups.length;
              this.stat.unSubmit = res.data.workPlanGroups.reduce((prex, item) => {
                if (!item.id) {
                  return prex + 1;
                } else {
                  return prex;
                }
              }, 0);
              this.stat.submitted = this.stat.total - this.stat.unSubmit;
            }
          } else {
            this.$message.error(`获取目标计划表公示汇总表失败：${res.message}`);
          }
        }
      );
    },
    // 获取指定公司分组
    getGroupList(group = '') {
      this.$axios.post(Api.newPerformance.planUserGroupList,
        { data: { company: { id: this.filters.company }}},
        res => {
          this.groupOptions = [];
          if (res.isSuccess) {
            console.log(res, res);
            if (res.data && res.data.length) {
              this.groupOptions = res.data;
              if (group) {
                this.filters.group = group;
              }
            }
          } else {
            this.$message.error(`获取分组设置失败：${res.message}`);
          }
        }
      );
    },
    changeCompany(val) {
      console.log(val, 'val');
      this.filters.group = '';
      this.tableData = [];
      this.getGroupList();
    },
    changeItem() {
      if (this.filters && this.filters.company && this.filters.group && this.filters.year && this.filters.quarter) {
        this.getList();
      } else {
        this.tableData = [];
      }
      console.log(1, '1');
    },
    getList () {
      this.$axios.post(
        Api.newPerformance.workPlanGroupList,
        {
          data: {
            // kpiScope: 'my_company_group',
            planUserGroup: {
              id: this.filters.group
            },
            targetTime: `${this.filters.year}${this.filters.quarter}`
          }
        },
        res => {
          console.log(res, 'res');
          this.tableData = [];
          this.stat.unSubmit = '';
          this.stat.submitted = '';
          this.stat.total = '';
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              this.tableData = res.data;
              this.stat.total = res.data.length;
              this.stat.unSubmit = res.data.reduce((prex, item) => {
                if (!item.id) {
                  return prex + 1;
                } else {
                  return prex;
                }
              }, 0);
              this.stat.submitted = this.stat.total - this.stat.unSubmit;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 拖拽方法
    rowDrop() {
      // 要侦听拖拽响应的DOM对象
      console.log('---rowDrop(拖拽初始化1)---');
      const el = document.querySelector('#announcement-table .el-table__body-wrapper tbody');
      console.log(el, 'el');
      const that = this;
      this.sorTable = new Sortable(el, {
        animation: 150,
        handle: '.handle_drop', // class类名执行事件
        ghostClass: 'blue-background-class',
        // 结束拖拽后的回调函数
        onEnd({ newIndex, oldIndex }) {
          console.log(oldIndex, '----拖动到--->', newIndex);
          const tempList = JSON.parse(JSON.stringify(that.tableData));
          // /** splice 新增删除并以数组的形式返回删除内容；（此处表示获取删除项对象） */
          const currentRow = tempList.splice(oldIndex, 1)[0];
          console.log(currentRow.user.name, 'currentRow');
          console.log(tempList.length, 'tempList');
          tempList.splice(newIndex > oldIndex ? newIndex - 1 : newIndex, 0, currentRow);
          // console.log('---新数组---', tempList);
          that.tableData = [...tempList];
          this.tableKey = Date.now();
          // console.log(that.tableData.map(item => item.user.name));
        }
      });
    },
    // 复制
    handleCopy(row, rowIndex) {
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    handleConfirm() {
      if (this.tableData && this.tableData.length == 0) {
        this.$message.warning('请选择需要提交的汇总！');
        return;
      }
      const filterArr = this.tableData.filter(item => {
        return !item.id || (item.id && (item.kpiGroupStatus == 'not_submitted' || item.kpiGroupStatus == 'rejected'));
      }).map(item => item.user.name);
      if (filterArr.length == this.tableData.length) {
        this.$message.warning('提交失败，当前分组没有人员提交目标计划表！');
        return;
      }
      if (filterArr && filterArr.length) {
        console.log(filterArr, 'filterArr');
        this.$confirm(`${filterArr.join(',')}未提交。确认发起公示后，系统自动剔除未提交目标计划表的人员，请确认是否提交？`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.saveSummary();
        }).catch(() => {
          this.$message({
            type: 'info',
            message: '已取消提交'
          });
        });
      } else {
        this.saveSummary();
      }
    },
    // 保存汇总
    saveSummary() {
      this.$axios.post(
        Api.newPerformance.workPlanPromulgateSave,
        {
          data: {
            id: null,
            companyId: this.filters.company,
            planUserGroupId: this.filters.group,
            targetTime: `${this.filters.year}${this.filters.quarter}`,
            workPlanGroups: this.tableData
          }
        },
        res => {
          console.log(res, 'res');
          if (res.isSuccess) {
            this.$emit('confirm');
            this.$emit('update:visible', false);
            this.$message({
              type: 'success',
              message: '提交成功!'
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    onView(row) {
      console.log(row, 'row');
      // 查看操作
      // this.$emit('view', row);
      this.$fm.show('flowDetail', {
        data: {
        // id: row.id, // 流程实例id
          flowInstanceBizRelevanceList: [{
            otherBiz: 'kpi2_work_plan', // 业务类型
            otherBizId: row.id // 业务id
          }]
        }
      });
    },
    onDelete(row, index) {
      console.log(row, 'row');
      console.log(index, 'index');
      this.$confirm(`确认删除${row.user.name}该行数据吗？`, '提示', {
        type: 'warning'
      }).then(() => {
        this.tableData.splice(index, 1); // 删除指定索引的行
        this.$message.success('删除成功！');
      }).catch(() => {
        this.$message.info('已取消删除');
      });
    },
    cellStyle({ rowIndex }) {
      // 第一行高亮
      if (rowIndex === 0) {
        return { background: '#faf1d6' };
      }
      return {};
    },
    rowClassName({ row }) {
      // 没有id 以及 id存在kpiGroupStatus是 not_submitted、rejected状态时
      return row.id ? (row.kpiGroupStatus == 'not_submitted' || row.kpiGroupStatus == 'rejected') ? 'warning-row' : '' : 'warning-row';
    }
  },
  mounted() {
    // this.rowDrop();
  },
  beforeDestroy() {
    this.sorTable = null;
  }
};
</script>

<style lang="scss" scoped>
.announcement-dialog {
  .el-dialog__body {
    padding: 0 40px 40px 40px;
    height: calc(100vh - 60px) !important;
    display: flex;
    flex-direction: column;
  }
}
.dialog-header {
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding-top: 20px;
  padding-bottom: 10px;
  .dialog-title {
    font-size: 28px;
    font-weight: bold;
    // margin-right: 20px;
    margin: 0 5px;
  }
  .header-select {
    width: 110px;
    vertical-align: middle;
  }
  .close-btn {
    position: absolute;
    right: 0;
    top: 0;
    z-index: 2;
  }
  ::v-deep .el-input__inner{
    font-size: 28px !important;
    font-weight: bold;
  }
}
.dialog-filter {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  .el-form {
    flex: 1;
  }
  .dialog-stat {
    min-width: 260px;
    text-align: right;
    font-size: 15px;
    color: #333;
    span {
      margin-left: 18px;
    }
  }
  ::v-deep .el-input__inner{
    font-size: 15px !important;
    font-weight: bold;
  }
  ::v-deep .el-link--inner{
    font-size: 15px !important;
    font-weight: bold;
  }
}
.dialog-table-wrapper {
  flex: 1;
  max-height: calc(100vh - 242px);
  overflow-y: auto;
  .el-table {
    width: 100%;
    font-size: 15px;
  }
  .el-link {
    font-size: 15px;
  }
}
::v-deep .el-dialog.is-fullscreen .el-dialog__body{
  min-height: 60vh !important;
  max-height: 100vh !important;
}
::v-deep .el-dialog__header {
  padding: 0px !important;
}
::v-deep .el-dialog__footer{
  text-align: center;
}
.drag-icon {
  font-size: 24px;
  cursor: move;
  color: #409EFF;
  transition: color 0.2s;
}
.drag-icon:hover {
  color: #66b1ff;
}
::v-deep .el-table .el-table__body-wrapper .el-table__row.warning-row {
  background-color: #ffebee !important;
}

::v-deep #announcement-table {
  .el-table__body tr:hover > td {
    background-color: transparent !important;
  }
  .el-table__row.hover-row > td {
    background-color: transparent !important;
  }
}
</style>
