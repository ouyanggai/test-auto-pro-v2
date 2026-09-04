<!--
 * @Descripttion: 流程列表选择后展示
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->

<template>
  <div class="custom-container">
    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoId" @selectFlow="selectFlow">
      <template>
          <el-button type="primary" :disabled="disabled" @click="openInfoDialog">{{ placeholder }}</el-button>
      </template>
    </MySendFlowList>
  </div>
</template>
<script>
import MySendFlowList from './MySendFlowList.vue';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: 'CustomeSelect',
  components: {
    MySendFlowList
  },
  props: {
    value: {
      type: String,
      default: ''
    },
    disabled: {
      type: Boolean,
      default: false
    },
    placeholder: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      currentInfoId: this.value,
      flowListVisible: false,
      currentData: []
    };
  },
  created() {
  },
  mounted() {
    // this.$emit('focus')
    // console.log('this',this)
    // console.log('this.value',this.value)
  },
  watch: {
    value(val) {
      // console.log('val123',val)
      this.currentInfoId = val;
    },
    placeholder(val) {
      console.log('======placeholder======', val);
    },
    currentInfoId(val) {
      // console.log('=======1',val)
      this.$emit('input', val);
    }
  },
  computed: {},
  methods: {
    openInfoDialog(data) {
      if (this.disabled) return;
      // form表达组件
      const doc = document.getElementsByClassName('form-container');
      if (doc[0]) {
        const values = doc[0].__vue__.$refs.generateForm.getValues();
        console.log('values', values);
        if (values.travelPersonnelVoList) { // 差旅报销
          if (!values.expenseCompanyId) {
            return this.$message.warning('请先选择报销单位！');
          }
        } else if (values.RequestPayoutList) { // 请款单
          const companyId = parseJsonObject(values.myCompanyName).id || '';
          console.log('companyId', companyId);
          if (!companyId) {
            console.log('111111111');
            return this.$message.warning('请先选择收款单位！');
          }
        }
      }

      // else{
      //   if(!values.applicationFundsVo_payCompanyId){ //还款单
      //   return this.$message.warning('请先选择收款单位！')
      //   }
      // }

      this.flowListVisible = true;
    },
    selectFlow(data) { // 选择赋值
      // console.log('selectFlow',data)
      // console.log('this',this)
      this.currentData = data;
      this.flowListVisible = false;
      console.log(this, '[8888888888]');
      this.$emit('onChange', data);
    },
    test() {
      console.log(123);
    }
  }
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
