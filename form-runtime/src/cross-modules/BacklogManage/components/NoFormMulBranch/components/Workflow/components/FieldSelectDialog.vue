<!--
 * @description: 节点字段的选择
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    title="选择字段"
    :visible="visible"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    width="70%"
    top="100px"
    append-to-body
    center
    @close="handleClose"
  >
    <div class="process-dialog">
      <h4 style="margin-bottom:10px;">{{ dictName }}</h4>
      <el-checkbox  style="margin-bottom:5px;" :indeterminate="isIndeterminate" v-model="checkAll" @change="handleCheckAllChange">全选</el-checkbox>
      <ul v-show="fieldTreeList.length" class="field-ul">
        <el-checkbox-group
          v-model="fieldSelectList"
          style="display: flex;justify-content: space-between;flex-flow: wrap;"
          @change="handleCheckedChange"
        >
          <li
            v-for="(item, key) in fieldTreeList"
            :key="key"
            class="field-li"
            style="margin-bottom:5px;width: 50%;min-width:200px;"
          >
            <el-checkbox :label="item.dictValue">{{ item.dictLabel }} / {{ item.dictValue }}</el-checkbox>
          </li>
        </el-checkbox-group>

      </ul>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button v-if="editType != 3" type="primary" @click="submit">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import { localstorageGet } from '@/utils/auth';
import api from '@/api/index';

export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    fieldCheckList: {
      type: Array,
      default: () => {
        return [];
      }
    },
    nodeType: {
      type: String,
      default: ''
    },
    editType: {
      type: [String, Number],
      default: ''
    }
  },
  data() {
    return {
      fieldSelectList: [],
      fieldTreeList: [],
      dictName: '',
      checkAll:false,
      isIndeterminate:false
    };
  },
  computed: {},
  watch: {},
  created() {
    this.initFieldList();
    this.initDicTree();
  },
  mounted() {
    if (this.nodeType == 'normalNode') {
      this.fieldSelectList = JSON.parse(JSON.stringify(this.fieldCheckList));
    } else if (this.nodeType == 'conditionNode') {
      this.fieldSelectList = JSON.parse(JSON.stringify(this.fieldCheckList));
    }
  },
  methods: {
    handleCheckAllChange(val){
      this.fieldSelectList = val ? this.fieldTreeList.map(item=>{return item.dictValue}) : [];
      this.isIndeterminate = false;
    },
    // 数据字典大类
    initDicTree() {
      this.$axios.post(api.algorithm.list, {
        data: {

        },
        pages: 1,
        size: 1000
      }, res => {
        if (res.isSuccess) {
          this.dictName = res.data.dataList.find(x => x.dictCode == (this.$parent.step1AuditWay))?.dictName;
        } else {
          this.$message.error(res.message);
        }
      });
    },
    // 数据字典大类下的字段列表
    initFieldList() {
      this.$axios.post(api.algorithm.getDicCodeTree, {
        data: {
          dictCode: localstorageGet('step1AuditWay')// this.$parent.step1AuditWay
        }
      }, res => {
        if (res.isSuccess) {
          this.fieldTreeList = res.data;
        } else {
          this.$message.error(res.message);
        }
      });
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    handleCheckedChange(val) {
      if (this.nodeType == 'normalNode') {

      } else if (this.nodeType == 'conditionNode') { // 只能选一个字段
        if (this.fieldSelectList.length > 1) {
          this.fieldSelectList.shift();
        }
      }
      let checkedCount = val.length;
      this.checkAll = checkedCount === this.fieldTreeList.length;
      this.isIndeterminate = checkedCount > 0 && checkedCount < this.fieldTreeList.length;
    },
    submit() {
      if (this.nodeType == 'normalNode') {
        this.$emit('select', this.fieldSelectList);
      } else if (this.nodeType == 'conditionNode') {
        let obj = {};
        this.fieldSelectList.forEach(x => {
          obj = this.fieldTreeList.find(y => y.dictValue == x);
        });
        this.$emit('checkConditionField', obj);
      }

      this.handleClose();
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
</style>
