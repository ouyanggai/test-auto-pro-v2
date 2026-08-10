<!--
 * @Descripttion: 自己发起的流程
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->
<template>
  <div class="dialog-container">
    <el-dialog :visible="visible" center append-to-body @close='handleClose' width="80%">
      <div>
        <el-input style="width:240px;margin-right: 10px;" v-model.trim="serachName" clearable placeholder="查询流程名称">
        </el-input>
        <el-button type="primary" @click="getList">查询</el-button>
        <dy-table
          :fetchData="getList"
          :keys="colKey"
          :list="myFlowList"
          :isPagination="true"
          :pagination="pagination"
          @rowClick="isRowClick"
        ></dy-table>
      </div>
      <span slot="footer" class="dialog-footer">
        <template>
          <el-button type="primary" @click="handleClose">关 闭</el-button>
          <el-button type="primary" @click="confirm">确 定</el-button>
        </template>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import { localstorageSet } from '@/utils/auth';
import { approveManageFlowStatus, deepClone } from '@/utils';
import DyTable from '@/components/DyTable';
import Api from '@/api';

export default {
  name: '',
  components: { DyTable },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    // selectFlowType: {
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      myFlowList: [],
      serachName: '',
      pagination: {
        total: 0,
        pages: 1,
        current: 1,
        size: 10
      },
      colKey: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '280',
        },
        flowName: {
          label: '流程名称',
          showTooltip: true,
          minWidth: '150',
        },
        createDate: {
          label: '发起时间',
          minWidth: '160',
          showTooltip: true
        },
        status: {
          label: '流程状态',
          minWidth: '100',
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.status));
          }
        },
      },
      rowData:{}
    };
  },
  computed: {},
  watch: {
  },
  created() {
  },
  mounted() {
  },
  methods: {
    getList() {
      const data = {
        flowName: this.serachName,
        useScope: 'invest',
        auditWayList: [],
        statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'company',
            otherBizId:''
          }
        ],
      };
      this.$axios.post(
        Api.schedule.getFlowInstanceList,
        {
          data,
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            this.pagination.total = res.total || 0;
            this.myFlowList = res.data || [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    isRowClick(row) {
      this.rowData = row.row;
    },  
    confirm(){
      if (!Object.keys(this.rowData).length) {
        this.$message.warning('请选择一条流程！')
        return;
      }
      this.$emit('confirmFlow',this.rowData)
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style lang="scss" scoped></style>
<style lang="scss">
</style>
