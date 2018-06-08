/*
 * Copyright (C) 2018-Present Pivotal Software, Inc. All rights reserved.
 *
 * This program and the accompanying materials are made available under
 * the terms of the under the Apache License, Version 2.0 (the "License”);
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
 package main

import (
	"code.cloudfoundry.org/cli/plugin"
	"fmt"
	"io"
	"strings"
)

type ErrorHandler func(string, error)

type DeployerOps interface {
	PushApp(string, string)
	GetEnvVars(string) []string
	Connect(plugin.CliConnection)
	CreateTunnelIn(string)
	FetchInetAddr(string) string
	GetSshCode() string
	GetGuid(string) string
}

type Deployer struct {
	cliConnection plugin.CliConnection
	errorFunc ErrorHandler
	out io.Writer
}

func (d *Deployer) PushApp(applicationName string, manifestPath string) {
	args := []string{"push", applicationName}
	fmt.Println("Pushing",applicationName)
	if manifestPath != "" {
		args = append(args, "-f", manifestPath)
	}
	if _,err := d.cliConnection.CliCommand(args...); err != nil {
		d.errorFunc("Could not push new version", err)
	}
}

func (d *Deployer) GetEnvVars(applicationName string) []string {
	args := []string{"env", applicationName}
	varData,err := d.cliConnection.CliCommandWithoutTerminalOutput(args...)
	if err != nil {
		d.errorFunc("Could not get env vars",err)
	}
	return varData
}

func (d *Deployer) CreateTunnelIn(applicationName string) {
	SSHD_PORT_IN_SHADOW_APP := "9099"
	args := []string{"ssh",applicationName,"--force-pseudo-tty","-L","2225:localhost:"+SSHD_PORT_IN_SHADOW_APP}
	fmt.Println("Creating local tunnel")
	if _,err := d.cliConnection.CliCommand(args...); err != nil {
		d.errorFunc("Problem creating tunnel", err)
	}
	fmt.Println("End of tunnel creation...")
}

func (d* Deployer) GetSshCode() string {
	args:=[]string{"ssh-code"}
	fmt.Println("Fetching ssh-code, command:\n  cf ssh-code")
	codeOutput,err := d.cliConnection.CliCommandWithoutTerminalOutput(args...)
	if err !=nil {
		d.errorFunc("Problem fetching an ssh-code",err)
	}
	return strings.Join(codeOutput,"")
}

func (d* Deployer) GetGuid(applicationName string) string {
	args:=[]string{"app",applicationName,"--guid"}
	fmt.Println("Fetching guid, command:\n  cf app",applicationName,"--guid")
	cmdOutput,err := d.cliConnection.CliCommandWithoutTerminalOutput(args...)
	if err !=nil {
		d.errorFunc("Problem fetching guid",err)
	}
	return strings.Join(cmdOutput,"")
}

func (d* Deployer) FetchInetAddr(applicationName string) string {
	args:=[]string{"ssh",applicationName,"--force-pseudo-tty","-c",`/sbin/ifconfig eth0 | grep "inet addr" | sed 's/^[^:]*:\([^ ]*\).*$/\1/'`}
//	args:=[]string{"spaces"}//ssh",applicationName,"-v","-c",`ls`}
	fmt.Println("Fetching inet address")
	//originalStdout := os.Stdout
	//r,w,_:=os.Pipe()
	//os.Stdout=w
	fmt.Println("Capturing output")
	x,err := d.cliConnection.CliCommand(args...)
	//w.Close()
	//out,_:=ioutil.ReadAll(r)
	//os.Stdout=originalStdout
	if err != nil {
		d.errorFunc("Problem fetch inet addr", err)
	}
	fmt.Println("Fetched: ",strings.Join(x,""))//,string(out))
	return strings.Join(x,"")
}

func (d *Deployer) Connect(cliConnection plugin.CliConnection) {
	d.cliConnection = cliConnection
}