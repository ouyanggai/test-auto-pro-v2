<!--
 * @Author: junshao
 * @Date: 2023-04-17 11:53:28
 * @LastEditors: Please set LastEditors
 * @LastEditTime: 2025-10-27 16:15:17
 * @Description: file content
-->
<template>
  <div>
    <el-dialog title="查看流程" :visible="dialogVisible" append-to-body center fullscreen
      :close-on-click-modal="false" :before-close="handleClose"><!-- width="98%" :top="'20px'" -->
      <DragZoomScroll>
      <section class="dingflow-design">
        <div
          v-if="already"
          class="box-scale"
          id="box-scale"
        >
          <NodeWrap
            :nodeConfig.sync="nodeConfig"
            :auditRecordNodeList="auditRecordNodeList"
            :isDraft="isDraft"
            :companyPersonList.sync="companyPersonList"
            :nextNodeProxyId="nextNodeProxyId"
            :editType="3"
            :initiatorId="initiatorId"
            :flowInstanceId="flowInstanceId"
            :nodeConfigUser="nodeConfigUser"
            :formData="formData"
            :companyId="companyId"
          ></NodeWrap>
          <div class="end-node">
            <div class="end-node-circle"></div>
            <div class="end-node-text">流程结束</div>
          </div>
        </div>
      </section>
    </DragZoomScroll>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleClose">关 闭</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import NodeWrap from '../CheckFlow/index.vue';
import DragZoomScroll from '@/components/DragZoomScroll';
export default {
  name: '',
  components: { NodeWrap, DragZoomScroll },
  data() {
    return {
      nodeConfig: {},
      already: false,
      companyPersonList: [],
      auditRecordNodeList: [],
      nodeConfigUser:{}, //节点的审批人对象，包含自选人列表
      formData:{}
    };
  },
  computed: {},
  props: {
    dialogVisible: {
      type: Boolean,
      default: false
    },
    isDraft: { // 从待发传true过来 发起人标识不显示‘已发送’
      type: Boolean,
      default: false
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
    flowId: {
      type: String,
      default: ''
    },
    //下一个节点得id，用于标识下一节点
    nextNodeProxyId:{
      type: String,
      default: ''
    },
    initiatorId:{
      type:String,
      default:''
    },
    companyId:{
      type:String,
      default:''
    },
  },
  watch: {},
  async created() { 
    if (this.flowInstanceId) {
      this.formData  = await this.getFormData();
    }
  },
  mounted() {
    this.getDetailNode();
    this.getCompanyPersonList();
    // 检测宽高，超出后可移动
    this.queryCurrentProcessor()
  },
  methods: {
    // checkRect() {
    //   this.$nextTick(() => {
    //     var section = document.querySelector('.dingflow-design');// .scrollLeft=10

    //     var boxScale = document.querySelector('#box-scale');

    //     if (boxScale.clientWidth > section.clientWidth || boxScale.clientHeight > section.clientHeight) {
    //       boxScale.style.cursor = 'move';
    //       var startX; var startY; var startPosX = 0; var startPosY = 0; var mx = 0; var my = 0;
    //       // console.log(boxScale.clientWidth - section.clientWidth)
    //       var gapX = section.clientWidth - boxScale.clientWidth;
    //       var gapY = section.clientHeight - boxScale.clientHeight;
    //       boxScale.onmousedown = (e) => {
    //         e.preventDefault();
    //         startX = e.clientX;
    //         startY = e.clientY;
    //         startPosX = mx;
    //         startPosY = my;
    //         boxScale.onmousemove = (ev) => {
    //           var moveX = ev.clientX;
    //           var moveY = ev.clientY;
    //           mx = moveX - startX + startPosX;
    //           my = moveY - startY + startPosY;
    //           if (mx < gapX) mx = gapX;
    //           mx = mx > 0 ? 0 : mx;
    //           if (my < gapY) my = gapY;
    //           my = my > 0 ? 0 : my;
    //           boxScale.style.transform = `translate(${mx}px,${my}px)`;
    //         };
    //         boxScale.onmouseup = () => {
    //           boxScale.onmousemove = null;
    //           boxScale.onmouseup = null;
    //         };
    //       };
    //       // section.addEventListener('mousedown',(e)=>{
    //       //   console.log(e)
    //       // })
    //     }
    //   });
    // },
    // 获取表单字段值
    getFormData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id: this.flowInstanceId
            },
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data.data);
            }
          }
        );
      });
    },
    // 获取该条在途流程的流程详情
    getDetailNode() {
      console.log('getDetailNode-this.flowInstanceId',this.flowInstanceId)
      const url = this.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      this.$axios.post(
        url,
        {
          data: {
            id: this.flowId
          }
        },
        res => {
          if (res.isSuccess) {
            this.already = true;
            console.log('res.data',res.data)
            this.nodeConfig = res.data.flowNodeTemplate;
            if (this.flowInstanceId) {
              this.getAuditRecord();
            }
            // this.checkRect();
          }
        }
      );
    },
    getAuditRecord() {
      this.$axios.post(
        Api.approveManage.findRecord,
        {
          data: {
            flowInstanceId: this.flowInstanceId
          }
        },
        res => {
          if (res.isSuccess) {
            this.auditRecordNodeList = res.data.map(item => {
              return {
                flowNodeProxyId: item.flowNodeProxyId,
                auditStatus: item.auditStatus,
                batchNo: item.batchNo,
                createDate: item.createDate
              };
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 用于人员回显
    getCompanyPersonList() {
      const param = {
        data: {
          companyId: this.$store.state.user.companyId
        },
        pagination: true,
        pages: 1,
        size: 1000
      };
      this.$axios.post(
        Api.user.findByCompanyIdUserList,
        param,
        res => {
          this.loading = false;
          if (res.isSuccess) {
            this.companyPersonList = res.data.dataList;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    //根据流程实例id获取流程下信息
    queryCurrentProcessor(){
      this.$axios.post(
        Api.approveManage.queryCurrentProcessor,
        {
          data: {},
          flowInstanceIds:[this.flowInstanceId],
        },
        res => {
          if (res.isSuccess) {
            let data = res.data
            let obj = data[this.flowInstanceId] || {}
            this.nodeConfigUser = obj
            console.log('queryCurrentProcessor',obj)
            // this.auditRecordNodeList = res.data.map(item => {
            //   return {
            //     flowNodeProxyId: item.flowNodeProxyId,
            //     auditStatus: item.auditStatus
            //   };
            // });
          } else {
            this.$message.error(res.message);
          }
        })
    },
    handleClose() {
      this.$emit('update:dialogVisible', false);
    }
  }
};

</script>
<style lang='scss' scoped>
@import "~@/assets/styles/workflow.scss";

::v-deep .el-dialog {
  // margin: 20px auto;
  .el-dialog__body {
    height: calc(100vh - 90px); // 750px;
    max-height: unset;
    min-height: unset;
    padding: 0
  }
  .el-dialog__footer{
    padding: 0;
  }
  .el-dialog__header{
    padding: 10px;
  }
}

.dingflow-design {
  // width: 100%;
  width:auto;
  // height: 750px;
  background: #fff;
  margin: 0;
  // overflow: auto;
  overflow: visible !important;
}

.dingflow-design .box-scale {
  padding: 20px 0;
}

.dingflow-design .branch-box .col-box {
  background: #fff;
  // background: #f5f5f7;
}

.bottom-left-cover-line,
.bottom-right-cover-line,
.top-left-cover-line,
.top-right-cover-line {
  background-color: #fff;
}
</style>
