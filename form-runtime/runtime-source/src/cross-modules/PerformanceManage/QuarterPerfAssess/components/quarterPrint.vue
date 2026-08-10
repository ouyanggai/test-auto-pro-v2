<template>
  <div class='app' v-if="quarterData">
    <h3 style="text-align: center;font-size:20px">{{quarterData.year}}年{{quarterEnums[quarterData.quarter]}}季度目标计划表</h3>
    <table class="print-table">
        <tr>
          <td style="width:100px;" class="fontWeighted">姓名</td>
          <td>
            <div>{{ quarterData.userName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">公司</td>
          <td>
            <div>{{ quarterData.companyName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">部门</td>
          <td>
            <div>{{ quarterData.departmentName }}</div>
          </td>
          <td style="width:100px;" class="fontWeighted">岗位</td>
          <td>
            <div>{{ quarterData.dutyName }}</div>
          </td>
          <!-- <td style="width:100px;" class="fontWeighted">公示状态</td>
          <td>
            formData.noticeStatus
          </td> -->
        </tr>
    </table>
    <table class="print-table">
        <tr>
            <th rowspan="2">编号</th>
            <th rowspan="2">年度目标及完成进度</th>
            <th rowspan="2">目标</th>
            <th rowspan="2">目标界定、定义等</th>
            <th rowspan="2">权重</th>
            <th rowspan="2">目标值</th>
            <th colspan="4">达成目标核心计划</th>
        </tr>
        <tr>
            <th>编号</th>
            <th>做什么</th>
            <th>达成什么结果</th>
            <th style="min-width:70px;">完成日期</th>
        </tr>
        <tr v-for="(item, index) in displayRows" :key="index">
            <td :rowspan="item.rowspan" v-if="item.rowspan">{{ item.planNumber }}</td>
            <td :rowspan="item.rowspan" v-if="item.rowspan">{{ item.ultimateAchieve }}</td>
            <td :rowspan="item.rowspan" v-if="item.rowspan">{{ item.phasedAchieve }}</td>
            <td :rowspan="item.rowspan" v-if="item.rowspan">{{ item.appraisalMethod }}</td>
            <td :rowspan="item.rowspan" v-if="item.rowspan">{{ item.weight }}</td>
            <td :rowspan="item.rowspan" v-if="item.rowspan">
                <div style="margin-bottom:7px">挑战值：{{ item.high_difficulty }}</div>
                <div style="margin-bottom:7px">达标值：{{ item.intermediate_difficulty }}</div>
                <div style="margin-bottom:7px">底限值：{{ item.easy }}</div>
            </td>
            <td v-if="!item.rowspan">{{ item.itemNumber }}</td>
            <td v-if="!item.rowspan">{{ item.doWhat }}</td>
            <td v-if="!item.rowspan">{{ item.result }}</td>
            <td v-if="!item.rowspan">{{ item.endTime }}</td>
        </tr>
        <tr>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td>{{ totalWeight }}%</td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
            <td></td>
        </tr>
    </table>
     <table class="print-table">
        <tr>
          <td style="width:25%;" class="fontWeighted">上级签字</td>
          <td>{{ quarterData.leaderConfirmTime }}</td>
        </tr>
        <tr>
          <td style="width:25%;" class="fontWeighted">本人签字</td>
          <td>{{ quarterData.myselfConfirmTime }}</td>
        </tr>
      </table>
  </div>
</template>

<script>
export default {
  name: '',
  props: {
    quarterData: {
      type: Object,
      default: _ => null
    }
  },
  components: {},
  data () {
    return {
      totalWeight: 0,
      quarterEnums: { 1: '一', 2: '二', 3: '三', 4: '四' }
    };
  },
  computed: {
    displayRows() {
      var data = this.quarterData;
      if (data) {
        console.log(data, 'quarterData--打印数据');
        var rows = [];
        this.totalWeight = 0;
        data.workPlans.map((item, index) => {
          var { ultimateAchieve, phasedAchieve, appraisalMethod, weight, high_difficulty, intermediate_difficulty, easy, items } = item;
          this.totalWeight += weight;
          rows.push({
            rowspan: item.items.length + 1,
            planNumber: index + 1,
            ultimateAchieve,
            phasedAchieve,
            appraisalMethod,
            weight,
            high_difficulty,
            intermediate_difficulty,
            easy
          });
          items.map((it, idx) => {
            var { doWhat, result, endTime } = it;
            rows.push({
              rowspan: 0,
              itemNumber: (index + 1) + '.' + (idx + 1),
              doWhat,
              result,
              endTime
            });
          });
        });
        return rows;
      }
      return [];
    }
  },
  watch: {},
  created() {},
  mounted() {},
  methods: {}
}

</script>
<style lang='scss' scoped>
.app {
 font-size: 12px;
}
.print-table + .print-table {
    margin-top: -1px;
}
.fontWeighted {
  font-weight: 700;
}
.print-table {
  border-collapse: collapse;
  width: 100%;
}
.print-table th,
.print-table td {
  border: 1px solid #a5a0a0;
  padding: 2px;
  text-align: center;
}
.print-table th {
  background-color: #f2f2f2;
}
</style>