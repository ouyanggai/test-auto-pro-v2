<!--
 * @Descripttion: 合同变更专用选择列表弹窗
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible" :title="'选择'+typeList[fieldType]['name']" width="80%" top="100px" fullscreen :close-on-click-modal="false"
     :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
     <div>
        <el-input style="width:180px;margin-right: 10px;" v-model.trim="searchForm.name" :placeholder="'查询'+typeList[fieldType]['name']+'名称'"></el-input>
        <el-input v-if="fieldType == 'contract'" style="width:180px;margin-right: 10px;" v-model.trim="searchForm.number" :placeholder="'查询'+typeList[fieldType]['name']+'编号'"></el-input>
        <el-input v-if="fieldType == 'contract'" style="width:180px;margin-right: 10px;" v-model.trim="searchForm.contractBody" :placeholder="'查询'+typeList[fieldType]['name']+'主体'"></el-input>

        <el-button type="primary" @click="searchList">查询</el-button>

        <div v-if="fieldType == 'contract'">
          <dy-table
            ref="contract"
            :height="'50vh'"
            :fetchData="getList"
            :keys="colKey1"
            :actions="actions1"
            :list="myFlowList"
            :isPagination="true"
            :pagination="pagination"
            @rowClick="isRowClick"
            ></dy-table>
        </div>
        <div v-if="fieldType == 'flow'">
          <dy-table
            ref="flow"
            :height="'50vh'"
            :fetchData="getList"
            :keys="colKey2"
            :actions="actions2"
            :list="myFlowList"
            :isPagination="true"
            :pagination="pagination"
            @rowClick="isRowClick"
          ></dy-table>
        </div>
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>

    <!-- 流程-查看弹窗 -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId"
    :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId"></EnterpriseExamineDialog>

    <!-- 查看流程 -->
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
    :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import Api from '@/api';
import mixin from './GeneralListSelectShow/mixin.js';
import { approveManageFlowStatus } from '@/utils';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import DyTable from '@/components/DyTable';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: 'ContractChangeListSelectList',
  components: {DyTable,EnterpriseExamineDialog,CheckFlowNodeDetail},
  model: {
    prop: 'myValue',
  },
  mixins:[mixin],
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    myValue: {
      type: [String, Object],
      default: ''
    },
    fieldSelectType: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      viewName:'',
      rowData:{},
      typeList: {
        'contract': {
          name:'合同',
        },
        'flow': {
          name:'流程',
        },
      },
      myFlowList: [],
      searchForm:{
        name:'',
        number:'',
        contractBody:'',
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      colKey1: {
        contractName: {
          label: '合同名称',
          showTooltip: true,
          minWidth: '220',
        },
        contractNumber: {
          label: '合同编号',
          showTooltip: true,
          minWidth: '180',
        },
      },
      actions1: [
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.checkFlowDetail(row);
          }
        },
      ],
      colKey2: {
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
      },
      actions2: [
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.previewHandle(row, false);
          }
        },
      ],
    };
  },
  computed: {
    parsedMyValue () {
      return parseJsonObject(this.myValue)
    },
    fieldType(){
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType || ''))
      var str = 'contract';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        if (result) {
          str = item;
          break;
        }
      }
      return str
    }
  },
  watch: {
    myValue: {
      immediate: true,
      handler(val) {
        const parsed = parseJsonObject(val)
        if (parsed) {
          this.viewName = parsed.name || parsed.contractName || parsed.title || ''
          this.rowData = parsed
        }
      }
    }
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    searchList() {
      this.pagination.pages = 1;
      this.getList();
    },
    async getList() {
      let data = {
        contractName: this.searchForm.name,
        contractNumber: this.searchForm.number,
      };
      return new Promise((resolve) => {
        this.$axios.post(
          Api.contractManage.contractInfo.getContractList,
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
              resolve(this.myFlowList);
            } else {
              this.myFlowList = [];
              resolve([]);
            }
          }
        );
      });
    },
    isRowClick(row) {
      this.rowData = row.row;
    },
    confirm() {
      if (!this.rowData || !Object.keys(this.rowData).length) {
        this.$message.warning('请选择一条数据！');
        return;
      }
      let data = {
        id: this.rowData.id,
        name: this.rowData.name || this.rowData.contractName,
        contractName: this.rowData.contractName || this.rowData.name,
        contractNumber: this.rowData.contractNumber,
        contractSum: this.rowData.contractSum,
      };
      this.$emit('selectFlow', JSON.stringify(data));
      this.viewName = data.name;
      this.handleClose();
    },
    async checkFlowDetail(row){
      let flowData = await this.getAllFormFlowDataByBizId(row.id);
      if (flowData && flowData[0]) {
        this.previewReserveHandle(flowData[0],'contract_seal_review')
      }
    }
  }
};
</script>
