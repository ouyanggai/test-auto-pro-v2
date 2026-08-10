import { Calculate } from '@/utils/number';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
const mixins = {
  methods: {
    //获取公司主岗
    getMainDuty() {
      this.$axios.post(Api.annualBudget.getCompanyListOfOnDuty, {}).then(res => {
        if (res.isSuccess) {
          let originCompanyOption = res.data || [];
          let companyOption = originCompanyOption.map(item => {
            return {
              label: item.name,
              value: item.id
            }
          })
          this.formList[0][0].data = companyOption
          let mainCompanyId = localstorageGet('companyId')
          let mainCompany = originCompanyOption.find(item => item.id == mainCompanyId)
          if (mainCompany) {
            this.formList[0][0].value = mainCompany.id
            this.getCompnayOtherInfo(mainCompany.id)
          }
        } else {
          this.$message.error('人员暂无公司岗位')
        }
      })
    },

    //公司切换
    getCompnayOtherInfo(id) {
      this.getDepartmentList(id)
      this.getProjectVosByCompanyId(id)
    },
    // 获取公司下的部门列表
    getDepartmentList(id) {
      this.$axios.post(
        Api.budgetManage.getCompanyDeptVoByCompanyId,
        {
          data: {
            id
          }
        },
        res => {
          if (res.isSuccess) {
            let list = []
            if (res.data && res.data.departmentVos) {
              list = res.data.departmentVos.map(item => {
                if (item.departmentName == '公司领导') item.departmentName = '公司固定费用'
                return {
                  id: item.id,
                  name: item.departmentName
                }
              })
            }
            let index = -1
            list.forEach((item, i) => {
              if (item.name == '公司固定费用') index = i
            })
            if (index > -1) {
              list.unshift(list.splice(index, 1)[0])
            }
            this.config.departOptions = list
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取关联项目
    getProjectVosByCompanyId(companyId) {
      this.$axios.post(
        Api.annualBudget.getProjectVosByCompanyId,
        {
          data: {
            companyId
          }
        },
        res => {
          if (res.isSuccess) {
            // console.log('res.data', res)
            // console.log('res.this.config.projectOptions', this.config.projectOptions)
            this.config.projectOptions = res.data
          }
        }
      )
    },
    //取消
    cancel() {
      this.$confirm("是否确认取消?", "提示", {
        closeOnClickModal: false,
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning"
      }).then(() => {
        this.prevStepHandle()
      })
    },
  }
}
export default mixins
