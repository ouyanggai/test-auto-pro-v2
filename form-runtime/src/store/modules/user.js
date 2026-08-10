/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 14:22:24
 */
import {
  localstorageSet,
  localstorageGet,
  localstorageRemove
} from '@/utils/auth';

const state = {
  token: localstorageGet('token') || '',
  customerCode: localstorageGet('customerCode') || '',
  userId: localstorageGet('userId') || '',
  userName: localstorageGet('userName') || '',
  dutyId: localstorageGet('dutyId') || '',
  dutyLevel: localstorageGet('dutyLevel') || '',
  dutyName: localstorageGet('dutyName') || '',
  userAccount: localstorageGet('userAccount') || '',
  projectId: localstorageGet('projectId') || '', // 项目id
  projectName: localstorageGet('projectName') || '', // 项目名称
  projectType: localstorageGet('projectType') || '', // 项目名称
  projectStartTime: localstorageGet('projectStartTime') || '', // 项目开始时间
  projectEndTime: localstorageGet('projectEndTime') || '', // 项目结束时间
  projectDepartmentId: localstorageGet('projectDepartmentId') || '', // 项目部id
  projectRelationType: localstorageGet('projectRelationType') || '', // 项目公有私有
  companyId: localstorageGet('companyId') || '',
  groupCompanyId: '',
  groupDepartment: localstorageGet('groupDepartment'), // 是否平台
  companyType: localstorageGet('companyType') || '',
  companyName: localstorageGet('companyName') || '',
  userDepartmentName: localstorageGet('userDepartmentName') || '', // 部门名称
  enName: localstorageGet('enName') || '',
  isChangePassword: '', // 是否需要打开修改密码弹框
  departmentId: localstorageGet('departmentId') || '', // 所属项目的部门id
  projectCompanyId: localstorageGet('projectCompanyId') || '', // 所属项目的公司id
  linkUsers: localstorageGet('linkUsers') || '',
  isFirstVisitBacklog: true // 是否第一次访问工作台页面
};
const mutations = {
  // 初始化
  RESET_STATE: (state) => {
    state.token = '';
    state.customerCode = '';
    state.customerFlag = '';
    state.userId = '';
    state.userName = '';
    state.dutyId = '';
    state.dutyLevel = '';
    state.dutyName = '';
    state.userAccount = '';
    state.projectId = '';
    state.projectName = '';
    state.projectType = '';
    state.projectStartTime = '';
    state.projectEndTime = '';
    state.projectDepartmentId = '';
    state.projectRelationType = ''; // 项目公有或者私有
    state.companyId = '';
    state.groupCompanyId = '';
    state.companyName = '';
    state.departmentName = '';
    state.groupDepartment = '';
    state.companyType = '';
    state.enName = '';
    state.isChangePassword = '';
    state.departmentId = '';
    state.projectCompanyId = '';
    state.projectList = [];
    state.linkUsers = ''
    localstorageRemove('token');
    localstorageRemove('customerCode');
    localstorageRemove('customerFlag');
    localstorageRemove('userId');
    localstorageRemove('userName');
    localstorageRemove('dutyId');
    localstorageRemove('dutyLevel');
    localstorageRemove('dutyName');
    localstorageRemove('token');
    localstorageRemove('userAccount');
    localstorageRemove('projectId');
    localstorageRemove('projectName');
    localstorageRemove('projectType');
    localstorageRemove('projectStartTime');
    localstorageRemove('projectEndTime');
    localstorageRemove('projectDepartmentId');
    localstorageRemove('projectRelationType');
    localstorageRemove('companyId');
    localstorageRemove('companyName');
    localstorageRemove('departmentName');
    localstorageRemove('groupDepartment');
    localstorageRemove('companyType');
    localstorageRemove('enName');
    localstorageRemove('departmentId');
    localstorageRemove('projectCompanyId');
    // 清除首次访问工作台标志
    localStorage.removeItem('isFirstVisitBacklog');
    //清除所有全过程的缓存
    const pattern = /^invest-power-system/
    for (let key in localStorage) {
      if (pattern.test(key)) {
        localStorage.removeItem(key)
      }
    }
  },
  SET_LINK_USERS:(state, data) => {
    state.linkUsers = data;
    localstorageSet('linkUsers', data);
  },
  SET_TOKEN: (state, token) => {
    state.token = token;
    localstorageSet('token', token);
  },
  SET_CUSTOMERCODE: (state, customerCode) => {
    state.customerCode = customerCode;
    localstorageSet('customerCode', customerCode);
  },
  SET_CUSTOMERFLAG: (state, customerFlag) => {
    state.customerFlag = customerFlag;
    localstorageSet('customerFlag', customerFlag);
  },
  SET_USER_ID: (state, id) => {
    state.userId = id;
    localstorageSet('userId', id);
  },
  SET_USER_NAME: (state, name) => {
    state.userName = name;
    localstorageSet('userName', name);
  },
  SET_DUTY_ID: (state, id) => {
    state.dutyId = id;
    localstorageSet('dutyId', id);
  },
  SET_DUTY_LEVEL: (state, id) => {
    state.dutyLevel = id;
    localstorageSet('dutyLevel', id);
  },
  SET_DUTY_NAME: (state, name) => {
    state.dutyName = name;
    localstorageSet('dutyName', name);
  },
  SET_EN_NAME: (state, name) => {
    state.enName = name;
    localstorageSet('enName', name);
  },
  SET_USER_ACCOUNT: (state, name) => {
    state.userAccount = name;
    localstorageSet('userAccount', name);
  },
  SAVE_PROJECT_ID: (state, id) => {
    state.projectId = id;
    localstorageSet('projectId', id);
  },
  SAVE_PROJECT_Name: (state, name) => {
    state.projectName = name;
    localstorageSet('projectName', name);
  },
  SAVE_PROJECT_Type: (state, name) => {
    state.projectType = name;
    localstorageSet('projectType', name);
  },
  SAVE_PROJECT_StartTime: (state, name) => {
    state.projectStartTime = name;
    localstorageSet('projectStartTime', name);
  },
  SAVE_PROJECT_EndTime: (state, name) => {
    state.projectEndTime = name;
    localstorageSet('projectEndTime', name);
  },
  SAVE_PROJECT_DEPARTMENT_ID: (state, id) => {
    state.projectDepartmentId = id;
    localstorageSet('projectDepartmentId', id);
  },
  SAVE_PROJECT_RELATION_TYPE: (state, type) => {
    state.projectRelationType = type;
    localstorageSet('projectRelationType', type);
  },
  SET_GROUP_DEPARTMENT: (state, value) => {
    state.groupDepartment = value;
    localstorageSet('groupDepartment', value);
  },
  SET_COMPANY_ID: (state, id) => {
    console.log(state, id);
    state.companyId = id;
    localstorageSet('companyId', id);
  },
  SET_GROUP_COMPANY_ID: (state, id) => {
    state.groupCompanyId = id;
  },
  SET_COMPANY_TYPE: (state, id) => {
    state.companyType = id;
    localstorageSet('companyType', id);
  },
  SET_COMPANY_NAME: (state, id) => {
    state.companyName = id;
    localstorageSet('companyName', id);
  },
  SET_DEPARTMENT_NAME: (state, id) => {
    state.userDepartmentName = id;
    localstorageSet('userDepartmentName', id);
  },
  SET_IS_CHANGE_PASSWORD: (state, data) => {
    state.isChangePassword = data;
  },
  SET_DEPARTMENTID: (state, data) => {
    state.departmentId = data;
  },
  SET_PROJECT_COMPANY_ID: (state, data) => {
    state.projectCompanyId = data;
  },
  SET_IS_FIRST_VISIT_BACKLOG: (state, data) => {
    state.isFirstVisitBacklog = data;
  }
};

const actions = {

};

export default {
  namespaced: true,
  state,
  mutations,
  actions
};
