# docker pull image 
### A small golang tool to pull docker images use http 
 <br>
<br>

> [!IMPORTANT]
>
> **旧版本代码已不再支持新版的 manifest version，现已弃用**
>
> **新的代码只是对 [containers/skopeo](https://github.com/containers/skopeo) 进行了简单封装**
>
> **如果您需要功能全面的镜像下载工具，请参考 [containers/skopeo](https://github.com/containers/skopeo)**
>




### 1)&emsp;start up
 - build
```
  go build -x -o gopull 
```


 - or run
```
  go run main.go help
```


<br>

### 2)&emsp;下载镜像到tar文件
```
  ./gopull download redis
```
  
### 3)&emsp;下载Digest格式镜像到tar文件(需要指定 -t 参数)
```
  ./gopull download sha256:c35af3bbcef51a62c8bae5a9a563c6f1b60d7ebaea4cb5a3ccbcc157580ae098 -t redis:custom_tag
```

### 4)&emsp; 导入下载的tar镜像
```
  # docker导入
  docker load -i redis.tar
  
  # ctr导入
  ctr image import redis.tar
```


### 5)&emsp;拉取镜像到docker
```
  ./gopull pull redis
```

### 6)&emsp;拉取Digest格式镜像到docker(需要指定 -t 参数)
```
  ./gopull pull sha256:c35af3bbcef51a62c8bae5a9a563c6f1b60d7ebaea4cb5a3ccbcc157580ae098 -t redis:custom_tag
```

### 7)&emsp;推送镜像到镜像仓库
```
  ./gopull push redis 
```

### 8)&emsp;推送镜像到镜像仓库并重名
```
  ./gopull push redis  -t your_registry/your_repository:your_tag
```

### 9)&emsp;login | logout
```
  ./gopull login docker.io 
  ./gopull logout docker.io
```

### 10)&emsp;获取镜像详情
```
  ./gopull inspect redis
```





