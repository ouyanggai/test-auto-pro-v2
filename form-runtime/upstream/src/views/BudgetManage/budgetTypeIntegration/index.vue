<template>
  <div style="width: 100%; height:100%;background-color:white" v-if="true">
    <div class="right-content">
        <div class="search-box">
        <el-input placeholder="按费用类型/编号查询" v-model="searchValue" clearable style="width: 200px;margin-right: 10px;"></el-input>
        <el-button type="primary" icon="el-icon-search" @click="handleSearch">搜索</el-button>
        <el-button type="primary" icon="el-icon-plus" @click="handleAdd">新增</el-button>
        </div>
        <div class="list">
        <dy-table maxTableHeight="650" :keys="colKey" :fetchData="fetchData" :list="tableData" :actions="actions1"
            style="padding:0px;" ref="usingTable" :isPagination="true" :pagination="pagination">
        </dy-table>
        </div>
    </div>
    <el-dialog v-if="addEditDialogVisible" :visible="addEditDialogVisible" width="700px" :close-on-click-modal="false"
      :title="isEdit ? '编辑' : '新增'" @close="addEditDialogVisible=false">
      <el-form ref="addEditForm" :model="formData" label-width="140px">
        <el-form-item label="费用类型:" prop="costName" :rules="{ required: true, message: ' ', trigger: 'blur' }">
          <el-input style="width:400px" v-model.trim="formData.costName" placeholder="请输入费用类型" maxlength="30"/>
        </el-form-item>
        <el-form-item label="关联费用项目:" prop="cloudName" :rules="{ required: true, message: ' ', trigger: 'blur' }">
          <el-input style="width:400px" v-model="formData.cloudName" placeholder="请输入费用项目" maxlength="30" readonly/>
          <el-button type="text" style="margin-left:5px" size="small" @click="getCloudList">获取</el-button>
        </el-form-item>
      </el-form>
      <span slot="footer">
        <el-button @click="addEditDialogVisible=false">{{isEdit ? '关 闭' : '取 消'}}</el-button>
        <el-button type="primary" @click="addEditSure">确 定</el-button>
      </span>
    </el-dialog>
    <el-dialog v-if="pickerCloudVisible" top="40px" :visible="pickerCloudVisible" width="830px" title="选择金蝶费用项目"
     :close-on-click-modal="false" @close="pickerCloudVisible=false">
        <div>
            <el-input v-model="pickerCloudValue" placeholder="按费用类型/编号查询" clearable style="width: 200px;margin-right: 10px;"></el-input>
            <el-button type="primary" icon="el-icon-search" @click="getCloudList">搜索</el-button>
            <dy-table :showRadio="true" @rowClick="handleRowClick" :fetchData="_=>_" maxTableHeight="600" :keys="colCloudKey" :list="cloudTableList" style="padding:0px;"/>
        </div>
        <span slot="footer">
            <el-button @click="pickerCloudVisible=false">关 闭</el-button>
            <el-button type="primary" @click="handleCloudSure">确 定</el-button>
        </span>
    </el-dialog>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
