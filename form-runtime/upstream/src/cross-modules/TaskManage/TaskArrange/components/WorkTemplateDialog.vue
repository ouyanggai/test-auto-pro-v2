<!--
 * @description:
 * @Author: Calvin
 * @Date: 2022-04-02 10:18:46
 * @FilePath: \src\views\TaskManage\TaskArrange\components\WorkTemplateDialog.vue
-->
<template>
  <el-dialog
    :visible="visible"
    title="任务模板"
    width="70%"
    :close-on-click-modal="false"
    class="adjust-department-dialog"
    @close='handleClose'
  >
    <div class="work-target-container">
      <div class="left-wrap">
        <ul style="list-style:none">
          <li>任务模板名称</li>
          <li
            v-for="(item,index) in workList"
            :key="index"
          >
            <el-radio
              class="radio-wrap"
              v-model="workItemRadio"
              :label="item.id"
              :title="item.name"
              @change="chooseRadio"
            >{{item.name}}</el-radio>
          </li>
        </ul>
      </div>
      <div class="right-wrap">
        <div style="margin-bottom:10px;font-weight: 600;">任务要求</div>
        <el-table
          :data="workDetailList"
          ref="table"
          border
          align="center"
          style="width: 100%"
        >
          <el-table-column label="任务名称">
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.name"
              >
              </el-input>
            </template>
          </el-table-column>
          <el-table-column label="任务要求">
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.standard"
              >
              </el-input>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="handleSubmit"
      >确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    workTemplateId: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      workList: [],
      workId: '',
      workDetailList: [],
      workItemRadio: '',
      selectDatas: []
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    this.getWorkList();
  },
  methods: {
    getWorkList() {
      const data = {};
      this.$axios.post(
        Api.taskManage.taskTemplate.planTemplateList,
        {
          data
        },
        res => {
          if (res.isSuccess) {
            this.workList = res.data || [];
            if (this.workTemplateId) {
              this.workList.forEach(item => {
                if (item.id == this.workTemplateId) {
                  this.workItemRadio = item.id;
                  this.chooseRadio(item.id);
                }
              });
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    chooseRadio(id) {
      this.workId = id;
      this.getWorkDetail(id);
    },
    getWorkDetail(id) {
      this.$axios.post(
        Api.taskManage.taskTemplate.planTemplateDetail,
        {
          data: {
            id
          }
        },
        res => {
          if (res.isSuccess) {
            this.workDetailList = res.data.templateItemList;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleSubmit() {
      this.$emit('workTemplateSelect', this.workDetailList);
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-table__header .el-checkbox {
  visibility: hidden;
}
.work-target-container {
  display: flex;
  min-height: 400px;
  user-select: none;
  .left-wrap {
    min-width: 170px;
    border-right: 1px solid #f2f2f2;
    li {
      padding-left: 10px;
      line-height: 40px;
      border-bottom: 1px solid #f2f2f2;
      &:first-child {
        padding-left: 10px;
        font-weight: 600;
      }
    }
    .radio-wrap {
      width: 160px;
      overflow: hidden;
      display: inline-block;
      text-overflow: ellipsis;
      vertical-align: middle;
    }
  }
  .right-wrap {
    flex: 1;
    padding: 10px 22px;
  }
}
</style>
