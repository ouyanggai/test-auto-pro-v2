/**
 * 跳转至项目空间的方法，因为用到的地方有点多，避免后去修改遗漏的问题，统一调用此方法
 */
import { localstorageGet, localstorageSet } from '@/utils/auth';
const mixin = {
  methods: {
    goToProject(row) {
      let companyId = row.projectMainDeptVo?.companyId || localstorageGet('companyId')
      return new Promise((resolve, reject) => {
        this.$axios.post( // 进入项目保存公司类型companyType
          '/web/project/api/getRelatedCompanyType',
          {
            data: {
              id: row.id,
              userId: this.$store.state.user.userId,
              companyId//: this.$store.state.user.companyId
            }
          },
          res => {
            if (res.isSuccess) {
              this.$store.commit('user/SET_COMPANY_TYPE', res.data.companyType || '');
            } else {
              this.$message.error(res.message);
            }
          }
        );
        // this.$store.commit('user/SET_COMPANY_ID', row.companyId); // 项目所属公司id
        // this.$store.commit('user/SAVE_PROJECT_RELATION_TYPE', row.relationType);
        this.$axios.post( // 进入项目h获取用户在项目部下的部门id
          '/web/project/api/proMainDept/getDepartmentId',
          {
            data: {
              projectMainDeptVo: {
                projectId: row.id,
                companyId//: this.$store.state.user.companyId
              }
            }
          },
          res => {
            if (res.isSuccess) {
              //存储当前路由，以便返回后直接跳回来
              localstorageSet('redirect_path',this.$route.fullPath)
              localstorageSet('projectId', row.id);
              localstorageSet('projectCompanyId', row.companyId);
              this.$store.commit('user/SET_GROUP_DEPARTMENT', 'department'); // 是否平台
              this.$store.commit('user/SAVE_PROJECT_DEPARTMENT_ID', row.proMainDeptId); // 项目部id
              this.$store.commit('user/SAVE_PROJECT_ID', row.id); // 项目id
              this.$store.commit('user/SAVE_PROJECT_Name', row.name); // 项目name
              this.$store.commit('user/SAVE_PROJECT_Type', row.type); // 项目type
              this.$store.commit('user/SAVE_PROJECT_RELATION_TYPE', row.relationType); // 项目权限：公有和私有s
              this.$store.commit('user/SAVE_PROJECT_StartTime', row.startTime.slice(0, 10)); // 项目开始时间
              this.$store.commit('user/SAVE_PROJECT_EndTime', row.endTime.slice(0, 10)); // 项目结束时间
              this.$store.commit('user/SET_PROJECT_COMPANY_ID', row.companyId); // 项目所属公司id


              this.$store.commit('user/SET_DEPARTMENTID', res.data); // 用户所在项目下的部门id
              localstorageSet('departmentId', res.data);
              this.$store.dispatch('permission/getPermissionList').then(() => {
                this.$store.commit('app/SAVE_TITLE', row.name);
                resolve();
              // this.$router.push({ path:'/approveManage',query:{action:'addApprove'}})
              }).catch(() => {
                this.$message.error('该用户没有权限访问');
                this.$store.commit('user/SET_GROUP_DEPARTMENT', 'group'); // 是否平台
                this.$store.commit('user/SAVE_PROJECT_DEPARTMENT_ID', ''); // 项目部id
                this.$store.commit('user/SAVE_PROJECT_ID', ''); // 项目id
                this.$store.commit('user/SAVE_PROJECT_Name', ''); // 项目name
                this.$store.commit('user/SAVE_PROJECT_RELATION_TYPE', ''); // 项目权限：公有和私有s
              // this.$store.commit('user/SET_COMPANY_ID', ''); // 项目所属公司id
              // reject()
              });
            } else {
              this.$message.error(res.message);
            // reject()
            }
          });
      });
    }
  }
};
export default mixin;
