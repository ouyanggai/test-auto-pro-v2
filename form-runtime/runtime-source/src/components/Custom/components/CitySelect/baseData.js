/**
 * 城市快捷选择基础数据
 * 用于 CitySelect 组件的快捷选择面板
 * 每个城市对象：{ name, code, country, haveFlight, haveTrain, haveHotel, fullName }
 */

// 辅助函数：创建城市对象
const city = (name, extra = {}) => ({
  name,
  code: extra.code || '',
  country: extra.country || '中国',
  haveFlight: extra.haveFlight !== undefined ? extra.haveFlight : true,
  haveTrain: extra.haveTrain !== undefined ? extra.haveTrain : true,
  haveHotel: extra.haveHotel !== undefined ? extra.haveHotel : true,
  fullName: extra.fullName || name
});

// 国内 Tab 定义
export const domesticTabs = [
  { key: 'hot', name: '热门', letters: [] },
  { key: 'abcdef', name: 'ABCDEF', letters: ['A', 'B', 'C', 'D', 'E', 'F'] },
  { key: 'ghjk', name: 'GHJK', letters: ['G', 'H', 'J', 'K'] },
  { key: 'lmnpq', name: 'LMNPQ', letters: ['L', 'M', 'N', 'P', 'Q'] },
  { key: 'rstw', name: 'RSTW', letters: ['R', 'S', 'T', 'W'] },
  { key: 'xyz', name: 'XYZ', letters: ['X', 'Y', 'Z'] }
];

// 国际港澳台 Tab 定义
export const overseasTabs = [
  { key: 'hot', name: '热门' },
  { key: 'asia', name: '亚洲' },
  { key: 'europe', name: '欧洲' },
  { key: 'america', name: '美洲' },
  { key: 'africa', name: '非洲' },
  { key: 'oceania', name: '大洋洲' }
];

