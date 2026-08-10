<template>
  <div class="group-set-wrapper">
    <el-container>
      <!-- 左侧公司列表 -->
      <el-aside width="240px">
        <h3 class="aside-header">公司列表</h3>
        <el-scrollbar class="aside-list">
          <ul>
            <li
              v-for="company in companyList"
              :key="company.id"
              :class="['company-item', { active: company.id === selectedCompanyId }]"
              @click="selectCompany(company.id)"
            >
              <span>{{ company.name }}</span>
            </li>
          </ul>
        </el-scrollbar>
      </el-aside>
      <!-- 右侧内容区 -->
      <el-main>
        <div class="main-header">
          <span style="font-size: 16px; font-weight: bold;">分组设置</span>
          <el-button style="margin-left: auto;" type="primary" size="small" @click="addGroup">新增分组</el-button>
        </div>
        <DyTable
          :fetchData="getGroupList"
          :height="tableHeight"
          :keys="groupTableKeys"
          :actions="groupTableActions"
          :list="groupList"
          :pagination="pagination"
          @current-change="handlePageChange"
          style="padding:0px;"
        />
      </el-main>
    </el-container>
    <group-set-dialog v-if="dialogVisible" :visible.sync="dialogVisible" :company-id="selectedCompanyId" :group-type="groupType" :group-data="selectedGroup" @submit="handleGroupSubmit" />
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import { localstorageGet } from '@/utils/auth';
import GroupSetDialog from './components/groupSetDialog.vue';
import Api from '@/api';
export default {
  name: 'GroupSet',
  components: { DyTable, GroupSetDialog },
  data() {
    return {
      dialogVisible: false,
      companyList: [
      ],
      selectedCompanyId: '', // 选中公司id
      selectedGroup: '', // 选中分组
      groupType: 'add ', // 新增分组 编辑分组
      groupList: [],
      groupTableKeys: {
        name: '分组名称',
        type: {
          label: '类型',
          width: '80px',
          handle: function (scope, createElement) {
            return createElement('span', scope.row.planGroupType == 'target_user' ? '指定人员' : scope.row.planGroupType == 'duty_level' ? '指定岗级' : '1');
          }
        },
        relation: {
          showTooltip: true,
          label: '关联设置',
          handle: function (scope, createElement) {
            return createElement('span', (scope.row.planGroupType == 'target_user' && scope.row.users && scope.row.users.length) ? scope.row.users.map(item => item.name).join(',') : (scope.row.planGroupType == 'duty_level' && scope.row.dutyLevels && scope.row.dutyLevels.length) ? scope.row.dutyLevels.map(item => item.name).join(',') : '');
          }
        }
      },
      groupTableActions: [
        {
          label: '编辑',
          width: '120px',
          action: row => { this.editGroup(row); }
        },
        {
          handle: (scope, createElement, self) => {
            const click = () => {
              this.deleteGroup(scope.row);
            };
            return createElement('button', { class: 'el-button el-button--text el-button--small', style: 'color:#F56C6C;' }, [
              <span onClick={click}>删除</span>
            ]);
          }
        }
      ],
      pagination: {
        total: 85,
        pages: 1,
        size: 10
      }
    };
  },
  computed: {
    tableHeight() {
      return `calc(100vh - 228px)`;
    }
  },
  methods: {
    // 获取公司列表
    getCompanyList() {
      this.$axios.post(Api.recruit.getCompanyList,
        { data: { id: localstorageGet('companyId') }},
        res => {
          this.companyList = [];
          this.groupList = [];
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              var cList = [];
              res.data.forEach(item => {
                if (item.parentId) {
                  cList.push(item);
                } else {
                  cList.unshift(item);
                }
              });
              this.companyList = cList;
              if (this.companyList && this.companyList.length) {
                this.selectedCompanyId = this.companyList[0].id;
                this.getGroupList();
              }
            }
          } else {
            this.$message.error(`获取公司列表失败：${res.message}`);
          }
        }
      );
    },
    // 获取指定公司分组
    getGroupList() {
      if (!this.selectedCompanyId) { return; }
      this.$axios.post(Api.newPerformance.planUserGroupList,
        { data: { company: { id: this.selectedCompanyId }}},
        res => {
          this.groupList = [];
          this.selectedGroup = {};
          if (res.isSuccess) {
            console.log(res, res);
            if (res.data && res.data.length) {
              this.groupList = res.data;
            }
          } else {
            this.$message.error(`获取分组设置失败：${res.message}`);
          }
        }
      );
    },
    selectCompany(id) {
      this.selectedCompanyId = id;
      // TODO: 切换公司时加载对应分组数据
      this.getGroupList();
    },
    addGroup() {
      // TODO: 新增分组逻辑
      this.groupType = 'add';
      this.dialogVisible = true;
      this.selectedGroup = {};
    },
    editGroup(row) {
      // TODO: 编辑分组逻辑
      console.log(row, 'row');
      this.groupType = 'edit';
      this.selectedGroup = row;
      this.dialogVisible = true;
    },
    deleteGroup(row) {
      // TODO: 删除分组逻辑
      this.$confirm(`此操作将永久删除 ${row.name} 该分组, 是否继续?`, '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(Api.newPerformance.planUserGroupDelete,
          { data: { id: row.id }},
          res => {
            if (res.isSuccess) {
              this.getGroupList();
              this.$message({
                type: 'success',
                message: `${row.name}分组删除成功`
              });
            } else {
              this.$message.error(`${row.name}分组删除失败：${res.message}`);
            }
          }
        );
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '已取消删除'
        });
      });
    },
    handleGroupSubmit(group) {
      this.getGroupList();
    },
    handlePageChange(page) {
      this.pagination.pages = page;
      // TODO: 分页切换逻辑
    }
  },
  mounted() {
    this.groupList = [];
    this.getCompanyList();
  }
};
</script>

