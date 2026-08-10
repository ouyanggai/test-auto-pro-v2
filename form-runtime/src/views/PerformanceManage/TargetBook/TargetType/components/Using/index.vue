<!--
 * @Descripttion: 任务管理 / 指标类型设置/使用中
 * @Author: Calvin
 * @Date: 2021-09-09 09:55:46
-->

<template>
  <div class="using-container">
    <el-button
      @click="addTargetType"
      type="primary"
      icon="el-icon-circle-plus-outline"
    >新增
    </el-button>

    <dy-table
      maxTableHeight="600"
      :keys="colKey"
      :fetchData="fetchData"
      :list="tableData"
      :actions="actions"
      style="padding:0px;"
      ref="usingTable"
      :isPagination="true"
      :pagination="pagination"
    >
    </dy-table>

    <AddTargetTypeDdialog
      v-if="addTargetTypeVisible"
      :visible.sync="addTargetTypeVisible"
      :id="targetTypeId"
      :title="typeDialogTitle"
      @success="fetchData"
    />
  </div>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
import AddTargetTypeDdialog from './components/AddTargetTypeDdialog.vue';

export default {
  name: '',
  components: {
    DyTable,
    AddTargetTypeDdialog
  },
  data() {
    return {
      targetTypeId: '',
      typeDialogTitle: '新增类型',
      tableData: [],
      addTargetTypeVisible: false,
      // 人员信息
      colKey: {
        name: '名称',
        applyScope: {
          label: '指标类型',
          handle: function (scope, createElement) {
            return createElement('span', scope.row.manageType == 'manager_target' ? '管理指标' : '工作指标');
          }
        },
        createDate: '创建时间',
        remarks: '说明'
      },
      actions: [
        {
          label: '编辑',
          action: row => {
            this.editTargetType(row);
          }
        },
        {
          handle: (scope, createElement, self) => {
            const canUpdate = scope.row.canUpdate;
            let canUpdateName = '';
            if (canUpdate) {
              canUpdateName = '删除';
              const click = () => {
                this.handleDelete(scope.row.id);
              };
              return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                <span onClick={click}>
                  {canUpdateName}</span>
              ]);
            } else {
              canUpdateName = '归档';
              const click = () => {
                this.handleArchive(scope.row.id);
              };
              return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                <span onClick={click}>
                  {canUpdateName}</span>
              ]);
            }
          }
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() { },
  updated() {
  },
  methods: {
    fetchData() {
      const param = {
        data: {
          enableType: 'enable',
          checkCanUpdate: true
        },
        pagination: true,
        current:  this.pagination.pages,
        size: this.pagination.size
      };

      this.$axios.post(
        Api.performance.indicatorsTypeList,
        param,
        res => {
          if (res.isSuccess) {
            this.tableData = res.data ? res.data : [];
            this.pagination.total = res.total
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    addTargetType() {
      this.targetTypeId = '';
      this.addTargetTypeVisible = true;
    },
    editTargetType(row) {
      console.log(row.id);
      this.targetTypeId = row.id;
      this.addTargetTypeVisible = true;
      this.typeDialogTitle = '修改类型';
    },
    // 删除
    handleDelete(id) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.performance.indicatorsTypeDelete,
          {
            data: {
              id
            }
          },
          res => {
            if (res.isSuccess) {
              this.$message.success('删除成功');
              this.fetchData();
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }).catch(() => {
      });
    },
    // 归档
    handleArchive(id) {
      this.$confirm('确认要归档吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.performance.indicatorsTypeUpdate,
          {
            data: {
              id,
              enableType: 'disable'
            }
          },
          res => {
            if (res.isSuccess) {
              this.$message.success('归档成功');
              this.fetchData();
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }).catch(() => {
      });
    }
  }
};
</script>

<style scoped lang="less">
</style>
