# 部署

安装好 `docker` 和 `docker-compose` 后，把项目中的 `docker-compose.yml` 文件复制到服务器中，然后执行以下命令：

```bash
docker-compose up -d
```

## 更新

```bash
docker-compose pull
```

部署完了以后就可以访问了，默认的访问地址是：`http://localhost:8091`，当然你也可以使用nginx反向代理出来一个域名。

部署完后第一时间请访问 `/login` 路径进行注册，第一个注册的用户就是管理员。

## 其他（有点懒，待探索）
