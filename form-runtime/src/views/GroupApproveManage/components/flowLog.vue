<!--
 * @Descripttion: 流程日志
 * @Author: zhengzetao
 * @Date: 2022-08-17
-->

<template>
  <div v-if="printStyle" class="flow-log-container" style="margin-top: 20px;">
    <div direction="vertical" style="color: 000;font-size:12px;">
      <div style="background:rgb(140,140,140);border: 1px solid rgb(140,140,140);border-bottom:none;padding:8px 10px;">流程日志</div>
      <div v-for="(val, index) in logTableData" :key="index"
        style="display: flex;padding:6px 10px;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
        <div style="margin-right:5px;width:80px;">{{ val.executorName }}</div>
        <div style="margin-right:5px;width:80px;">{{ val.auditStatus }} </div>
        <div style="margin-right:30px;">
          {{ val.createDate }}
        </div>
        <div style="display: flex;align-items: center;flex-wrap: wrap;">
          <div style="white-space: break-spaces;margin-right: 5px;">{{ val.executeDesc }}</div>
          <div v-if="val.attachmentList && val.attachmentList.length" class="node-attachment-list print-attachment-list">
            <div
              v-for="(file, fileIndex) in val.attachmentList"
              :key="file.id || file.fileId || `${index}-${fileIndex}`"
              class="node-attachment-item"
            >
              <i :class="file.fileName | fileIcon"></i>
              <span class="node-attachment-name">{{ file.fileName | shortName }}</span>
            </div>
          </div>
          <!-- <img :src="require('@/assets/images/'+val.device+'-icon.png')" class="log-icon" v-if="index > 0"> -->
        </div>
      </div>
    </div>
  </div>
  <div v-else class="flow-log-container">
    <el-card class="box-card mt-10 flowWrap" :class="{ 'flowLogContainer': isNoEnterprise }" shadow="never">
      <div slot="header" class="clearfix">
        <span>流程日志</span>
      </div>
        <el-steps direction="vertical" >
          <el-step  v-for="(val,index) in logTableData" :title="val.executorName">
            <template slot="title">
              <div style="display: flex;align-items: center;">
                <span>{{val.executorName}}</span>
                <img :src="require('@/assets/images/'+val.device+'-icon.png')" class="log-icon" v-if="index > 0" :title="val.device+'客户端'">
              </div>
            </template>
            <template slot="description">
              <div style="display: flex;align-items: center;">
                <div style="white-space: break-spaces;margin-right: 5px;">{{ val.executeDesc }}</div>
              </div>
              <div>
                {{ val.auditNodeName }}
                <span :style="{'color':iconList[val.status]['color']}">{{ val.auditStatus }} </span>
                <i :class="iconList[val.status]['icon']" :style="{'color':iconList[val.status]['color']}"></i>
              </div>
              <div>
                {{ val.createDate }}
              </div>
              <div v-if="val.attachmentList && val.attachmentList.length" class="node-attachment-list">
                <div class="node-attachment-title">附件：</div>
                <div
                  v-for="(file, fileIndex) in val.attachmentList"
                  :key="file.id || file.fileId || `${index}-${fileIndex}`"
                  class="node-attachment-item"
                >
                  <el-link type="primary" :underline="false" @click="viewAttachment(file)">
                    <i :class="file.fileName | fileIcon"></i>
                    <span class="node-attachment-name">{{ file.fileName | shortName }}</span>
                  </el-link>
                  <i class="el-icon-view" @click="viewAttachment(file)"></i>
                  <i class="el-icon-download" @click="downloadAttachment(file)"></i>
                </div>
              </div>
            </template>
          </el-step>
        </el-steps>
    </el-card>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { viewFile } from '@/utils';

const iconArr = ['png', 'jpg', 'jpeg', 'gif'];

