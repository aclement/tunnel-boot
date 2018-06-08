# tunnel-boot
Disclaimer: this is an experiment, feedback is welcome, there are rough edges!

tunnel-boot is a CF CLI plugin that enables you to easily develop Spring Boot applications
locally on your desktop that are reachable from endpoints on Cloud Foundry. This can be invaluable
when working on a system composed of microservices as it enables you to only run the service you want
to develop/debug locally whilst all the others continue to run in your space on Cloud Foundry.

In a couple of steps tunnel-boot CLI commands will push an empty app up to CF that can host tunnels,
create a tunnel from your local machine to that app and then provide you with all the necessary
properties/variables you need to launch your local app that enable it to act as the local endpoint
for the tunnel and also communicate with other remote services.

## Installation

If you want to install a release directly into your CF CLI, choose one of these:

```
cf install-plugin https://github.com/aclement/tunnel-boot/releases/download/0.0.1/tunnel-boot-darwin-amd64-0.0.1
cf install-plugin https://github.com/aclement/tunnel-boot/releases/download/0.0.1/tunnel-boot-linux-386-0.0.1
cf install-plugin https://github.com/aclement/tunnel-boot/releases/download/0.0.1/tunnel-boot-linux-amd64-0.0.1
```

You can build it yourself:

```bash
$ rm $GOPATH/bin/tunnel-boot
$ cd $GOPATH/src/github.com/aclement/tunnel-boot
$ ./build.sh
```

To print the version number of the built plugin, run it as a stand-alone executable, for example:

```bash
$ $GOPATH/bin/tunnel-boot
This program is a plugin which expects to be installed into the cf CLI. It is not intended to be run stand-alone.
Plugin version: 0.0.1
```

To install the plugin in the `cf` CLI, first build it and then issue:

```bash
$ cf install-plugin -f $GOPATH/bin/tunnel-boot
```

The plugin's commands may then be listed by issuing `cf help`.

To update the plugin, uninstall it as follows and then re-install the plugin as above:
```bash
$ cf uninstall-plugin tunnel-boot
```

## Usage:

There are three commands to run in order to get up and running. Once run you can just develop your app,
there is only a need to re-run these commands if something changes about your setup (the kinds of
event requiring this are indicated below).

### STEP 1:

First you want to push an app to CF that will host the ssh tunnels. Note: this app
should be bound to the same services as the real app.  Here is the `manifest.yml` for a simple
example app that we want to work on locally (the fortune-teller service from the
SCS fortune-teller sample):

```
---
applications:
- name: fortune-service
  memory: 1024M
  host: fortunes-andy
  path: target/fortune-teller-fortune-service-0.0.1-SNAPSHOT.jar
  services:
  - fortunes-db
  - fortunes-config-server
  - fortunes-service-registry
```

So the first CLI command to be run is of the form:

```
cf push-tunnel-app CF_APP_NAME SPRING_APPLICATION_NAME [--services list_of_services]
```

In our case:

```
cf push-tunnel-app fortune-service-tunnel fortune-service --services fortunes-db,fortunes-config-server,fortunes-service-registry
```

This means push a cf app called fortune-service-tunnel with spring application name fortune-service that
should be bound to the database, config-server and service-registry services.
The spring application name is important if binding to a service registry since that is the name that
will be used for registration. Anyone looking up
that name in the registry will find the tunnel app and potential send traffic to it

TODOs around this...
- enable the command to take a manifest as input for discovering configuration data
- support more configuration than just services (domains, etc)

This push make take a couple of minutes but doesn't need repeating *unless* the service bindings for your
application change (you need to add or remove a service, for example)

### STEP 2:

Once the app is running, we want to start a tunnel from that app to our local machine. 

```
cf start-tunnel CF_APPLICATION_NAME LOCAL_PORT_NUMBER
```

The key information
we need is the name of the CF application and the local port we want it to forward traffic too. In our case
the app name was fortune-service-tunnel and let's choose port 9000 locally:

```
cf start-tunnel fortune-service-tunnel 9000
```

This is going run a sequence of cf and ssh commands to startup a tunnel that is using reverse
port forwarding. It does use `sshpass` to pass the necessary password to the ssh command, you made need
to install that. It will print out all the commands so if you don't want to install sshpass it will show
you what it is attempting and you can run it yourself:

This step only needs repeating if repeating step 1 or something strange happens that takes down the tunnel.
This will continue to run in the shell in which you invoke it, Ctrl+C will shut down the tunnel.

### STEP 3:

Finally we want to launch the app. We want to launch our app like we would on CF, and typically spring
apps would be using something like Spring Cloud Connectors to read the VCAP environment variables and
use those to construct real connections to services. If we'd like connectors to do the same job on the 
desktop we should set the same variables. You can do this by hand by calling `cf env CF_APPLICATION_NAME`
and manually processing the output, or you can use the third command:

```
cf get-local-env CF_APPLICATION_NAME
```

(Note: there is a shortcut below if using eclipse)

In our case:

```
cf get-local-env fortune-service-tunnel
```

This will print out values for VCAP_SERVICES and VCAP_APPLICATION that you can either paste into
launch options inside your IDE, or export on the command line if that is how you are going to run the app.
In addition to setting VCAP_SERVICES and VCAP_APPLICATION you should set these spring properties:

```
server.port=LOCAL_PORT_WHERE_TUNNEL_WAS_TARGETING
spring.profiles.active=cloud
eureka.client.register-with-eureka=false
```

With those two VCAP environment variables set and the three properties, you can launch your application (and
restart it) and any traffic sent to the cloud foundry tunnel app will be forwarded down to your local
app. Due to the use of VCAP env vars, if your local app is making use of services up on the cloud, it
will still have access (service registry, config server, database).

If using eclipse, there is a shortcut where you can ask this command to build you a valid launch
configuration directly:

```
cf get-local-env fortune-service-tunnel --create-eclipse-launch-config --application-main FULLY_QUALIFIED_APPLICATION_MAIN_TYPE --port LOCAL_PORT --project ECLIPSE_PROJECT_NAME --target-dir ECLIPSE_PROJECT_FOLDER_ON_DISK
```

This will create a `.launch` file in the specified target directory that, after refreshing your package
explorer will appear as a launch option immediately for your project. This is the command for our sample:

```
cf get-local-env fortune-service-tunnel --create-eclipse-launch-config --application-main io.spring.cloud.samples.fortuneteller.fortuneservice.Application --port 9000 --project fortune-teller-fortune-service --target-dir ~/gits/fortune-teller/fortune-teller-fortune-service
```

Once setup to launch the app, you can repeatedly launch it, there is no need to restart the tunnel,
you can launch the app in debug mode and debugger will start when the next request comes in.


Lots of potential TODOs:

- Proper IDE tooling to use these building blocks and run them all in one step from the IDE
- Provide service rewriting to enable services also be tunnelled
- Suggestions?
