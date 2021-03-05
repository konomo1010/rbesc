package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	"log"
	"os"

	//"os"
)

type Host struct {
	ip string
	port string
	user string
	password string
}
var (
	shellScript = "./config/command.sh"
	hostsConfig = "./config/hosts_config.csv"

	remoteSaveShellScriptPath = "/tmp/command.sh"
)
func main() {
	fmt.Println("starting ... ... ...")
	// 执行前目录文件检查
	if judge,_ := PathExists(shellScript); !judge {
		log.Fatalln("Eorror : 请检查当前目录下 config/command.sh 文件是否存在 ... ...")
		os.Exit(100)
	}


	fmt.Println(os.Getwd())
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

	fmt.Println(records)

	for i,v := range records {
		if i != 0 {
			host := HostSliceToStruct(v)
			huixian := ShellScriptRemoteExec(host)
			fmt.Println(huixian)
		}
	}
}


func connectSSH(host Host) *ssh.Client {
	// setup SSH connection
	cfg := &ssh.ClientConfig{
		User: host.user,
		Auth: []ssh.AuthMethod{ssh.Password(host.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host.ip+":"+host.port, cfg)
	if err != nil {
		log.Fatalf("SSH dial error: %s", err.Error())
	}

	return client
}

func ShellScriptRemoteExec(host Host) string {
	sshClient := connectSSH(host)
	client, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		fmt.Println("Error creating new SSH session from existing connection", err)
	};defer client.Close()



	// scp 脚本
	f, _ := os.Open(shellScript)
	defer f.Close()


	err = client.CopyFile(f, remoteSaveShellScriptPath, "0777")
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
		panic("Failed to run: " + err.Error())
	}
	return b.String()
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