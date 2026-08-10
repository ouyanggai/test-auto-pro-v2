<!--
 * @Descripttion: 任务管理 / 指标类型设置/已归档
 * @Author: Calvin
 * @Date: 2021-09-09 09:55:46
-->

<template>
  <div class="pigeonhole-container">

    <dy-table
      maxTableHeight="600"
      :keys="colKey"
      :fetchData="fetchData"
      :list="tableData"
      :actions="actions"
      style="padding:0px;"
      ref="companyPersonInfoTable"
      :isPagination="true"
      :pagination="pagination"
    >
    </dy-table>
  </div>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';

export default {
  name: '',
  components: {
    DyTable
  },
  data() {
    return {
      tableData: [],
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
          label: '还原',
          action: row => {
            this.handleRestore(row.id);
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
  mounted() {
  },
  updated() {
  },
  methods: {
    fetchData() {
      const param = {
        data: {
          enableType: 'disable',
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
    handleRestore(id) {
      this.$confirm('确认要还原吗?', '提示', {
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
              enableType: 'enable'
            }
          },
          res => {
            if (res.isSuccess) {
              this.$message.success('还原成功');
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
