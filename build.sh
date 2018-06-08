mkdir resources 2>/dev/null

cd TunnelApp
mvn package
cd ..

# Copy in the pieces from the tunnel app project that will be included in the CLI plugin
cp TunnelApp/manifest.yml.template resources/.
cp TunnelApp/target/TunnelApp-0.0.1.RELEASE.jar resources/tunnelapp.jar

go-bindata -pkg main -o resources.go resources/
govendor install -ldflags="-X main.pluginVersion=$(cat version)" +local
