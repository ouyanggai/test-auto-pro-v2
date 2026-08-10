<template>
  <div class="ec-input-range" :style="{'width':width+'px'}">
    <div class="inner">
      <el-input v-model="val.min"  @input="handleVal('min')"
        :placeholder="placeholderMin"  clearable>
        <i slot="prefix" :class="`el-input__icon ${icon}`" v-if="icon"></i>
      </el-input>
      <!-- <div class="ec-input-range-divider"></div> -->
      <div style="font-size: 12px;">{{ dvidFlag }}</div>
      <el-input v-model="val.max"  @input="handleVal('max')"
        :placeholder="placeholderMax" clearable></el-input>
      </div>
  </div>
</template>
<script>
export default {
  name: "RangeInput",
  data() {
    return {
      val: {
        min: '',
        max: '',
      },
    };
  },
  props: {
    placeholderMin: {
      type: String,
      default: "请输入",
    },
    placeholderMax: {
      type: String,
      default: "请输入",
    },
    dvidFlag:{
      type:String,
      default:'-'
    },
    icon:{
      type:String,
      default:''
    },
    width:{
      type:String,
      default:'240'
    }
  },
  model: {
    prop: "val",
    event: "input",
  },
  methods: {
    clear(){
      this.val.min = ''
      this.val.max = ''
    },
    handleVal(key) {
      const _this = this;
      this.val[key]=this.val[key].replace(/[^\d]/g,' ')
      if (_this.val.min == "" || _this.val.min == undefined || _this.val.min == null ||
        _this.val.max == "" || _this.val.max == undefined || _this.val.max == null) {
        _this.$emit("input", _this.val);
      } else {
        // if (Number(_this.val.min) >= Number(_this.val.max)) {
        //   _this.$message({
        //     type: 'warning',
        //     message: '区间最小值需要小于最大值!',
        //     onClose: function () {
        //       _this.val.min = '';
        //       _this.val.max = '';
        //     }
        //   });
        //   return
        // }
        _this.$emit("input", _this.val);
      }
    },
  },
};
</script>
<style lang="scss" scoped>
.ec-input-range{
  display: inline-block;
  border:1px solid #DCDFE6;
  border-radius: 4px;
  height: 28px;
  .inner{
    display: flex;
    align-items: center;
  }
}
.ec-input-range-divider {
  display: inline-block;
  width: 7px;
  height: 1px;
  background: #c0c4cc;
  margin-bottom: 4px;
}
::v-deep{
  .el-input--mini .el-input__inner{
    border: none;
    height: 26px;
  }
}
</style>
