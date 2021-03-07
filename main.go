package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"sync"

	"encoding/csv"
	"fmt"
	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os"
	"time"

)

type Host struct {
	ip string
	port string
	user string
	password string
}

type PathFile struct {
	shellScript  string
	hostsConfig  string
	remoteSaveShellScriptPath  string
	logfile string
}

/*获取当前文件执行的路径*/
func GetCurPath() string {
	file, _ := exec.LookPath(os.Args[0])
	//得到全路径，比如在windows下E:\\golang\\test\\a.exe
	path, _ := filepath.Abs(file)
	rst := filepath.Dir(path)
	return rst
}

func main() {
	fmt.Println("starting ... ... ...")
	var pf PathFile
	b := make([]byte, 1)

	curPath := GetCurPath()

	pf.shellScript = curPath + "/config/command.sh"
	pf.hostsConfig = curPath + "/config/hosts_config.csv"
	pf.remoteSaveShellScriptPath = "/tmp/command.sh"

	// 执行前目录检查
	if pe,_ := PathExists(curPath + "/config");!pe {
		log.Fatal(curPath + "/config  目录不存在， 请创建")
	}

	// 执行前目录文件检查
	if judge,_ := PathExists(pf.shellScript); !judge {
		log.Fatalf("Eorror : 请检查当前目录下 %s 文件是否存在 ... ...",pf.shellScript)
		os.Exit(100)
	}

	// 读取 csv 远端机器信息
	file, err := os.Open(pf.hostsConfig)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if reader == nil {
		log.Fatalln("NewReader return nil, file:", file)
	}

	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalln(err)
	}

	// 等待所有goroutine结束后才退出main
	gw := sync.WaitGroup{}
	// 批量在远端机器上执行command.sh脚本。
	//BatchExec(records, pf, curPath, gw)
	for i,v := range records {
		if i != 0 {
			host := HostSliceToStruct(v)
			if pe,_ := PathExists(curPath + "/logs");!pe {
				os.Mkdir(curPath + "/logs", 0755)
			}  //  logs 目录是否存在，不存在则创建
			pf.logfile = curPath + "/logs/" + host.ip + ".log"
			gw.Add(1)
			go ShellScriptRemoteExec(host, pf, &gw)
		}
	}
	gw.Wait() // 阻塞，直到WaitGroup中的计数器为0
	fmt.Printf("Press any key to exit...")
	os.Stdin.Read(b)
}

func ShellScriptRemoteExec(host Host, pf PathFile, group *sync.WaitGroup)  {
	defer group.Done() //defer标记当前函数作用域执行结束后 释放一个计数器
	// 日志文件准备
	file, _ := os.OpenFile(pf.logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 755)
	logger := log.New(file, "", log.LstdFlags)
	//logger.SetPrefix("Test- ") // 设置日志前缀
	logger.SetFlags(log.LstdFlags | log.Lshortfile)

	// 建立 SSH connection
	cfg := &ssh.ClientConfig{
		User: host.user,
		Auth: []ssh.AuthMethod{ssh.Password(host.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 5 * time.Second, // time.Duration
	}

	sshClient, err := ssh.Dial("tcp", host.ip+":"+host.port, cfg)
	if err != nil {
		logger.Println("SSH 连接 error: ", err.Error())
		fmt.Printf("%s              fail\n", host.ip)
		return
	}

	client, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		logger.Println("Error creating new SSH session from existing connection", err)
		fmt.Printf("%s              fail\n", host.ip)
		return
	};defer client.Close()

	// scp 脚本
	buff, _ := ioutil.ReadFile(pf.shellScript)
	r := bytes.NewReader(buff)

	err = client.CopyFile(r, pf.remoteSaveShellScriptPath, "0777")
	if err != nil {
		logger.Println("Error : 拷贝脚本失败 ... ...  ", err)
		fmt.Printf("%s              fail\n", host.ip)
		return
	}

	// 建立新会话
	session, err := sshClient.NewSession()
	if err != nil {
		logger.Println("new session error: %s", err.Error())
		fmt.Printf("%s              fail\n", host.ip)
		return
	};defer session.Close()

	// 执行command.sh脚本
	var b bytes.Buffer
	session.Stdout = &b
	err = session.Run(pf.remoteSaveShellScriptPath);
	if err != nil {
		logger.Printf("\n%s",b.String())   // session output
		logger.Println(err.Error())
		logger.Println("执行脚本失败 ... ... ...")
		fmt.Printf("%s              fail\n", host.ip)
	}else {
		logger.Printf("\n%s",b.String())  // session output
		fmt.Printf("%s              success\n", host.ip)
	}
}

func HostSliceToStruct(sli []string) Host {
	host := Host{}
	host.ip = sli[0]
	host.port = sli[1]
	host.user = sli[2]
	host.password = sli[3]
	return host
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}





