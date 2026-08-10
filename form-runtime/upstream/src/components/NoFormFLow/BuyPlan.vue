<!-- 采购计划 -->
<template>
  <div :style="{'width':formWidth}" class="form-container">
    <div class="title">
      <h3>{{ formTitle }}</h3>
    </div>
    <el-row class="attach-div">
      <el-col :span="14" >
        <div style="min-height:10px;">
          <elupload ref="elupload" title="上传附件" v-if="operaType == 'create'"></elupload>
          <elupload ref="elupload" v-else-if="operaType == 'preview'" :attachFile="attachFile" :showOnly="true">
          </elupload>
          <elupload ref="elupload" v-else-if="operaType == 'edit'" :attachFile="attachFile" >
          </elupload>
          <elupload ref="elupload" v-else-if="operaType == 'examine'" :attachFile="attachFile" :showOnly="true">
          </elupload>
        </div>
      </el-col>
      <el-col :span="5">
        <div class="col-div">
          公司编号：<el-input placeholder="公司编号" v-model="companyCode" maxlength="200" style="width: 60%;" :disabled="topDisable"></el-input>
        </div>
      </el-col>
      <el-col :span="5">
        <div class="col-div">
          项目编号：<el-input placeholder="项目编号" v-model="projectCode" maxlength="200" style="width: 60%;" :disabled="topDisable"></el-input>
        </div>
      </el-col>
    </el-row>
    <!-- 主要信息 -->
    <el-card>
      <CommonForm :formConfig="formMainConfig" :width="formWidth" :initForm="initMainForm" ref="mainForm"></CommonForm>
    </el-card>
    <!-- 分标段信息，可多个标段 -->
    <div style="margin-top: 10px;">
      <el-card v-for="(form,index) in section" :key="index" class="card-div" @mouseover.native="mouseOnCard(index)" @mouseout.native="mouseOutCard" >
        <div>
          <CommonForm :formConfig="formContentConfig" :initForm="form"  :width="formWidth" :ref="'section'+index" :pickerOptions="pickerOptions[index]"></CommonForm>
          <template v-if="operaType=='edit' || operaType=='create'">
            <el-button type="danger" v-show="mouseIndex == index && section.length > 1" icon="el-icon-delete" circle class="delete" @click="deleteSection(index)"></el-button>
          </template>
        </div>
      </el-card>
    </div>
    <div class="padding right">
      <el-button type="primary" icon="el-icon-circle-plus-outline" plain @click="addSection" v-if="operaType=='edit' || operaType=='create'">添加标段</el-button>
    </div>
    <CommonFooter v-if="operaType =='create' || operaType =='edit'"  @submit="submit" @reSubmit="reSubmit" :isReInitiate="isReInitiate"></CommonFooter>
    <Flow ref="flow" v-bind="$attrs"></Flow>
  </div>
</template>

