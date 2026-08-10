/*
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-12-31 14:26:36
 */
import Api from '@/api';
const mixin = {
  data() {
    return {
      roleList:[],
      originalRoleList:[],
    }
  },
  methods:{
    // 找到父节点和祖先节点
    getCheckTag(list, id) {
      for (let i in list) {
        if (list[i].id === id) {
          return [list[i]]
        }
        if (list[i].childrenList != null) {
          let node = this.getCheckTag(list[i].childrenList, id)
          if (node !== undefined) {
            return node.concat(list[i])
          }
        }
      }
    },
    // 根据岗位id查询用户列表
    getUserListByDutyId(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          '/web/user/api/user/getUserVosByBizIds', {
          data: {
            queryTypeEnum:'DUTY',
            bizIds: [id]
          }
        },
        res => {
          if (res.isSuccess) {
            resolve(res.data)
          } else {
          }
        })
      })
    },
    // 获取角色列表
    getRoleList() {
      this.$axios.post(
        Api.roleManage.getRoleList,
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            scope: 'invest'
          }
        },
        res => {
          if (res.isSuccess) {
            this.roleList = res.data;
            this.roleList.forEach(async item=>{
              let result = await this.getRoleUserById(item.id);
              this.$set(item,'roleUserList',result)
              this.originalRoleList = JSON.parse(JSON.stringify(this.roleList));
            })
            // console.log('获取角色列表',this.roleList)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取角色下的用户
    getRoleUserById(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
        Api.roleManage.getRoleUserList,
        {
          data: {
            flowRoleId: id
          },
          pagination: false
        },
        res => {
          if (res.isSuccess) {
            resolve(res.data || [])
          } else {
          }
        })
      })
    },
  }
}
export default mixin;