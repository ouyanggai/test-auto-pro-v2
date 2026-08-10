<template>
  <div>
    <el-button type="primary" @click="handleOpenDialog" :disabled="disabled">{{ placeholder || '库存材料选择' }}</el-button>
    <el-dialog
      :title="placeholder || '库存材料选择' "
      :visible.sync="dialogVisible"
      width="1000px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      fullscreen
      append-to-body
    >
      <div>
        <el-form :inline="true">
          <el-form-item label="材料名称">
            <el-input v-model="searchForm.goodsName" placeholder="材料名称" clearable></el-input>
          </el-form-item>
          <el-form-item label="关联仓库" >
            <el-select v-model="searchForm.library" placeholder="请选择" clearable @change="fetchData">
              <el-option
                v-for="item in libraryList"
                :key="item.value"
                :label="item.label"
                :value="item.value">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleSearch">查询</el-button>
          </el-form-item>
        </el-form>
        <dy-table
          ref="dyTable"
          :fetchData="fetchData"
          :keys="colKey"
          :actions="actionKey"
          :list="tableData"
          :isPagination="false"
          :height="tableHeight"
          :showCheckBox="true"
          @selectDataEvent="selectDataEvent"
        />
        <div>
          <el-tag
            style="margin: 5px;"
            v-for="tag in allSelectData"
            :key="tag.id"
            closable @close="handleCloseTag(tag)">
            {{tag.goodsName + '-' + tag.warehouse.name}}
          </el-tag>
        </div>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseDialog">取消</el-button>
        <el-button type="primary" @click="handleSelected">选中</el-button>
        <el-button type="primary" @click="handleConfirmData">确认添加</el-button>
      </div>
    </el-dialog>
  </div>
</template>
<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
export default {
  name: 'InventoryMaterialSelect',
  props: {
    placeholder: {
      type: String,
      default: ''
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  components: {
    DyTable
  },
  data() {
    return {
      searchForm: {
        goodsName: '',
        library: ''
      },
      libraryList: [],
      allSelectData: [],
      selectData: [],
      dialogVisible: false,
      tableData: [],
      colKey: {
        goodsName: {
          label: '材料名称',
          showTooltip: true
        },
        cas: {
          label: 'CAS'
        },
        modelNumber: {
          label: '规格型号'
        },
        goodsType: {
          label: '材料类型'
        },
        packagingContext: {
          label: '包装',
          handle: (scope, h) => {
            if (scope.row.packagingInfo && scope.row.packagingInfo.packagingContext) {
              return h('span', scope.row.packagingInfo.packagingContext);
            }
            return h('span', '');
          }
        },
        unit: {
          label: '单位',
          handle: (scope, h) => {
            if (scope.row.packagingInfo && scope.row.packagingInfo.unit) {
              return h('span', scope.row.packagingInfo.unit);
            }
            return h('span', '');
          }
        },
        // freezeCount: {
        //   label: '冻结数据'
        // },
        // totalCount: {
        //   label: '库存总数'
        // },
        warehouse: {
          label: '仓库',
          handle: (scope, h) => {
            if (scope.row.warehouse) {
              return h('span', scope.row.warehouse.name);
            }
            return h('span', '');
          }
        }
      },
      actionKey: [
        // {
        //   label: '编辑',
        //   width: '160',
        //   actionFixed: 'right',
        //   action: row => {
        //     this.onEdit(row);
        //   }
        // },
        // {
        //   handle: (scope, createElement, self) => {
        //     const click = () => {
        //       this.onDelete(scope.row);
        //     };
        //     return createElement('button', { class: 'el-button el-button--text el-button--small', style: 'color:#F56C6C;' }, [
        //       <span onClick={click}>删除</span>
        //     ]);
        //   }
        // }
      ],
      // pagination: {
      //   total: 400,
      //   pages: 1,
      //   size: 10
      // },
      tableHeight: '460px'
    };
  },
  methods: {
    getLibraryList() {
      this.$axios.post(Api.inventoryManage.warehouseList, {
        data: {
          name: '',
          enableType: 'enable',
          company: {
            id: localstorageGet('companyId')
          }
        },
        pagination: false
      }, res => {
        this.libraryList = [];
        if (res.isSuccess) {
          if (res.data && res.data.length) {
            this.libraryList = res.data.map(item => {
              return {
                label: item.name,
                value: item.id
              };
            });
          }
          // if (res.data && res.data.length) {
          //   this.libraryList = res.data(item => {
          //     return {
          //       label: item.name,
          //       value: item.id
          //     };
          //   });
          // }
        }
        console.log(this.libraryList, 'this.libraryList');
      });
    },
    handleSearch() {
      this.fetchData();
    },

    selectDataEvent(data) {
      console.log(data, 'data');
      this.selectData = data;
    },
    handleCloseTag(tag) {
      this.allSelectData = this.allSelectData.filter(item => item.id !== tag.id);
    },

    // 打开弹窗
    handleOpenDialog() {
      this.dialogVisible = true;
      this.allSelectData = [];
      this.selectData = [];

      this.getLibraryList();
      this.fetchData();
    },
    fetchData() {
      this.$axios.post(Api.inventoryManage.getSetLedgerGoods,
        {
          data: {
            goodsName: this.searchForm.goodsName,
            warehouse: {
              id: this.searchForm.library
            }
          },
          pagination: false
          // current: this.pagination.pages,
          // size: this.pagination.size
        },
        res => {
          this.tableData = [];
          // this.pagination.total = 0;
          if (res.isSuccess) {
            console.log(res, res);
            if (res.data && res.data.length) {
              this.tableData = res.data;
              // this.pagination.total = res.total;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 关闭弹窗
    handleCloseDialog() {
      this.dialogVisible = false;
    },
    handleSelected() {
      // 过滤掉已经存在于allSelectData中的项
      const newItems = this.selectData.filter(item =>
        !this.allSelectData.some(existing => existing.id === item.id)
      );
      this.allSelectData = this.allSelectData.concat(newItems);
    },
    // 确认导入
    handleConfirmData() {
      if (this.allSelectData.length === 0) {
        this.$message.error('请选中入库材料');
        return;
      }
      this.dialogVisible = false;
      this.$emit('input', JSON.stringify(this.allSelectData));
    }
  }
};
</script>
<style scoped lang="scss">
::v-deep .el-dialog.is-fullscreen .el-dialog__body{
  max-height: calc(100vh) !important;
  height: calc(100vh - 116px) !important;
}
::v-deep .dialog-footer{
  text-align: center;
}
</style>
