<template>
  <el-dialog
    width="900px"
    title="岗级选择"
    :visible.sync="innerVisible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    append-to-body
  >
    <div class="dialog-content">
      <div class="left-panel">
        <div class="search-box">
          <el-input placeholder="搜索" v-model="searchKeyword" clearable>
            <i slot="suffix" class="el-input__icon el-icon-search"></i>
          </el-input>
        </div>
        <div class="org-list">
          <el-scrollbar class="org-list-scroll">
            <div
              class="org-item"
              v-for="(item, index) in filteredList"
              :key="index"
              :class="{'active': currentGrade && currentGrade.id === item.id}"
            >
              <div class="item-content">
                <el-checkbox
                  v-model="item.checked"
                  @change="handleCheckChange(item)"
                  @click.stop
                ></el-checkbox>
                <span class="item-name" @click="handleGradeClick(item)">{{ item.name }}</span>
              </div>
            </div>
          </el-scrollbar>
        </div>
      </div>

      <div class="right-panel">
        <div class="panel-title">{{ currentGrade ? currentGrade.name : '未选择' }}下的人员</div>
        <div class="staff-list" v-if="currentGrade">
          <el-scrollbar class="staff-list-scroll">
            <el-tag
              v-for="(staff, index) in staffList"
              :key="index"
              class="staff-tag"
              size="medium"
            >
              {{ staff.name }}
            </el-tag>
          </el-scrollbar>
          <div class="empty-tip" v-if="staffList.length === 0">暂无人员数据</div>
        </div>
        <div class="empty-tip" v-else>请先选择左侧岗级</div>
      </div>
    </div>

    <div slot="footer" class="dialog-footer">
      <el-button @click="handleCancel">取 消</el-button>
      <el-button type="primary" @click="handleConfirm">确 定</el-button>
    </div>
  </el-dialog>
</template>

<script>
import Api from '@/api';

export default {
  name: 'GradeDialong',
  components: {
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    companyId: {
      type: String,
      default: ''
    },
    selectedGrade: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      innerVisible: this.visible,
      selectedList: [],
      searchKeyword: '',
      currentGrade: null,
      gradeList: [
        // { id: 1, name: '关键岗位-集团', checked: false }
      ],
      staffList: [
        // { id: 101, name: '张小平' }
      ],
      returnSelectedGrade: []
    };
  },
  computed: {
    filteredList() {
      if (!this.searchKeyword) return this.gradeList;
      return this.gradeList.filter(item =>
        item.name.toLowerCase().includes(this.searchKeyword.toLowerCase())
      );
    }
  },
  watch: {
    visible(newVal) {
      this.innerVisible = newVal;
      if (newVal) {
        this.getGradeList();
      }
    }
  },
  methods: {
    // 获取全部岗级
    getGradeList() {
      this.$axios.post(
        Api.postInfo.dutyLevel,
        {
          data: {}
        },
        res => {
          console.log(res, 'res');
          this.gradeList = [];
          this.currentGrade = null;
          this.staffList = [];
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              this.gradeList = res.data.map(item => {
                return {
                  id: item.id,
                  name: item.name,
                  checked: false
                };
              });
              this.getGradePersonList(res.data[0].id);
              this.currentGrade = res.data[0];
              // 初始化选中状态
              this.initSelectedItems();
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取指定岗级下当前公司人员
    getGradePersonList(levelId) {
      this.$axios.post(
        Api.newPerformance.findUserByCompanyIdAndDutyLevelIds,
        {
          data: {
            companyId: this.companyId,
            queryDutyLevelIds: [levelId]
            // companyId: '8fe922a5da21445a8a26aba74d0af5e1',
            // queryDutyLevelIds: ['8c0341b2270b4d2ea0003d000d5aa746']
          }
        },
        res => {
          console.log(res, 'res');
          this.staffList = [];
          if (res.isSuccess) {
            if (res.data && res.data.length) {
              this.staffList = res.data.map(item => {
                return {
                  id: item.id,
                  name: item.name
                };
              });
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    initSelectedItems() {
      // 根据传入的已选项设置选中状态
      if (this.selectedGrade && this.selectedGrade.length) {
        const selectedIds = this.selectedGrade.map(item => item.id);
        console.log(selectedIds, 'selectedIds');
        this.gradeList = this.gradeList.map(item => {
          return {
            ...item,
            checked: !!selectedIds.includes(item.id)
          };
        });
        this.returnSelectedGrade = this.gradeList.filter(item => item.checked);
        // this.gradeList.forEach(item => {
        //   if (selectedIds.includes(item.id)) {
        //     item.checked = true;
        //   }
        // });
        console.log(this.gradeList, 'this.gradeList');
        console.log(this.selectedGrade, 'this.selectedGrade');
        console.log(this.returnSelectedGrade, 'this.returnSelectedGrade');
      }
      // 更新已选列表
      this.updateSelectedList();
    },
    handleCheckChange(item) {
      console.log(item, 'item');
      if (item.checked) {
        this.returnSelectedGrade.push(item);
      } else {
        const findIndex = this.returnSelectedGrade.findIndex(grade => item.id == grade.id);
        console.log(findIndex, 'findIndex');
        this.returnSelectedGrade.splice(findIndex, 1);
      }
      this.updateSelectedList();
      // 如果取消选中当前查看的岗级，则清空右侧
      if (!item.checked && this.currentGrade && this.currentGrade.id === item.id) {
        this.currentGrade = null;
      }
    },

    handleGradeClick(item) {
      this.getGradePersonList(item.id);
      this.currentGrade = item;
    },
    updateSelectedList() {
      this.selectedList = this.gradeList.filter(item => item.checked);
    },
    handleCancel() {
      this.$emit('update:visible', false);
    },
    handleConfirm() {
      this.$emit('confirm', this.returnSelectedGrade);
      this.$emit('update:visible', false);
    }
  },
  mounted () {
  }
};
</script>

<style lang='scss' scoped>
.dialog-content {
  display: flex;
  height: 500px;
}

.left-panel, .right-panel {
  height: 100%;
  border: 1px solid #e6e6e6;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.left-panel {
  flex: 0 0 40%;
  margin-right: 20px;
}

.right-panel {
  flex: 1;
}

.search-box {
  padding: 10px;
  border-bottom: 1px solid #e6e6e6;
}

.panel-title {
  padding: 10px;
  font-weight: bold;
  border-bottom: 1px solid #e6e6e6;
  background-color: #f5f7fa;
}

.org-list {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  .org-list-scroll{
    height: 449px;
  }
}

.org-item {
  padding: 0px 10px;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;

  &:hover {
    background-color: #f5f7fa;
  }

  &.active {
    // border-left: 4px solid #1890ff;
    color: #1890ff;
    background-color: #e6f7ff;
  }

  .item-content {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
  }

  .el-checkbox {
    margin-right: 8px;
  }

  .item-name {
    flex: 1;
    cursor: pointer;
    padding: 8px 0px;
  }
}

.staff-list {
  flex: 1;
  padding: 10px;
  overflow-y: auto;
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  .staff-list-scroll{
    height: 434px;
  }
}

.staff-tag {
  margin: 5px;
}

.empty-tip {
  width: 100%;
  height: 100px;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #909399;
  font-size: 14px;
}
</style>
