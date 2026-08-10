<template>
  <!-- 在线编辑(查看) -->
  <el-dialog
    title=""
    :visible.sync="dialogVisible"
    :append-to-body="true"
    :close-on-click-modal="false"
    @close='handleClose'
    :fullscreen="true">
    <div v-if="dialogVisible" style="height: 88vh;">
    <!-- <div style="height:80vh;" v-if="dialogVisible"> -->
      <officeExcel :iframeOrigin="iframeOrigin" :fileUrl="wordFileUrl" :callbackUrl="callbackUrl" :mode="mode" :documentType="documentType"
        :title="excelTitle" :token="myToken" :user="user" :excelKey="excelKey" ref="officeExcelRef">
      </officeExcel>
    </div>
  </el-dialog>
</template>

<script>
import { baseUrl,onlyOfficeUrl } from '@/config/env';
import officeExcel from '@/components/OfficeExcel'
import { generateRandomId,generateUid,findPathById } from "@/utils";
import { localstorageGet,localstorageSet } from '@/utils/auth';

export default {
  name: '',
  components: {officeExcel},
  props:{
    excelVisible:{
      type:Boolean,
      default:false
    },
    wordFileUrl:{
      type:String,
      default:''
    }
  },
  // watch: {
  //   // excelVisible(newVal, oldVal) {
  //   //   console.log('打开1')
  //   //   // console.log(`状态变化: ${oldVal} → ${newVal}`);
  //   //   // if(newVal) {
  //   //   //   console.log('打开1')
  //   //   // } else {
  //   //   //   console.log('打开2')
  //   //   // }
  //   // }
  //   excelVisible: {
  //     handler(newVal) {
  //       console.log('打开1')
  //       // this.dialogVisible = true;
  //       // if(newVal) {
  //       //   console.log('打开1')
  //       // } else {
  //       //   console.log('打开2')
  //       // }
  //     },
  //     // immediate: true // 初始立即执行
  //   },
  // },
  data () {
    return {
      dialogVisible:false,
      documentType:'word',
      // fileUrl: '',
      callbackUrl: '',
      // iframeOrigin: `http://192.168.1.195:1080/web-apps/apps/`,
      iframeOrigin: `${onlyOfficeUrl}/web-apps/apps/`,
      myToken: '',
      user: {},
      excelKey: '',
      mode:'view', // view
      excelTitle:'只读模式',
    };
  },
  computed: {},
  created() {
    console.log(1111)
    // console.log('excelVisible',this.excelVisible)
    this.openView();
    this.dialogVisible = true;
  },
  mounted() {},
  methods: {
    openView(){
      console.log('openView')

      console.log(1)
      let bindRandomId = generateUid();
      // this.fileUrl = url;
      this.newFileName = 'aaa'
      this.mode = 'view'
      this.myToken = generateRandomId(32)
      this.excelKey = generateRandomId(24)
      let sid = localstorageGet('token')
      console.log(2)
      // let sid = 'ab9e4002-940a-4962-a511-8ad8f76c5851'
      // this.user = {
      //   name: '郑泽涛',
      //   id: 'b3b514aa6114416bbdbf46ebc247283e',
      // }
      this.user = {
        name: localstorageGet('userName') || '',
        id: localstorageGet('userId') || '',
      }

      console.log(3)
      // fileName需要传给后端，因为有文件后缀，后端根据后缀返回文件格式
      this.callbackUrl = `${baseUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${bindRandomId}&fileName=${this.newFileName}`
      console.log('this.callbackUrl',this.callbackUrl)
      // this.$nextTick(() => {
      //   this.excelVisible = true
      // })
    },
    handleClose() {
      console.log('handleClose')
      this.$emit('update:dialogVisible', false);
      this.$emit('update:excelVisible', false);
    },
  }
}

</script>
<style lang='scss' scoped>
  ::v-deep {
    .el-dialog.is-fullscreen .el-dialog__body{
      min-height: 95vh;
    }
  }
</style>