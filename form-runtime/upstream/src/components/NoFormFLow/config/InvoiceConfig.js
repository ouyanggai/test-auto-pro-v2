const formMainConfig = [
  [
    {
      type:'label',
      title:'项目名称',
      span:4,
    },
    {
      type:'input',
      prop:'projectName',
      span:8,
      // options:[],
      value:'',
      // rules:[
      //   { required: true, message: '项目名称不能为空',trigger:'change'},
      // ]
    },
    // {
    //   type:'label',
    //   title:'合同编号',
    //   span:4,
    // },
    // {
    //   type:'input',
    //   prop:'contractCode',
    //   span:8,
    //   value:''
    // },
  ],
  [
    {
      type:'label',
      title:'采购单位',
      span:4,
    },
    {
      type:'input',
      prop:'firstParty',
      span:4,
      value:'',
      // rules:[
      //   { required: true, message: '采购单位不能为空'},
      // ],
      disabled:true
    },
    {
      type:'label',
      title:'采购单位负责人',
      span:4,
    },
    {
      type:'select',
      // type:'input',
      prop:'handledBy',
      span:4,
      value:'',
      options:[],
      filterable:true,
      rules:[
        { required: true, message: '采购单位负责人不能为空',trigger:'change'},
      ]
    },
    {
      type:'label',
      title:'电话',
      span:4,
    },
    {
      type:'input',
      prop:'telephoneNumber',
      span:4,
      value:''
    },
  ]
]

const mainTableConfig = {
  isSelection:false,
  isIndex:true,
  align:'center',
  border:true,
  column:[
    {label:'名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'发货数量',prop:'arrivalAmount',width:''},
    {label:'审核数量',prop:'reviewAmount',width:'',
      slot:{
        type:'number',
        // inputEvent:()=>{},
        // rules:[
        //   { required: true, message: '审核数量不能为空'},
        // ],
        disabled:true
      }
    },
    {label:'品牌',prop:'brand',width:'',

    },
    {label:'备注',prop:'remark',width:'',

    },
  ],
}

const addressFormConfig = [
  [
    {
      type:'label',
      title:'送货方',
      span:4,
    },
    {
      span:8,
      children:[
        [
          {
            type:'label',
            align:'left',
            title:'送货单位（盖章）',
            span:24,
            borderRight:true,
            height:'45px'
          },
        ],[
          {
            type:'label',
            title:'代表签字',
            align:'left',
            span:24,
            borderRight:true,
          }
        ],[
          {
            type:'label',
            align:'left',
            title:'日期',
            span:24,
            borderRight:true,
            height:'75px'
          }
        ]
      ]
    },
    {
      type:'label',
      title:'收货方',
      span:4,
    },
    {
      span:8,
      children:[
        [
          {
            type:'label',
            title:'收货单位（盖章）',
            span:24,
            align:'left',
            height:'45px'
          },
        ],[
          {
            type:'label',
            title:'代表签字',
            span:24,
            align:'left',
          }
        ],[
          {
            type:'label',
            title:'日期',
            span:24,
            align:'left',
            height:'75px'
          }
        ]
      ]
    },
  ],
]

const listConfig = {
  isSelection:false,
  isIndex:false,
  align:'center',
  border:false,
  isRadio:true,
  column:[
    {label:'表单编号',prop:'orderCode',width:''},
    {label:'项目名称',prop:'projectName',width:''},
    {label:'采购单位',prop:'purchaseCompany',width:''},
    // {label:'采购单位负责人',prop:'handleBy',width:''},
  ],
  // action:{
  //   width:100,
  //   buttons:[
  //     {
  //       label:'详情',
  //       clickEvent:()=>{}
  //     }
  //   ]
  // }
}


const listStep2Config = {
  isSelection:true,
  isIndex:true,
  align:'center',
  border:false,
  column:[
    {label:'名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'采购量',prop:'purchaseAmount',width:''},
    {label:'发货量',prop:'arrivalAmount',width:'',
      slot:{
        type:'number',
        inputEvent:()=>{},
        // rules:[
        //   { required: true, message: '发货量不能为空'},
        // ]
      }
    },
    {label:'剩余量',prop:'leftAmount',width:''},
  ],
}

const listDetailConfig = {
  isSelection:false,
  isIndex:false,
  align:'center',
  border:false,
  column:[
    {label:'名称',prop:'a',width:''},
    {label:'合同名称',prop:'aa',width:''},
    {label:'规格型号',prop:'b',width:''},
    {label:'质量标准及要求',prop:'d',width:''},
    {label:'单位',prop:'c',width:''},
    {label:'采购量',prop:'e',width:''},
    {label:'合同名称',prop:'f',width:''},
    {label:'供货单位',prop:'g',width:''},
    {label:'到货时间',prop:'h',width:''},
    {label:'到货地址',prop:'i',width:''},
    {label:'不含税价格',children:[
      {label:'单价',prop:'j',width:''},
    ]},
    {
      label:'含税价格',children:[
        {label:'税率%',prop:'k',width:''},
        {label:'单价',prop:'l',width:''},
        {label:'合计',prop:'m',width:''},
      ]
    },
    {label:'品牌',prop:'n',width:''},
    {label:'备注',prop:'p',width:''},
  ],
}
export {formMainConfig,mainTableConfig,addressFormConfig,listConfig,listStep2Config,listDetailConfig}