<style lang="scss" scoped>
.group-set-wrapper {
  height: calc(100vh - 120px);
  background: #f5f6fa;
}
.el-container {
  height: calc(100vh - 120px);
}
.el-aside {
  background: #fff;
  border-right: 1px solid #eee;
  padding: 0;
  overflow: hidden;
  position: relative;
}
.aside-header {
  line-height: 44px;
  padding-left: 16px;
  font-size: 16px;
  font-weight: bold;
  border-bottom: 1px solid #f0f0f0;
  margin-bottom: 0;
  background: #fff;
  position: sticky;
  top: 0;
  z-index: 0;
}
.aside-list {
  height: calc(100vh - 166px);
  padding: 0;
  // 让el-scrollbar撑满剩余高度
}
.aside-list ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
.company-item {
  padding: 10px;
  cursor: pointer;
  color: #333;
  // border-top-left-radius: 4px;
  // border-bottom-left-radius: 4px;
  margin-bottom: 2px;
  transition: background 0.2s;
  font-size: 14px;
  border-left: 4px solid #fff;
}
.company-item.active {
  border-left: 4px solid #1890ff;
  color: #1890ff;
  background-color: #e6f7ff;
}
.el-main {
  background: #fff;
  padding: 24px 24px 0 24px;
  height: calc(100vh - 120px);
  overflow-y: auto;
}
.main-header {
  display: flex;
  justify-content: flex-start;
  align-items: center;
  margin-bottom: 10px;
}
.pagination-bar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  margin-top: 16px;
  gap: 16px;
}
// 滚动条美化（Element风格）
::v-deep .el-scrollbar__bar.is-vertical {
  width: 6px;
  background: rgba(144,147,153,.2);
  border-radius: 4px;
  right: 2px;
}
::v-deep .el-scrollbar__thumb {
  background: #c1c1c1;
  border-radius: 4px;
  transition: background .3s;
}
::v-deep .el-scrollbar__bar.is-vertical:hover .el-scrollbar__thumb {
  background: #a0a0a0;
}
::v-deep .is-vertical{
  background: transparent !important
}
</style>
