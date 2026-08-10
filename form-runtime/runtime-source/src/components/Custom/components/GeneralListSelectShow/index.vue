<!--
 * @Descripttion: 通用列表选择后展示（选择单个数据）（目前支持合同和流程列表选择）(参考合规评审表的手续文件或者合同付款单选择合同)
 * 注意：refresh里面需要传this.setData({contractObj: JSON.stringify({'initiatorCompanyId':initiatorCompanyId})})
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->

<template>
  <div class="custom-container">
    <!-- <template v-if="printRead"> -->
      <!-- <div>123</div> -->
      <!-- <span>{{ JSON.parse(currentInfoObj).name || '' }}</span> -->
    <!-- </template> -->

    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoObj" @selectFlow="selectFlow" :fieldSelectType="fieldSelectType">
      <!-- <el-input v-if="JSON.parse(currentInfoObj).name" readonly v-model="scope.viewName" :disabled="disabled" @focus="openInfoDialog" style="width:100%;padding-left: 5px;">
        <el-button slot="append" icon="el-icon-search" v-if="fieldType == 'flow'" @click="checkDetail"></el-button>
      </el-input> 
      <el-button v-else type="primary" :disabled="disabled" @click="openInfoDialog" style="margin-left:5px;">{{ placeholder }}</el-button> -->

      <template scope="scope">
        <span v-if="printRead" class="print-read-label">
          <!-- <span>333</span> -->
          <span>{{ JSON.parse(currentInfoObj).name }}</span>
          <!-- <span>{{ JSON.parse(currentInfoObj) }}</span> -->
        </span>

        <template v-else>
          <el-input v-if="JSON.parse(currentInfoObj).name" readonly v-model="scope.viewName" :disabled="disabled" @focus="openInfoDialog" style="width:100%;padding-left: 5px;">
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
import mixin from './mixin.js';
import MySendFlowList from './MySendFlowList.vue';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';

export default {
  name: 'GeneralListSelectShow',
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
      type: [Boolean], // [Array, String, Number]
      default() {
        return false;
      }
    },
    printRead: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return true;
        // return false;
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
    // console.log('this',this)
    console.log('printRead',this.printRead)
    console.log('this.fieldSelectType1',this.fieldSelectType)
   },
  watch: {
    value(val) {
      console.log('val123-通用列表选择',val)
      this.currentInfoObj = val
      // this.currentInfoId = val
    },
    placeholder(val) {
      console.log('======placeholder======',val)
    },
    currentInfoObj(val){
      console.log('=======1-通用列表选择',val)
      this.$emit('input', val)
    },
    disabled(val){
      console.log('====disabled===',this.disabled)
    },
    printRead(val){
      console.log('====printRead22===',this.printRead)
    },
  },
  computed: {
    fieldType(){
      console.log('this.fieldSelectType',this.fieldSelectType)
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
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
      // console.log(111)
      console.log(111,this.currentInfoObj)
      let row = JSON.parse(JSON.parse(this.currentInfoObj).rowData);
      this.previewHandle(row, false);
    },
    openInfoDialog(data){
      console.log('openInfoDialog-data',this.currentInfoObj)
      let selectCompanyId = this.currentInfoObj != '' && JSON.parse(this.currentInfoObj).selectCompanyId ? JSON.parse(this.currentInfoObj).selectCompanyId : '';
      console.log('selectCompanyId',selectCompanyId)
      console.log('this.currentInfoObj.formType',JSON.parse(this.currentInfoObj).formType)
      if (JSON.parse(this.currentInfoObj) && JSON.parse(this.currentInfoObj).formType == 'contract_receipt_form') {
        if (selectCompanyId) {
          this.flowListVisible = true;
        } else {
          this.$message.warning('请先选择收款单位！');
        }
      } else {
        this.flowListVisible = true;
      }
    },
    selectFlow(data) { // 选择赋值
      console.log('selectFlow-data',data)
      this.currentInfoObj = data;
      this.flowListVisible = false;

      // 添加表单校验
      this.$nextTick(() => { 
        this.$parent.$parent.validate()
      })
    },
    test(){
      console.log(123)
    }
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