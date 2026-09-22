<template>
  <div class="custom-container">
    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoObj" @selectFlow="selectFlow" :fieldSelectType="fieldSelectType">
      <template scope="scope">
        <span v-if="printRead" class="print-read-label">
          <span>{{ displayName }}</span>
        </span>

        <template v-else>
          <el-input v-if="displayName" readonly :value="displayName" :disabled="disabled" @focus="openInfoDialog" style="width:100%;padding-left: 5px;">
            <el-button slot="append" icon="el-icon-search" @click="checkDetail"></el-button>
          </el-input>
          <el-button v-else type="primary" :disabled="disabled" @click="openInfoDialog" style="margin-left:5px;">{{ placeholder }}</el-button>
        </template>
      </template>
    </MySendFlowList>

    <!-- 流程-查看弹窗(对formMakiing制作的表单的查看) -->
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
import MySendFlowList from './ContractChangeListSelectList.vue';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: 'ContractChangeListSelect',
  components: {
    MySendFlowList,EnterpriseExamineDialog,CheckFlowNodeDetail
  },
  mixins:[mixin],
  props: {
    value: {
      type: String,
      default: ''
    },
    placeholder: {
      type: String,
      default: ''
    },
    disabled: { // value
      type: [Boolean],
      default() {
        return false;
      }
    },
    printRead: { // value
      type: [Boolean],
      default() {
        return true;
      }
    }
  },
  data() {
    return {
      currentInfoObj: this.value,
      flowListVisible: false,
      fieldSelectType: this.$parent.modelName,
      typeList: {
        'contract': {
          name:'合同',
        },
        'flow': {
          name:'流程',
        },
      },
      btnVisible:false,
    };
  },
  created() {
  },
  mounted() {
  },
  watch: {
    value(val) {
      this.currentInfoObj = val
    },
    currentInfoObj(val){
      this.$emit('input', val)
    },
  },
  computed: {
    parsedInfo () {
      return parseJsonObject(this.currentInfoObj)
    },
    displayName () {
      const info = this.parsedInfo
      if (!info) return ''
      if (typeof info === 'string') return info
      if (info.name) return info.name
      if (info.contractName) return info.contractName
      if (info.title) return info.title
      if (info.contractNumber) return info.contractNumber
      if (info.rowData) {
        const row = parseJsonObject(info.rowData)
        if (row && (row.name || row.contractName)) return row.name || row.contractName
      }
      return info.id || ''
    },
    fieldType(){
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType || ''))
      var str = '';
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
  methods: {
    checkDetail(){
      const info = parseJsonObject(this.currentInfoObj)
      if (info && info.rowData) {
        let row = parseJsonObject(info.rowData);
        if (row && Object.keys(row).length) this.previewHandle(row, false);
      }
    },
    openInfoDialog(){
      const info = parseJsonObject(this.currentInfoObj)
      let selectCompanyId = info && info.selectCompanyId ? info.selectCompanyId : '';
      if (info && info.formType == 'contract_receipt_form') {
        if (selectCompanyId) {
          this.flowListVisible = true;
        } else {
          this.$message.warning('请先选择收款单位！');
        }
      } else {
        this.flowListVisible = true;
      }
    },
    selectFlow(data) {
      this.currentInfoObj = data;
      this.flowListVisible = false;

      this.$nextTick(() => { 
        if (this.$parent && this.$parent.$parent && typeof this.$parent.$parent.validate === 'function') {
          const result = this.$parent.$parent.validate([this.fieldSelectType])
          if (result && typeof result.catch === 'function') result.catch(() => {})
        }
      })
    },
  },
};
</script>
<style lang="scss" scoped>
.custom-container {
  width: 100%;
}

table {
  width: 100%;
}

td {
  text-align: center;
  padding: 0 2px;
}
</style>
