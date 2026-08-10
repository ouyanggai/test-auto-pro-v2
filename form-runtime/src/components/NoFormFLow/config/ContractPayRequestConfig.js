const formMainConfig = [
  [
    {
      type: 'label',
      title: '合同名称',
      span: 4,
    },
    {
      type: 'select',
      prop: 'reviewId',
      span: 12,
      value: '',
      options: [],
      changeEvent: () => { },
      rules: [
        { required: true, message: '请选择合同', trigger: 'change' },
      ]
    }
  ],
  [
    {
      type: 'label',
      title: '合同编号',
      span: 4,
    },
    {
      type: 'input',
      prop: 'contractNumber',
      placeholder: '请输入合同编号',
      span: 8,
      value: '',
      disabled: true
    },
    {
      type: 'label',
      title: '合同类型',
      span: 4,
    },
    {
      type: 'select',
      options: [
        { value: 'server', label: '服务类' },
        { value: 'material', label: '材料类' },
        { value: 'engineering', label: '工程类' },
        { value: 'device', label: '设备类' }
      ],
      prop: 'contractType',
      span: 8,
      value: '',
      disabled: true
    },
  ],
  [
    {
      type: 'label',
      title: '合同金额',
      span: 4,
    },
    {
      type: 'input',
      prop: 'contractSum',
      placeholder: '',
      span: 4,
      value: '',
      disabled: true
    },
    {
      type: 'label',
      title: '已支付金额',
      span: 4,
    },
    {
      type: 'input',
      prop: 'paidAmount',
      placeholder: '',
      span: 4,
      value: '',
      disabled: true
    },
    {
      type: 'label',
      title: '剩余支付金额',
      span: 4,
    },
    {
      type: 'input',
      prop: 'residueAmount',
      placeholder: '',
      span: 4,
      value: '',
      disabled: true
    },
  ],
  [
    {
      type: 'label',
      title: '本月可支付金额',
      span: 4,
    },
    {
      span: 8,
      children: [
        [
          {
            type: 'button',
            title: '选择金额汇总表',
            prop: 'selectList',
            span: 10,
            clickEvent: () => { },
            disabled: true
          },
          {
            type: 'input',
            prop: 'monthAmount',
            placeholder: '本月可支付金额',
            span: 14,
            value: '',
            disabled: true,
            rules: [
              { required: true, message: '请选择金额汇总表' },
            ]
          },
        ]
      ]
    },
    {
      type: 'label',
      title: '申请支付金额',
      span: 4,
    },
    {
      type: 'inputNum',
      prop: 'paymentAmount',
      placeholder: '',
      span: 8,
      value: '',
      rules: [
        { required: true, message: '请填写申请支付金额' },
        // {
        //   pattern: /^[1-9]*[1-9][0-9]*$/,
        //   message: "请填写大于0的金额",
        //   trigger: "blur"
        // }
      ]
    },
  ],
  [
    {
      type: 'label',
      title: '账户信息',
      span: 4,
    },
    {
      span: 20,
      children: [
        [
          {
            type: 'label',
            title: '账户名称',
            span: 4,
          },
          {
            type: 'input',
            // prop:'secondPartyId',
            prop: 'secondParty',
            placeholder: '',
            span: 8,
            value: '',
            disabled: true
          },
        ],
        [
          {
            type: 'label',
            title: '收款账号',
            span: 4,
          },
          {
            type: 'input',
            prop: 'paymentAccount',
            placeholder: '',
            span: 8,
            value: '',
            disabled: true
          },
        ],
        [
          {
            type: 'label',
            title: '开户行',
            span: 4,
          },
          {
            type: 'input',
            prop: 'bank',
            placeholder: '',
            span: 8,
            value: '',
            disabled: true
          },
        ],
        [
          {
            type: 'label',
            title: '开户行行号',
            span: 4,
          },
          {
            type: 'input',
            prop: 'lineNumber',
            placeholder: '',
            span: 8,
            value: '',
            disabled: true
          },
        ]
      ]
    }
  ],
  [
    {
      type: 'label',
      title: '申请单位',
      span: 4,
    },
    {
      type: 'input',
      prop: 'application',
      placeholder: '',
      span: 8,
      value: '',
      disabled: true
    },
    {
      type: 'label',
      title: '申请人',
      span: 4,
    },
    {
      type: 'input',
      prop: 'userName',
      placeholder: '',
      span: 8,
      value: '',
      disabled: true
    },
  ],
  [
    {
      type: 'label',
      title: '建设单位',
      span: 4,
    },
    {
      span: 20,
      children: [
        [
          {
            type: 'label',
            title: '合同履行情况（请注明）',
            borderRight: true,
            span: 4,
          },
          {
            span: 20,
            children: [
              [
                {
                  type: 'label',
                  title: '合同履行',
                  span: 4,
                },
                {
                  type: 'radio',
                  span: 6,
                  prop: 'contractPerform',
                  options: [
                    { label: '正常', value: '1' },
                    { label: '非正常', value: '0' }
                  ],
                  value: '1'
                }
              ],
              [
                {
                  type: 'label',
                  title: '质量异议',
                  span: 4,
                },
                {
                  type: 'radio',
                  span: 6,
                  prop: 'qualityObjection',
                  options: [
                    { label: '有', value: '1' },
                    { label: '无', value: '0' }
                  ],
                  value: '0'
                }
              ],
              [
                {
                  type: 'label',
                  title: '索赔事项',
                  span: 4,
                },
                {
                  type: 'radio',
                  span: 6,
                  prop: 'claimantMatter',
                  options: [
                    { label: '有', value: '1' },
                    { label: '无', value: '0' }
                  ],
                  value: '0'
                }
              ],
              [
                {
                  type: 'label',
                  title: '预留质保金',
                  span: 4,
                },
                {
                  type: 'radio',
                  prop: 'retentionMoney',
                  span: 6,
                  options: [
                    { label: '有', value: '1' },
                    { label: '无', value: '0' }
                  ],
                  value: '1'
                },
                {
                  type: 'input',
                  prop: 'retentionMoneyDetail',
                  span: 8,
                  value: '',
                  placeholder: '如有，请输入金额',
                  disabled: false
                }
              ],
            ]
          }
        ],
        [
          {
            type: 'label',
            title: '付款说明',
            borderRight: true,
            span: 4,
          },
          {
            span: 20,
            children: [
              [
                {
                  type: 'label',
                  title: '付款日期',
                  borderRight: true,
                  span: 6
                },
                {
                  type: 'label',
                  title: '付款方式',
                  borderRight: true,
                  span: 6
                },
                {
                  type: 'label',
                  title: '金额复核',
                  borderRight: true,
                  span: 6
                },
                {
                  type: 'label',
                  title: '备注',
                  span: 6
                },
              ],
              [
                {
                  type: 'date',
                  dateType: 'date',
                  prop: 'payDate',
                  span: 6,
                  value: '',
                  formate: 'yyyy-MM-dd',
                  borderRight: true
                },
                {
                  type: 'select',
                  options: [
                    { label: '现金', value: 'cash' },
                    { label: '支票', value: 'cheque' },
                    { label: '汇票', value: 'draft' },
                    { label: '电汇', value: 'WireTransfer' },
                    { label: '银行承兑', value: 'bankHonour' },
                    { label: '其他', value: 'other' },
                  ],
                  prop: 'bankPayMethod',
                  span: 6,
                  value: '',
                  borderRight: true,
                  disabled: true
                },
                {
                  type: 'inputNum',
                  prop: 'amountReview',
                  span: 6,
                  borderRight: true,
                  rules: '',
                  // rules:[
                  //   { required: true, message: '请填写复核金额'},
                  // ],
                  disabled: true
                },
                {
                  type: 'input',
                  prop: 'remarks',
                  span: 6,
                },
              ]
            ]
          }
        ]
      ]
    },
  ]
]

const listDetailConfig = {
  isSelection: true,
  isIndex: false,
  align: 'center',
  border: false,
  column: [
    { label: '名称', prop: 'name', width: '' },
    { label: '结算金额', prop: 'settle', width: '' },
    { label: '提交人', prop: 'initiator', width: '' },
    { label: '关联合同名称', prop: 'relativeContract', width: '' },
    { label: '提交时间', prop: 'createDate', width: '' }
    // { label: '名称', prop: 'scheduleName', width: '' },
    // { label: '结算金额', prop: 'settlementMoney', width: '' },
    // { label: '提交人', prop: 'userName', width: '' },
    // { label: '关联合同名称', prop: 'contractName', width: '' },
    // { label: '提交时间', prop: 'createDate', width: '' }
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
export { formMainConfig, listDetailConfig }
