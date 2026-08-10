<!-- 扩展属性选择 -->
<template>
    <el-dialog :visible="visible" title="选择人员" :close-on-click-modal="false" width="300px" @close='closeChoose'
      class="examiner-dialog" append-to-body>
      <div style="padding:25px;">
        <el-radio-group v-model="choose">
          <el-radio v-for="item in personList" :label="item.id" >{{ item.name }}</el-radio>
        </el-radio-group>
      </div>
      <span slot="footer">
        <el-button @click="closeChoose">取 消</el-button>
        <el-button type="primary" @click="confirmChoose">确 定</el-button>
      </span>
    </el-dialog>
</template>

<script>
export default {
  name:'ExtendedAttributeUsers',
  components: {},
  props: {
    visible:{
      type:Boolean,
      default:false
    },
    personList:{
      type:Array,
      default:()=>{
        return []
      }
    },
    nextNodeProxyId:{
      type:String,
      default:''
    }
  },
  data() {
    return {
      choose:''
    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {},
  methods: {
    closeChoose(){
      this.choose = ''
      this.$emit('update:visible',false)
    },
    confirmChoose(){
      if(this.choose){
        // console.log('this.choose',this.choose)
        let data = this.personList.find(el=>el.id == this.choose)
        if(data){
          if(this.nextNodeProxyId)data.nodeProxyId = this.nextNodeProxyId
          this.$emit('confirmChooseExtendPerson',data)
        }
      }else{
        this.$message.error('请选择审核人员')
      }
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
