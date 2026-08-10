<template>
  <addNewBudget v-if="type == 'add'" @changeComponent="changeComponent"></addNewBudget>
  <appendBueget v-else-if="type == 'append'" :params="params"></appendBueget>
  <detailBudget v-else-if="type == 'detail'" :params="params" ></detailBudget>
  <editeBudget v-else-if="type == 'edit'" :paramsInfo="params" @changeComponent="changeComponent"></editeBudget>
</template>
<script>
// import { deepClone } from '@/utils/index'
import addNewBudget from './addNewBudget.vue'
import appendBueget from './appendBueget.vue'
import detailBudget from './detailBudget.vue'
import editeBudget from './editeBudget.vue'
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
export default {
  name: 'Budget',
  components: { addNewBudget, appendBueget, detailBudget, editeBudget },
  created() {
    this.type = this.$route.query.type
    if (this.type == 'append' || this.type == 'detail' || this.type == 'edit') {
      let paramStr = this.$route.params.str
      if (paramStr) {
        localstorageSet('companyDetailParams', paramStr)
        this.params = JSON.parse(paramStr)
      } else {
        let str = localstorageGet('companyDetailParams')
        if (str) {
          this.params = JSON.parse(str)
        }
      }
      this.$once('hook:beforeDestroy', () => {
        localstorageRemove('companyDetailParams')
      })
    }
  },
  beforeRouteEnter(to, from, next) {
    to.meta.activeMenu = from.fullPath
    if(to.query.type == 'detail'){
      to.meta.saasTitle  = '预算详情'
      to.meta.activeMenu = from.fullPath//'/groupBudgetManage/companyBudget'
    }
    next()
  },
  data() {
    return {
      type: 'add',
      params: ''
    }
  },
  methods: {
    changeComponent({ list, type }) {
      this.type = type
      this.params = list
    }
  }
}
</script>