// 国内城市数据
export const domesticCities = {
  // 热门
  hot: [
    city('北京市区'), city('上海市区'), city('杭州市区'), city('广州市区'),
    city('成都市区'), city('深圳市区'), city('青岛市区'), city('南京市区'),
    city('西安市区'), city('重庆市区'), city('苏州市区', { haveFlight: false }), city('长沙市区'),
    city('三亚市区'), city('武汉市区'), city('天津市区'), city('大连市区'),
    city('珠海市区'), city('厦门市区'), city('贵阳市区'), city('昆明市区'),
    city('哈尔滨市区'), city('沈阳市区'), city('福州市区'), city('济南市区')
  ],

  // ABCDEF
  abcdef: {
    A: [
      city('安庆市区'), city('鞍山市区'), city('安康市区'),
      city('安阳市区'), city('阿里地区'), city('安顺市区')
    ],
    B: [
      city('北海市区'), city('巴中市区'), city('百色市区'),
      city('滨州市区', { haveFlight: false }), city('白城市区'), city('本溪市区', { haveFlight: false }),
      city('白银市区', { haveFlight: false }), city('宝鸡市区', { haveFlight: false }), city('包头市区'),
      city('保亭黎族苗族自治县', { haveFlight: false }), city('毕节市区'), city('白沙黎族自治县', { haveFlight: false }),
      city('保定市区', { haveFlight: false }), city('白山市区'), city('亳州市区'),
      city('北京市区'), city('保山市区'), city('巴彦淖尔市区'),
      city('蚌埠市区')
    ],
    C: [
      city('潮州市区', { haveFlight: false }), city('常德市区'), city('昌江黎族自治县', { haveFlight: false }),
      city('沧州市区', { haveFlight: false }), city('郴州市区'), city('长春市区'),
      city('承德市区'), city('昌都市区'), city('池州市区'),
      city('长治市区'), city('长沙市区'), city('常州市区'),
      city('澄迈县', { haveFlight: false }), city('朝阳市区'), city('赤峰市区'),
      city('成都市区'), city('崇左市区', { haveFlight: false }), city('滁州市区', { haveFlight: false }),
      city('重庆市区')
    ],
    D: [
      city('达州市区'), city('大同市区'), city('德阳市区', { haveFlight: false }),
      city('德州市区', { haveFlight: false }), city('东营市区'), city('丹东市区'),
      city('定西市区', { haveFlight: false }), city('大连市区', { haveFlight: false }), city('定安县', { haveFlight: false }),
      city('大庆市区'), city('大兴安岭地区'), city('东莞市区', { haveFlight: false })
    ],
    E: [
      city('鄂州市区'), city('鄂尔多斯市区')
    ],
    F: [
      city('阜新市区', { haveFlight: false }), city('阜阳市区'), city('防城港市区', { haveFlight: false }),
      city('佛山市区'), city('抚州市区', { haveFlight: false }), city('福州市区'),
      city('抚顺市区', { haveFlight: false })
    ]
  },

  // GHJK
  ghjk: {
    G: [
      city('广元市区'), city('桂林市区'), city('贵港市区', { haveFlight: false }),
      city('贵阳市区'), city('广安市区', { haveFlight: false }), city('固原市区'),
      city('赣州市区'), city('广州市区')
    ],
    H: [
      city('淮南市区', { haveFlight: false }), city('汉中市区'), city('淮北市区', { haveFlight: false }),
      city('海口市区'), city('呼和浩特市区'), city('惠州市区'),
      city('海东市区', { haveFlight: false }), city('葫芦岛市区', { haveFlight: false }), city('黄山市区'),
      city('河池市区'), city('淮安市区'), city('邯郸市区'),
      city('哈尔滨市区'), city('菏泽市区'), city('衡阳市区'),
      city('衡水市区', { haveFlight: false }), city('鹤岗市区', { haveFlight: false }), city('贺州市区', { haveFlight: false }),
      city('黄石市区', { haveFlight: false }), city('河源市区', { haveFlight: false }), city('黑河市区'),
      city('怀化市区'), city('鹤壁市区', { haveFlight: false }), city('黄冈市区', { haveFlight: false }),
      city('呼伦贝尔市区', { haveFlight: false }), city('合肥市区'), city('哈密市区')
    ],
    J: [
      city('揭阳市区'), city('鸡西市区'), city('景德镇市区'),
      city('佳木斯市区'), city('江门市区', { haveFlight: false }), city('济宁市区'),
      city('晋中市区', { haveFlight: false }), city('荆州市区'), city('吉安市区'),
      city('焦作市区', { haveFlight: false }), city('金昌市区'), city('嘉峪关市区'),
      city('晋城市区', { haveFlight: false }), city('锦州市区'), city('济南市区'),
      city('九江市区'), city('酒泉市区', { haveFlight: false }), city('吉林市区', { haveFlight: false }),
      city('荆门市区', { haveFlight: false })
    ],
    K: [
      city('昆明市区'), city('开封市区', { haveFlight: false }), city('克拉玛依市区')
    ]
  },

  // LMNPQ
  lmnpq: {
    L: [
      city('吕梁市区'), city('临沂市区'), city('丽江市区'),
      city('辽阳市区', { haveFlight: false }), city('龙岩市区'), city('六安市区', { haveFlight: false }),
      city('陵水黎族自治县', { haveFlight: false }), city('乐东黎族自治县', { haveFlight: false }), city('临沧市区'),
      city('洛阳市区'), city('娄底市区', { haveFlight: false }), city('拉萨市区'),
      city('来宾市区', { haveFlight: false }), city('陇南市区'), city('临汾市区'),
      city('聊城市区', { haveFlight: false }), city('连云港市区'), city('林芝市区'),
      city('辽源市区', { haveFlight: false }), city('泸州市区'), city('柳州市区'),
      city('临高县', { haveFlight: false }), city('廊坊市区', { haveFlight: false }), city('漯河市区', { haveFlight: false }),
      city('兰州市区'), city('六盘水市区'), city('乐山市区', { haveFlight: false })
    ],
    M: [
      city('眉山市区', { haveFlight: false }), city('牡丹江市区'), city('绵阳市区'),
      city('梅州市区'), city('马鞍山市区', { haveFlight: false }), city('茂名市区', { haveFlight: false })
    ],
    N: [
      city('南通市区'), city('南平市区', { haveFlight: false }), city('南充市区'),
      city('内江市区', { haveFlight: false }), city('南京市区'), city('南阳市区'),
      city('宁德市区', { haveFlight: false }), city('南宁市区'), city('南昌市区')
    ],
    P: [
      city('平顶山市区', { haveFlight: false }), city('萍乡市区', { haveFlight: false }), city('平凉市区', { haveFlight: false }),
      city('莆田市区', { haveFlight: false }), city('攀枝花市区'), city('濮阳市区', { haveFlight: false }),
      city('盘锦市区', { haveFlight: false }), city('普洱市区')
    ],
    Q: [
      city('钦州市区', { haveFlight: false }), city('齐齐哈尔市区'), city('曲靖市区', { haveFlight: false }),
      city('琼中黎族苗族自治县', { haveFlight: false }), city('七台河市区', { haveFlight: false }), city('泉州市区'),
      city('秦皇岛市区'), city('庆阳市区'), city('清远市区', { haveFlight: false }),
      city('青岛市区')
    ]
  },

  // RSTW
  rstw: {
    R: [
      city('日喀则市区'), city('日照市区')
    ],
    S: [
      city('石家庄市区'), city('上饶市区'), city('十堰市区'),
      city('遂宁市区', { haveFlight: false }), city('上海市区'), city('石嘴山市区', { haveFlight: false }),
      city('苏州市区', { haveFlight: false }), city('三明市区'), city('山南市区', { haveFlight: false }),
      city('绥化市区', { haveFlight: false }), city('邵阳市区'), city('商洛市区', { haveFlight: false }),
      city('四平市区', { haveFlight: false }), city('三亚市区'), city('朔州市区'),
      city('松原市区'), city('三沙市区'), city('韶关市区'),
      city('神农架林区'), city('随州市区', { haveFlight: false }), city('汕尾市区', { haveFlight: false }),
      city('商丘市区', { haveFlight: false }), city('双鸭山市区', { haveFlight: false }), city('汕头市区', { haveFlight: false }),
      city('三门峡市区', { haveFlight: false }), city('厦门市区'), city('宿迁市区', { haveFlight: false }),
      city('沈阳市区'), city('深圳市区'), city('宿州市区', { haveFlight: false })
    ],
    T: [
      city('泰安市区', { haveFlight: false }), city('屯昌县', { haveFlight: false }), city('铁岭市区', { haveFlight: false }),
      city('天津市区'), city('铜仁市区'), city('天水市区'),
      city('铜陵市区', { haveFlight: false }), city('吐鲁番市区'), city('泰州市区', { haveFlight: false }),
      city('唐山市区'), city('通辽市区'), city('太原市区'),
      city('通化市区'), city('铜川市区', { haveFlight: false })
    ],
    W: [
      city('武汉市区'), city('西宁市区'), city('武威市区', { haveFlight: false }),
      city('梧州市区'), city('威海市区'), city('吴忠市区', { haveFlight: false }),
      city('乌兰察布市区'), city('无锡市区'), city('潍坊市区'),
      city('芜湖市区'), city('西安市区'), city('渭南市区', { haveFlight: false }),
      city('乌海市区'), city('乌鲁木齐市区')
    ]
  },

  // XYZ
  xyz: {
    X: [
      city('信阳市区'), city('新乡市区', { haveFlight: false }), city('忻州市区'),
      city('邢台市区'), city('咸阳市区', { haveFlight: false }), city('新余市区', { haveFlight: false }),
      city('徐州市区'), city('湘潭市区', { haveFlight: false }), city('宣城市区', { haveFlight: false }),
      city('许昌市区', { haveFlight: false }), city('咸宁市区', { haveFlight: false }), city('襄阳市区'),
      city('孝感市区', { haveFlight: false })
    ],
    Y: [
      city('营口市区'), city('烟台市区'), city('盐城市区'),
      city('伊春市区'), city('宜宾市区'), city('榆林市区'),
      city('鹰潭市区', { haveFlight: false }), city('岳阳市区'), city('雅安市区', { haveFlight: false }),
      city('益阳市区', { haveFlight: false }), city('运城市区'), city('银川市区'),
      city('扬州市区'), city('永州市区'), city('阳泉市区', { haveFlight: false }),
      city('宜春市区'), city('玉林市区'), city('宜昌市区'),
      city('延安市区'), city('阳江市区', { haveFlight: false }), city('云浮市区', { haveFlight: false }),
      city('玉溪市区', { haveFlight: false })
    ],
    Z: [
      city('昭通市区'), city('肇庆市区', { haveFlight: false }), city('镇江市区', { haveFlight: false }),
      city('驻马店市区', { haveFlight: false }), city('珠海市区'), city('遵义市区'),
      city('自贡市区', { haveFlight: false }), city('张家口市区'), city('张家界市区'),
      city('枣庄市区', { haveFlight: false }), city('漳州市区', { haveFlight: false }), city('周口市区', { haveFlight: false }),
      city('淄博市区', { haveFlight: false }), city('中卫市区'), city('张掖市区'),
      city('资阳市区', { haveFlight: false }), city('株洲市区', { haveFlight: false }), city('郑州市区'),
      city('湛江市区')
    ]
  }
};

