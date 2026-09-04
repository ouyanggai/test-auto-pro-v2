<!--
 * @Descripttion: 出差行程组件
 * @Author: zhengzetao
 * @Date: 2026-06-02
-->
<template>
  <div class="travel-route-planning">
    <!-- 标题行 -->
    <div class="header-row">
      <el-button type="primary" size="small" icon="el-icon-plus" @click="addRoute" :disabled="disabled">
        添加行程
      </el-button>
      <span class="titleName">出差行程</span>
    </div>

    <!-- 表格 -->
    <el-table
        :data="routeList"
        border
        style="width: 100%"
        :header-cell-style="{ background: 'transparent', color: '#606266', fontSize: '14px', textAlign: 'center', fontWeight: 400 }"
        :cell-style="{ textAlign: 'center', verticalAlign: 'middle' }"
      >
        <!-- 序号 -->
        <el-table-column type="index" label="序号" width="50" align="center" />
        <!-- 交通工具 -->
        <el-table-column prop="transportationTypesObj" label="交通工具" min-width="110">
          <template slot-scope="scope">
            <el-select
              v-model="scope.row.transportationTypesObj"
              placeholder="请选择"
              :disabled="disabled"
              @change="handleTransportationChange(scope.$index)"
            >
              <el-option
                v-for="type in transportTypes"
                :key="type"
                :label="type"
                :value="type"
              ></el-option>
            </el-select>
          </template>
        </el-table-column>

        <!-- 出发地 -->
        <el-table-column prop="departureCityObj" label="出发地" min-width="120">
          <template slot-scope="scope">
            <CitySelect
              v-if="isCityPickerType(scope.row.transportationTypesObj)"
              v-model="scope.row.departureCityObj"
              placeholder="请选择出发地"
              :disabled="disabled || !scope.row.transportationTypesObj"
              :transport-type="transportTypeMap[scope.row.transportationTypesObj] || ''"
              @change="handleFieldChange(scope.$index, 'departureCityObj')"
            />
            <el-input
              class="text-left"
              v-else
              v-model="scope.row.departureCityObj"
              placeholder="请输入出发地"
              :disabled="disabled || !scope.row.transportationTypesObj"
              @change="handleFieldChange(scope.$index, 'departureCityObj')"
            />
          </template>
        </el-table-column>

        <!-- 目的地 -->
        <el-table-column prop="destinationCityObj" label="目的地" min-width="120">
          <template slot-scope="scope">
            <CitySelect
              v-if="isCityPickerType(scope.row.transportationTypesObj)"
              v-model="scope.row.destinationCityObj"
              placeholder="请选择目的地"
              :disabled="disabled || !scope.row.transportationTypesObj"
              :transport-type="transportTypeMap[scope.row.transportationTypesObj] || ''"
              @change="handleFieldChange(scope.$index, 'destinationCityObj')"
            />
            <el-input
              class="text-left"
              v-else
              v-model="scope.row.destinationCityObj"
              placeholder="请输入目的地"
              :disabled="disabled || !scope.row.transportationTypesObj"
              @change="handleFieldChange(scope.$index, 'destinationCityObj')"
            />
          </template>
        </el-table-column>

        <!-- 行程类型 -->
        <el-table-column prop="isBackTracking" label="行程类型" min-width="70">
          <template slot-scope="scope">
            <el-select
              v-model="scope.row.isBackTracking"
              placeholder="请选择"
              :disabled="disabled"
              @change="handleFieldChange(scope.$index, 'isBackTracking')"
            >
              <el-option label="往返" :value="true"></el-option>
              <el-option label="单程" :value="false"></el-option>
            </el-select>
          </template>
        </el-table-column>

        <!-- 是否住宿 -->
        <el-table-column prop="isAccommodation" label="是否住宿" min-width="70">
          <template slot-scope="scope">
            <el-select
              v-model="scope.row.isAccommodation"
              placeholder="请选择"
              :disabled="disabled"
              @change="handleAccommodationChange(scope.$index)"
            >
              <el-option label="是" value="是"></el-option>
              <el-option label="否" value="否"></el-option>
            </el-select>
          </template>
        </el-table-column>

        <!-- 住宿城市 -->
        <el-table-column prop="accommodationLocationObj" label="住宿城市" min-width="180">
          <template slot-scope="scope">
            <CitySelect
              v-if="scope.row.isAccommodation === '是'"
              v-model="scope.row.accommodationLocationObj"
              placeholder="请选择住宿城市(多选)"
              :disabled="disabled || scope.row.isAccommodation !== '是'"
              :required="true"
              @change="handleFieldChange(scope.$index, 'accommodationLocationObj')"
            />
            <span v-else class="disabled-text">-</span>
          </template>
        </el-table-column>

        <!-- 操作 -->
        <el-table-column label="操作" width="50">
          <template slot-scope="scope">
            <i
              class="el-icon-delete-solid delete-btn"
              @click="deleteRoute(scope.$index)"
              :class="{ 'disabled': disabled || routeList.length <= 1 }"
            ></i>
          </template>
        </el-table-column>
      </el-table>
  </div>
