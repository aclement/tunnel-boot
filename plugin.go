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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"os/signal"
	"syscall"

	"code.cloudfoundry.org/cli/cf/flags"
	"code.cloudfoundry.org/cli/plugin"

	"io/ioutil"
	"log"
	"strings"
	"text/template"

	"github.com/aclement/tunnel-boot/cli"
	"github.com/aclement/tunnel-boot/format"
	"github.com/aclement/tunnel-boot/pluginutil"

	"bytes"

	"os/exec"

	"github.com/dynport/gossh"
)

// Plugin version. Substitute "<major>.<minor>.<build>" at build time, e.g. using -ldflags='-X main.pluginVersion=1.2.3'
var pluginVersion = "invalid version - plugin was not built correctly"

// Plugin is a struct implementing the Plugin interface, defined by the core CLI, which can
// be found in the "code.cloudfoundry.org/cli/plugin/plugin.go"
type Plugin struct {
	cliConnection plugin.CliConnection
	deployer      DeployerOps
}

func MakeLogger(prefix string) gossh.Writer {
	return func(args ...interface{}) {
		log.Println(append([]interface{}{prefix}, args...)...)
	}
}

func (p *Plugin) Run(cliConnection plugin.CliConnection, args []string) {
	var positionalArgs []string
	var flags map[string]string
	var err error

	p.deployer.Connect(cliConnection)

	flags, positionalArgs, err = parseFlagsAndOptions(args)

	if err != nil {
		format.Diagnose(string(err.Error()), os.Stderr, func() {
			os.Exit(1)
		})
	}

	argsConsumer := cli.NewArgConsumer(positionalArgs, diagnoseWithHelp)

	// TODO verify ssh enabled on target system
	switch args[0] {

	case "push-tunnel-app":
		cfApplicationName := getApplicationName(argsConsumer)
		springApplicationName := getSpringApplicationName(argsConsumer)
		fmt.Println("Pushing tunnel hosting application:", cfApplicationName)
		tempDir := getTempDir()
		shadowAppPath := unpackTunnelApplication(tempDir)
		manifestPath := unpackManifestTemplateAndFillIn(tempDir, cfApplicationName, springApplicationName, flags["services"], shadowAppPath)
		p.deployer.PushApp(cfApplicationName, manifestPath)

	case "get-local-env":
		applicationName := getApplicationName(argsConsumer)
		// fmt.Println("Fetching env vars for tunnel application:",applicationName)
		varData := p.deployer.GetEnvVars(applicationName)
		vars := processVars(varData)
		if flags["create-eclipse-launch-config"] == "true" {
			produceEclipseLaunchConfiguration(flags["target-dir"], flags["project"], flags["application-main"], flags["port"], vars)
		} else {
			fmt.Println("Variables for use in your IDE:")
			for varName, varValue := range vars {
				fmt.Println(varName + "=" + varValue)
			}
			fmt.Println("Variables for use on the command line:")
			for varName, varValue := range vars {
				fmt.Println(varName + "=\"" + strings.Replace(varValue, "\"", "\\\"", -1) + "\"")
			}
		}

	case "start-tunnel":
		applicationName := getApplicationName(argsConsumer)
		localPort := getLocalPort(argsConsumer)
		code := p.deployer.GetSshCode()
		guid := p.deployer.GetGuid(applicationName)

		fmt.Println("The guid for the app is " + guid)
		fmt.Println("The one time ssh code is " + code)

		sigs := make(chan os.Signal, 1)

		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		_, err := exec.LookPath("sshpass")
		if err != nil {
			fmt.Printf("Unable to find sshpass, please install it and re-run or execute the following ssh command manually to start the tunnel\n")
			fmt.Println("  ssh -N -p 2222 cf:" + guid + "/0@ssh.run.pivotal.io -R *:8080:localhost:" + localPort)
			fmt.Printf("(supply the sshcode printed above, or create a new one via: cf ssh-code")
			os.Exit(1)
		}

		fmt.Println("Connecting tunnel, command:\n  sshpass -p " + code + " ssh -N -p 2222 cf:" + guid + "/0@ssh.run.pivotal.io -R *:8080:localhost:" + localPort)
		sshCmd := exec.Command("sshpass", "-p", code, "ssh", "-N", "-p", "2222", "cf:"+guid+"/0@ssh.run.pivotal.io", "-R", "*:8080:localhost:"+localPort)

		stdoutIn, _ := sshCmd.StdoutPipe()
		stderrIn, _ := sshCmd.StderrPipe()

		var stdoutBuf, stderrBuf bytes.Buffer
		var errStdout, errStderr error

		stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
		stderr := io.MultiWriter(os.Stderr, &stderrBuf)

		err = sshCmd.Start()
		if err != nil {
			log.Fatalf("sshCmd.Start() failed with %s\n", err)
		}

		go func() {
			_, errStdout = io.Copy(stdout, stdoutIn)
		}()

		go func() {
			_, errStderr = io.Copy(stderr, stderrIn)
		}()

		err = sshCmd.Wait()
		if err != nil {
			log.Fatalf("sshCmd.Start() failed with '%s'\n", err)
		}
		if errStdout != nil || errStderr != nil {
			log.Fatal("failed to capture stdout or stderr\n")
		}
		outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
		fmt.Printf("\nout:\n%s\nerr:\n%s\n", outStr, errStr)
	}
}

