/*
 * @Descripttion:项目接口路径js
 * @Author: Calvin
 * @Date: 2021-04-19 19:08:32
 */
const api = {
  user: {
    login: '/web/user/api/login/user/login',
    linkageLogin:'/web/user/api/login/user/login',//web/user/api/login/user/linkageLogin', //新登录接口
    switchLinkage :'/web/user/api/login/user/switchLinkage', //切换租户
    checkLoginStatus :'/web/user/api/login/user/checkLoginStatus', //查看当前登录状态
    getPlaltformName: '/web/user/api/clientelePlatform/findBySign', // 获取平台名字
    saveFileFold: '/web/file/api/file/saveFile', // 操作文件/文件夹
    uploadFile: '/web/file/api/file/uploadFile', // 上传文件
    uploadFileList: '/web/file/api/file/uploadFileList', // 上传文件(批量)
    // login: 'user/login',
    loginOut: '/web/user/api/login/user/loginOut',
    // loginOut: 'user/loginOut',
    modifyPassWord: 'web/user/api/user/updatePassword',
    checkPassword: '/web/user/api/login/user/checkPassword', // 校验密码是否正确
    getVertifyCode: '/web/user/api/login/user/getVerifiCode', // 获取验证码
    getGroupPermissionList: '/web/user/api/resources/getRoleResourceTree', // 获取平台权限菜单
    getPermissionList: '/web/user/api/resources/getRoleResourceTree', // 获取项目权限菜单
    getCmsTreeList: '/web/cms/api/getCmsTreeList', // 帮助中心
    findByCompanyIdUserList: '/web/user/api/user/findByCompanyIdUserList',
    getUserVosByBizIds: '/web/user/api/user/getUserVosByBizIds',

    getFileBizIdByBusinessId: '/web/file/api/biz/list', // 根据业务id查询多个文件业务id（通用，全量，以最后一次保存内容为准）
    saveBatchByFileBizId: '/web/file/api/biz/saveBatch', // 业务id绑定多个文件业务id（通用）
    saveBatchFile: '/web/file/api/relationFile/saveBatch', // 批量文件id绑定业务id
    saveFile: '/web/file/api/relationFile/save', // 文件业务关联操作
    findByRelationIdList: '/web/file/api/relationFile/findByRelationIdList', // 根据业务id查询文件数据
    deleteBatchFile: '/web/file/api/relationFile/deleteByRelationIdAndFileIds', // 批量文件id解绑业务id
    updateFile: '/web/file/api/file/editFile', // 重命名文件
    replaceFile: '/web/file/api/relationFile/replaceFile', // 替换文件
    deleteFileRelation: '/web/file/api/relationFile/deleteByRelationIdAndFileIds', // 删除业务与文件的关联关系
    getAllCompanyList: '/web/user/api/company/getCompanyList', // 获取所有公司列表
    getSelfAndChildrenCompanyList: '/web/user/api/company/getSelfAndChildrenCompanyList', // 查询自己及子公司列表
    getJobTitleTree: '/web/user/api/company/children', // 获取公司岗位树数据
    dutyLevel: '/web/user/api/dutyLevel/list', // 查询职级列表
    getSingleCompany: '/web/user/api/company/getSingleCompanyVosWithinProjectCompany', // 获取名下所有公司和工程公司
    getUserInfoById: '/web/user/api/user/getUserInfoById', // 根据id获取用户信息
    findUserSet: '/web/user/api/userSet/findUserSet', // 获取个性化设置
    saveUserSet: '/web/user/api/userSet/saveUserSet', // 保存个性化设置
    findDeptVosByRefId :'/web/user/api/company/findDeptVosByRefId' //通过公司获取项目列表
  },
  admin: {
    findByDictCode: '/web/dict/api/dictData/findByDictCode' // 获取数据字典
  },
  flowProcessStatistics:{
    getFlowProcessStatistics: '/web/flowStatistics/list', // 获取流程统计
    exportProcessStatistics: '/web/flowStatistics/exportExcel', // 导出流程统计
    getFlowAuditRecordList: '/web/flowAuditRecord/list', // 流程统计详情
    getTimeoutSkipStatisticsList: '/web/flowInstanceApi/timeoutSkipStatisticsList', // 获取超时流程统计
    exportTimeoutSkipStatisticsList: 'web/flowStatistics/exportTimeoutSkipStatisticsExcel', // 导出超时流程统计数据
  },
  // 表单所需接口
  formMaking: {
    deleteReserveKpiGroup: '/web/plan/api/reserveKpiGroup/delete', // 后备英才轮岗考核表删除
    contractPaymentDelete: '/web/measuring/api/contractPaymentForm/delete', // 合同付款表单删除
    contractCollectionDelete: '/web/measuring/api/contractCollection/delete', // 合同收款表单删除
    contractInvoicingDelete: '/web/measuring/api/contractInvoicing/delete', // 合同开票申请表单删除
    projectApprovalBudgetDelete: '/web/project/api/projectApprovalBudget/delete', // 项目立项删除
    proposedSupplierDelete: '/web/warehouse/center/api/supplierChooseInfo/delete', // 拟选用供应商评审表删除
    fixedAssetsDelete: '/web/warehouse/api/fixedAssets/delete', // 删除固定资产业务
    costFundsTransactionsDelete: '/web/measuring/api/costFundsTransactions/delete', // 删除请款单（资金往来和投资款）
    costFundsTransactionsSave: '/web/measuring/api/costFundsTransactions/save', // 保存请款单（资金往来和投资款）
    bidBondRefundSave: '/newMeasuringApi/measuringApi/bidBondRefund/save', // 保存投标保证金退还请款
    costFundsTransferDelete: '/web/measuring/api/costFundsTransfer/delete', // 删除资金调拨单（资金上划和资金下拨）
    costFundsTransferSave: '/web/measuring/api/costFundsTransfer/save', // 保存资金调拨单（资金上划和资金下拨）
  },
  dataCenter:{
    ruleSet:{
      getRuleList:'/web/measuringApi/yearAccounting/list', // 规则列表查询
      deleteRule:'/web/measuringApi/yearAccounting/delete', // 规则删除
      saveRule:'/web/measuringApi/yearAccounting/save', // 规则新增和更新
      getCustomerAccounting:'/web/measuringApi/yearAccounting/getCustomerAccounting', // 数据统计查询
      manualInputCompanyAccounting:'/web/measuringApi/yearAccounting/manualInputCompanyAccounting', // 手动录入数据保存接口
      saveSummaryReport: '/web/measuringApi/yearAccounting/saveSummaryReport', // 分析报告保存
      findReportList: '/web/measuringApi/yearAccounting/findReportList', // 分析报告表格数据
      deleteSummaryReport: '/web/measuringApi/yearAccounting/deleteSummaryReport', // 删除分析报告
      getAccountingDetails: '/web/measuringApi/yearAccounting/getAccountingDetails' // 删除分析报告
    }
  },
  home: {
    bimUrl: 'HLYYsipmweb/api/version',
    wbsTypePlanList: '/web/plan/api/wbsTypePlan/list', // 进度计划列表数据
    getTopPlanList: '/web/plan/api/wbsTypePlan/getTopPlanList', // wbs列表数据
    getMyWbsList: '/web/plan/api/wbsTypePlan/getMyWbs', // wbs列表数据
    saveComponentUrl: '/web/plan/api/wbsTypePlan/relationComponent', // 关联构件
    unhookComponent: '/web/plan/api/wbsTypePlan/unhookComponent', // 解除关联构件
    findByProjectIdList: 'proBim/findByProjectIdList', // 根据项目id获取模型数据
    wbsTypePlanDelete: '/web/plan/api/wbsTypePlan/delete' // 开发计划列表数据
  },
  // 全局消息
  globalMessage: {
    changeToRead: '/web/message/api/changeReadState', // 消息标为已读
    getMessageList: '/web/message/api/list', // 获取消息列表
    closeMessage: '/web/message/api/closeMessage', // 取消提醒
    deleteMessage: '/web/message/api/delete'// 删除消息列表
  },
  myProject: {
    getProjectList: 'web/project/api/getProjectList',
    // getProjectList: 'web/project/api/list',
    getRelatedCompanyList: '/project/getRelatedCompanyList', // 相关方信息
    getOrganizeTree: '/project/getOrganizeTree', // 组织架构

    getProjectDetail: '/web/project/api/findById', // 项目详情
    getProjectEnum: '/web/project/api/getProjectEnum', // 项目相关枚举值接口
    getProjectTypeList: '/web/project/api/type/list', // 获取项目类型
    getProjectArea: '/web/areaApi/area/findProvince', // 获取项目所在地
    editBasicInfo: '/web/project/api/editDetail', // 编辑基本信息
    saveProject: '/web/project/api/save', // 新增项目
    updateProject: '/web/project/api/update', // 编辑项目
    delProject: '/web/project/api/delete' // 删除项目
  },
  // // 基本信息
  // basicInfo:{
  //   getProjectList: 'web/project/api/getProjectEnum',
  // },
  // 成长计划
  growPlan: {
    indicatorModel: {
      // 分级指标
      getClassIndicatorData: '/web/growCenterApi/kpi/kpiLevelByDutyId', // 分级 KPI 指标概况接口
      kpiLevelDetailByDutyId: '/web/growCenterApi/kpi/kpiLevelDetailByDutyId', // 分级指标模型详情接口
      chooseKpi: '/web/growCenterApi/kpi/chooseKpi', // 查询工作项对应的技能（分级指标选择技能和能力的列表）
      kpiLevelDetailEdit: '/web/growCenterApi/kpi/kpiLevelDetailEdit', // 新增和编辑分级指标
      deleteLevelKpi: '/web/growCenterApi/kpi/deleteLevelKpi', // 删除分级指标
      saveWorkExamine: '/web/growCenterApi/kpi/saveWorkExamine', // 保存考核方法（分级指标）

      // 基础指标
      queryKpiByDutyId: '/web/growCenterApi/kpi/queryKpiByDutyId', // 基础指标概况和详情接口
      saveSkill: '/web/growCenterApi/kpi/save', // 保存技能和能力的接口(基础指标的新增和编辑)
      delete: '/web/growCenterApi/kpi/delete' // kpi编辑删除接口(基础指标的删除)
    },
    personGrowth: {
      getPersonYearGrowData: '/web/growCenterApi/report/userYearReport', // 获取个人年度成长数据
      getPersonYearMonthlyData: '/web/growCenterApi/report/userLearnReport', // 获取个人年度成长数据中的月度指标
      getPersonYearTargetRate: '/web/growCenterApi/report/userIndexReport', // 查询用户年度达标率
      getPersonYearRank: '/web/growCenterApi/report/userRankReport', // 查询用户年度岗位排名
      getPersonYearExtraGrow: '/web/growCenterApi/report/bonusPromote' // 查询用户年度岗位排名
    },
    groupGrowth: {
      getCompanyDepyList:
        '/web/growCenterApi/growUser/findCompanyAndDeptByUserId', // 获取当前用户所有的公司和部门
      checkIsMadeGroupPlan: '/web/growCenterApi/growTarget/isExist', // 查询当前部门是否已经制定年度计划
      // 制定成长计划
      getIndicatorByDutyId: '/web/growCenterApi/growTarget/list', // 部门-职位-职位目标接口（制定成长目标页面根据岗位id查指标）
      saveGrowthTarget: '/web/growCenterApi/growTarget/save', // 成长目标制定保存接口
      findCompanyAndDeptByUserId:
        '/web/growCenterApi/growUser/findCompanyAndDeptByUserId', // 查询用户所有的公司以及公司下所有的部门
      findDutyByDeptId: '/web/growCenterApi/growUser/findDutyByDeptId', // 根据部门id查询部门下所有的职位

      // 确定成长计划
      queryGroupTarget: '/web/growCenterApi/growTarget/query', // 查询已经制定好的团队目标(确认团队成长目标)
      findUserTargetByDutyId: '/web/growCenterApi/targetPerson/queryByDeptId', // 根据职位id查询该职位下所有用户的目标
      findPersonTargetList: '/web/growCenterApi/targetPerson/list', // 制定个人目标查询接口
      savePersonTarget: '/web/growCenterApi/targetPerson/save', // 保存团队个人目标

      // 员工成长记录
      findUserLevel: '/web/growCenterApi/userFunction/findUserLevel', // 查询员工等级接口
      getUserLevelOptions: '/web/growCenterApi/userFunction/findLevelByDutyId', // 查询某职位的等级列表
      saveUserGrading: '/web/growCenterApi/userFunction/save', // 员工定级接口

      // 用户信息查询
      findDutyByUserId: '/web/growCenterApi/growUser/findDutyByUserId', // 根据用户id查询某个空间的职位和部门以及职位对应的职级
      findDutyInfoAndLevelInfoByUserId:
        '/web/growCenterApi/growUser/findDutyInfoAndLevelInfoByUserId', // 根据用户id和岗位id查询职位查询职级和岗位信息

      // 上级评定
      judgeSave: '/web/growCenterApi/judge/save', // 为用户添加技能或能力
      judgeDelete: '/web/growCenterApi/judge/delete', // 删除用户技能或者能力
      judgeError: '/web/growCenterApi/judge/error', // 标记用户技能或者能力为失误
      judgeUserMonthReport: '/web/growCenterApi/judge/userMonthReport', // 当月的技能和能力情况查询接口
      // judgeUserMonthReport: '/web/growCenterApi/kpi/findPersonalMonthlyReport', // 当月的技能和能力情况查询接口
      judgeGetYear: '/web/growCenterApi/judge/getMonth', // 返回月报的年份
      judgeDeleteError: '/web/growCenterApi/judge/deleteError', // 删除标记用户技能或者能力的失误

      // 公司月度成长报表
      getCompanyMonthlyChartData:
        '/web/growCenterApi/report/companyIndexMonthReport', // 获取公司月度左侧两个图表数据
      getDeptGrowData: '/web/growCenterApi/report/companyDeptIndexMonthReport', // 获取公司月度部门成长情况
      getOutstandingStaffList:
        '/web/growCenterApi/kpi/findUserYearReportGrowProminent', // 获取公司月度突出员工&&年度突出员工
      getOutstandingIndicatorList:
        '/web/growCenterApi/kpi/findPromoteProminentKpi', // 获取公司月度提升突出指标
      getCompanyGrowRecordInMonth:
        '/web/growCenterApi/kpi/findUserYearReportGrowRecord', // 获取公司月度提升记录

      // 公司年度成长报表
      getCompanyGrowBaseData:
        '/web/growCenterApi/report/companyIndexYearReport', // 获取公司年度达标及超出预期数据
      getCompanyDeptRate:
        '/web/growCenterApi/report/companyDeptIndexYearReport', // 获取公司年度部门完成情况
      getCompanyYearIndicatorList:
        '/web/growCenterApi/report/companyKpiYearReport', // 获取公司年度部门完成情况

      // 部门月报
      teamMonthReport: '/web/growCenterApi/report/teamMonthReport', // 部门月报列表
      teamWorkRecordMonthReport:
        '/web/growCenterApi/report/teamWorkRecordMonthReport', // 部门月度成长记录

      // 部门年报
      teamIndexYearReport: '/web/growCenterApi/report/teamIndexYearReport', // 左边 3小框
      teamKpiYearReport: '/web/growCenterApi/report/teamKpiYearReport', // 中间
      teamDutyYearReport: '/web/growCenterApi/report/teamDutyYearReport', // 下部
      teamWorkYearReport: '/web/growCenterApi/report/teamWorkYearReport' // 右边
    },
    groupGrowRecord: {
      getRecordUserList: '/web/growCenterApi/userFunction/list', // 查询当前部门下成员的成长记录和岗位等级列表
      getStaffGrowRecordList: '/web/growCenterApi/userFunction/record', // 获取员工的成长记录列表
      getUserGrowDetail: '/web/growCenterApi/userFunction/findById' // 查询员工成长详情
    }
  },
  // 任务管理
  taskManage: {
    // taskManage.myTask.getCompanyDepartTree
    // getDutyList: '/web/user/api/duty/getDepartmentIdList', // 获取岗位信息列表结构
    myTask: {
      getTaskList: '/web/plan/api/targetTypePlan/getPlanByUser', // 查询任务列表接口
      getTaskIndicatorList: '/web/plan/api/workTarget/getWorkTargetByPlanId', // 查询任务指标栏接口
      getWorkPlanList: '/web/plan/api/planItem/getPlanItemListByPlanId', // 查询工作计划栏接口
      getNewWorkPlanList: '/web/plan/api/planItem/getPlanItemListByWorkPlan', // 新版查询工作计划栏接口
      getTaskDistributeList:
        '/web/plan/api/targetTypePlan/getChildrenPlanListById', // 查询任务下发栏接口
      getTaskListDetail: '/web/plan/api/targetTypePlan/findById', // 查看我的任务详情接口
      findUserDetail: '/web/user/api/user/findById', // 查询用户详情
      getUserDutyList: '/web/user/api/duty/findById', // 根据岗位id，查询岗位
      getDepartmentInfo:
        'web/project/api/proMainDept/getMainDeptForPlatformPlan', // 拿不到项目部id的时候用这个接口查询项目部id
      getIndicatorListWithDuty:
        '/web/plan/api/workTargetType/getWorkTargetTypeByDuty', // 根据岗位获取指标类型
      getWorkItemListWithType:
        '/web/plan/api/planItemTemplate/getPlanItemTemplateListByWorkTargetType', // 根据指标类型获取工作项接口
      getWorkItemListByDuty:
        '/web/plan/api/planItemTemplate/getPlanItemTemplateListByDuty', // 根据岗位获取工作项接口
      saveWorkPlan: '/web/plan/api/planItem/addPlanItemForWorkPlan', // 新版添加工作计划
      addPlanItemForWorkTarget:
        '/web/plan/api/planItem/addPlanItemForWorkTarget', // 指标添加工作项保存接口
      editWorkPlanTime: '/web/plan/api/planItemStep/editStepTime', // 修改工作计划时间接口
      getWorkPlanDetailList:
        '/web/plan/api/planItem/getPlanItemListByWorkTargetId', // 获取工作计划详情列表
      getCompanyDepartTree: '/web/user/api/company/children', // 获取公司部门架构数据
      getMainDeptTree: '/web/project/api/proMainDept/getMainDeptTree', // 获取项目部架构数据
      saveDistribute: '/web/plan/api/workTarget/splitWorkTarget', // 保存任务分解
      editDistribute: '/web/plan/api/workTarget/editSplitWorkTarget', // 编辑任务分解
      updatePlanKpiRelation:
        '/web/plan/api/targetTypePlan/updatePlanKpiRelation', // 更新kpi接口(任务详情更新目标责任书关联)
      updatePlanProjectRelation:
        '/web/plan/api/targetTypePlan/updatePlanProjectRelation', // 更新关联项目接口(任务详情更新关联项目)
      submitPlanItemStep: '/web/plan/api/planItemStep/submitPlanItemStep', // 提交完成
      submitPlanTask: '/web/plan/api/targetTypePlan/submitPlanTask', // 审核任务提交
      getPlanCountByUser: '/web/plan/api/targetTypePlan/getPlanCountByUser', // 待办页面数量
      getPlanItemStepCountByFinishStatus: '/web/plan/api/targetTypePlan/getPlanItemStepCountByFinishStatus' // 待办页面数量
    },
    taskArrange: {
      // 任务下发
      getAssocialProjectList: '/web/project/api/getProjectList', // 获取关联项目列表数据
      getCompanyDepartTree: '/web/user/api/company/children', // 获取公司部门架构数据
      getAllUsersOfRelation: '/web/user/api/user/getAllUsersOfRelation', // 获取已经添加到项目中的人员
      getMainDeptTree: '/web/project/api/proMainDept/getMainDeptTree', // 获取项目部架构数据
      getRelatedPartyTree: '/web/project/api/relatedParty/getRelatedPartyTree', // 获取项目相关方数据
      getIndicatorTypeList: '/web/plan/api/workTargetType/list', // 查询指标类型接口
      getCompanyHeaderDutyList: '/web/user/api/duty/findByUserIdDutyList', // 根据负责人id查询负责人的岗位列表(平台)
      getProjectHeaderDutyList:
        '/web/project/api/proMainDept/getDutyListByProjectId', // 根据负责人id查询负责人的岗位列表(项目)
      getItemKpiList: '/web/growCenterApi/taskKpi/queryWorkItemKpi', // 查询工作项关联的kpi列表
      getMyIssuePlan: '/web/plan/api/targetTypePlan/getMyIssuePlan', // 查询下发任务的数据列表
      deleteTask: '/web/plan/api/targetTypePlan/delete', // 删除任务
      saveTask: '/web/plan/api/targetTypePlan/save', // 保存任务
      targetTypePlanDetail: '/web/plan/api/targetTypePlan/findById', // 下发任务详情
      addSubTask: '/web/plan/api/targetTypePlan/addChildrenPlan', // 添加子任务
      updateTask: '/web/plan/api/targetTypePlan/update', // 编辑任务
      getCurrentKpiGroup: '/web/plan/api/kpiGroup/getCurrentKpiGroup', // 获取责任书接口
      cancelBusiness: '/web/plan/api/targetTypePlan/removePlanBusinessRelation', // 解除任务关联业务
      getWorkTargetDetail: '/web/plan/api/kpiGroup/findById',
      getTargetCompleteInfo: '/web/plan/api/targetTypePlan/getKpiRelationPlan', // 获取目标责任书任务完成情况
      getUserGroupList: '/web/user/api/userGroup/list', // 群组列表
      addUserGroupList: '/web/user/api/userGroup/save', // 新增群组
      updateUserGroupList: '/web/user/api/userGroup/update', // 更新群组
      getGroupPersonList: '/web/user/api/userGroup/findById' // 获取群组中人员列表
    },
    indicatorType: {
      // 指标类型
      saveWorkTargetType: '/web/plan/api/workTargetType/save', // 新增指标类型
      getWorkTargetType: '/web/plan/api/workTargetType/list', // 获取指标类型列表
      findWorkTargetType: '/web/plan/api/workTargetType/findById', // 获取指标类型详情
      updateWorkTargetType: '/web/plan/api/workTargetType/update', // 更新指标类型
      deleteWorkTargetType: '/web/plan/api/workTargetType/delete', // 删除指标类型
      setEnableType: '/web/plan/api/workTargetType/setEnableType', // 指标类型归档/还原
      setSort: '/web/plan/api/workTargetType/setSort', // 排序
      uploadTemplate: '/web/plan/api/workTargetType/uploadTemplate'
    },
    jobWbsSet: {
      // 岗位WBS设置
      getProjectDutyList: '/web/user/api/projectMainDept/children', // 获取岗位信息列表结构 项目入口的调用这个
      getCompanyDutyList: '/web/user/api/company/children', // 获取岗位信息列表结构 平台入口的调用这个
      getWorkItemList: '/web/plan/api/planItemTemplate/list', // 查询工作项
      getIndicatorTypeList: '/web/plan/api/workTargetType/list', // 查询指标类型接口
      saveWorkItemList: '/web/plan/api/planItemTemplate/save', // 保存工作项
      editWorkItemList: '/web/plan/api/planItemTemplate/update', // 编辑工作项
      deleteWorkItemList: '/web/plan/api/planItemTemplate/delete', // 删除工作项
      updatePersonInfoSort: '/web/plan/api/planItemTemplate/setSort', // 删除工作项
      uploadTemplate: '/web/plan/api/planItemTemplate/uploadTemplate',
      multiDutyUploadTemplate: '/web/plan/api/planItemTemplate/multiDutyUploadTemplate', // 导入wbs
      downloadTemplate: '/web/file/api/brm/findByParams' // wbs模板下载
    },
    // taskManage.myWork.getMyWorkTimeLine
    myWork: {
      // 我的全景图
      getMyWorkTimeLine: '/web/plan/api/planItemStep/getMyPlanItemStepList', // 获取我的全景图计划
      getMyWorkPlanDetail: '/web/plan/api/planItemStep/getPlanItemStepById', // 获取我的全景图计划详情
      startWork: '/web/plan/api/planItemStep/startPlanItemStep', // 我的全景图工作详情内点击开始工作
      postWorkDetail: '/web/plan/api/planItemStep/submitPlanItemStep', // 提交工作情况
      getPlanItemStepCountByFinishStatus: '/web/plan/api/planItemStep/getPlanItemStepCountByFinishStatus' // 提交工作情况
    },
    branchWork: {
      // 分管全景图
      findUserIdSubordinateList: '/web/user/api/user/findUserIdSubordinateList', // 根据用户id,项目部或平台类型,公司或项目部id 查询直系下属人员
      getBranchWorkPlanList:
        '/web/plan/api/planItemStep/getPlanItemStepListByUsers', // 获取分管全景图计划
      findByOrganizationList: '/web/user/api/user/findByOrganizationList', // 获取分管全景图组织架构
      findByOrganizationUserList:
        '/web/user/api/user/findByOrganizationUserList' // 分管全景图组织用户list
    },
    workList: {
      getMyAllWorkTarget: '/web/plan/api/workTarget/getMyAllWorkTarget', // 常规工作列表
      designRelationDrawing: '/web/plan/api/desginTypePlan/relationDrawing', // 设计分部分项批量上传图纸
      getPostCheckDrawingList:
        '/web/plan/api/desginTypePlan/getLastPlanListByPlanState', // 设计分部分项的获取提交审核的图纸列表
      getDrawingByCatalogueId:
        '/web/project/api/drawing/getDrawingByCatalogueId' // 查询图纸
    },
    workAudit: {
      // 工作审批
      getWorkAuditList:
        '/web/plan/api/planItemStep/getMyExaminePlanItemStepList' // 获取工作审批表格接口
    },
    // 任务模板
    taskTemplate: {
      planTemplateSave: '/web/plan/api/planTemplate/save',
      planTemplateList: '/web/plan/api/planTemplate/list',
      planTemplateUpdate: '/web/plan/api/planTemplate/update',
      planTemplateDetail: '/web/plan/api/planTemplate/findById',
      planTemplateDelete: '/web/plan/api/planTemplate/delete'
    },
    scheduleTask: {
      save: '/web/plan/api/preparatoryPlan/save',
      list: '/web/plan/api/preparatoryPlan/list',
      update: '/web/plan/api/preparatoryPlan/update',
      findById: '/web/plan/api/preparatoryPlan/findById',
      delete: '/web/plan/api/preparatoryPlan/delete'
    }
  },
  // 图纸文件接口
  drawings: {
    getCatalogueTree: '/web/project/api/catalogue/tree', // 获取目录树
    // saveCatalogue: '/catalogue/saveCatalogue', // 新增/编辑目录树
    saveCatalogue: '/web/project/api/catalogue/save', // 新增目录树
    editCatalogue: '/web/fictitious/file/catalogue/api/update', // 修改目录树
    delCatalogue: '/web/project/api/catalogue/delete', // 删除目录树

    getDrawingList: '/web/catalogue/file/api/list', // 根据目录id查询图纸列表
    saveDrawing: '/web/catalogue/file/api/save', // 上传、更新图纸
    updateDrawing: '/web/catalogue/file/api/update', // 更新图纸
    getDrawingVersionList: '/web/catalogue/file/api/getVersionList', // 查看图纸版本
    deleteDrawing: '/web/project/api/drawing/delete', // 删除图纸
    getDrawingByCatalogueId: '/web/project/api/drawing/getDrawingByCatalogueId', // 查询图纸
    batchUploadDrawing: '/web/project/api/drawingRelation/batchUploadDrawing' // 批量上传图纸
  },
  roleManage: {
    getRoleList: '/web/flowRoleApi/list', // 获取角色列表/web
    getRoleUserList: '/web/flowRoleUserApi/list', // 根据角色查询用户列表
    relateUser: '/web/flowRoleUserApi/relevance', // 角色关联用户
    deleteRole: '/web/flowRoleApi/delete', // 删除角色
    saveRole: '/web/flowRoleApi/save', // 创建角色
    updateRole: '/web/flowRoleApi/update', // 修改角色名称
    offRelateRole: '/web/flowRoleUserApi/makeOffline' // 用户解除关联角色
  },
  // 进度计划-流程审批
  schedule: {
    transpondFlow: '/web/flowInstanceApi/transpond', // 转发流程
    storageFormData: '/web/flowInstanceApi/storageFormData', // 暂存实例表单数据
    queryStorageFormData: '/web/flowInstanceApi/queryStorageFormData', // 查询暂存实例表单数据
    getPropertyData: '/web/bim/api/bim/findById', // 获取构件属性数据
    uploadWBS: '/web/plan/api/wbsTypePlan/uploadWBS', // 导入WBS
    setPlanDate: '/web/plan/api/wbsTypePlan/setPlanDate', // 上报进度
    getFlowInstanceList: '/web/flowInstanceApi/list', // 获取质量报验表格（运行的流程）
    findIdsList: '/web/measuring/quantities/api/findIdsList', // 报验量列表查询
    getFlowTemplateList: '/web/flowTemplateApi/list', // 查询流程模板列表
    getMainAndDeputyCompanyList: '/web/user/api/company/findCompanyListByUserId', // 查询主副岗下的所有公司
    getQualityCheckForm: 'buildPlan/findFormByName', // 获取报验表格关联的表单
    getFormDetail: '/web/formTemplateApi/findById', // 获取表单详情
    flowTemplateFindById: '/web/flowTemplateApi/findById', // 根据id查流程
    getFlowInstanceTemplateNode: '/web/flowProxy/findById', // 获取运行中的流程详情
    saveFlowInstance: '/web/flowInstanceApi/submit', // '表单流程填报'
    saveBizRelevance: '/web/flowInstanceApi/saveBizRelevance', // 保存Relevance
    saveFlowInstanceAgain: '/web/flowInstanceApi/reSubmit', // '重新发起表单流程填报'
    rollBackNode: '/web/flowInstanceApi/rollBackThePreviousLevel', // 回退上一审批节点
    handOver: '/web/flowInstanceApi/approverAppend', // 移交(添加当前节点审批人)
    approvalOpinionList: '/web/flowApprovalOpinionApi/list', // 审批人常用意见列表
    approvalOpinionSave: '/web/flowApprovalOpinionApi/save', // 新增审批人常用意见
    approvalOpinionDelete: '/web/flowApprovalOpinionApi/delete', // 删除审批人常用意见
    getSupervisor: '/web/user/api/user/findSuperiorLeader', // 获取发起人部门主管
    getDeptLeader: '/web/user/api/user/findGrandpaSuperiorLeader', // 获取发起人分管副总
    getRoleList: '/web/flowRoleApi/list', // 获取角色列表/web
    getRoleUserList: '/web/flowRoleUserApi/list', // 根据角色查询用户列表
    getFlowData: 'buildPlan/findAllFlow', // 查询流程
    postFromAndFlow: 'buildPlan/bindingFormAndFlow', // 绑定表单和流程
    relieveForm: '/web/flowInstanceApi/delete', // 表单解除关联
    getStepData: 'buildPlan/findFlowById', // 获取流程数据
    getFlowTemplate: 'buildPlan/getFlowTemplate', // 获取流程模板数据
    postEditForm: 'buildPlan/submitForm', // 提交编辑好的表单
    getFormData: 'formData/getFormData', // 获取表单数据
    getFormRoleData: 'roleData/getRoleData', // 获取表单权限
    findRelationDrawing: '/web/plan/api/wbsTypePlan/findRelationDrawing', // 关联图纸列表
    getAllDrawList: '/web/file/project/api/list', // 获取图纸列表
    relationDrawing: '/web/plan/api/wbsTypePlan/relationDrawing', // 关联图纸
    disRelationDrawing: 'buildPlan/disRelationDrawing', // 解除关联图表
    getAttachmentList: '/web/file/api/relationFile/findByRelationIdList', // 查询关联附件
    deleteAttachment: '/web/file/api/relationFile/deleteIdsList', // 删除关联附件
    saveAttachment: '/web/file/api/relationFile/save', // 提交关联附件
    saverelationUser: '/web/plan/api/wbsTypePlan/relationUser', // 设置责任人和相关方接口
    queryProjectListForWBS: '/web/project/api/queryProjectListForWBS', // 获取项目列表
    getBimComponentList: '/web/plan/api/wbsTypePlan/getComponentStatus', // 获取BIM构件着色列表
    flowTracking :'/web/flowInstanceApi/flowTracking', //跟踪事项操作
    queryProjectForPay :'/web/external/project/queryProjectForPay', //查询关联列表
  },
  // 设计进度
  designProgress: {
    wbsTypePlanList: '/web/plan/api/desginTypePlan/list', // 进度计划列表数据
    getMyWbsList: '/web/plan/api/desginTypePlan/getMyWbs', // wbs列表数据
    saveComponentUrl: '/web/plan/api/desginTypePlan/relationComponent', // 关联构件
    unhookComponent: '/web/plan/api/desginTypePlan/unhookComponent', // 解除关联构件
    uploadWBS: '/web/plan/api/desginTypePlan/uploadWBS', // 导入WBS
    saverelationUser: '/web/plan/api/desginTypePlan/relationUser', // 设置责任人和相关方接口
    getAttachmentList: '/web/file/api/relationFile/findByRelationIdList', // 查询关联附件
    deleteAttachment: '/web/file/api/relationFile/deleteIdsList', // 删除关联附件
    saveAttachment: '/web/file/api/relationFile/save', // 提交关联附件
    getAllDrawList: '/web/file/project/api/list', // 获取图纸列表
    queryProjectListForWBS: '/web/project/api/queryProjectListForWBS', // 获取项目列表
    getTopPlanList: '/web/plan/api/desginTypePlan/getTopPlanList', // wbs列表数据
    findByProjectIdList: 'proBim/findByProjectIdList', // 根据项目id获取模型数据
    getFormData: '/web/flowInstanceApi/getCurrentFromData', // 查询待办审批节点字段
    getTaskFormDetail: '/web/formProxy/findById', //  查询已建任务的表单模板
    getFlowTemplateList: '/web/flowTemplateApi/list', // 查询流程模板列表
    getFormDetail: '/web/formTemplateApi/findById', // 获取表单详情
    flowTemplateFindById: '/web/flowTemplateApi/findById', // 根据id查流程
    saveFlowInstance: '/web/flowInstanceApi/submit', // '表单流程填报'
    relieveForm: '/web/flowInstanceApi/delete', // 表单解除关联
    getFlowInstanceList: '/web/flowInstanceApi/list', // 获取质量报验表格
    setPlanDate: '/web/plan/api/desginTypePlan/setPlanDate', // 上报进度
    getStepData: 'buildPlan/findFlowById', // 获取流程数据
    saveUploadSuccessList: '/web/project/api/catalogue/saveList', // 获取流程数据
    deleteProjectPlanDrawing: '/web/project/api/drawingRelation/deleteProjectPlanDrawing' // 删除
  },
  // 开发进度
  developProgress: {
    developPlanList: '/web/plan/api/softwareTypePlan/list', // 开发计划列表数据
    developPlanDelete: '/web/plan/api/softwareTypePlan/delete', // 开发计划列表数据
    developPlanSave: '/web/plan/api/softwareTypePlan/save', // 创建计划
    getProcedureData: '/web/project/api/formalities/getFormalitiesByProjectId', // 手续文件列表数据
    judgeProcedureBindStatus:
      '/web/plan/api/targetTypePlan/getRelationStatusByBusinessIds', // 手续文件列表数据
    uploadWBS: '/web/plan/api/softwareTypePlan/uploadWBS', // 导入WBS
    setPlanDate: '/web/plan/api/softwareTypePlan/setPlanDate', // 上报进度
    saverelationUser: '/web/plan/api/softwareTypePlan/relationUser', // 设置责任人和相关方接口
    getPlanByBusinessIdAndFinishStatus: '/web/plan/api/softwareTypePlan/getPlanByBusinessIdAndFinishStatus' // 计划查看任务
  },
  // 工程量管理
  amountManage: {
    // 计量节点
    measuringList: '/web/measuring/quantities/listing/api/list', // 查询计量节点所有数据
    savemeasuringList: '/web/measuring/quantities/listing/api/save', // 填写提交保存报验数据
    // 我发起的
    myInitiate: 'init'
    // 需我审批

    // 我已审批

    // 发起报验工程量

    // 查看报验信息
  },
  // 项目数据
  ProjectData: {
    progressStatistics: 'buildPlan/progressStatistics', // 进度数据
    statisticsFormByFormType: 'buildPlan/statisticsFormByFormType' // 质量数据
  },
  // 质量管理
  qualityManage: {
    findList: '/web/flowJobTaskLink/list', //  待办，已办列表
    getQualityType: '/type/list', // 获取表单类型
    submitTask: '/flowInstanceApi/audit',
    findApprovePermission: '/web/flowProxy/getFlowNodePower', // 查询待办审批节点字段
    getFormData: '/web/flowInstanceApi/getCurrentFromData', // 查询待办审批节点字段
    getTaskFormDetail: '/web/formProxy/findById' //  查询已建任务的表单模板
  },
  // 平台架构信息>架构列表
  frameworkManageInfo: {
    getCompanyDepartTree: '/web/user/api/company/children', // 获取公司部门架构数据
    getCompanyPersonnelTree: '/web/user/api/user/childrenUserDuty', // 获取公司人员架构数据
    getProjectDepartList: '/web/project/api/proMainDept/list', // 获取底部项目部列表数据
    getProjectListByCompanyId:
      '/web/project/api/proMainDept/queryProjectListForMainDept', // 获取下拉选择相关项目数据
    getUserListByCompanyId:
      '/web/user/api/projectMainDept/getUserListByCompanyId', // 获取下拉选择项目经理数据
    modifyProjectDepart: '/web/project/api/proMainDept/save', // 创建项目提交
    deleteProject: '/web/project/api/proMainDept/delete', // 删除项目部
    downloadTemplate: '/web/user/api/duty/downloadTemplate' // 下载模板
  },
  // 项目部架构信息>架构列表
  departmentFrameworkPage: {
    getProjectMainDeptChildren: '/web/user/api/projectMainDept/children', // 获取项目部组织架构树
    getProjectMainDeptChildrenUserDuty:
      '/web/user/api/projectMainDept/childrenUserDuty', // 获取项目部人员架构树

    getRelatedCompanyList: '/web/project/api/relatedParty/getRelatedPartyList', // 获取相关方列表
    // 相关方信息
    getRelTreeData: '/web/project/api/relatedParty/getRelatedPartyTree', // 获取项目相关方数据
    handleNodeClick: '/web/user/api/user/findDepartmentIdList', // 点击相关方树获取个人信息
    // 更新相关方
    loadRelTreeList: '/web/project/api/relatedParty/getRelatedPartyList', // 获取更新页面相关方列表
    deleteRelevant: '/web/project/api/relatedParty/delRelatedParty', // 删除相关方
    hasInput: '/web/user/api/projectMainDept/queryCompanyListByName', // 搜索相关方
    addOrSaveRel: '/web/project/api/relatedParty/addRelatedParty', // 搜索后添加相关方
    updatePersonInfoSort: '/web/project/api/relatedParty/adjustRelatedPartySort', // 调整相关方顺序
    getUserStructure: '/web/user/api/user/getUserStructure'
  },
  // 平台和项目权限集
  frameworkManagePermission: {
    getResourceTree: '/web/user/api/clientelePlatform/findResources', // 获取平台授权的资源树（授权）
    // getResourceTree: '/web/user/api/resources/getResourceTree', // 获取平台所有权限的树(全部)
    addRight: '/web/user/api/roleTemplate/save', // 新增创建新的权限集
    rightsListChange: '/web/user/api/roleTemplate/findById', // 点击权限集角色列表查询对应勾选的权限
    getRoleTemplate: '/web/user/api/roleTemplate/list', // 获取权限集角色列表
    toEditRights: '/web/user/api/roleTemplate/update', // 权限集选择后更新
    handleEditOrDelete: '/web/user/api/roleTemplate/delete' // 修改或删除权限列表
  },

  // 架构信息
  frameworkInfo: {
    saveCompanyFrameworkModify: '/web/user/api/company/save', // 创建公司上下级组织,修改当前组织
    deleteCompanyFramework: '/web/user/api/company/delete', // 根据id删除公司
    getCompanyFrameworkData: '/web/user/api/company/children', // 查询公司部门架构
    getDepartFrameworkData: '/web/user/api/department/list', // 根据公司id查询整个树下所有部门（添加人员弹窗内使用）
    getUperDepartmentList: '/web/user/api/department/findCompanyIdList', // 根据部门的父级公司下所有部门（添加人员弹窗内使用）
    saveDuty: '/web/user/api/duty/save', // 新增岗位
    saveUser: '/web/user/api/user/save', // 新增用户
    deleteUserList: '/web/user/api/user/deleteList', // 批量删除人员
    deleteDutyList: '/web/user/api/duty/deleteList', // 批量删除岗位
    getSuperiorUserList:
      '/web/user/api/user/findByCompanyIdMainDepartmentUserDutyList', // 查询公司下领导部门人员列表

    saveDepartFrameworkModify: '/web/user/api/department/save', // 创建更新公司部门
    deleteDepartFramework: '/web/user/api/department/delete', // 根据id删除公司部门
    adjustmentDepartment: '/web/user/api/user/adjustmentDepartment', // 人员调整部门
    getRoleResourceTree: '/web/user/api/resources/getRoleResourceTree', // 平台获取人员权限

    updatePersonSort: '/web/user/api/user/updateUserDutySort', // 人员调整顺序
    updateDutySort: '/web/user/api/duty/updateDutySort', // 岗位调整顺序
    findUserDetail: '/web/user/api/user/findById', // 获取人员详细信息
    findDutyDetail: '/web/user/api/duty/findById', // 获取岗位详细信息
    getUserList: '/web/user/api/user/findDepartmentIdList', // 获取人员信息列表结构
    getDutyList: '/web/user/api/duty/getDepartmentIdList', // 获取岗位信息列表结构
    getParentCompanyList: '/web/user/api/company/getParentCompanyList', // 获取公司

    getResourceTree: '/web/user/api/clientelePlatform/findResources', // 获取权限资源树（授权）
    // getResourceTree: '/web/user/api/resources/getResourceTree', // 获取权限资源树（全部）--弃用

    getSystemUserPermission: '/web/user/api/role/save', // 平台用户分配权限
    getProjectUserPermission: '/web/user/api/role/save', // 项目部用户分配权限
    saveDeptFunction: '/web/user/api/departmentFunction/save', // 新增平台部门职能
    updateDeptFunction: '/web/user/api/departmentFunction/update', // 修改平台部门职能
    deleteDeptFunction: '/web/user/api/departmentFunction/delete', // 删除平台部门职能
    getDeptFuncList: '/web/user/api/departmentFunction/list', // 获取部门职能列表
    importUserList: '/web/user/api/user/excelAddList', // 批量导入人员
    // importDutyList: '/web/user/api/duty/excelAddList', // 旧批量导入岗位
    importDutyList: '/web/user/api/duty/importData', // 新批量导入岗位
    updateDeptFuncSort:
      '/web/user/api/departmentFunction/sortDepartmentFunctionList', // 更新职能排序

    getPermissionUnionList: '/web/user/api/roleTemplate/list', // 获取权限集下拉列表
    getRolePermissionUnion: '/web/user/api/roleTemplate/findById', // 获取权限集权限
    // 项目部架构
    departmentFramework: {
      // 架构设置
      frameSetting: {
        saveProDuty: '/web/user/api/duty/save', // 项目部新增岗位
        saveProUser: '/web/user/api/user/saveUserDutyList', // 项目部新增人员
        delProUser: '/web/user/api/user/saveUserDutyList', // 项目部批量删除人员
        delProDuty: '/web/user/api/duty/deleteList', // 项目部批量删除岗位
        getSuperiorUser: '/web/user/api/user/findMainDepartmentUserDutyList', // 项目部查询领导部门人员

        saveProDepartment: '/web/user/api/department/save', // 添加项目部部门
        editProDepartment: '/web/user/api/projectMainDept/save', // 编辑项目部部门
        deleteProDepartment: '/web/user/api/department/delete', // 删除项目部部门
        adjustProDept: '/web/user/api/user/adjustmentDepartment', // 项目部调整部门
        getRoleResourceTree: '/web/user/api/resources/getRoleResourceTree', // 项目部获取人员权限
        changProjectSelectList: '/web/user/api/projectMainDept/getMainDeptManagerList', // 修改项目经理选择下拉列表
        saveChangeProject: '/web/user/api/projectMainDept/changeMainDeptManager', // 提交修改项目经理接口

        adjustProUserSort: '/web/user/api/user/updateUserDutySort', // 项目部人员调整顺序
        adjustProDutySort: '/web/user/api/duty/updateDutySort', // 项目部岗位调整顺序
        getProUserDetail: '/web/user/api/user/findById', // 项目部获取人员详情
        getProDutyDetail: '/web/user/api/duty/findById', // 项目部获取岗位详情
        getProDeptUserList: '/web/user/api/user/findDepartmentIdList', // 查询项目部部门下人员列表
        getProDeptDutyList: '/web/user/api/duty/getDepartmentIdList', // 查询项目部部门下岗位列表

        getProDeptList: '/web/user/api/projectMainDept/children', // 查询项目部和部门结构树
        // getProDeptList: '/web/user/api/department/findCompanyIdList',  // 查询项目部和部门结构树
        // getProDeptList: '/proDepartment/getDeptVosByProMainDeptId', // 查询项目部和部门结构树
        getProCompanyUser: '/web/user/api/department/queryUserListForDept' // 查询项目部关联的公司下的所有人员
      },
      // 审批流程
      flow: {
        getProjectFlowList: '/web/flowTemplateApi/list', // 项目部审批流程列表
        getFlowGroupList: '/web/flowGroup/list', // 获取流程分组列表
        saveFlowGroup: '/web/flowGroup/save', // 新建分组
        updateFlowGroupList: '/web/flowGroup/update', // 修改分组
        relevanceGroup: '/web/flowTemplateApi/relevanceGroup', // 移动分组
        getCompanyListByProjectId: '/web/project/api/queryCompanyListByProjectId', // 流程设计获取流程权限
        directInitiation: '/web/flowInstanceApi/directInitiation', // 定义流程并直接发起
        saveCustomFlowTemplate: '/web/flowTemplateApi/createCustomFlowTemplate', // 保存自定义流程
        editCustomFlowTemplate: '/web/flowTemplateApi/editCustomFlowTemplate', // 更新提交自定义流程
        getFlowTemplateDetail: '/web/flowTemplateApi/findById', // 获取自定义流程详情
        getMainDeptUserListById:
          '/web/user/api/projectMainDept/getMainDeptUserListById', // 获取项目部人员
        findRelevantFlowNode: '/flow/findRelevantFlowNode', // 获取单个流程列表流程节点
        designatedPersonnel: '/web/flowNodeAuditConfig/designatedPersonnel', // 指定人员提交接口
        typeList: '/type/list', // 查询类型集合
        standardList: 'standard/list', // 查询标准集合
        getFormTemplateList: '/web/formTemplateApi/list', // 模板表单列表
        findAllByFormTemplateIds:
          '/web/flowTemplateApi/findAllByFormTemplateIds', // 根据表单id获取流程列表
        saveFlowforTemplate:
          '/web/flowTemplateApi/pullOpsFlowTemplateCreateBatch', // 保存模板库流程
        flowTemplateDelete: '/web/flowTemplateApi/delete', // 删除流程
        findRelevanceData: '/web/flowTemplateApi/findRelevanceData', // 查询指定人员
        operationFlowTemplate: '/flowTemplateApi/operation', // 启用停用流程
        withDraw: '/web/flowInstanceApi/revocation', // 撤销任务
        flowGroupDelete: '/web/flowGroup/delete' // 删除流程分组
      }
    },
    sortDept: '/web/user/api/department/sortDept', // 同级部门拖拽排序
    sortCompany: '/web/user/api/company/sortCompany' // 同级公司拖拽排序
  },
  // 表单库
  formLibrary: {
    formList: '/web/formTemplateApi/list', // 表单列表
    getFormDetail: '/web/formTemplateApi/findById' // 获取表单详情
  },
  // 流程回调配置
  flowCallBackConfig: {
    // saveInterFace: '/web/requestConfig/save', // 新增回调接口
    getCallBackList: '/web/requestConfig/list', // 查询回调列表
    // deleteCallBackInstance: '/web/requestConfig/delete', // 删除回调
    saveTriggerConfig: '/web/requestTriggerConfig/save', // 保存回调的触发配置
    // getConfigList: '/web/requestTriggerConfig/list', // 查询配置列表
    // deleteConfigInstance: '/web/requestTriggerConfig/delete', // 删除配置
    configRelateCallback: '/web/requestTriggerConfig/binding/request', // 配置绑定回调
    // getFlowListData: '/web/flowTemplateApi/list', // 查询流程列表
    saveFlowConfigRelation: '/web/flowTriggerConfigRelevance/save' // 保存流程配置关联信息
    // getFlowConfigList: 'web/flowTriggerConfigRelevance/list', // 查询流程配置列表
    // deleteFlowConfigInstance: 'web/flowTriggerConfigRelevance/delete', // 删除流程配置
    // getHistoryListData: '/web/requestConfig/history/list', // 查询历史数据
    // getProjectVosByCompanyId: '/web/project/api/getProjectVosByCompanyId'
  },
  /**
   * 迁移至外部的流程管理
   */
  flowManage: {
    getCompanyList: '/web/user/api/company/list', // 查询流程节点公司列表
    getFlowListData: '/web/flowTemplateApi/list', // 查询流程列表
    getFlowGroupList: '/web/flowGroup/list', // 获取流程分组列表
    saveFlowGroup: '/web/flowGroup/save', // 新建分组
    updateFlowGroupList: '/web/flowGroup/update' // 修改分组
  },
  // 岗位信息
  postInfo: {
    savePostMessage: '/web/user/api/duty/save', // 新建更新岗位
    dutyLevel: '/web/user/api/dutyLevel/list' // 查询职级列表
  },
  // 权限集
  permissionTml: {},
  // 流程
  customFlow: {
    updateTemplate: '/template/update', // 保存模板,
    getTemplate: '/template/get', // 获取模板,
    findByCompanyIdUserList: '/web/user/api/user/findByCompanyIdUserList' // 根据公司id查询人员列表,
  },
  backlogManage: {
    getTookDutyMainDeptAndProject:
      '/web/project/api/proMainDept/getTookDutyMainDeptAndProject',
    getPlanItemStepByFinishStatus:
      '/web/plan/api/planItemStep/getPlanItemStepByFinishStatus'
  },
  // 绩效考核
  performance: {
    getReserveDetail: '/web/plan/api/reserveKpiGroup/findById', // 轮岗月度绩效详情列
    getReserveList: '/web/plan/api/reserveKpiGroup/list', // 轮岗月度绩效列表
    indicatorsTypeList: '/web/plan/api/indicatorsType/list',
    indicatorsTypeSave: '/web/plan/api/indicatorsType/save',
    indicatorsTypeDetail: '/web/plan/api/indicatorsType/findById',
    indicatorsTypeUpdate: '/web/plan/api/indicatorsType/update',
    indicatorsTypeDelete: '/web/plan/api/indicatorsType/delete',
    findDepartmentId: '/web/user/api/department/findById',
    postKpiGroup: '/web/plan/api/kpiGroup/save',
    updateKpiGroup: '/web/plan/api/kpiGroup/update',
    getKpiGroupList: '/web/plan/api/kpiGroup/list',
    getWorkTargetDetail: '/web/plan/api/kpiGroup/findById',
    getWorkTargetDetail2: '/web/plan/api/kpiGroup/findByIdIgnoreEnableType',
    getBeforeKpiGroup: '/web/plan/api/kpiGroup/getBeforeKpiGroup', // 导入查询最近一次数据
    deleteWorkTarget: '/web/plan/api/kpiGroup/delete',
    getPerfVal: '/web/plan/api/kpiGroup/calculateScore', // 计算岗位绩效
    downloadTemplate: '/web/file/api/brm/findByParams', // 下载模板
    getKpiListByTargetTime: '/web/plan/api/kpi/getKpiListByTargetTime',
    getFinishedMonthKpiByKpiId: '/web/plan/api/kpi/getFinishedMonthKpiByKpiId',
    getSubordinateUser: '/web/user/api/user/getSubordinateUser',
    getPlanByUser: '/web/plan/api/targetTypePlan/getPlanByUser',
    getKpiRelationPlan: '/web/plan/api/targetTypePlan/getKpiRelationPlan',
    kpiSummarySave: 'web/plan/api/kpiSummary/save',
    kpiSummaryUpdate: 'web/plan/api/kpiSummary/update',
    kpiSummaryList: '/web/plan/api/kpiSummary/list',
    KpiSummaryUserSortSetList: '/web/plan/api/KpiSummaryUserSortSet/list',
    KpiSummaryUserSortSetUpdate: '/web/plan/api/KpiSummaryUserSortSet/update',
    getKpiGroupByTargetTime: 'web/plan/api/kpiGroup/getKpiGroupByTargetTime',
    getLastKpiSummary: 'web/plan/api/kpiSummary/getLastKpiSummary',
    calculateKpi: 'web/plan/api/kpiGroup/calculateKpi',
    findById: 'web/plan/api/kpiSummary/findById',
    saveYearScore: '/web/plan/api/kpi/saveYearScore',
    findKpiByYearScoreId: '/web/plan/api/kpiGroup/findKpiByYearScoreId'
  },
  // 流程审批相关
  approveManage: {
    getFlowNodeProxyConfigUserList:
      'web/flowNodeProxy/getFlowNodeProxyConfigUserList',
    findRecord: '/web/flowAuditRecord/list', // 查询流程日志
    getTaskList: '/web/flowJobTaskLink/list', // 查询已办待办待发已发
    taskFlowDelete: '/web/flowInstanceApi/delete', // 待发任务删除
    savePostScript: '/web/flowInstancePostscript/save', // 新增附言
    getPostScriptList: '/web/flowInstancePostscript/list', // 获取附言列表
    retrieveProcess: '/web/flowInstanceApi/retrieveProcess', // 流程取回
    queryCurrentProcessor: 'web/flowNodeAuditConfigProxy/queryCurrentProcessor', // 获取流程各节点的信息
    trackingList: '/web/flowInstanceApi/tracking/list', // 跟踪事项查询
    updateFlowProxy: '/web/flowInstanceApi/updateFlowProxy', // 流程加签保存功能
    findAllAuditConfig: '/web/flowInstanceApi/findAllAuditConfig', // 查看流程节点审批范围人员
  },
  // 手续文件项目部
  projectDept: {
    getFormalities: '/web/project/api/formalities/getFormalitiesByProjectId',
    saveFormalities: '/web/project/api/formalities/saveFormalities',
    // getTempList: '/web/project/api/formalities/enableList', // 获取模板列表
    getTempList: '/web/project/api/formalities/list', // 获取模板列表
    withoutTemplate: '/web/project/api/formalities/disUsedTemplate', // 不使用模板
    detail: '/web/project/api/formalities/findById', // 模板详情
    updateFormalities: '/web/project/api/formalities/updateFormalities',
    getFileList: '/web/project/api/doc/type/enableList', // 获取文件列表筛选
    getFileListNofilter: '/web/project/api/doc/type/list', // 获取文件列表不筛选
    getProjStatus: '/web/plan/api/targetTypePlan/getPlanByBusinessIds',
    getRelatedFormalities: '/web/project/api/formalities/getRelatedFormalities', // 查询相关方手续
    saveRelatedFormalities: '/web/project/api/formalities/saveRelatedFormalities', // 保存相关方手续
    getProcedureDetail: '/web/project/api/formalities/getFormalitiesDocDetailById', // 根据业务id查询手续详情
    updateGroupFormalities: '/web/project/api/formalities/updateGroup', // 更新分组
    changeGroup: '/web/project/api/formalities/changeGroup', // 修改分组
    deleteGroup: '/web/project/api/formalities/delGroup', // 删除分组
    getProupList: '/web/project/api/formalities/groupList' // 获取分组列表
  },
  // BIM管理
  BimManage: {
    getComponentIds: '/web/bim/api/bimModel/getComponentIds', // 小程序 着色id
    uploadBIMModel: '/web/bim/api/bimModel/bimSave', // BIM模型上传
    update: '/web/bim/api/bimModel/update', // BIM模型及数据更新
    delete: '/web/bim/api/bimModel/delete', // BIM模型及数据删除
    bimFileSave: '/web/bim/api/bimModelFile/bimFileSave', // 模型更新
    bimFiledelete: '/web/bim/api/bimModelFile/delete', // BIM模型历史数据删除
    batchDownFile: '/web/bim/api/bimModelFile/batchDownFile', // 模型下载
    batchGetViewToken: '/web/bim/api/bimModelFile/batchGetViewToken', // 获取viewToken
    getElements: '/web/bim/api/bimModelFile/getElements', // 根据条件筛选符合的id
    batchElements: '/web/bim/api/bimModelFile/batchElements', // 批量获取构件属性
    findByType: '/web/bim/api/modelClassify/findByType', // 模型分类列表查询
    typeList: '/web/bim/api/modelClassify/list', // 模型分类列表查询(分页)
    saveType: '/web/bim/api/modelClassify/save', // 模型分类新增
    updateType: '/web/bim/api/modelClassify/update', // 模型分类修改
    deleteType: '/web/bim/api/modelClassify/delete', // 模型分类删除
    getBimList: '/web/bim/api/bimModel/findByNameList', // 模型列表查询
    bimModelVersion: '/web/bim/api/bimModelFile/save', // 模型版本新增
    combinationBim: '/web/bim/api/combination/save', // 组合模型新增
    uploadModelClassify: '/web/bim/api/modelClassify/save', // 分类新增
    getModelClassify: '/web/bim/api/modelClassify/findByType', // 分类查询（模型列表查询）
    getUserId: '/web/project/api/proMainDept/getMainDeptTree', // 修改人查询
    bimModelVersionList: '/web/bim/api/bimModelFile/list', // 查询版本
    CompositeList: '/web/bim/api/combination/list', // 组合模型列表获取
    CompositeSave: '/web/bim/api/combination/save', // 组合模型新增
    CompositeDelete: '/web/bim/api/combination/delete', // 组合模型删除
    batchUpdate: '/web/bim/api/bimModel/batchUpdate', // 修改分类
    getChoiceSetList: '/web/bim/api/choiceSet/findByNameList', // 选择集列表
    saveChoiceSet: '/web/bim/api/choiceSet/save', // 选择集新增
    deleteChoiceSet: '/web/bim/api/choiceSet/delete', // 选择集删除
    viewpointList: '/web/bim/api/viewpoint/list', // 视点列表
    modelClassify: '/web/bim/api/modelClassify/findByType', // 视点分类下拉数据
    viewpointSave: '/web/bim/api/viewpoint/save', // 视点新增
    viewpointDelete: '/web/bim/api/viewpoint/delete', // 视点删除
    viewpointUpdate: '/web/bim/api/viewpoint/update', // 视点删除
    choiceList: '/web/bim/api/choice/listTree', // 选择集分类查询
    choiceSave: '/web/bim/api/choice/save', // 选择集分类新增节点
    choiceUpdate: '/web/bim/api/choice/save', // 选择集分类修改节点
    choiceDelete: '/web/bim/api/choice/delete', // 选择集分类刪除节点
    viewportList: '/web/bim/api/modelClassify/list', // 视点列表查询
    viewportSave: '/web/bim/api/modelClassify/save', // 视点列表新增
    viewportUpdate: '/web/bim/api/modelClassify/update', // 视点列表修改
    viewportDelete: '/web/bim/api/modelClassify/delete', // 视点列表删除
    choiceSetFileList: '/web/bim/api/choiceSetFile/list', // 选择集资料文件列表查询
    choiceSetStructureList: '/web/bim/api/choiceSetStructure/list', // 选择集构件列表查询
    choiceSetFileDelete: '/web/bim/api/choiceSetFile/delete', // 选择集资料列表文件删除
    choiceSetFileSave: '/web/bim/api/choiceSetFile/saveAll', // 选择集文件上传保存
    findByChoiceId: '/web/bim/api/choiceSetModel/findByChoiceId', // 选择集关联模型
    findByViewpointId: '/web/bim/api/viewpoint/findByViewpointId', // 视点关联模型
    menuSetList: '/web/user/menuSet/list', // 模型分配list
    saveExpansion: '/web/user/menuSet/saveExpansion', // 模型分配更新
    findByResourceId: '/web/user/menuSet/findByResourceId', // 根据资源查模型
    findByExpansionId: '/web/user/menuSet/findByExpansionId', // 是否模型绑定项目
    updateComponentId: '/web/bim/api/bimModel/updateComponentId'
  },
  annualBudget: {
    getProjectVosByCompanyId: '/web/project/api/getProjectVosByCompanyId', // 关联的项目
    costBudgetSave: 'web/measuring/api/costBudget/save', // 保存预算
    costBudgetSaveTask: 'web/measuring/api/costBudget/saveTask', // 保存预算--包含项目预算
    budgetDelete: 'web/measuring/api/costBudget/delete', // 删除预算
    budgetList: 'web/measuring/api/costBudget/list', // 预算列表
    appendCostBudgetSave: 'web/measuring/api/appendCostBudget/save', // 追加保存
    costBudgetUpdate: 'web/measuring/api/appendCostBudget/update', // 追加修改后提交
    initBudgetUpdate: 'web/measuring/api/costBudget/update', // 初始修改后提交
    getDepartByCompanyId: 'web/user/api/company/getCompanyDeptVoByCompanyId', // 通过公司获取公司部门
    budgetAdjust: '/web/measuring/api/budgetAdjust/save', // 保存金额调剂
    budgetAdjustUpdate: '/web/measuring/api/budgetAdjust/update', // 编辑金额调剂,审核不通过时编辑重新提交
    getBudgetAdjustDetail: '/web/measuring/api/costBudget/pageBaseId', // 金额调剂回显
    findBudgetById: '/web/measuring/api/costBudget/pageBasefindById', // 根据业务id获取整个预算数据
    getCompanyListOfOnDuty: '/web/user/api/company/getCompanyListOfOnDuty', // 获取当前登录人所在公司
    budgetDetailsList: '/web/measuring/api/budgetDetails/list', // 删除之前调用
    expenseLedgerList: '/web/measuring/api/expenseLedger/list'
  },
  // 合同管理（公司空间）
  contractManage: {
    typeSet: { // 类型设置
      getTreeList: '/web/api/measuring/contract/type/treeList', // 获取合同分类树
      getEnableTreeList: '/web/api/measuring/contract/type/enableTreeList', // 获取已启用的合同分类树
      typeSave: '/web/api/measuring/contract/type/save', // 合同分类新增
      typeEdit: '/web/api/measuring/contract/type/update', // 合同分类编辑
      deleteTree: '/web/api/measuring/contract/type/delete', // 合同分类单个删除
      batchDeleteTree: '/web/api/measuring/contract/type/deleteBatch'// 合同分类批量删除
    },
    template: { // 合同模板
      getTemplateList: '/web/measuring/api/contractTemplate/list', // 查询模板
      updateTemplate: '/web/measuring/api/contractTemplate/update', // 编辑模板
      saveTemplate: '/web/measuring/api/contractTemplate/save', // 新增模板
      deleteTemplate: '/web/measuring/api/contractTemplate/delete', // 删除模板
      getBrmList: '/web/file/api/brm/findByParams', // 从saas平台的模板管理里面拿模板数据
      copyFilesByRelationId: '/web/file/api/relationFile/copyFilesByRelationId' // 根据业务id拷贝文件
    },
    contractInfo: { // 合同信息
      getContractDetail: '/web/measuring/api/contractReview/findById', // 获取合同详情
      getContractList: '/web/measuring/api/contractReview/queryList', // 查询合同列表
      saveContractInfo: '/web/measuring/api/contractReview/increase', // 添加合同信息
      modifyContract: '/web/measuring/api/contractReview/modify', // 修改合同
      deleteContract: '/web/measuring/api/contractReview/delete', // 删除合同
      getLegalContractLogList: '/web/contractReviewLog/list', // 查询合同合规性审查日志记录列表
      saveLegalContractLog: '/web/contractReviewLog/save', // 保存合同合规性审查日志记录
      updateLegalContractLog: '/web/contractReviewLog/update', // 更新合同合规性审查日志记录
      saveContractRefFile: '/web/measuring/api/contractReview/saveContractRefFile', // 合同合规性审查表中的合规手续文件保存
      getContractLedgerList: '/web/measuring/api/contractReview/getContractLedgerList', // 合同台账列表
      getRelatedPartyByUser: '/web/user/api/company/queryCompanyListByNameForRelatedParty', // 获取客户下相关方列表
      getFileByContractId: '/web/measuring/api/contractReview/getRefListById', // 根据合同id查文件类相关
      getAllUserGroup: '/web/user/api/user/getPageUserVoListOfGroup', // 获取集团所有公司人员列表
      exportContractLedger: '/web/measuring/api/contractReview/exportContractLedger', // 导出合同台账
    },
    receiptAndPayContract: {
      getInvoicingList: '/web/measuring/api/contractInvoicing/list', // 查询合同开票列表（根据合同id查询）
      getPaymentList: '/web/measuring/api/contractPaymentForm/list', // 查询合同付款列表（根据合同id查询）
      getCollectionList: '/web/measuring/api/contractCollection/list', // 查询合同收款列表（根据合同id查询）
      getReceivedInvoiceDetailList: '/web/measuring/api/contractReview/invoicingDetailList', // 查询收票明细列表
      exportLegalList: '/web/measuring/api/contractReview/exportToResponse', // 合规审查记录表导出
      exportToCollection: '/web/measuring/api/contractReview/exportToCollection', // 收款管理导出
      exportToPayment: '/web/measuring/api/contractReview/exportToPayment', // 付款管理导出
    }
  },
  // 企划中心后台管理
  enterpriseManage: {
    carousel: {
      saveCarousel: '/web/file/loop/picture/save', // 新增轮播图
      updateCarousel: '/web/file/loop/picture/update', // 更新轮播图
      carouselList: '/web/file/loop/picture/list', // 轮播图列表
      deleteCarousel: '/web/file/loop/picture/delete' // 删除轮播图
    },
    notice: {
      saveNotice: '/web/information/api/staffNotice/save', // 新增通知公告
      updateNotice: '/web/information/api/staffNotice/update', // 更新通知公告
      noticeList: '/web/information/api/staffNotice/list', // 通知公告列表
      checkNoticeById: '/web/information/api/staffNotice/findById', // 根据id查询通知公告
      clickNumber: '/web/information/api/staffNotice/findByClickNumber', // 添加查看次数
      deleteNotice: '/web/information/api/staffNotice/delete', // 添加查看次数
      webReleaseFindById : '/web/information/api/webRelease/findById'
    }
  },
  budgetManage: {
    getBudgetCentralizedOfGroup: '/web/api/measuring/budget/type/budget/centralized/getBudgetCentralizedOfGroup',
    getCompanyDeptVoByCompanyId: '/web/user/api/company/getCompanyDeptVoByCompanyId', // 获取公司下的部门列表
    getexpendList: '/web/expenseReimbursement/budget/list', // 获取费用报销列表
    costTypeList: '/web/measuring/api/costType/list', // 获取费用类型列表
    verifyName :'/web/expenseReimbursement/verifyName', //检查报销单位
    getCostTypeList: '/web/measuring/api/budgetType/listAll', // 费用类型列表
    getCostTypeById: '/web/measuring/api/budgetType/findById', // 通过id查询归口详情
    getEchDetailData: '/web/expenseReimbursement/findById', // 获取表单详情数据
    getBudgetList: '/web/measuring/api/budgetType/list', // 预算列表
    getParentCompanyList: '/web/user/api/company/getParentCompanyList', // 获取公司列表
    budgetTypeSave: '/web/measuring/api/budgetType/save', // 新增预算类型
    budgetTypeUpdate: '/web/measuring/api/budgetType/update', // 修改预算类型
    budgetTypeDelete: '/web/measuring/api/budgetType/delete', // 删除预算类型
    expenseReimbursementSave: '/web/expenseReimbursement/submit', // 预算提审前保存
    expenseReimbursementDelete: '/web/expenseReimbursement/delete', // 报销删除
    saveBatchFile: '/web/file/api/relationFile/saveBatch', // 批量文件id绑定业务id
    deleteBatchFile: '/web/file/api/relationFile/deleteByRelationIdAndFileIds', // 批量文件id解绑业务id
    getTookDutyMainDeptAndProject: '/web/project/api/proMainDept/getTookDutyMainDeptAndProject', // 获取某人所有关联项目
    saveCompanyNumber: '/web/companyExpenseBaseData/saveAll', // 保存公司序号
    getCompanyNumberList: '/web/companyExpenseBaseData/list', // 保存公司序号
    listByBudgetId: '/web/measuring/api/budgetDetails/listByBudgetId', // 使用详情列表
    detailedMoney: '/web/expenseReimbursement/detailedMoney', // 根据ids查询请借款未还已还金额
    projectCompany: '/web/user/api/projectCompany/list', // 获取项目公司
    loanMoney: '/web/expenseReimbursement/loanMoney' // 根据ids查询请借款未还已还金额
  },
  // 项目进度
  projectSchedule: {
    getSuperViseScheduleList: '/flowInstanceApi/list', // 查询监理进度列表数据
    getStatisticData: '/flowInstanceApi/statistical', // 查询监理进度统计数据
    getDesignList: '/web/project/api/design/list', // 查询设计进度列表数据
    getDesignSave: '/web/project/api/design/save', // 设计进度详情增
    getDesignFindById: '/web/project/api/design/findById', // 设计进度详情
    getDesignUpdate: '/web/project/api/design/update', // 设计进度修改
    getDesignDelete: '/web/project/api/design/delete', // 设计进度删除
    getProcurementSave: '/web/project/api/procurement/save', // 招标采购进度弹窗新增
    getProcurementList: '/web/project/api/procurement/list', // 招标采购进度弹窗列表
    getProcurementFindById: '/web/project/api/procurement/findById', // 招标采购进度弹窗详情
    getProcurementUpdate: '/web/project/api/procurement/update', // 招标采购进度弹窗修改
    getProcurementDelete: '/web/project/api/procurement/delete', // 招标采购进度弹窗删除
    getProcurementDataCircle: '/web/project/api/procurement/dataCircle', // 招标采购echar图数据
    getProcurementDataIndex: '/web/project/api/procurement/dataIndex', // 招标采购左侧板块数据
    getProcurementDataClassify: '/web/project/api/procurement/dataClassify', // 招标采购右侧列表数据
    getContractSave: '/web/project/api/contract/save', // 合同进度新增
    getContractList: '/web/project/api/contract/list', // 合同进度列表
    getContractFindById: '/web/project/api/contract/findById', // 合同进度详情
    getContractUpdate: '/web/project/api/contract/update', // 合同进度更新
    getContractDelete: '/web/project/api/contract/delete', // 合同进度删除
    getContractDataIndex: '/web/project/api/contract/contractDataIndex', // 获取合同左侧面板数据
    getContractDataCircle: '/web/project/api/contract/contractDataCircle', // 获取合同中间图表数据
    getContractDataClassify: '/web/project/api/contract/contractDataClassify' // 获取合同右侧表格数据
  },
  // 简历招聘
  recruit: {
    list: '/web/resume/api/resume/list', // 获取简历库列表
    inviteInterview: '/web/resume/api/resume/inviteInterview', // 发送面试邀请
    reInviteInterview: '/web/resume/api/resume/reInvite', // 重新发送面试邀请
    setInterviewResults: '/web/resume/api/resume/setInterviewResults', // 设置面试结果
    queryByCode: '/web/resume/api/resume/queryByCode', // 根据邀请码或ID查询面试人员信息
    sendOffer: '/web/resume/api/resume/sendOffer', // 发放offer
    validateCode: '/web/resume/api/resume/validateCode', // 校验登记表单链接是否可用
    getCode: '/web/resume/api/resume/getCode', // 生成新的邀请码
    submitInviewForm: '/web/resume/api/resume/submitForm', // 提交面试登记表单
    getResumeLog: '/web/resume/api/resume/getInterviewResultLog', // 查询简历流程(查看日志)
    setEntryResults: '/web/resume/api/resume/setEntryResults', // 设置入职结果
    getNoticeList: '/web/notificationBiz/list', // 查询通知业务表
    getResumeStatistics: '/web/resumeStatistics/list', // 查询简历统计表列表
    // 下面的接口可能不用了
    getCompanyList: '/web/user/api/company/getSelfAndChildrenCompanyList', // 获取子公司列表
    fileUpload: '/web/resume/api/resume/fileUpload', // 导入简历文档
    cover: '/web/resume/api/resume/cover', // 覆盖重复的简历接口
    rollback: '/web/resume/api/resume/rollback', // 退回
    statusSubmit: '/web/resume/api/resume/statusSubmit', // 设置面试结果（已通过面试）
    updateInfo: '/web/resume/api/resume/updateInfo', // 更新时间
    del: '/web/resume/api/resume/del', // 删除简历
    getStatistic: '/web/resume/api/resume/report', // 查询统计数据
    updateAppointDate: '/web/resume/api/resume/updateAppointDate', // 更新时间
    importExcel: '/web/resume/api/resume/importExcel', // 初步面试导入表格
    saveBatchInviteInterview: '/web/resume/api/resume/saveBatchInviteInterview', // 初步面试-导入表格后保存面试人员信息
    importStaffRecordExcel: '/web/resume/api/resume/importStaffRecordExcel' // 员工档案导入表格
  },
  // 文件管理
  filesManage: {
    getFolderTreeList: '/web/file/api/file/getFolderTreeList', // 获取文件夹目录树
    getFoldersAndFiles: '/web/file/api/file/getFoldersAndFiles', // 查询文件夹下文件夹及文件
    getParentFolder: '/web/file/api/file/getFolderUrl', // 查询文件父系
    getCompanyListOfOnDuty: '/web/user/api/company/getCompanyListOfOnDuty', // 查询用户担任的主、副岗的公司列表
    saveFolder: '/web/file/api/file/saveFolder', // 新建文件夹
    checkPassword: '/web/user/api/login/user/checkPassword', // 验证密码
    updateFile: '/web/file/api/file/editFile', // 重命名文件
    deleteFileData: '/web/file/api/file/deleteFileData', // 删除文件
    deleteFile: '/web/file/api/file/deleteFile', // 删除文件2
    getFoldersSizeByIds: '/web/file/api/file/getFoldersSizeByIds', // 获取文件夹大小
    checkUploadStatus: '/web/file/api/file/checkUploadStatus', // 检查上传状态
    uploadChunk: '/web/file/api/file/uploadChunk', // 上传文件切片
    mergeChunks: '/web/file/api/file/mergeFile' // 合并文件切片

  },
  // 斯能数据看板
  snDataBoard: {
    yearTargetStatistic: '/web/target/statistics', // 本年目标统计
    getFinanceProjList: '/web/measuring/finance/data/getFinanceDataList', // 获取财务数据创建的项目
    // 公告
    announcement: {
      getAnnounceMentList: '/web/announcement/list', // 获取列表
      saveAnnouncement: '/web/announcement/save', // 保存公告
      updateAnnouncement: '/web/announcement/update', // 更新编辑公告
      deleteAnnouncement: '/web/announcement/delete' // 删除公告
    },
    completeStatus: {
      getCompanyTree: '/web/user/api/company/getCompanyTree',
      getProjectVosOfCompanyAndGroup: '/web/project/api/getProjectVosOfCompanyAndGroup',
      // 以上两个接口组合查询当前登录人所在公司的项目

      save: '/web/target/project/save', // 保存完成情况
      list: '/web/target/project/list', // 完成情况列表
      update: '/web/target/project/update', // 修改完成情况
      delete: '/web/target/project/delete' // 删除完成情况
    }
  },
  /**
   * 前期模块-投资模型/风机选址/数据录入
   */
  earlyInvest: {
    ecnomicAssess: {
      getExcelSheetData: '/web/windPowerEconomyEvaluationModel/viewExcel', // 查询投资模型excel表数据
      generateExcel: '/web/windPowerEconomyEvaluationModel/generateReport', // 创建经济评价数据
      getProjectAssessList: '/web/windPowerEconomyEvaluationModel/list', // 经济评价-列表
      deleteProjectAssessList: '/web/windPowerEconomyEvaluationModel/delete', // 经济评价-列表
      generate: '/web/windPowerEconomyEvaluationModel/generate',
      outPut: '/web/windPowerEconomyEvaluationModel/exportListData', // 导出
      modelBaseList: '/web/modelBase/list', // 模型库列表
      saveModel: '/web/modelBase/save', // 模型库保存
      deleteModel: '/web/modelBase/delete', // 模型库保存
      exportExcelData: '/web/flowInstanceApi/exportExcelData', // 导出流程数据
      generateNewFile: '/web/windPowerEconomyEvaluationModel/generateNewFile', // 预览生成投资经济评价，未保存
      generateByFile: '/web/windPowerEconomyEvaluationModel/generateByFile', // 保存生成投资经济评价
      relationProject: '/web/windPowerEconomyEvaluationModel/relationProject', // 符合测算关联项目
      saveFile: '/web/windPowerEconomyEvaluationModel/save' // 调用onlyoffice强制保存
    },
    dataEnter: {
      findProvince: '/web/areaApi/area/findProvince', // 省
      findCity: '/web/areaApi/area/findCity', // 市
      findArea: '/web/areaApi/area/findArea', // 区
      getProjectOfSN: '/web/project/api/getProjectVosOfSN', // 根据用户身份获取平台的项目
      getBasicDataList: '/web/windFieldBaseData/list', // 获取基础数据列表
      saveBasicData: '/web/windFieldBaseData/save', // 保存创建基础数据
      updateBasicData: '/web/windFieldBaseData/update', // 更新编辑基础数据
      deleteBasicData: '/web/windFieldBaseData/delete', // 删除基础数据
      saveTowerData: '/web/anemometerTowerData/saveAll', // 保存创建测风塔数据
      updateTowerData: '/web/anemometerTowerData/update', // 保存创建测风塔数据
      uploadTowerData: '/web/anemometerTowerData/import', // 导入测风塔数据
      deleteTowerData: '/web/anemometerTowerData/delete', // 删除测风塔数据
      getTowerDataList: '/web/anemometerTowerData/listGroup', // 获取测风塔数据列表
      saveTowerEnergyData: '/web/anemometerTowerData/saveDetail', // 保存测风塔风向风能数据
      getFanData: '/web/fanPointData/list', // 获取风机点位列表
      getFanDataById: '/web/fanPointData/findByProjectId', // 获取项目风机点位详情
      saveFanData: '/web/fanPointData/save', // 保存风机点位数据
      deleteFanData: '/web/fanPointData/delete', // 删除风机点位数据
      updateFanData: '/web/fanPointData/update', // 修改风机点位数据
      importFanData: '/web/fanPointData/import', // 导入风机点位数据
      downloadFanData: '/web/fanPointData/exportExcel', // 下载风机点位数据
      importKmz: '/web/project/api/fan/location/importKmz', // 导入KMZ文件
      getKmz: '/web/project/api/fan/location/getFanLocationVosByProjectId' // 查询导入的kmz数据
    }
  },
  // 存档管理
  ArchiveManage: {
    importFolderTemplate: 'web/file/api/file/importFolderTemplate',
    folderTemplateList: '/web/file/api/folderTemplate/list',
    getRelatedPartyList: 'web/project/api/relatedParty/getRelatedPartyList',
    saveProjectFolder: '/web/file/api/file/saveProjectFolder',
    getFileDataByProject: 'web/file/api/file/getFileDataByProject'
  },
  engineeringSettlement: {
    // serveSave:'/web/measuring/api/settlementServeApi/save',// 工程结算服务-服务结算
    // serveUpdate:'/web/measuring/api/settlementServeApi/update', // 工程结算服务-服务编辑
    // serveDelete:'/web/measuring/api/settlementServeApi/delete', // 工程结算服务-服务删除
    // serveList:'/web/measuring/api/settlementServeApi/list', // 工程结算服务-服务列表
    // expenseTypeList:'/web/measuring/api/settlementServeApi/expenseTypeList',// 工程结算-费用类型
    serveSave: '/web/measuring/api/settlementServeApi/save', // 工程结算服务-服务结算
    serveUpdate: '/web/measuring/api/settlementServeApi/update', // 工程结算服务-服务编辑
    serveDelete: '/web/measuring/api/settlementServeApi/delete', // 工程结算服务-服务删除
    serveList: '/web/measuring/api/settlementServeApi/list', // 工程结算服务-服务列表
    equipmentUnsettledList: '/web/measuringApi/orderReceipt/list', // 工程设备未结算列表获取
    equipmentUnsettledFindById: '/web/measuringApi/orderReceipt/findById', // 工程设备未结算详情获取
    equipmentSave: '/web/measuring/api/equipmentSettlementApi/save', // 工程结算-设备/材料-结算
    equipmentDelete: '/web/measuring/api/equipmentSettlementApi/delete', // 工程结算-设备/材料-删除
    equipmentList: '/web/measuring/api/equipmentSettlementApi/list', // 工程结算-设备/材料-列表
    equipmentSummary: '/web/measuring/api/equipmentSettlementApi/summaryListOne', // 工程结算-设备/材料-汇总
    expenseTypeList: '/web/measuring/api/settlementServeApi/expenseTypeList', // 工程结算-费用类型
    engineeringList: '/web/measuring/api/settlementSummary/list', // 工程结算列表查询
    engineeringSave: '/web/measuring/api/settlementSummary/save', // 工程结算新增
    engineeringUpdateById: '/web/measuring/api/settlementSummary/updateById', // 工程结算编辑
    engineeringDelete: '/web/measuring/api/settlementSummary/delete', // 删除工程结算数据
    engineeringListByAll: '/web/measuring/api/settlementSummary/listByAll', // 工程结算子页面新增查询
    engineeringFindById: '/web/measuring/api/settlementSummary/findById', // 工程结算根据列表id查询当前数据
    engineeringUpdate: '/web/measuring/api/settlementSummary/update', // 表六单项工程提交暂估价
    engineeringBidSave: '/web/measuring/api/bidDetails/save', // 表五中的上传的新增提交接口
    engineeringBidUpdate: '/web/measuring/api/bidDetails/update', // 表五中的重新上传的提交接口
    engineeringUploadFile: '/web/measuring/api/materialSummary/uploadFile' // 上传材差接口
  },
  // 合同采购
  bidPurchase: {
    list: '/web/measuring/api/procurePlan/list', // 列表查询
    findById: '/web/measuring/api/procurePlan/findById', // 详情查询
    pushBidding: '/web/measuring/api/procurePlan/pushBidding', // 推送招投标
    contractReviewList: '/web/measuring/api/contractReview/list', // 合同评审列表查询
    biddingPlatformList: '/web/measuring/api/biddingPlatform/list' // 招标平台列表查询
  },
  // 合同
  contract: {
    contractReviewList: '/web/measuring/api/contractReview/list',
    contractPaymentList: '/web/measuring/api/contractPayment/list'
  },
  // 设备材料清单
  materialsAndEquipment: {
    purchaseInventoryList: '/web/measuringApi/purchaseInventory/list',
    uploadInventory: '/web/measuringApi/purchaseInventory/uploadInventory',
    purchaseInventoryFindById: '/web/measuringApi/purchaseInventory/findById',
    purchaseInventoryDelete: '/web/measuringApi/purchaseInventory/delete',
    purchaseSummaryList: '/web/measuringApi/purchaseSummary/list'
  },
  // 汇总页
  summaryList: {
    summaryListOne: '/web/measuring/api/equipmentSettlementApi/summaryListOne',
    summaryList: '/web/measuring/api/settlementSummary/summaryList',
    settlementSummaryList: '/web/measuring/api/settlementSummary/list',
    settlementServeApiList: '/web/measuring/api/settlementServeApi/list',
    equipmentSettlementApiList: '/web/measuring/api/equipmentSettlementApi/list'
  },
  algorithm: {
    getDicCodeTree: '/web/dict/api/dictInfo/getDictTree', // 获取字典树
    list: '/web/dict/api/dictInfo/list' // 查询数据字典信息列表
  },
  noForm: {
    createBuyPlan: '/web/measuring/api/procurePlan/save', // 创建采购计划
    deleteBuyPlan: '/web/measuring/api/procurePlan/delete', // 删除采购计划
    findBuyPlanById: '/web/measuring/api/procurePlan/findById', // 查询采购计划
    saveContractReview: '/web/measuring/api/contractReview/save', // 新建合同评审
    deleteContractReview: '/web/measuring/api/contractReview/delete', // 删除合同评审
    getBuyPlanList: '/web/measuring/api/procurePlan/list', // 获取采购计划申请列表
    getContractReviewById: '/web/measuring/api/contractReview/findById', // 获取合同申请详情
    deviceMaterialList: '/web/measuring/api/equipmentSettlementApi/deviceMaterialList', // 设备材料清单列表
    contractReviewList: '/web/measuring/api/contractReview/list', // 合同评审列表
    getContractPaymentByid: '/web/measuring/api/contractPayment/findById', // 合同评审详情
    contractPaymentSave: '/web/measuring/api/contractPayment/save', // 合同付款申请保存
    deleteContractPayment: '/web/measuring/api/contractPayment/delete', // 删除合同付款申请
    getContractPaymentById: '/web/measuring/api/contractPayment/findById', // 获取合同付款申请详情
    saveBuyDemand: '/web/measuringApi/materialDeviceDemandTable/save', // 保存采购需求
    deleteBuyDemand: '/web/measuringApi/materialDeviceDemandTable/delete', // 删除采购需求
    savebuyOrder: '/web/measuringApi/purchaseOrder/save', // 保存采购单
    deleteBuyOrder: '/web/measuringApi/purchaseOrder/delete', // 删除采购单
    getBuyDemandById: '/web/measuringApi/materialDeviceDemandTable/findById', // 获取采购需求详情
    getDeviceMaterialList: '/web/measuringApi/purchaseSummary/list', // 获取设备列表
    getBuyOrderList: '/web/measuringApi/materialDeviceDemandTable/getEnableDemandTable', // 获取采购清单列表
    getMergeDemandTableByIds: '/web/measuringApi/materialDeviceDemandTable/getMergeDemandTableByIds',
    getMyInventory: '/web/measuringApi/purchaseInventory/getMyInventory',
    getByContractId: '/web/measuringApi/purchaseInventory/getByContractId',
    getBuyOrderById: '/web/measuringApi/purchaseOrder/findById',
    getInvoicNoticeList: '/web/measuringApi/orderNotice/getReceiveNotice', // 获取发货通知单
    getInvoicNoticeById: '/web/measuringApi/orderNotice/findById', // 获取发货通知单详细
    saveInvoice: '/web/measuringApi/orderReceipt/save', // 提交发货通知单
    deleteInvoice: '/web/measuringApi/orderReceipt/delete', // 删除发货通知单
    getEquipmentSettlementList: '/web/measuring/api/equipmentSettlementApi/list',
    getInvoiceById: '/web/measuringApi/orderReceipt/findById', // 获取发货单详情
    updateBuyDemand: '/web/measuringApi/materialDeviceDemandTable/update', // 更新采购申请单
    updatebuyOrder: '/web/measuringApi/purchaseOrder/update', // 更新采购单
    updateInvoice: '/web/measuringApi/orderReceipt/update', // 更新采购申请单
    getAllUsersOfGroup: '/web/user/api/user/getAllUsersOfGroup', // 获取公司所有员工
    serveList: '/web/measuring/api/settlementServeApi/list', // 工程结算服务-服务列表
    loadRelTreeList: '/web/project/api/relatedParty/findCompanyByProjectId' //
  },
  // 订发货
  orderAndDeliverGoods: {
    materialDeviceDemandTableList: '/web/measuringApi/materialDeviceDemandTable/list', // 采购需求list
    purchaseOrderList: '/web/measuringApi/purchaseOrder/list', // 采购单
    orderReceiptList: '/web/measuringApi/orderReceipt/list', // 发货单
    orderNoticeFindById: '/web/measuringApi/orderNotice/findById',
    orderNoticeList: '/web/measuringApi/orderNotice/list',
    orderNoticeSendNotice: '/web/measuringApi/orderNotice/sendNotice'
  },
  // 通知
  notice: {
    webReleaseList: '/web/information/api/webRelease/list', // 通知列表
    setNoticeReaded: '/web/information/api/noticeUser/save', // 设置已读
    setClickNums: '/web/information/api/webRelease/findByld' // 统计点击次数
  },
  assets: {
    assetsDetails: '/web/warehouse/api/assetsDetails/list', // 固定资产列表
    uploadExcel: '/web/warehouse/api/assetsDetails/uploadExcel', // 固定资产导入
    save: '/web/warehouse/api/assetsDetails/save', // 固定资产导入后保存
    exportToResponse: '/web/warehouse/api/assetsDetails/exportToResponse', // 固定资产导出
    vehicleInformationSave: '/web/warehouse/api/vehicleInformation/save', // 保存车辆
    vehicleInformationUpdate: '/web/warehouse/api/vehicleInformation/update', // 保存车辆
    vehicleInformationList: '/web/warehouse/api/vehicleInformation/list', // 车辆列表
    vehicleInformation: '/web/warehouse/api/vehicleInformation/findById', // 车辆详情
    annualInformationSave: '/web/warehouse/api/annualInformation/save', // 添加年审
    insuranceInformationSave: '/web/warehouse/api/insuranceInformation/save', // 添加保险
    maintenanceInformationSave: '/web/warehouse/api/maintenanceInformation/save', // 新增保养维修记录
    driverSave: '/web/warehouse/api/driverInformation/save', // 新增司机
    driverUpdate: '/web/warehouse/api/driverInformation/update', // 司机编辑
    driverList: '/web/warehouse/api/driverInformation/list', // 司机列表
    driverInfo: '/web/warehouse/api/driverInformation/findById' // 司机详情
  },
  // 新绩效考核
  newPerformance: {
    planUserGroupList: '/web/plan/api/planUserGroup/list', // 分组列表
    planUserGroupById: '/web/plan/api/planUserGroup/findById', // 分组根据ID查询接口
    planUserGroupSave: '/web/plan/api/planUserGroup/save', // 分组保存
    planUserGroupUpdate: '/web/plan/api/planUserGroup/update', // 分组更新
    planUserGroupDelete: '/web/plan/api/planUserGroup/delete', // 分组删除
    findUserByCompanyIdAndDutyLevelIds: '/web/user/api/user/findUserByCompanyIdAndDutyLevelIds', // 查询指定岗级下人员
    workPlanGroupList: '/web/plan/api/workPlanGroup/list', // 查询某年某季度下指定公司分组下人员计划表
    workPlanPromulgateSave: '/web/plan/api/workPlanPromulgate/save', // 计划表公示汇总表提交
    workPlanPromulgateList: '/web/plan/api/workPlanPromulgate/list', // 公示汇总表列表
    workPlanPromulgateById: '/web/plan/api/workPlanPromulgate/findById', // 根据id获取指定公示汇总表
    getMyWorkPlanPromulgate: '/web/plan/api/workPlanPromulgate/getMyWorkPlanPromulgate' // 获取当前用户可以看到的公示汇总表
  },
  // 金蝶
  kingdee: {
    // 供应商分组
    getCompanyVosContainPro: '/web/user/api/company/getCompanyVosContainPro', // 搜索条件使用组织列表
    supplierSave: '/web/warehouse/center/api/supplierGroup/save', // 供应商分组新增
    supplierUpdate: '/web/warehouse/center/api/supplierGroup/update', // 供应商分组更新
    supplierDelete: '/web/warehouse/center/api/supplierGroup/delete', // 供应商分组删除
    supplierList: '/web/warehouse/center/api/supplierGroup/list', // 供应商分组列表
    supplierGroupSyncKingdee: '/externalApi/external/invoke/supplier/syncSupplierGroup', // 一键增量同步金蝶供应商分组
    supplierGroupPushKingdee: '/externalApi/external/invoke/supplier/supplierGroupSave', // 推送供应商分组到金蝶
    supplierPushLogs: '/web/warehouse/center/api/supplierInfo/supplierPushLogs', // 供应商库操作日志
    cloudSupplierList: '/externalApi/external/invoke/supplier/cloudSupplierList', // 金蝶供应商快照列表
    kingdeeCompanyByCustomerCode: '/newWarehouseApi/warehouse/center/api/supplierInfo/findCompanyByCustomerCode', // 金蝶公司快照列表

    // 供应商库
    getKingdeeCompanyList: '/web/org/getCompanyListWithinProCompany', // 获取金蝶公司列表
    supplierInfoSave: '/web/warehouse/center/api/supplierInfo/save', // 供应商库新增
    supplierInfoDelete: '/newWarehouseApi/warehouse/center/api/supplierInfo/delete', // 供应商库删除
    supplierAlonePush: '/newWarehouseApi/warehouse/center/api/supplierInfo/alonePush', // 供应商库推送
    bindKingdeeSupplier: '/newWarehouseApi/warehouse/center/api/supplierInfo/bindKingdeeSupplier', // 供应商库关联金蝶供应商
    cancelBindKingdeeRelation: '/newWarehouseApi/warehouse/center/api/supplierInfo/cancelBindKingdeeRelation', // 供应商库解除关联
    supplierCorrelation: '/newWarehouseApi/warehouse/center/api/supplierInfo/correlationSupplier', // 供应商库分配
    supplierInfoList: '/newWarehouseApi/warehouse/center/api/supplierInfo/list', // 供应商库列表
    supplierInfoSyncKingdee: '/newWarehouseApi/warehouse/center/api/supplierInfo/syncKingdeeSupplier', // 供应商库一键同步金蝶供应商
    getKingdeeCompany: '/web/org/get/externalOrg/by/companyId', // 获取指定公司在金蝶的公司以及下面的项目公司

    // 公司供应商
    allocateSupplier: '/web/warehouse/center/api/supplierInfo/allocateSupplier', // 公司供应商分配
    supplierCompanyList: '/web/warehouse/center/api/supplierInfo/distributionList', // 公司供应商列表
    cancelAllocateSupplier: '/web/warehouse/center/api/supplierInfo/cancelAllocateSupplier', // 公司供应商取消分配

    pushRecordsList: '/web/externalApplicationApi/api/pushRecords/list', // 金蝶对接日志列表
    pushRecordManagementList: '/externalApi/external/pushRecordManagement/list', // 金蝶对接日志列表
    pushRecordManagementRetry: '/externalApi/external/pushRecordManagement/retry', // 金蝶对接日志重试
    pushRecordManagementDetail: '/externalApi/external/pushRecordManagement/findById', // 金蝶对接日志详情
    pushRecordLogPermissionList: '/externalApi/external/pushRecordLogPermission/list', // 新金蝶日志权限列表
    pushRecordLogPermissionSave: '/externalApi/external/pushRecordLogPermission/save', // 新金蝶日志权限保存
    pushRecordLogPermissionDelete: '/externalApi/external/pushRecordLogPermission/delete' // 新金蝶日志权限删除
  },
  // 库存管理
  inventoryManage: {
    goodsSave: '/web/warehouse/center/api/w2/goods/save', // 材料库新增接口
    goodsUpdate: '/web/warehouse/center/api/w2/goods/update', // 材料库更新接口
    goodsDelete: '/web/warehouse/center/api/w2/goods/delete', // 材料库删除接口
    goodsList: '/web/warehouse/center/api/w2/goods/list', // 材料库列表接口
    goodsDetail: '/web/warehouse/center/api/w2/goods/findById', // 材料库详情接口
    goodsUploadLedger: '/web/warehouse/center/api/w2/goods/uploadLedger', // 材料库导入接口

    warehouseSave: '/web/warehouse/center/api/w2/warehouseInfo/save', // 仓库新增接口
    warehouseUpdate: '/web/warehouse/center/api/w2/warehouseInfo/update', // 仓库更新接口
    warehouseDelete: '/web/warehouse/center/api/w2/warehouseInfo/delete', // 仓库删除接口
    warehouseList: '/web/warehouse/center/api/w2/warehouseInfo/list', // 仓库列表接口
    warehouseDetail: '/web/warehouse/center/api/w2/warehouseInfo/findById', // 仓库详情接口

    ledgerSaveAll: '/web/warehouse/center/api/w2/goodsLedger/saveAll', // 库存台账-批量创建台账
    getNotSetLedgerGoods: '/web/warehouse/center/api/w2/goodsLedger/getNotSetLedgerGoods', // 库存台账-查询未加入台账的数据列表
    ledgerList: '/web/warehouse/center/api/w2/goodsLedger/list', // 台账列表接口
    ledgerExportDetail: '/web/warehouse/center/api/w2/goodsLedger/exportDetail', // 库存台账明细导出
    ledgerDetail: '/web/warehouse/center/api/w2/goodsLedger/findById', // 台账详情接口
    getSetLedgerGoods: '/web/warehouse/center/api/w2/goodsLedger/getSetLedgerGoods', // 库存台账-查询已加入台账的数据列表
    goodsBillList: '/web/warehouse/center/api/w2/goodsBill/list', // 出入库列表查询
    findGoodsBillItemById: '/web/warehouse/center/api/w2/goodsLedger/findGoodsBillItemById', // 台账出入库详情查询
    goodsBillDetail: '/web/warehouse/center/api/w2/goodsBill/findById', // 出入库界面出入库详情查询

    // 盘点
    goodsCheckRecordSave: '/web/warehouse/center/api/w2/goodsCheckRecord/save', // 盘点保存
    goodsCheckRecordUpdate: '/web/warehouse/center/api/w2/goodsCheckRecord/update', // 盘点更新
    goodsCheckRecordList: '/web/warehouse/center/api/w2/goodsCheckRecord/list', // 盘点列表查询
    goodsCheckRecordDetail: '/web/warehouse/center/api/w2/goodsCheckRecord/findById' // 盘点详情查询
  }
};
export default api;
