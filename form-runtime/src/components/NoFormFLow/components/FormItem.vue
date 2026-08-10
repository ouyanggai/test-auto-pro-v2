<template>
  <div>
    <el-row v-for="(colVal ,index) in config" :key="index" :class="noBorder?'noBorder':''" class="child-row">
        <el-col v-for="val in colVal" :span="val.span" :style = "{'height':val.height||''}" :class="val.type == 'button' ? 'flex-center':''">
          <div class="label" v-if="val.type == 'label'" :class="[noBorder?'noBorder':'',val.align == 'left' ? 'alignLeft':'']" :style="{'border-right':val.borderRight ? '1px solid #999':'none'}">{{ val.title }}</div>
          <div v-if="val.type == 'button'" >
            <el-button :type="val.buttonType || 'primary'" @click="()=>{clickEvent(val)}" :disabled="val.disabled">{{ val.title }}</el-button>
          </div>
          <div v-if="val.children && val.children.length">
            <FormItem :config="val.children" :noBorder="true" :form="form" :pickerOptions="pickerOptions"></FormItem>
          </div>
          <div v-else>
            <el-form-item :prop="val.prop" :class="val.borderRight?'borderRight':''" :rules="val.rules || null">
              <div v-if="val.type == 'input'" class="input-container">
                  <el-input :placeholder="val.placeholder|| '请输入'"  v-model="form[val.prop]" :disabled="val.disabled"
                            @input="(value)=>inputEvent(value,val)"
                            :maxlength="val.maxlength || '200'"
                            ></el-input>
              </div>
              <div v-if="val.type == 'inputNum'" class="input-container">
                  <el-input type="number" :placeholder="val.placeholder|| '请输入'"  v-model="form[val.prop]" :disabled="val.disabled"
                            @input="(value)=>inputEvent(value,val)"
                            :maxlength="val.maxlength || '200'"
                            :controls="false"
                            ></el-input>
              </div>

              <div v-if="val.type == 'select'" class="input-container">
                <el-select v-model="form[val.prop]" :placeholder="val.placeholder || '请选择'"
                          :filterable="val.allowCreate" :allow-create = "val.allowCreate || false"
                          @change="(value)=>{changeEvent(value,val)}"
                          :disabled="val.disabled">
                  <el-option
                    v-for="(item,k) in val.options"
                    :key="k"
                    :label="item.label"
                    :value="item.value">
                  </el-option>
                </el-select>
              </div>
              <div v-if="val.type == 'date'" class="input-container" :class="noBorder?'noBorder':''">
                <el-date-picker :placeholder="val.placeholder|| '选择日期'"
                :format="val.formate || 'yyyy-MM-dd HH:mm:ss'" :value-format="val.formate || 'yyyy-MM-dd HH:mm:ss'"
                                  v-model="form[val.prop]" :type="val.dateType" :disabled="val.disabled"
                                  :picker-options = "val.pickerOptions"
                                  >
                                </el-date-picker>
              </div>
              <div v-if="val.type == 'checkbox'" class="checkbox-div">
                <el-checkbox-group v-model="form[val.prop]" :disabled="val.disabled">
                  <el-checkbox v-for="item in val.options" :label="item.label" :key="item.label"></el-checkbox>
                </el-checkbox-group>
              </div>
              <div v-if="val.type == 'radio'" class="input-container" >
                <el-radio-group v-model="form[val.prop]" :disabled="val.disabled">
                  <el-radio v-for="item in val.options" :label="item.value" :key="item.label">{{ item.label }}</el-radio>
                </el-radio-group>
              </div>
            </el-form-item>
          </div>
        </el-col>
      </el-row>
  </div>
</template>

<script>

export default {
  name:'FormItem',
  components: {},
  props:{
    form:{
      type:Object,
      default(){
        return{}
      }
    },
    config:{
      type:Array,
      default:()=>{
        return []
      }
    },
    noBorder:{
      type:Boolean,
      default:false
    },
    borderTop:{
      type:String,
      default:''
    },
    pickerOptions:{
      type:Object,
      default(){
        return{}
      }
    }
  },
  data() {
    return {
      childrenConfig:[]
    };
  },
  mounted() {},
  computed: {},
  methods: {
    inputEvent(value,item){
      this.$emit('inputEvent',value,item)
    },
    changeEvent(value,item){
      this.$emit('changeEvent',value,item)
    },
    clickEvent(item){
      this.$emit('clickEvent',item)
    }
  },
};
</script>
<style lang="scss" scoped>
.container{
  width: 100%;
  .form-div{
    border: 1px double #999999;
    margin: 0 auto;
  }
  .form-div.noBorder{
    border:none;
  }
}
.el-row{
  display: flex;
  border-bottom: 1px solid #999999;
  .input-container{
    padding: 0 8px;
    border-right: 1px solid #999;
    line-height: 36px;
  }
  .input-container.noBorder{
    border-right: none;
  }
  .el-col:last-child .input-container{
    border: none;
  }
}
.el-row.noBorder{
  border-bottom: none;
}
.el-row:last-child{
  border: none;
}
.label{
  width: 100%;
  height: 100%;
  padding: 0 8px;
  box-sizing: border-box;
  border-right: 1px  solid #999999;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}
.label.noBorder{
  border-right: none;
}
.checkbox-div{
  padding: 0 10px;
  display: flex;
  align-items: center;
}
::v-deep .el-form-item--mini.el-form-item{
  margin: 0;
}
::v-deep .el-date-editor.el-input, .el-date-editor.el-input__inner{
  width: 100% !important;
}
.borderRight{
  border-right:1px solid #999;
}
.alignLeft{
  justify-content: start;
}
.el-radio{
  margin: 2px 4px;
}
.flex-center{
  display: flex;
  align-items: center;
}
</style>
