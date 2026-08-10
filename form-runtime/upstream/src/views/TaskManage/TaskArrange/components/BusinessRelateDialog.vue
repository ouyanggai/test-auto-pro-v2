<!--
 * @Author: junshao
 * @Date: 2022-04-21 16:07:22
 * @LastEditors: Please set LastEditors
 * @LastEditTime: 2024-10-31 17:44:00
 * @Description: file content
-->
<template>
  <div>
    <el-dialog width="90%" h :title="dialogTitle" :visible="visible" :close-on-click-modal="false" top="150px"
      @close="handleClose">
      <!-- 关联手续文件 -->
      <div v-if="isRelateProcedure" class="relate-container">
        <div class="procedure-type">
          <div v-for="type in procedureTypeList" :class="{
            'procedure-type-item': true,
            'active-item': type.id == procedureTypeId,
          }" :key="type.id" @click="selectProcedureType(type)">
            {{ type.name }}
          </div>
        </div>
        <div class="procedure-data">
          <div v-if="procedureTypeId">
            <div v-if="procedureTypeDataList.length">
              <div style="margin-bottom: 15px">手续名称</div>
              <div v-for="item in procedureTypeDataList" style="margin: 5px 0" :key="item.id">
                <el-radio v-model="businessProcedureId" :disabled="item.binded" :label="item.id"
                  @change="getProcedBusinessName(item)">{{ item.type }}</el-radio>
              </div>
            </div>
            <div v-else style="margin: 30px 240px">该手续类型暂无数据</div>
          </div>
          <div v-else style="margin: 30px 240px">请选择要关联的手续类型</div>
        </div>
      </div>
      <!-- 关联开发计划 -->
      <div v-else class="relate-container">
        <el-table ref="multipleTable" :data="tableData" lazy :load="load" tooltip-effect="dark" row-key="id"
          style="width: 100%; overflow: auto" @selection-change="handleSelectionChange" @row-click="clickRow"
          :tree-props="{ children: 'children', hasChildren: 'hasChildren' }">
          <el-table-column type="selection" width="55"> </el-table-column>
          <el-table-column prop="name" label="计划名称" width="350">
          </el-table-column>
          <el-table-column label="负责人" align="center">
            <template slot-scope="scope">
              {{ scope.row.user ? scope.row.user.name : "" }}
            </template>
          </el-table-column>
          <el-table-column prop="startTime" label="计划开始" align="center">
          </el-table-column>
          <el-table-column prop="endTime" label="计划结束" align="center">
          </el-table-column>
        </el-table>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="handleConfirm">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