</template>

<script>
import CitySelect from '@/components/Custom/components/CitySelect/index.vue';
import { parseJsonArray } from '@/utils/parse-value';

const CITY_PICKER_TYPES = ['飞机', '火车'];

const VALID_TRANSPORT_TYPES = ['飞机', '火车', '汽车', '轮船', '公务车', '自备车'];

/**
 * 校验行程列表
 * @param {Array} tripList 行程数组
 * @returns {{ isValid: boolean, errorMsg: string }}
 */
function validateTripList(tripList) {
  if (!Array.isArray(tripList) || tripList.length === 0) {
    return { isValid: false, errorMsg: '请填写至少一条完整行程规划' };
  }

  for (let i = 0; i < tripList.length; i++) {
    const item = tripList[i];

    // 1. 交通工具
    if (!VALID_TRANSPORT_TYPES.includes(item.transportationTypesObj)) {
      return { isValid: false, errorMsg: '请填写完整行程规划' };
    }

    // 2. 出发地不能为空
    if (
      !item.departureCityObj ||
      (typeof item.departureCityObj === 'string' &&
        (item.departureCityObj.trim() === '' || item.departureCityObj.trim() === '{}'))
    ) {
      return { isValid: false, errorMsg: '请填写完整行程规划' };
    }

    // 3. 目的地不能为空
    if (
      !item.destinationCityObj ||
      (typeof item.destinationCityObj === 'string' &&
        (item.destinationCityObj.trim() === '' || item.destinationCityObj.trim() === '{}'))
    ) {
      return { isValid: false, errorMsg: '请填写完整行程规划' };
    }

    // 4. 是否住宿
    if (item.isAccommodation !== '是' && item.isAccommodation !== '否') {
      return { isValid: false, errorMsg: '请填写完整行程规划' };
    }

    // 5. 住宿联动
    if (item.isAccommodation === '是') {
      if (
        !item.accommodationLocationObj ||
        (typeof item.accommodationLocationObj === 'string' &&
          (item.accommodationLocationObj.trim() === '' || item.accommodationLocationObj.trim() === '[]'))
      ) {
        return { isValid: false, errorMsg: '请填写完整行程规划' };
      }
    }

    // 6. 行程类型
    if (typeof item.isBackTracking !== 'boolean') {
      return { isValid: false, errorMsg: '请填写完整行程规划' };
    }
  }

  return { isValid: true, errorMsg: '' };
}