export default {
  name: '',
  components: { DyTable },
  data() {
    return {
      logColKey: {
        auditNodeName: {
          label: '节点名称',
          width: '150px'
        },
        executorName: {
          label: '操作人',
          width: '100px'
        },
        createDate: {
          label: '操作时间',
          width: '200px'
        },
        auditStatus: {
          label: '操作状态',
          width: '100px'
        },
        executeDesc: {
          label: '操作描述',
          handle: (scope, createElement) => {
            return createElement('div', {
              style: {
                whiteSpace: 'pre-wrap!important'
              }
            }, scope.row.executeDesc);
          }
        }
      },
      iconList:{
        'pass':{
          color:'rgb(37,172,0)',
          icon:'el-icon-success'
        },
        'no_pass':{
          color:'rgb(231,56,46)',
          icon:'el-icon-error'
        },
        'transfer':{
          color:'rgb(231,129,46)',
          icon:'el-icon-refresh'
        },
        'retrieve':{
          color:'rgb(255,109,0)',
          icon:'el-icon-info'
        },
        'roll_back_the_previous_level':{
          color:'rgb(84,188,189)',
          icon:'el-icon-time'
        },
      }
      // logTableData: []
    };
  },
  props: {
    flowInstanceId: {
      type: String,
      default: ''
    },
    isNoEnterprise: {
      type: Boolean,
      default: true
    },
    printStyle: {
      type: Boolean,
      default: false
    },
    isTranspondFlow: {
      type: Boolean,
      default: false
    },
    logTableData: {
      type: Array,
      default: () => {
        return [];
      }
    },
  },
  computed: {},
  filters: {
    fileIcon(name) {
      if (!name) return 'el-icon-s-order';
      const lastIndex = name.lastIndexOf('.');
      const extendName = lastIndex > -1 ? name.substr(lastIndex + 1).toLowerCase() : '';
      let icon = 'el-icon-s-order';
      if (iconArr.indexOf(extendName) !== -1) icon = 'el-icon-picture';
      return icon;
    },
    shortName(name) {
      if (!name) return '';
      const lastIndex = name.lastIndexOf('.');
      if (lastIndex === -1) return name;
      const extendName = name.substr(lastIndex + 1);
      const nameWithoutExd = name.substring(0, lastIndex);
      let shortName = nameWithoutExd + '.' + extendName;
      if (nameWithoutExd.length >= 25) {
        const first = nameWithoutExd.substr(0, 12);
        const last = nameWithoutExd.substr(-12);
        shortName = first + '...' + last + '.' + extendName;
      }
      return shortName;
    }
  },
  watch: {
    logTableData: {
      handler(val) {
        if (val.length) {
          // 去除设备信息
          val.forEach(el => {
            if(!el?.device){
              let {executeDesc,device} = this.getDevice(el.executeDesc || '')
              el.executeDesc = executeDesc
              el.device = device
            }
          });
          this.loadNodeAttachments(val);
          this.$nextTick(()=>{
            //滚动到最底部
            var card = document.querySelector('.flow-log-container .el-card__body')
            if(card){
              let height = card.scrollHeight
              card.scrollTop = height
            }
          })
        }
      },
      immediate: true
    }
  },
  created() { },
  mounted() { },
  methods: {
    async loadNodeAttachments(list = []) {
      const relationIds = [...new Set(list.map(item => item.flowJobTaskId).filter(Boolean))];
      if (!relationIds.length) {
        list.forEach(item => {
          this.$set(item, 'attachmentList', []);
        });
        return;
      }
      const attachmentMap = {};
      await Promise.all(relationIds.map(async relationId => {
        attachmentMap[relationId] = await this.getAttachmentListByRelationId(relationId);
      }));
      list.forEach(item => {
        this.$set(item, 'attachmentList', attachmentMap[item.flowJobTaskId] || []);
      });
    },
    getAttachmentListByRelationId(relationId) {
      return this.$axios.post(
        Api.schedule.getAttachmentList,
        {
          data: {
            relationId
          }
        }
      ).then(res => {
        if (res.isSuccess) {
          const list = res.data || [];
          return list.map(item => ({
            id: item.fileId || item.id,
            fileId: item.fileId,
            fileName: item.fileName,
            fileUrl: item.fileUrl,
            absolutelyFileUrl: item.fileUrl
          }));
        }
        return [];
      }).catch(() => {
        return [];
      });
    },
    viewAttachment(file) {
      const url = file.absolutelyFileUrl || file.fileUrl;
      if (!url) return;
      viewFile(url);
    },
    downloadAttachment(file) {
      const href = file.absolutelyFileUrl || file.fileUrl;
      const name = file.fileName;
      if (!href || !name) return;
      const ele = document.createElement('a');
      ele.target = '_blank';
      fetch(href)
        .then(res => res.blob())
        .then(blob => {
          const url = window.URL.createObjectURL(new Blob([blob]));
          ele.href = url;
          ele.download = name;
          document.body.appendChild(ele);
          ele.click();
          document.body.removeChild(ele);
          window.URL.revokeObjectURL(url);
        });
    },
    getDevice(val){
      let reg = /\#\{(.+?)\}\#/g
      let find = val.match(reg)
      let executeDesc,device='web'
      if(find && find.length){
        executeDesc = val.replace(find[0],'')
        device = find[0].replace('#{', '').replace('}#', '')
      }else{
        executeDesc = val
      }
      return {executeDesc,device}
    },
    fetchNoData() {

    },
    // fetchLogData() {
    //   this.$axios.post(
    //     Api.approveManage.findRecord,
    //     {
    //       data: {
    //         flowInstanceId: this.flowInstanceId
    //       }
    //     },
    //     res => {
    //       if (res.isSuccess) {
    //         this.logTableData = res.data;
    //         this.logTableData.forEach(item => {
    //           this.translateStatus(item);
    //         });
    //       } else {
    //         this.$message.error(res.message);
    //       }
    //     }
    //   );
    // },
    // 已建任务和流程日志操作状态字符转换
    // translateStatus(obj) {
    //   let chnStatus;
    //   if (obj.auditStatus) {
    //     switch (obj.auditStatus) {
    //       case 'pass':
    //         chnStatus = '通过';
    //         break;

    //       case 'no_pass':
    //         chnStatus = '驳回';
    //         break;

    //       case 'withdraw':
    //         chnStatus = '撤销';
    //         break;

    //       default:
    //         chnStatus = '';
    //         break;
    //     }
    //   } else if (obj.flowStatus) {
    //     switch (obj.flowStatus) {
    //       case 'await_sent':
    //         chnStatus = '待发';
    //         break;

    //       case 'run':
    //         chnStatus = '运行中';
    //         break;

    //       case 'withdraw':
    //         chnStatus = '撤销';
    //         break;

    //       case 'termination':
    //         chnStatus = '终止';
    //         break;

    //       case 'rejected':
    //         chnStatus = '驳回';
    //         break;

    //       case 'end':
    //         chnStatus = '完结';
    //         break;

    //       default:
    //         chnStatus = '';
    //         break;
    //     };
    //   }
    //   obj.auditStatus = chnStatus;
    // }
  }
};

