package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"flag"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

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
	var (
		dir_csv  string
		file_csv string
	)

	// 绑定短参数和长参数到同一个变量
	flag.StringVar(&dir_csv, "d", "", "指定 csv 目录(短参数-d，长参数--dir)")
	flag.StringVar(&dir_csv, "dir", dir_csv, "指定 csv 目录(短参数-d，长参数--dir)") // 复用默认值

	flag.StringVar(&file_csv, "f", "", "指定 csv 文件(短参数-f，长参数--file)")
	flag.StringVar(&file_csv, "file", file_csv, "指定 csv 文件(短参数-f，长参数--file)")

	interac_confirm := flag.Bool("y", false, "是否 取消 交互式确认执行 (短参数-y 代表 是)")
	log_confirm := flag.Bool("l", false, "是否将 日志标准输出到终端 (短参数-l 代表 是)")

	// 解析参数
	flag.Parse()

	var pf PathFile
	var csvdir string
	var csvfiles []string
	exeflg := false

	curPath := GetCurPath()
	pf.shellScript = curPath + "/config/command.sh"
	pf.remoteSaveShellScriptPath = "/tmp"

	//b := make([]byte, 1)

	if dir_csv != "" {
		csvdir = dir_csv
		csvfiles = DelCsvDir(dir_csv)
	} else if file_csv != "" {
		csvfiles = append(csvfiles, file_csv)
	} else {
		csvdir = curPath + "/config/"
		csvfiles = DelCsvDir(csvdir)
	}

	exe_csv_file_count := 0

	Pre_execution_inspection(curPath, pf, csvfiles)

	for _, csvfile := range csvfiles {
		LogStdout("starting " + csvfile + "... ... ...")
		filename_with_ext := filepath.Base(csvfile)
		file_ext := path.Ext(filename_with_ext)
		file_name := strings.Split(filename_with_ext, file_ext)[0]
		logs_path := curPath + "/logs/" + file_name

		if *interac_confirm {
			exeflg = true
		} else if EndJudge(csvfile) {
			exeflg = true
		}

		if exeflg {
			ExecWithCSV(csvfile, logs_path, pf, *log_confirm)
			exe_csv_file_count++
		}
	}

	if exe_csv_file_count == 0 {
		LogStdout("Error : 请检查 " + csvdir + " 目录下 csv 文件是否存在 ... ...")
		os.Exit(100)
	}

}

// 处理csvdir
func DelCsvDir(csvdir string) []string {
	dir_filelist, drierr := ioutil.ReadDir(csvdir)
	var csvfiles []string
	if drierr != nil {
		LogStdout(drierr.Error())
	}

	for _, dirfile := range dir_filelist {
		if judge, _ := regexp.MatchString(".csv$", dirfile.Name()); judge {
			csvfiles = append(csvfiles, csvdir+dirfile.Name())
		}
	}

	return csvfiles
}

// 按csv内容执行
func ExecWithCSV(csvfile string, logs_path string, pf PathFile, log_confirm bool) {

	b := make([]byte, 1)

	file, err := os.Open(csvfile)
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
			go ShellScriptRemoteExec(host, pf, &gw, csvfile, log_confirm)
		}
	}
	gw.Wait() // 阻塞，直到WaitGroup中的计数器为0
	fmt.Printf("Press any key to exit...")
	os.Stdin.Read(b)
}

// 执行前检查
func Pre_execution_inspection(curPath string, pf PathFile, csvfiles []string) {

	// 执行前目录检查
	if pe, _ := PathExists(curPath + "/config"); !pe {
		LogStdout(curPath + "/config  目录不存在， 请创建")
	}

	// 检查 shell 命令脚本文件, 默认 command.sh
	if judge, _ := PathExists(pf.shellScript); !judge {
		LogStdout("Error : 请检查当前目录下 " + pf.shellScript + " 文件是否存在 ... ...")
		os.Exit(100)
	}

	// 检查 -f xxx.csv 文件是否存在
	for _, csvfile := range csvfiles {
		if judge, _ := PathExists(csvfile); !judge {
			LogStdout("Error : 请检查当前目录下 " + csvfile + " 文件是否存在 ... ...")
			os.Exit(100)
		}
	}
}

// 日志标准输出格式
func LogStdout(message string) {
	// 获取当前时间并格式化为 %Y%m%d %H:%M:%S
	// 注意：月份是小写m，日期是小写d（大写M是分钟）
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	// 输出带时间戳的日志
	fmt.Printf("\n[%s] %s\n", timestamp, message)
}

// 交互式
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

func ShellScriptRemoteExec(host Host, pf PathFile, group *sync.WaitGroup, csvfile string, log_confirm bool) {
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
		LogStdout(csvfile + "    " + host.ip + "              fail")
		LogStdout("SSH 连接 error: " + err.Error())
		return
	}

	client, err := scp.NewClientBySSH(sshClt)
	if err != nil {
		LogStdout(csvfile + "    " + host.ip + "              fail")
		LogStdout("Error creating new SSH session from existing connection " + err.Error())
		return
	}
	defer client.Close()

	// scp 脚本
	buff, _ := ioutil.ReadFile(pf.shellScript)
	r := bytes.NewReader(buff)

	ctx := context.Background()
	err = client.CopyFile(ctx, r, tmpscript, "0777")
	if err != nil {
		LogStdout(csvfile + "    " + host.ip + "              fail")
		LogStdout("Error : 拷贝脚本失败 ... ...  " + err.Error())
		return
	}

	// 建立新会话
	session, err := sshClt.NewSession()
	if err != nil {
		LogStdout(csvfile + "    " + host.ip + "              fail")
		LogStdout("new session error: " + err.Error())
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // 回显（0禁用，1启动）
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud 波特（Baud）即调制速率
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud 波特（Baud）即调制速率
	}

	if err = session.RequestPty("xterm", 80, 40, modes); err != nil {
		LogStdout(csvfile + "    " + host.ip + "              fail")
		LogStdout("Error : " + err.Error()) // session output
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

	LogAct(err, stdoutBuf.String(), logger, host, csvfile, log_confirm)

}

func LogAct(err error, info interface{}, logger *log.Logger, host Host, csvfile string, log_confirm bool) {
	if err == nil {
		LogStdout(csvfile + "    " + host.ip + "              done")
		if log_confirm {
			fmt.Printf("session output :\n%s", info)
		} else {
			logger.Printf("session output :\n%s", info) // session output
		}
	} else {
		LogStdout(csvfile + "    " + host.ip + "              fail")
		if log_confirm {
			fmt.Printf("\n%s", info) // session output
			fmt.Println(err.Error())
			fmt.Println("执行脚本失败 ... ... ...")
		} else {
			logger.Printf("\n%s", info) // session output
			logger.Println(err.Error())
			logger.Println("执行脚本失败 ... ... ...")
		}
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
