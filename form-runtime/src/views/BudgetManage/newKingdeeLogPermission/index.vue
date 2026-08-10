<template>
  <div class="new-kingdee-log-permission">
    <div class="permission-panel">
      <div class="toolbar">
        <div class="query-item">
          <span class="query-label">姓名</span>
          <el-input
            v-model.trim="searchForm.name"
            clearable
            placeholder="请输入姓名"
            class="query-input"
            @keyup.enter.native="handleSearch"
          />
        </div>
        <div class="query-item">
          <span class="query-label">手机号</span>
          <el-input
            v-model.trim="searchForm.phone"
            clearable
            placeholder="请输入手机号"
            class="query-input"
            @keyup.enter.native="handleSearch"
          />
        </div>
        <el-button type="success" @click="handleSearch">搜索</el-button>
        <el-button @click="handleReset">重置</el-button>
      </div>

      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
        class="permission-table"
        empty-text="暂无人员数据"
      >
        <el-table-column prop="name" label="姓名" min-width="120" />
        <el-table-column prop="phone" label="手机号" min-width="140" />
        <el-table-column prop="companyName" label="所属公司" min-width="220" show-overflow-tooltip />
        <el-table-column prop="departmentName" label="所属部门" min-width="160" show-overflow-tooltip />
        <el-table-column prop="dutyName" label="岗位" min-width="160" show-overflow-tooltip />
        <el-table-column label="状态" width="90" align="center">
          <template slot-scope="scope">
            <el-tag v-if="scope.row.enableType === 'disable'" size="small" type="info">禁用</el-tag>
            <el-tag v-else size="small" type="success">启用</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="查看权限" min-width="280" show-overflow-tooltip>
          <template slot-scope="scope">
            <el-tag
              v-if="scope.row.scopeType === 'ALL' || scope.row.permissionText === '全部公司'"
              size="small"
              type="success"
            >
              全部公司
            </el-tag>
            <span v-else>{{ scope.row.permissionText || '未配置' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="center">
          <template slot-scope="scope">
            <el-button type="text" size="small" @click="handleAssign(scope.row)">分配权限</el-button>
            <el-button
              v-if="scope.row.permissionId"
              type="text"
              size="small"
              @click="handleClearPermission(scope.row)"
            >
              清除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
      :title="dialogTitle"
      :visible.sync="dialogVisible"
      width="620px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form ref="form" :model="form" :rules="rules" label-width="110px">
        <el-form-item label="选择用户" prop="userId">
          <el-select
            v-model="form.userId"
            filterable
            placeholder="请选择用户"
            style="width: 100%"
            disabled
          >
            <el-option
              v-for="user in userOptions"
              :key="user.id"
              :label="formatUserOptionLabel(user)"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="权限类型" prop="scopeType">
          <el-radio-group v-model="form.scopeType" @change="handleScopeTypeChange">
            <el-radio label="ALL">全部公司</el-radio>
            <el-radio label="COMPANY">指定公司</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item
          v-if="form.scopeType === 'COMPANY'"
          label="查看公司"
          prop="companyIds"
        >
          <div class="tree-box">
            <el-tree
              ref="companyTree"
              :data="companyTree"
              show-checkbox
              node-key="id"
              default-expand-all
              :props="treeProps"
              @check="handleCompanyCheck"
            />
          </div>
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';

export default {
  name: 'NewKingdeeLogPermission',
  data() {
    return {
      searchForm: {
        name: '',
        phone: ''
      },
      loading: false,
      tableData: [],
      userOptions: [],
      permissionMap: {},
      dialogVisible: false,
      form: {
        userId: '',
        name: '',
        phone: '',
        scopeType: 'COMPANY',
        companyIds: []
      },
      companyTree: [],
      treeProps: {
        children: 'children',
        label: 'name'
      },
      rules: {
        userId: [{ required: true, message: '请选择用户', trigger: 'change' }],
        scopeType: [{ required: true, message: '请选择权限类型', trigger: 'change' }],
        companyIds: [
          {
            validator: (rule, value, callback) => {
              if (this.form.scopeType === 'COMPANY' && (!value || !value.length)) {
                callback(new Error('请选择查看公司'));
                return;
              }
              callback();
            },
            trigger: 'change'
          }
        ]
      }
    };
  },
  computed: {
    dialogTitle() {
      return '分配日志权限';
    },
    currentCompanyId() {
      return this.$store.state.user.companyId || this.$store.state.user.projectCompanyId || '';
    },
    currentCustomerCode() {
      return this.$store.state.user.customerCode || '';
    }
  },
  mounted() {
    this.getPermissionList();
    this.getCompanyTree();
    this.getUserList();
  },
  methods: {
    handleSearch() {
      this.getUserList();
    },
    handleReset() {
      this.searchForm = {
        name: '',
        phone: ''
      };
      this.getUserList();
    },
    getPermissionList() {
      this.$axios.post(
        Api.kingdee.pushRecordLogPermissionList,
        {
          pagination: false,
          data: {
            customerCode: this.currentCustomerCode
          }
        },
        res => {
          if (!res.isSuccess) {
            return;
          }
          const list = Array.isArray(res.data) ? res.data : [];
          const permissionMap = {};
          list.forEach(item => {
            if (!item.userId) {
              return;
            }
            const companyIds = item.companyIds || [];
            permissionMap[item.userId] = {
              id: item.id,
              scopeType: item.scopeType,
              companyIds,
              permissionText: this.buildPermissionText(item.scopeType, companyIds)
            };
          });
          this.permissionMap = permissionMap;
          this.refreshPermissionText();
        },
        false,
        { noLoading: true }
      );
    },
    getUserList() {
      if (!this.currentCompanyId) {
        this.$message.warning('未获取到当前公司，无法查询人员');
        return;
      }
      this.loading = true;
      Promise.all([
        this.fetchUserListByEnableType('enable'),
        this.fetchUserListByEnableType('disable')
      ]).then(([enableList, disableList]) => {
        const userList = this.normalizeUserList(enableList.concat(disableList));
        this.tableData = userList;
        this.userOptions = userList;
      }).finally(() => {
        this.loading = false;
      });
    },
    fetchUserListByEnableType(enableType) {
      return this.$axios.post(
        '/web/org/getPageUserListForExternalOrg',
        {
          platformCode: 999999,
          pagination: true,
          current: 1,
          size: 9999,
          data: {
            companyVo: {
              id: this.currentCompanyId
            },
            userVo: {
              enableType,
              name: this.searchForm.name || ''
            },
            kingdeeRef: null,
            customerCode: this.currentCustomerCode
          }
        },
        null,
        false,
        { noLoading: true }
      ).catch(() => {
        return [];
      }).then(res => {
        const list = res && res.isSuccess ? (res.data || []) : [];
        if (!this.searchForm.phone) {
          return list;
        }
        return list.filter(item => {
          const user = item.userCompanyDeptOnDutyVo || item.rshUserVo || item.userVo || item;
          return user && user.phone && user.phone.indexOf(this.searchForm.phone) > -1;
        });
      });
    },
    getCompanyTree() {
      if (!this.currentCompanyId) {
        return;
      }
      const companyTreePromise = this.$axios.post(
        '/web/user/api/company/children',
        {
          data: {
            customerCode: this.currentCustomerCode,
            flag: 3,
            id: this.currentCompanyId
          }
        },
        null,
        false,
        { noLoading: true }
      ).then(res => {
        if (res.isSuccess) {
          return this.filterCompanyNodes(res.data || []);
        }
        this.$message.error(res.message || '查询公司树失败');
        return [];
      }).catch(() => []);

      Promise.all([companyTreePromise, this.fetchProjectCompanyList()]).then(([tree, projectCompanies]) => {
        this.companyTree = this.appendProjectCompaniesToTree(tree, projectCompanies);
        this.refreshPermissionText();
      });
    },
    normalizeUserList(data) {
      const userMap = {};
      data.forEach(item => {
        const row = this.normalizeUserRow(item);
        if (!row || !row.id) {
          return;
        }
        const oldRow = userMap[row.id];
        if (!oldRow) {
          userMap[row.id] = row;
          return;
        }
        oldRow.departmentName = this.mergeText(oldRow.departmentName, row.departmentName);
        oldRow.dutyName = this.mergeText(oldRow.dutyName, row.dutyName);
      });
      return Object.keys(userMap).map(id => this.applyPermission(userMap[id]));
    },
    normalizeUserRow(item) {
      const user = item.userCompanyDeptOnDutyVo || item.rshUserVo || item.userVo || item;
      if (!user) {
        return null;
      }
      const relation = this.resolveCurrentCompanyRelation(user);
      return {
        id: user.id || item.id,
        name: user.name || user.username || '',
        phone: user.phone || '',
        enableType: user.enableType || 'enable',
        companyId: relation.id || user.companyId || this.currentCompanyId,
        companyName: relation.name || this.$store.state.user.companyName || '',
        departmentId: relation.deptVo ? relation.deptVo.id : user.departmentId,
        departmentName: relation.deptVo ? relation.deptVo.departmentName : user.departmentName,
        dutyId: relation.dutyVo ? relation.dutyVo.id : user.dutyId,
        dutyName: relation.dutyVo ? relation.dutyVo.dutyName : user.dutyName
      };
    },
    resolveCurrentCompanyRelation(user) {
      const list = user.companyDeptVoList || [];
      if (!list.length) {
        return {};
      }
      return list.find(item => item.id === this.currentCompanyId) || list[0] || {};
    },
    applyPermission(row) {
      const permission = this.permissionMap[row.id];
      if (!permission) {
        return {
          ...row,
          permissionId: '',
          scopeType: '',
          companyIds: [],
          permissionText: '未配置'
        };
      }
      return {
        ...row,
        permissionId: permission.id,
        scopeType: permission.scopeType,
        companyIds: permission.companyIds.slice(),
        permissionText: this.buildPermissionText(permission.scopeType, permission.companyIds)
      };
    },
    mergeText(oldText, newText) {
      if (!newText) {
        return oldText || '';
      }
      if (!oldText) {
        return newText;
      }
      const values = oldText.split('、');
      return values.indexOf(newText) > -1 ? oldText : `${oldText}、${newText}`;
    },
    filterCompanyNodes(nodes) {
      return nodes
        .filter(node => node.type !== '5' && node.type !== '2' && node.type !== 'department')
        .map(node => {
          const next = {
            id: node.id,
            name: node.name || node.departmentName
          };
          const children = node.childrenList || node.children || [];
          if (children.length) {
            next.children = this.filterCompanyNodes(children);
          }
          return next;
        });
    },
    fetchProjectCompanyList() {
      return this.$axios.post(
        Api.budgetManage.projectCompany,
        {
          pagination: false,
          data: {
            companyId: '',
            name: ''
          }
        },
        null,
        false,
        { noLoading: true }
      ).then(res => {
        if (!res || !res.isSuccess) {
          return [];
        }
        return res.data && Array.isArray(res.data.dataList) ? res.data.dataList : [];
      }).catch(() => []);
    },
    appendProjectCompaniesToTree(tree, projectCompanies) {
      if (!projectCompanies || !projectCompanies.length) {
        return tree;
      }
      const projectCompanyMap = {};
      projectCompanies.forEach(item => {
        if (!item || !item.companyId || !item.id) {
          return;
        }
        if (!projectCompanyMap[item.companyId]) {
          projectCompanyMap[item.companyId] = [];
        }
        projectCompanyMap[item.companyId].push({
          id: item.id,
          name: item.name,
          type: 'projectCompany'
        });
      });
      const append = nodes => {
        return (nodes || []).map(node => {
          const children = append(node.children || []);
          const projectChildren = projectCompanyMap[node.id] || [];
          return {
            ...node,
            children: children.concat(projectChildren)
          };
        });
      };
      return append(tree);
    },
    handleAssign(row) {
      const companyIds = row.scopeType === 'ALL'
        ? this.collectCompanyIds(this.companyTree)
        : (row.companyIds ? row.companyIds.slice() : []);
      this.form = {
        userId: row.id,
        name: row.name,
        phone: row.phone,
        scopeType: row.scopeType === 'ALL' || this.isAllCompanySelected(companyIds) ? 'ALL' : (row.scopeType || 'COMPANY'),
        companyIds
      };
      this.dialogVisible = true;
      this.$nextTick(() => {
        if (this.$refs.companyTree) {
          this.$refs.companyTree.setCheckedKeys(this.form.companyIds);
        }
      });
    },
    handleScopeTypeChange(scopeType) {
      if (scopeType === 'ALL') {
        this.form.companyIds = this.collectCompanyIds(this.companyTree);
        if (this.$refs.form) {
          this.$refs.form.clearValidate('companyIds');
        }
        return;
      }
      this.$nextTick(() => {
        if (this.$refs.companyTree) {
          this.$refs.companyTree.setCheckedKeys(this.form.companyIds || []);
        }
        if (this.$refs.form) {
          this.$refs.form.validateField('companyIds');
        }
      });
    },
    handleCompanyCheck() {
      if (!this.$refs.companyTree) {
        return;
      }
      this.form.companyIds = this.$refs.companyTree.getCheckedKeys();
      this.$refs.form.validateField('companyIds');
    },
    handleSave() {
      this.$refs.form.validate(valid => {
        if (!valid) {
          return;
        }
        const companyIds = this.form.scopeType === 'ALL'
          ? this.collectCompanyIds(this.companyTree)
          : this.form.companyIds.slice();
        if (!companyIds.length) {
          this.$message.warning('未获取到可分配公司，无法保存全部公司权限');
          return;
        }
        const scopeType = 'COMPANY';
        this.$axios.post(
          Api.kingdee.pushRecordLogPermissionSave,
          {
            data: {
              userId: this.form.userId,
              userName: this.form.name,
              phone: this.form.phone,
              scopeType,
              companyIds,
              customerCode: this.currentCustomerCode
            }
          },
          res => {
            if (!res.isSuccess) {
              return;
            }
            this.$set(this.permissionMap, this.form.userId, {
              id: res.data ? res.data.id : '',
              scopeType,
              companyIds,
              permissionText: this.buildPermissionText(scopeType, companyIds)
            });
            this.refreshPermissionText();
            this.dialogVisible = false;
            this.$message.success('保存成功');
          }
        );
      });
    },
    handleClearPermission(row) {
      this.$confirm(`确认清除用户[${row.name}]的新金蝶日志权限吗？`, '提示', {
        type: 'warning',
        closeOnClickModal: false
      }).then(() => {
        this.$axios.post(
          Api.kingdee.pushRecordLogPermissionDelete,
          {
            data: {
              id: row.permissionId,
              userId: row.id,
              customerCode: this.currentCustomerCode
            }
          },
          res => {
            if (!res.isSuccess) {
              return;
            }
            this.$delete(this.permissionMap, row.id);
            this.refreshPermissionText();
            this.$message.success('清除成功');
          }
        );
      }).catch(() => {});
    },
    refreshPermissionText() {
      this.tableData = this.tableData.map(item => this.applyPermission(item));
      this.userOptions = this.userOptions.map(item => this.applyPermission(item));
    },
    resetForm() {
      this.form = {
        userId: '',
        name: '',
        phone: '',
        scopeType: 'COMPANY',
        companyIds: []
      };
      if (this.$refs.form) {
        this.$refs.form.resetFields();
      }
    },
    resolveCompanyNames(ids) {
      const result = [];
      const walk = nodes => {
        nodes.forEach(node => {
          if (ids.indexOf(node.id) > -1) {
            result.push(node.name);
          }
          if (node.children && node.children.length) {
            walk(node.children);
          }
        });
      };
      walk(this.companyTree);
      return result;
    },
    collectCompanyIds(nodes) {
      const result = [];
      const walk = list => {
        (list || []).forEach(node => {
          if (node.id && result.indexOf(node.id) === -1) {
            result.push(node.id);
          }
          if (node.children && node.children.length) {
            walk(node.children);
          }
        });
      };
      walk(nodes);
      return result;
    },
    isAllCompanySelected(companyIds) {
      const allCompanyIds = this.collectCompanyIds(this.companyTree);
      if (!allCompanyIds.length || !companyIds || !companyIds.length) {
        return false;
      }
      const selectedSet = new Set(companyIds);
      return allCompanyIds.every(id => selectedSet.has(id));
    },
    buildPermissionText(scopeType, companyIds) {
      if (scopeType === 'ALL' || this.isAllCompanySelected(companyIds || [])) {
        return '全部公司';
      }
      const names = this.resolveCompanyNames(companyIds || []);
      if (names.length) {
        return names.join('、');
      }
      return companyIds && companyIds.length ? `已配置${companyIds.length}个公司` : '未配置';
    },
    formatUserOptionLabel(user) {
      return `${user.name || '-'} / ${user.phone || '-'}`;
    }
  }
};
</script>

<style lang="scss" scoped>
.new-kingdee-log-permission {
  min-height: 100%;
  padding: 20px;
  background: #f0f2f5;

  .permission-panel {
    min-height: calc(100vh - 120px);
    padding: 26px 20px;
    background: #fff;
  }

  .toolbar {
    display: flex;
    align-items: center;
    margin-bottom: 30px;

    .query-item {
      display: flex;
      align-items: center;
      margin-right: 30px;
    }

    .query-label {
      margin-right: 14px;
      color: #333;
      font-size: 16px;
      font-weight: 600;
      white-space: nowrap;
    }

    .query-input {
      width: 280px;
    }

    .el-button {
      width: 84px;
      margin-left: 12px;
      border-radius: 0;
      font-weight: 600;
    }
  }

  .permission-table {
    width: 100%;
  }

  .tree-box {
    max-height: 320px;
    overflow: auto;
    padding: 12px;
    border: 1px solid #dcdfe6;
    border-radius: 2px;
  }
}
</style>
