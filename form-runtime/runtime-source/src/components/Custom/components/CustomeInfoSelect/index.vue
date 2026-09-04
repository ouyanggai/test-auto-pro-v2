<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-15 18:41:45
-->
<!-- 经办公司、经办部门、岗位、人员选择 -->
<!-- 表单控件的字段名需要一一对上：companyName、depName、dutyName、userName -->
<!-- 一张表单有多个人员、部门、或者岗位选择，固定字段，人员：userName，部门：depName，公司：companyName，岗位：dutyName。
如果表单有多个人员、部门等选择，用人员举例，字段可以是newUserName、userNameComeOn、userName2等包含userName，不区分大小写，
选择人员的字段只要包含了userName就能进行组件使用，部门、公司和岗位同理。FormMaking有一个通用组件可以选人员、部门、公司、岗位，
并带出。（根据后端要求修改，现在改成以文字带出，不带出id） -->
<!-- printRead: 打印前的阅读模式 -->

<template>
  <div class="custom-container">
    <IndicatorHeaderDialog :visible.sync="indicatorHeaderVisible" v-model="currentInfoObj" @selectHeader="selectHeader" :fieldSelectType="fieldSelectType">
      <template scope="scope">
        <span v-if="printRead && currentInfoObj" class="print-read-label">
          <span>{{ parsedInfo.name }}</span>
        </span>
        <span v-else-if="printRead && !currentInfoObj" class="print-read-label">
          <span></span>
        </span>
        <el-input v-else readonly type="textarea" :autosize="true" :resize="'none'" class="info-textarea" v-model="scope.viewName" :disabled="disabled" @focus="openInfoDialog" style="width:100%"></el-input>
        <!-- v-model动态回显名称 -->
      </template>
    </IndicatorHeaderDialog>
  </div>
</template>
<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import IndicatorHeaderDialog from './IndicatorHeaderDialog.vue';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: 'CustomeInfoSelect',
  components: {
    IndicatorHeaderDialog
  },
  props: {
    // value: {
    //   type: [Object], // [Array, String, Number]
    //   default() {
    //     return {};
    //   }
    // },
    value: {
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
        return false;
      }
    }
  },
  data() {
    return {
      currentInfoObj: this.value,
      // currentInfoId: this.value,
      indicatorHeaderVisible: false,
      fieldSelectType: this.$parent.modelName,
    };
  },
  created() {
  },
  mounted() {
    // this.$emit('focus')
    // console.log('this',this)
    // console.log('this.fieldSelectType',this.fieldSelectType)
    // console.log('this.value',this.value)
    // console.log('this.currentInfoObj',this.currentInfoObj)
  },
  watch: {
    value(val) {
      console.log('val123',val)
      this.currentInfoObj = val
      // this.currentInfoId = val
    },
    currentInfoObj(val){
      console.log('=======1',val)
      if (!val) {
        this.$parent.$set(this.$parent.dataModels, this.$parent.modelName + '__condition', '');
        this.$parent.$set(this.$parent.dataModels, this.$parent.modelName + '__formPersonId', '');
        this.$emit('input', val);
        return;
      }
      try {
        const parsed = parseJsonObject(val);
        // 添加后缀'__condition'的虚拟字段，并赋值选中的name
        this.$parent.$set(this.$parent.dataModels, this.$parent.modelName + '__condition', parsed.name);
        this.$parent.$set(this.$parent.dataModels, this.$parent.modelName + '__formPersonId', parsed.id);
      } catch (e) {
        console.error('JSON parse error:', e);
      }
      // this.setData({'get':123})
      this.$emit('input', val)
    },
    // currentInfoId(val){
    //   // console.log('=======1',val)
    //   this.$emit('input', val)
    // },
    disabled(val){
      console.log('====disabled===',this.disabled)
    },
    // printRead(val){
    //   console.log('====printRead===',this.printRead)
    // },
  },
  computed: {
    // parsedInfo 为展示层提供稳定对象，空值和异常历史值都按未选择处理。
    parsedInfo () {
      return parseJsonObject(this.currentInfoObj)
    }
  },
  methods: {
    openInfoDialog(data){
      this.indicatorHeaderVisible = true;

    },
    selectHeader(data) { // 选择赋值
      console.log('selectHeader',data)
      // console.log('this',this)
      this.currentInfoObj = data;
      // this.currentInfoId = data.name;
      this.indicatorHeaderVisible = false;
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
