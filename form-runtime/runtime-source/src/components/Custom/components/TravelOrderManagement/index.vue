<!--
 * @Descripttion: 差旅订单管理组件
 * @Author: [author]
 * @Date: 2026-06-04
-->
<template>
  <div class="travel-order-management" :class="{ 'no-border': !hasAnyData }">
    <!-- 外层 Tabs：机票、火车、酒店 -->
    <el-tabs v-model="outerActiveTab" type="border-card" v-if="hasAnyData">
      <!-- 机票订单 -->
      <el-tab-pane
        v-if="hasFlightData"
        name="flight"
        :label="`机票订单${getFlightBadge()}`">
        <el-tabs v-model="flightInnerTab" type="card">
          <!-- 出票订单 -->
          <el-tab-pane
            v-if="flightOrder.issued && flightOrder.issued.length > 0"
            :label="`出票/改签订单${getFlightIssuedBadge()}`"
            name="flight-issued">
            <div class="table-container">
              <el-table
                :data="flightOrder.issued"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="120" />
                <el-table-column prop="travelPeople" label="出行人" min-width="80" />
                <el-table-column prop="travelNum" label="航班号" min-width="80" />
                <el-table-column prop="travelName" label="行程" min-width="140" />
                <el-table-column label="起飞/到达时间" min-width="150">
                  <template slot-scope="scope">{{ formatTime(scope.row) }}</template>
                </el-table-column>
                <el-table-column prop="totalPrice" label="实付合计" min-width="80" align="right" />
                <el-table-column label="下单时间" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.bookingTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系人手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>

          <!-- 退票订单 -->
          <el-tab-pane
            v-if="flightOrder.refunded && flightOrder.refunded.length > 0"
            :label="`退票订单${getFlightRefundedBadge()}`"
            name="flight-refunded">
            <div class="table-container">
              <el-table
                :data="flightOrder.refunded"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="120" />
                <el-table-column prop="travelPeople" label="出行人" min-width="85" />
                <el-table-column prop="travelNum" label="航班号" min-width="85" />
                <el-table-column prop="travelName" label="行程" min-width="120" />
                <el-table-column label="起飞/到达时间" min-width="150">
                  <template slot-scope="scope">{{ formatTime(scope.row) }}</template>
                </el-table-column>
                <el-table-column prop="actualBackMoney" label="实退合计" min-width="80" align="right" />
                <el-table-column label="退票日期" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.confirmTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系人手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>

      <!-- 火车票订单 -->
      <el-tab-pane
        v-if="hasTrainData"
        name="train"
        :label="`火车票订单${getTrainBadge()}`">
        <el-tabs v-model="trainInnerTab" type="card">
          <!-- 出票订单 -->
          <el-tab-pane
            v-if="trainOrder.issued && trainOrder.issued.length > 0"
            :label="`出票/改签订单${getTrainIssuedBadge()}`"
            name="train-issued">
            <div class="table-container">
              <el-table
                :data="trainOrder.issued"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="95" />
                <el-table-column prop="travelPeople" label="乘车人" min-width="85" />
                <el-table-column prop="travelNum" label="车次" min-width="85" />
                <el-table-column prop="travelName" label="行程" min-width="120" />
                <el-table-column label="出发/到达时间" min-width="145">
                  <template slot-scope="scope">{{ formatTime(scope.row) }}</template>
                </el-table-column>
                <el-table-column prop="totalPrice" label="实付合计" min-width="80" align="right" />
                <el-table-column label="下单时间" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.bookingTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>

          <!-- 退票订单 -->
          <el-tab-pane
            v-if="trainOrder.refunded && trainOrder.refunded.length > 0"
            :label="`退票订单${getTrainRefundedBadge()}`"
            name="train-refunded">
            <div class="table-container">
              <el-table
                :data="trainOrder.refunded"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="95" />
                <el-table-column prop="travelPeople" label="退票人" min-width="85" />
                <el-table-column prop="travelNum" label="车次" min-width="85" />
                <el-table-column prop="travelName" label="行程" min-width="120" />
                <el-table-column label="出发/到达时间" min-width="145">
                  <template slot-scope="scope">{{ formatTime(scope.row) }}</template>
                </el-table-column>
                <el-table-column prop="actualBackMoney" label="实退合计" min-width="80" align="right" />
                <el-table-column label="退票时间" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.confirmTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>

      <!-- 酒店订单 -->
      <el-tab-pane
        v-if="hasHotelData"
        name="hotel"
        :label="`酒店订单${getHotelBadge()}`">
        <el-tabs v-model="hotelInnerTab" type="card">
          <!-- 酒店预订单 -->
          <el-tab-pane
            v-if="hotelOrder.booked && hotelOrder.booked.length > 0"
            :label="`酒店预订单${getHotelBookedBadge()}`"
            name="hotel-booked">
            <div class="table-container">
              <el-table
                :data="hotelOrder.booked"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="95" />
                <el-table-column prop="travelPeople" label="入住人" min-width="85" />
                <el-table-column prop="travelNum" label="酒店名称" min-width="125" />
                <el-table-column prop="travelName" label="城市" min-width="85" />
                <el-table-column label="入住/离店时间" min-width="100">
                  <template slot-scope="scope">{{ formatTime(scope.row, false) }}</template>
                </el-table-column>
                <el-table-column prop="totalPrice" label="订单实付" min-width="80" align="right" />
                <el-table-column label="下订时间" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.bookingTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>

          <!-- 酒店退订单 -->
          <el-tab-pane
            v-if="hotelOrder.cancelled && hotelOrder.cancelled.length > 0"
            :label="`酒店退订单${getHotelCancelledBadge()}`"
            name="hotel-cancelled">
            <div class="table-container">
              <el-table
                :data="hotelOrder.cancelled"
                border
                style="width: 1242px"
                :header-cell-style="headerCellStyle"
                :cell-style="cellStyle">
                <el-table-column type="index" label="序号" width="50" align="center" />
                <el-table-column prop="orderNo" label="订单号" min-width="95" />
                <el-table-column prop="travelPeople" label="退订入住人" min-width="85" />
                <el-table-column prop="travelNum" label="酒店名称" min-width="125" />
                <el-table-column prop="travelName" label="城市" min-width="85" />
                <el-table-column label="入住/离店时间" min-width="100">
                  <template slot-scope="scope">{{ formatTime(scope.row, false) }}</template>
                </el-table-column>
                <el-table-column prop="actualBackMoney" label="订单实退" min-width="80" align="right" />
                <el-table-column label="退订时间" min-width="120">
                  <template slot-scope="scope">{{ formatDateTime(scope.row.confirmTime) }}</template>
                </el-table-column>
                <el-table-column prop="booker" label="预定人" min-width="80" />
                <el-table-column prop="bookerPhone" label="联系手机" min-width="95" />
                <el-table-column prop="companyName" label="企业名称" min-width="140" />
                <el-table-column prop="ticketStatusName" label="订单类型" min-width="90" />
              </el-table>
            </div>
          </el-tab-pane>
        </el-tabs>
      </el-tab-pane>
    </el-tabs>

    <!-- 无数据时显示 -->
    <!-- <div v-else class="no-data">
      <span>暂无差旅订单数据</span>
    </div> -->
  </div>
