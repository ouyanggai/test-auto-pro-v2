<!--
 * @Descripttion:选择项目弹窗
 * @Author: zhengzetao
 * @Date: 2022-04-29
-->

<template>
  <el-dialog
    :visible="visible"
    title="选择项目"
    width="60%"
    :close-on-click-modal="false"
    class="adjust-department-dialog"
    @close='handleClose'
    style="min-height:400px;"
  >
    <div v-show="active == 1">
      <dy-table
        :fetchData="getNoData"
        :keys="projectColKey"
        :list='assoicaProject'
        :showCheckBox="true"
        :isShowBorder="true"
        :maxTableHeight="500"
        @selectDataEvent="selectProjectCheckBox"
        ref="selectProjectTable"
        style="margin-top: 0px;padding: 0px;"
      ></dy-table>
    </div>
    <div v-show="active == 2">
      <div class="info-wrap">
        <div class="info-item">
          <div class="info-item-title">项目名称</div>
          <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].name : ''  }}</div>
        </div>
        <div class="info-item">
          <div class="info-item-title">项目经理</div>
          <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].managerName : '' }}</div>
        </div>
        <div class="info-item">
          <div class="info-item-title">范围</div>
          <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].scope : '' }}</div>
        </div>
      </div>
      <div class="info-wrap">
        <div class="info-item">
          <div class="info-item-title">地点</div>
          <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].address : ''  }}</div>
        </div>
        <div class="info-item">
          <div class="info-item-title">项目概况</div>
          <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].basicInfo : '' }}</div>
        </div>
        <div class="info-item">
          <!-- <div class="info-item-title">范围</div> -->
          <!-- <div>{{ projectCheckBoxData[0] ? projectCheckBoxData[0].scope : '' }}</div> -->
        </div>
      </div>
    </div>
    <span slot="footer">
      <el-button
        v-if="active < 2"
        @click="nextStep"
      >下一步</el-button>
      <el-button
        v-if="active == 2"
        @click="preStep"
      >上一步</el-button>
      <el-button
        v-if="active == 2"
        type="primary"
        @click="postAdjustDepartment"
      >确 定</el-button>
      <!-- <el-button @click="handleClose">取 消</el-button> -->
    </span>
  </el-dialog>

</template>

<script>
import DyTable from '@/components/DyTable';
import Api from '@/api';

export default {
  name: '',
  components: { DyTable },
  data() {
    return {
      active: 1,
      projectList: [
      ],
      projectColKey: {
        name: '项目名称',
        managerName: '项目经理'
        // name: {
        //   label: '项目名称'
        //   // width: 160
        // },
        // managerName: {
        //   label: '项目经理'
        //   // width: 160
        // }
        // address: {
        //   label: '项目周期'
        //   // width: 160
        // }
      },
      projectCheckBoxData: [],
      isShowBorder: true
    };
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    associateId: {
      type: [Number, String],
      default: ''
    },
    assoicaProject: {
      type: Array,
      default: function () {
        return [];
      }
    }
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    const arr = this.assoicaProject.find(x => x.id == this.associateId);
    this.$nextTick(x => {
      this.$refs.selectProjectTable.handleToggleRowSelection(arr);
    });
  },
  methods: {
    nextStep() {
      if (!this.projectCheckBoxData.length) {
        this.$message.warning('请先勾选项目！');
        return;
      }
      this.active++;
    },
    preStep() {
      this.active--;
    },
    selectProjectCheckBox(data) { // 只允许选择一个
      if (data.length > 1) {
        const newArr = data[1];
        data.shift();
        this.$refs.selectProjectTable.handleClearSelection();
        this.$refs.selectProjectTable.handleToggleRowSelection(newArr);
      }
      this.projectCheckBoxData = data;
    },
    getNoData() {

    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    postAdjustDepartment() {
      this.$emit('selectProject', this.projectCheckBoxData);
      this.handleClose();
    }
  }
};

</script>
<style lang='scss' scoped>
::v-deep .el-table__header .el-checkbox {
  visibility: hidden;
}
.info-wrap {
  display: flex;
  .info-item {
    margin-bottom: 20px;
    flex: 1;
    .info-item-title {
      font-weight: bold;
      margin-bottom: 10px;
    }
  }
}
</style>
