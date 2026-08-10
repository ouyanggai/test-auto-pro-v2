<template>
  <div class="powerSet">
    <div class="query">
      <span>关键字查询</span>
      <el-input v-model="input" size="small" placeholder="请输入关键字"></el-input>
      <el-button type="success" size="small" @click="queryFunc">搜索</el-button>
      <el-button type="primary" size="small" @click="addFunc">新增</el-button>
      <el-button type="danger" size="small" @click="delFunc">删除</el-button>
    </div>
    <el-table
      ref="multipleTable"
      :data="tableData"
      style="width: 100%"
      @selection-change="handleSelectionChange">
      <el-table-column
        type="selection"
        width="55">
      </el-table-column>
      <el-table-column
        prop="name"
        label="姓名"
        width="120">
      </el-table-column>
      <el-table-column
        prop="phone"
        label="手机号"
        width="120"
        align="center">
      </el-table-column>
      <el-table-column
        prop="companyJurisdiction"
        label="公司权限">
      </el-table-column>
      <el-table-column
        prop="projectJurisdiction"
        label="项目权限">
      </el-table-column>
      <el-table-column
        label="操作"
        width="120"
        align="center">
        <template slot-scope="scope">
            <el-link :underline="false" type="primary" @click="editFunc(scope.row)">分配权限</el-link>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog
      :title="isAddOrEdit?'新增权限':'编辑权限'"
      :visible.sync="dialogVisible"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      width="810px">
      <el-tabs type="border-card">
        <el-tab-pane label="公司预算权限" class="middle">
          <el-row>
            <el-col :span="6" style="text-align: right;margin-bottom: 25px;">选择用户：</el-col>
            <el-col :span="18" style="margin-bottom: 25px;">
              <el-cascader
                :options="options"
                style="width:420px;"
                :disabled="!isAddOrEdit"
              ></el-cascader>
            </el-col>
            <el-col :span="6" style="text-align: right;">分配数据权限：</el-col>
            <el-col :span="18">
              <el-tree
                :data="data"
                show-checkbox
                node-key="id"
                style="width:420px;margin-bottom: 20px;"
                :default-expanded-keys="[5]"
                :default-checked-keys="[5]"
                :props="defaultProps">
              </el-tree>
            </el-col>
          </el-row>
        </el-tab-pane>
        <el-tab-pane label="项目预算权限" class="middle">

        </el-tab-pane>
      </el-tabs>
      <span slot="footer" class="dialog-footer">
        <el-button @click="dialogVisible = false">取 消</el-button>
        <el-button type="primary" @click="dialogVisible = false">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
