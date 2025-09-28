package configdef

import (
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// Definition 定义了单个配置项的所有属性。
type Definition struct {
	Key      constant.SettingKey
	Value    string
	Comment  string
	IsPublic bool
}

// UserGroupDefinition 定义了单个用户组的所有属性。
type UserGroupDefinition struct {
	ID          uint
	Name        string
	Description string
	Permissions model.Boolset
	MaxStorage  int64
	SpeedLimit  int64
	Settings    model.GroupSettings
}

// AllSettings 是我们系统中所有配置项的"单一事实来源"
var AllSettings = []Definition{
	// --- 站点基础配置 ---
	{Key: constant.KeyAppName, Value: "安和鱼", Comment: "应用名称", IsPublic: true},
	{Key: constant.KeySubTitle, Value: "生活明朗，万物可爱", Comment: "应用副标题", IsPublic: true},
	{Key: constant.KeySiteURL, Value: "https://anheyu.com", Comment: "应用URL", IsPublic: true},
	{Key: constant.KeyAppVersion, Value: "1.0.0", Comment: "应用版本", IsPublic: true},
	{Key: constant.KeyApiURL, Value: "/", Comment: "API地址", IsPublic: true},
	{Key: constant.KeyAboutLink, Value: "https://github.com/anzhiyu-c/anheyu-app", Comment: "关于链接", IsPublic: true},
	{Key: constant.KeyIcpNumber, Value: "湘ICP备2023015794号-2", Comment: "ICP备案号", IsPublic: true},
	{Key: constant.KeyUserAvatar, Value: "/static/img/avatar.jpg", Comment: "用户默认头像URL", IsPublic: true},
	{Key: constant.KeyLogoURL, Value: "/static/img/logo.svg", Comment: "Logo图片URL (通用)", IsPublic: true},
	{Key: constant.KeyLogoURL192, Value: "/static/img/logo-192x192.png", Comment: "Logo图片URL (192x192)", IsPublic: true},
	{Key: constant.KeyLogoURL512, Value: "/static/img/logo-512x512.png", Comment: "Logo图片URL (512x512)", IsPublic: true},
	{Key: constant.KeyLogoHorizontalDay, Value: "/static/img/logo-horizontal-day.png", Comment: "横向Logo (白天模式)", IsPublic: true},
	{Key: constant.KeyLogoHorizontalNight, Value: "/static/img/logo-horizontal-night.png", Comment: "横向Logo (暗色模式)", IsPublic: true},
	{Key: constant.KeyIconURL, Value: "/favicon.ico", Comment: "Icon图标URL", IsPublic: true},
	{Key: constant.KeySiteKeywords, Value: "安和鱼,博客,blog,搭建博客,服务器,搭建网站,建站,相册,图片管理", Comment: "站点关键词", IsPublic: true},
	{Key: constant.KeySiteDescription, Value: "新一代博客，就这么搭，Vue渲染颜值，Go守护性能，SSR打破加载瓶颈。", Comment: "站点描述", IsPublic: true},
	{Key: constant.KeyThemeColor, Value: "#163bf2", Comment: "应用主题颜色", IsPublic: true},
	{Key: constant.KeySiteAnnouncement, Value: "", Comment: "站点公告，用于在特定页面展示", IsPublic: true},
	{Key: constant.KeyFooterCode, Value: "", Comment: "页脚自定义HTML代码", IsPublic: true},
	{Key: constant.KeyDefaultThumbParam, Value: "", Comment: "默认缩略图处理参数", IsPublic: true},
	{Key: constant.KeyDefaultBigParam, Value: "", Comment: "默认大图处理参数", IsPublic: true},
	{Key: constant.KeyGravatarURL, Value: "https://cdn.sep.cc/", Comment: "Gravatar 服务器地址", IsPublic: true},
	{Key: constant.KeyDefaultGravatarType, Value: "mp", Comment: "Gravatar默认头像类型", IsPublic: true},
	{Key: constant.KeyUploadAllowedExtensions, Value: "", Comment: "允许上传的文件后缀名白名单，逗号分隔", IsPublic: true},
	{Key: constant.KeyUploadDeniedExtensions, Value: "", Comment: "禁止上传的文件后缀名黑名单，在白名单未启用时生效", IsPublic: true},
	// --- 缩略图生成器配置 ---
	{Key: constant.KeyEnableVipsGenerator, Value: "false", Comment: "是否启用 VIPS 缩略图生成器 (true/false)", IsPublic: true},
	{Key: constant.KeyVipsPath, Value: "vips", Comment: "VIPS 命令的路径或名称 (默认 'vips'，让系统自动搜索)", IsPublic: false},
	{Key: constant.KeyVipsMaxFileSize, Value: "78643200", Comment: "VIPS 生成器可处理的最大原始文件大小(单位:字节)，0为不限制", IsPublic: true},
	{Key: constant.KeyVipsSupportedExts, Value: "3fr,ari,arw,bay,braw,crw,cr2,cr3,cap,data,dcs,dcr,dng,drf,eip,erf,fff,gpr,iiq,k25,kdc,mdc,mef,mos,mrw,nef,nrw,obm,orf,pef,ptx,pxn,r3d,raf,raw,rwl,rw2,rwz,sr2,srf,srw,tif,x3f,csv,mat,img,hdr,pbm,pgm,ppm,pfm,pnm,svg,svgz,j2k,jp2,jpt,j2c,jpc,gif,png,jpg,jpeg,jpe,webp,tif,tiff,fits,fit,fts,exr,jxl,pdf,heic,heif,avif,svs,vms,vmu,ndpi,scn,mrxs,svslide,bif", Comment: "VIPS 此生成器可用的文件扩展名列表", IsPublic: true},
	{Key: constant.KeyEnableMusicCoverGenerator, Value: "true", Comment: "是否启用歌曲封面提取生成器 (true/false)", IsPublic: true},
	{Key: constant.KeyMusicCoverMaxFileSize, Value: "1073741824", Comment: "歌曲封面生成器可处理的最大原始文件大小(单位:字节, 默认1GB)，0为不限制", IsPublic: true},
	{Key: constant.KeyMusicCoverSupportedExts, Value: "mp3,m4a,ogg,flac", Comment: "歌曲封面提取器可用的文件扩展名列表", IsPublic: true},
	{Key: constant.KeyEnableFfmpegGenerator, Value: "false", Comment: "是否启用 FFmpeg 视频缩略图生成器 (true/false)", IsPublic: true},
	{Key: constant.KeyFfmpegPath, Value: "ffmpeg", Comment: "FFmpeg 命令的路径或名称 (默认 'ffmpeg'，让系统自动搜索)", IsPublic: false},
	{Key: constant.KeyFfmpegMaxFileSize, Value: "10737418240", Comment: "FFmpeg 生成器可处理的最大原始文件大小(单位:字节, 默认10GB)，0为不限制", IsPublic: true},
	{Key: constant.KeyFfmpegSupportedExts, Value: "3g2,3gp,asf,asx,avi,divx,flv,m2ts,m2v,m4v,mkv,mov,mp4,mpeg,mpg,mts,mxf,ogv,rm,swf,webm,wmv", Comment: "FFmpeg 此生成器可用的文件扩展名列表", IsPublic: true},
	{Key: constant.KeyFfmpegCaptureTime, Value: "00:00:01.00", Comment: "FFmpeg 定义缩略图截取的时间点", IsPublic: true},
	{Key: constant.KeyEnableBuiltinGenerator, Value: "true", Comment: "是否启用内置缩略图生成器 (true/false)", IsPublic: true},
	{Key: constant.KeyBuiltinMaxFileSize, Value: "78643200", Comment: "内置生成器可处理的最大原始文件大小(单位:字节)，0为不限制", IsPublic: true},
	{Key: constant.KeyBuiltinDirectServeExts, Value: "avif,webp", Comment: "内置生成器支持的直接服务扩展名列表", IsPublic: true},
	{Key: constant.KeyEnableLibrawGenerator, Value: "false", Comment: "是否启用 LibRaw/DCRaw 缩略图生成器 (true/false)", IsPublic: true},
	{Key: constant.KeyLibrawPath, Value: "simple_dcraw", Comment: "LibRaw/DCRaw 命令的路径或名称", IsPublic: false},
	{Key: constant.KeyLibrawMaxFileSize, Value: "78643200", Comment: "LibRaw/DCRaw 生成器可处理的最大原始文件大小(单位:字节, 75MB)", IsPublic: true},
	{Key: constant.KeyLibrawSupportedExts, Value: "3fr,ari,arw,bay,braw,crw,cr2,cr3,cap,data,dcs,dcr,dng,drf,eip,erf,fff,gpr,iiq,k25,kdc,mdc,mef,mos,mrw,nef,nrw,obm,orf,pef,ptx,pxn,r3d,raf,raw,rwl,rw2,rwz,sr2,srf,srw,tif,x3f", Comment: "LibRaw/DCRaw 此生成器可用的文件扩展名列表", IsPublic: true},

	// --- 队列配置 ---
	{Key: constant.KeyQueueThumbConcurrency, Value: "15", Comment: "缩略图生成队列的工作线程数", IsPublic: false},
	{Key: constant.KeyQueueThumbMaxExecTime, Value: "300", Comment: "单个缩略图生成任务的最大执行时间（秒）", IsPublic: false},
	{Key: constant.KeyQueueThumbBackoffFactor, Value: "2", Comment: "任务重试时间间隔的指数增长因子", IsPublic: false},
	{Key: constant.KeyQueueThumbMaxBackoff, Value: "60", Comment: "任务重试的最大退避时间（秒）", IsPublic: false},
	{Key: constant.KeyQueueThumbMaxRetries, Value: "3", Comment: "任务失败后的最大重试次数（0表示不重试）", IsPublic: false},
	{Key: constant.KeyQueueThumbRetryDelay, Value: "5", Comment: "任务重试的初始延迟时间（秒）", IsPublic: false},

	// --- 媒体信息提取配置 ---
	{Key: constant.KeyEnableExifExtractor, Value: "true", Comment: "是否启用 EXIF 提取 (true/false)", IsPublic: true},
	{Key: constant.KeyExifMaxSizeLocal, Value: "1073741824", Comment: "本地存储EXIF提取最大文件大小(单位:字节, 默认1GB)", IsPublic: true},
	{Key: constant.KeyExifMaxSizeRemote, Value: "104857600", Comment: "远程存储EXIF提取最大文件大小(单位:字节, 默认100MB)", IsPublic: true},
	{Key: constant.KeyExifUseBruteForce, Value: "true", Comment: "是否启用EXIF暴力搜索 (true/false)", IsPublic: true},
	{Key: constant.KeyEnableMusicExtractor, Value: "true", Comment: "是否启用音乐元数据提取 (true/false)", IsPublic: true},
	{Key: constant.KeyMusicMaxSizeLocal, Value: "1073741824", Comment: "本地存储音乐元数据提取最大文件大小(单位:字节, 默认1GB)", IsPublic: true},
	{Key: constant.KeyMusicMaxSizeRemote, Value: "1073741824", Comment: "远程存储音乐元数据提取最大文件大小(单位:字节, 默认1GB)", IsPublic: true},

	// --- Header/Nav 配置 ---
	{Key: constant.KeyHeaderMenu, Value: `[{"title":"文章","items":[{"title":"分类","path":"/categories","icon":"anzhiyu-icon-shapes","isExternal":false},{"title":"标签","path":"/tags","icon":"anzhiyu-icon-tags","isExternal":false},{"title":"文档","path":"https://dev.anheyu.com/","icon":"anzhiyu-icon-book","isExternal":true}]},{"title":"友链","items":[{"title":"友情链接","path":"/link","icon":"anzhiyu-icon-link","isExternal":false},{"title":"宝藏博主","path":"/travelling","icon":"anzhiyu-icon-cube","isExternal":false}]},{"title":"我的","items":[{"title":"音乐馆","path":"/ music","icon":"anzhiyu-icon-music","isExternal":false},{"title":"小空调","path":"/air-conditioner","icon":"anzhiyu-icon-fan","isExternal":false},{"title":"相册集","path":"/album","icon":"anzhiyu-icon-images","isExternal":false}]},{"title":"关于","items":[{"title":"随便逛逛","path":"/random-post","icon":"anzhiyu-icon-shoe-prints1","isExternal":false},{"title":"关于本站","path":"/about","icon":"anzhiyu-icon-paper-plane","isExternal":false},{"title":"我的装备","path":"/equipment","icon":"anzhiyu-icon-dice-d20","isExternal":false}]}]`, Comment: "主菜单配置 (有序数组结构)", IsPublic: true},
	{Key: constant.KeyHeaderNavTravel, Value: "true", Comment: "是否开启开往项目链接(火车图标)", IsPublic: true},
	{Key: constant.KeyHeaderNavClock, Value: "false", Comment: "导航栏和风天气开关", IsPublic: true},
	{Key: constant.KeyHeaderNavMenu, Value: `[{"title":"网页","items":[{"name":"个人主页","link":"https://index.anheyu.com/","icon":"https://upload-bbs.miyoushe.com/upload/2025/09/22/125766904/0a908742ef6ca443860071f8a338e26d_3396385191921661874.jpg?x-oss-process=image/format,avif"},{"name":"博客","link":"https://blog.anheyu.com/","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/ff8efb94f09b751a46b331ca439e9e62_2548658293798175481.png?x-oss-process=image/format,avif"},{"name":"安知鱼图床","link":"https://image.anheyu.com/","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/308b0ee69851998d44566a3420e6f9f2_2603983075304804470.png?x-oss-process=image/format,avif"}]},{"title":"项目","items":[{"name":"安知鱼主题","link":"https://dev.anheyu.com/","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/6bc70317b1001fe739ffb6189d878bbc_5557049562284776022.png?x-oss-process=image/format,avif"}]}]`, Comment: "导航栏下拉菜单配置 (结构化JSON)", IsPublic: true},
	{Key: constant.KeyHomeTop, Value: `{"title":"生活明朗","subTitle":"万物可爱。","siteText":"ANHEYU.COM","category":[{"name":"前端","path":"/categories/前端开发/","background":"linear-gradient(to right,#358bff,#15c6ff)","icon":"anzhiyu-icon-dove","isExternal":false},{"name":"大学","path":"/categories/大学生涯","background":"linear-gradient(to right,#f65,#ffbf37)","icon":"anzhiyu-icon-fire","isExternal":false},{"name":"生活","path":"/categories/生活日常","background":"linear-gradient(to right,#18e7ae,#1eebeb)","icon":"anzhiyu-icon-book","isExternal":false}],"banner":{"tips":"新品主题","title":"Theme-AnZhiYu","image":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/00961b9c22d3e633de8294555f3a3375_3621787229868352573.png?x-oss-process=image/format,avif","link":"https://dev.anheyu.com/","isExternal":true}}`, Comment: "首页顶部UI配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyCreativity, Value: `{"title":"技能","subtitle":"开启创造力","creativity_list":[{"name":"Java","color":"#fff","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/26ba17ce013ecde9afc8b373e2fc0b9d_1804318147854602575.jpg"},{"name":"Docker","color":"#57b6e6","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/544b2d982fd5c4ede6630b29d86f3cae_7350393908531420887.png"},{"name":"Photoshop","color":"#4082c3","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/4ce1d081b9b37b06e3714bee95e58589_1613929877388832041.png"},{"name":"Node","color":"#333","icon":"https://npm.elemecdn.com/anzhiyu-blog@2.1.1/img/svg/node-logo.svg"},{"name":"Webpack","color":"#2e3a41","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/32dc115fbfd1340f919f0234725c6fb4_4060605986539473613.png"},{"name":"Pinia","color":"#fff","icon":"https://npm.elemecdn.com/anzhiyu-blog@2.0.8/img/svg/pinia-logo.svg"},{"name":"Python","color":"#fff","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/02c9c621414cc2ca41035d809a4154be_7912546659792951301.png"},{"name":"Vite","color":"#937df7","icon":"https://npm.elemecdn.com/anzhiyu-blog@2.0.8/img/svg/vite-logo.svg"},{"name":"Flutter","color":"#4499e4","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/b5aa93e0b61d8c9784cf76d14886ea46_4590392178423108088.png"},{"name":"Vue","color":"#b8f0ae","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/cf23526f451784ff137f161b8fe18d5a_692393069314581413.png"},{"name":"React","color":"#222","icon":"data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9Ii0xMS41IC0xMC4yMzE3NCAyMyAyMC40NjM0OCI+PHRpdGxlPlJlYWN0IExvZ288L3RpdGxlPjxjaXJjbGUgY3g9IjAiIGN5PSIwIiByPSIyLjA1IiBmaWxsPSIjNjFkYWZiIi8+PGcgc3Ryb2tlPSIjNjFkYWZiIiBzdHJva2Utd2lkdGg9IjEiIGZpbGw9Im5vbmUiPjxlbGxpcHNlIHJ4PSIxMSIgcnk9IjQuMiIvPjxlbGxpcHNlIHJ4PSIxMSIgcnk9IjQuMiIgdHJhbnNmb3JtPSJyb3RhdGUoNjApIi8+PGVsbGlwc2Ugcng9IjExIiByeT0iNC4yIiB0cmFuc2Zvcm09InJvdGF0ZSgxMjApIi8+PC9nPjwvc3ZnPg=="},{"name":"CSS3","color":"#2c51db","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/02/125766904/948767d87de7c5733b5f59b036d28b4b_3573026798828830876.png"},{"name":"JS","color":"#f7cb4f","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/06216e7fddb6704b57cb89be309443f9_7269407781142156006.png"},{"name":"HTML","color":"#e9572b","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/02/125766904/f774c401c8bc2707e1df1323bdc9e423_1926035231499717029.png"},{"name":"Git","color":"#df5b40","icon":"https://upload-bbs.miyoushe.com/upload/2025/07/29/125766904/fcc0dbbfe206b4436097a8362d64b558_6981541002497327189.webp"},{"name":"Apifox","color":"#e65164","icon":"https://upload-bbs.miyoushe.com/upload/2025/08/02/125766904/b61bc7287d7f7f89bd30079c7f04360e_2465770520170903938.png"}]}`, Comment: "首页技能/创造力模块配置 (JSON格式)", IsPublic: true},

	// --- FrontDesk 配置 ---
	{Key: constant.KeyFrontDeskSiteOwnerName, Value: "安知鱼", Comment: "前台网站拥有者名", IsPublic: true},
	{Key: constant.KeyFrontDeskSiteOwnerEmail, Value: "anzhiyu-c@qq.com", Comment: "前台网站拥有者邮箱", IsPublic: true},
	{Key: constant.KeyFooterOwnerName, Value: "安知鱼", Comment: "页脚版权所有者名", IsPublic: true},
	{Key: constant.KeyFooterOwnerSince, Value: "2020", Comment: "页脚版权起始年份", IsPublic: true},
	{Key: constant.KeyFooterCustomText, Value: "", Comment: "页脚自定义文本", IsPublic: true},
	{Key: constant.KeyFooterRuntimeEnable, Value: "false", Comment: "页脚网站运行时间模块是否启用", IsPublic: true},
	{Key: constant.KeyFooterRuntimeLaunchTime, Value: "04/01/2021 00:00:00", Comment: "网站上线时间", IsPublic: true},
	{Key: constant.KeyFooterRuntimeWorkImg, Value: "https://npm.elemecdn.com/anzhiyu-blog@2.0.4/img/badge/安知鱼-上班摸鱼中.svg", Comment: "上班状态图", IsPublic: true},
	{Key: constant.KeyFooterRuntimeWorkDesc, Value: "距离月入25k也就还差一个大佬带我~", Comment: "上班状态描述", IsPublic: true},
	{Key: constant.KeyFooterRuntimeOffDutyImg, Value: "https://npm.elemecdn.com/anzhiyu-blog@2.0.4/img/badge/安知鱼-下班啦.svg", Comment: "下班状态图", IsPublic: true},
	{Key: constant.KeyFooterRuntimeOffDutyDesc, Value: "下班了就该开开心心的玩耍，嘿嘿~", Comment: "下班状态描述", IsPublic: true},
	{Key: constant.KeyFooterSocialBarCenterImg, Value: "https://upload-bbs.miyoushe.com/upload/2025/07/26/125766904/3acc3fb80887f4df723ff6842fdfe063_8129797316116697018.gif", Comment: "社交链接栏中间图片", IsPublic: true},
	{Key: constant.KeyFooterListRandomFriends, Value: "3", Comment: "页脚列表随机友链数量", IsPublic: true},
	{Key: constant.KeyFooterBarAuthorLink, Value: "/about", Comment: "底部栏作者链接", IsPublic: true},
	{Key: constant.KeyFooterBarCCLink, Value: "/copyright", Comment: "底部栏CC协议链接", IsPublic: true},
	{Key: constant.KeyFooterBadgeList, Value: `[{"link":"https://blog.anheyu.com/","shields":"https://npm.elemecdn.com/anzhiyu-theme-static@1.0.9/img/Theme-AnZhiYu-2E67D3.svg","message":"本站使用AnZhiYu主题"},{"link":"https://www.dogecloud.com/","shields":"https://npm.elemecdn.com/anzhiyu-blog@2.2.0/img/badge/CDN-多吉云-3693F3.svg","message":"本站使用多吉云为静态资源提供CDN加速"},{"link":"http://creativecommons.org/licenses/by-nc-sa/4.0/","shields":"https://npm.elemecdn.com/anzhiyu-blog@2.2.0/img/badge/Copyright-BY-NC-SA.svg","message":"本站采用知识共享署名-非商业性使用-相同方式共享4.0国际许可协议进行许可"}]`, Comment: "徽标列表 (JSON格式)", IsPublic: true},
	{Key: constant.KeyFooterSocialBarLeft, Value: `[{"title":"email","link":"http://mail.qq.com/cgi-bin/qm_share?t=qm_mailme&email=VDU6Ljw9LSF5NxQlJXo3Ozk","icon":"anzhiyu-icon-envelope"},{"title":"微博","link":"https://weibo.com/u/6378063631","icon":"anzhiyu-icon-weibo"},{"title":"facebook","link":"https://www.facebook.com/profile.php?id=100092208016287&sk=about","icon":"anzhiyu-icon-facebook1"},{"title":"RSS","link":"atom.xml","icon":"anzhiyu-icon-rss"}]`, Comment: "社交链接栏左侧列表 (JSON格式)", IsPublic: true},
	{Key: constant.KeyFooterSocialBarRight, Value: `[{"title":"Github","link":"https://github.com/anzhiyu-c","icon":"anzhiyu-icon-github"},{"title":"Bilibili","link":"https://space.bilibili.com/372204786","icon":"anzhiyu-icon-bilibili"},{"title":"抖音","link":"https://v.douyin.com/DwCpMEy/","icon":"anzhiyu-icon-tiktok"},{"title":"CC","link":"/copyright","icon":"anzhiyu-icon-copyright-line"}]`, Comment: "社交链接栏右侧列表 (JSON格式)", IsPublic: true},
	{Key: constant.KeyFooterProjectList, Value: `[{"title":"服务","links":[{"title":"站点地图","link":"https://blog.anheyu.com/atom.xml"},{"title":"十年之约","link":"https://foreverblog.cn/go.html"},{"title":"开往","link":"https://github.com/travellings-link/travellings"}]},{"title":"主题","links":[{"title":"文档","link":"https://dev.anheyu.com"},{"title":"源码","link":"https://github.com/anzhiyu-c/anheyu-app"},{"title":"更新日志","link":"/update"}]},{"title":"导航","links":[{"title":"小空调","link":"/air-conditioner"},{"title":"相册集","link":"/album"},{"title":"音乐馆","link":"/music"}]},{"title":"协议","links":[{"title":"隐私协议","link":"/privacy"},{"title":"Cookies","link":"/cookies"},{"title":"版权协议","link":"/copyright"}]}]`, Comment: "页脚链接列表 (JSON格式)", IsPublic: true},
	{Key: constant.KeyFooterBarLinkList, Value: `[{"link":"https://github.com/anzhiyu-c/anheyu-app","text":"主题"},{"link":"https://index.anheyu.com","text":"主页"}]`, Comment: "底部栏链接列表 (JSON格式)", IsPublic: true},
	{Key: constant.KeyIPAPI, Value: `https://api.nsmao.net/api/ipip/query`, Comment: "获取IP信息 API 地址", IsPublic: false},
	{Key: constant.KeyIPAPIToKen, Value: `QnNqOJUHuQ2dPwiWqItdr7vX3s`, Comment: "获取IP信息 API Token", IsPublic: false},
	{Key: constant.KeyPostDefaultCover, Value: ``, Comment: "文章默认封面", IsPublic: true},
	{Key: constant.KeyPostDefaultDoubleColumn, Value: "true", Comment: "文章默认双栏", IsPublic: true},
	{Key: constant.KeyPostDefaultPageSize, Value: "12", Comment: "文章默认分页大小", IsPublic: true},
	{Key: constant.KeyPostExpirationTime, Value: "365", Comment: "文章过期时间(单位天)", IsPublic: true},
	{Key: constant.KeyPostRewardEnable, Value: "true", Comment: "文章打赏功能是否启用", IsPublic: true},
	{Key: constant.KeyPostRewardWeChatQR, Value: "https://npm.elemecdn.com/anzhiyu-blog@1.1.6/img/post/common/qrcode-weichat.png", Comment: "微信打赏二维码图片URL", IsPublic: true},
	{Key: constant.KeyPostRewardAlipayQR, Value: "https://npm.elemecdn.com/anzhiyu-blog@1.1.6/img/post/common/qrcode-alipay.png", Comment: "支付宝打赏二维码图片URL", IsPublic: true},
	{Key: constant.KeyPostCodeBlockCodeMaxLines, Value: "10", Comment: "代码块最大行数（超过会折叠）", IsPublic: true},

	// --- 装备页面配置 ---
	{Key: constant.KeyPostEquipmentBannerBackground, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/27160402b1840dbc85ccf9bec2665f0d_5042209802832493877.png", Comment: "装备页面横幅背景图", IsPublic: true},
	{Key: constant.KeyPostEquipmentBannerTitle, Value: "好物", Comment: "装备页面横幅标题", IsPublic: true},
	{Key: constant.KeyPostEquipmentBannerDescription, Value: "实物装备推荐", Comment: "装备页面横幅描述", IsPublic: true},
	{Key: constant.KeyPostEquipmentBannerTip, Value: "跟 安知鱼 一起享受科技带来的乐趣", Comment: "装备页面横幅提示", IsPublic: true},
	{Key: constant.KeyPostEquipmentList, Value: `[{"title":"生产力","description":"提升自己生产效率的硬件设备","equipment_list":[{"name":"MacBook Pro 2021 16 英寸","specification":"M1 Max 64G / 1TB","description":"屏幕显示效果好、色彩准确、对比度强、性能强劲、续航优秀。可以用来开发和设计。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/b95852537e96a482957b8e5ff647ff4c_764505066454514675.png","link":"https://support.apple.com/zh-cn/111901"},{"name":"iPad 2020","specification":"深空灰 / 128G","description":"事事玩得转，买前生产力，买后爱奇艺。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/bf9219494c6da12fdfd844987a369360_291371561164874211.png","link":"https://www.apple.com.cn/ipad-10.2/"},{"name":"iPhone 15 Pro Max","specification":"白色 / 512G","description":"钛金属，坚固轻盈，Pro 得真材实料，人生第一台这么贵的手机，心疼的一批，不过确实好用，续航，大屏都很爽，缺点就是信号信号差。","image":"https://upload-bbs.miyoushe.com/upload/2023/11/06/125766904/89059eb5043ced7a38ddbe7d9141927e_6382001755098640538..png","link":"https://www.apple.com.cn/iphone-15-pro/"},{"name":"iPhone 12 mini","specification":"绿色 / 128G","description":"超瓷晶面板，玻璃背板搭配铝金属边框，曲线优美的圆角设计，mini大小正好一只手就抓住，深得我心，唯一缺点大概就是续航不够。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/ca85003734c7ae16e0885de6ddf70edf_5092364343528935349.png","link":"https://www.apple.com.cn/iphone-12/specs/"},{"name":"AirPods（第三代）","specification":"标准版","description":"第三代对比第二代提升很大，和我一样不喜欢入耳式耳机的可以入，空间音频等功能确实新颖，第一次使用有被惊艳到。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/e95d49a35c4ada2e347e148db21bd8b2_6597868370689784858.png","link":"https://www.apple.com.cn/airpods-3rd-generation/"}]},{"title":"出行","description":"用来出行的实物及设备","equipment_list":[{"name":"Apple Watch Series 8","specification":"黑色","description":"始终为我的健康放哨，深夜弹出站立提醒，不过确实有效的提高了我的运动频率，配合apple全家桶还是非常棒的产品，缺点依然是续航。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/3106e7079e4c2bacacc90d0511aa64a9_2946560183649110408.png","link":"https://www.apple.com.cn/apple-watch-series-8/"},{"name":"NATIONAL GEOGRAPHIC双肩包","specification":"黑色","description":"国家地理黑色大包，正好装下16寸 Macbook Pro，并且背起来很舒适，底部自带防雨罩也好用，各种奇怪的小口袋深得我心。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/35c080f680dc41ce62915f9f3ffa425c_7289389531712378214.png","link":"https://item.jd.com/100011269828.html"},{"name":"NATIONAL GEOGRAPHIC学生书包🎒","specification":"红白色","description":"国家地理黑色大包，冰冰🧊同款，颜值在线且实用。","image":"https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/c56fc8e461a855f8fe1b040bec559f42_4252151225488526637.png","link":"https://item.jd.com/100005889786.html"}]}]`, Comment: "装备列表配置 (JSON格式)", IsPublic: true},

	{Key: constant.KeyRecentCommentsBannerBackground, Value: "https://upload-bbs.miyoushe.com/upload/2025/09/03/125766904/ef4aa528bb9eec3b4a288d1ca2190145_4127101134334568741.jpg?x-oss-process=image/format,avif", Comment: "最近评论页面横幅背景图", IsPublic: true},
	{Key: constant.KeyRecentCommentsBannerTitle, Value: "评论", Comment: "最近评论页面横幅标题", IsPublic: true},
	{Key: constant.KeyRecentCommentsBannerDescription, Value: "最近评论", Comment: "最近评论页面横幅描述", IsPublic: true},
	{Key: constant.KeyRecentCommentsBannerTip, Value: "发表你的观点和看法，让更多人看到", Comment: "最近评论页面横幅提示", IsPublic: true},
	{Key: constant.KeyCommentLoginRequired, Value: "false", Comment: "是否开启登录后评论", IsPublic: true},
	{Key: constant.KeyCommentPageSize, Value: "10", Comment: "评论每页数量", IsPublic: true},
	{Key: constant.KeyCommentMasterTag, Value: "博主", Comment: "管理员评论专属标签文字", IsPublic: true},
	{Key: constant.KeyCommentPlaceholder, Value: "欢迎留下宝贵的建议啦～", Comment: "评论框占位文字", IsPublic: true},
	{Key: constant.KeyCommentEmojiCDN, Value: "https://npm.elemecdn.com/anzhiyu-theme-static@1.1.3/twikoo/twikoo.json", Comment: "评论表情 cdn链接", IsPublic: true},
	{Key: constant.KeyCommentBloggerEmail, Value: "me@anheyu.com", Comment: "博主邮箱，用于博主标识", IsPublic: true},
	{Key: constant.KeyCommentShowUA, Value: "true", Comment: "是否显示评论者操作系统和浏览器信息", IsPublic: true},
	{Key: constant.KeyCommentShowRegion, Value: "true", Comment: "是否显示评论者IP归属地", IsPublic: true},
	{Key: constant.KeyCommentLimitPerMinute, Value: "5", Comment: "单个IP每分钟允许提交的评论数", IsPublic: false},
	{Key: constant.KeyCommentLimitLength, Value: "10000", Comment: "单条评论最大字数", IsPublic: true},
	{Key: constant.KeyCommentForbiddenWords, Value: "习近平,空包,毛泽东,代发", Comment: "违禁词列表，逗号分隔，匹配到的评论将进入待审", IsPublic: false},
	{Key: constant.KeyCommentNotifyAdmin, Value: "false", Comment: "是否在收到评论时邮件通知博主", IsPublic: false},
	{Key: constant.KeyCommentNotifyReply, Value: "true", Comment: "是否开启评论回复邮件通知功能", IsPublic: false},
	{Key: constant.KeyPushooChannel, Value: "", Comment: "即时消息推送平台名称，支持：bark, webhook", IsPublic: false},
	{Key: constant.KeyPushooURL, Value: "", Comment: "即时消息推送URL地址 (支持模板变量)", IsPublic: false},
	{Key: constant.KeyWebhookRequestBody, Value: `{"title":"#{TITLE}","content":"#{BODY}","site_name":"#{SITE_NAME}","comment_author":"#{NICK}","comment_content":"#{COMMENT}","parent_author":"#{PARENT_NICK}","parent_content":"#{PARENT_COMMENT}","post_url":"#{POST_URL}","author_email":"#{MAIL}","author_ip":"#{IP}","time":"#{TIME}"}`, Comment: "Webhook自定义请求体模板，支持变量替换：#{TITLE}, #{BODY}, #{SITE_NAME}, #{NICK}, #{COMMENT}, #{PARENT_NICK}, #{PARENT_COMMENT}, #{POST_URL}, #{MAIL}, #{IP}, #{TIME}", IsPublic: false},
	{Key: constant.KeyWebhookHeaders, Value: "", Comment: "Webhook自定义请求头，每行一个，格式：Header-Name: Header-Value", IsPublic: false},
	{Key: constant.KeyScMailNotify, Value: "false", Comment: "是否同时通过IM和邮件2种方式通知博主 (默认仅IM)", IsPublic: false},
	{Key: constant.KeyCommentMailSubject, Value: "您在 [{{.SITE_NAME}}] 上的评论收到了新回复", Comment: "用户收到回复的邮件主题模板", IsPublic: false},
	{Key: constant.KeyCommentMailSubjectAdmin, Value: "您的博客 [{{.SITE_NAME}}] 上有新评论了", Comment: "博主收到新评论的邮件主题模板", IsPublic: false},
	{Key: constant.KeyCommentMailTemplate, Value: `<div class="flex-col page"><div class="flex-col box_3" style="display: flex;position: relative;width: 100%;height: 206px;background: #ef859d2e;top: 0;left: 0;justify-content: center;"><div class="flex-col section_1" style="background-image: url('{{.PARENT_IMG}}');position: absolute;width: 152px;height: 152px;display: flex;top: 130px;background-size: cover;border-radius: 50%;"></div></div><div class="flex-col box_4" style="margin-top: 92px;display: flex;flex-direction: column;align-items: center;"><div class="flex-col justify-between text-group_5" style="display: flex;flex-direction: column;align-items: center;margin: 0 20px;"><span class="text_1" style="font-size: 26px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #000000;line-height: 37px;text-align: center;">嘿！你在&nbsp;{{.SITE_NAME}}&nbsp;博客中收到一条新回复。</span><span class="text_2" style="font-size: 16px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #00000030;line-height: 22px;margin-top: 21px;text-align: center;">你之前的评论&nbsp;在&nbsp;{{.SITE_NAME}} 博客中收到来自&nbsp;{{.NICK}}&nbsp;的回复</span></div><div class="flex-row box_2" style="margin: 0 20px;min-height: 128px;background: #F7F7F7;border-radius: 12px;margin-top: 34px;display: flex;flex-direction: column;align-items: flex-start;padding: 32px 16px;width: calc(100% - 40px);"><div class="flex-col justify-between text-wrapper_4" style="display: flex;flex-direction: column;margin-left: 30px;margin-bottom: 16px;"><span class="text_3" style="height: 22px;font-size: 16px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #C5343E;line-height: 22px;">{{.PARENT_NICK}}</span><span class="text_4" style="margin-top: 6px;margin-right: 22px;font-size: 16px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #000000;line-height: 22px;">{{.PARENT_COMMENT}}</span></div><hr style="display: flex;position: relative;border: 1px dashed #ef859d2e;box-sizing: content-box;height: 0px;overflow: visible;width: 100%;"><div class="flex-col justify-between text-wrapper_4" style="display: flex;flex-direction: column;margin-left: 30px;"><hr><span class="text_3" style="height: 22px;font-size: 16px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #C5343E;line-height: 22px;">{{.NICK}}</span><span class="text_4" style="margin-top: 6px;margin-right: 22px;font-size: 16px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #000000;line-height: 22px;">{{.COMMENT}}</span></div><a class="flex-col text-wrapper_2" style="min-width: 106px;height: 38px;background: #ef859d38;border-radius: 32px;display: flex;align-items: center;justify-content: center;text-decoration: none;margin: auto;margin-top: 32px;" href="{{.POST_URL}}"><span class="text_5" style="color: #DB214B;">查看详情</span></a></div><div class="flex-col justify-between text-group_6" style="display: flex;flex-direction: column;align-items: center;margin-top: 34px;"><span class="text_6" style="height: 17px;font-size: 12px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #00000045;line-height: 17px;">此邮件由评论服务自动发出，直接回复无效。</span><a class="text_7" style="height: 17px;font-size: 12px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #DB214B;line-height: 17px;margin-top: 6px;text-decoration: none;" href="{{.SITE_URL}}">前往博客</a></div></div></div>`, Comment: "用户收到回复的邮件HTML模板", IsPublic: false},
	{Key: constant.KeyCommentMailTemplateAdmin, Value: `<div class="flex-col page"><div class="flex-col box_3" style="display: flex;position: relative;width: 100%;height: 206px;background: #ef859d2e;top: 0;left: 0;justify-content: center;"><div class="flex-col section_1" style="background-image: url('{{.IMG}}');position: absolute;width: 152px;height: 152px;display: flex;top: 130px;background-size: cover;border-radius: 50%;"></div></div><div class="flex-col box_4" style="margin-top: 92px;display: flex;flex-direction: column;align-items: center;"><div class="flex-col justify-between text-group_5" style="display: flex;flex-direction: column;align-items: center;margin: 0 20px;"><span class="text_1" style="font-size: 26px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #000000;line-height: 37px;text-align: center;">嘿！你的&nbsp;{{.SITE_NAME}}&nbsp;博客中收到一条新消息。</span></div><div class="flex-row box_2" style="margin: 0 20px;min-height: 128px;background: #F7F7F7;border-radius: 12px;margin-top: 34px;display: flex;flex-direction: column;align-items: flex-start;padding: 32px 16px;"><div class="flex-col justify-between text-wrapper_4" style="display: flex;flex-direction: column;margin-left: 30px;"><hr><span class="text_3" style="height: 22px;font-size: 16px;font-family: PingFang-SC-Bold, PingFang-SC;font-weight: bold;color: #C5343E;line-height: 22px;">{{.NICK}} ({{.MAIL}}, {{.IP}})</span><span class="text_4" style="margin-top: 6px;margin-right: 22px;font-size: 16px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #000000;line-height: 22px;">{{.COMMENT}}</span></div><a class="flex-col text-wrapper_2" style="min-width: 106px;height: 38px;background: #ef859d38;border-radius: 32px;display: flex;align-items: center;justify-content: center;text-decoration: none;margin: auto;margin-top: 32px;" href="{{.POST_URL}}"><span class="text_5" style="color: #DB214B;">查看详情</span></a></div><div class="flex-col justify-between text-group_6" style="display: flex;flex-direction: column;align-items: center;margin-top: 34px;"><span class="text_6" style="height: 17px;font-size: 12px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #00000045;line-height: 17px;">此邮件由评论服务自动发出，直接回复无效。</span><a class="text_7" style="height: 17px;font-size: 12px;font-family: PingFangSC-Regular, PingFang SC;font-weight: 400;color: #DB214B;line-height: 17px;margin-top: 6px;text-decoration: none;" href="{{.SITE_URL}}">前往博客</a></div></div></div>`, Comment: "博主收到新评论的邮件HTML模板", IsPublic: false},

	{Key: constant.KeySidebarAuthorEnable, Value: "true", Comment: "是否启用侧边栏作者卡片", IsPublic: true},
	{Key: constant.KeySidebarAuthorDescription, Value: `<div style="line-height:1.38;margin:0.6rem 0;text-align:justify;color:rgba(255, 255, 255, 0.8);">这有关于<b style="color:#fff">产品、设计、开发</b>相关的问题和看法，还有<b style="color:#fff">文章翻译</b>和<b style="color:#fff">分享</b>。</div><div style="line-height:1.38;margin:0.6rem 0;text-align:justify;color:rgba(255, 255, 255, 0.8);">相信你可以在这里找到对你有用的<b style="color:#fff">知识</b>和<b style="color:#fff">教程</b>。</div>`, Comment: "作者卡片描述 (HTML)", IsPublic: true},
	{Key: constant.KeySidebarAuthorStatusImg, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/04/125766904/e3433dc6f4f78a9257060115e339f018_1105042150723011388.png?x-oss-process=image/format,avif", Comment: "作者卡片状态图片URL", IsPublic: true},
	{Key: constant.KeySidebarAuthorSkills, Value: `["🤖️ 数码科技爱好者","🔍 分享与热心帮助","🏠 智能家居小能手","🔨 设计开发一条龙","🤝 专修交互与设计","🏃 脚踏实地行动派","🧱 团队小组发动机","💢 壮汉人狠话不多"]`, Comment: "作者卡片技能列表 (JSON数组)", IsPublic: true},
	{Key: constant.KeySidebarAuthorSocial, Value: `{"Github":{"link":"https://github.com/anzhiyu-c","icon":"anzhiyu-icon-github"},"BiliBili":{"link":"https://space.bilibili.com/372204786","icon":"anzhiyu-icon-bilibili"}}`, Comment: "作者卡片社交链接 (JSON对象)", IsPublic: true},
	{Key: constant.KeySidebarWechatEnable, Value: "true", Comment: "是否启用侧边栏微信卡片", IsPublic: true},
	{Key: constant.KeySidebarWechatFace, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/06/125766904/cf92d0f791458c288c7e308e9e8df1f5_5078983739960715024.png", Comment: "微信卡片正面图片URL", IsPublic: true},
	{Key: constant.KeySidebarWechatBackFace, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/06/125766904/ed37b3b3c45bccaa11afa7c538e20b58_8343041924448947243.png?x-oss-process=image/format,avif", Comment: "微信卡片背面图片URL", IsPublic: true},
	{Key: constant.KeySidebarWechatBlurBackground, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/06/125766904/92d74a9ef6ceb9465fec923e90dff04d_3079701216996731938.png", Comment: "微信卡片图片URL", IsPublic: true},
	{Key: constant.KeySidebarTagsEnable, Value: "true", Comment: "是否启用侧边栏标签卡片", IsPublic: true},
	{Key: constant.KeySidebarTagsHighlight, Value: "[]", Comment: "侧边栏高亮标签", IsPublic: true},
	{Key: constant.KeySidebarSiteInfoRuntimeEnable, Value: "true", Comment: "是否在侧边栏显示建站天数", IsPublic: true},
	{Key: constant.KeySidebarSiteInfoTotalPostCount, Value: "0", Comment: "侧边栏网站信息-文章总数 (此值由系统自动更新)", IsPublic: true},
	{Key: constant.KeySidebarSiteInfoTotalWordCount, Value: "0", Comment: "侧边栏网站信息-全站总字数 (此值由系统自动更新)", IsPublic: true},
	{Key: constant.KeySidebarArchiveCount, Value: "0", Comment: "侧边栏归档个数", IsPublic: true},

	{Key: constant.KeyFriendLinkApplyCondition, Value: `["我已添加 <b>安知鱼</b> 博客的友情链接","我的链接主体为 <b>个人</b>，网站类型为<b>博客</b>","我的网站现在可以在中国大陆区域正常访问","网站内容符合中国大陆法律法规","我的网站可以在1分钟内加载完成首屏"]`, Comment: "申请友链条件 (JSON数组格式，用于动态生成勾选框)", IsPublic: true},
	{Key: constant.KeyFriendLinkApplyCustomCode, Value: "", Comment: "申请友链自定义代码", IsPublic: true},
	{Key: constant.KeyFriendLinkDefaultCategory, Value: "2", Comment: "友链默认分类", IsPublic: true},

	// --- 内部或敏感配置 ---
	{Key: constant.KeyJWTSecret, Value: "", Comment: "JWT密钥", IsPublic: false},
	{Key: constant.KeyLocalFileSigningSecret, Value: "", Comment: "本地文件签名密钥", IsPublic: false},
	{Key: constant.KeyResetPasswordSubject, Value: "【{{.AppName}}】重置您的账户密码", Comment: "重置密码邮件主题模板", IsPublic: false},
	{Key: constant.KeyResetPasswordTemplate, Value: `<!DOCTYPE html><html><head><title>重置密码</title></head><body><p>您好, {{.Nickname}}！</p><p>您正在请求重置您在 <strong>{{.AppName}}</strong> 的账户密码。</p><p>请点击以下链接以完成重置（此链接24小时内有效）：</p><p><a href="{{.ResetLink}}">重置我的密码</a></p><p>如果链接无法点击，请将其复制到浏览器地址栏中打开。</p><p>如果您没有请求重置密码，请忽略此邮件。</p><br/><p>感谢, <br/>{{.AppName}} 团队</p></body></html>`, Comment: "重置密码邮件HTML模板", IsPublic: false},
	{Key: constant.KeyActivateAccountSubject, Value: "【{{.AppName}}】激活您的账户", Comment: "用户激活邮件主题模板", IsPublic: false},
	{Key: constant.KeyActivateAccountTemplate, Value: `<!DOCTYPE html><html><head><title>激活您的账户</title></head><body><p>您好, {{.Nickname}}！</p><p>欢迎注册 <strong>{{.AppName}}</strong>！</p><p>请点击以下链接以激活您的账户（此链接24小时内有效）：</p><p><a href="{{.ActivateLink}}">激活我的账户</a></p><p>如果链接无法点击，请将其复制到浏览器地址栏中打开。</p><p>如果您并未注册，请忽略此邮件。</p><br/><p>感谢, <br/>{{.AppName}} 团队</p></body></html>`, Comment: "用户激活邮件HTML模板", IsPublic: false},
	{Key: constant.KeyEnableUserActivation, Value: "false", Comment: "是否开启新用户邮箱激活功能 (true/false)", IsPublic: false},
	{Key: constant.KeySmtpHost, Value: "smtp.qq.com", Comment: "SMTP 服务器地址", IsPublic: false},
	{Key: constant.KeySmtpPort, Value: "587", Comment: "SMTP 服务器端口 (587 for STARTTLS, 465 for SSL)", IsPublic: false},
	{Key: constant.KeySmtpUsername, Value: "user@example.com", Comment: "SMTP 登录用户名", IsPublic: false},
	{Key: constant.KeySmtpPassword, Value: "", Comment: "SMTP 登录密码", IsPublic: false},
	{Key: constant.KeySmtpSenderName, Value: "安和鱼", Comment: "邮件发送人名称", IsPublic: false},
	{Key: constant.KeySmtpSenderEmail, Value: "user@example.com", Comment: "邮件发送人邮箱地址", IsPublic: false},
	{Key: constant.KeySmtpReplyToEmail, Value: "", Comment: "回信邮箱地址", IsPublic: false},
	{Key: constant.KeySmtpForceSSL, Value: "false", Comment: "是否强制使用 SSL (设为true通常配合465端口)", IsPublic: false},

	// --- 关于页面配置 ---
	{Key: constant.KeyAboutPageName, Value: "安知鱼", Comment: "关于页面姓名", IsPublic: true},
	{Key: constant.KeyAboutPageDescription, Value: "是一名 前端工程师、学生、独立开发者、博主", Comment: "关于页面描述", IsPublic: true},
	{Key: constant.KeyAboutPageAvatarImg, Value: "https://npm.elemecdn.com/anzhiyu-blog-static@1.0.4/img/avatar.jpg", Comment: "关于页面头像图片URL", IsPublic: true},
	{Key: constant.KeyAboutPageSubtitle, Value: "生活明朗，万物可爱✨", Comment: "关于页面副标题", IsPublic: true},
	{Key: constant.KeyAboutPageAvatarSkillsLeft, Value: `["🤖️ 数码科技爱好者","🔍 分享与热心帮助","🏠 智能家居小能手","🔨 设计开发一条龙"]`, Comment: "头像左侧技能标签列表 (JSON数组)", IsPublic: true},
	{Key: constant.KeyAboutPageAvatarSkillsRight, Value: `["专修交互与设计 🤝","脚踏实地行动派 🏃","团队小组发动机 🧱","壮汉人狠话不多 💢"]`, Comment: "头像右侧技能标签列表 (JSON数组)", IsPublic: true},
	{Key: constant.KeyAboutPageAboutSiteTips, Value: `{"tips":"追求","title1":"源于","title2":"热爱而去 感受","word":["学习","生活","程序","体验"]}`, Comment: "关于网站提示配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageStatisticsBackground, Value: "https://upload-bbs.miyoushe.com/upload/2025/08/20/125766904/0d61be5d781e63642743883eb5580024_4597572337700501322.png", Comment: "个人信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageMap, Value: `{"title":"我现在住在","strengthenTitle":"中国，长沙市","background":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/29da8e2cd0e5f5e5bb50d2110ef71575_4355468272920245477.png","backgroundDark":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/d8d89f53ce2e7b368a0ac03092be3f78_3149317008469616077.png"}`, Comment: "地图信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageSelfInfo, Value: `{"tips1":"生于","contentYear":"2002","tips2":"湖南信息学院","content2":"软件工程","tips3":"现在职业","content3":"软件工程师👨"}`, Comment: "个人信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPagePersonalities, Value: `{"tips":"性格","authorName":"执政官","personalityType":"ESFJ-A","personalityTypeColor":"#ac899c","personalityImg":"https://npm.elemecdn.com/anzhiyu-blog@2.0.8/img/svg/ESFJ-A.svg","nameUrl":"https://www.16personalities.com/ch/esfj-%E4%BA%BA%E6%A0%BC","photoUrl":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/c4aa8dcbeef6362c65e0266ab9dd5b19_7893582960672134962.png?x-oss-process=image/format,avif"}`, Comment: "性格信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageMaxim, Value: `{"tips":"座右铭","top":"生活明朗，","bottom":"万物可爱。"}`, Comment: "格言配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageBuff, Value: `{"tips":"特长","top":"脑回路新奇的 酸菜鱼","bottom":"二次元指数 MAX"}`, Comment: "增益配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageGame, Value: `{"tips":"爱好游戏","title":"原神","uid":"UID: 125766904","background":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/df170ee157232de18d1a990e72333f65_3745939416973154749.png?x-oss-process=image/format,avif"}`, Comment: "游戏信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageComic, Value: `{"tips":"爱好番剧","title":"追番","list":[{"name":"约定的梦幻岛","cover":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/40398029fd438c90395e3f6363be9210_3056370406171442679.png?x-oss-process=image/format,avif","href":"https://www.bilibili.com/bangumi/media/md5267750/?spm_id_from=666.25.b_6d656469615f6d6f64756c65.1"},{"name":"咒术回战","cover":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/9e8c4fd98c7d2c58ba9f58074f6b31d4_8434426529088986040.png?x-oss-process=image/format,avif","href":"https://www.bilibili.com/bangumi/media/md28229899/?spm_id_from=666.25.b_6d656469615f6d6f64756c65.1"},{"name":"紫罗兰永恒花园","cover":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/c654e3823523369aa9ac3f2d9ac14471_8582606285447891616.png?x-oss-process=image/format,avif","href":"https://www.bilibili.com/bangumi/media/md8892/?spm_id_from=666.25.b_6d656469615f6d6f64756c65.1"},{"name":"鬼灭之刃","cover":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/3ce8719fa9414801fb81654c7cee7549_4007505277882210341.png?x-oss-process=image/format,avif","href":"https://www.bilibili.com/bangumi/media/md22718131/?spm_id_from=666.25.b_6d656469615f6d6f64756c65.1"},{"name":"JOJO的奇妙冒险 黄金之风","cover":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/ea1fc16baccef3f3d04e1dced0a8eb39_6591444362443588368.png?x-oss-process=image/format,avif","href":"https://www.bilibili.com/bangumi/media/md135652/?spm_id_from=666.25.b_6d656469615f6d6f64756c65.1"}]}`, Comment: "漫画信息配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageLike, Value: `{"tips":"关注偏好","title":"数码科技","bottom":"手机、电脑软硬件","background":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/b30e2d6a8cfaa36b8110b5034080adf6_5639323093964199346.png?x-oss-process=image/format,avif"}`, Comment: "喜欢的技术配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageMusic, Value: `{"tips":"音乐偏好","title":"许嵩、民谣、华语流行","link":"/music","background":"https://p2.music.126.net/Mrg1i7DwcwjWBvQPIMt_Mg==/79164837213438.jpg"}`, Comment: "音乐配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageCareers, Value: `{"tips":"生涯","title":"无限进步","img":"https://upload-bbs.miyoushe.com/upload/2025/08/21/125766904/a0c75864c723d53d3b9967e8c19a99c6_2075143858961311655.png?x-oss-process=image/format,avif","list":[{"desc":"EDU,软件工程专业","color":"#357ef5"}]}`, Comment: "职业经历配置 (JSON格式)", IsPublic: true},
	{Key: constant.KeyAboutPageSkillsTips, Value: `{"tips":"技能","title":"开启创造力"}`, Comment: "技能信息提示配置 (JSON格式)", IsPublic: true},

	// --- 音乐播放器配置 ---
	{Key: constant.KeyMusicPlayerEnable, Value: "false", Comment: "是否启用音乐播放器功能 (true/false)", IsPublic: true},
	{Key: constant.KeyMusicPlayerPlaylistID, Value: "8152976493", Comment: "音乐播放器播放列表ID (网易云歌单ID)", IsPublic: true},
	{Key: constant.KeyMusicPlayerCustomPlaylist, Value: "", Comment: "自定义音乐歌单JSON文件链接", IsPublic: true},
}

// AllUserGroups 是所有默认用户组的"单一事实来源"
var AllUserGroups = []UserGroupDefinition{
	{
		ID:          1,
		Name:        "管理员",
		Description: "拥有所有权限的系统管理员",
		Permissions: model.NewBoolset(model.PermissionAdmin, model.PermissionCreateShare, model.PermissionAccessShare, model.PermissionUploadFile, model.PermissionDeleteFile),
		MaxStorage:  0, // 0 代表无限容量
		SpeedLimit:  0,
		Settings:    model.GroupSettings{SourceBatch: 100, PolicyOrdering: []uint{1}, RedirectedSource: true},
	},
	{
		ID:          2,
		Name:        "普通用户",
		Description: "标准用户组，拥有基本上传和分享权限",
		Permissions: model.NewBoolset(model.PermissionCreateShare, model.PermissionAccessShare, model.PermissionUploadFile),
		MaxStorage:  5 * 1024 * 1024 * 1024, // 默认 5 GB
		SpeedLimit:  0,
		Settings:    model.GroupSettings{SourceBatch: 10, PolicyOrdering: []uint{1}, RedirectedSource: true},
	},
	{
		ID:          3,
		Name:        "匿名用户组",
		Description: "未登录用户或游客，仅能访问公开的分享",
		Permissions: model.NewBoolset(model.PermissionAccessShare),
		MaxStorage:  0,
		SpeedLimit:  0,
		Settings:    model.GroupSettings{SourceBatch: 0, PolicyOrdering: []uint{}, RedirectedSource: false},
	},
}
