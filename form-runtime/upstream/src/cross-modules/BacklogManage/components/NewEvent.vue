<template>
  <div class='print' ref="print">
    <el-form :model="form" :rules="formRules" ref="form" label-width="100px">
      <el-form-item prop="name" label="标题:">
        <div style="display:flex;">
          <el-input style="width: 80%" placeholder="请填写标题" v-model="form.name" :readonly="type == 'edit' && !isReInitiate">
          </el-input>
          <div style="width:100px;margin-left:10px;">
            <el-button v-if="type == 'add'" type="primary" @click="editFlow">编辑流程<i v-if="Object.keys(this.newParam).length" class="el-icon-success" style="color:#fff;margin-left: 5px;font-size: 14px;"></i></el-button>
          </div>
        </div>
      </el-form-item>
      <el-form-item prop="file" label="上传附件:">
        <div style="display:flex;" v-if="type == 'add' || isReInitiate" >
          <el-upload ref="upload" :action="pdfAction" :data="fileData" accept="" multiple
              :before-upload="beforeAvatarUpload" :on-success="handleAvatarSuccess" :on-remove="handleRemove"
              :file-list="form.fileList">
              <el-button size="small" type="primary">选择文件</el-button>
          </el-upload>
        </div>
        <div v-else>
          <ul class="flex-box attach-ul" v-if="form.fileList.length">
            <li v-for="(val, index) in form.fileList" class="attach-li" :key="index">
              <div class="attach-div">
                <el-link>
                  <i :class="(val['name']||val['data']['originFileName']) | fileIcon"></i>
                  <span >{{ (val['name']||val['data']['originFileName'])| shortName  }}</span>
                </el-link>
                <i class="el-icon-view" @click="viewFile(val)" style="color:#409eff;margin:0 5px 0 10px;cursor: pointer;"></i>
              </div>
            </li>
          </ul>
          <div v-else>暂无文件</div>
        </div>
      </el-form-item>
      <el-form-item prop="other" label="关联流程:">
        <!-- <FlowSelect ref="flowListSelect" :newFlowList="newFlowList" :isExamine="flowSelectFlag" :isReInitiate="flowSelectFlag"></FlowSelect> -->
        <FlowSelect ref="flowListSelect" :newFlowList="newFlowList" :isExamine="flowSelectFlag" :isReInitiate="flowSelectFlag"
        :initiatorId="initiatorId" :btnVisible="type == 'add'"></FlowSelect>
        <!-- :enableData="enableData" :btnVisible="btnVisible" -->
      </el-form-item>
      <el-form-item prop="content" label="内容:">
        <!-- <div style="width:100%"> -->
        <!-- 富文本编辑器 -->
        <div v-show="type == 'edit'" v-html="form.content"></div>
        <RichEditor v-show="type == 'add'" ref="richEditorRef"></RichEditor>
        <!-- <div v-show="isPrintPage" v-html="form.content"></div>
        <RichEditor v-show="!isPrintPage" ref="richEditorRef"></RichEditor> -->
      </el-form-item>
    </el-form>

    <div class="flow-log-container" v-if="postscriptList.length && printList.indexOf('发起人附言') > -1">
      <div direction="vertical" style="color: 000;margin-top:10px;font-size:12px;">
        <div style="background:rgb(140,140,140);">发起人附言</div>
        <div v-for="(val, index) in postscriptList" :key="index"
          style="padding:6px 10px;margin:5px 0;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
          <div style="display:flex;">
            <div style="margin-right:5px;width:80px;">{{ val.replyName || val.sendName }}</div>
            <div style="margin-right:30px;">{{ val.createDate }} </div>
          </div>
          <div style="margin-left:5px;width:100%;">{{val.text}}</div>
          <span style="margin-left:10px;color: #47a1fb;" v-if="val.relationFileDataVos && val.relationFileDataVos.length>0"><span style="margin-left:5px" :key="file.id" v-for="file in val.relationFileDataVos">{{ file.fileName }}</span></span>

          <div v-if="val.children.length" style="margin-left:10px;border: 1px solid #ccc;padding: 4px;margin: 5px 0px;" class="script-item-child">
            <div v-for="( childItem, childIndex) in val.children" :key="childItem.id">
              <div class="item-info-child">
                <span style="margin-right:30px;">{{ childItem.replyName || childItem.sendName }}</span>
                <span class="item-info-date">{{ childItem.createDate }}</span>
              </div>
              <div style="text-indent: 1rem;">{{ childItem.text }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="flow-log-container" v-if="printList.indexOf('流程日志') > -1">
      <div direction="vertical" style="color: 000;margin-top:10px;font-size:12px;">
        <div style="background:rgb(140,140,140);">流程日志</div>
        <div v-for="(val, index) in logTableData" :key="index"
          style="display: flex;padding:6px 10px;margin:5px 0;background:rgb(245,245,245);border:1px solid rgb(153,153,153);">
          <div style="margin-right:5px;width:80px;">{{ val.executorName }}</div>
          <div style="margin-right:5px;width:80px;">{{ val.auditStatus }} </div>
          <!-- <div style="margin-right:30px;">
            {{ val.auditStatus }}
          </div> -->
          <div style="margin-right:30px;">
            {{ val.createDate }}
          </div>
          <div>
            {{val.executeDesc}}
          </div>
        </div>
      </div>
    </div>
      
    <!-- 编辑流程弹窗 -->
    <EditFlowDialog v-if="editFlowVisible" ref="EditFlowDialog" :type="type" :originalNodeConfig="originalNodeConfig" :flowProxyId="flowProxyId" :visible.sync="editFlowVisible" @confirm="confirm"></EditFlowDialog>
  </div>
</template>

<script>
const icon_arr = ['png', 'jpg', 'jpeg', 'gif'];
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils/index';
import { baseUrl } from '@/config/env';
import RichEditor from '@/components/RichEditor/index.vue';
// import FlowSelect from '@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue';
import EditFlowDialog from './EditFlowDialog.vue';
import { viewFile } from '@/utils';
import { Print as $print } from '@/utils/print.js';
// 异步调用组件
const FlowSelect = () => import('@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue');

export default {
  name: '',
  components: { RichEditor,FlowSelect,EditFlowDialog },
  props: {
    // visible: {
    //   type: Boolean,
    //   default: false
    // },
    isReInitiate: {
      type: Boolean,
      default: false
    },
    // isPrintPage: {
    //   type: Boolean,
    //   default: false
    // },
    type: {
      type: String,
      default: 'add'
    },
    flowProxyId: {
      // 流程id
      type: String,
      default: ''
    },
    flowInstanceId: { // 流程实例id
      type: String,
      default: ''
    },
    initiatorId: {
      type: String,
      default: ''
    },
    copyTemplateData: {
      type: Object,
      default: function(){
        return {};
      }
    }
  },
  data() {
    return {
      editFlowVisible:false,
      form: {
        name: '',
        fileList: [],
        content:'',
        flowList:[],
      },
      formRules: {
        name: [{ required: true, message: '请填写标题', trigger: 'blur' }],
        content: [{ required: true, message: '请填写内容', trigger: 'blur' }],
      },
      fileData: {
        fileType: 'ordinaryFile'
      },
      fileList: [
      ],
      newParam:{},
      typeList:[],
      newFlowList:[],
      flowSelectFlag: true,
      originalNodeConfig:{},
      postscriptList: [],
      logTableData: [],
      printList:[],
    };
  },
  computed: {
    pdfAction() {
      const sid = this.$store.state.user.token;
      return `${baseUrl}/web/file/api/file/uploadFile?sid=${sid}&platformCode=200001`;
    }
  },
  filters: {
    fileSize(size) {
      return Math.ceil(size / 1024) + 'k';
    },
    fileIcon(name) {
      console.log(name,'name********************')
      const lastIndex = name.lastIndexOf('.');
      const extendName = name.substr(lastIndex + 1);
      let icon = 'el-icon-s-order';
      if (icon_arr.indexOf(extendName) !== -1) icon = 'el-icon-picture';
      return icon;
    },
    shortName(name) {
      console.log(name,'name++++')
      const lastIndex = name.lastIndexOf('.');
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
  created() {
    console.log('newEvent-created')
    this.getFlowTypeList();
    if (this.type == 'add') {
      this.form = {
        name: '',
        fileList: [],
        content:'',
        flowList:[],
      }
    } else if (this.type == 'edit') {
      this.$nextTick(x=>{
        this.form = deepClone(this.copyTemplateData)
        console.log('this.form',this.form)
        console.log('this.form.content',this.form.content)
        console.log('this.$refs.richEditorRef',this.$refs.richEditorRef)
        this.$refs.richEditorRef.contentHtml = this.form.content;
        this.newFlowList = this.form.flowList;
        // this.fileList = this.form.fileList;
        this.flowSelectFlag = this.isReInitiate ? true : false;
        // console.log('this.newFlowList1',this.newFlowList)
      },300)
    }
  },
  mounted() {
  },
  methods: {
    // ========复用其他人的无表单打印-开始=============
    async printPage(list) {
      this.printList = list
      console.log('this.printList',this.printList)
      await Promise.all([this.fetchLogData(), this.getPostScriptList()]);
      var printInst = $print(this.$refs.print, {}, () => {
        console.log(printInst, 'printInst-弹窗关闭');
      });
      // $print(this.$refs.print);
    },
    getPostScriptList() {
      return this.$axios.post(
        Api.approveManage.getPostScriptList,
        {
          data: {
            flowInstanceId: this.flowInstanceId
          }
        },
        (res) => {
          if (res.isSuccess) {
            this.postscriptList = this.generateTree(res.data);
            // this.postscriptList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree(flatArray) {
      // 创建一个映射，用于存储每个节点的引用
      const nodeMap = {};
      // 创建一个数组，用于存储树的根节点
      const tree = [];

      // 遍历扁平数组，初始化每个节点
      flatArray.forEach(item => {
        nodeMap[item.id] = { ...item, children: [] };
      });

      // 再次遍历扁平数组，构建树结构
      flatArray.forEach(item => {
        const node = nodeMap[item.id];
        if (item.pid === null) {
          // 如果没有父节点，则为根节点
          tree.push(node);
        } else {
          // 如果有父节点，则将当前节点添加到父节点的子节点数组中
          const parentNode = nodeMap[item.pid];
          if (parentNode) {
            node.isReplay = true;
            parentNode.children.push(node);
          }
        }
      });
      return tree;
    },
    fetchLogData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.approveManage.findRecord,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          res => {
            if (res.isSuccess) {
              this.logTableData = this.filterWithdraw(res.data);
              this.logTableData.forEach(item => {
                this.translateStatus(item);
              });
              console.log('this.logTableData',this.logTableData)
              resolve()
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    filterWithdraw(data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    // 已建任务和流程日志操作状态字符转换
    translateStatus(obj) {
      let chnStatus;
      if (obj.auditStatus) {
        switch (obj.auditStatus) {
          case 'pass':
            chnStatus = '通过';
            break;
          case 'no_pass':
            chnStatus = '驳回';
            break;
          case 'withdraw':
            chnStatus = '撤销';
            break;
          case 'retrieve':
            chnStatus = '取回';
            break;
          case 'transfer':
            chnStatus = '移交';
            break;
          case 'roll_back_the_previous_level':
            chnStatus = '回退上一节点';
            break;
          default:
            chnStatus = '';
            break;
        }
      } else if (obj.flowStatus) {
        switch (obj.flowStatus) {
          case 'await_sent':
            chnStatus = '待发';
            break;

          case 'run':
            chnStatus = '运行中';
            break;

          case 'withdraw':
            chnStatus = '撤销';
            break;

          case 'termination':
            chnStatus = '终止';
            break;

          case 'rejected':
            chnStatus = '驳回';
            break;

          case 'end':
            chnStatus = '完结';
            break;

          default:
            chnStatus = '';
            break;
        };
      }
      obj.auditStatus = chnStatus;
    },
    // ========复用其他人的无表单打印-结束=============

    viewFile(row) {
      // console.log('viewFile')
      if (!row.url) return;
      viewFile(row.url)
    },
    confirm(data){
      console.log('confirm-data',data)
      this.newParam = data;
    },
    submit(){
      console.log('newEvent-submit')
      return new Promise((resolve,reject)=>{
        this.$refs.form.validate(res=>{
          console.log('richEditorRef.contentHtml',this.$refs.richEditorRef.contentHtml)
          const reg = /^<p><br><\/p>$/;
          // const reg = /(<\/?.*?>)/gi;
          const contentText = this.$refs.richEditorRef.contentHtml.replace(reg, '');
          if (!contentText) {
            this.$message.error('请输入内容！');
            resolve(false)
            // return;
          } else {
            if (res) {
              resolve(true)
            } else {
              resolve(false)
            }
          }
        })
      })
    },
    // 查询类型集合
    getFlowTypeList(){
      this.$axios.post(
        '/type/list',
        {
          data: {
            name: '',
            useScope: 'invest',
            customerCode: this.$store.getters.customerCode
          },
          pages: 1,
          size: 99999
        },
        res => {
          if (res.isSuccess) {
            this.typeList = res.data;
          }
        }
      );
    },
    editFlow(){
      console.log('editFlow',this.newParam)
      this.originalNodeConfig = Object.keys(this.newParam).length ? this.newParam.flowTemplateProtocol.data.flowNodeTemplate : {};
      console.log('originalNodeConfig1',this.originalNodeConfig)
      this.editFlowVisible = true;
    },
    handleRemove(file, fileList) {
      console.log(file, 'file');
      console.log('handleRemove-fileList',fileList);
      this.form.fileList = fileList.map(x=>{
        let obj = {};
        if (x.response) {
          obj = {
            originFileName:x.response.data.originFileName,
            name:x.response.data.originFileName,
            fileId:x.response.data.id,
            fileUrl:x.response.data.absolutelyFileUrl,
            url:x.response.data.absolutelyFileUrl,
            status:"success",
            percent:100
          }
        } else {
          obj = x;
        }
        return obj;
      });
      // this.fileList = fileList;
      // const id = file.response ? file.response.data.id : file.fileId;
      // this.flieId = this.flieId.filter(e => e != id);
    },
    // 文件上传
    handleAvatarSuccess(res, file) {
      console.log(res, file, '++++');
      if (res.code == 'RESP200') {
        console.log('文件-res',res);
        this.form.fileList.push({
          originFileName:res.data.originFileName,
          name:res.data.originFileName,
          fileId:res.data.id,
          fileUrl:res.data.absolutelyFileUrl,
          url:res.data.absolutelyFileUrl,
          status:"success",
          percent:100
        })
        // this.flieId.push(res.data.id);
        // this.$axios.post(
        //   Api.schedule.saveAttachment,
        //   {
        //     data: {
        //       relationId: this.detailId,
        //       fileId: res.data.id
        //     }
        //   },
        //   res => {
        //     this.loading = false;
        //     if (res.isSuccess) {
              
        //       // this.$message.success('关联附件成功');
        //     } else {
        //       this.$message.error(res.message);
        //     }
        //   }
        // );
      } else {
        this.$message.error(`文件上传失败,请重新上传`);
      }
    },
    beforeAvatarUpload(file) {
      console.log(file, '3333');
      // if (!/\.(xlsx|xls|XLSX|XLS)$/.test(file.name)) {
      //   this.$message.error(
      //     '上传文件只能为excel文件，且为xlsx,xls格式'
      //   );
      //   return false;
      // }
    },
  }
}
</script>

<style scoped lang="scss">
  .attach-ul {
    margin-left: 5px;
    flex-wrap: wrap;

    .attach-li {
      margin-right: 5px;
      margin-top: 5px;
      background: rgb(230, 238, 247);

      .attach-div {
        position: relative;
        padding: 1px 5px;
        background: transparent;
        display: flex;
        align-items: center;

        i {
          color: rgb(55, 126, 240);
        }

        i.el-icon-close:hover {
          background: #377ef0;
          color: #fff;
        }
      }
    }
  }

  .flow-log-container {
    display: none;
  }
  @media print {
    ::v-deep {
    .postscript-divWrap {
      width: 900px;
      margin: 0 auto;

      .flow-wrap {
        width: 900px !important;
      }
    }

    .flow-log-container {
      margin: 0 auto;
      overflow: initial;
      display: block;

      .flowWrap {
        margin-top: 0px !important;
        padding: 0px;
      }
    }

    // .script-item-child {
    //   margin: 2px 0;
    //   padding: 4px;
    //   margin-left: 8px;
    //   margin-bottom: 5px;
    //   background: aliceblue;
    //   border: 1px solid #ccc;
    //   .item-info-child {
    //     position: relative;
    //     color: #8c8c8c;
    //     .item-info-date {
    //       margin-left: 15px;
    //     }
    //   }
    // }
  }
    // @page {
    //   size: A4 landscape;
    //   // size: A3 landscape;
    //   // size: 297mm 420mm;
    //   // size: auto; //打印可以选择布局：横向，纵向
    //   // size: landscape;//横向
    //   // size: portrait;//纵向
    //   // margin: 23.5mm; //默认边距
    //   // paper-type: custom;
    //   // custom-paper-source: OMB-A;
    // }
    // .print {
    //   zoom: 0.7;
    //   .flow-log-container {
    //     display: block;
    //   }
    //   ::v-deep input {
    //     border: none;
    //   }
    //   ::v-deep textarea {
    //     border: none;
    //     resize: none;
    //     font-size: 14px !important;
    //     color: rgba(0, 0, 0, 0.847);
    //   }
    //   ::v-deep .el-input__count {
    //     display: none;
    //   }
    //   ::v-deep .attachFiles .el-upload {
    //     display: none;
    //   }
    //   ::v-deep .attach-ul .el-icon-view {
    //     display: none;
    //   }
    //   .colorTip {
    //     display: none;
    //   }
    //   .attack_button {
    //     display: none;
    //   }
    // }
  }

  ::v-deep .my-eidtor {
    min-height:500px !important;
  }

</style>