// 国际港澳台城市数据
export const overseasCities = {
  hot: [
    city('中国香港', { country: '中国' }),
    city('中国澳门', { country: '中国' }),
    city('首尔', { country: '韩国' }),
    city('曼谷', { country: '泰国' }),
    city('新加坡', { country: '新加坡' }),
    city('莫斯科', { country: '俄罗斯' }),
    city('伦敦', { country: '英国' }),
    city('巴黎', { country: '法国' }),
    city('洛杉矶', { country: '美国' }),
    city('悉尼', { country: '澳大利亚' }),
    city('墨尔本', { country: '澳大利亚' }),
    city('迪拜', { country: '阿联酋' }),
    city('普吉岛', { country: '泰国' }),
    city('大阪', { country: '日本' }),
    city('开普敦', { country: '南非' })
  ],
  asia: [
    city('芽庄', { country: '越南', haveFlight: false }),
    city('京都', { country: '日本', haveFlight: false }),
    city('中国澳门', { country: '中国' }),
    city('中国香港', { country: '中国' }),
    city('新加坡', { country: '新加坡' }),
    city('曼谷', { country: '泰国' }),
    city('科伦坡', { country: '斯里兰卡' }),
    city('金边', { country: '柬埔寨' }),
    city('大阪', { country: '日本' }),
    city('巴厘岛', { country: '印度尼西亚' }),
    city('伊斯坦布尔', { country: '土耳其' }),
    city('迪拜', { country: '阿联酋' }),
    city('阿布扎比', { country: '阿联酋' }),
    city('胡志明市', { country: '越南' }),
    city('清迈', { country: '泰国' }),
    city('首尔', { country: '韩国' }),
    city('加德满都', { country: '尼泊尔' }),
    city('普吉岛', { country: '泰国' }),
    city('吉隆坡', { country: '马来西亚' })
  ],
  europe: [
    city('巴塞罗那省', { country: '西班牙', haveFlight: false }),
    city('莫斯科', { country: '俄罗斯' }),
    city('阿姆斯特丹', { country: '荷兰' }),
    city('巴黎', { country: '法国' }),
    city('慕尼黑', { country: '德国' }),
    city('威尼斯', { country: '意大利' }),
    city('米兰', { country: '意大利' }),
    city('伦敦', { country: '英国' }),
    city('剑桥', { country: '英国' }),
    city('牛津', { country: '英国' }),
    city('布达佩斯', { country: '匈牙利' }),
    city('圣彼得堡', { country: '俄罗斯' })
  ],
  america: [
    city('大温哥华', { country: '加拿大', haveFlight: false }),
    city('旧金山', { country: '美国' }),
    city('塞班岛', { country: '美国' }),
    city('里约热内卢', { country: '巴西' }),
    city('墨西哥城', { country: '墨西哥' }),
    city('洛杉矶', { country: '美国' }),
    city('多伦多', { country: '加拿大' }),
    city('圣保罗', { country: '巴西' })
  ],
  africa: [
    city('卡萨布兰卡', { country: '摩洛哥', haveFlight: false }),
    city('大阿克拉', { country: '加纳', haveFlight: false }),
    city('突尼斯', { country: '突尼斯' }),
    city('开普敦', { country: '南非' }),
    city('开罗', { country: '埃及' }),
    city('亚历山大', { country: '埃及' }),
    city('阿斯旺', { country: '埃及' }),
    city('马拉喀什', { country: '摩洛哥' })
  ],
  oceania: [
    city('基督城', { country: '新西兰' }),
    city('奥克兰', { country: '新西兰' }),
    city('墨尔本', { country: '澳大利亚' }),
    city('凯恩斯', { country: '澳大利亚' }),
    city('悉尼', { country: '澳大利亚' }),
    city('黄金海岸', { country: '澳大利亚' }),
    city('布里斯班', { country: '澳大利亚' })
  ]
};

// 将字母 Tab key 映射到其包含的字母列表
export const tabLetterMap = {
  abcdef: ['A', 'B', 'C', 'D', 'E', 'F'],
  ghjk: ['G', 'H', 'J', 'K'],
  lmnpq: ['L', 'M', 'N', 'P', 'Q'],
  rstw: ['R', 'S', 'T', 'W'],
  xyz: ['X', 'Y', 'Z']
};
