<!--
 * @description:  选择范围
 * @Author: zhengzetao
 * @Date: 2024-12-27
-->
<template>
  <el-dialog
    :visible="visible"
    title="选择范围"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    append-to-body
    center
    top="100px"
    width="960px"
    @close="handleClose"
  >
    <el-tabs v-model="activeName" @tab-click="handleClick">
      <el-tab-pane label="公司" name="company">
        <TransferPerson v-if="activeName == 'company'" :defaultCheckedKeys="defaultCheckedKeys" :activeName="activeName" @getSelectPerson="getSelectPerson"></TransferPerson>
      </el-tab-pane>
      <el-tab-pane label="部门" name="department">
        <TransferPerson v-if="activeName == 'department'" :defaultCheckedKeys="defaultCheckedKeys" :activeName="activeName" @getSelectPerson="getSelectPerson"></TransferPerson>
      </el-tab-pane>
      <el-tab-pane label="人员" name="personnel">
        <TransferPerson v-if="activeName == 'personnel'" :defaultCheckedKeys="defaultCheckedKeys" :activeName="activeName" @getSelectPerson="getSelectPerson"></TransferPerson>
      </el-tab-pane>
      <el-tab-pane label="角色" name="role">
        <TransferPerson v-if="activeName == 'role'" :defaultCheckedKeys="defaultCheckedKeys" :activeName="activeName" @getSelectPerson="getSelectPerson"></TransferPerson>
      </el-tab-pane>
      <el-tab-pane label="岗位" name="position">
        <TransferPerson v-if="activeName == 'position'" :defaultCheckedKeys="defaultCheckedKeys" :activeName="activeName" @getSelectPerson="getSelectPerson"></TransferPerson>
      </el-tab-pane>
    </el-tabs>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="submit">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils';
import TransferPerson from '@/views/flowLibrary/components/TransferPerson.vue';

export default {
  name: '',
  components: {TransferPerson},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    rangeActiveType: {
      type: String,
      default: 'company'
    },
    defaultCheckedKeys:{
      type:Array,
      default: function(){
        return [];
      }
    }
  },
  data() {
    return {
      // activeName: 'company',
      activeName: '',
      selectedData:{}
    };
  },
  computed: {
    // activeName:function(){
    //   return this.rangeActiveType
    // }
  },
  watch: {},
  created() {
    // console.log('this.rangeActiveType',this.rangeActiveType)
    // console.log('this.defaultCheckedKeys',this.defaultCheckedKeys)
    this.activeName = this.rangeActiveType ? JSON.parse(JSON.stringify(this.rangeActiveType)) : 'company';
    // this.getJobTitleTree();
  },
  mounted() {
  },
  methods: {
    handleClick(tab, event) {
      // console.log(tab, event);
    },
    getSelectPerson(checkData){
      this.selectedData = {
        'rangeType': this.activeName,
        'checkData':checkData
      }
    },
    getSelectDuty(){

    },
    handleClose() {
      this.$emit('update:visible', false);
    },

    submit() {
      console.log('submit');
      console.log('this.selectedData',this.selectedData)
      this.$emit('handleSelectRange', this.selectedData);
      this.handleClose();
      // const checkNode = this.$refs.personSelectTree.getCheckedNodes();
      // const personList = checkNode.filter(x => x.type == 4);
      // this.$emit('select', personList);
      // this.handleClose();
    },
    // getJobTitleTree() { // 获取岗位树
    //   this.$axios.post(
    //     Api.taskManage.taskArrange.getJobTitleTree,
    //     {
    //       data: {
    //         flag: 4,
    //         id: this.company || localstorageGet('companyId'), // 公司id
    //         customerCode: localstorageGet('customerCode') ?? undefined // customerCode
    //       }
    //     },
    //     res => {
    //       if (res.isSuccess) {
    //         this.treeData = res.data;
    //         this.defaultFirstLevelId = [res.data[0].id];
    //       } else {
    //         this.$message.error(res.message);
    //       }
    //     }
    //   );
    // }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
::v-deep {
  .el-dialog--center .el-dialog__body {
    // height: 540px;
    padding-top: 0px !important;
    padding-bottom: 0px !important;
  }
}
</style>