func (p *Plugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:    "tunnel-boot",
		Version: pluginutil.ParsePluginVersion(pluginVersion, failInstallation),
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "push-tunnel-app",
				HelpText: "Push an application to act as an ssh tunnel host",
				Alias:    "pta",
				UsageDetails: plugin.Usage{
					Usage: `   cf push-tunnel-app CF_APPLICATION_NAME SPRING_APPLICATION_NAME`,
					// add a flag to indicate the spring app name (eureka) vs the cf app name
					Options: map[string]string{
						"--services/--s <servicesList>": "comma separated list of services to bind to",
					},
				},
			},
			{
				Name:     "get-local-env",
				HelpText: "Retrieve environment vars to specify for local app launching",
				Alias:    "gle",
				UsageDetails: plugin.Usage{
					Usage: `   cf get-local-env APPLICATION_NAME`,
					Options: map[string]string{
						"--create-eclipse-launch-config":      "Produce a .launch file suitable for eclipse",
						"--project <ideProjectName>":          "for eclipse config creation, this is the eclipse project name",
						"--application-main <fqAppClassName>": "for eclipse config creation, the application main class",
						"--port <nnnn>":                       "for eclipse config creation, the local port number being tunneled to",
						"--target-dir <folder>":               "for eclipse config creation, target directory in which to create .launch file",
					},
				},
			},
			{
				Name:     "start-tunnel",
				HelpText: "Create the ssh tunnel to connect a local port to the CF application",
				Alias:    "stun",
				UsageDetails: plugin.Usage{
					Usage: `   cf start-tunnel CF_APPLICATION_NAME LOCAL_PORT`,
				},
			},
		},
	}
}

func uninstalling() {
	os.Remove(filepath.Join(os.TempDir(), "uninstall-test-file-for-test_1.exe"))
}

func getApplicationName(ac *cli.ArgConsumer) string {
	return ac.Consume(1, "application name")
}

func getSpringApplicationName(ac *cli.ArgConsumer) string {
	return ac.Consume(2, "application name")
}

func getLocalPort(argsConsumer *cli.ArgConsumer) string {
	return argsConsumer.Consume(2, "local port")
}

func diagnoseWithHelp(message string, command string) {
	fmt.Printf("%s See 'cf help %s'.\n", message, command)
	os.Exit(1)
}

func getTempDir() string {
	tempDir, err := ioutil.TempDir("", "tunnel-boot")
	if err != nil {
		format.Diagnose(string(err.Error()), os.Stderr, func() {
			os.Exit(1)
		})
	}
	return tempDir
}

// Pull the Spring Boot shadow application from the resources package and write it
// to somewhere on disk that we can push from
func unpackTunnelApplication(tempDir string) string {
	data, err := Asset("resources/tunnelapp.jar")
	if err != nil {
		format.Diagnose(string(err.Error()), os.Stderr, func() {
			os.Exit(1)
		})
	}

	shadowAppFile := filepath.Join(tempDir, "tunnelapp.jar")

	err = ioutil.WriteFile(shadowAppFile, []byte(data), 0644)
	if err != nil {
		format.Diagnose(string(err.Error()), os.Stderr, func() {
			os.Exit(1)
		})
	}
	fmt.Println("Unpacked tunnel application to", shadowAppFile)
	return shadowAppFile
}

// <stringAttribute key="org.eclipse.jdt.launching.MAIN_TYPE" value="io.spring.cloud.samples.fortuneteller.fortuneservice.Application"/>
// <stringAttribute key="org.eclipse.jdt.launching.PROJECT_ATTR" value="fortune-teller-fortune-service"/>

