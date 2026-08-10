<!--
 * @Author: 暂时不用这个组件
 * @Date: 2025-04-17 11:53:28
 * @LastEditors: Please set LastEditors
 * @LastEditTime: 2025-03-12 09:53:28
 * @Description: file content
-->
<template>
  <div>
    <el-dialog title="转发" :visible="dialogVisible" width="500px" :top="'50px'" append-to-body
      :close-on-click-modal="false" :before-close="handleClose">
      <el-form
        :model="approveForm"
        :rules="rules"
        label-width="80px"
        ref="approveForm"
      >
        <el-form-item label="选择人员" prop="personName">
          <el-input
            v-model="approveForm.personName"
            placeholder="请选择人员"
            readonly
            @focus="openTranspondPerson"
          ></el-input>
        </el-form-item>
        <el-form-item label="附言" prop="approveMessage">
          <el-input
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 7 }"
            maxlength="200"
            show-word-limit
            v-model="approveForm.approveMessage"
            placeholder="请填写附言"
          ></el-input>
        </el-form-item>
        <el-form-item label="" v-if="showSelectGroup">
          <el-checkbox-group v-model="approveForm.checkList">
            <el-checkbox label="fuyan">转发原附言</el-checkbox>
            <el-checkbox label="yijian">转发原意见</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirmTranspond">确 定</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import NodeWrap from '../CheckFlow/index.vue';
export default {
  name: '',
  components: { NodeWrap },
  data() {
    return {
      nodeConfig: {},
      already: false,
      companyPersonList: [],
      auditRecordNodeList: [],
      nodeConfigUser:{} //节点的审批人对象，包含自选人列表
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
    }
  },
  watch: {},
  created() { },
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
    // 获取该条在途流程的流程详情
    getDetailNode() {
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
                auditStatus: item.auditStatus
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
  margin: 20px auto;

  .el-dialog__body {
    height: 720px;
  }
}

.dingflow-design {
  width: 100%;
  height: 660px;
  background: #fff;
  margin: 0;
  overflow: auto;
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
