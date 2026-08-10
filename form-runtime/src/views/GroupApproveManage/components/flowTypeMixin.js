import Api from '@/api';
import moment from 'moment';
import { deepClone } from '@/utils';
import { localstorageGet } from '@/utils/auth';

const mixin = {
  data(){
    return {
      batchSelectDatas:[],
      transpondVisible:false,
      approveForm: {
        approveMessage: '', // 审批信息
        checkList:[],
        personName:'',
        personId:'',
        createTime:'',
      },
      rules: {
        approveMessage: [
          { required: true, message: '请填写审批意见', trigger: 'blur' },
        ],
        personName: [
          { required: true, message: '请选择人员', trigger: 'change' },
        ]
      },
      fielSelectType:'',
      indicatorHeaderVisible:false,
      showSelectGroup: true,
      currentFormData: {},
      formProxyData: {},
      transpondFlowInstance: {},
      transpondFlowType:'',

      formExist: '',
      selectFlowName:'', //prop的flowName好像没值
      formId: '',
      flowInstanceId: '', // 流程实例id
      businessId:'',
      selectFlowType: '',
      oldFlowInstanceBizRelevanceList:[],
      batchNo:''
    }
  },
  methods: {
    /******** 转发逻辑开始  *********/
    // 人员选择选择赋值
    selectHeader(data, selectCompany) {
      // this.selectData = data;
      console.log('selectHeader',data)
      this.approveForm.personName = data.name;
      this.approveForm.personId = data.id;
    },
    openTranspondPerson(){
      console.log('openTranspondPerson')
      this.fielSelectType = 'company';
      this.indicatorHeaderVisible = true;
    },
    // 点击转发按钮
    async transpond(type){
      console.log('transpond-type',type)
      this.transpondFlowType = type;
      this.batchSelectDatas = this.$refs.dytable.selectDatas
      if(!this.batchSelectDatas.length){
        return this.$message.error('请选择流程')
      }
      if(this.batchSelectDatas.length > 1){
        return this.$message.error('只能选择一条流程')
      }
      console.log('this.batchSelectDatas',this.batchSelectDatas)

      let row = this.batchSelectDatas[0];
      console.log('row',row)
      this.formExist = row.formExist;
      this.oldFlowInstanceBizRelevanceList = row.flowInstanceBizRelevanceList;
      // if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
      //   const find = this.oldFlowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_flowName');
      //   this.selectFlowName = find.otherBizId;
      // } else {
      //   this.selectFlowName = row.flowInstanceName;
      //   // this.selectFlowName = row.flowName || row.formName;
      // }

      if (!this.formExist) { // formMaking表单
        this.flowInstanceId = row.flowInstanceId || row.id;
        this.batchNo = row.batchNo;
        this.selectFlowType = row.auditWay;
        this.formId = row.formProxyId;
        // this.selectFlowName = row.flowName || row.formName; // 不要用flowInstanceName，可能会出现叠加
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
      } else { // 固定页面
        this.selectFlowType = row.auditWay;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId || row.id;
        this.batchNo = row.batchNo;
        // this.selectFlowName = row.flowName || row.formName;
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
      }
      let formData = await this.getFormDataById(this.flowInstanceId)
      console.log('formData',formData)
      let formProxyData = await this.getFormData(this.formId)
      console.log('formProxyData',formProxyData)
      if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
        this.selectFlowName = formData.transpond_flowName;
      } else {
        this.selectFlowName = row.flowInstanceName || row.name;
        // this.selectFlowName = row.flowInstanceName;
        // this.selectFlowName = row.flowName || row.formName;
      }
      this.currentFormData = formData ? deepClone(formData) : {};
      this.formProxyData = formProxyData ? deepClone(formProxyData) : {};
      this.transpondFlowInstance = {
        selectFlowType:this.selectFlowType,
        businessId:this.businessId,
        formExist:this.formExist,
      }
      if (type == 'backlog' && formData.dynamicParam) this.showSelectGroup = false; // 转发的时候如果表单数据有之前转发的数据，就不用再显示流程日志和附言勾选
      this.approveForm = {
        approveMessage: '', // 审批信息
        checkList:[],
        personName:'',
        personId:'',
      }
      this.transpondVisible = true;
    },
    // 确定转发流程
    async confirmTranspond(){
      console.log('confirmTranspond')
      console.log('this.selectFlowName',this.selectFlowName)
      console.log('this.flowInstanceId',this.flowInstanceId)
      let row = this.batchSelectDatas[0];
      console.log('this.approveForm',this.approveForm)
      let originTranspondData = [];
      let paramData = {};
      let finalFlowName = '';
      this.approveForm.createTime = moment().format('YYYY-MM-DD HH:mm');
      console.log('this.currentFormData',this.currentFormData)

      let hasFuyan = this.approveForm.checkList.some(x=>x == 'fuyan');
      let hasYijian = this.approveForm.checkList.some(x=>x == 'yijian');
      // let fuyan = hasFuyan ? await this.getPostScriptList() : [];
      // let yijian = hasYijian ? await this.fetchLogData() : [];
      let fuyan = await this.getPostScriptList();
      let yijian = await this.fetchLogData();
      console.log('fuyan',fuyan)
      console.log('yijian',yijian)
      let dynamicParamResult = this.currentFormData.dynamicParam;
      if (dynamicParamResult) { // 如果之前已有转发数据，需要带上之前的数据
        console.log('第二次转发')
        // 第二次转发，每次都要获取新的意见和附言覆盖上一次
        // dynamicParamResult[dynamicParamResult.length-1]['fuyan'] = fuyan;
        // dynamicParamResult[dynamicParamResult.length-1]['yijian'] = yijian;
        originTranspondData = originTranspondData.concat(dynamicParamResult);
        // console.log('originTranspondData1',originTranspondData)
        // return;
        // if (this.transpondFlowType == 'backlog') { // 如果是在待办转发，后面转发的意见和附言就不要带过去了
        //   originTranspondData.push({
        //     approveForm: [this.approveForm],
        //     fuyan:[],
        //     yijian:[]
        //   })
        // } else {
          originTranspondData.push({
            approveForm: [this.approveForm],
            fuyan: hasFuyan ? await this.getPostScriptList() : [],
            yijian: hasYijian ? await this.fetchLogData() : []
          })
        // }

        // 自定义转发数据，和表单数据一起存
        paramData = {
          dynamicParam: originTranspondData, // 转发时填写的数据
          formProxyData: this.currentFormData.formProxyData,// 表单模板数据
          transpondFlowInstance: this.currentFormData.transpondFlowInstance // 实例所需的数据
        }

        console.log('this.currentFormData',this.currentFormData)
        const find = this.oldFlowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_originalName');
        finalFlowName = `${this.selectFlowName}(由${find.otherBizId}原发)`;
      } else {
        console.log('第一次转发')
        // let hasFuyan = this.approveForm.checkList.some(x=>x == 'fuyan');
        // let hasYijian = this.approveForm.checkList.some(x=>x == 'yijian');
        if (hasFuyan || hasYijian) {
          originTranspondData.push({
            approveForm: [this.approveForm],
            fuyan,
            yijian
          })
        }
        
        // 自定义转发数据，和表单数据一起存
        paramData = {
          dynamicParam: originTranspondData, // 转发时填写的数据
          transpond_flowName:this.selectFlowName, // 流程名称
          formProxyData: this.formProxyData,// 表单模板数据
          transpondFlowInstance: this.transpondFlowInstance, // 实例所需的数据
        }
        finalFlowName = `${this.selectFlowName}(由${localstorageGet('userName')}原发)`
      }
      console.log('paramData',paramData)
      console.log('finalFlowName',finalFlowName)
      
      // return;

      let param = {
        data:{
          name: finalFlowName,
        },
        receiverId: this.approveForm.personId,
        formDataMongoVo: {
          data: Object.assign(this.currentFormData,paramData)
        }
      }
      
      console.log('this.oldFlowInstanceBizRelevanceList',this.oldFlowInstanceBizRelevanceList)
      // 其实下面有表单好像是不用传这些数据，最多只需要一个company，传空值
      if (!dynamicParamResult) { // 初次转发传flowInstanceBizRelevanceList保留所需要的数据
        let customData = param.data.flowInstanceBizRelevanceList = [
          {
            otherBiz: 'transpond_flowInstanceId',
            otherBizId: this.flowInstanceId
          },
          {
            otherBiz: this.selectFlowType + '_transpondFlow',
            otherBizId: this.businessId
          },
          {
            otherBiz: 'transpond_formExist',
            otherBizId: this.formExist
          },
          {
            otherBiz: 'transpond_auditWay',
            otherBizId: this.selectFlowType
          },
          // {
          //   otherBiz: 'transpond_flowName',
          //   otherBizId: this.selectFlowName
          // },
          {
            otherBiz: 'transpond_originalName',
            otherBizId: localstorageGet('userName')
          },
          {
            otherBiz: 'company',
            otherBizId: ''
          },
        ];
        // 合并两个数组，以 oldList 为优先级
        let oldList = this.oldFlowInstanceBizRelevanceList.map(x=>({otherBiz:x.otherBiz,otherBizId:x.otherBizId}))
        const mergedArray = oldList.concat(customData.filter(item2 => {
          // 检查 oldList 中是否存在相同 otherBiz 的项
          // 另外年度目标责任书、年度绩效、月度绩效、后备英才绩效、月度绩效汇总、后备英才汇总、公司年度预算汇总、公司月度预算汇总、部门月度预算的auditType要过滤掉，
          // 因为后端在处理业务页面时候会去查这个类型，转发流程的type和正常流程的type不能重复。
          return !oldList.some(item1 => item1.otherBiz === item2.otherBiz);
        }));
        let newMergeArray = mergedArray.filter(item1=>item1.otherBiz !== 'annual_perf' && item1.otherBiz !== 'year_kpi_work_target' && item1.otherBiz !== 'monthly_perf'
        && item1.otherBiz !== 'monthly_perf_reserveTalent'&& item1.otherBiz !== 'monthly_perf_companySum'&& item1.otherBiz !== 'monthly_perf_departmentSum'
        && item1.otherBiz !== 'reserveMonthSum'&& item1.otherBiz !== 'group_finance'&& item1.otherBiz !== 'company_monthly_budget'
        && item1.otherBiz !== 'depart_monthly_budget')
        console.log('newMergeArray',newMergeArray)
        param.data.flowInstanceBizRelevanceList = newMergeArray;
      } else {
        console.log('第二次传参')
        console.log('第二次传参2',this.oldFlowInstanceBizRelevanceList)
        param.data.flowInstanceBizRelevanceList = this.oldFlowInstanceBizRelevanceList.map(x=>({otherBiz:x.otherBiz,otherBizId:x.otherBizId}))
      }
      console.log('param',param)
      // return;
      this.$axios.post(
        Api.schedule.transpondFlow,
        param,
        async (res) => {
          if (res.isSuccess) {
            this.handleSubmitPostscript(res.data.id).then((y)=>{
              this.fetchData();
              this.transpondVisible = false;
            })
          } else {
          }
        }
      );
    },
    // 发起人附言提交
    handleSubmitPostscript(id) {
      this.newPostscriptId = this.uuidGenerator();
      // if (this.postscriptMessage == '') {
      //   this.$message.warning('附言信息不能为空！');
      //   return false;
      // }
      const data = {
        text: this.approveForm.approveMessage,
        id: this.newPostscriptId,
        flowInstanceId: id
      };
      return this.$axios.post(
        Api.approveManage.savePostScript,
        { data }
      );
    },
    /* 随机id生成 */
    uuidGenerator() {
      const originStr = 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx';
      const originChar = '0123456789abcdef';
      const len = originChar.length;
      return originStr.replace(/x/g, function (match) {
        return originChar.charAt(Math.floor(Math.random() * len));
      });
    },
    // 查询附言
    getPostScriptList() {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.approveManage.getPostScriptList,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    // 查询流程日志
    fetchLogData () {
      console.log('获取流程日志')
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.approveManage.findRecord,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          res => {
            if (res.isSuccess) {
              let result = this.filterWithdraw(res.data);
              result.forEach(item => {
                this.translateStatus(item);
              });
              resolve(result);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    handleCloseTranspond(){
      this.transpondVisible = false
    },
    // 获取表单字段值
    getFormData(formProxyId) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getTaskFormDetail,
          {
            data: {
              id: formProxyId
            }
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data);
            }
          }
        );
      });
    },
    selectDataEvent(val){
      this.$emit('selectDataEvent',val)
    },
    // 获取表单字段值
    getFormDataById(id) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id//: this.flowInstanceId
            },
            // excludeFieldNames: ['auto_audit_info_']
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res?.data?.data || {});
            }
          },
          '',
          {noErrMsg:false,noLoading:true}
        );
      });
    },
    // 已建任务和流程日志操作状态字符转换
    translateStatus (obj) {
      let chnStatus;
      if (obj.auditStatus) {
        switch (obj.auditStatus) {
          case 'pass':
            chnStatus = '通过';
            break;
          case 'no_pass':
            chnStatus = '驳回';
            break;
          case 'withdraw':
            chnStatus = '撤销';
            break;
          case 'retrieve':
            chnStatus = '取回';
            break;
          case 'transfer':
            chnStatus = '移交';
            break;
          case 'roll_back_the_previous_level':
            chnStatus = '回退上一节点';
            break;
          default:
            chnStatus = '';
            break;
        }
      } else if (obj.flowStatus) {
        switch (obj.flowStatus) {
          case 'await_sent':
            chnStatus = '待发';
            break;

          case 'run':
            chnStatus = '运行中';
            break;

          case 'withdraw':
            chnStatus = '撤销';
            break;

          case 'termination':
            chnStatus = '终止';
            break;

          case 'rejected':
            chnStatus = '驳回';
            break;

          case 'end':
            chnStatus = '完结';
            break;

          default:
            chnStatus = '';
            break;
        };
      }
      obj.status = obj.auditStatus
      obj.auditStatus = chnStatus;
    },
    filterWithdraw (data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    /******** 转发逻辑结束  *********/
  }
};
export default mixin;