export default {
  data() {
    return {
      clickRow: null,
      colCloudKey: {
        number: {
          label: '金蝶编号',
          handle(scope, createElement) {
            return createElement('span', scope.row.number);
          }
        },
        name: {
          label: '金蝶费用项目',
          handle(scope, createElement) {
            return createElement('span', scope.row.name);
          }
        },
      },
      pickerCloudValue: '',
      cloudTableList:[{}],
      pickerCloudVisible: false,
      isEdit: false,
      addEditDialogVisible: false,
      formData: {
        costName: '',
        cloudName: '',
        number: '',
        enabled: '1'
      },
      actions1: [],
      actions12: [
        {
          label: '详情',
          width: '80',
          actionFixed:'right',
          action: row => {
            console.log('row',row)
            this.checkFormFlow(row,this.invoiceDialogType)
          }
        },
      ],
      tableData: [{userName:'test润世华软件公司',userName2:'test润世华软件公司2'}],
      colKey: {
        costName: {
          label: '费用类型',
          // ifFixed:true,
          handle(scope, createElement) {
            return createElement('span', scope.row.costName);
          }
        },
        cloudName: {
          label: '金蝶费用项目',
          handle(scope, createElement) {
            return createElement('span', scope.row.cloudName);
          }
        },
        number: {
          label: '金蝶编号',
          handle(scope, createElement) {
            return createElement('span', scope.row.number);
          }
        },
        updateDate: {
          label: '更新时间',
          handle(scope, createElement) {
            return createElement('span', scope.row.updateDate);
          }
        },
        enabled: {
          label: "启用/停用",
          handle: (scope, createElement, that) => {
            return (
              <el-switch
                active-value='1'
                inactive-value='0'
                v-model={scope.row.enabled}
                onChange={(state) => {
                  this.$axios.post('/web/measuring/api/costType/update',
                    { data: {...scope.row, enabled: state } },
                    (res) => {
                      if (res.isSuccess) {
                        this.$message.success('修改成功')
                      }
                    }
                  );
                }}
              ></el-switch>
            );
          }
        },
        edit: {
          label: '操作',
          width: '100',
          handle: (scope, createElement) => {
            return <span>
              <el-button type="text" size="small" onClick={_ => { this.handleEdit(scope.row) }}>编辑</el-button>
              <el-button type='text' onClick={_ => { this.handleDelete(scope.row) }}>删除</el-button>
            </span>;
          }
        },
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      searchValue: '',
    }
  },
  components: { DyTable },
  methods: {
    handleRowClick({ row, event, column }) {
      this.clickRow = row;
    },
    handleCloudSure() {
      this.formData.cloudName = this.clickRow.name;
      this.formData.number = this.clickRow.number;
      this.pickerCloudVisible = false;
    },
    getCloudList() {
      this.$axios.post('/web/measuring/api/costType/cloudExpenseList',
        {
          data: {
            "status": "C",
            "name": this.pickerCloudValue,
            "forbidStatus": "A"
          }
        },
        res => {
          if (res.isSuccess) {
            this.cloudTableList = res.data.dataList || [];
            this.pickerCloudVisible = true;
          }
        }
      );
    },
    handleEdit(row) {
      this.isEdit = true;
      this.formData = JSON.parse(JSON.stringify(row));
      this.addEditDialogVisible = true;
    },
    async handleDelete(row) {
      await this.$confirm('确认删除?','提示', {type: 'warning'});
      this.$axios.post('/web/measuring/api/costType/delete', { data: { id: row.id } },
        res => {
          if (res.isSuccess) {
            this.$message.success('删除成功')
            this.fetchData();
          }
        }
      );
    },
    handleAdd() {
      this.isEdit = false;
      this.formData = {
        costName: '',
        cloudName: '',
        number: '',
        enabled: '1'
      };
      this.addEditDialogVisible = true;
    },
    addEditSure() {
      this.$refs.addEditForm.validate((valid) => {
        if (valid) {
          var data = JSON.parse(JSON.stringify(this.formData));
          this.$axios.post(
            this.isEdit
              ? "/web/measuring/api/costType/update"
              : "/web/measuring/api/costType/save",
            {
              data
            },
            (res) => {
              if (res.isSuccess) {
                this.addEditDialogVisible = false;
                this.fetchData();
              }
            }
          );
        }
      });
    },
    fetchData() {
      const param = {
        data: {
          costName: this.searchValue || '',
        //   enabled: "1"
        },
        pagination: true,
        size: this.pagination.size,
        current: this.pagination.pages
      };
      this.$axios.post('/web/measuring/api/costType/list', param,
        res => {
          if (res.isSuccess) {
            this.tableData = res.data?.dataList || [];
            this.pagination.total = res.data.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleSearch() {
      this.pagination.pages = 1;
        this.fetchData();
    },
    handleRatioChange(ratio) {
      console.log('当前比例:', ratio);
    }
  }
};
</script>

<style lang="scss" scoped>
.left-panel {
  width: 100%;
  height: 100%;
  .item {
    padding-left: 10px;
    line-height: 37px;
    height: 37px;
    white-space: nowrap;
    overflow: hidden;
    cursor: pointer;
    &:hover {
      background-color: #e6f7ff;
      // color: #1890ff;
    }
  }
  .active {
    background-color: #e6f7ff;
    color: #1890ff;
    border-right: 3px solid #1890ff;
  }
}
.right-content {
  width: 100%;
  height: 100%;
  padding: 10px;
  // box-sizing: border-box;
}
</style>