<!-- 自定义获取设备台账 -->
<template>
  <div class="custom-container">
    <table class="fm-report-table__table">
      <tr>
        <td>设备台账</td>
        <td><el-input placeholder='获取设备台账' v-model="deviceName" @focus="show" :disabled="disabled" readonly></el-input>
        </td>
        <td>设备编号</td>
        <td><el-input placeholder='设备台账编号' v-model="deviceNo" @focus="show" :disabled="disabled" readonly></el-input></td>
      </tr>
    </table>
    <StationDevice :visible.sync="visible" ref="windStation" @confirmChecked="confirmChecked"></StationDevice>
  </div>
</template>
<script>
/* eslint-disable */
import StationDevice from './StationDevice'
import { parseJsonObject } from '@/utils/parse-value'
export default {
  name: 'CustomeDevice',
  components: {
    StationDevice
  },
  props: {
    value: {
      type: Object | String,
      default: () => {
        return {}
      }
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      visible: false,
      dataModel: this.value,
      deviceName: '',
      deviceNo: ''
    };
  },
  created() { },
  mounted() { },
  watch: {
    value: {
      handler(val) {
        if (val) {
          const dataModel = parseJsonObject(val)
          this.dataModel = dataModel
          this.deviceName = this.dataModel['deviceName']
          this.deviceNo = this.dataModel['deviceNo']
        }
      },
      immediate: true
    },
    dataModel(val) {
      this.$emit('input', val)
    }
  },
  computed: {},
  methods: {
    show() {
      this.visible = true
    },
    confirmChecked(data) {
      if (data.length) {
        this.deviceName = data.map(item => item.name).join(',')
        this.deviceNo = data.map(item => item.deviceCode).join(',')
        var dataModel = {
          deviceName: this.deviceName,
          deviceNo: this.deviceNo
        }
        this.dataModel = JSON.stringify(dataModel)
      }
      this.visible = false
    }
  },
};
</script>
<style lang="scss" scoped>
.custom-container {
  width: 100%;
}

table {
  width: 100%;
}

td {
  text-align: center;
  padding: 0 2px;
}</style>