export default {
  name: 'PowerSet',
  components: {

  },
  data() {
    return {
      isBudgetOrFinance: true, // 是预算管理中还是财务管理中的界面 true：预算管理 ； false 财务管理
      input: '',
      tableData: [{
        name: '王小虎',
        phone: '13800000007',
        companyJurisdiction: [
          {
            year: 2020
          }
        ],
        projectJurisdiction: ['大唐一期', '大唐二期']
      }],
      multipleSelection: [],
      dialogVisible: false,
      isAddOrEdit: true, // 权限是新增还是编辑 true：新增   false：编辑
      options: [{
        value: 'zhinan',
        label: '指南',
        children: [{
          value: 'shejiyuanze',
          label: '设计原则',
          children: [{
            value: 'yizhi',
            label: '一致',
            disabled: true
          }, {
            value: 'fankui',
            label: '反馈'
          }, {
            value: 'xiaolv',
            label: '效率'
          }, {
            value: 'kekong',
            label: '可控'
          }]
        }, {
          value: 'daohang',
          label: '导航',
          children: [{
            value: 'cexiangdaohang',
            label: '侧向导航'
          }, {
            value: 'dingbudaohang',
            label: '顶部导航'
          }]
        }]
      }, {
        value: 'zujian',
        label: '组件',
        children: [{
          value: 'basic',
          label: 'Basic',
          children: [{
            value: 'layout',
            label: 'Layout 布局'
          }, {
            value: 'color',
            label: 'Color 色彩'
          }, {
            value: 'typography',
            label: 'Typography 字体'
          }, {
            value: 'icon',
            label: 'Icon 图标'
          }, {
            value: 'button',
            label: 'Button 按钮'
          }]
        }, {
          value: 'form',
          label: 'Form',
          children: [{
            value: 'radio',
            label: 'Radio 单选框'
          }, {
            value: 'checkbox',
            label: 'Checkbox 多选框'
          }, {
            value: 'input',
            label: 'Input 输入框'
          }, {
            value: 'input-number',
            label: 'InputNumber 计数器'
          }, {
            value: 'select',
            label: 'Select 选择器'
          }, {
            value: 'cascader',
            label: 'Cascader 级联选择器'
          }, {
            value: 'switch',
            label: 'Switch 开关'
          }, {
            value: 'slider',
            label: 'Slider 滑块'
          }, {
            value: 'time-picker',
            label: 'TimePicker 时间选择器'
          }, {
            value: 'date-picker',
            label: 'DatePicker 日期选择器'
          }, {
            value: 'datetime-picker',
            label: 'DateTimePicker 日期时间选择器'
          }, {
            value: 'upload',
            label: 'Upload 上传'
          }, {
            value: 'rate',
            label: 'Rate 评分'
          }, {
            value: 'form',
            label: 'Form 表单'
          }]
        }, {
          value: 'data',
          label: 'Data',
          children: [{
            value: 'table',
            label: 'Table 表格'
          }, {
            value: 'tag',
            label: 'Tag 标签'
          }, {
            value: 'progress',
            label: 'Progress 进度条'
          }, {
            value: 'tree',
            label: 'Tree 树形控件'
          }, {
            value: 'pagination',
            label: 'Pagination 分页'
          }, {
            value: 'badge',
            label: 'Badge 标记'
          }]
        }, {
          value: 'notice',
          label: 'Notice',
          children: [{
            value: 'alert',
            label: 'Alert 警告'
          }, {
            value: 'loading',
            label: 'Loading 加载'
          }, {
            value: 'message',
            label: 'Message 消息提示'
          }, {
            value: 'message-box',
            label: 'MessageBox 弹框'
          }, {
            value: 'notification',
            label: 'Notification 通知'
          }]
        }, {
          value: 'navigation',
          label: 'Navigation',
          children: [{
            value: 'menu',
            label: 'NavMenu 导航菜单'
          }, {
            value: 'tabs',
            label: 'Tabs 标签页'
          }, {
            value: 'breadcrumb',
            label: 'Breadcrumb 面包屑'
          }, {
            value: 'dropdown',
            label: 'Dropdown 下拉菜单'
          }, {
            value: 'steps',
            label: 'Steps 步骤条'
          }]
        }, {
          value: 'others',
          label: 'Others',
          children: [{
            value: 'dialog',
            label: 'Dialog 对话框'
          }, {
            value: 'tooltip',
            label: 'Tooltip 文字提示'
          }, {
            value: 'popover',
            label: 'Popover 弹出框'
          }, {
            value: 'card',
            label: 'Card 卡片'
          }, {
            value: 'carousel',
            label: 'Carousel 走马灯'
          }, {
            value: 'collapse',
            label: 'Collapse 折叠面板'
          }]
        }]
      }, {
        value: 'ziyuan',
        label: '资源',
        children: [{
          value: 'axure',
          label: 'Axure Components'
        }, {
          value: 'sketch',
          label: 'Sketch Templates'
        }, {
          value: 'jiaohu',
          label: '组件交互文档'
        }]
      }],
      data: [{
        id: 1,
        label: '一级 1',
        children: [{
          id: 4,
          label: '二级 1-1',
          children: [{
            id: 9,
            label: '三级 1-1-1'
          }, {
            id: 10,
            label: '三级 1-1-2'
          }]
        }]
      }, {
        id: 2,
        label: '一级 2',
        children: [{
          id: 5,
          label: '二级 2-1'
        }, {
          id: 6,
          label: '二级 2-2'
        }]
      }, {
        id: 3,
        label: '一级 3',
        children: [{
          id: 7,
          label: '二级 3-1'
        }, {
          id: 8,
          label: '二级 3-2'
        }]
      }],
      defaultProps: {
        children: 'children',
        label: 'label'
      }
    };
  },
  watch: {

  },
  computed: {

  },
  methods: {
    queryFunc() {},
    addFunc() {
      this.isAddOrEdit = true;
      this.dialogVisible = true;
    },
    editFunc(item) {
      this.isAddOrEdit = false;
      this.dialogVisible = true;
      console.log(item);
    },
    delFunc() {
      if (!this.multipleSelection.length) {
        this.$message.warning('请选择需要删除的项');
        return false;
      }
      this.$confirm('检测到未保存的内容，是否在离开页面前保存修改？', '确认信息', {
        closeOnClickModal: false,
        distinguishCancelAndClose: true,
        confirmButtonText: '保存',
        cancelButtonText: '放弃修改'
      })
        .then(() => {
          this.$message({
            type: 'info',
            message: '保存修改'
          });
        })
        .catch(action => {
          this.$message({
            type: 'info',
            message: action === 'cancel'
              ? '放弃保存并离开页面'
              : '停留在当前页面'
          });
        });
    },
    handleSelectionChange(val) {
      this.multipleSelection = val;
    }
  },
  created() {
    this.isBudgetOrFinance = this.$route.path.includes('groupBudgetManage');
  },
  mounted() {

  },
  updated() {

  },
  destroyed() {

  }
};
</script>

<style lang="scss" scoped>
.powerSet{
  overflow: hidden;
  background: white;
  display: flow-root;
  height: 100%;
  padding: 25px;
  .query{
    display: flex;
    height: 30px;
    margin-bottom: 24px;
    &>span{
      display: inline-block;
      width: 80px;
      line-height: 32px;
    }
    &>div{
      width: 240px;
      margin-right: 10px;
    }
  }
  .middle{
    min-height: 240px;display: flex;
    align-items: center;
  }
}
::v-deep .el-dialog__body{
  padding: 15px 30px !important;
}
</style>
