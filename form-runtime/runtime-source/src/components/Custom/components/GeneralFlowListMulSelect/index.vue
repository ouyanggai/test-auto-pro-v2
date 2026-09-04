<!--
 * @Descripttion: 通用流程列表选择后展示（选择多个数据）（参考调休单关联多个加班流程） 
 * 需要在refresh中设置具体流程类型，比如this.setData({'flowObj':JSON.stringify({'flowType': 'extra_work_register'})})
 * @Author: zhengzetao
 * @Date: 2024-11-21
-->

<template>
  <div class="custom-container">

    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoObj" @selectFlow="selectFlow" :fieldSelectType="fieldSelectType">

      <template scope="scope">
        <div v-if="printRead" class="print-read-label">
          <div v-for="(item,index) in dataShowList" :key="index">{{ item.name }}</div>
        </div>

        <template v-else>
          <div v-if="dataShowList.length">
            <!-- <div v-for="(item,index) in dataShowList" :key="index">
              <span style="cursor:pointer" title="打开流程列表" @click="openInfoDialog">{{ item.name }}</span>
              <i class="el-icon-search" title="查看流程详情" @click="checkDetail(item)" style="cursor:pointer;margin-left: 10px;"></i>
            </div> -->
            <div v-for="(item,index) in dataShowList" :key="index">
              <el-input readonly :disabled="disabled" v-model="item.name" @focus="openInfoDialog" style="width:100%;padding-left: 5px;">
                <el-button slot="append" icon="el-icon-search" @click="checkDetail(item)"></el-button>
              </el-input>
            </div>
          </div>
          <el-button v-else type="primary" :disabled="disabled" @click="openInfoDialog" style="margin-left:5px;">{{ placeholder }}</el-button>
        </template>
      </template>
    </MySendFlowList>

    <!-- 流程-查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId" 
    :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" />

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
import { parseJsonArray, parseJsonObject } from '@/utils/parse-value';

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
        'flow': {
          name:'流程',
        },
      },
      btnVisible:false,
      dataShowList:[]
    };
  },
  created() {
  },
  mounted() {
    console.log('this',this)
    // console.log('printRead',this.printRead)
    // console.log('this.fieldSelectType1',this.fieldSelectType)
    // console.log('this.currentInfoObj',this.currentInfoObj)
    this.dataShowList = parseJsonArray(parseJsonObject(this.currentInfoObj).flowList);
  },
  watch: {
    value(val) {
      console.log('val-流程多选-传值',val)
      this.currentInfoObj = val;
      this.dataShowList = parseJsonArray(parseJsonObject(val).flowList);
      // this.currentInfoId = val
    },
    placeholder(val) {
      // console.log('======placeholder======',val)
    },
    currentInfoObj(val){
      console.log('emit-流程多选-控件赋值',val)
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
    checkDetail(item){
      console.log('checkDetail',item)
      if (item.rowData) {
        const row = parseJsonObject(item.rowData)
        if (Object.keys(row).length) this.previewHandle(row, false)
      }
    },
    openInfoDialog(data){
      this.flowListVisible = true;
    },
    selectFlow(data) { // 选择赋值
      console.log('selectFlow-data',data)
      this.currentInfoObj = data;
      this.dataShowList = parseJsonArray(parseJsonObject(data).flowList);
      this.flowListVisible = false;

      // 添加表单校验
      this.$nextTick(() => { 
        this.$parent.$parent.validate()
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
