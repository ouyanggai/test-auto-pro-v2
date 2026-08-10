<!--
 * @Descripttion: 编辑流程弹窗
 * @Author: zhengzetao
 * @Date: 2025-03-18
-->
<template>
  <!-- <div class="dialog-container"> -->
    <el-dialog :visible="visible" center append-to-body @close='handleClose' :fullscreen="true">
      <NewEventFlowEdit @confirm="confirm" :type="type" :originalNodeConfig="originalNodeConfig" :flowProxyId="flowProxyId"></NewEventFlowEdit>
      <!-- <span slot="footer" class="dialog-footer">
        <template>
          <el-button type="primary" @click="handleClose">关闭</el-button>
        </template>
      </span> -->
    </el-dialog>
  <!-- </div> -->
</template>

<script>
import Api from '@/api';
import NewEventFlowEdit from './NoFormMulBranch/index.vue';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils/index';

export default {
  name: '',
  components: {NewEventFlowEdit },
  props: {
    visible: {
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
    }
  },
  data() {
    return {
    };
  },
  computed: {
  },
  watch: {
  },
  created() {
    console.log('originalNodeConfig2',this.originalNodeConfig)
  },
  mounted() {
  },
  methods: {
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
  
</style>
