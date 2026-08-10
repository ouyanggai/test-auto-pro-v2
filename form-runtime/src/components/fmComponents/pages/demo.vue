<!--  -->
<template>
  <div class='fmCostomPage23333'>
    aa  fmCostomPage23333
    <el-input v-model="inputVal"></el-input>
    <el-button @click="change">emit</el-button>
  </div>
</template>

<script>
export default {
  name: '',
  components: {},
  props: ['value', 'propData'],
  inject: ['flowDialog'],
  data () {
    return {
      inputVal: '',
      generateForm: null,
      tableData: []
    };
  },
  watch: {
    value: {
      handler(newName, oldName) {
        console.log(this.value, 'aa-watch-this.value');
      },
      deep: true,
      immediate: true
    },
    tableData: {
      handler(newName, oldName) {
        console.log(this.tableData, 'aa-watch-this.tableData');
      },
      deep: true
    }
  },
  computed: {},
  created() {
    // var vdd = this.getComponent('vdd');
    // console.log(vdd.$children[0], '在表单自定义js中获取当前组件this');
    console.log(this.value, 'this.value');
    console.log(this.propData, 'this.propData');
    // var rowIndex = this.propData.t.$parent.rowIndex // 当前行下标
    // var group = this.propData.t.$parent.group // 子表单字段名
    this.inputVal = this.propData.d;
  },
  mounted() {
    console.log(this, 'aa-this');
    setTimeout(() => {
      // this.tableData = [];
      this.tableData.push({ er: 34 });
    }, 1500);
    console.log(this.flowDialog, 'aa-this.flowDialog');
    // console.log(this.flowDialog.flowName || this.flowDialog.selectFlowName, 'aa-this.flowDialog');
    this.generateForm = this.flowDialog.$refs.generateForm || this.flowDialog.$refs.OtherSteps2.$refs.generateForm;
    this.generateForm.$on('someMethod', (val) => {
      console.log(val, 'someMethod');
      console.log(this.generateForm.getValue('userName'), 'getValues()');
    });
    console.log(this.generateForm, 'aa-this.generateForm');
  },
  methods: {
    change() {
      this.$emit('input', { inputVal: this.inputVal });
    }
  }
};

</script>
<style lang='scss' scoped>
</style>
