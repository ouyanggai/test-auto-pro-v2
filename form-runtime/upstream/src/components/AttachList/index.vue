<!--
 * @Descripttion:附件列表
 * @Author: liufuze
 * @Date: 2022-03-29 09:37:17
-->
<template>
  <div v-if="isShowList" class="flex-box attach" style="align-items:center;">
    <div class="title" style="font-size:20px;width: 128px;"><i class="el-icon-paperclip" style="color:rgb(251,191,36);"></i>
      <el-link :underline="false" @click="uploadFileButton" :disabled="noClick" style="font-size:16px;margin-left: 4px;">添加附件({{ attachList.length }})</el-link>
    </div>
    <ul class="flex-box attach-ul" v-if="attachList.length>0">
      <li v-for="(val, index) in attachList" class="attach-li" :key="index">
        <div class="attach-div">
          <transition name="el-fade-in-linear">
            <div class="progress-div" v-show="val['status'] == 'uploading'" @mouseover="showCloseIcon(index)"
              @mouseout="hideCloseIcon(index)">
              <el-progress style="margin-top:3px;" :percentage="val['percent'] - 0" :text-inside="true"
                :stroke-width="17"></el-progress>

              <i class="el-icon-close upload-close" @click.stop="handleUplodingRemove(val['key'], val['formType'])"
                v-show="val['isShowUploadIcon']"></i>
            </div>
          </transition>
          <!-- <el-link @click="downLoad(val['url'], val['name'])"> -->
          <el-link @click="downLoad(val['url'], val.name,val)">
            <i :class="(val['name']||val['data']['originFileName']) | fileIcon"></i>
            <span >{{ (val['name']||val['data']['originFileName'])| shortName  }}</span>
            <!-- <span v-else>{{ val['data']['originFileName'] | shortName }}</span> -->
            <!-- <span>{{ val['name'] | shortName }}</span> -->
            <!-- <span>({{ val['size'] | fileSize }})</span> -->
          </el-link>
          <i class="el-icon-view" @click="viewFile(val)" style="color:#409eff;margin:0 5px 0 10px;cursor: pointer;"></i>
          <i class="el-icon-download" @click="downLoad(val['url'], (val['name']||val['data']['originFileName']),val)" style="color:#409eff;margin:0 5px 0 0px;cursor: pointer;"></i>
          <!-- <i class="el-icon-download" @click="downLoad(val['url'], val['name'])" style="color:#409eff;margin:0 5px 0 0px;cursor: pointer;"></i> -->
          <i class="el-icon-close" @click.stop="handleRemove(val['key'],val['formType'])"
            v-if="isShowClose(val['status'], val['formType'], val['flowStatus'])"></i>
        </div>
      </li>
    </ul>
    <el-dialog
      title=""
      :visible.sync="excelVisible"
      :append-to-body="true"
      :close-on-click-modal="false"
      :fullscreen="true"
      width="70%"
      top="5vh">
      <div style="height:100vh;" v-if="excelVisible">
        <officeExcel :iframeOrigin="iframeOrigin" :fileUrl="fileUrl" :callbackUrl="callbackUrl" :mode="mode" :documentType="documentType"
          :title="excelTitle" :token="myToken" :user="user" :excelKey="excelKey" ref="officeExcelRef">
        </officeExcel>
      </div>
    </el-dialog>
  </div>
</template>

