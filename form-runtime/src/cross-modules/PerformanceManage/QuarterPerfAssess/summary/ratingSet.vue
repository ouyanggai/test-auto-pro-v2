<template>
  <div class="performance-setting-page">
    <!-- 组织绩效评级设置 -->
    <div class="personal-performance-item" style="margin-bottom: 20px;">
      <h3 class="company-performance-title">组织绩效评级设置</h3>
      <el-table :data="company_kpi" border style="width: 100%; margin-bottom: 20px;">
        <el-table-column prop="level" label="等级" align="center"></el-table-column>
        <el-table-column label="分数范围（0-5分）" align="center" min-width="200">
          <template slot-scope="scope">
            <el-select v-model="scope.row.minCompareSymbol" style="width:100px; margin-right: 5px;" >
              <el-option label="大于" value="cs_greater_than"></el-option>
              <el-option label="大于等于" value="cs_greater_than_or_equal_to"></el-option>
            </el-select>
            <el-input-number :min="0" :max="5" v-model="scope.row.minScore" :controls="false" style="width:60px"/>
            <span>{{' - '}}</span>
            <el-select v-model="scope.row.maxCompareSymbol" style="width:100px; margin-right: 5px;" >
              <el-option label="小于" value="cs_less_than"></el-option>
              <el-option label="小于等于" value="cs_less_than_or_equal_to"></el-option>
            </el-select>
            <el-input-number :min="0" :max="5" v-model="scope.row.maxScore" :controls="false" style="width:60px"/>
          </template>
        </el-table-column>
        <el-table-column prop="groupRatio" label="小组比例控制（0-100%）" align="center">
          <template slot-scope="scope">
            <el-input-number :min="0" :max="100" v-model="scope.row.minRankingScale" :controls="false" style="width:60px"/>
            <span>{{' - '}}</span>
            <el-input-number :min="0" :max="100" v-model="scope.row.maxRankingScale" :controls="false" style="width:60px"/>
          </template>
        </el-table-column>
        <el-table-column prop="coefficient" label="激励系数（0-5）" align="center">
          <template slot-scope="scope">
            <el-input-number :min="0" :max="5" v-model="scope.row.coefficient" :controls="false" style="width:60px"/>
          </template>
        </el-table-column>
        <el-table-column prop="remarks" label="备注" align="center">
          <template slot-scope="scope">
            <el-input v-model="scope.row.remarks" type="textarea" :autosize="{minRows:1,maxRows:3}"></el-input>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <!-- 个人绩效设置（A/B/C/D/E 分区） -->
    <div class="personal-performance-container">
      <div class="personal-performance-item" v-for="(item, index) in personal_kpi" :key="index">
        <h4 class="personal-performance-title">个人绩效{{item.base.level}}设置</h4>
        <div class="score-range">
          <span>分数范围：</span>
          <el-select v-model="item.base.minCompareSymbol" style="width:100px; margin-right: 5px;" >
            <el-option label="大于" value="cs_greater_than"></el-option>
            <el-option label="大于等于" value="cs_greater_than_or_equal_to"></el-option>
          </el-select>
          <el-input-number :min="0" :max="5" v-model="item.base.minScore" :controls="false" style="width:60px"/>
          <span>{{' - '}}</span>
          <el-select v-model="item.base.maxCompareSymbol" style="width:100px; margin-right: 5px;" >
            <el-option label="小于" value="cs_less_than"></el-option>
            <el-option label="小于等于" value="cs_less_than_or_equal_to"></el-option>
          </el-select>
          <el-input-number :min="0" :max="5" v-model="item.base.maxScore" :controls="false" style="width:60px"/>
          <span style="margin-left:15px;">激励系数：</span>
          <el-input-number :min="0" :max="5" v-model="item.base.coefficient" :controls="false" style="width:60px"/>
        </div>
        <el-table :data="item.list" border style="width: 100%; margin-top: 10px;">
          <el-table-column prop="level" label="组织绩效等级" align="center"></el-table-column>
          <el-table-column label="排名比例（0-100%）" align="center">
            <template slot-scope="scope">
              <div v-if="(item.base.level == 'A' && scope.row.level == 'E') || (item.base.level == 'E' && scope.row.level == 'A')">无</div>
              <div v-else>
                <el-input-number :min="0" :max="100" v-model="scope.row.minRankingScale" :controls="false" style="width:60px"/>
                <span>{{' - '}}</span>
                <el-input-number :min="0" :max="100" v-model="scope.row.maxRankingScale" :controls="false" style="width:60px"/>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="remarks" label="备注" align="center">
            <template slot-scope="scope">
              <el-input v-model="scope.row.remarks" type="textarea" :autosize="{minRows:1,maxRows:3}"></el-input>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
    <div style="margin-top:15px;margin-bottom:10px;text-align: center;">
        <el-button type="primary" @click="handleSave"> 保 存 </el-button>
    </div>
  </div>
