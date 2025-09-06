package main

import (
	"bytes"
	"context"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	ip       string
	port     string
	user     string
	password string
	env      string
}

type PathFile struct {
	shellScript               string
	hostsConfig               string
	remoteSaveShellScriptPath string
	logfile                   string
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
	pf.remoteSaveShellScriptPath = "/tmp"

	// 执行前目录检查
	if pe, _ := PathExists(curPath + "/config"); !pe {
		log.Fatal(curPath + "/config  目录不存在， 请创建")
	}

	// 执行前目录文件检查
	if judge, _ := PathExists(pf.shellScript); !judge {
		log.Fatalf("Eorror : 请检查当前目录下 %s 文件是否存在 ... ...", pf.shellScript)
		os.Exit(100)
	}

	// 检索 config 目录下 csv 远端机器信息
	dir_filelist, drierr := ioutil.ReadDir(curPath + "/config/")
	if drierr != nil {
		log.Fatalln(drierr.Error())
	}

	for _, dirfile := range dir_filelist {
		if judge, _ := regexp.MatchString(".csv$", dirfile.Name()); judge {
			file_ext := path.Ext(dirfile.Name())
			file_name := strings.Split(dirfile.Name(), file_ext)[0]
			logs_path := curPath + "/logs/" + file_name
			if EndJudge(curPath + "/config/" + dirfile.Name()) {
				file, err := os.Open(curPath + "/config/" + dirfile.Name())
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
				for i, v := range records {
					if i != 0 {
						host := HostSliceToStruct(v)
						if pe, _ := PathExists(logs_path); !pe {
							os.MkdirAll(logs_path, 0755)
						} //  logs 目录是否存在，不存在则创建
						pf.logfile = logs_path + "/" + host.ip + ".log"
						gw.Add(1)
						go ShellScriptRemoteExec(host, pf, &gw)
					}
				}
				gw.Wait() // 阻塞，直到WaitGroup中的计数器为0
				fmt.Printf("Press any key to exit...")
				os.Stdin.Read(b)
			}
		} else {
			log.Fatalf("Eorror : 请检查config目录下 csv 文件是否存在 ... ...")
			os.Exit(100)
		}
	}

}

func EndJudge(stage string) bool {
	var continuerFlag string
	for {
		fmt.Printf("now : " + stage + "    do ? [yes | no | pass]: ")
		_, e := fmt.Scanln(&continuerFlag)
		if e != nil {
			continue
		}
		switch continuerFlag {
		case "yes":
			return true
		case "no":
			os.Exit(100)
		case "pass":
			return false
		default:
			continue
		}
	}
}

func ShellScriptRemoteExec(host Host, pf PathFile, group *sync.WaitGroup) {
	env_slice := strings.Split(host.env, ",")
	timer := strconv.FormatInt(time.Now().Unix(), 10)
	tmpscript := pf.remoteSaveShellScriptPath + "/" + timer

	defer group.Done() //defer标记当前函数作用域执行结束后 释放一个计数器
	// 日志文件准备
	file, _ := os.OpenFile(pf.logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 755)
	logger := log.New(file, "", log.LstdFlags)
	//logger.SetPrefix("Test- ") // 设置日志前缀
	logger.SetFlags(log.LstdFlags | log.Lshortfile)

	// 建立 SSH connection
	cfg := &ssh.ClientConfig{
		User:            host.user,
		Auth:            []ssh.AuthMethod{ssh.Password(host.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second, // time.Duration
	}

	sshClt, err := ssh.Dial("tcp", host.ip+":"+host.port, cfg)
	if err != nil {
		logger.Println("SSH 连接 error: ", err.Error())
		fmt.Printf("%s              fail\n", host.ip)
		return
	}

	client, err := scp.NewClientBySSH(sshClt)
	if err != nil {
		logger.Println("Error creating new SSH session from existing connection", err)
		fmt.Printf("%s              fail\n", host.ip)
		return
	}
	defer client.Close()

	// scp 脚本
	buff, _ := ioutil.ReadFile(pf.shellScript)
	r := bytes.NewReader(buff)

	ctx := context.Background()
	err = client.CopyFile(ctx, r, tmpscript, "0777")
	if err != nil {
		logger.Println("Error : 拷贝脚本失败 ... ...  ", err)
		fmt.Printf("%s              fail\n", host.ip)
		return
	}

	// 建立新会话
	session, err := sshClt.NewSession()
	if err != nil {
		logger.Println("new session error: %s", err.Error())
		fmt.Printf("%s              fail\n", host.ip)
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // 回显（0禁用，1启动）
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud 波特（Baud）即调制速率
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud 波特（Baud）即调制速率
	}

	if err = session.RequestPty("xterm", 80, 40, modes); err != nil {
		logger.Println("Error : " + err.Error()) // session output
		fmt.Printf("%s              fail\n", host.ip)
		return
	}

	// 执行command.sh脚本
	var export_env string
	for _, ele_v := range env_slice {
		export_env += "export " + ele_v + ";"
	}

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	err = session.Run(export_env + tmpscript + ";rm -fr " + tmpscript)

	LogAct(err, stdoutBuf.String(), logger, host)

}

func LogAct(err error, info interface{}, logger *log.Logger, host Host) {
	if err == nil {
		logger.Printf("session output :\n%s", info) // session output
		fmt.Printf("%s              done\n", host.ip)
	} else {
		logger.Printf("\n%s", info) // session output
		logger.Println(err.Error())
		logger.Println("执行脚本失败 ... ... ...")
		fmt.Printf("%s              fail\n", host.ip)
	}

}

func HostSliceToStruct(sli []string) Host {
	host := Host{}
	host.ip = sli[0]
	host.port = sli[1]
	host.user = sli[2]
	host.password = sli[3]
	host.env = sli[4]
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
