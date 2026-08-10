<!--
 * @Descripttion: 转发数据显示
 * @Author: zhengzetao
 * @Date: 2022-08-17
-->

<template>
   <!-- style="width: 80%;margin: 0 auto;" -->
  <div class="transpondFlow-wrap">
    <!-- style="padding: 0px 50px;" -->
    <div v-if="transpondFlowData.length >1 || (transpondFlowData.length && (transpondFlowData[0]['fuyan'].length || transpondFlowData[0]['yijian'].length))">
      <div style="padding: 10px;background: rgb(225, 243, 216);">转发数据</div>
      <!-- v-if="transpondIndex < transpondFlowData.length-1" -->
      <div v-if="item.fuyan.length || item.yijian.length" v-for="(item, transpondIndex) in transpondFlowData" :key="transpondIndex" style="margin-bottom:10px;border: 1px solid rgb(235, 238, 245);">
        <div style="font-weight:bold;font-size:16px;padding:10px;">{{ '第'+(transpondIndex+1)+'次转发' }}</div>
        <!-- 附言 -->
        <div class="new-postscript-divWrap" v-if="item.fuyan.length">
          <div class="flow-wrap" v-loading="loading">
            <div  v-if="item.fuyan && item.fuyan.length > 0" class="postscript-list">
              <p style="font-weight:bold;">附言记录</p>
              <div v-for="( value, index) in item.fuyan" :key="value.id" class="script-item">
                <div>
                  <p class="item-info">
                    <span>{{ value.sendName }}</span>
                    <span class="item-info-date">{{ value.createDate }}</span>
                    <!-- <el-button type="text" class="item-info-reply" @click="handleReply(index)">回复</el-button> -->
                  </p>
                  <p style="text-indent: 1rem;">{{ value.text }}</p>
                  <p v-if="value.relationFileDataVos && value.relationFileDataVos.length>0" style="text-indent: 1rem;">
                    <span>附件：</span>
                    <span v-for="file in value.relationFileDataVos" :key="file.id" class="file-item">
                      {{ file.originFileName }}
                      <i class="el-icon-view fileClick-icon" @click.stop="viewFile(file)"></i>
                      <i class="el-icon-download fileClick-icon" @click.stop="downloadFileItem(file)"></i>
                    </span>
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- 意见 -->
        <div class="new-flow-log-container">
          <div direction="vertical" style="color: 000;font-size:12px;">
            <!-- background:rgb(140,140,140);border: 1px solid rgb(140,140,140);border-bottom:none; -->
            <div v-if="item.yijian.length" style="padding:8px 20px;font-weight:bold;">处理人意见</div>
            <!-- border:1px solid rgb(153,153,153); -->
            <div v-for="(val, index) in item.yijian" :key="index" style="display: flex;padding:6px 20px;background:rgb(245,245,245);">
              <div style="margin-right:5px;width:60px;">{{ val.executorName }}</div>
              <div style="margin-right:5px;width:60px;">{{ val.auditStatus }} </div>
              <div style="margin-right:30px;">
                {{ val.createDate }}
              </div>
              <div style="margin-right:30px;width:110px;" :title="val.auditNodeName">
                {{ val.auditNodeName }}
              </div>
              <div>
                {{val.executeDesc | filterDevice}}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';
import { viewFile } from '@/utils';
import moment from 'moment';

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
      },
      loading:false,
      isAddPostscript: false,
      attachmentFileList: [],
      postscriptMessage: '',
      tempFileResult:[]
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
    // logTableData: {
    //   type: Array,
    //   default: () => {
    //     return [];
    //   }
    // },
    transpondFlowData: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  filters:{
    filterDevice(val){
      if(val){
        let reg = /\#\{(.+?)\}\#/g
        let str = val.replace(reg,'')
        return str
      }else{
        return val
      }
    }
  },
  computed: {},
  watch: {
  },
  created() {
    console.log('转发组件',this.transpondFlowData)
   },
  mounted() { },
  methods: {
    viewFile(row) {
      console.log('viewFile',row)
      if (!row.fileUrl) return;
      viewFile(row.fileUrl)
    },
    downloadFileItem(file) {
      if (!!window.ActiveXObject || 'ActiveXObject' in window) { // IE无法识别download属性，用户自行保存
        this.$message.error(
          '当前浏览器不支持点击下载，请手动保存，或者切换到Google Chrome浏览器进行下载'
        );
      } else {
        // this.loading = true
        const x = new XMLHttpRequest();
        x.open('GET', file.fileUrl, true);
        x.responseType = 'blob';
        var _this = this
        x.onload = function () {
          const url = window.URL.createObjectURL(x.response);
          const a = document.createElement('a');
          a.href = url;
          a.download = file.originFileName;
          a.click();
          a.remove();
          _this.loading = false
        };
        x.onerror=()=>{
          _this.loading = false
          // this.loading = false
        };
        x.send();
      }
    }
  }
};

</script>
<style lang='scss' scoped>
$color:#333;

.transpondFlow-wrap {
  margin: 0 auto;
  width: 950px;
  padding: 10px;
}

// 下面是复制过来的附言样式
.flow-wrap {
  margin: 0 auto;
  // width: 950px;
  padding: 10px;

  .postscript-list {
    max-height: 260px;
    overflow-y: auto;
    // padding: 10px 8px;
    padding: 10px 20px;
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

    .script-item {
      margin: 2px 0;
      padding: 5px;

      .item-info {
        position: relative;
        color: #8c8c8c;

        .item-info-date {
          margin-left: 15px;
        }
      }
      .file-item {
        font-size: 12px;
        color: #1989fa;
        margin-right: 12px;
        cursor: default;
        // cursor: pointer;
      }
      .fileClick-icon {
        text-indent: 8px;
        &:hover{
          cursor: pointer;
        }
      }
    }

    .script-item:hover {
      background-color: #f5f5f6;
      border-radius: 8px;
    }
  }
}

// 下面是复制过来的流程样式
.flowWrap {
  margin: 0 auto;
  margin-top: 20px;
  height: calc(100% - 30px);
  overflow:auto;
}

.flowLogContainer {
  width: 100%;
  overflow: hidden;
}
::v-deep .el-card__body{
  overflow: auto;
  height:calc(100% - 85px);
  .el-steps--vertical{
    height:auto;
  }
}

::v-deep .el-card__header {
  background-color: #f5f7fa;
  border-bottom: 1px solid #e4e7ed;
  padding: 5px 20px;
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
}

@media screen and (min-width: 1300px) {
  .transpondFlow-wrap {
    width: 80%;
  }
}
</style>