<script>
const icon_arr = ['png', 'jpg', 'jpeg', 'gif'];
import { viewFile,generateUid,generateRandomId } from '@/utils';
import { localstorageGet } from '@/utils/auth';
import officeExcel from '@/components/OfficeExcel'
import {onlyOfficeUrl } from '@/config/env';
export default {
  name: 'AttachList',
  props: ['uploading', 'models', 'enableData', 'disabledData', 'btnVisible', 'uploadedFile', 'isShowList', 'jsonData', 'fromPage','isExamine','isReInitiate','flowStatus'],
  data() {
    return {
      attachList: [],
      objFromFile: {},
      modelsData: [],

      excelVisible:false,
      documentType:'word',
      fileUrl: '',
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
  components:{officeExcel},
  beforeDestroy() {
    Object.assign(this.$data, this.$options.data());
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
  watch: {
    uploading(current, old) {
      // console.log('watch-uploading')
      const uploading = current;
      const key = uploading.modelName;
      this.objFromFile[key] = uploading.fileList;
      this.createUploadingList(key, uploading.fileList);
    },
    models: {
      handler(current, old) {
        console.log(current, old, 'current, old');
        this.modelsData = current;
        const copy_current = JSON.parse(JSON.stringify(current));
        this.upDateUpfileStatus(copy_current);
        console.log(this.attachList, 'current, old');
      },
      deep: true
    },
    uploadedFile: {
      handler(current, old) {
        // console.log('watch-uploadedFile')
        if (current.val && current.val instanceof Array) {
          const key = current.field;
          this.objFromFile[key] = current.val;
          this.modelsData[key] = current.val;
          this.createUploadingList(key, current.val);
        }
      },
      deep: true
    },
    jsonData(current) {
      this.createCommonFileButton(current);
    }
  },
  computed: {
    showList() { return !!this.isShowList; },
    formJsonData() {
      return this.jsonData;
    },
    noClick() {
      if (this.fromPage == 'startFlow') {
        return false;
      } else {
        if(this.isExamine === false && this.isReInitiate === false){ // 查看流程
          return true
        } else {
          if(this.isExamine === false){
            return false
          }
        }
        if (this.fromPage == 'finishedView' || this.fromPage == 'submitView' || this.fromPage == 'submittedView') {
          return true;
        }
        if ((this.enableData && this.enableData.findIndex(item => item == 'fileupload_common_file') == -1) || this.btnVisible === false) {
          return true;
        }
        return false;
      }
    },
    isShowClose() {
      return (status, formType, flowStatus) => {
        console.log(status, formType,'status, formType++++++++++++',flowStatus)
        if (this.fromPage == 'startFlow') {
          return true;
        }
        if(this.flowStatus=='draft'|| this.flowStatus=='withdraw'){
          return true
        }
        // if (this.fromPage == 'submittedView' || this.fromPage == 'submitView') {
        //   return false
        // } else {
        let disabled = false;
        if (this.disabledData && this.disabledData.indexOf(formType) !== -1) disabled = true;
        if (this.enableData && this.enableData.indexOf(formType) !== -1) disabled = false;
        console.log(disabled,'disabled++++++++++++',this.btnVisible)
        if (status == 'success' && disabled === false && this.btnVisible !== false) {
          return true;
        } else {
          return false;
        }
        // }
      };
    }
  },
  methods: {
    viewFile(row) {
      console.log('row',row)
      // let {name} = row
      let name = row?.name || row.key
      let extendName = name.split('.').pop().toLowerCase();
      if(extendName.indexOf('doc') > -1){
        // console.log('extendName',extendName)
        // return
        let bindRandomId = generateUid();
        // let bindRandomId = 'e14e-5f56-48da-a8b1-1b2a113c';
        this.fileUrl = row.url;
        this.newFileName = row.name
        this.mode = 'view'
        this.myToken = generateRandomId(32)
        this.excelKey = generateRandomId(24)
        this.user = {
          name: window.localStorage.getItem('invest-power-system-userName') || '',
          id: window.localStorage.getItem('invest-power-system-userId') || '',
        }
        const sid = localstorageGet('token') || '';
        // fileName需要传给后端，因为有文件后缀，后端根据后缀返回文件格式
        this.callbackUrl = `${onlyOfficeUrl}/web/windPowerEconomyEvaluationModel/onlyOfficeCallBack?sid=${sid}&platformCode=200001&id=${bindRandomId}&fileName=${this.newFileName}`
        console.log('this.callbackUrl',this.callbackUrl)
        this.$nextTick(() => {
          this.excelVisible = true
        })
      }else{
        if (!row.url) return;
        viewFile(row.url)
      }

      // return
      // console.log('viewFile')

    },
    setModels(data){
      console.log('setModels')
      if(data){
        data.map(item=>{
          console.log(1,item)
          const k ={
            c_time:'',
            code:"RESP200",
            data:item.key?item.data:item,
            formType:"fileupload_common_file",
            isSuccess:true,
            key:item.key||item.fileName,
            message:"success",
            name:item.key.split(item.c_time+'_')[1]||item.fileName,
            originFileName:item.key.split(item.c_time+'_')[1]||item.fileName,
            status:"success",
            percent:100,
            url:item.absolutelyFileUrl||item.data.absolutelyFileUrl,
            flowStatus:item.flowStatus||''
          }
        this.attachList.push(k)
        })
        // console.log(3,this.attachList)
      }
      // this.modelsData = data;
      // data.map(item=>{
      //   const copy_current = JSON.parse(JSON.stringify(item));
      // this.upDateUpfileStatus(copy_current);
      // })

    },
    downLoad(href, name,val) {
      console.log('download',name,href,val)
      // const lastIndex = name.lastIndexOf('.');
      // const extendName = name.substr(lastIndex + 1);
      let fileName=''
      if(!name){
        fileName=val.key.split(val.c_time+'_')[1]
      }

      const ele = document.createElement('a');
      ele.target = '_blank';
      // if (extendName == 'pdf' || icon_arr.indexOf(extendName) !== -1) { // 预览
      //   ele.href = href;
      //   ele.style.display = 'none';
      //   document.body.appendChild(ele);
      //   ele.click();
      //   document.body.removeChild(ele);
      // } else {
        fetch(href).then(res => {
          return res.blob();
        }).then(blob => {
          const b = new Blob([blob]);
          const url = window.URL.createObjectURL(b);
          ele.href = url;
          ele.download = fileName||name;
          document.body.appendChild(ele);
          ele.click();
          document.body.removeChild(ele);
          window.URL.revokeObjectURL(url);
        });
      // }
    },
    // 只有合同盖章评审时，如果有更新文件，表单覆盖的同时，附件跟着改变
    // ifSealContractChangeFile(key,data,i){
    //   if (data[i]['formType'] == "sealContractFile") {
    //     let findIndex = this.attachList.findIndex(x=>x.formType == key)
    //     data[i]['data']['fileSize'] = this.attachList[findIndex]['data']['fileSize']
    //     this.attachList[findIndex]['data'] = data[i]['data']
    //     this.attachList[findIndex]['data']['absolutelyFileUrl'] = data[i]['data']['fileUrl']
    //     this.attachList[findIndex]['key'] = this.attachList[findIndex]['c_time']+'_'+data[i]['name']
    //     this.attachList[findIndex]['name'] = data[i]['name']
    //     this.attachList[findIndex]['url'] = data[i]['url']
    //   }
    // },
    createUploadingList(key, val) {
      console.log('createUploadingList')
      if (val && val instanceof Array && val.length && val[0].status) {
        const data = JSON.parse(JSON.stringify(val));


        for (let i = 0; data[i]; i++) {
          // this.ifSealContractChangeFile(key,data,i); // 25.1.13更改表单文件和公共附件不关联时注释

          if (data[i].data && data[i].data.fileSize > 0) {
            continue;
          } else {
            let isHas = false;

            for (let j = 0; this.attachList[j]; j++) {
              if (data[i].key == this.attachList[j].key) {
                isHas = true;
                this.attachList[j].status = data[i].status;
                this.attachList[j].percent = data[i].percent;
                continue;
              }
            }
            if (!isHas) {
              data[i].formType = key;
              data[i].c_time = data[i].key.substr(0, 13) - 0;
              this.attachList.push(data[i]);
            }
            this.attachList.sort(this.compare('c_time'));
          }
        }
      }
    },
    upDateUpfileStatus(val) {
      console.log('upDateUpfileStatus')
      if (!val) return false;
      // this.attachList = [];
      // console.log(1,this.attachList)
      const data = JSON.parse(JSON.stringify(val));
      const list = JSON.parse(JSON.stringify(this.attachList));
      for (const key in data) {
        // console.log('key',key)
        if (key.endsWith('common_file')) {
          const detail = data[key];
          if (detail && detail instanceof Array && detail.length && detail[0].url) {
            for (let i = 0; detail[i]; i++) {
            console.log(55555,detail[i])
              if (detail[i].data && detail[i].data.fileSize > 0) {
                const dataIndex = detail[i].key;
                detail[i].originFileName =  detail[i].name
                const index = list.findIndex(item => item.key == dataIndex);
                if (index !== -1) {
                  list[index].url = detail[i].url;
                  list[index].formType = key;
                  list[index].c_time = detail[i].key.substr(0, 13) - 0;
                  list[index].status = 'success';
                  list[index].percent = 100;
                // list[index].originFileName = detail[i];
                } else {
                  detail[i].c_time = detail[i].key.substr(0, 13) - 0;
                  detail[i].formType = key;
                  list.push(detail[i]);
                }
              }
            }
          }
        }
      }
      list.sort(this.compare('c_time'));
      this.attachList = list;
      // console.log(2,this.attachList)
    },
    compare(property) {
      return function (a, b) {
        var value1 = a[property];
        var value2 = b[property];
        return value1 - value2;
      };
    },
    showCloseIcon(index) {
      this.$set(this.attachList[index], 'isShowUploadIcon', true);
    },
    hideCloseIcon(index) {
      this.attachList[index].isShowUploadIcon = false;
    },
    handleUplodingRemove(k, formType) {
      // 删除input list
      const target = this.objFromFile[formType];
      target.splice(target.findIndex(item => item.key == k), 1);
      // 删除本组件list
      this.attachList.splice(this.attachList.findIndex(item => item.key == k), 1);
    },
    handleRemove(k, formType) {
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const models = this.modelsData;
        const targetObj = models[formType];
        const index = targetObj.findIndex(item => item.key == k);
        targetObj.splice(index, 1);
        this.attachList.splice(this.attachList.findIndex(item => item.key == k), 1);
        // 删除list
      }).catch(() => { });
    },
    uploadFileButton() {
      if (this.noClick) {
        return false;
      } else {
        var uploadbt = document.querySelector('input[type=file].fileupload_common_file');
        if (uploadbt) uploadbt.click();
        else {
          uploadbt = document.querySelector('div.fileupload_common_file input[type="file"]');
          if (uploadbt) uploadbt.click();
        }
      }
    },
    createCommonFileButton(jsonData) {
      console.log('createCommonFileButton')
      const list = jsonData?.list || [];
      const index = list.findIndex(item => {
        return item.model == 'fileupload_common_file';
      });
      const staticForm = {
        events: {
          onChange: '',
          onRemove: '',
          onUploadError: '',
          onUploadSuccess: ''
        },
        icon: 'icon-wenjianshangchuan',
        key: 'common_file',
        model: 'fileupload_common_file',
        name: '文件',
        rules: [],
        type: 'fileupload',
        options: {
          defaultValue: [],
          width: '',
          tokenFunc: 'funcGetToken',
          token: '',
          tokenType: 'datasource',
          domain: 'http://tcdn.form.making.link/',
          disabled: false,
          tip: '',
          action: '/api/web/file/api/file/uploadFile',
          customClass: 'fileupload_common_file',
          limit: 200,
          multiple: true,
          isQiniu: false,
          labelWidth: 100,
          hideLabel: true,
          isLabelWidth: false,
          hidden: false,
          dataBind: true,
          headers: [],
          required: false,
          withCredentials: false,
          remoteFunc: 'func_fra8ko0k',
          remoteOption: 'option_fra8ko0k',
          tableColumn: false
        }
      };
      if (index == -1) {
        list.push(staticForm);
      } else {
        list[index] = staticForm;
      }
    }
  }
};
</script>

<style scoped lang="scss">
.attach {
  margin-bottom: 5px;
  align-items: center;

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
}

.attach .progress-div {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  background: transparent;
  z-index: 2;
}

.attach .upload-close {
  position: absolute;
  top: 50%;
  right: 10px;
  transform: translateY(-50%);
  z-index: 3;
}

.attach .title {
  width: 95px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

// .attach .noClick {
//   cursor: not-allowed;
// }
</style>
