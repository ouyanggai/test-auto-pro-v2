<template>
    <!-- 个性化设置弹窗 -->
    <el-dialog :visible="visible" title="个性化设置" :close-on-click-modal="false" width="800px" @close='close' append-to-body @open="open">
      <div style="padding:25px;" >
        <!-- <el-radio-group v-model="flowChoosed">
          <el-radio v-for="item in [{type:'workTarget',name:'工作指标'},{type:'manageTarget',name:'管理指标'},]" :label="item.type" >{{ item.name }}</el-radio>
        </el-radio-group> -->
        <div style="display:flex;margin-bottom:10px;">
          <div style="margin-right:10px;">是否启用个性化设置</div>
          <el-switch
            v-model="enableType">
          </el-switch>
        </div>
        <draggable v-model="items" :animation="200" v-show="enableType">
          <div v-for="(val,index) in items" :key="index" class="drag-div">
            <div class="main-drag">拖动调整顺序</div>
            <div class="main-name">{{val.name}}</div>
            <div style="display:flex;align-items:center;width:160px;">
              <div style="margin-right:10px;">是否显示</div>
              <el-switch
                v-model="val.enableType"
                >
              </el-switch>
            </div>
          </div>
        </draggable>
      </div>
      <span slot="footer">
        <el-button @click="close">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>
</template>

<script>
import Api from '@/api';
import draggable from 'vuedraggable'
import {
  localstorageGet
} from '@/utils/auth';
import {
  deepClone
} from '@/utils';
export default {
  name:'UserSetDialog',
  components: {draggable},
  props: {
    visible:{
      type:Boolean,
      default:false
    },
    userComponentSetList:{
      type:Array,
      default(){
        return []
      }
    },
    userSetEnableType:{
      type:Boolean,
      default:false
    },
    id:{
      type:String,
      default:''
    }
  },
  data() {
    return {
      enableType:false,
      items:[]
    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {
    resourceId() {
      return this.$store.state.app.resourceId;
    },
  },
  methods: {
    open(){
      this.enableType = this.userSetEnableType
      this.items = deepClone(this.userComponentSetList)
      // console.log('resourceId',this.resourceId)
      // this.findUserSet()
    },
    close(){
      this.$emit('update:visible',false)
    },
    confirm(){
      let userComponentSetList = this.items.map(item=>{
        item.enableType ? item.enableType = 'enable' : item.enableType = 'disable'
        return item
      })
      let resourceId = this.resourceId
      let resourceName = this.resourceName
      let userId = localstorageGet('userId')
      let enableType = this.enableType ? 'enable' : 'disable'
      let data = {
        resourceId,
        resourceName,//:"我的工作台设置",
        userId,
        enableType,//:"enable",//启用禁用enable,disable
        userComponentSetList
      }
      if(this.id)data.id = this.id
      this.saveSet(data).then(res=>{
        if(res.isSuccess){

        }
        this.$emit('confirmSet')
        this.close()
      })
    },
    saveSet(data){
      return this.$axios.post(Api.user.saveUserSet,{data})
    }
  },
};
</script>
<style lang="scss" scoped>
.drag-div{
  width:100%;
  border:1px solid rgb(196,196,196);
  border-radius:5px;
  margin-top:10px;
  margin-bottom:5px;
  display:flex;
  align-items:center;
  padding-right:8px;
  background:rgb(235,235,235);
  overflow:hidden;
  height:40px;
}
.main-drag{
  background:rgb(194,194,194);
  color:rgb(35,35,35);
  width:140px;
  padding:0px 4px;
  cursor:move;
  height:100%;
  line-height: 38px;
  border-top-right-radius:5px;
  border-bottom-right-radius:5px;
}
.main-name{
  width:100%;
  text-align:center;
  padding:2px 0;
}
</style>
