<!--
 * @Descripttion: 城市选择组件 - 支持单选/多选模式
 * @Author: zhengzetao
 * @Date: 2026-05-16
-->
<template>
  <div class="city-select-wrapper">
    <!-- 输入框区域 -->
    <div
      class="city-input-area"
      :class="{ 'is-disabled': disabled, 'is-error': showError }"
      @click="openPicker"
    >
      <div class="tags-container">
        <!-- 单选模式 tag（有×） -->
        <el-tag
          v-if="!isMultiple && selectedCityData.length === 1 && selectedCityData[0]"
          size="small"
          :closable="!disabled"
          @close.stop="removeCity(0)"
          class="city-tag"
        >
          {{ getCityDisplay(selectedCityData[0]) }}
        </el-tag>
        <!-- 多选模式 tag（有×） -->
        <template v-if="isMultiple">
          <el-tag
            v-for="(city, index) in selectedCityData"
            :key="city.code || index"
            size="small"
            :closable="!disabled"
            @close.stop="removeCity(index)"
            class="city-tag"
          >
            {{ getCityDisplay(city) }}
          </el-tag>
        </template>
        <span v-if="selectedCityData.length === 0" class="placeholder-text">{{ placeholder }}</span>
      </div>
      <i class="el-icon-arrow-down arrow-icon" :class="{ 'is-reverse': isPickerOpen }"></i>
    </div>

    <el-dialog
      :visible.sync="isPickerOpen"
      :title="title"
      width="680px"
      :close-on-click-modal="false"
      append-to-body
      class="city-picker-dialog"
      @closed="onDialogClosed"
    >
      <div class="city-picker-container">
        <!-- 多选模式顶部已选列表 -->
        <div v-if="isMultiple && pendingCityData.length > 0" class="selected-cities-bar">
          <span class="selected-label">已选：</span>
          <div class="selected-tags">
            <el-tag
              v-for="(city, index) in pendingCityData"
              :key="city.code || index"
              size="small"
              closable
              @close.stop="removeCityInDialog(index)"
              class="selected-tag"
            >
              {{ city.name }}
            </el-tag>
          </div>
        </div>

        <!-- 搜索框 -->
        <div class="top-search">
          <el-input
            v-model="searchKeyword"
            :placeholder="isMultiple ? '搜索并选择城市' : '搜索城市'"
            clearable
            @input="handleSearch"
          >
            <i slot="prefix" class="el-icon-search"></i>
          </el-input>
        </div>

        <!-- 快捷选择 & 搜索结果 -->
        <div class="right-content">
          <!-- 快捷选择面板 -->
          <div v-if="!searchKeyword.trim()" class="quick-select-panel">
            <!-- 左右结构：左侧一级 Tab，右侧内容 -->
            <div class="quick-select-body">
              <div class="quick-level-sidebar">
                <span class="quick-level-tab" :class="{ active: quickTab === 'domestic' }" @click="switchQuickTab('domestic')">国内</span>
                <span class="quick-level-tab" :class="{ active: quickTab === 'overseas' }" @click="switchQuickTab('overseas')">国际/中国港澳台</span>
              </div>
              <div class="quick-level-content">
                <!-- 二级 Tab -->
                <div class="quick-sub-tabs" ref="subTabsRef">
                  <span
                    v-for="tab in currentSubTabs"
                    :key="tab.key"
                    ref="subTabRefs"
                    class="quick-sub-tab"
                    :class="{ active: activeSubTab === tab.key }"
                    @click="handleSubTabClick(tab.key)"
                  >{{ tab.name }}</span>
                  <span class="quick-sub-indicator" :style="indicatorStyle"></span>
                </div>
                <!-- 城市列表 -->
                <el-scrollbar wrap-class="city-scroll-wrap" style="height: 280px;">
                  <div class="quick-city-list">
                    <!-- 热门/海外：平铺展示 -->
                    <template v-if="activeSubTab === 'hot' || quickTab === 'overseas'">
                      <span
                        v-for="city in currentFlatCities"
                        :key="city.name"
                        v-show="!isCityTransportDisabled(city)"
                        class="quick-city-item"
                        @click="handleQuickCitySelect(city)"
                      >{{ city.name }}</span>
                    </template>
                    <!-- 国内字母分组展示 -->
                    <template v-else>
                      <div v-for="(cities, letter) in currentGroupedCities" :key="letter" v-show="visibleCitiesInGroup(cities) > 0" class="quick-letter-group">
                        <div class="quick-letter-header">{{ letter }}</div>
                        <div class="quick-letter-cities">
                          <span
                            v-for="city in cities"
                            :key="city.name"
                            v-show="!isCityTransportDisabled(city)"
                            class="quick-city-item"
                            @click="handleQuickCitySelect(city)"
                          >{{ city.name }}</span>
                        </div>
                      </div>
                    </template>
                  </div>
                </el-scrollbar>
              </div>
            </div>
          </div>
          <!-- 搜索结果 -->
          <div v-else class="search-result">
            <el-scrollbar wrap-class="city-scroll-wrap" style="height: 320px;" @scroll="handleSearchScroll" ref="searchScrollbar">
              <div v-if="searchResults.length > 0" class="search-list">
                <div
                  v-for="(city, index) in searchResults"
                  :key="city.key ? city.key + index : city.code + index"
                  class="search-item"
                  :class="{ 'is-selected': isCitySelected(city) }"
                  @click="selectCity(city)"
                >
                  <span class="city-name" v-html="highlightKeyword(city.name)"></span>
                  <span class="city-fullname" v-if="city.fullName">{{ city.fullName }}</span>
                  <i v-if="isCitySelected(city)" class="el-icon-check selected-check"></i>
                </div>
                <div v-if="searchLoading" class="loading-more">
                  <i class="el-icon-loading"></i> 加载中...
                </div>
                <div v-if="!searchHasMore && searchResults.length > 0" class="no-more-data">
                  已加载全部
                </div>
              </div>
              <div v-else-if="searchKeyword.trim() && !searchLoading && noTransportCity" class="no-result">当前交通工具下暂无可用城市</div>
              <div v-else-if="searchKeyword.trim() && !searchLoading" class="no-result">未找到相关城市</div>
            </el-scrollbar>
          </div>
        </div>

        <!-- 多选模式底部按钮 -->
        <div v-if="isMultiple" class="dialog-footer">
          <el-button size="small" @click="handleCancel" :disabled="disabled">取消</el-button>
          <el-button type="primary" size="small" @click="handleConfirm" :disabled="disabled">确认</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script>