const eclipseLaunchConfigTemplate = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<launchConfiguration type="org.springframework.ide.eclipse.boot.launch">
{{if .EnvVars}}<mapAttribute key="org.eclipse.debug.core.environmentVariables">
{{range $key, $value := .EnvVars }}<mapEntry key="{{$key}}" value="{{quote $value}}"/>
{{end}}</mapAttribute>{{end}}
<booleanAttribute key="org.eclipse.jdt.launching.ATTR_USE_START_ON_FIRST_THREAD" value="true"/>
<stringAttribute key="org.eclipse.jdt.launching.MAIN_TYPE" value="{{.AppMainClass}}"/>
<stringAttribute key="org.eclipse.jdt.launching.PROJECT_ATTR" value="{{.ProjectName}}"/>
<booleanAttribute key="spring.boot.ansi.console" value="true"/>
<booleanAttribute key="spring.boot.dash.hidden" value="false"/>
<booleanAttribute key="spring.boot.debug.enable" value="false"/>
<booleanAttribute key="spring.boot.fast.startup" value="true"/>
<booleanAttribute key="spring.boot.jmx.enable" value="true"/>
<booleanAttribute key="spring.boot.lifecycle.enable" value="true"/>
<stringAttribute key="spring.boot.lifecycle.termination.timeout" value="15000"/>
<booleanAttribute key="spring.boot.livebean.enable" value="false"/>
<stringAttribute key="spring.boot.livebean.port" value="0"/>
<stringAttribute key="spring.boot.profile" value=""/>
{{range $key, $value := .ExtraProps}}<stringAttribute key="{{$key}}" value="1{{$value}}"/>
{{end}}</launchConfiguration>
`

// These are the kinds of value we want to insert in the template:

//<mapAttribute key="org.eclipse.debug.core.environmentVariables">
//<mapEntry key="VCAP_APPLICATION" value="..."/>
//<mapEntry key="VCAP_SERVICES" value="..."/>
//</mapAttribute>

// <stringAttribute key="spring.boot.prop.eureka.client.register-with-eureka:0" value="1false"/>
// <stringAttribute key="spring.boot.prop.server.port:1" value="18081"/>
// <stringAttribute key="spring.boot.prop.spring.profiles.active:2" value="1cloud"/>

type LaunchConfigInfo struct {
	ProjectName  string
	AppMainClass string
	ExtraProps   map[string]string
	EnvVars      map[string]string
}

func quote(value string) string {
	return strings.Replace(value, "\"", "&quot;", -1)
}

func produceEclipseLaunchConfiguration(targetDir string, projectName string, applicationMain string, port string, envVars map[string]string) {

	extraProps := make(map[string]string)
	extraProps["spring.boot.prop.eureka.client.register-with-eureka:0"] = "false"
	extraProps["spring.boot.prop.server.port:1"] = port
	extraProps["spring.boot.prop.spring.profiles.active:2"] = "cloud"

	funcs := template.FuncMap{"quote": quote}
	launchConfigInfo := LaunchConfigInfo{projectName, applicationMain, extraProps, envVars}
	scriptBuf := &bytes.Buffer{}
	parsedTemplate := template.Must(template.New("").Funcs(funcs).Parse(eclipseLaunchConfigTemplate))
	err := parsedTemplate.Execute(scriptBuf, launchConfigInfo)
	if err != nil {
		fmt.Printf("Busted: %s\n", err)
	}

	if len(targetDir) != 0 {
		launchConfigFile := filepath.Join(targetDir, projectName+" (local).launch")

		err = ioutil.WriteFile(launchConfigFile, []byte(scriptBuf.String()), 0644)
		if err != nil {
			format.Diagnose(string(err.Error()), os.Stderr, func() {
				os.Exit(1)
			})
		}
		fmt.Println("Created launch configuration ", launchConfigFile)
	} else {
		fmt.Printf(scriptBuf.String())
	}
}

func processVars(varData []string) map[string]string {
	// varData like this

	// blahblahblah
	// {
	//   "VCAP_APPLICATION": {
	//      useful_stuff1
	//   }
	// }
	// {
	//   "VCAP_SERVICES": {
	//      useful_stuff2
	//   }
	// }
	// blahblahblah

	// output should be a map of VCAP_APPLICATION=useful_stuff1 and VCAP_SERVICES=useful_stuff2 with appropriate
	// escaping on quotes in the useful_stuff data

	var usefulDataAccumulator []string
	vars := make(map[string]string)
	for i := 0; i < len(varData); i++ {
		line := varData[i]
		if strings.HasPrefix(line, "}") {
			varName := usefulDataAccumulator[0][2:strings.LastIndex(usefulDataAccumulator[0], "\"")]
			varValue := "{"
			for j := 1; j < len(usefulDataAccumulator); j++ {
				varLine := usefulDataAccumulator[j]
				varValue = varValue + strings.TrimSpace(varLine) //strings.Replace(strings.TrimSpace(varLine),"\"","\\\"",-1)
			}
			vars[varName] = varValue
			usefulDataAccumulator = nil
		}
		if usefulDataAccumulator != nil {
			usefulDataAccumulator = append(usefulDataAccumulator, line)
		}
		if strings.HasPrefix(line, "{") {
			usefulDataAccumulator = make([]string, 0)
		}
	}
	return vars
}

func unpackManifestTemplateAndFillIn(tempDir string, cfApplicationName string, springApplicationName string, services string, packagedAppPath string) string {
	data, _ := Asset("resources/manifest.yml.template")
	manifestTemplateString := string(data)

	//	eureka.client.serviceUrl.defaultZone: SERVICE_REGISTRY_URL
	manifestTemplateString = strings.Replace(manifestTemplateString, "APPNAME", cfApplicationName, 1)
	manifestTemplateString = strings.Replace(manifestTemplateString, "PATH", packagedAppPath, 1)
	// What else the bash did:
	//if [ ! -z $2 ]
	//then
	//echo "    eureka.client.serviceUrl.defaultZone: $2" >> manifest.yml
	//fi
	//
	//if [ ! -z $3 ]
	//then
	//# convert it to eureka.instance.metadataMap.key: value
	//echo "    eureka.instance.metadataMap.needsExplicitRouting: true" >> manifest.yml
	//echo "    eureka.instance.metadataMap.$3" | sed 's/=/: /' >> manifest.yml
	//fi
	//
	manifestTemplateString = manifestTemplateString + "    spring.application.name: " + springApplicationName + "\n"

	if len(services) != 0 {
		manifestTemplateString += "  services:\n"
		serviceList := strings.Split(services, ",")
		for i := 0; i < len(serviceList); i++ {
			manifestTemplateString += "    - " + serviceList[i] + "\n"
		}
	}

	fmt.Println("Manifest to be used for deployment of tunnel application:\n", manifestTemplateString)
	manifestFile := filepath.Join(tempDir, "manifest.yml")
	ioutil.WriteFile(manifestFile, []byte(manifestTemplateString), 0644)
	return manifestFile
}

func failInstallation(format string, inserts ...interface{}) {
	// There is currently no way to emit the message to the command line during plugin installation. Standard output and error are swallowed.
	fmt.Printf(format, inserts...)
	fmt.Println("")

	// Fail the installation
	os.Exit(64)
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println("This program is a plugin which expects to be installed into the cf CLI. It is not intended to be run stand-alone.")
		pv := pluginutil.ParsePluginVersion(pluginVersion, failInstallation)
		fmt.Printf("Plugin version: %d.%d.%d\n", pv.Major, pv.Minor, pv.Build)
		os.Exit(0)
	}
	p := Plugin{
		deployer: &Deployer{
			errorFunc: func(message string, err error) {
				log.Fatalf("%v - %v", message, err)
			},
			out: os.Stdout,
		},
	}
	plugin.Start(&p)
}

func parseFlagsAndOptions(args []string) (map[string]string, []string, error) {
	const flagServices = "services"
	options := make(map[string]string)
	fc := flags.New()
	fc.NewStringFlag(flagServices, "s", "services")
	fc.NewBoolFlag("set", "set", "set")
	fc.NewBoolFlag("create-eclipse-launch-config", "celc", "create-eclipse-launch-config")
	fc.NewIntFlag("port", "port", "port")
	fc.NewStringFlag("target-dir", "target-dir", "target-dir")
	fc.NewStringFlag("project", "project", "project")
	fc.NewStringFlag("application-main", "application-main", "application-main")
	err := fc.Parse(args...)
	if err != nil {
		return nil, nil, fmt.Errorf("Error parsing arguments: %s", err)
	}
	if fc.IsSet(flagServices) {
		options[flagServices] = fc.String(flagServices)
	}
	if fc.IsSet("project") {
		options["project"] = fc.String("project")
	}
	if fc.IsSet("target-dir") {
		options["target-dir"] = fc.String("target-dir")
	} else {
		options["target-dir"] = ""
	}
	if fc.IsSet("port") {
		options["port"] = fmt.Sprint(fc.Int("port"))
	}
	if fc.IsSet("application-main") {
		options["application-main"] = fc.String("application-main")
	}
	if fc.IsSet("set") {
		options["set"] = "true"
	}
	if fc.IsSet("create-eclipse-launch-config") {
		options["create-eclipse-launch-config"] = "true"
	}
	return options, fc.Args(), nil
}
