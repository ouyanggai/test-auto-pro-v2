
<template>
  <div
    v-loading="loading"
    class="form-container"
  >
    <el-form
      :model="selectForm"
      inline
      style="margin-top:20px"
    >
      <el-form-item label="类型类别：">
        <el-select
          v-model="selectForm.typeId"
          clearable
          placeholder="请选择所属类型"
          :disabled="type==1 || type==2"
          @change="typeChange"
        >
          <el-option
            v-for="item in typeList"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="标准类别：">
        <el-select
          v-model="selectForm.standardId"
          placeholder="请选择所属标准"
          :disabled="type==1 ||type==2"
          @change="standardChange"
        >
          <el-option
            v-for="item in standardList"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="表单名称：">
        <el-input
          v-model="name"
          style="width:200px"
          placeholder="输入表单名称关键字"
          :disabled="type==1 ||type==2"
        />
        <el-button
          style="margin-left:20px"
          type="primary"
          size="small"
          @click="search"
        >搜索</el-button>
      </el-form-item>
    </el-form>
    <el-divider />
    <h3>选择所需表单</h3>
    <el-divider />
    <el-row
      class="formSelect-wrap"
      :gutter="20"
    >
      <i
        v-if="(tableData.length && showArrowFlag)"
        title="加载下一页"
        class="el-icon-d-arrow-right arrow-icon"
        @click="getNextPageFormList"
      />
      <el-col
        :span="12"
        style="height: 400px;overflow: auto;"
      >
        <el-checkbox
          v-model="checkAll"
          class="mt-10"
          :disabled="type==1"
          @change="handleCheckAllChange"
        >全选
        </el-checkbox>
        <el-checkbox-group
          v-model="checkedList"
          class="checkbox-group"
          :disabled="type==1"
          @change="handleCheckedCitiesChange"
        >
          <el-checkbox
            v-for="(item,key) in tableData"
            :key="key"
            :label="item.id"
            style="width: 90%;"
          >
            <span style="display: inline-block;width: 100%;overflow-x: auto;">{{ item.name }} </span>
            <el-button
              type="text"
              @click="preview(item.id)"
            >查看</el-button>
          </el-checkbox>
        </el-checkbox-group>
      </el-col>
      <el-col
        :span="12"
        style="height: 400px;overflow: auto;"
      >
        <h3 style="margin-bottom:7px">
          已选择表单
        </h3>
        <ul>
          <li
            v-for="(item,key) in selectList"
            :key="key"
            class="select-list"
          >
            {{ item.name }}
          </li>
        </ul>
      </el-col>
    </el-row>
    <div class="btn-box">
      <el-button
        type="primary"
        class="btn"
        @click="handleLast"
      >上一步</el-button>
      <el-button
        type="primary"
        class="btn"
        @click="handleNext"
      >下一步</el-button>
    </div>
    <!-- 查看表单 -->
    <el-dialog
      title="查看表单"
      fullscreen
      center
      :visible.sync="previewDialogVisible"
    >
      <fm-generate-form
        ref="generateForm"
        :data="jsonData"
        :edit-data="editData"
      />
      <span
        slot="footer"
        class="dialog-footer"
      >
        <el-button @click="previewDialogVisible = false">关 闭</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/utils/api.js';
