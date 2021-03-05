package main

import (
	"bytes"
	//"debug/macho"
	"encoding/csv"
	"fmt"
	scp "github.com/bramvdbogaerde/go-scp"
	termbox "github.com/nsf/termbox-go"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os"
	"time"

	//"os"
)

type Host struct {
	ip string
	port string
	user string
	password string
}

var (
	//shellScript string
	//hostsConfig string

	shellScript = "./config/command.sh"
	hostsConfig = "./config/hosts_config.csv"

	remoteSaveShellScriptPath = "/tmp/command.sh"
)

// 清屏
func init() {
	if err := termbox.Init(); err != nil {
		panic(err)
	}
	termbox.SetCursor(0, 0)
	termbox.HideCursor()
}


func main() {
	fmt.Println("starting ... ... ...")
	//fmt.Println("---> ",getExecutePath2())
	//execPwd := getExecutePath2()
	//shellScript = execPwd + "./config/command.sh"
	//hostsConfig = execPwd + "./config/hosts_config.csv"
	// 执行前目录文件检查
	if judge,_ := PathExists(shellScript); !judge {
		log.Fatalf("Eorror : 请检查当前目录下 %s 文件是否存在 ... ...",shellScript)
		os.Exit(100)
	}

	//fmt.Println(os.Getwd())
	file, err := os.Open(hostsConfig)
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

	BatchExec(records)
	pause()  //最后请按任意键继续
}

func BatchExec(records [][]string) {
	for i,v := range records {
		if i != 0 {
			host := HostSliceToStruct(v)
			fmt.Printf("%s              ", host.ip )
			if flag, err := ShellScriptRemoteExec(host); flag {
				fmt.Printf("ok\n")
			}else {
				fmt.Printf("%s\n",err.Error())
			}

		}
	}
}


func ShellScriptRemoteExec(host Host) (bool, error) {
	//timer := time.NewTimer(5 * time.Second)

	// setup SSH connection
	cfg := &ssh.ClientConfig{
		User: host.user,
		Auth: []ssh.AuthMethod{ssh.Password(host.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: 5 * time.Second, // time.Duration
	}

	sshClient, err := ssh.Dial("tcp", host.ip+":"+host.port, cfg)

	if err != nil {
		//log.Fatalf("SSH dial error: %s", err.Error())
		//fmt.Printf("SSH dial error: %s", err.Error())
		return false, err
	}

	client, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		fmt.Println("Error creating new SSH session from existing connection", err)
	};defer client.Close()

	// scp 脚本
	//f, _ := os.OpenFile(shellScript, os.O_RDONLY , 6)
	buff, _ := ioutil.ReadFile(shellScript)

	//defer f.Close()
	//fmt.Println(string(buff))
	r := bytes.NewReader(buff)
	err = client.CopyFile(r, remoteSaveShellScriptPath, "0777")
	if err != nil {
		fmt.Println("Error : 拷贝脚本失败 ... ...  ", err)
	}

	// 建立新会话
	session, err := sshClient.NewSession()
	if err != nil {
		log.Fatalf("new session error: %s", err.Error())
	};defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(remoteSaveShellScriptPath); err != nil {
		//panic("Failed to run: " + err.Error())
		return false, err

	}
	return true, nil
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

func pause() {
	fmt.Println("请按任意键继续...")
Loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			break Loop
		}
	}
}