</template>

<script>
export default {
  name: 'TravelOrderManagement',
  props: {
    // FormMaking 适配
    value: {
      type: [Object, String],
      default: () => ({})
    },
    options: {
      type: Object,
      default: () => ({})
    },
    events: {
      type: Object,
      default: () => ({})
    },
    disabled: {
      type: Boolean,
      default: false
    },
    // 是否使用 mock 数据（测试环境无数据时启用）
    useMock: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      flightInnerTab: 'flight-issued',
      trainInnerTab: 'train-issued',
      hotelInnerTab: 'hotel-booked',
      orderData: null
    };
  },
  created() {
    if (this.value || this.useMock) {
      this.fetchOrderData();
    }
  },
  watch: {
    value(newVal) {
      if (newVal || this.useMock) {
        this.fetchOrderData();
      }
    }
  },
  computed: {
    // 动态计算外层活动 Tab
    outerActiveTab: {
      get() {
        if (this.hasFlightData) return 'flight';
        if (this.hasTrainData) return 'train';
        if (this.hasHotelData) return 'hotel';
        return 'flight';
      },
      set(val) {
        // 只在有数据时才切换
      }
    },
    // 返回请求的数据，优先使用接口返回的 orderData
    travelData() {
      // 优先使用接口返回的数据
      if (this.orderData) {
        return {
          flightOrder: {
            issued: this.orderData.flightOrder?.issued || [],
            refunded: this.orderData.flightOrder?.refunded || []
          },
          trainOrder: {
            issued: this.orderData.trainOrder?.issued || [],
            refunded: this.orderData.trainOrder?.refunded || []
          },
          hotelOrder: {
            booked: this.orderData.hotelOrder?.booked || [],
            cancelled: this.orderData.hotelOrder?.cancelled || []
          }
        };
      }
      // fallback 到 props value
      let valueData = {};
      if (this.value && typeof this.value === 'object') {
        valueData = this.value;
      }
      return {
        flightOrder: {
          issued: valueData.flightOrder?.issued || [],
          refunded: valueData.flightOrder?.refunded || []
        },
        trainOrder: {
          issued: valueData.trainOrder?.issued || [],
          refunded: valueData.trainOrder?.refunded || []
        },
        hotelOrder: {
          booked: valueData.hotelOrder?.booked || [],
          cancelled: valueData.hotelOrder?.cancelled || []
        }
      };
    },

    flightOrder() {
      return this.travelData.flightOrder;
    },

    trainOrder() {
      return this.travelData.trainOrder;
    },

    hotelOrder() {
      return this.travelData.hotelOrder;
    },

    // 判断是否有任何数据
    hasAnyData() {
      return this.hasFlightData || this.hasTrainData || this.hasHotelData;
    },

    // 机票数据判断
    hasFlightData() {
      return (this.flightOrder.issued?.length > 0) || (this.flightOrder.refunded?.length > 0);
    },

    // 火车数据判断
    hasTrainData() {
      return (this.trainOrder.issued?.length > 0) || (this.trainOrder.refunded?.length > 0);
    },

    // 酒店数据判断
    hasHotelData() {
      return (this.hotelOrder.booked?.length > 0) || (this.hotelOrder.cancelled?.length > 0);
    },

    // 表头样式
    headerCellStyle() {
      return {
        background: 'transparent',
        color: '#606266',
        fontSize: '14px',
        textAlign: 'center',
        fontWeight: 400,
        padding: '8px 0'
      };
    },

    // 单元格样式
    cellStyle() {
      return {
        textAlign: 'center',
        verticalAlign: 'middle',
        padding: '8px 0'
      };
    }
  },
  methods: {
    // 格式化时间，合并出发和到达时间
    formatTime(row, showTime = true) {
      const { departTime, arriveTime } = row;
      if (!departTime && !arriveTime) return '-';
      const fmt = (time) => {
        if (!time) return '-';
        const d = new Date(time);
        const year = d.getFullYear();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        if (!showTime) return `${year}-${month}-${day}`;
        const hour = String(d.getHours()).padStart(2, '0');
        const min = String(d.getMinutes()).padStart(2, '0');
        return `${year}-${month}-${day} ${hour}:${min}`;
      };
      return `${fmt(departTime)} 至 ${fmt(arriveTime)}`;
    },

    // 格式化日期时间
    formatDateTime(time) {
      if (!time) return '-';
      const d = new Date(time);
      const year = d.getFullYear();
      const month = String(d.getMonth() + 1).padStart(2, '0');
      const day = String(d.getDate()).padStart(2, '0');
      const hour = String(d.getHours()).padStart(2, '0');
      const min = String(d.getMinutes()).padStart(2, '0');
      return `${year}-${month}-${day} ${hour}:${min}`;
    },

    // 格式化金额
    formatMoney(row, field) {
      const value = row[field];
      if (!value && value !== 0) return '-';
      return Number(value).toFixed(2);
    },

    // 获取机票角标
    getFlightBadge() {
      const issued = this.flightOrder.issued?.length || 0;
      const refunded = this.flightOrder.refunded?.length || 0;
      const total = issued + refunded;
      return total > 0 ? `(${total})` : '';
    },

    // 获取火车角标
    getTrainBadge() {
      const issued = this.trainOrder.issued?.length || 0;
      const refunded = this.trainOrder.refunded?.length || 0;
      const total = issued + refunded;
      return total > 0 ? `(${total})` : '';
    },

    // 获取酒店角标
    getHotelBadge() {
      const booked = this.hotelOrder.booked?.length || 0;
      const cancelled = this.hotelOrder.cancelled?.length || 0;
      const total = booked + cancelled;
      return total > 0 ? `(${total})` : '';
    },

    // 获取机票出票角标
    getFlightIssuedBadge() {
      const count = this.flightOrder.issued?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取机票退票角标
    getFlightRefundedBadge() {
      const count = this.flightOrder.refunded?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取火车出票角标
    getTrainIssuedBadge() {
      const count = this.trainOrder.issued?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取火车退票角标
    getTrainRefundedBadge() {
      const count = this.trainOrder.refunded?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取酒店预订角标
    getHotelBookedBadge() {
      const count = this.hotelOrder.booked?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取酒店退订角标
    getHotelCancelledBadge() {
      const count = this.hotelOrder.cancelled?.length || 0;
      return count > 0 ? `(${count})` : '';
    },

    // 获取数据（供外部调用）
    getData() {
      return this.travelData;
    },

    // 获取 sid
    getSid() {
      for (const key in window.localStorage) {
        if (key.endsWith('invest-power-system-token')) {
          return window.localStorage.getItem(key);
        }
      }
      return '';
    },

    // 根据 productTypeId 和 ticketStatusName 结构化订单数据
    structurizeOrderData(rawData) {
      const result = {
        flightOrder: { issued: [], refunded: [] },
        trainOrder: { issued: [], refunded: [] },
        hotelOrder: { booked: [], cancelled: [] }
      };

      if (!Array.isArray(rawData)) return result;

      rawData.forEach(item => {
        const { productTypeId, ticketStatusName } = item;
        // productTypeId: 1-酒店, 2-机票, 3-火车
        if (productTypeId === 1) {
          if (ticketStatusName === '确认订单') {
            result.hotelOrder.booked.push(item);
          } else if (ticketStatusName === '退订') {
            result.hotelOrder.cancelled.push(item);
          }
        } else if (productTypeId === 2) {
          if (ticketStatusName === '出票' || ticketStatusName === '改签') {
            result.flightOrder.issued.push(item);
          } else if (ticketStatusName === '退票') {
            result.flightOrder.refunded.push(item);
          }
        } else if (productTypeId === 3) {
          if (ticketStatusName === '出票' || ticketStatusName === '改签') {
            result.trainOrder.issued.push(item);
          } else if (ticketStatusName === '退票') {
            result.trainOrder.refunded.push(item);
          }
        }
      });

      return result;
    },

    // 调用接口获取订单数据
    async fetchOrderData() {
      console.log(this.value, 'flowInstanceId');

      // 使用 mock 数据（测试环境无数据时）
      if (this.useMock) {
        try {
          const mockModule = await import('./mockData');
          const res = mockModule.default;
          console.log('使用 mock 数据:', res);
          if (res.isSuccess || res.code === 'RESP200') {
            this.orderData = this.structurizeOrderData(res.data);
            console.log('结构化订单数据:', this.orderData);
          }
        } catch (e) {
          console.error('加载 mock 数据失败:', e);
        }
        return;
      }

      if (!this.value) {
        console.warn('未获取到 flowInstanceId');
        return;
      }
      const sid = this.getSid();
      if (!sid) {
        console.warn('未获取到 sid');
        return;
      }
      try {
        const res = await this.$axios.post(
          `/web/hesi/mall/bill/findDetailByFlowInstanceId`,
          {
            sid: sid,
            data: {
              flowInstanceId: this.value
            }
          }
        );
        console.log('接口返回原始数据:', res);
        if (res.isSuccess || res.code === 'RESP200') {
          this.orderData = this.structurizeOrderData(res.data);
          console.log('结构化订单数据:', this.orderData);
        } else {
          console.error('接口返回失败:', res.message);
        }
      } catch (error) {
        console.error('接口调用异常:', error);
      }
    }
  }
};
</script>

<style lang="scss" scoped>
.travel-order-management {
  width: 100%;
  border: 1px solid #999;
  overflow: hidden;

  &.no-border {
    border: none;
  }

  .table-container {
    padding: 0 1px;
  }

  .no-data {
    text-align: center;
    padding: 30px 0;
    color: #909399;
    font-size: 14px;
    background: #fff;
  }
}

::v-deep {
  // 外层 border-card tabs 样式
  .el-tabs--border-card {
    border: none;
    background: #f5f7fa;
  }

  .el-tabs--border-card > .el-tabs__header {
    background: #fff;
    border-bottom: 1px solid #e4e7ed;
    margin: 0;
  }

  .el-tabs--border-card > .el-tabs__header .el-tabs__item {
    height: 44px;
    line-height: 44px;
    padding: 0 16px;
    font-size: 14px;
    color: #606266;
    border: none;
    border-radius: 6px 6px 0 0;
    margin-right: 0;
    margin-top: 4px;

    &:hover {
      color: #409eff;
    }

    &.is-active {
      color: #409eff;
      background: #f5f7fa;
      font-weight: 500;
    }
  }

  .el-tabs--border-card > .el-tabs__header .el-tabs__nav {
    border: none;
  }

  .el-tabs--border-card > .el-tabs__content {
    padding: 12px;
    background: #f5f7fa;
  }

  .el-tabs__content {
    overflow: visible;
  }

  // 内层 card tabs 样式
  .el-tabs--card {
    background: #fff;
    border-radius: 6px;
    overflow: hidden;
  }

  .el-tabs--card > .el-tabs__header {
    margin: 0;
    background: #fff;
    border-bottom: 1px solid #e4e7ed;
  }

  .el-tabs--card > .el-tabs__header .el-tabs__nav {
    border: none;
  }

  .el-tabs--card > .el-tabs__header .el-tabs__item {
    height: 40px;
    line-height: 40px;
    padding: 0 12px;
    font-size: 13px;
    color: #606266;
    border: none;
    border-bottom: 1px solid #e4e7ed;
    border-top-left-radius: 6px;
    border-top-right-radius: 6px;
    background: #fff;
    margin-right: 0;
    margin-top: 0px;

    &:hover {
      color: #409eff;
    }

    &.is-active {
      color: #fff;
      background: #409eff;
      border-bottom-color: #409eff;
      font-weight: 500;
    }
  }

  .el-tabs--card > .el-tabs__content {
    padding: 12px 6px;
  }

  // 表格样式
  .el-table {
    border: 1px solid #e4e7ed;
    overflow: hidden;

    &::before,
    &::after {
      display: none;
    }

    .el-table__header-wrapper {
      border-right: none;

      th {
        background: #f5f7fa !important;
        color: #606266;
        font-weight: 500;
        font-size: 13px;
      }
    }

    .el-table__body-wrapper {
      border-right: none;

      td {
        color: #606266;
        font-size: 13px;
      }
    }

    .el-table__header th,
    .el-table__body td {
      border-right: 1px solid #e4e7ed;
      border-bottom: 1px solid #e4e7ed;
      padding: 12px 4px;
    }

    .cell {
      padding-left: 5px !important;
      padding-right: 5px !important;
    }

    .el-table__body {
      border-right: none;
    }

    // 斑马纹
    .el-table__row:nth-child(even) {
      background: #fafafa;
    }

    // 悬停效果
    .el-table__row:hover > td {
      background: #ecf5ff !important;
    }
  }
}
</style>