/* eslint-disable */
import { domesticTabs, overseasTabs, domesticCities, overseasCities } from './baseData';
import { parseJsonArray, parseJsonObject } from '@/utils/parse-value';
export default {
  name: 'CitySelect',
  components: {},
  props: {
    value: { type: [String, Object, Array], default: '' },
    disabled: { type: Boolean, default: false },
    placeholder: { type: String, default: '请选择城市' },
    title: { type: String, default: '选择城市' },
    onChange: { type: Function, default: null },
    // 交通工具类型：flight-飞机, train-火车, 空字符串-不过滤
    transportType: { type: String, default: '' },
    // 是否必填（用于校验）
    required: { type: Boolean, default: false }
  },
  data() {
    return {
      isPickerOpen: false,
      displayText: [],
      selectedCityData: [],
      // 确认前的临时选中（多选模式）
      pendingCityData: [],
      searchKeyword: '',
      searchResults: [],
      isSearching: false,
      searchTimer: null,
      searchCurrent: 1,
      searchSize: 20,
      searchTotal: 0,
      searchLoading: false,
      searchHasMore: true,
      // 交通工具筛选后无数据
      noTransportCity: false,
      // 快捷选择状态
      quickTab: 'domestic',
      activeSubTab: 'hot',
      indicatorStyle: {},
      searchRequestId: 0
    };
  },
  computed: {
    isMultiple() {
      console.log(this.placeholder, 'this.placeholder')
      return this.placeholder && this.placeholder.includes('多选');
    },
    showError() {
      return this.required && this.selectedCityData.length === 0;
    },
    // 当前一级 Tab 对应的二级 Tab 列表
    currentSubTabs() {
      return this.quickTab === 'domestic' ? domesticTabs : overseasTabs;
    },
    // 平铺展示的城市列表（热门/海外）
    currentFlatCities() {
      if (this.quickTab === 'overseas') {
        return overseasCities[this.activeSubTab] || [];
      }
      return domesticCities.hot || [];
    },
    // 字母分组的城市（国内非热门 Tab）
    currentGroupedCities() {
      if (this.quickTab === 'domestic' && this.activeSubTab !== 'hot') {
        return domesticCities[this.activeSubTab] || {};
      }
      return {};
    }
  },
  watch: {
    value: {
      immediate: true,
      handler(val) {
        if (!val) {
          this.selectedCityData = [];
          return;
        }

        try {
          if (this.isMultiple) {
            // 多选模式
            let arr = parseJsonArray(val, null);
            if (!arr) {
              arr = String(val).split(',').filter(Boolean).map(item => parseJsonObject(item, { name: item, code: '' }));
            }
            this.selectedCityData = arr;
          } else {
            // 单选模式
            const obj = parseJsonObject(val, null);
            this.selectedCityData = obj && Object.keys(obj).length > 0 ? [obj] : [];
          }
        } catch (e) {
          this.selectedCityData = [];
        }
      }
    },
  },
  created() {
    // 快捷选择使用 baseData 静态数据，无需额外初始化
  },
  methods: {
    openPicker() {
      if (this.disabled) return;
      this.isPickerOpen = true;

      this.$nextTick(() => {
        this.updateIndicator();
      });

      if (this.isMultiple) {
        // 多选模式：打开时同步 pending
        this.pendingCityData = [...this.selectedCityData];
        if (this.searchKeyword.trim()) {
          this.searchRequestId++;
          this.resetSearchPagination();
          this.doSearch();
        }
      } else {
        // 单选模式：已有值则回填搜索
        if (this.selectedCityData.length === 1) {
          this.searchKeyword = this.selectedCityData[0].name || '';
          if (this.searchKeyword.trim()) {
            this.searchRequestId++;
            this.resetSearchPagination();
            this.doSearch();
          }
        } else {
          this.resetSearchState();
        }
      }
    },
    onDialogClosed() {
      if (this.isMultiple) {
        // 多选取消时恢复
        this.pendingCityData = [];
      }
      this.resetSearchState();
    },
    handleCancel() {
      this.isPickerOpen = false;
    },
    handleConfirm() {
      // 多选确认：将 pending 合并到 selected
      const cityString = JSON.stringify(this.pendingCityData);
      this.selectedCityData = [...this.pendingCityData];
      this.$emit('input', cityString);
      this.$emit('change', this.selectedCityData);
      if (this.onChange && typeof this.onChange === 'function') {
        this.onChange(this.selectedCityData);
      }
      this.isPickerOpen = false;
      this.pendingCityData = [];
      this.searchKeyword = '  '; // 用来触发formMaking的原始校验，必须通过其中的input中的值变化来触发校验（无语，搞死人了）
    },
    removeCity(index) {
      this.selectedCityData.splice(index, 1);
      // 单选模式空值 emit {}（FormMaking 表格期望对象），多选模式 emit "[]"
      const emitValue = this.isMultiple ? JSON.stringify(this.selectedCityData) : JSON.stringify({});
      this.$emit('input', emitValue);
      this.$emit('change', this.selectedCityData);
      if (this.onChange && typeof this.onChange === 'function') {
        this.onChange(this.selectedCityData);
      }
      this.searchKeyword = '  '; // 用来触发formMaking的原始校验，必须通过其中的input中的值变化来触发校验（无语，搞死人了）
    },
    removeCityInDialog(index) {
      // 弹窗内移除（仅影响 pending）
      this.pendingCityData.splice(index, 1);
    },
    selectCity(city) {
      if (this.isMultiple) {
        // 多选模式
        const existIndex = this.pendingCityData.findIndex(
          item => item.code === city.code || item.key === city.key
        );
        if (existIndex === -1) {
          // 不存在才添加，已存在则忽略
          this.pendingCityData.push(city);
        }
        // 清空搜索框，继续搜索
        this.searchKeyword = '';
        this.searchResults = [];
        this.resetSearchPagination();
      } else {
        // 单选模式
        this.selectedCityData = [city];
        const cityString = JSON.stringify(city);
        this.$emit('input', cityString);
        this.$emit('change', city);
        if (this.onChange && typeof this.onChange === 'function') {
          this.onChange(city);
        }
        this.isPickerOpen = false;
      }
    },
    isCitySelected(city) {
      const list = this.isMultiple ? this.pendingCityData : this.selectedCityData;
      return list.some(item => item.code === city.code || item.key === city.key);
    },
    resetSearchState() {
      this.searchKeyword = '';
      this.searchResults = [];
      this.isSearching = false;
      this.resetSearchPagination();
    },
    handleSearch() {
      if (this.searchTimer) {
        clearTimeout(this.searchTimer);
      }
      if (!this.searchKeyword.trim()) {
        this.isSearching = false;
        this.searchResults = [];
        this.resetSearchPagination();
        return;
      }
      this.searchTimer = setTimeout(() => {
        this.searchRequestId++;
        this.resetSearchPagination();
        this.doSearch();
      }, 300);
    },
    resetSearchPagination() {
      this.searchCurrent = 1;
      this.searchTotal = 0;
      this.searchHasMore = true;
      this.searchResults = [];
    },
    handleSearchScroll({ target }) {
      const { scrollTop, scrollHeight, clientHeight } = target;
      const distanceToBottom = scrollHeight - scrollTop - clientHeight;
      if (distanceToBottom < 50 && !this.searchLoading && this.searchHasMore) {
        this.loadMoreSearchResults();
      }
    },
    loadMoreSearchResults() {
      if (this.searchLoading || !this.searchHasMore) return;
      this.searchCurrent++;
      this.doSearch(true);
    },
    async doSearch(isLoadMore = false) {
      this.isSearching = true;
      this.searchLoading = true;

      try {
        // 捕获当前请求 ID，用于后续校验响应是否过时
        const requestId = this.searchRequestId;

        let sid = '';
        for (const key in window.localStorage) {
          if (key.endsWith('invest-power-system-token')) {
            sid = window.localStorage.getItem(key);
            break;
          }
        }

        const params = {
          sid: sid,
          pagination: true,
          current: this.searchCurrent,
          size: this.searchSize,
          data: {
            name: this.searchKeyword,
            cityType: ''
          }
        };
        const res = await this.$axios.post(
          '/web/hesi/city/local/list',
          params
        );
        // 如果在此期间发起了新的搜索，丢弃过时响应
        if (requestId !== this.searchRequestId) return;
        console.log('res', res);
        if (res.isSuccess || res.code === 'RESP200') {
          const data = res.data;
          let records = data.items || [];
          const total = data.count || 0;
          const originalCount = records.length;

          // 根据交通工具类型筛选
          if (this.transportType === 'flight') {
            records = records.filter(item => item.haveFlight === true);
          } else if (this.transportType === 'train') {
            records = records.filter(item => item.haveTrain === true);
          }

          // 设置交通工具筛选后无数据的标志
          this.noTransportCity = this.transportType && records.length === 0;
          console.log(this.noTransportCity, 'this.noTransportCity');
          const normalizeCity = (item) => {
            let country = item.country || '国内';
            // 如果 country 为 "国外" 但 fullName 中有具体国家名，从 fullName 提取
            if (country === '国外' && item.fullName && item.fullName.includes(',')) {
              const firstPart = item.fullName.split(',')[0].trim();
              if (firstPart) {
                country = firstPart;
              }
            }
            return {
              name: item.name || '',
              code: item.code || item.key || '',
              key: item.key || '',
              pinyin: item.nameSpell ? item.nameSpell.toLowerCase() : '',
              fullName: item.fullName || item.name || '',
              country: country,
              parentId: item.parentId || ''
            };
          };

          if (isLoadMore) {
            this.searchResults = [...this.searchResults, ...records.map(normalizeCity)];
          } else {
            this.searchResults = records.map(normalizeCity);
          }

          this.searchTotal = total;
          this.searchHasMore = this.searchResults.length < total;
        } else {
          console.error('搜索城市失败:', res.message);
          if (!isLoadMore) {
            this.searchResults = [];
          }
        }
      } catch (error) {
        console.error('搜索城市接口异常:', error);
        if (!isLoadMore) {
          this.searchResults = [];
        }
      } finally {
        this.searchLoading = false;
      }
    },
    highlightKeyword(text) {
      if (!text || !this.searchKeyword) return text || '';
      const keyword = this.searchKeyword.toLowerCase();
      const lowerText = text.toLowerCase();
      const index = lowerText.indexOf(keyword);
      if (index === -1) return text;
      const before = text.substring(0, index);
      const match = text.substring(index, index + keyword.length);
      const after = text.substring(index + keyword.length);
      return `${before}<span class="highlight">${match}</span>${after}`;
    },
    getCityDisplay(city) {
      if (!city) return '';
      if (city.country && city.country !== '中国' && city.country !== '国内') {
        return city.name + '(' + city.country + ')';
      }
      return city.name;
    },
    isCityTransportDisabled(city) {
      if (this.transportType === 'flight') return !city.haveFlight;
      if (this.transportType === 'train') return !city.haveTrain;
      return false;
    },
    // 获取字母分组中可见城市数量
    visibleCitiesInGroup(cities) {
      return cities.filter(c => !this.isCityTransportDisabled(c)).length;
    },
    // 切换内部 Tab（带动画）
    handleSubTabClick(key) {
      this.activeSubTab = key;
      this.$nextTick(() => {
        this.updateIndicator();
      });
    },
    // 更新滑动指示器位置
    updateIndicator() {
      if (!this.$refs.subTabsRef) return;
      const tabs = this.$refs.subTabsRef.querySelectorAll('.quick-sub-tab');
      let activeEl = null;
      tabs.forEach(el => {
        if (el.classList.contains('active')) {
          activeEl = el;
        }
      });
      if (activeEl) {
        this.indicatorStyle = {
          left: activeEl.offsetLeft + 'px',
          width: activeEl.offsetWidth + 'px'
        };
      }
    },
    // 切换一级 Tab
    switchQuickTab(tab) {
      this.quickTab = tab;
      this.activeSubTab = 'hot';
      this.$nextTick(() => {
        this.updateIndicator();
      });
    },
    // 点击快捷城市 → 填充搜索框并触发搜索
    handleQuickCitySelect(city) {
      if (this.isCityTransportDisabled(city)) return;
      // 取消可能触发的 handleSearch 定时器，避免干扰
      if (this.searchTimer) {
        clearTimeout(this.searchTimer);
        this.searchTimer = null;
      }
      this.searchKeyword = city.name;
      this.searchRequestId++;
      this.resetSearchPagination();
      // 先以 baseData 中的城市数据作为兜底，确保搜索结果有展示
      this.searchResults = [{ ...city }];
      this.searchHasMore = false;
      this.doSearch();
    },
  }
};
</script>
<style lang="scss" scoped>
.city-select-wrapper {
  width: 100%;
}
.city-input-area {
  display: flex;
  align-items: center;
  min-height: 32px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 0 10px;
  cursor: pointer;
  background: #fff;
  &:hover {
    border-color: #409eff;
  }
  &.is-disabled {
    background: #f5f7fa;
    cursor: not-allowed;
    &:hover {
      border-color: #dcdfe6;
    }
  }
}
.tags-container {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  min-height: 24px;
}
.city-tag {
  margin: 0;
}
.placeholder-text {
  color: #999;
  font-size: 14px;
}
.arrow-icon {
  color: #999;
  transition: transform 0.3s;
  &.is-reverse {
    transform: rotate(180deg);
  }
}
.city-picker-dialog {
  .city-picker-container {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .selected-cities-bar {
    display: flex;
    align-items: flex-start;
    padding: 8px 0;
    border-bottom: 1px solid #f0f0f0;
    margin-bottom: 10px;
    flex-shrink: 0;
    .selected-label {
      font-size: 14px;
      color: #666;
      flex-shrink: 0;
      line-height: 28px;
    }
    .selected-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      flex: 1;
    }
    .selected-tag {
      margin: 0;
    }
  }
  .top-search {
    padding-bottom: 10px;
    flex-shrink: 0;
    .el-input__prefix {
      left: 10px !important;
      top: 6px !important;
    }
  }
  .right-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .search-result {
    flex: 1;
    overflow: hidden;
  }
  .search-tip {
    text-align: center;
    color: #999;
    padding: 20px;
    font-size: 14px;
  }
  .no-result {
    text-align: center;
    color: #999;
    padding: 20px;
  }
  .loading-more {
    text-align: center;
    color: #999;
    padding: 15px;
    font-size: 13px;
  }
  .no-more-data {
    text-align: center;
    color: #ccc;
    padding: 15px;
    font-size: 13px;
  }
  .search-list {
    display: flex;
    flex-direction: column;
  }
  .search-item {
    display: flex;
    flex-direction: column;
    justify-content: center;
    height: 48px;
    padding: 0 10px;
    font-size: 14px;
    color: #333;
    cursor: pointer;
    border-bottom: 1px solid #f0f0f0;
    position: relative;
    &:hover {
      background: #f5f5f5;
    }
    &.is-selected {
      background: #ecf5ff;
      color: #409eff;
    }
    .city-name {
      line-height: 1.4;
      ::v-deep .highlight {
        color: #409eff;
        font-weight: 500;
      }
    }
    .city-fullname {
      font-size: 12px;
      color: #999;
      line-height: 1.2;
      margin-top: 2px;
    }
    .selected-check {
      position: absolute;
      right: 10px;
      top: 50%;
      transform: translateY(-50%);
      color: #409eff;
      font-weight: bold;
    }
  }
  .city-scroll-wrap {
    overflow-y: auto;
  }
  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding-top: 15px;
    border-top: 1px solid #f0f0f0;
    margin-top: 10px;
    flex-shrink: 0;
  }
  .quick-select-panel {
    height: 320px;
  }
  .quick-select-body {
    display: flex;
    height: 100%;
  }
  .quick-level-sidebar {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    width: 84px;
    padding: 8px 0;
    background: #f0f7ff;
    .quick-level-tab {
      padding: 10px;
      margin: 0 4px;
      text-align: center;
      font-size: 14px;
      color: #666;
      cursor: pointer;
      user-select: none;
      border-radius: 4px;
      &.active {
        color: #fff;
        font-weight: 500;
        background: #409eff;
      }
      &.active:hover {
        color: #fff;
        background: #409eff;
      }
      &:hover {
        color: #409eff;
      }
    }
  }
  .quick-level-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 0 12px;
  }
  .quick-sub-tabs {
    display: flex;
    flex-wrap: wrap;
    padding: 8px 0 0 0;
    border-bottom: 1px solid #f0f0f0;
    flex-shrink: 0;
    position: relative;
    .quick-sub-tab {
      padding: 8px 10px;
      margin-right: 12px;
      margin-bottom: -1px;
      font-size: 13px;
      color: #666;
      cursor: pointer;
      user-select: none;
      position: relative;
      &.active {
        color: #409eff;
      }
      &:hover {
        color: #409eff;
      }
      &::before {
        content: '';
        position: absolute;
        left: 2px;
        right: 2px;
        top: 3px;
        bottom: 3px;
        border-radius: 6px;
        background: transparent;
        pointer-events: none;
      }
      &:hover::before {
        background: rgba(64, 158, 255, 0.1);
      }
    }
    .quick-sub-indicator {
      position: absolute;
      bottom: -1px;
      height: 3px;
      background: #409eff;
      border-radius: 3px 3px 0 0;
      transition: left 0.3s ease, width 0.3s ease;
    }
  }
  .quick-city-list {
    padding: 10px 5px;
    display: flex;
    flex-wrap: wrap;
    align-content: flex-start;
  }
  .quick-city-item {
    display: inline-block;
    padding: 4px 10px;
    margin: 4px 11px 4px 0;
    font-size: 13px;
    color: #333;
    border: 1px solid #e4e7ed;
    border-radius: 4px;
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
    transition: all 0.2s;
    &:hover {
      color: #409eff;
      border-color: #409eff;
      background: #ecf5ff;
    }
    &.is-disabled {
      color: #c0c4cc;
      border-color: #e4e7ed;
      background: #f5f7fa;
      cursor: not-allowed;
      &:hover {
        color: #c0c4cc;
        border-color: #e4e7ed;
        background: #f5f7fa;
      }
    }
  }
  .quick-letter-group {
    width: 100%;
    margin-bottom: 6px;
    display: flex;
    align-items: baseline;
    .quick-letter-header {
      font-size: 15px;
      font-weight: 500;
      color: #409eff;
      padding: 4px 14px 4px 0;
      flex-shrink: 0;
    }
    .quick-letter-cities {
      display: flex;
      flex-wrap: wrap;
      flex: 1;
    }
  }
}
::v-deep .el-input__prefix {
  left: 10px !important;
  top: 6px !important;
}
::v-deep .el-input__validateIcon {
  display: none !important;
}
</style>
