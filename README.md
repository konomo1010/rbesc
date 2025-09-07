# rbesc

批量远端执行shell脚本  ssh 连接 **超时时间默认设定为 5 秒**

# 文件说明

```
./
  | -- rbesc.exe       win使用命令
  | -- rbesc           linux使用命令
./config/
    | -- command.sh          远端执行的shell命令
    | -- hosts_config.csv    配置 远端主机ip,port,user,password
    | -- run.sh              源码编译运行
```

# 使用说明

* 在 hosts_config.csv 文件中写 远端主机ip,port,user,password。<font color="red" size="4">顺序不能乱</font>
* 在 command.sh 写 想在远端机器上执行的shell命令。<font color="red" size="4">需要确保远端linux机器该命令存在且有可执行权限</font>

```shell
win  :    执行 rbesc.exe
linux :   执行 rbesc 

Usage of ./rbesc:
  -d string
    	指定 csv 目录(短参数-d，长参数--dir)
  -dir string
    	指定 csv 目录(短参数-d，长参数--dir)
  -f string
    	指定 csv 文件(短参数-f，长参数--file)
  -file string
    	指定 csv 文件(短参数-f，长参数--file)
  -l	是否将 日志标准输出到终端 (短参数-l 代表 是)
  -y	是否 取消 交互式确认执行 (短参数-y 代表 是)


# 指定 csv 文件
./rbesc  -f  ../../a.csv -y -l

# 指定 csv 目录
./rbesc  -d  ../../csvdir -y -l
```