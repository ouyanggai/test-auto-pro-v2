<!--
 * @Author: junshao
 * @Date: 2021-11-23 10:17:01
 * @LastEditors: junshao
 * @LastEditTime: 2021-12-02 16:47:10
 * @Description: 选择个人提升目标
-->
<template>
  <el-dialog
    :visible="visible"
    title="个人提升目标"
    width="1200px"
    :close-on-click-modal="false"
    class="adjust-department-dialog"
    @close='handleClose'
  >
    <div class="grow-target-container">
      <div class="left-wrap">
        <div class="header-txt">指标类型</div>
        <ul
          class="fix-height"
          style="list-style:none"
        >
          <li
            v-for="item in indicatorTypeList"
            :class="{'activeLi': item.id == activeIndicatorId}"
            :key="item.id"
            @click="handleCheckIndicator(item.id)"
          >{{item.name}}
          </li>
        </ul>
      </div>
      <div class="work-item-wrap">
        <div class="header-txt">工作项</div>
        <ul
          v-if="activeIndicatorId"
          class="fix-height"
          style="list-style:none"
        >
          <li
            v-for="item in workItemList"
            :class="{'activeLi': item.id == activeWorkItemId}"
            :key="item.id"
            @click="handleCheckWorkItem(item.id)"
          >{{item.name}}
          </li>
        </ul>
      </div>
      <div class="skill-wrap">
        <div class="header-txt">工作技能</div>
        <div
          v-if="!workSkillList.length"
          style="text-align:center;margin-top:15px;color:#ccc"
        >尚未给该岗位设置指标</div>
        <el-checkbox-group
          v-if="activeWorkItemId && workSkillList.length>0"
          class="wrap-content fix-height"
          v-model="skillCheckList"
        >
          <el-checkbox
            :disabled="arrangeType=='check'"
            v-for="item in workSkillList"
            :key="item.kpiId"
            :label="item.kpiId"
            style="display: block;line-height: 36px;"
          >{{item.kpiName}}</el-checkbox>
        </el-checkbox-group>
      </div>
      <div class="ability-wrap">
        <div class="header-txt">匹配能力</div>
        <div
          v-if="!workSkillList.length"
          style="text-align:center;margin-top:15px;color:#ccc"
        >尚未给该岗位设置指标</div>
        <el-checkbox-group
          v-if="activeWorkItemId && workAbilityList.length>0"
          class="wrap-content fix-height"
          v-model="abilityCheckList"
        >
          <div
            v-for="item in workAbilityList"
            :key="item.kpiId"
          >
            <el-checkbox
              :disabled="arrangeType=='check'"
              :label="item.kpiId"
              style="display: block;"
            >{{item.kpiName}}</el-checkbox>
            <div class="item-note">{{item.powerStandard}}</div>
          </div>
        </el-checkbox-group>
      </div>
    </div>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="handleSubmitData"
      >确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    arrangeType: { // 下发操作类型 check-add-edit
      type: String,
      default: ''
    },
    dutyId: { // 负责人岗位id
      type: String,
      default: ''
    },
    userId: { // 负责人id
      type: String,
      default: ''
    },
    checkedSkillList: {
      type: Array,
      default: () => {
        return [];
      }
    },
    checkedAbilityList: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    return {
      activeIndicatorId: '',
      activeWorkItemId: '',
      skillCheckList: [...this.checkedSkillList],
      abilityCheckList: [...this.checkedAbilityList],
      indicatorTypeList: [],
      workItemList: [],
      workSkillList: [],
      workAbilityList: []
    };
  },
  watch: {
  },
  created() { },
  mounted() {
    this.getIndicatorTypeList();
  },
  methods: {
    getIndicatorTypeList() { // 根据岗位id查指标类型
      const data = {
        duty: {
          id: this.dutyId
        }
      };
      this.$axios.post(
        Api.taskManage.myTask.getIndicatorListWithDuty,
        {
          data
        },
        res => {
          if (res.isSuccess) {
            this.indicatorTypeList = res.data;
            // 默认激活第一项
            this.activeIndicatorId = this.indicatorTypeList[0].id;
            this.getWorkItemList(this.activeIndicatorId);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getWorkItemList(id) { // 根据岗位id和指标类型id查工作项
      if (id) {
        const data = {
          workTargetType: {
            id: id
          },
          duty: {
            id: this.dutyId
          }
        };
        this.$axios.post(
          Api.taskManage.myTask.getWorkItemListWithType,
          {
            data
          },
          res => {
            if (res.isSuccess) {
              this.workItemList = res.data;
              this.activeWorkItemId = this.workItemList[0].id;
              this.getItemKpiList(this.activeWorkItemId);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }
    },
    getItemKpiList(id) { // 根据工作项id和岗位id查询技能能力指标
      if (id) {
        const data = {
          workItemId: id,
          positionId: this.dutyId,
          userId: this.userId
        };
        this.$axios.post(
          Api.taskManage.taskArrange.getItemKpiList,
          {
            data
          },
          res => {
            if (res.isSuccess) {
              this.workSkillList = res.data.skillList;
              this.workAbilityList = res.data.powerList;
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }
    },
    handleCheckIndicator(id) {
      if (this.activeIndicatorId == id) {
        return false;
      } else {
        this.activeIndicatorId = id;
        this.activeWorkItemId = '';
        this.getWorkItemList(id);
      }
    },
    handleCheckWorkItem(id) { // 切换工作项查询对应的kpi
      if (this.activeWorkItemId == id) {
        return false;
      } else {
        this.activeWorkItemId = id;
        this.getItemKpiList(id);
      }
    },
    handleSubmitData() {
      this.$emit('setGrowTargetData', this.skillCheckList, this.abilityCheckList);
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};

</script>
<style lang='scss' scoped>
.grow-target-container {
  display: flex;
  height: 400px;
  .header-txt {
    line-height: 38px;
    font-weight: bold;
    border-bottom: 1px solid #ddd;
    text-align: center;
  }
  .fix-height {
    height: 360px;
    overflow-y: auto;
  }
  .left-wrap,
  .work-item-wrap {
    text-align: center;
    width: 180px;
    height: 100%;
    border-right: 1px solid #ddd;
    ul {
      li {
        width: 100%;
        line-height: 36px;
      }
      li:hover {
        cursor: pointer;
        background-color: #edf8fd;
      }
      .activeLi {
        background-color: #e6f7ff;
        color: #1890ff;
        border-right: 3px solid #1890ff;
      }
    }
  }
  .work-item-wrap {
    width: 240px;
  }
  .skill-wrap,
  .ability-wrap {
    .wrap-content {
      padding: 5px 10px;
      font-size: 14px;
    }
  }
  .skill-wrap {
    width: 300px;
    border-right: 1px solid #ddd;
  }
  .ability-wrap {
    width: 480px;
    .item-note {
      color: #a8a5a5;
      padding-left: 25px;
      margin-bottom: 10px;
    }
  }
}

::v-deep .el-dialog__header {
  border-bottom: 1px solid #f2f2f2;
  cursor: default;
}
::v-deep .el-dialog__footer {
  border-top: 1px solid #f2f2f2;
}
::v-deep .el-dialog__body {
  padding: 0px;
}
</style>