</script>
<style lang='scss' scoped>
$color:#333;
.flowWrap {
  margin: 0 auto;
  margin-top: 15px;
  height: calc(100% - 30px);
  overflow:auto;
}

.flowLogContainer {
  width: 100%;
  // height: calc(100% - 50px);
  overflow: hidden;
}
::v-deep .el-card__body{
  padding: 5px 15px 0px 15px;
  // overflow: auto;
  // height:calc(100% - 85px);
  .el-steps--vertical{
    height:auto;
  }
}
// @media screen and (min-width: 1300px) {
//   .flowLogContainer {
//     width: 80%;
//   }
// }

::v-deep .el-card__header {
  background-color: #f5f7fa;
  border-bottom: 1px solid #e4e7ed;
  padding: 5px 20px;
}
.flow-log-container{
  height: 86%;
  overflow: auto;
}
::v-deep {
  .el-step__head.is-wait{
    color: $color;
    border-color: $color;
  }
  .el-step__line{
    background-color: $color;
  }
  .el-card{
    color: $color;
  }
  .el-step__title.is-wait{
    color: $color;
  }
  .el-step__description.is-wait{
    color: $color;
  }
  .log-icon{
    width: 15px;
    height: 15px;
    margin-left: 3px;
  }
  // .el-step__main {
  //   white-space: inherit;
  // }
}

.node-attachment-list{
  margin-top: 6px;
}

.node-attachment-title{
  margin-bottom: 4px;
  color: #606266;
}

.node-attachment-item{
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  color: #409eff;
  margin-top: 4px;

  .el-link{
    display: inline-flex;
    align-items: center;
  }

  .el-icon-view,
  .el-icon-download{
    color: #409eff;
    cursor: pointer;
    margin-left: 6px;
  }
}

.node-attachment-name{
  margin-left: 4px;
}

.print-attachment-list{
  display: flex;
  flex-wrap: wrap;
  margin-top: 4px;

  .node-attachment-item{
    color: #606266;
    margin-right: 12px;
  }
}
</style>
