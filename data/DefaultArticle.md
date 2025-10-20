# 欢迎使用 Anheyu-App

这是一篇系统生成的默认文章，你可以编辑或删除它。

## 段落文本 p

```markdown
<font color=#00ffff size=7> color=#00ffff </font>

<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
<font color=#00ffff size=7> color=#00ffff </font>
<p style="text-align: left">我是居中文字</p>
```

:::tabs

== tab 标签语法

```markdown
<font color=#00ffff size=7> color=#00ffff </font>

<p style="text-align: left">我是居中文字</p>
```

== tab 配置参数

颜色: color 十六进制值
大小: size 数字值(number)

p 标签支持写 自定义 css

== tab 样式预览

<font color=#00ffff size=7> color=#00ffff </font>

<p style="text-align: left">我是居中文字</p>

== tab 示例源码

```markdown
<font color=#00ffff size=7> color=#00ffff </font>

<p style="text-align: left">我是居中文字</p>
```

:::

### 🤖 基本演示

**加粗**，<u>下划线</u>，_斜体_，~~删除线~~，上标^26^，下标~1~，`inline code`，[超链接](https://github.com/imzbf)

> 引用：《I Have a Dream》

1. So even though we face the difficulties of today and tomorrow, I still have a dream.
2. It is a dream deeply rooted in the American dream.
3. I have a dream that one day this nation will rise up.

- [ ] 周五
- [ ] 周六
- [x] 周天

![图片](https://imzbf.github.io/md-editor-rt/imgs/mark_emoji.gif)

## 🤗 代码演示

```vue
<template>
  <MdEditor v-model="text" />
</template>

<script setup>
import { ref } from "vue";
import { MdEditor } from "md-editor-v3";
import "md-editor-v3/lib/style.css";

const text = ref("Hello Editor!");
</script>
```

## 🖨 文本演示

依照普朗克长度这项单位，目前可观测的宇宙的直径估计值（直径约 930 亿光年，即 8.8 × 10^26^ 米）即为 5.4 × 10^61^倍普朗克长度。而可观测宇宙体积则为 8.4 × 10^184^立方普朗克长度（普朗克体积）。

## 📈 表格演示

| 表头 1 |  表头 2  | 表头 3 |
| :----- | :------: | -----: |
| 左对齐 | 中间对齐 | 右对齐 |

## 📏 公式

行内：$x+y^{2x}$

$$
\sqrt[3]{x}
$$

## 🧬 图表

```mermaid
flowchart TD
  Start --> Stop
```

```mermaid
---
title: Example Git diagram
---
gitGraph
   commit
   commit
   branch develop
   checkout develop
   commit
   commit
   checkout main
   merge develop
   commit
   commit
```

## 🪄 提示

!!! success 支持的类型

note、abstract、info、tip、success、question、warning、failure、danger、bug、example、quote、hint、caution、error、attention

!!!

## 🖼️ 图片组

图片组插件可以创建美观的网格布局图片展示。

:::gallery
![示例图片1](https://picsum.photos/800/600?random=1 "随机图片 1")
![示例图片2](https://picsum.photos/800/600?random=2 "随机图片 2")
![示例图片3](https://picsum.photos/800/600?random=3 "随机图片 3")
:::

**自定义列数**

:::gallery cols=4 gap=8px
![照片1](https://picsum.photos/600/400?random=4)
![照片2](https://picsum.photos/600/400?random=5)
![照片3](https://picsum.photos/600/400?random=6)
![照片4](https://picsum.photos/600/400?random=7)
![照片5](https://picsum.photos/600/400?random=8)
![照片6](https://picsum.photos/600/400?random=9)
![照片7](https://picsum.photos/600/400?random=10)
![照片8](https://picsum.photos/600/400?random=11)
:::

**正方形布局**

:::gallery cols=3 ratio=1:1
![方形图1](https://picsum.photos/800/800?random=12)
![方形图2](https://picsum.photos/800/800?random=13)
![方形图3](https://picsum.photos/800/800?random=14)
![方形图4](https://picsum.photos/800/800?random=15)
![方形图5](https://picsum.photos/800/800?random=16)
![方形图6](https://picsum.photos/800/800?random=17)
:::

```markdown
:::gallery cols=列数 gap=间距 ratio=宽高比
![图片1](https://upload-bbs.miyoushe.com/upload/2025/10/20/125766904/d9bd6eaa4bd95b4a3822697d2a02b9fe_3838888873972014349.jpg "标题1")
![图片2](https://upload-bbs.miyoushe.com/upload/2025/10/20/125766904/70dd78e6ccdebf05ea6cca4926dab2f3_3988741683324456483.jpg "标题2")
:::
```

## ☘️ 占个坑@！

没了