<script>
import Api from '@/api';
import { deepClone } from '@/utils';
import elupload from '@/components/EleUpload';
import Flow from './components/Flow';
import CommonForm from './components/CommonForm';
import { formMainConfig, formContentConfig } from './config/BuyPlanConfig';
import CommonFooter from './components/CommonFooter';
import { localstorageGet } from '@/utils/auth';
import mixin from './mixin/mixin';
export default {
  name: 'buy_plan',
  components: { elupload, Flow, CommonForm, CommonFooter },
  props: ['operaType', 'isReInitiate', 'otherBizId', 'flowNodeProxyId'],
  mixins: [mixin], // mixin--------------------------------------
  data() {
    return {
      formWidth: '1080px',
      formTitle: '招标/采购计划评审表',
      section: [],
      mouseIndex: -1,
      formMainConfig: deepClone(formMainConfig),
      formContentConfig: deepClone(formContentConfig),
      companyCode: '',
      projectCode: '',
      attachFile: [],
      initMainForm: {},
      initContentForm: {},
      pickerOptions: [],
      topDisable: false
    };
  },
  created() {
    this.loadRelTreeList();
    this.initMainForm = this.initFormList(formMainConfig);
    this.initContentForm = this.initFormList(formContentConfig);
    // this.initProjectName()
    this.assignValue(this.formMainConfig, 'entrust', 'changeEvent', this.entrustChange);
    // this.assignValue(this.formContentConfig,'bidOpeningDate','pickerOptions',this.pickerOptionsBid)
    if (this.otherBizId) { // 查看有数据表单
      this.findBuyPlanById().then(res => {
        // 根据数据回显表单
        if (res.isSuccess) {
          this.insertDataToForm(res.data);
          if (this.operaType == 'examine') {
            this.getInputPermision().then(res => {
              const data = res || [];
              this.setDisableData(data);
            });
          } else if (this.operaType == 'preview') {
            this.setDisableData([]);
          }
        } else {
          this.$message.error(res.message);
        }
      });
    } else {
      if (this.operaType == 'create' || this.operaType == 'preview') {
        this.section.push(deepClone(this.initContentForm));
      }
      if (this.operaType == 'preview') {
        this.topDisable = true;
        this.setDisableData([]);
      }
    }
  },
  watch: {
    section: {
      handler(val) {
        this.createPickerOptions(val);
      },
      deep: true
    }
  },
  inject: {
    handleClose: { value: 'handleClose', default: null }
  },
  methods: {
    createPickerOptions(list) {
      if (list.length) {
        list.forEach((item, index) => {
          var sellDate = item.sellDate;
          if (sellDate) {
            var pickerOptions = {
              disabledDate(time) {
                return time.getTime() < new Date(sellDate).getTime();
              }
            };
            this.pickerOptions.splice(index, 1, pickerOptions);
          }
        });
      }
    },
    entrustChange(val) {
      if (val == 1) {
        this.assignValue(this.formMainConfig, 'entrustCompanyId', 'disabled', false);
        if (this.relativeList.length) {
          this.initMainForm.entrustCompanyId = this.relativeList[0].value;
        }
      } else {
        this.assignValue(this.formMainConfig, 'entrustCompanyId', 'disabled', true);
        this.initMainForm.entrustCompanyId = '';
      }
    },
    insertDataToForm(data) {
      this.originData = data;
      this.bizId = data.id;
      var attachment = data.attachment;
      this.companyCode = data.companyCode;
      this.projectCode = data.projectCode;
      if (attachment) {
        this.getBatchFile(data.id).then(res => {
          if (res.isSuccess) {
            this.attachFile = res.data || [];
          }
        });
      }
      var initMainForm = deepClone(this.initMainForm);
      for (const key in initMainForm) {
        initMainForm[key] = data[key];
      }
      this.initMainForm = initMainForm;

      var section = [];
      data.procureSectionInfoVoList.sort((a, b) => {
        return a.sort - b.sort;
      });
      var procureSectionInfoVoList = data.procureSectionInfoVoList || [];
      procureSectionInfoVoList.forEach(item => {
        // var formConfig = deepClone(this.formContentConfig)
        var initContentForm = deepClone(this.initContentForm);
        for (const key in initContentForm) {
          initContentForm[key] = item[key] || '';
        }
        // formConfig.forEach(row=>{
        //   row.forEach(el=>{
        //     var prop = el.prop
        //     if(el.label != 'label'){
        //       el.value = item[prop]
        //       if(el.type == 'date')el.value = el.value.substr(0,10)
        //     }
        //   })
        // })
        // var id = item.id || ''
        section.push(initContentForm);
      });
      this.section = section;
    },
    findBuyPlanById() {
      var data = {
        id: this.otherBizId
      };
      return this.$axios.post(Api.noForm.findBuyPlanById, { data });
    },
    loadRelTreeList() { // 获取相关方列表
      this.$axios.post(
        Api.noForm.loadRelTreeList,
        {
          data: {
            projectId: localstorageGet('projectId')
            // projectMainDeptVo: {
            //   id: this.$store.state.user.projectDepartmentId // 项目部id
            // }
          },
          pages: 1,
          size: 999
        },
        res => {
          if (res.isSuccess) {
            const dataList = res.data?.dataList || [];
            var relativeList = this.relativeList = dataList.map(item => {
              return {
                label: item.name,
                value: item.id
              };
            });
            this.assignValue(this.formMainConfig, 'procureCompanyId', 'options', relativeList);
            this.assignValue(this.formMainConfig, 'entrustCompanyId', 'options', relativeList);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // initProjectName(){
    //   var projectName = localstorageGet('projectName')
    //   this.initMainForm['projectName'] = projectName
    // },
    // assignValue(arr,prop,key,val){
    //   arr.forEach(item=>{
    //     item.forEach(el=>{
    //       var find = false
    //       if(el?.prop == prop){
    //         el[key] = val
    //       }
    //     })
    //   })
    // },
    setDisableData(data) {
      var formMainConfig = deepClone(this.formMainConfig);
      formMainConfig.forEach(row => {
        this.checkDisable(row, data);
      });
      this.formMainConfig = formMainConfig;

      var formConfig = deepClone(this.formContentConfig);
      formConfig.forEach(row => {
        this.checkDisable(row, data);
      });
      this.formContentConfig = formConfig;
    },
    checkDisable(row, data) {
      row.forEach(el => {
        if (el.type != 'label') {
          if (this.operaType == 'preview') {
            el.disabled = true;
          } else if (this.operaType == 'examine') {
            el.disabled = true;
            const prop = el.prop;
            if (data.indexOf(prop) > -1)el.disabled = false;
          }
        }
      });
    },
    genRandIndex() {
      return new Date().getTime();
    },
    mouseOnCard(val) {
      this.mouseIndex = val;
    },
    mouseOutCard() {
      this.mouseIndex = -1;
    },
    addSection() {
      this.section.push(deepClone(this.initContentForm));
      // this.section.push({id:'', formConfig:deepClone(formContentConfig)})
    },
    deleteSection(index) {
      this.$confirm('删除后内容不可恢复，是否确认删除', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.section.splice(index, 1);
      }).catch(() => {});
    },

    async processData(status) {
      const mainData = await this.$refs.mainForm.getData();
      if (!mainData) return false;
      const procureSectionInfoVoList = [];
      var ref = true;
      await this.section.forEach(async (item, index) => {
        var sectionData = await this.$refs['section' + index][0].getData();
        if (!sectionData)ref = false;
        if (sectionData) {
          sectionData.sort = index;
          if (this.operaType == 'create') delete sectionData.id;
          procureSectionInfoVoList.push(sectionData);
        }
      });
      if (!ref) return false;
      mainData.procureSectionInfoVoList = procureSectionInfoVoList;
      mainData.companyCode = this.companyCode;
      mainData.projectCode = this.projectCode;
      mainData.pushBidding = '0';
      mainData.status = status;
      mainData.examineStatus = '0';
      mainData.attachment = this.$refs.elupload.getFileId().join(',');
      if (this.operaType == 'create') delete mainData.id;
      return mainData;
    },
    async processSaveData() {
      var mainData = await this.processData();
      if (!mainData) return false;
      var originData = deepClone(this.originData);
      for (const key in originData) {
        if (key != 'procureSectionInfoVoList') {
          if (mainData[key] !== undefined) {
            originData[key] = mainData[key];
          }
        } else {
        }
      }
      mainData.pushBidding = originData.pushBidding;
      mainData.examineStatus = originData.examineStatus;
      mainData.status = originData.status;
      // var postData = deepClone(this.originData)
      // postData.procureSectionInfoVoList = mainData.procureSectionInfoVoList
      var listData = [];
      var procureSectionInfoVoList = originData.procureSectionInfoVoList;
      mainData.procureSectionInfoVoList.forEach(item => {
        if (item.id) {
          var id = item.id;
          var index = procureSectionInfoVoList.findIndex(el => el.id == id);
          if (index > -1) {
            var el = procureSectionInfoVoList[index];
            for (const key in el) {
              if (item[key] === undefined) {
                item[key] = el[key];
              }
            }
          }
        } else {
          listData.push(item);
        }
      });

      originData.procureSectionInfoVoList = mainData.procureSectionInfoVoList;
      return originData;
    }
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
