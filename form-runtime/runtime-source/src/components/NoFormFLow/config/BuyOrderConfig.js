const formMainConfig = {
  isSelection:false,
  isIndex:true,
  align:'center',
  border:false,
  column:[
    {label:'名称',prop:'name',width:'',fixed:true},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'质量标准及要求',prop:'qualityRequirement',width:'',
      slot:{
        type:'input',
      }
    },
    {label:'单位',prop:'unit',width:'50'},
    {label:'采购数量',prop:'purchaseAmount',width:'70'},
    {label:'合同名称',prop:'contractName',width:''},
    {label:'供货单位',prop:'secondParty',width:''},
    {label:'到货时间',prop:'arrivalTime',width:''},
    {label:'到货地址',prop:'deliveryAddress',width:''},
    {
      label:'不含税单价',prop:'unitPriceNotIncludingTax',width:'90px'
    },
    {
      label:'含税价格',children:[
        {label:'税率%',prop:'taxRate',width:'80px',
          slot:{
            type:'number'
          }
        },
        {label:'单价',prop:'unitPriceIncludingTax',width:'90px'},
        {label:'合计',prop:'totalPriceIncludingTax',width:'90px'},
      ]
    },
    {label:'品牌',prop:'brand',width:'80px',
      slot:{
        type:'input'
      }
    },
    {label:'备注',prop:'remark',width:'80px',
      slot:{
        type:'input'
      }
    },
  ]
}

const listConfig = {
  isSelection:true,
  isIndex:false,
  align:'center',
  border:false,
  isRadio:false,
  column:[
    {label:'申请单编号',prop:'code',width:''},
    {label:'项目编号',prop:'contractNumber',width:''},
    {label:'项目名称',prop:'contractName',width:''},
    {label:'项目地点',prop:'projectAddress',width:''},
  ],
  action:{
    width:100,
    buttons:[
      {
        label:'查看',
        clickEvent:()=>{}
      }
    ]
  }
}

const listStep2Config = {
  isSelection:false,
  isIndex:false,
  align:'center',
  border:false,
  column:[
    {label:'设备/材料名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'申请量',prop:'applyAmount',width:''},
    {label:'采购量',prop:'orderAmount',width:''},
    {label:'已分配量',prop:'purchaseAmount',width:''},
    // {label:'剩余量',prop:'f',width:''},
    // {label:'不含税单价（元）',prop:'g',width:''},
    // {label:'不含税合价（元）',prop:'h',width:''},
    // {label:'含税单价（元）',prop:'i',width:''},
    // {label:'含税合价（元）',prop:'j',width:''},
    {label:'备注',prop:'remark',width:''},
  ],
  action:{
    width:100,
    buttons:[
      {
        label:'分配合同',
        clickEvent:()=>{}
      }
    ]
  }
}

const listDetailConfig = {
  isSelection:false,
  isIndex:false,
  align:'center',
  border:false,
  column:[
    {label:'设备/材料名称',prop:'name',width:''},
    {label:'规格型号',prop:'modelNumber',width:''},
    {label:'单位',prop:'unit',width:''},
    {label:'申请量',prop:'applyAmount',width:''},
    // {label:'不含税单价（元）',prop:'e',width:''},
    // {label:'不含税合价（元）',prop:'f',width:''},
    // {label:'含税单价（元）',prop:'g',width:''},
    // {label:'含税合价（元）',prop:'h',width:''},
    {label:'备注',prop:'remark',width:''},
  ],
}

const contractConfig = {
  isSelection:false,
  isIndex:false,
  align:'center',
  border:false,
  column:[
    {label:'合同名称',prop:'contractId',width:'180px',slot:{
      type:'select',
      options:[],
      changeEvent:()=>{},
      rules:[
        { required: true, message: '请选择合同',trigger:'change'},
      ]
    }},
    {label:'供货单位',prop:'secondParty',width:'180px'},
    {label:'到货时间',prop:'arrivalTime',width:'',slot:{
      type:'date',
      rules:[
        { required: true, message: '请选择到货时间'},
      ]
      },
    },
    {label:'到货地址',prop:'deliveryAddress',width:'200px',
      slot:{
        type:'text',
        rules:[
          { required: true, message: '请输入到货地址'},
        ]
      }
    },
    {label:'采购量',prop:'purchaseAmount',width:'',
      slot:{
        type:'number',
        rules:[
          { required: true, message: '请输入采购量'},
        ]
      }
    },
    {label:'含税单价',prop:'unitPriceIncludingTax',width:'100px'},
    {label:'不含税单价',prop:'unitPriceNotIncludingTax',width:'100px'}
  ],
  action:{
    width:100,
    buttons:[
      {
        label:'删除',
        clickEvent:()=>{}
      }
    ]
  }
}

const contractAssignTemp = {
  contractId:'',
  contractName:'',
  agency:'',
  date:'',
  address:'',
  applyCount:'',
  unitPriceIncludingTax:'',
  unitPriceNotIncludingTax:''
}

export {formMainConfig,listConfig,listStep2Config,listDetailConfig,contractConfig,contractAssignTemp}
