<!--
 * @description:  选择岗级
 * @Author: zhengzetao
 * @Date: 2022-09-14
-->
<template>
  <el-dialog
    :visible="visible"
    title="选择岗级"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    width="500px"
    top="100px"
    append-to-body
    center
    @close="handleClose"
  >
    <!-- <el-tree
      ref="personSelectTree"
      node-key="id"
      :data="treeData"
      :props="defaultProps"
      :default-expand-all="false"
      :default-expanded-keys="defaultFirstLevelId"
      show-checkbox
      :indent="10"
      auto-expand-parent
    >
      <span slot-scope="{node,data}">
        <span>{{ data.name }}</span>
        <span style="color:#ccc;margin-left: 10px;">{{ data.roleName }}</span>
      </span>
    </el-tree> -->
    <!-- 职级 -->
    <el-radio-group v-model="duty" style="width: 100%;">
      <div v-for="val in radioList" class="radio-div">
        <el-radio :label="val.id">{{ val.name }}</el-radio>
      </div>
    </el-radio-group>

    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="submit">确 定</el-button>
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
    examinerId: {
      type: [Number, String],
      default: ''
    },
    auditCondition:{
      type:String,
      default:''
    }
  },
  data() {
    return {
      treeData: [],
      defaultProps: {
        children: 'childrenList',
        label(data) {
          return data.name;
        }
      },
      defaultFirstLevelId: [],
      checkboxList: [],
      company: '',
      radioList:[],
      duty:''
    };
  },
  computed: {},
  watch: {},
  created() {
    if (this.$route.query?.relative) {
      this.company = localstorageGet('formDetailCompany');
    }
    // this.getJobTitleTree();
    this.getDutyLevelList();
  },
  mounted() {
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },

    // submit() {
    //   console.log('submit');
    //   const checkNode = this.$refs.personSelectTree.getCheckedNodes();
    //   const personList = checkNode.filter(x => x.type == 4);
    //   this.$emit('select', personList);
    //   this.handleClose();
    // },
    submit() {
      // console.log('submit',this.duty);
      if(!this.duty)return this.$message.error('未选择岗位')
      let find = this.radioList.find(item=>item.id == this.duty)
      let dutyObj = {id:find.id,name:find.name}
      this.$emit('select', dutyObj);
      this.handleClose();
    },
    //获取职级列表
    getDutyLevelList(){
      this.$axios.post(
        Api.user.dutyLevel,
        // 'http://192.168.1.171:9044/userCenterApi/dutyLevel/list',
        {
          data: { enableType: 'enable'}
        },
        res => {
          if (res.isSuccess) {
            this.radioList = res?.data || [];
            this.$nextTick(()=>{
              this.duty = this.auditCondition
            })
            // console.log('this.radioList',this.radioList)
            // this.defaultFirstLevelId = [res.data[0].id];
          } else {
            // this.$message.error(res.message);
          }
        }
      );
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-dialog__body {
  max-height: 600px;
}
.radio-div{
  width: 100%;
  margin: 7px 0;
  padding: 0 5px;
}
</style>