import { mapState } from 'vuex';
import Bus from '@/utils/bus.js';
export default {
  components: {},
  props: {
    selectForm: {
      type: Object,
      default: () => {
        return {
          typeId: '',
          standardId: ''
        };
      }
    },
    formTemplateList: { // 已选择表单
      type: Array,
      default: () => {
        return [];
      }
    },
    flowNodeTemplateList: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    return {
      loading: false,
      type: '',
      templateId: '',
      name: '',
      typeList: [{}],
      standardList: [{}],
      jsonData: {},
      editData: {},
      checkAll: false,
      previewDialogVisible: false,
      selectList: [], // 选择表单
      checkedList: [], // 全部表单
      tableData: [],
      pagination: {
        pages: 1,
        // size: 100
        size: 10
      },
      showArrowFlag: true
    };
  },
  computed: {
    // 获取vuex的标签集合数据
    ...mapState({
      checkedListArr: (state) => state.workflow.selectFormList
    })
  },
  watch: {
    formTemplateList(val) {
      this.checkedList = val.map(item => {
        return item.id;
      });
      this.selectList = JSON.parse(JSON.stringify(val));
    },
    checkedList(val) {
      // this.selectList = [];
      // console.log('this.selectList1', this.selectList);
      // this.checkedList.forEach(item => {
      //   this.tableData.forEach(val => {
      //     if (val.id == item) {
      //       this.selectList.push(val);
      //     }
      //   });
      // });
      // console.log('this.selectList2', this.selectList);
    },
    selectForm(val) {
      if (val.typeId) {
        this.getStandardList();
        if (this.type) {
          this.fetchData();
        }
        // if (this.type) {
        //   this.fetchData();
        //   this.checkedList = this.formTemplateList.map(item => {
        //     return item.id;
        //   });
        //   console.log('this.checkedList', this.checkedList);
        //   this.selectList = JSON.parse(JSON.stringify(this.formTemplateList));
        //   console.log('this.selectList', this.selectList);
        // }
      }
    },
    checkedListArr(val) {
      // console.log('selectFormArr', val);
      // this.selectList = val;
    }
  },
  created() {
    this.type = this.$route.query.type;
  },
  mounted() {
    this.getTypeList();
    if (this.type) {
      // this.getStandardList();
      // console.log('this.selectForm1', this.selectForm);
      // this.fetchData();

      // this.checkedList = this.formTemplateList.map(item => {
      //   return item.id;
      // });
      // console.log('this.checkedList', this.checkedList);
      // this.selectList = JSON.parse(JSON.stringify(this.formTemplateList));
      // console.log('this.selectList', this.selectList);
    }
  },
  methods: {
    getNextPageFormList() {
      this.pagination.pages += 1;
      this.fetchData();
    },
    // 类型
    getTypeList() {
      const data = {
        name: ''
      };
      this.$axios.post(Api.formLibrary.typeList, {
        data,
        pages: 1,
        // platformCode: '600001',
        size: 99999
      }, (res) => {
        if (res.isSuccess) {
          this.typeList = res.data;
        }
      });
    },
    // 标准
    getStandardList() {
      const data = {
        typeId: this.selectForm.typeId
      };
      this.$axios.post(Api.formLibrary.standardList, {
        data,
        pages: 1,
        // platformCode: '600001',
        size: 99999
      }, (res) => {
        if (res.isSuccess) {
          this.standardList = res.data;
        }
      });
    },
    // 查询
    search() {
      if (!this.selectForm.typeId || !this.selectForm.standardId) {
        this.$message.error('请先选择类型类别和标准类别');
        return;
      }
      this.checkAll = false;
      this.fetchData();
    },
    fetchData() {
      this.loading = true;
      // this.checkedList = [];

      // const data = Object.assign({}, this.selectForm);
      console.log('this.selectForm1', this.selectForm);
      const data = Object.assign({
        // ignoreTemplateData: true, // 查询表单传true
        // ignoreTemplateData: false, // 查询字段传false
        customerCode: this.$store.state.user.customerCode
      }, this.selectForm);

      data.name = this.name;
      this.$axios.post(Api.formLibrary.formList, {
        data,
        pages: this.pagination.pages,
        size: this.pagination.size,
        pagination: true,
        platformCode: '999999'
        // platformCode: 'null'
        // platformCode: '600001',
        // size: 99999

      }, (res) => {
        this.loading = false;
        if (res.isSuccess) {
          this.tableData = this.tableData.concat(res.data);
          if (res.data.length < 10) {
            // if (res.data.length < 100) {
            this.showArrowFlag = false;
          }
          // this.checkedList = this.formTemplateList.map(item => {
          //   return item.id;
          // });
          // 暂时注释
          // res.data.map(item => {
          //   this.formTemplateList.map(param => {
          //     if (item.id == param.id) {
          //       this.selectList.push(item);
          //     }
          //   });
          // });
          // this.handleCheckedCitiesChange();// 暂时注释
          // this.tableData.map(item => {
          //   this.formTemplateList.map(param => {
          //     if (item.id == param.id) {
          //       this.selectList.push(item);
          //     }
          //   });
          // });
        }
      });
    },
    typeChange() {
      this.selectForm.standardId = '';
      this.name = '';
      this.checkAll = false;
      this.tableData = [];
      this.checkedList = [];
      this.selectList = [];
      this.getStandardList();
    },
    standardChange() {
      this.tableData = [];
      this.checkedList = [];
      this.selectList = [];
      this.name = '';
      this.checkAll = false;
      this.fetchData();
    },
    handleCheckAllChange(val) {
      if (this.checkAll) {
        this.checkedList = this.tableData.map(item => {
          return item.id;
        });
        this.selectList = this.tableData;
      } else {
        this.checkedList = [];
        this.selectList = [];
      }
    },
    handleCheckedCitiesChange(value) {
      // console.log('handleCheckedCitiesChange');
      // console.log('value', value);
      if (this.checkedList.length == this.tableData.length) {
        this.checkAll = true;
      } else {
        this.checkAll = false;
      }
      this.selectList = [];
      this.checkedList.forEach(item => {
        this.tableData.forEach(val => {
          if (val.id == item) {
            this.selectList.push(val);
          }
        });
      });
    },
    preview(id) {
      const data = {
        id
      };
      this.$axios.post(Api.formLibrary.getFormDetail, {
        // platformCode: '600001',
        data
      }, (res) => {
        if (res.isSuccess) {
          this.jsonData = JSON.parse(res.data.templateData);
          this.previewDialogVisible = true;
          this.$nextTick(() => {
            this.$refs.generateForm.refresh();
          });
        }
      });
    },
    handleLast() {
      this.$emit('updateActive', 0);
    },
    handleNext() {
      const selectForm = this.selectForm;
      if (!selectForm.typeId || !selectForm.standardId) {
        this.checkAll = false;
        this.$message.error('请先选择类型类别和标准类别');
        return;
      }
      if (!this.selectList.length) {
        this.$message.error('请先选择表单');
        return;
      }
      for (var item of this.selectList) {
        // 处理第editData
        item.editData = {};
        if (item.fieldsTemplateList && item.fieldsTemplateList.length) {
          item.fieldsTemplateList.map(param => {
            param.fieldPower = 'only_read';
            this.$set(item.editData, param.name, param.fieldPower == 'edit');
          });
        } else {
          this.$message.error(`${item.name}中字段不存在`);
          return;
        }
      }
      this.$emit('updateActive', 2);
      Bus.$emit('sendCheckedList', this.selectList);
      Bus.$emit('sendSelectForm', this.selectForm);
      if (this.type) {
        Bus.$emit('sendFlowNodes', this.flowNodeTemplateList);
      }
    }
  }
};
</script>

<style scoped lang="scss">
.form-container {
  position: relative;
  width: 100%;
  height: 660px;
  background-color: #fff;

  .btn-box {
    width: 100%;
    text-align: center;
    margin-top: 20px;
    text-align: center;
  }

  ::v-deep .el-dialog.is-fullscreen {
    width: 95%;
    height: 95%;
    margin: 20px auto;
  }

  ::v-deep .el-checkbox-group {
    .el-checkbox {
      display: block;
    }
  }

  .select-list {
    font-size: 14px;
    padding: 8.5px 0;
    color: #606266;
  }

  .formSelect-wrap {
    // height: 400px;
    overflow: auto;
    position: relative;
    .arrow-icon {
      position: fixed;
      z-index: 100;
      bottom: 107px;
      left: 430px;
      transform: rotate(90deg) scale(2);
      font-weight: 600;
      cursor: pointer;
    }
  }
  ::v-deep .el-checkbox .el-checkbox__label {
    width: 100%;
  }
}
</style>

<style lang="scss">
// @import "@/assets/styles/formMaking.scss";
</style>