</template>

<script>
import math from '@/utils/math.js';
export default {
  name: 'PerformanceSetting',
  data() {
    return {
      levelEnums: { 'level_a': 'A', 'level_b': 'B', 'level_c': 'C', 'level_d': 'D', 'level_e': 'E' },
      sortEnums: { level_a: 1, level_b: 2, level_c: 3, level_d: 4, level_e: 5 },
      company_kpi: [
        { level: 'A', ratingLevel: 'level_a', coefficient: 2, minScore: 4.2, maxScore: 5, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to', minRankingScale: 0, maxRankingScale: 10, remarks: '' },
        { level: 'B', ratingLevel: 'level_b', coefficient: 1.5, minScore: 3.75, maxScore: 4.2, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to', minRankingScale: 11, maxRankingScale: 30, remarks: '' },
        { level: 'C', ratingLevel: 'level_c', coefficient: 1, minScore: 3, maxScore: 3.75, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to', minRankingScale: 31, maxRankingScale: 80, remarks: '' },
        { level: 'D', ratingLevel: 'level_d', coefficient: 0.5, minScore: 2.4, maxScore: 3, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to', minRankingScale: 81, maxRankingScale: 95, remarks: '' },
        { level: 'E', ratingLevel: 'level_e', coefficient: 0, minScore: 0, maxScore: 2.4, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to', minRankingScale: 96, maxRankingScale: 100, remarks: '' }
      ],
      personal_kpi: [
        {
          base: { level: 'A', ratingLevel: 'level_a', coefficient: 2, minScore: 4.2, maxScore: 5, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to' },
          list: [
            { level: 'A', ratingLevel: 'level_a', minRankingScale: 0, maxRankingScale: 30, remarks: '' },
            { level: 'B', ratingLevel: 'level_b', minRankingScale: 31, maxRankingScale: 50, remarks: '' },
            { level: 'C', ratingLevel: 'level_c', minRankingScale: 51, maxRankingScale: 60, remarks: '' },
            { level: 'D', ratingLevel: 'level_d', minRankingScale: 61, maxRankingScale: 65, remarks: '' },
            { level: 'E', ratingLevel: 'level_e', minRankingScale: undefined, maxRankingScale: undefined, remarks: '' }
          ]
        },
        {
          base: { level: 'B', ratingLevel: 'level_b', coefficient: 1.5, minScore: 3.75, maxScore: 4.2, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to' },
          list: [
            { level: 'A', ratingLevel: 'level_a', minRankingScale: 31, maxRankingScale: 50, remarks: '' },
            { level: 'B', ratingLevel: 'level_b', minRankingScale: 21, maxRankingScale: 40, remarks: '' },
            { level: 'C', ratingLevel: 'level_c', minRankingScale: 11, maxRankingScale: 30, remarks: '' },
            { level: 'D', ratingLevel: 'level_d', minRankingScale: 6, maxRankingScale: 20, remarks: '' },
            { level: 'E', ratingLevel: 'level_e', minRankingScale: 1, maxRankingScale: 10, remarks: '' }
          ]
        },
        {
          base: { level: 'C', ratingLevel: 'level_c', coefficient: 1, minScore: 3, maxScore: 3.75, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to' },
          list: [
            { level: 'A', ratingLevel: 'level_a', minRankingScale: 51, maxRankingScale: 90, remarks: '' },
            { level: 'B', ratingLevel: 'level_b', minRankingScale: 41, maxRankingScale: 85, remarks: '' },
            { level: 'C', ratingLevel: 'level_c', minRankingScale: 31, maxRankingScale: 80, remarks: '' },
            { level: 'D', ratingLevel: 'level_d', minRankingScale: 21, maxRankingScale: 75, remarks: '' },
            { level: 'E', ratingLevel: 'level_e', minRankingScale: 11, maxRankingScale: 70, remarks: '' }
          ]
        },
        {
          base: { level: 'D', ratingLevel: 'level_d', coefficient: 0.5, minScore: 2.4, maxScore: 3, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to' },
          list: [
            { level: 'A', ratingLevel: 'level_a', minRankingScale: 0, maxRankingScale: 10, remarks: '' },
            { level: 'B', ratingLevel: 'level_b', minRankingScale: 5, maxRankingScale: 15, remarks: '' },
            { level: 'C', ratingLevel: 'level_c', minRankingScale: 5, maxRankingScale: 20, remarks: '' },
            { level: 'D', ratingLevel: 'level_d', minRankingScale: 10, maxRankingScale: 25, remarks: '' },
            { level: 'E', ratingLevel: 'level_e', minRankingScale: 10, maxRankingScale: 30, remarks: '' }
          ]
        },
        {
          base: { level: 'E', ratingLevel: 'level_e', coefficient: 0, minScore: 0, maxScore: 2.4, minCompareSymbol: 'cs_greater_than_or_equal_to', maxCompareSymbol: 'cs_less_than_or_equal_to' },
          list: [
            { level: 'A', ratingLevel: 'level_a', minRankingScale: undefined, maxRankingScale: undefined, remarks: '' },
            { level: 'B', ratingLevel: 'level_b', minRankingScale: 0, maxRankingScale: 5, remarks: '' },
            { level: 'C', ratingLevel: 'level_c', minRankingScale: 0, maxRankingScale: 5, remarks: '' },
            { level: 'D', ratingLevel: 'level_d', minRankingScale: 0, maxRankingScale: 10, remarks: '' },
            { level: 'E', ratingLevel: 'level_e', minRankingScale: 0, maxRankingScale: 10, remarks: '' }
          ]
        }
      ]
    };
  },
  created() {
    this.initData('company_kpi');
    this.initData('personal_kpi');
    window.abb = this;
  },
  methods: {
    initData(kpi2Type) {
      this.$axios.post('/web/plan/api/kpi2RatingSet/list', { data: { kpi2Type } },
        res => {
          if (res.isSuccess && res.data && res.data[0].ratingSetItems) {
            var rates = res.data[0].ratingSetItems;
            rates.sort((a, b) => this.sortEnums[a.ratingLevel] - this.sortEnums[b.ratingLevel]);
            if (kpi2Type === 'company_kpi') {
              this.company_kpi = rates.map(item => {
                return {
                  level: this.levelEnums[item.ratingLevel] || '',
                  ratingLevel: item.ratingLevel,
                  coefficient: item.coefficient,
                  minScore: item.ratingScoreSet ? item.ratingScoreSet.minScore : 0,
                  maxScore: item.ratingScoreSet ? item.ratingScoreSet.maxScore : 0,
                  minCompareSymbol: item.ratingScoreSet ? item.ratingScoreSet.minCompareSymbol : 'cs_greater_than_or_equal_to',
                  maxCompareSymbol: item.ratingScoreSet ? item.ratingScoreSet.maxCompareSymbol : 'cs_less_than_or_equal_to',
                  minRankingScale: item.ratingRankingSet ? math.multiply(item.ratingRankingSet.minRankingScale || 0, 100) : 0,
                  maxRankingScale: item.ratingRankingSet ? math.multiply(item.ratingRankingSet.maxRankingScale || 0, 100) : 0,
                  remarks: item.remarks || ''
                };
              });
            } else if (kpi2Type === 'personal_kpi') {
              this.personal_kpi = rates.map(item => {
                var extra = item.ratingRankingSet.extraRankingSets || [];
                extra.sort((a, b) => this.sortEnums[a.ratingLevel] - this.sortEnums[b.ratingLevel]);
                return {
                  base: {
                    level: this.levelEnums[item.ratingLevel] || '',
                    ratingLevel: item.ratingLevel,
                    coefficient: item.coefficient,
                    minScore: item.ratingScoreSet ? item.ratingScoreSet.minScore : 0,
                    maxScore: item.ratingScoreSet ? item.ratingScoreSet.maxScore : 0,
                    minCompareSymbol: item.ratingScoreSet ? item.ratingScoreSet.minCompareSymbol : 'cs_greater_than_or_equal_to',
                    maxCompareSymbol: item.ratingScoreSet ? item.ratingScoreSet.maxCompareSymbol : 'cs_less_than_or_equal_to'
                  },
                  list: extra.map(subItem => {
                    return {
                      level: this.levelEnums[subItem.ratingLevel] || '',
                      ratingLevel: subItem.ratingLevel,
                      minRankingScale: subItem.minRankingScale !== null ? math.multiply(subItem.minRankingScale, 100) : undefined,
                      maxRankingScale: subItem.maxRankingScale !== null ? math.multiply(subItem.maxRankingScale, 100) : undefined,
                      remarks: subItem.remarks || ''
                    };
                  })
                };
              });
            }
          }
        }
      );
    },
    saveCompanyKpi() {
      var data = {
        kpi2Type: 'company_kpi',
        ratingSetItems: this.company_kpi.map(item => {
          return {
            ratingLevel: item.ratingLevel,
            coefficient: item.coefficient,
            ratingScoreSet: {
              minScore: item.minScore,
              maxScore: item.maxScore,
              minCompareSymbol: item.minCompareSymbol,
              maxCompareSymbol: item.maxCompareSymbol
            },
            ratingRankingSet: {
              minRankingScale: math.divide(item.minRankingScale, 100), // math.multiply
              maxRankingScale: math.divide(item.maxRankingScale, 100)
            },
            remarks: item.remarks
          };
        })
      };
      console.log(data, 'data');
      return this.$axios.post('/web/plan/api/kpi2RatingSet/save', { data },
        res => {
          if (res.isSuccess) {
            // this.$message.success('组织绩效设置保存成功');
          }
        }
      );
    },
    savePersonalKpi() {
      var data = {
        kpi2Type: 'personal_kpi',
        ratingSetItems: this.personal_kpi.map(item => {
          return {
            ratingLevel: item.base.ratingLevel,
            coefficient: item.base.coefficient,
            ratingScoreSet: {
              minScore: item.base.minScore,
              maxScore: item.base.maxScore,
              minCompareSymbol: item.base.minCompareSymbol,
              maxCompareSymbol: item.base.maxCompareSymbol
            },
            ratingRankingSet: {
              extraRankingSets: item.list.map(subItem => {
                return {
                  ratingLevel: subItem.ratingLevel,
                  minRankingScale: subItem.minRankingScale !== undefined ? math.divide(subItem.minRankingScale, 100) : null,
                  maxRankingScale: subItem.maxRankingScale !== undefined ? math.divide(subItem.maxRankingScale, 100) : null,
                  remarks: subItem.remarks
                };
              })
            }
          };
        })
      };
      console.log(data, 'data - personal');
      // return Promise.reject(34);
      return this.$axios.post('/web/plan/api/kpi2RatingSet/save', { data },
        res => {
          if (res.isSuccess) {
            // this.$message.success('个人绩效设置保存成功');
          }
        }
      );
    },
    handleSave() {
      Promise.all([this.saveCompanyKpi(), this.savePersonalKpi()]).then(() => {
        this.$message.success('保存成功');
      }).catch(err => {
       // this.$message.error('保存失败' + (err ? ('：' + err) : ''));
      });
    }
  }
};
</script>
<style scoped lang="scss">
.performance-setting-page {
  padding: 10px;
  background-color: #fff;
}

.personal-performance-container {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.personal-performance-item {
  flex: 1;
  min-width: 45%;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  padding: 15px;
  box-sizing: border-box;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
    border-radius: 5px;
}
.personal-performance-title {
    font-weight: bold;
    color: #409EFF;
    margin-bottom: 15px;
    padding-left: 10px;
    border-left: 4px solid #409EFF;
}
.company-performance-title {
    font-weight: bold;
    color: #ffcc66;
    margin-bottom: 15px;
    padding-left: 10px;
    border-left: 4px solid #ffcc66;
}

.score-range {
  margin-bottom: 10px;
}

h3,
h4 {
  margin-top: 0;
  margin-bottom: 15px;
}

.el-table {
  margin-bottom: 15px;
}
</style>
