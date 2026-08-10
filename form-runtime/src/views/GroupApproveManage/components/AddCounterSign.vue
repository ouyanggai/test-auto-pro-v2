<!--
 * @Descripttion: 加签弹窗
 * @Author: zhengzetao
 * @Date: 2025-06-13
-->
<template>
  <el-dialog :visible="visible" center append-to-body @close='handleClose' :fullscreen="true">
    <NoFormMulBranch v-if="isNoForm && Object.keys(outFlowNodeTemplate).length" :outFlowNodeTemplate="outFlowNodeTemplate" :flowNodeProxyId="newFlowNodeProxyId" :flowInstanceId="flowInstanceId"
    @saveSuccess="confirm"></NoFormMulBranch>
    <FormMulBranch v-if="!isNoForm && Object.keys(outFlowNodeTemplate).length" :outFlowNodeTemplate="outFlowNodeTemplate" :flowNodeProxyId="newFlowNodeProxyId" :flowInstanceId="flowInstanceId"
    @saveSuccess="confirm"></FormMulBranch>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import FormMulBranch from './FormMulBranch/index.vue';
import NoFormMulBranch from './NoFormMulBranch/index.vue';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils/index';

export default {
  name: '',
  components: {FormMulBranch,NoFormMulBranch },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    isNoForm: {
      type: Boolean,
      default: false
    },
    type: {
      type: String,
      default: 'add'
    },
    flowProxyId: {
      // 流程id
      type: String,
      default: ''
    },
    originalNodeConfig:{
      type:Object,
      default:function(){
        return {};
      }
    },
    // outFlowNodeTemplate:{
    //   type:Object,
    //   default:function(){
    //     return {};
    //   }
    // },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    tFlowNodeProxyId: { // 当前节点id
      type: String,
      default: ''
    },
    tFlowProxyId: { // 流程id
      type: String,
      default: ''
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
  },
  data() {
    return {
      outFlowNodeTemplate:{},
      newFlowNodeProxyId:'',
      newFlowProxyId:'',
    };
  },
  computed: {
  },
  watch: {
  },
  created() {
    this.newFlowNodeProxyId = this.tFlowNodeProxyId ? this.tFlowNodeProxyId : this.flowNodeProxyId;
    this.newFlowProxyId = this.tFlowProxyId ? this.tFlowProxyId : this.flowProxyId;
    console.log('this.tFlowProxyId',this.tFlowProxyId)
    console.log('this.flowProxyId',this.flowProxyId)
    console.log('this.tFlowNodeProxyId',this.tFlowNodeProxyId)
    console.log('this.flowNodeProxyId',this.flowNodeProxyId)
    console.log('this.newFlowProxyId',this.newFlowProxyId)
    this.getFlowDetailFindFormPerson()
  },
  mounted() {
  },
  methods: {
    getFlowDetailFindFormPerson() { // 获取下个节点表单人员字段List
      console.log('getFlowDetailFindFormPerson-方法2')
      // console.log('this.$attrs.clickRow-test',this.$attrs.clickRow)
      // this.$attrs.clickRow.flowInstanceId
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      console.log('getFlowDetailFindFormPerson-url',url)
      // this.$attrs.clickRow.flowProxyId
      this.$axios.post(url, { data: { id: this.newFlowProxyId }},
        res => {
          if (res.isSuccess) {
            this.outFlowNodeTemplate = res.data;
            console.log('this.outFlowNodeTemplate2',this.outFlowNodeTemplate)
          }
        }
      );
    },
    confirm(data){
      this.$emit('confirm',data)
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style lang="scss" scoped>
  ::v-deep {
    .el-dialog.is-fullscreen {
      overflow: hidden;
    }
    .el-dialog.is-fullscreen .el-dialog__body {
      overflow-y: hidden;
    }
  }
</style>