export default {
  name: 'TravelRoutePlanning',
  components: {
    CitySelect
  },
  props: {
    // FormMaking 适配
    value: {
      type: [Array, String],
      default: () => []
    },
    options: {
      type: Object,
      default: () => ({})
    },
    events: {
      type: Object,
      default: () => ({})
    },
    // 禁用状态
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      routeList: [],
      transportTypeMap: {
        '飞机': 'flight',
        '火车': 'train'
      }
    };
  },
  computed: {
    // 交通工具选项列表
    transportTypes() {
      return VALID_TRANSPORT_TYPES;
    }
  },
  watch: {
    value: {
      immediate: true,
      handler(val) {
        if (!val) {
          this.routeList = [];
          this.routeList.push(this.createEmptyRoute());
          return;
        }
        let parsedVal = parseJsonArray(val);

        if (Array.isArray(parsedVal) && parsedVal.length > 0) {
          this.routeList = [...parsedVal];
        } else {
          this.routeList = [];
          this.routeList.push(this.createEmptyRoute());
        }
      }
    }
  },
  methods: {
    // 判断交通工具是否使用城市选择器
    isCityPickerType(transportType) {
      return CITY_PICKER_TYPES.includes(transportType);
    },

    // 创建空行程对象
    createEmptyRoute() {
      return {
        transportationTypesObj: '',
        departureCityObj: '',
        destinationCityObj: '',
        isAccommodation: '',
        accommodationLocationObj: '',
        isBackTracking: null
      };
    },

    // 添加行程
    addRoute() {
      this.routeList.push(this.createEmptyRoute());
      this.emitChange('routes');
    },

    // 删除行程
    deleteRoute(index) {
      if (this.disabled || this.routeList.length <= 1) {
        return;
      }
      this.routeList.splice(index, 1);
      this.emitChange('routes');
    },

    // 交通工具变化，清空出发地和目的地
    handleTransportationChange(index) {
      const route = this.routeList[index];
      route.departureCityObj = '';
      route.destinationCityObj = '';
      this.emitChange('transportationTypesObj');
    },

    // 是否住宿切换
    handleAccommodationChange(index) {
      const route = this.routeList[index];
      if (route.isAccommodation !== '是') {
        route.accommodationLocationObj = '';
      }
      this.emitChange('isAccommodation');
    },

    // 字段变化
    handleFieldChange(index, field) {
      this.emitChange(field);
    },

    // 触发变更
    emitChange(fieldName) {
      const data = this.routeList;
      this.$emit('input', JSON.stringify(data));
    },
    // 校验行程数据（FormMaking 可通过 ref 调用）
    validate() {
      return validateTripList(this.routeList);
    },

    // 获取数据
    getData() {
      return this.routeList;
    }
  }
};
</script>

<style lang="scss" scoped>
.travel-route-planning {
  width: 100%;
  ::v-deep {
    .text-left {
      .el-input__inner {
        text-align: left !important;
        padding-left: 5px !important;
      }
    }
  }
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px;
  border-right: 1px solid #999999;
  border-left: 1px solid #999999;

  .titleName {
    font-size: 15px;
    color: #333;
    position: absolute;
    font-weight: bold;
    left: 50%;
    transform: translateX(-50%);
  }
}

::v-deep {
  .el-table {
    border: 1px solid #999999;
    border-bottom: none;

    &::before,
    &::after {
      display: none;
    }

    .el-table__header-wrapper,
    .el-table__body-wrapper {
      border-right: none;
      overflow: visible;
    }

    .el-table__header th,
    .el-table__body td {
      border-right: 1px solid #999999;
      border-bottom: 1px solid #999999;
      vertical-align: middle;
      overflow: visible;
    }

    .el-checkbox-group {
      display: flex;
      flex-wrap: nowrap;
      gap: 8px;
      justify-content: center;
    }

    .el-radio-group {
      display: flex;
      flex-wrap: nowrap;
      gap: 4px;
      justify-content: center;
    }

    .el-form-item {
      width: 100%;
      margin-bottom: 0;
    }

    .el-form-item__error {
      position: relative;
      font-size: 12px;
      display: block;
      margin-top: 4px;
      line-height: 1.2;
      background: transparent;
      white-space: normal;
    }

    .cell {
      overflow: visible;
      height: auto;
    }

    .el-table__row .cell {
      padding: 0 5px;
    }
  }
}

.delete-btn {
  font-size: 18px;
  color: #f56c6c;
  cursor: pointer;

  &:hover {
    color: #f56c6c;
    opacity: 0.8;
  }

  &.disabled {
    color: #c0c4cc;
    cursor: not-allowed;
    opacity: 1;
  }
}

.disabled-text {
  color: #c0c4cc;
}

.footer-note {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}
</style>

<style lang="scss">
.itinerary{
  padding: 0px !important;
  .el-input__inner {
    border-color: #c0c4cc !important;
  }
}
</style>
