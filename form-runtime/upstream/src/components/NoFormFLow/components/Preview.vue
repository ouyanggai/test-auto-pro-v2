<template>
  <el-dialog
    title="详情"
    :visible="visible"
    custom-class="dialog-fullscreen"
    center
    append-to-body
    :close-on-click-modal="false"
    @close='handleClose'>
    <noForm
        v-if="visible"
        operaType="preview"
        :currentComponent="formName"
        :isReInitiate="false"
        :otherBizId="bizId"
    ></noForm>
  </el-dialog>
</template>

<script>
import noForm from '@/views/ApproveManage/components/NoForm'
export default {
  name:'',
  components: {noForm},
  props: {
    visible:{
      type:Boolean,
      default:false
    },
    bizId:{
      type:String,
      default:''
    },
    formName:{
      type:String,
      default:''
    }
  },
  data() {
    return {

    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {},
  methods: {
    handleClose(){
      this.$emit("update:visible",false)
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
