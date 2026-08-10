html使用

```html
<eleupload ref="eleupload"></eleupload>
```

js获取文件id

```javascript
//fileId （Array [1111,2222,3333]）
let fileId = this.$refs.eleupload.getFileId();
```

文件回显

```html
<eleupload ref="eleupload" :showOnly="true" :attachFile="attachFile"></eleupload>
```

| 参数       | 类型  | 说明                       | 默认值 |
| ---------- | ----- | -------------------------- | ------ |
| showOnly   | Bool  | 是否只显示文件，不显示按钮 | false  |
| attachFile | Array | 文件列表                   | []     |

**attachFile**:

| 属性              | 类型   | 说明     |
| ----------------- | ------ | -------- |
| name              | String | 文件名称 |
| id                | String | 文件id   |
| absolutelyFileUrl | String | 文件地址 |

