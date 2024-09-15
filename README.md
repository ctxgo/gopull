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

### 2)&emsp;Pull docker images and generate a tar archive on a machine without docker
```
  ./gopull download redis
```
  
### 3)&emsp;Pull docker images whith Digest
```
  ./gopull download sha256:c35af3bbcef51a62c8bae5a9a563c6f1b60d7ebaea4cb5a3ccbcc157580ae098 -t redis:custom_tag
```

### 4)&emsp;login | logout
```
  ./gopull login docker.io 
  ./gopull logout
```

### 4)&emsp;Get image details
```
  ./gopull inspect redis
```


### 6)&emsp; Import the downloaded image
```
  # docker导入
  docker load -i redis.tar
  
  # ctr导入
  ctr image import redis.tar
```


