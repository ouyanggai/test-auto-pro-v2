<!--
 * @Descripttion: 已提-查看流程(暂时不用)
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div class="dialog-container">
    <el-dialog custom-class="dialog-fullscreen" :visible="visible" center append-to-body @close='handleClose'>
      <!-- 费用报销单 -->
      <ExpensesClaimForm v-if="selectFlowType == 'expense_reimbursement'" />
      <!-- 公司预算金额调剂申请单 -->
      <CompanyAmountAdjustForm v-if="selectFlowType == 'expense_budget'" type='check' />
      <span slot="footer" class="dialog-footer">
        <template>
          <el-button type="primary" @click="handleClose">关闭</el-button>
        </template>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
// import ExpensesClaimForm from './Form/ExpensesClaimForm.vue';
import CompanyAmountAdjustForm from './Form/CompanyAmountAdjustForm.vue';
import Api from '@/api';

export default {
  name: '',
  components: { DyTable, CompanyAmountAdjustForm },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    selectFlowType: {
      type: String,
      default: ''
    }
  },
  data() {
    const vm = this;
    return {
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
    handleClose() {
      this.$emit('update:visible', false);
      this.editData = {};
    }
  }
};
</script>

<style lang="scss" scoped></style>
<style lang="scss">
// @import "@/assets/styles/formMaking.scss";
</style>
