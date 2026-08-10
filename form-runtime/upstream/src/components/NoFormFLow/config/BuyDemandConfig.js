const formMainConfig = [
  [
    {
      type:'label',
      title:'合同名称',
      span:5,
    },
    {
      type:'select',
      prop:'reviewId',
      span:7,
      options:[],
      value:'',
      rules:[
        { required: true, message: '项目名称不能为空',trigger:'change'},
      ]
    },
    {
      type:'label',
      title:'项目编号',
      span:5,
    },
    {
      type:'input',
      // prop:'contractNumber',
      prop:'projectCode',
      placeholder:'自动获取',
      span:7,
      value:'',
      disabled:true
    },
  ],
  [
    {
      type:'label',
      title:'项目地点',
      span:5,
    },
    {
      type:'input',
      prop:'projectAddress',
      span:7,
      value:''
    },
    {
      type:'label',
      title:'项目经理及联系方式',
      span:5,
    },
    {
      type:'input',
      prop:'projectManager',
      span:7,
      value:''
    },
  ],
  // [
  //   {
  //     type:'label',
  //     title:'合同付款方式',
  //     span:5,
  //   },
  //   {
  //     type:'input',
  //     prop:'projectName2',
  //     span:7,
  //     value:''
  //   },
  //   {
  //     type:'label',
  //     title:'总包合同保质期',
  //     span:5,
  //   },
  //   {
  //     type:'input',
  //     prop:'projectCode',
  //     span:7,
  //     value:''
  //   },
  // ],
  [
    {
      type:'label',
      title:'合同签订单位（甲方）',
      span:5,
    },
    {
      type:'input',
      prop:'firstParty',
      span:12,
      value:'',
      disabled:true
    },
  ],
  [
    {
      type:'label',
      title:'采购范围及配置要求',
      span:5,
    },
    {
      span:19,
      type:'table',
      tableData:[]
    }
    // {
    //   span:19,
    //   children:[
    //     [
    //       {
    //         type:'label',
    //         title:'序号',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'label',
    //         title:'设备/材料名称',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'label',
    //         title:'规格型号',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'label',
    //         title:'单位',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'label',
    //         title:'申请量',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'label',
    //         title:'备注',
    //         span:4,
    //         borderRight:false
    //       },
    //     ],
    //     [
    //       {
    //         type:'input',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'input',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'input',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'input',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'input',
    //         span:4,
    //         borderRight:true
    //       },
    //       {
    //         type:'input',
    //         title:'备注',
    //         span:4,
    //       },
    //     ],
    //   ]
    // },
  ],
  // [
  //   {
  //     type:'label',
  //     title:'交货日期',
  //     span:5,
  //   },
  //   {
  //     type:'input',
  //     prop:'giveDate',
  //     span:12,
  //     value:''
  //   },
  // ],
  [
    {
      type:'label',
      title:'备注（特殊需求）',
      span:5,
    },
    {
      type:'input',
      prop:'remark',
      span:12,
      value:''
    },
  ],
  [
    {
      type:'label',
      title:'经办部门',
      span:5,
    },
    {
      type:'input',
      prop:'handlingDepartment',
      span:7,
      value:''
    },
    {
      type:'label',
      title:'经办人',
      span:5,
    },
    {
      type:'input',
      prop:'handledBy',
      span:7,
      value:''
    },
  ],
]

const listDetailConfig = {
  isSelection:true,
  isIndex:true,
  align:'center',
  border:false,
  column:[
    {label:'设备/材料名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'数量',prop:'contractAmount',width:''},
  ],
  // action:{
  //   width:100,
  //   buttons:[
  //     {
  //       label:'查看',
  //       clickEvent:()=>{}
  //     }
  //   ]
  // }
}

const tableConfig = {
  isSelection:false,
  isIndex:true,
  align:'center',
  border:true,
  column:[
    {label:'设备/材料名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'申请量',prop:'applyAmount',width:'',
      slot:{
        type:'number',
        rules:[
          { required: true, message: '申请量不能为空'},
        ]
      }
    },
    {label:'备注',prop:'remark',width:'',
      slot:{
        type:'input'
      }
    },
  ],
}
export {formMainConfig,listDetailConfig,tableConfig}
