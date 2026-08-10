<!--
 * @Author: treasure
 * @Description: 新增节点
 * @FilePath: \workflowEngine\src\views\home\components\workflow.vue
-->
<template>
  <div class="workflow-engine">
    <div class="workflow-engine-con">
      <div class="flow-con">
        <div v-for="(item, index) in flowList" :key="index">
          <div class="content">
            <div class="title">
              <span>{{ item.nodeName }}</span>
              <i class="el-icon-close icon-close" @click="handleDel(index)" />
            </div>
            <div class="con" @click="handleNode(item, index)">
              <div class="node-text">定义节点属性</div>
              <i class="el-icon-arrow-right arrow" />
            </div>
          </div>
          <AddNode :sort="index" :add-node-data="addNodeData" @handleAddNode="handleAddNode" />
        </div>
        <div class="end-node">
          <div class="end-text">流程结束</div>
        </div>
      </div>
      <div class="btn-box">
        <el-button type="primary" class="btn" @click="handleLast">上一步</el-button>
        <el-button type="primary" class="btn" :disabled="type == 3 || isNextClick" @click="handleNext">提交</el-button>
        <!-- :disabled="type == 1 || isNextClick" -->
      </div>
    </div>
    <!-- 弹窗 -->
    <el-dialog
      :title="dialogTitle"
      width="600px"
      :visible.sync="dialogVisible"
      :before-close="handleClose"
      :close-on-click-modal="false"
    >
      <div class="dialog-con">
        <el-form ref="ruleForm" :model="form" :rules="rules">
          <el-form-item label="节点名称：" :label-width="formLabelWidth" prop="nodeName">
            <el-input v-model="form.nodeName" placeholder="请输入节点名称" autocomplete="off" :disabled="type == 1" />
          </el-form-item>
          <el-form-item label="有何权限：" :label-width="formLabelWidth" prop="hasAuth">
            <el-input
              v-model="form.hasAuth"
              maxlength="20"
              placeholder="请选择有何权限"
              autocomplete="off"
              suffix-icon="el-icon-arrow-right"
              readonly
              class="icon-input-h"
              @click.native="handleHasAuth"
            />
          </el-form-item>
        </el-form>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button type="primary" plain @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="handleSubmit">提 交</el-button>
      </div>
    </el-dialog>
    <!-- 有何权限 -->
    <el-dialog
      title="定义节点属性"
      fullscreen
      center
      :visible.sync="dialogVisibleHasAuth"
      :before-close="handleCloseAuth"
      :close-on-click-modal="false"
    >
      <div class="has-auth-content">
        <div class="has-auth-con-left">
          <div class="h-tit">表单</div>
          <ul class="scroll-bar has-auth-ul-left">
            <li
              v-for="(item, index) in selectFormList"
              :key="index"
              :class="{ active: isActive == index }"
              @click="handleFormName(item, index)"
            >
              <img v-if="item.isChecked" src="~@/assets/images/success-icon.png" class="warn-icon">
              <img v-else src="~@/assets/images/warn-icon.png" class="warn-icon">
              <span>{{ item.name }}</span>
            </li>
          </ul>
        </div>
        <div class="has-auth-con-right">
          <div style="display:flex;">
            <div class="h-tit" style="flex:1;">表单字段</div>
            <el-checkbox v-if="selectFormList.length" v-model="selectFormList[isActive].allChecked" @change="checkAllPermission" style="padding-right: 20px;line-height: 40px;background-color: #ebebeb;">全选</el-checkbox>
          </div>
          <fm-generate-form ref="generateForm" :data="jsonData" :value="editData" @on-change="onInputChange" />
        </div>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" :disabled="type == 1 || isClick" @click="handleSureHasAuth">提 交</el-button>
      </span>
    </el-dialog>
  </div>