export default {
  name: 'BusinessDialog',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    title: {
      type: String,
      default: '关联业务'
    },
    projectId: {
      type: String,
      default: ''
    },
    businessTag: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      tableData: [],
      saveTreeObj: {},
      businessProcedureId: '',
      procedureTypeList: [],
      procedureData: {}, // 所有手续数据
      procedureTypeDataList: [], // 类型手续数据
      procedureTypeId: '',
      procedureTypeName: '',
      developPlanId: '', // 选中的计划id
      planName: '' // 选中的计划层级名称
    };
  },
  computed: {
    dialogTitle: function () {
      return '关联' + this.title;
    },
    isRelateProcedure: function () {
      return this.businessTag == 'prophase_procedures';
    }
  },
  watch: {
    visible(val) {
      if (val) {
        if (this.businessTag == 'software_progress_plan') {
          this.getDevelopPlanData();
        } else if (this.businessTag == 'prophase_procedures') {
          this.getProcedureDataList();
        }
      } else {
        this.tableData = [];
        this.saveTreeObj = {};
        this.developPlanId = '';
        this.planName = '';
        this.procedureData = {};
        this.businessProcedureId = '';
        this.procedureTypeId = '';
      }
    }
  },
  created() { },
  mounted() { },
  methods: {
    // 关闭弹窗
    handleClose() {
      this.$emit('update:visible', false);
    },
    // 确定按钮
    handleConfirm() {
      if (this.isRelateProcedure) {
        // 提交关联前期手续
        this.handleSubmitProcedureData();
      } else {
        // 提交关联开发计划
        this.handleSubmitDevelopData();
      }
      // this.$emit('update:visible', false);
    },
    // 提交关联开发任务数据
    handleSubmitDevelopData() {
      if (this.developPlanId == '') {
        this.$message.warning('您没有选择任何一条数据');
        return;
      }
      this.$emit('getBusinessData', this.planName, this.developPlanId);
    },
    // 提交关联前期手续数据
    handleSubmitProcedureData() {
      if (this.businessProcedureId) {
        this.$emit('getBusinessData', this.planName, this.businessProcedureId);
      } else {
        this.$message.warning('您还没有选择数据，请先选择');
      }
    },
    // 获取开发进度列表
    getDevelopPlanData(type) {
      let param = {};
      if (type == 'treeLoad') {
        param = {
          data: {
            pid: this.saveTreeObj.tree.id,
            projectId: this.projectId
          }
        };
      } else {
        param = {
          data: {
            project: {
              id: this.projectId
            }
          }
        };
      }
      this.$axios.post(Api.developProgress.developPlanList, param, (res) => {
        if (res.isSuccess && res.data) {
          res.data.map((x) => {
            x.hasChildren = true;
            return x;
          });
          if (type == 'treeLoad') {
            // 将懒加载的子节点放进tableData
            this.updateTableData(this.tableData, res.data);
            this.saveTreeObj.resolve(res.data);
          } else {
            this.tableData = res.data;
          }
        }
      });
    },
    // 将懒加载的子节点放进tableData
    updateTableData(tableData, data) {
      if (tableData && tableData.length > 0) {
        tableData.map((item) => {
          if (item.id == this.saveTreeObj.tree.id) {
            item.children = data;
          } else if (item.children && item.children.length > 0) {
            this.updateTableData(item.children, data);
          }
        });
      }
    },
    // 懒加载子节点
    load(tree, treeNode, resolve) {
      this.saveTreeObj = {
        tree: tree,
        resolve: resolve
      };
      this.getDevelopPlanData('treeLoad');
    },
    // 点击行事件
    clickRow(row, column, event) { },
    // 选择关联的数据
    handleSelectionChange(val) {
      // 只允许选择一个
      if (val.length > 1) {
        // this.$message.warning('只能关联一项业务');
        this.$refs.multipleTable.clearSelection();
        this.$refs.multipleTable.toggleRowSelection(val.pop());
      } else if (val.length == 1) {
        this.developPlanId = val[0].id;
        if (val[0].pid) {
          this.planName = val[0].name;
          this.getDevelopBusinessName(this.tableData, val[0].pid);
        }
      } else if (val.length == 0) {
        this.developPlanId = '';
        this.planName = '';
      }
    },
    // 获取关联开发计划计划的层级名称
    // getDevelopBusinessName(pid) {
    //   this.tableData.map(item => {
    //     if (item.id == pid) {
    //       this.planName = item.name + '/' + this.planName;
    //       if (item.pid) {
    //         this.getDevelopBusinessName(item.pid);
    //       }
    //     }
    //   });
    // },
    getDevelopBusinessName(data, pid) {
      data.map((item) => {
        if (item.id == pid) {
          this.planName = item.name + '/' + this.planName;
          if (item.pid) {
            this.getDevelopBusinessName(this.tableData, item.pid);
          }
        } else {
          if (item.children && item.children.length) {
            this.getDevelopBusinessName(item.children, pid);
          }
        }
      });
    },
    // 获取关联手续的名称
    getProcedBusinessName(item) {
      this.planName = this.procedureTypeName + '/' + item.type;
    },
    // 切换手续类型
    selectProcedureType(type) {
      this.procedureTypeId = type.id;
      this.procedureTypeName = type.name;
      const ids = [];
      let dataList = [];
      this.procedureData.map((v) => {
        if (v.id == type.id) {
          dataList = v.projectDocTypeTemplateApiVos||[];
        }
      });
      if (dataList.length === 0) {
        this.procedureTypeDataList = [];
        return;
      }
      this.procedureTypeDataList = [];
      dataList.map((item) => {
        ids.push(item.id);
      });
      // 这里要判断手续的数据是否已经被关联过，关联过要禁用
      this.$axios.post(
        Api.developProgress.judgeProcedureBindStatus,
        {
          data: {
            businessIds: ids
          }
        },
        (res) => {
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              dataList.forEach((procedure) => {
                res.data.forEach((item) => {
                  if (procedure.id == item.businessId) {
                    if (item.binded) {
                      procedure.binded = true;
                    } else {
                      procedure.binded = false;
                    }
                  }
                });
              });
              this.procedureTypeDataList = dataList;
            }
          }
        }
      );
    },
    // 获取手续业务的数据
    getProcedureDataList() {
      console.log('getProcedureDataList')
      // console.log('this.projectId',this.projectId)
      this.$axios.post(
        Api.developProgress.getProcedureData,
        {
          data: {
            projectApiVo: {
              id: this.projectId
            },
            formalitiesDeal: false
          }
        },
        (res) => {
          if (res.isSuccess) {
            if (res.data.formalitiesSonApiVos) {
              this.procedureTypeList = res.data.formalitiesSonApiVos.map(
                (item) => {
                  return {
                    name: item.name,
                    id: item.id
                  };
                }
              );
              this.procedureData = res.data.formalitiesSonApiVos;
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }
  }
};
</script>

<style lang="scss" scoped>
.relate-container {
  display: flex;
  height: 400px;

  .procedure-type {
    width: 120px;
    height: 100%;
    overflow-y: auto;
    text-align: center;
    border-right: 1px solid #ccc;

    .procedure-type-item {
      line-height: 32px;
      cursor: pointer;
    }

    .procedure-type-item.active-item,
    .procedure-type-item:hover {
      color: #108ee9;
      background-color: #e6f7ff;
    }
  }

  .procedure-data {
    width: 700px;
    height: 100%;
    padding-left: 30px;
    overflow-y: auto;
  }

  ::v-deep .el-table__body-wrapper {
    height: 360px;
    overflow: auto;
  }
}
</style>
<style lang="scss">
.relate-container {
  ::v-deep .el-table__body-wrapper {
    height: 360px;
    overflow: auto;
  }
}
</style>
