<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-12-18
-->
<!-- 集团公司或部门多选与单选弹窗：单选或多选是通过表单字段包含的字段进行识别，mulSelect代表多选、singleSelect代表单选 -->
<!-- 用单选举例，字段可以是newSingleSelect、singleSelectComeOn、singleSelect2等包含singleSelect，不区分大小写 -->
<!-- printRead: 打印前的阅读模式 -->

<template>
  <div class="custom-container">
    <MyCompanyList :visible.sync="indicatorHeaderVisible" v-model="currentInfoObj" @selectHeader="selectHeader" :fieldSelectType="fieldSelectType">
      <template scope="scope">
        <div v-if="printRead" class="print-read-label">
          <span v-for="(item,index) in dataShowList" :key="index">{{ item.name }}<i v-if="(index+1) != dataShowList.length">、</i></span>
        </div>
        <template v-else>
          <div v-if="dataShowList.length">
            <div class="personSelect-wrap">
              <div class="name-wrap">
                <span v-for="(item,index) in dataShowList" :key="index">{{ item.name }}<i v-if="(index+1) != dataShowList.length">、</i></span>
              </div>
              <div class="person-icon-wrap" v-show="!disabled">
                <i class="el-icon-office-building person-icon1" @click="openInfoDialog"></i>
                <!-- <i class="el-icon-user person-icon2" @click="openInfoDialog"></i> -->
              </div>
            </div>
          </div>
          <el-input v-else readonly v-model="scope.viewName" :disabled="disabled" @focus="openInfoDialog" style="width:100%"></el-input> <!-- v-model动态回显名称 -->
          <!-- <el-button v-else type="primary" :disabled="disabled" @click="openInfoDialog" style="margin-left:5px;">{{ placeholder }}</el-button> -->
        </template>
      </template>
    </MyCompanyList>
  </div>
</template>
<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import MyCompanyList from './MyCompanyList.vue';

export default {
  name: 'CustomeInfoSelect',
  components: {
    MyCompanyList
  },
  props: {
    value: {
      type: String,
      default: ''
    },
    placeholder: {
      type: String,
      default: ''
    },
    disabled: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return false;
      }
    },
    printRead: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return true;
        // return false;
      }
    }
  },
  data() {
    return {
      currentInfoObj: this.value,
      indicatorHeaderVisible: false,
      fieldSelectType: this.$parent.modelName,
      dataShowList:[]
    };
  },
  created() {
   },
  mounted() {
    if (this.currentInfoObj) {
      this.dataShowList = JSON.parse(this.currentInfoObj);
    }
   },
  watch: {
    value(val) {
      console.log('val-公司和部门多选-传值',val)
      this.currentInfoObj = val;
      this.dataShowList = JSON.parse(val);
    },
    placeholder(val) {
      // console.log('======placeholder======',val)
    },
    currentInfoObj(val){
      console.log('emit-公司和部门多选-控件赋值',val)
      this.$emit('input', val)
    },
    disabled(val){
      console.log('====disabled===',this.disabled)
    },
    printRead(val){
      console.log('====printRead22===',this.printRead)
    },
  },
  computed: {},
  methods: {
    openInfoDialog(data){
      this.indicatorHeaderVisible = true;
        
    },
    selectHeader(data) { // 选择赋值
      console.log('selectHeader-data',data)
      this.currentInfoObj = data;
      this.dataShowList = JSON.parse(data);
      console.log('this.dataShowList',this.dataShowList)
      this.indicatorHeaderVisible = false;

      // 添加表单校验
      this.$nextTick(() => { 
        this.$parent.$parent.validate()
      })
    },
  },
};
</script>
<style lang="scss" scoped>
.custom-container {
  width: 100%;
}

.personSelect-wrap{
  display:flex;
  padding: 4px;
  .name-wrap {
    flex:1;
    line-height: 1.2;
  }
  .person-icon-wrap {
    font-size: 19px;
    width: 20px;
    position: relative;
    cursor: pointer;
    color: #409EFF;
    .person-icon1{
      position: absolute;
      top: 14%;
      // top: 36%;
      right: 0px;
    }
    .person-icon2 {
      position: absolute;
      top: 36%;
      right: 3px;
    }
  }
}

table {
  width: 100%;
}

td {
  text-align: center;
  padding: 0 2px;
}

</style>