</template>
<script>
import Bus from '@/utils/bus.js';
import Api from '@/utils/api.js';
import AddNode from './SingleNode.vue';
import { arrayToTree } from '@/utils/utils';
export default {
  name: '',
  components: { AddNode },
  props: {
    flowId: {
      type: [String, Number],
      default: ''
    },
    formSelectList: {
      // 已选择表单
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    return {
      jsonData: {},
      editData: {},
      formStatus: '',
      infoForm: {},
      selectFormList: [],
      selectFormListDefault: [],
      selectForm: {},
      id: '',
      nodeFieldPowerList: [],
      type: '',
      formLabelWidth: '130px',
      dialogTitle: '详情',
      dialogVisible: false,
      dialogVisibleHasAuth: false,
      isClick: false,
      isNextClick: false,
      isDisabled: true,
      isActive: 0,
      formNum: 0,
      nodeIndex: 0,
      form: {},
      formList: [],
      hasAuthList: [],
      CheckedKeys: [],
      addNodeData: {
        id: 1,
        nodeName: '审核人'
      },
      flowList: [
        {
          sort: 1,
          nodeName: '审核人',
          flowNodeFieldPowerTemplateList: []
        }
      ],
      authList: [],
      rules: {
        nodeName: [
          { required: true, message: '请输入节点名称', trigger: 'blur' },
          { max: 20, message: '节点名称最大不能超过20个字符', trigger: 'blur' }
        ],
        auth: [{ required: true, message: '请选择谁有权限' }],
        hasAuth: [{ required: true, message: '请选择有何权限' }]
      }
    };
  },
  computed: {

  },
  watch: {
    'selectFormList': {
      handler(val) {
        val.forEach(item=>{
          let isAllChecked = Object.values(item.editData).every(x=>{return x == true});
          // console.log('isAllChecked',isAllChecked)
          item.allChecked = isAllChecked;
        })
      },
      deep: true
    }
  },
  created() {
    this.id = this.$route.query.id;
    this.type = this.$route.query.type;
  },
  mounted() {
    Bus.$on('sendInfoForm', (data) => {
      this.infoForm = JSON.parse(JSON.stringify(data));
    });
    Bus.$on('sendCheckedList', (data) => {
      console.log('sendCheckedList')
      data.forEach(x=>{
        x.allChecked = false
      })
      // console.log('sendCheckedList=data',data)
      this.selectFormList = JSON.parse(JSON.stringify(data));
      this.selectFormListDefault = JSON.parse(JSON.stringify(data));
    });
    Bus.$on('sendSelectForm', (data) => {
      this.selectForm = data;
    });
    if (this.type) {
      Bus.$on('sendFlowNodes', (data) => {
        this.flowList = [];
        this.flowList = JSON.parse(JSON.stringify(data));
      });
    }
  },
  methods: {
    // 点击定义节点属性
    handleNode(item, index) {
      this.dialogVisible = true;
      this.nodeIndex = index;
      if (this.id) {
        this.recombineDataEdit();
      } else {
        this.recombineData();
      }
    },
    // 新增查看重组数据
    recombineData() {
      console.log('recombineData')
      const form = {
        nodeName: '',
        hasAuth: ''
      };
      const selectFormList = JSON.parse(JSON.stringify(this.selectFormListDefault));
      const flowNodeFieldPowerTemplateList = this.flowList[this.nodeIndex].flowNodeFieldPowerTemplateList;
      this.nodeFieldPowerList = flowNodeFieldPowerTemplateList;
      if (flowNodeFieldPowerTemplateList.length != 0) {
        form.nodeName = this.flowList[this.nodeIndex].nodeName;
        form.hasAuth = '已选择';

        selectFormList.map((item) => {
          // let isAllChecked = Object.values(item.editData).every(x=>{return x === true});
          // console.log('isAllChecked',isAllChecked)
          // item.allChecked = isAllChecked;
          item.isChecked = false;
          item.fieldsTemplateList.map((param) => {
            flowNodeFieldPowerTemplateList.map((arg) => {
              if (arg.formFieldTemplateId == param.id) {
                item.editData[param.name] = true;
                item.isChecked = true;
              }
            });
          });
        });
      }
      this.selectFormList = selectFormList;
      this.form = form;
    },
    // 查看，编辑重组装
    recombineDataEdit() {
      console.log('recombineDataEdit')
      const selectFormList = JSON.parse(JSON.stringify(this.selectFormListDefault));
      const form = {
        nodeName: '',
        hasAuth: ''
      };
      const flowNodeFieldPowerTemplateList = this.flowList[this.nodeIndex].flowNodeFieldPowerTemplateList;
      this.nodeFieldPowerList = flowNodeFieldPowerTemplateList;
      if (flowNodeFieldPowerTemplateList.length != 0) {
        form.nodeName = this.flowList[this.nodeIndex].nodeName;
        form.hasAuth = '已选择';
        selectFormList.map((item) => {
          item.isChecked = false;
          item.fieldsTemplateList.map((param) => {
            flowNodeFieldPowerTemplateList.map((arg) => {
              if (arg.formFieldTemplateId == param.id) {
                item.editData[param.name] = true;
                item.isChecked = true;
              }
            });
          });
        });
      }
      this.selectFormList = selectFormList;
      this.form = form;
    },
    // 有何权限
    handleHasAuth() {
      this.dialogVisibleHasAuth = true;
      this.isActive = 0;
      this.hasAuthList = this.selectFormList && this.selectFormList[0].fieldsTemplateList;
      // 判断表单模板
      if (!this.selectFormList[0].templateDatas) {
        this.getFromTemplateData(0, this.selectFormList[0].id, () => {
          this.$nextTick(() => {
            this.$refs.generateForm.refresh();
          });
        });
      } else {
        this.jsonData = this.selectFormList[0].templateDatas;
        for (const item in this.selectFormList[0].editData) {
          this.$set(this.editData, item, this.selectFormList[0].editData[item]);
        }
        this.$nextTick(() => {
          this.$refs.generateForm.refresh();
        });
      }
    },
    // 权限全选
    checkAllPermission(val){
      console.log('checkAllPermission',val)
      let editDataObj = this.selectFormList[this.isActive].editData;
      for (var i in editDataObj) {
        editDataObj[i] = val;
      }
      this.$refs.generateForm.setData(editDataObj)
    },
    // 点击表单名称
    handleFormName(val, index) {
      if (index == this.isActive) return;
      this.formStatus = '';
      this.isActive = index;
      const fieldsTemplateList = val.fieldsTemplateList;
      this.hasAuthList = fieldsTemplateList;
      if (!this.selectFormList[this.isActive].templateDatas) {
        this.getFromTemplateData(index, val.id, () => {
          this.$nextTick(() => {
            this.$refs.generateForm.refresh();
          });
        });
      } else {
        this.jsonData = this.selectFormList[this.isActive].templateDatas;
        for (const item in this.selectFormList[this.isActive].editData) {
          this.$set(this.editData, item, this.selectFormList[this.isActive].editData[item]);
        }
        this.$nextTick(() => {
          this.$refs.generateForm.refresh();
        });
      }
    },
    getFromTemplateData(index, id, cb) {
      console.log('getFromTemplateData')
      const data = {
        id
      };
      this.$axios.post(Api.formLibrary.getFormDetail, {
        // platformCode: '600001',
        data
      }, (res) => {
        if (res.isSuccess) {
          const templateDatas = JSON.parse(res.data.templateData);
          this.generateModle(templateDatas.list);
          this.$set(this.selectFormList[index], 'templateDatas', templateDatas);
          this.jsonData = templateDatas;
          for (const item in this.selectFormList[index].editData) {
            this.$set(this.editData, item, this.selectFormList[index].editData[item]);
          }
          cb();
        }
      });
    },
    generateModle(genList) {
      genList.map((item, key) => {
        if (item.type == 'grid') {
          item.columns.map(val => {
            this.generateModle(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.generateModle(i.list);
            });
          });
        } else {
          if (item.model) {
            const obj =
            {
              type: 'checkbox',
              icon: 'icon-check-box',
              options: {
                inline: false,
                defaultValue: [
                  '编辑'
                ],
                showLabel: true,
                options: [
                  {
                    value: '编辑',
                    label: ''
                  }
                ],
                required: false,
                requiredMessage: '',
                width: '',
                remote: false,
                remoteType: 'datasource',
                remoteOption: item.remoteOption,
                remoteOptions: [],
                props: {
                  value: 'value',
                  label: 'label'
                },
                remoteFunc: item.remoteFunc,
                customClass: '',
                labelWidth: 10,
                isLabelWidth: true,
                hidden: false,
                dataBind: true,
                disabled: this.type == 1,
                hideLabel: false
              },
              events: {
                onChange: ''
              },
              name: '',
              key: item.key,
              model: item.model,
              rules: []
            };
            this.$set(genList, key, obj);
          }
        }
      });
    },
    tabHandel(status) {
      this.hasAuthList.map((item) => {
        item.fieldPower = status;
        this.handleChangeRadio(item);
      });
    },
    // 新增节点
    handleAddNode(val) {
      const arr = JSON.parse(JSON.stringify(this.flowList));
      arr.splice(val.sort, 0, val);
      arr.map((item, index) => {
        item.sort = index + 1;
      });
      this.flowList = arr;
    },
    onInputChange(value) {
      // console.log('onInputChange',value)
      const formValue = this.$refs.generateForm.getValues();
      // console.log('formValue',formValue)
      for (const item in formValue) {
        if(item) {
          this.$set(this.selectFormList[this.isActive].editData, item, formValue[item]);
        }
      }
    },
    // 删除流程中的一个节点
    handleDel(i) {
      const arr = JSON.parse(JSON.stringify(this.flowList));
      if (arr.length == 1) {
        this.$message.error('流程至少保留一项');
        return;
      }
      arr.splice(i, 1);
      arr.map((item, index) => {
        item.sort = index + 1;
      });
      this.flowList = arr;
      this.$emit('handleWorkflow', this.flowList);
    },
    // 提交节点数据
    handleSubmit() {
      this.$refs.ruleForm.validate((valid) => {
        if (valid) {
          this.isClick = true;
          const nodeFieldPowerList = [];
          this.selectFormList.forEach(item => {
            if (item.editData) {
              item.fieldsTemplateList.map((val) => {
                for (const key in item.editData) {
                  if (item.editData[key]) {
                    if (key == val.name) {
                      const obj = {};
                      obj.formTemplateId = val.formTemplateId;
                      obj.fieldPower = 'edit';
                      obj.formFieldTemplateId = val.id;
                      obj.fieldName = key;
                      nodeFieldPowerList.push(obj);
                    }
                  }
                }
              });
            }
          });
          this.flowList[this.nodeIndex].flowNodeFieldPowerTemplateList = nodeFieldPowerList;
          this.flowList[this.nodeIndex].nodeName = this.form.nodeName;
          setTimeout(() => {
            this.reset();
            this.isClick = false;
          }, 600);
        } else {
          this.$message.error('缺少必填的数据或数据格式不正确,提交失败!');
        }
      });
    },
    // 单选(编辑、只读)
    handleChangeRadio(params) {
      const data = JSON.parse(JSON.stringify(params));
      data.formFieldTemplateId = params.id;
      delete data.id;
      let nodeFieldPowerList = JSON.parse(
        JSON.stringify(this.nodeFieldPowerList)
      );
      if (data.fieldPower == 'edit') {
        nodeFieldPowerList.push(data);
      } else {
        nodeFieldPowerList = nodeFieldPowerList.filter((item) => {
          if (item.formFieldTemplateId != data.formFieldTemplateId) {
            return item;
          }
        });
      }
      this.nodeFieldPowerList = nodeFieldPowerList;
    },
    nodeFieldPowerFields(fieldsTemplateList) {
      const flowNodeFieldPowerTemplateList = this.flowList[this.nodeIndex].flowNodeFieldPowerTemplateList;
      if (flowNodeFieldPowerTemplateList) {
        flowNodeFieldPowerTemplateList.map((item) => {
          fieldsTemplateList.map((param) => {
            if (item.id == param.id) {
              param.fieldPower = 'edit';
            }
          });
        });
      }
      return fieldsTemplateList;
    },
    // 每一个节点定义节点属性的提交按钮
    handleSureHasAuth() {
      console.log('handleSureHasAuth')
      this.isClick = true;
      let flag = false;
      this.selectFormList.map((item, index) => {
        item.isChecked = false;
        for (const key in item.editData) {
          if (item.editData[key]) {
            item.isChecked = true;
          }
        }
      });
      this.selectFormList.map((item) => {
        if (!item.isChecked) {
          flag = true;
        }
      });
      setTimeout(() => {
        this.isClick = false;
      }, 600);
      if (flag) {
        this.$message.error('有表单参数没有选择编辑，必须选择一个编辑的参数!');
        return;
      }
      const data = Object.assign({}, this.form);
      data.hasAuth = '已选择';
      this.form = data;
      this.dialogVisibleHasAuth = false;
      this.handleCloseAuth();
    },
    handleLast() {
      this.$emit('updateActive', 1);
      const selectFormList = this.selectFormList;
      this.$store.commit('workflow/CHANGE_SELECT_FORM_LIST', selectFormList);
    },

    // 提交
    handleNext() {
      const infoForm = this.infoForm;
      const selectForm = this.selectForm;
      const flowNodeTemplateList = this.flowList;
      const forms = this.selectFormList.map((item, key) => {
        const obj = {};
        obj.id = item.id;
        obj.code = item.code;
        return obj;
      });
      if (flowNodeTemplateList.length < 2) {
        this.$message.error('流程至少设置两个节点');
        return;
      }
      const len = forms.length;
      let flag = false;
      const powerArr = [];

      // console.log('flowNodeTemplateList', flowNodeTemplateList);
      // return;
      flowNodeTemplateList.map((item, key) => {
        item.treeId = key;
        if (key == 0) {
          item.type = 'start';
          item.childFlowNodeTemplate = {};
        } else {
          item.type = 'common';
          item.childFlowNodeTemplate = null;
        }
        // else if (key == flowNodeTemplateList.length - 1) {
        //   item.type = 'end';
        //   item.childFlowNodeTemplate = null;
        // }
        const arr = [];
        if (item.flowNodeFieldPowerTemplateList.length == 0) {
          flag = true;
        }
        item.flowNodeFieldPowerTemplateList.map((param) => {
          arr.push(param.formTemplateId);
        });
        const uniqueArr = Array.from(new Set(arr));
        powerArr.push(uniqueArr);
      });
      powerArr.map((item) => {
        if (item.length != len) {
          flag = true;
        }
      });
      if (flag) {
        this.$message.error('流程中存在表单没有定义可编辑的参数');
        return;
      }
      this.isNextClick = true;
      const flowNodeTemplate = arrayToTree(flowNodeTemplateList, 1, 'sort', 'treeId');
      // console.log('flowNodeTemplate', flowNodeTemplate);
      // return;
      const data = {
        flowName: infoForm.flowName, // 流程名称
        remark: infoForm.remark, // 流程备注
        groupId: infoForm.groupId,
        typeId: selectForm.typeId, // 类型ID
        standardId: selectForm.standardId, // 标准ID
        formTemplateList: forms, // 表单模板
        flowNodeTemplate
      };
      if (!this.id) {
        this.flowTemplateSave(data);
      } else {
        data.id = this.flowId;
        this.flowTemplateUpdate(data);
      }
      setTimeout(() => {
        this.isNextClick = false;
      }, 600);
    },
    // 新增流程
    flowTemplateSave(data) {
      this.$axios.post(
        Api.templateLibrary.flowTemplateSave,
        {
          // platformCode: '600001',
          // formTemplateBizRelevance: {
          //   otherBiz: 'customerCode',
          //   otherBizId: this.$store.state.user.customerCode
          // },
          data
        },
        (res) => {
          if (res.isSuccess) {
            this.$message.success(`成功`);
            this.$router.push({ path: '/flowLibrary' });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 修改流程
    flowTemplateUpdate(data) {
      this.$axios.post(
        Api.templateLibrary.flowTemplateUpdate,
        {
          // platformCode: '600001',
          // formTemplateBizRelevance: {
          //   otherBiz: 'customerCode',
          //   otherBizId: this.$store.state.user.customerCode
          // },
          data
        },
        (res) => {
          if (res.isSuccess) {
            this.$message.success(`成功`);
            this.$router.push({ path: '/flowLibrary' });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleCloseAuth() {
      this.dialogVisibleHasAuth = false;
      this.hasAuthList = [];
      // this.selectFormList = JSON.parse(
      //   JSON.stringify(this.selectFormListDefault)
      // );
    },
    handleClose() {
      this.reset();
    },
    // 重置
    reset() {
      this.dialogVisible = false;
      this.isClick = false;
      this.form = {};
      if (this.$refs.ruleForm) {
        this.$refs.ruleForm.resetFields();
      }
    }
  }
};
</script>
<style scoped lang='scss'>
.workflow-engine {
  position: relative;
  min-height: 600px;
  padding-bottom: 60px;

  ::v-deep .el-dialog.is-fullscreen {
    width: 95%;
    height: 95%;
    margin: 20px auto;
  }

  .btn-box {
    width: 100%;
    position: absolute;
    text-align: center;
    bottom: 10px;
    text-align: center;
  }

  .workflow-engine-con {
    margin-top: 30px;
    text-align: center;
  }

  ::v-deep .icon-input-h {
    .el-input__inner {
      cursor: pointer;
    }
  }

  .icon-w {
    font-size: 14px;
    padding-right: 10px;

    span {
      font-weight: bold;
      color: #f44336;
    }
  }

  .i-t {
    font-weight: bold;
    margin-right: 3px;
    color: #f44336;
  }

  .flow-con {
    display: inline-block;

    .content {
      width: 260px;
      min-height: 80px;
      cursor: pointer;
      // box-sizing: border-box;
      border: 1px solid #f5f5f7;
      border-radius: 4px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);

      .title {
        display: flex;
        -webkit-box-align: center;
        -ms-flex-align: center;
        align-items: center;
        height: 26px;
        line-height: 26px;
        color: #fff;
        text-align: left;
        background: rgb(255, 148, 62);

        span {
          flex: 4;
          font-size: 12px;
          padding: 15px;
        }

        .icon-close {
          flex: 1;
          font-size: 14px;
          text-align: right;
          padding-right: 10px;
        }
      }

      .editable-title {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        border-bottom: 1px dashed transparent;
        background: rgb(255, 148, 62);
      }

      .con {
        font-size: 14px;
        padding: 15px;
        display: flex;

        .node-text {
          flex: 2;
          text-align: left;
          color: #8d8d8d;
        }

        .arrow {
          flex: 1;
          text-align: right;
        }
      }

      &:hover {
        border-color: #3296fa;
        // box-sizing: border-box;
        // box-shadow: 0 0 6px 0 rgba(50, 150, 250, 0.3);
      }
    }

    .end-node-con {
      position: relative;
      width: 260px;

      .end-node {
        border-radius: 50%;
        font-size: 14px;
        color: rgba(25, 31, 37, 0.4);
        // text-align: left;

        .end-node-text {
          margin-top: 5px;
          text-align: center;
        }
      }
    }
  }

  .end-node {
    width: 239px;
    font-size: 16px;
    text-align: center;

    .end-line {
      display: inline-block;
      width: 2px;
      height: 50px;
      background-color: #cacaca;
    }

    .end-text {
      position: relative;
      top: 3px;
      color: #409eff;

      .end-circle {
        .end-node-circle {
          display: inline-block;
          width: 10px;
          height: 10px;
          border-radius: 50%;
          background: #dbdcdc;
        }
      }
    }
  }

  .auth-content {
    .auth-checkbox {
      max-height: 560px;
      overflow: auto;

      p {
        padding: 5px;
      }
    }
  }

  .has-auth-content {
    display: flex;
    border: 1px solid #ebebeb;

    .h-tit {
      font-size: 14px;
      height: 40px;
      line-height: 40px;
      padding-left: 30px;
      border-bottom: 1px solid #ebebeb;
      color: #000;
      background-color: #ebebeb;
    }

    .has-auth-con-left {
      flex: 2;

      ul.has-auth-ul-left {
        font-size: 14px;
        max-height: 560px;
        overflow: auto;
        width: 100%;

        li {
          width: 100%;
          // height: 30px;
          // line-height: 30px;
          padding-top: 15px;
          padding-bottom: 15px;
          padding-left: 10px;
          cursor: pointer;
          overflow: hidden;
          white-space: nowrap;
          text-overflow: ellipsis;

          &:hover {
            background-color: #e4e5e6;
          }

          img {
            width: 16px;
            vertical-align: middle;
            margin-right: 10px;
          }
        }

        .active {
          background-color: #e4e5e6;
        }
      }
    }

    .has-auth-con-right {
      flex: 3;
      border-left: 1px solid #ebebeb;

      ul {
        max-height: 560px;
        overflow: auto;

        li {
          font-size: 14px;
          padding: 15px;
          border-bottom: 1px solid #ebebeb;

          span {
            display: inline-block;
            width: 130px;
            padding-left: 10px;
            padding-right: 5px;
            cursor: pointer;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
          }

          &:last-child {
            border-bottom: 0;
          }
        }
      }
    }
  }

  // 滚动条样式
  .scroll-bar::-webkit-scrollbar {
    width: 9px;
    height: 6px;
  }

  .scroll-bar::-webkit-scrollbar-track {
    background: rgb(239, 239, 239);
    border-radius: 2px;
  }

  .scroll-bar::-webkit-scrollbar-thumb {
    background: #bfbfbf;
    border-radius: 10px;
  }

  .scroll-bar::-webkit-scrollbar-thumb:hover {
    background: #333;
  }

  .scroll-bar::-webkit-scrollbar-corner {
    background: #179a16;
  }
}
</style>
