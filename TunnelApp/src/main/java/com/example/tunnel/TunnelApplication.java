/**
 * Copyright (c) 2018 the original author or authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package com.example.tunnel;

import java.io.File;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.CommandLineRunner;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.embedded.EmbeddedWebApplicationContext;
import org.springframework.cloud.netflix.sidecar.EnableSidecar;
import org.springframework.stereotype.Component;

/**
 * This does start a web app on port 5678 because of the use of @EnableSidecar
 * but we don't want that to hang around so we close it early on.  This leaves
 * the sidecar behaviour running. The sidecar behaviour is to 
 * check a specific health endpoint and if that seems ok, register it with
 * the service registry.  So what happens is that a reverse tunnel is setup
 * when the developer starts their app locally from this containers 8080 port.
 * Since that is the tunneled port, the sidecar is effectively checking the
 * health of the local developers app and if up the service registry will contain
 * an UP entry for this app. So if the service registry indicates the app is
 * UP then it means the developer is running the app on localhost and this
 * app can forward traffic to it!
 * 
 * @author Andy Clement
 */
@EnableSidecar
@SpringBootApplication
public class TunnelApplication {

	public static void main(String[] args) {
		SpringApplication.run(TunnelApplication.class, args);
	}
}

@Component
class MyCommandLineRunner implements CommandLineRunner {

	@Autowired
    	EmbeddedWebApplicationContext server;
	
	@Override
	public void run(String... args) throws Exception {
		
		// Don't want this thing, this is started incidentally because we are running @EnableSideCar
		server.getEmbeddedServletContainer().stop();
		
		while (true) {
			Thread.sleep(10000); // Zzzzzz
			System.out.println("z");
		}
	}
	
}
